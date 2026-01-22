package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
)

var (
	titleColor   = color.New(color.FgCyan, color.Bold)
	successColor = color.New(color.FgGreen)
	errorColor   = color.New(color.FgRed)
	warnColor    = color.New(color.FgYellow)
	infoColor    = color.New(color.FgBlue)
	promptColor  = color.New(color.FgYellow)
	defaultColor = color.New(color.FgHiBlack)
	valueColor   = color.New(color.FgCyan)
	boxColor     = color.New(color.FgCyan)
	boxTitleColor = color.New(color.FgGreen, color.Bold)
)

type MenuOption struct {
	Key   string
	Label string
}

func PrintBanner(version string) {
	banner := `
    ____  _   _______  ________  ___
   / __ \/ | / / ___/ /_  __/  |/  /
  / / / /  |/ /\__ \   / / / /|_/ /
 / /_/ / /|  /___/ /  / / / /  / /
/_____/_/ |_//____/  /_/ /_/  /_/
`
	titleColor.Println(banner)
	fmt.Printf("DNS Tunnel Manager v%s\n\n", version)
}

func PrintTitle(title string) {
	fmt.Println()
	titleColor.Println("╔" + strings.Repeat("═", utf8.RuneCountInString(title)+2) + "╗")
	titleColor.Println("║ " + title + " ║")
	titleColor.Println("╚" + strings.Repeat("═", utf8.RuneCountInString(title)+2) + "╝")
	fmt.Println()
}

func PrintSuccess(msg string) {
	successColor.Println("✓ " + msg)
}

func PrintStatus(msg string) {
	successColor.Println("✓ " + msg)
}

func PrintError(msg string) {
	errorColor.Println("✗ " + msg)
}

func PrintWarning(msg string) {
	warnColor.Println("⚠ " + msg)
}

func PrintInfo(msg string) {
	infoColor.Println("ℹ " + msg)
}

func PrintStep(step int, total int, msg string) {
	fmt.Printf("[%d/%d] %s\n", step, total, msg)
}

func Prompt(label string) string {
	reader := bufio.NewReader(os.Stdin)
	promptColor.Printf("%s: ", label)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func PromptWithDefault(prompt, defaultVal string) string {
	reader := bufio.NewReader(os.Stdin)

	if defaultVal != "" {
		promptColor.Printf("%s ", prompt)
		defaultColor.Printf("[%s]", defaultVal)
		fmt.Print(": ")
	} else {
		promptColor.Printf("%s: ", prompt)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func PromptInt(prompt string, defaultVal, min, max int) int {
	reader := bufio.NewReader(os.Stdin)

	for {
		promptColor.Printf("%s ", prompt)
		defaultColor.Printf("(%d-%d) [%d]", min, max, defaultVal)
		fmt.Print(": ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			return defaultVal
		}

		val, err := strconv.Atoi(input)
		if err != nil || val < min || val > max {
			PrintError(fmt.Sprintf("Please enter a number between %d and %d", min, max))
			continue
		}

		return val
	}
}

func PromptChoice(prompt string, options []string, defaultVal string) string {
	reader := bufio.NewReader(os.Stdin)

	optStr := strings.Join(options, "/")
	promptColor.Printf("%s ", prompt)
	defaultColor.Printf("(%s) [%s]", optStr, defaultVal)
	fmt.Print(": ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}

	for _, opt := range options {
		if strings.EqualFold(input, opt) {
			return opt
		}
	}

	return defaultVal
}

func Confirm(prompt string, defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)

	defaultStr := "y/N"
	if defaultYes {
		defaultStr = "Y/n"
	}

	promptColor.Printf("%s ", prompt)
	defaultColor.Printf("[%s]", defaultStr)
	fmt.Print(": ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes
	}

	return input == "y" || input == "yes"
}

func ShowMenu(options []MenuOption) {
	fmt.Println()
	for _, opt := range options {
		fmt.Printf("  %s. %s\n", opt.Key, opt.Label)
	}
	fmt.Println()
}

func PrintBox(title string, lines []string) {
	maxLen := utf8.RuneCountInString(title)
	for _, line := range lines {
		if utf8.RuneCountInString(line) > maxLen {
			maxLen = utf8.RuneCountInString(line)
		}
	}

	border := strings.Repeat("═", maxLen+2)

	fmt.Println()
	boxColor.Printf("╔%s╗\n", border)
	boxColor.Print("║ ")
	boxTitleColor.Printf("%s", title)
	boxColor.Printf("%s ║\n", strings.Repeat(" ", maxLen-utf8.RuneCountInString(title)))
	boxColor.Printf("╠%s╣\n", border)

	for _, line := range lines {
		padding := maxLen - utf8.RuneCountInString(line)
		boxColor.Print("║ ")
		// Color the values differently
		if strings.Contains(line, ":") && !strings.HasPrefix(line, " ") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				fmt.Print(parts[0] + ":")
				valueColor.Print(parts[1])
			} else {
				fmt.Print(line)
			}
		} else {
			fmt.Print(line)
		}
		boxColor.Printf("%s ║\n", strings.Repeat(" ", padding))
	}

	boxColor.Printf("╚%s╝\n", border)
	fmt.Println()
}

func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

func ClearLine() {
	fmt.Print("\r\033[K")
}

func PrintProgress(downloaded, total int64) {
	if total <= 0 {
		return
	}

	percent := float64(downloaded) / float64(total) * 100
	barWidth := 40
	filled := int(float64(barWidth) * float64(downloaded) / float64(total))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	fmt.Printf("\r[%s] %.1f%%", bar, percent)

	if downloaded >= total {
		fmt.Println()
	}
}

func WaitForEnter() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nPress Enter to continue...")
	reader.ReadString('\n')
}
