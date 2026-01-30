package router

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"

	"github.com/net2share/dnstm/internal/config"
)

var adjectives = []string{
	"swift", "quick", "silent", "hidden", "shadow",
	"bright", "dark", "rapid", "fast", "eager",
	"quiet", "stealth", "brave", "bold", "calm",
	"cool", "deep", "wild", "free", "pure",
	"safe", "sharp", "smart", "soft", "warm",
	"wise", "frost", "storm", "night", "dawn",
}

var nouns = []string{
	"tunnel", "stream", "channel", "bridge", "gateway",
	"path", "route", "link", "portal", "passage",
	"conduit", "relay", "proxy", "node", "point",
	"eagle", "falcon", "hawk", "raven", "wolf",
	"tiger", "lion", "bear", "fox", "owl",
	"river", "ocean", "cloud", "star", "moon",
}

var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// GenerateName generates a random adjective-noun name.
func GenerateName() string {
	adj := adjectives[rand.IntN(len(adjectives))]
	noun := nouns[rand.IntN(len(nouns))]
	return adj + "-" + noun
}

// GenerateUniqueTag generates a unique tag that doesn't conflict with existing tunnels.
func GenerateUniqueTag(cfg *config.Config) string {
	maxAttempts := 100
	for i := 0; i < maxAttempts; i++ {
		tag := GenerateName()
		if cfg.GetTunnelByTag(tag) == nil {
			return tag
		}
	}
	// Fallback: add a random suffix
	return GenerateName() + fmt.Sprintf("-%d", rand.IntN(1000))
}

// ValidateName validates an instance name.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(name) < 3 {
		return fmt.Errorf("name must be at least 3 characters")
	}

	if len(name) > 63 {
		return fmt.Errorf("name must be at most 63 characters")
	}

	if !nameRegex.MatchString(name) {
		return fmt.Errorf("name must start with a lowercase letter and contain only lowercase letters, numbers, and hyphens")
	}

	// Check for reserved names
	reserved := []string{"coredns", "router", "default", "all", "none"}
	for _, r := range reserved {
		if name == r {
			return fmt.Errorf("name '%s' is reserved", name)
		}
	}

	return nil
}

// NormalizeName normalizes a name to lowercase and replaces underscores with hyphens.
func NormalizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

// SuggestSimilarTags suggests similar tags if a tag is taken.
func SuggestSimilarTags(baseTag string, cfg *config.Config, count int) []string {
	suggestions := make([]string, 0, count)

	// Try adding numbers
	for i := 2; i <= count+10 && len(suggestions) < count; i++ {
		candidate := fmt.Sprintf("%s-%d", baseTag, i)
		if cfg.GetTunnelByTag(candidate) == nil {
			suggestions = append(suggestions, candidate)
		}
	}

	// Try different adjectives with the same noun
	parts := strings.Split(baseTag, "-")
	if len(parts) >= 2 {
		noun := parts[len(parts)-1]
		for _, adj := range adjectives {
			if len(suggestions) >= count {
				break
			}
			candidate := adj + "-" + noun
			if candidate != baseTag {
				if cfg.GetTunnelByTag(candidate) == nil {
					suggestions = append(suggestions, candidate)
				}
			}
		}
	}

	return suggestions
}

// GetServiceName returns the systemd service name for a tunnel.
func GetServiceName(tag string) string {
	return "dnstm-" + tag
}

// GenerateUniqueTunnelTag generates a unique tag that doesn't conflict with existing tunnels.
// This function takes a slice of tunnel configs directly.
func GenerateUniqueTunnelTag(tunnels []config.TunnelConfig) string {
	maxAttempts := 100
	existingTags := make(map[string]bool)
	for _, t := range tunnels {
		existingTags[t.Tag] = true
	}

	for i := 0; i < maxAttempts; i++ {
		tag := GenerateName()
		if !existingTags[tag] {
			return tag
		}
	}
	// Fallback: add a random suffix
	return GenerateName() + fmt.Sprintf("-%d", rand.IntN(1000))
}
