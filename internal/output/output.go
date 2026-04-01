package output

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var (
	infoIcon    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("[i]")
	successIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("[✓]")
	warnIcon    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[!]")
	errorIcon   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("[!]")
	statusOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("✓")
	statusFail  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗")
	statusNone  = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render("·")
	bold        = lipgloss.NewStyle().Bold(true)
)

func Info(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", infoIcon, fmt.Sprintf(format, a...))
}

func Success(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", successIcon, fmt.Sprintf(format, a...))
}

func Warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", warnIcon, fmt.Sprintf(format, a...))
}

func Error(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", errorIcon, fmt.Sprintf(format, a...))
}

func StatusOK(label, detail string) string {
	return fmt.Sprintf("  %s %s  %s", statusOK, bold.Render(label), detail)
}

func StatusFail(label, detail string) string {
	return fmt.Sprintf("  %s %s  %s", statusFail, bold.Render(label), detail)
}

func StatusNone(label, detail string) string {
	return fmt.Sprintf("  %s %s  %s", statusNone, bold.Render(label), detail)
}
