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
// Two tiers of gray are intentional:
//   - Muted (245) is *readable secondary* prose — infobox bodies,
//     deliberate "(not configured)" placeholders, action affordances
//     like "[enter to install]". The user should still notice it.
//   - Subtle (240) is *background hints* the eye should slide past —
//     help-key footers, "(was X)" diff annotations, version metadata
//     on the welcome page.
//
// If a row carries status (success / warning / error) it should use
// the matching colored style, not Muted/Subtle. Demoting warnings to
// gray was the main reason the CLI used to read as flat.
var (
	colorAccent  = lipgloss.Color("212") // charm pink/purple — accent / brand / focused selection / spinner
	colorMuted   = lipgloss.Color("245") // light gray — readable secondary prose
	colorSubtle  = lipgloss.Color("240") // medium gray — background hints (help keys, diff annotations)
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
	Heading lipgloss.Style
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
	Heading: lipgloss.NewStyle().Foreground(colorAccent).Bold(true),
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

// SectionHeader returns "── Title ──" with the title in accent + bold
// (Style.Heading) and the dashes in plain bold; used to delimit sections
// within multi-section forms and to head doctor/status output.
func SectionHeader(title string, width int) string {
	if width <= 0 {
		width = 56
	}
	lead := "── "
	gap := " "
	rest := width - lipgloss.Width(lead+title+gap)
	if rest < 3 {
		rest = 3
	}
	return Style.Bold.Render(lead) + Style.Heading.Render(title) + Style.Bold.Render(gap+strings.Repeat("─", rest))
}
