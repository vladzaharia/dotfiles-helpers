// Package output centralizes terminal styling. Every color in the
// wizard, dispatch warnings, and doctor output flows through the Style
// palette below so the whole CLI feels consistent.
package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette. Numeric codes are 256-color palette indices that look
// reasonable on both dark and light backgrounds; lipgloss/termenv falls
// back to bold/underline distinctions when NO_COLOR is set.
//
// Roles are intentionally separated: a "secondary" string can be muted
// (descriptive prose), subtle (help keys, diff hints), or a border —
// these used to all collapse into one gray and the wizard read as flat.
var (
	colorAccent  = lipgloss.Color("212") // charm pink/purple — accent / focused selection / spinner
	colorMuted   = lipgloss.Color("245") // light gray — descriptive prose, infobox body
	colorSubtle  = lipgloss.Color("240") // medium gray — help keys, "(was X)" diff hints
	colorBorder  = lipgloss.Color("99")  // soft indigo — infobox / panel borders
	colorSuccess = lipgloss.Color("42")  // bright green — ✓ markers
	colorWarning = lipgloss.Color("214") // amber — ⚠ markers
	colorError   = lipgloss.Color("196") // red — ✗ markers
)

// Style exposes the named styles used across the CLI.
var Style = struct {
	Accent  lipgloss.Style
	Muted   lipgloss.Style
	Subtle  lipgloss.Style
	Border  lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Loading lipgloss.Style
	Bold    lipgloss.Style
	InfoBox lipgloss.Style
}{
	Accent:  lipgloss.NewStyle().Foreground(colorAccent),
	Muted:   lipgloss.NewStyle().Foreground(colorMuted),
	Subtle:  lipgloss.NewStyle().Foreground(colorSubtle),
	Border:  lipgloss.NewStyle().Foreground(colorBorder),
	Success: lipgloss.NewStyle().Foreground(colorSuccess),
	Warning: lipgloss.NewStyle().Foreground(colorWarning),
	Error:   lipgloss.NewStyle().Foreground(colorError),
	Loading: lipgloss.NewStyle().Foreground(colorAccent),
	Bold:    lipgloss.NewStyle().Bold(true),
	InfoBox: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Foreground(colorMuted).
		Padding(0, 1),
}

var (
	infoIcon    = Style.Accent.Render("[i]")
	successIcon = Style.Success.Render("[✓]")
	warnIcon    = Style.Warning.Render("[!]")
	errorIcon   = Style.Error.Render("[!]")
	statusOK    = Style.Success.Render("✓")
	statusFail  = Style.Error.Render("✗")
	statusWarn  = Style.Warning.Render("⚠")
	statusNone  = Style.Subtle.Render("·")
)

// Info / Success / Warn / Error print one styled line to stderr.
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

// StatusOK / StatusFail / StatusNone / StatusWarn return styled status
// rows for inclusion in detection summaries.
func StatusOK(label, detail string) string {
	return fmt.Sprintf("  %s %s  %s", statusOK, Style.Bold.Render(label), detail)
}

func StatusFail(label, detail string) string {
	return fmt.Sprintf("  %s %s  %s", statusFail, Style.Bold.Render(label), detail)
}

func StatusWarn(label, detail string) string {
	return fmt.Sprintf("  %s %s  %s", statusWarn, Style.Bold.Render(label), detail)
}

func StatusNone(label, detail string) string {
	return fmt.Sprintf("  %s %s  %s", statusNone, Style.Bold.Render(label), detail)
}

// InfoBox renders explanatory prose with a rounded border + muted
// foreground at a sensible default width (60 cols of inner content).
// Used by both huh forms (via custom theme Description) and custom
// Bubble Tea pages.
func InfoBox(s string) string {
	return InfoBoxW(60, s)
}

// InfoBoxW is InfoBox with an explicit max content width, so long prose
// wraps instead of bleeding past the right edge of the terminal. width
// is the inside-the-border content width.
func InfoBoxW(width int, s string) string {
	if width <= 0 {
		return Style.InfoBox.Render(s)
	}
	return Style.InfoBox.Width(width).Render(s)
}

// SectionHeader returns "── Title ──" styled bold; used to delimit
// sections within multi-section forms.
func SectionHeader(title string, width int) string {
	if width <= 0 {
		width = 56
	}
	pre := "── " + title + " "
	rest := width - lipgloss.Width(pre)
	if rest < 3 {
		rest = 3
	}
	return Style.Bold.Render(pre + strings.Repeat("─", rest))
}
