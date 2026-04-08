package livestats

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/net2share/dnstm/internal/monitor"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/go-corelib/tui"
)

// RefreshInterval is how often the stats view reloads data.
const RefreshInterval = 1 * time.Second

// tickMsg signals a refresh.
type tickMsg time.Time

// Model is the bubbletea model for live-updating stats.
type Model struct {
	tunnels  []*router.Tunnel
	width    int
	height   int
	scroll   int
	lines    []string
	quitting bool
}

// New creates a new live stats model.
func New(tunnels []*router.Tunnel) Model {
	m := Model{tunnels: tunnels}
	m.lines = m.buildLines()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(RefreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.scroll > 0 {
				m.scroll--
			}
		case "down", "j":
			if m.scroll < m.maxScroll() {
				m.scroll++
			}
		case "home":
			m.scroll = 0
		case "end":
			m.scroll = m.maxScroll()
		case "pgup":
			m.scroll -= m.visibleLines()
			if m.scroll < 0 {
				m.scroll = 0
			}
		case "pgdown":
			m.scroll += m.visibleLines()
			if m.scroll > m.maxScroll() {
				m.scroll = m.maxScroll()
			}
		}
	case tickMsg:
		m.lines = m.buildLines()
		return m, tea.Tick(RefreshInterval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	boxWidth := m.width - 10
	if boxWidth > 90 {
		boxWidth = 90
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	// Title
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	var b strings.Builder
	b.WriteString(titleStyle.Render("Tunnel Statistics"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Live — refreshing every 1s"))
	b.WriteString("\n\n")

	// Apply scroll window
	visible := m.visibleLines()
	start := m.scroll
	end := start + visible
	if end > len(m.lines) {
		end = len(m.lines)
	}

	if start > 0 {
		b.WriteString(mutedStyle.Render("  ↑ more above"))
		b.WriteString("\n")
	}

	for _, line := range m.lines[start:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if end < len(m.lines) {
		b.WriteString(mutedStyle.Render("  ↓ more below"))
		b.WriteString("\n")
	}

	// Help line
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  ↑/↓ scroll • q/esc close"))

	// Wrap in box
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(1, 2).
		Width(boxWidth).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) visibleLines() int {
	v := m.height - 15
	if v < 5 {
		v = 5
	}
	return v
}

func (m Model) maxScroll() int {
	max := len(m.lines) - m.visibleLines()
	if max < 0 {
		return 0
	}
	return max
}

// buildLines reads stats from disk and formats them.
func (m Model) buildLines() []string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	var lines []string

	for _, t := range m.tunnels {
		active := service.IsServiceActive(t.ServiceName)
		status := inactiveStyle.Render("Stopped")
		if active {
			status = activeStyle.Render("Running")
		}

		lines = append(lines, sectionStyle.Render(fmt.Sprintf("─── %s ", t.Tag))+status)
		lines = append(lines, kv(keyStyle, valStyle, "Domain", t.Domain))

		// Show metrics endpoint status
		if metricsAddr := monitor.ReadMetricsConf(t.Tag); metricsAddr != "" {
			lines = append(lines, kv(keyStyle, valStyle, "Metrics", metricsAddr+"/metrics"))
		}

		result, err := monitor.ReadStats(t.Tag)
		if err != nil {
			lines = append(lines, kv(keyStyle, valStyle, "Error", err.Error()))
			lines = append(lines, "")
			continue
		}
		if result == nil {
			if !monitor.IsSnifferRunning(t.Tag) {
				lines = append(lines, kv(keyStyle, valStyle, "Monitor", "Not running"))
			} else {
				lines = append(lines, kv(keyStyle, valStyle, "Status", "Waiting for data..."))
			}
			lines = append(lines, "")
			continue
		}

		tr := findTunnelResult(t.Domain, result)
		if tr == nil || tr.TotalQueries == 0 {
			lines = append(lines, kv(keyStyle, valStyle, "Traffic", "(no traffic yet)"))
			lines = append(lines, "")
			continue
		}

		lines = append(lines, kv(keyStyle, valStyle, "Uptime", result.Duration.Round(time.Second).String()))
		lines = append(lines, kv(keyStyle, valStyle, "Queries", fmt.Sprintf("%d (%.1f/sec)", tr.TotalQueries, tr.QueriesPerSec)))
		lines = append(lines, kv(keyStyle, valStyle, "Bandwidth In", monitor.FormatBytes(tr.TotalBytesIn)))
		lines = append(lines, kv(keyStyle, valStyle, "Bandwidth Out", monitor.FormatBytes(tr.TotalBytesOut)))

		connStr := fmt.Sprintf("%d", tr.ActiveClients)
		if tr.TotalClients > tr.ActiveClients {
			connStr = fmt.Sprintf("%d  "+keyStyle.Render("(%d total seen)"), tr.ActiveClients, tr.TotalClients)
		}
		lines = append(lines, kv(keyStyle, valStyle, "Connected", connStr))
		lines = append(lines, kv(keyStyle, valStyle, "Peak", fmt.Sprintf("%d", tr.PeakClients)))

		// Sparkline graph of connected users over time
		if len(result.History) > 1 {
			lines = append(lines, "")
			lines = append(lines, keyStyle.Render("  Users over time:"))
			lines = append(lines, "  "+renderSparkline(result.History, 50, valStyle, keyStyle))
		}

		if s := tr.Summary(); s.Count > 0 {
			lines = append(lines, "")
			lines = append(lines, keyStyle.Render("  Per-User Traffic:"))
			lines = append(lines, fmt.Sprintf("    %s  %s  %s",
				keyStyle.Render("min ")+valStyle.Render(monitor.FormatBytes(s.MinBytesTotal)),
				keyStyle.Render("med ")+valStyle.Render(monitor.FormatBytes(s.MedianBytes)),
				keyStyle.Render("max ")+valStyle.Render(monitor.FormatBytes(s.MaxBytesTotal)),
			))
			lines = append(lines, keyStyle.Render("  Session Length:"))
			lines = append(lines, fmt.Sprintf("    %s  %s  %s",
				keyStyle.Render("min ")+valStyle.Render(formatDuration(s.MinDuration)),
				keyStyle.Render("med ")+valStyle.Render(formatDuration(s.MedianDuration)),
				keyStyle.Render("max ")+valStyle.Render(formatDuration(s.MaxDuration)),
			))
		}

		if len(tr.Clients) > 0 {
			lines = append(lines, "")
			lines = append(lines, keyStyle.Render("  Users:"))
			limit := len(tr.Clients)
			if limit > 15 {
				limit = 15
			}
			for _, c := range tr.Clients[:limit] {
				marker := activeStyle.Render("●")
				if !c.Active {
					marker = inactiveStyle.Render("○")
				}
				lines = append(lines, fmt.Sprintf("    %s %s  %s  %s",
					marker,
					valStyle.Render(c.ClientID),
					valStyle.Render(monitor.FormatBytes(c.BytesTotal)),
					keyStyle.Render(fmt.Sprintf("(%d q)", c.Queries)),
				))
			}
			if len(tr.Clients) > 15 {
				lines = append(lines, keyStyle.Render(fmt.Sprintf("    ... and %d more", len(tr.Clients)-15)))
			}
		}

		lines = append(lines, "")
	}

	return lines
}

// renderSparkline renders a compact ASCII sparkline graph from history data points.
// width is the number of columns. If there are more data points than width,
// they are downsampled by averaging buckets.
func renderSparkline(history []monitor.DataPoint, width int, valStyle, keyStyle lipgloss.Style) string {
	if len(history) == 0 {
		return ""
	}

	// Downsample or use raw values
	values := make([]float64, 0, width)
	if len(history) <= width {
		for _, dp := range history {
			values = append(values, float64(dp.ActiveClients))
		}
	} else {
		// Bucket and average
		bucketSize := float64(len(history)) / float64(width)
		for i := 0; i < width; i++ {
			start := int(float64(i) * bucketSize)
			end := int(float64(i+1) * bucketSize)
			if end > len(history) {
				end = len(history)
			}
			sum := 0.0
			for _, dp := range history[start:end] {
				sum += float64(dp.ActiveClients)
			}
			values = append(values, sum/float64(end-start))
		}
	}

	// Find max for scaling
	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}

	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var sb strings.Builder

	for _, v := range values {
		if maxVal == 0 {
			sb.WriteRune(blocks[0])
			continue
		}
		idx := int(v / maxVal * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		sb.WriteRune(blocks[idx])
	}

	graph := valStyle.Render(sb.String())

	// Time labels
	elapsed := time.Since(history[0].Time).Round(time.Second)
	timeLabel := keyStyle.Render(fmt.Sprintf("  ← %s ago", elapsed))

	return graph + timeLabel
}

// formatDuration formats a duration into a compact human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func kv(keyStyle, valStyle lipgloss.Style, key, value string) string {
	return fmt.Sprintf("  %s %s", keyStyle.Render(fmt.Sprintf("%-16s", key+":")), valStyle.Render(value))
}

func findTunnelResult(domain string, result *monitor.CaptureResult) *monitor.TunnelResult {
	if result == nil || result.Tunnels == nil {
		return nil
	}
	domain = strings.ToLower(domain)
	if tr, ok := result.Tunnels[domain]; ok {
		return tr
	}
	for d, tr := range result.Tunnels {
		if strings.HasSuffix(domain, "."+d) || strings.HasSuffix(d, "."+domain) {
			return tr
		}
	}
	return nil
}

// Run launches the live stats TUI.
func Run(tunnels []*router.Tunnel) error {
	m := New(tunnels)
	var p *tea.Program
	if tui.InSession() {
		// Already in alt-screen from the TUI menu — clear and run inline
		fmt.Print("\033[H\033[2J")
		p = tea.NewProgram(m)
	} else {
		p = tea.NewProgram(m, tea.WithAltScreen())
	}
	_, err := p.Run()
	return err
}
