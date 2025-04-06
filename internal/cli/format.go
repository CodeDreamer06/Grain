package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"grain/internal/data"
)

const separator = "────────────────────────────"

// FormatHeader creates a standard header with an emoji and title.
func FormatHeader(title string) string {
	return fmt.Sprintf("%s\n%s", title, separator)
}

// FormatLogEntry formats a single log entry for display.
func FormatLogEntry(log data.Log) string {
	sign := "+"
	if log.Type == data.LogTypeBreak {
		sign = "-"
	}
	return fmt.Sprintf("[%s] %s%d %s", log.Timestamp.Format("15:04"), sign, log.Amount, log.Type)
}

// FormatDuration formats a duration in a human-readable way (e.g., 1h 30m).
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute

	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	} else if h > 0 {
		return fmt.Sprintf("%dh", h)
	} else {
		return fmt.Sprintf("%dm", m)
	}
}

// PrintError prints an error message to stderr.
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

// PromptConfirmation asks the user for confirmation.
func PromptConfirmation(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s ", prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "yes" || input == "y" || input == "reset grain"
}
