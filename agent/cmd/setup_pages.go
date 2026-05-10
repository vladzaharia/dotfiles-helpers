package cmd

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// robotLogo is the welcome ASCII art: a 6-row robot beside the
// "ag" wordmark (figlet "standard" font, 5 rows; backticks swapped for
// single quotes since Go raw strings can't contain backticks). The
// wordmark's descender (`|___/`) lands on robot row 5 so the visual
// baseline aligns; robot feet (`|_|`) extend one row below the
// wordmark. The pageModel frame centers the whole block, so absolute
// column padding here only matters relative to the two halves.
const robotLogo = `   ___       __ _  __ _
  [o.o]     / _' |/ _' |
  /|=|\    | (_| | (_| |
 d |_| b    \__,_|\__, |
   | |             |___/
   |_|`

// runWelcomePage shows the welcome screen and returns false if the
// user picks "Cancel".
func runWelcomePage(version, configPath, secretsPath string) (bool, error) {
	header := output.Style.Accent.Render(robotLogo)

	blurb := "ag dispatches between AI coding agents (Claude Code, Codex) and runs them in sandboxed OrbStack VMs by default. " +
		"This wizard configures agent defaults, isolation mode, OrbStack/LM Studio integration, and credentials.\n\n" +
		"You can re-run any section later with `agent-helper setup <section>`."

	meta := output.Style.Subtle.Render(strings.Join([]string{
		fmt.Sprintf("version    %s", version),
		fmt.Sprintf("config     %s", configPath),
		fmt.Sprintf("secrets    %s  (mode 0600)", secretsPath),
	}, "\n"))

	body := "\n" + header + "\n\n" +
		output.Style.Bold.Render("Welcome to agent-helper.") + "\n\n" +
		output.InfoBox(blurb) + "\n\n" + meta + "\n"

	choice := "begin"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("").Description(body),
			huh.NewSelect[string]().
				Title("").
				Options(
					huh.NewOption("Begin setup", "begin"),
					huh.NewOption("Cancel", "cancel"),
				).
				Value(&choice),
		),
	)
	if err := runForm(form); err != nil {
		return false, err
	}
	return choice == "begin", nil
}

// renderSummary builds a user-friendly diff between cfgBefore and cfg
// for the Apply page. Field names are translated to plain-English
// labels; unchanged fields are marked, changed fields show "(was X)".
func renderSummary(before, after Config, statuses []providerSummary, lmReachable, claudeReady bool) string {
	var b strings.Builder

	section := func(title string) {
		b.WriteString("\n")
		b.WriteString(output.SectionHeader(title, 56))
		b.WriteString("\n")
	}
	row := func(label, value, was string) {
		line := fmt.Sprintf("  %-12s %s", label, value)
		if was != "" {
			line += output.Style.Subtle.Render("  (was " + was + ")")
		}
		b.WriteString(line + "\n")
	}

	section("Defaults")
	row("Agent", agentLabel(after.Defaults.Agent), diffWas(before.Defaults.Agent, after.Defaults.Agent, agentLabel))
	row("Isolation", isolationLabel(after.Defaults.Isolated), diffWas(before.Defaults.Isolated, after.Defaults.Isolated, isolationLabel))

	section("Providers")
	for _, s := range statuses {
		row(s.label, s.summary, "")
	}

	if runtime.GOOS == "darwin" {
		section("VMs")
		if after.Defaults.Isolated == "none" {
			row("OrbStack", output.Style.Muted.Render("(not used — isolation=none)"), "")
		} else {
			row("OrbStack", "✓ Active", "")
			if len(after.Orb.DefaultPacks) > 0 {
				row("Packs", strings.Join(after.Orb.DefaultPacks, ", "), packsDiff(before.Orb.DefaultPacks, after.Orb.DefaultPacks))
			}
			if len(after.Orb.MountRoots) > 0 {
				row("Mounts", strings.Join(after.Orb.MountRoots, ", "), packsDiff(before.Orb.MountRoots, after.Orb.MountRoots))
			}
		}
	}

	section("Local models")
	if lmReachable {
		row("LM Studio", after.Local.URL, diffWas(before.Local.URL, after.Local.URL, identity))
		if after.Local.Models.Haiku != "" {
			row("  Haiku", after.Local.Models.Haiku, diffWas(before.Local.Models.Haiku, after.Local.Models.Haiku, identity))
		}
		if after.Local.Models.Sonnet != "" {
			row("  Sonnet", after.Local.Models.Sonnet, diffWas(before.Local.Models.Sonnet, after.Local.Models.Sonnet, identity))
		}
		if after.Local.Models.Opus != "" {
			row("  Opus", after.Local.Models.Opus, diffWas(before.Local.Models.Opus, after.Local.Models.Opus, identity))
		}
	} else {
		row("LM Studio", output.Style.Muted.Render("(not configured)"), "")
	}

	section("Credentials")
	if claudeReady {
		row("Claude", "✓ OAuth token saved for VM use", "")
	} else {
		row("Claude", output.Style.Muted.Render("(not set up — VMs may need manual auth)"), "")
	}

	b.WriteString("\n")
	b.WriteString(output.InfoBox(strings.Join([]string{
		"Re-run any section later with:",
		"  agent-helper setup defaults",
		"  agent-helper setup providers",
		"  agent-helper setup orb",
		"  agent-helper setup lmstudio",
		"  agent-helper setup claude",
	}, "\n")))

	return b.String()
}

// providerSummary is what runSetup hands to renderSummary so the
// summary can show per-provider status without re-detecting.
type providerSummary struct {
	label   string
	summary string
}

// runApplyPage shows the user-friendly summary + diff and returns the
// user's decision: "save", "back", or "cancel".
func runApplyPage(summary string) (string, error) {
	choice := "save"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(output.Style.Bold.Render("Here's what you've configured:")).
				Description(summary),
			huh.NewSelect[string]().
				Title("").
				Options(
					huh.NewOption("Save and finish", "save"),
					huh.NewOption("Back", "back"),
					huh.NewOption("Cancel without saving", "cancel"),
				).
				Value(&choice),
		),
	)
	if err := runForm(form); err != nil {
		return "", err
	}
	return choice, nil
}

// agentLabel translates a config agent value into a friendly label.
func agentLabel(s string) string {
	switch s {
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex CLI"
	}
	return s
}

// isolationLabel translates a config isolation value into a friendly,
// context-rich label.
func isolationLabel(s string) string {
	switch s {
	case "none":
		return "none · run on host"
	case "shared":
		return "shared · long-lived per-provider VM"
	case "full":
		return "full · dedicated ephemeral VM per task"
	}
	return s
}

// isolationDescription returns the longer-form explanation rendered in
// the focus-aware infobox below the isolation Select. Re-evaluated on
// each cursor move via huh's DescriptionFunc.
func isolationDescription(s string) string {
	switch s {
	case "none":
		return output.InfoBox(
			"none\n" +
				"Run on host. Full filesystem access. Fastest. Use only for trusted prompts.")
	case "shared":
		return output.InfoBox(
			"shared\n" +
				"Long-lived per-provider VM. Faster startup than full, but state persists across sessions.")
	case "full":
		return output.InfoBox(
			"full\n" +
				"Dedicated ephemeral VM per task (recommended). Strong isolation. ~10s startup. Auto-cleanup.")
	}
	return ""
}

// identity is a no-op label translator for fields that already render
// fine as their raw string (e.g., URLs, model IDs).
func identity(s string) string { return s }

// diffWas returns "(was X)"-style text iff before differs from after.
// Empty strings are normalized to "(unset)" so a fresh config still
// shows the user what got filled in.
func diffWas(before, after string, label func(string) string) string {
	if before == after {
		return ""
	}
	if before == "" {
		return "(unset)"
	}
	return label(before)
}

// packsDiff renders a "(was X)" hint for slice fields when they differ.
// Slice equality uses reflect.DeepEqual since the items are strings.
func packsDiff(before, after []string) string {
	if reflect.DeepEqual(before, after) {
		return ""
	}
	if len(before) == 0 {
		return "(unset)"
	}
	return strings.Join(before, ", ")
}

// configPath returns the user-facing config file path, mirroring
// internal/config without re-importing.
func configPathStr() string { return config.Path("agent-helper") }

// secretsPathStr is a friendly wrapper around the package-private
// secretsPath() so callers outside auth.go can reference it.
func secretsPathStr() string { return secretsPath() }

// snapshotConfig returns a deep-copy of cfg suitable for diffing later.
// Only string and slice fields need duplication; the rest are value
// types and copy by assignment.
func snapshotConfig(cfg Config) Config {
	out := cfg
	out.Orb.MountRoots = append([]string(nil), cfg.Orb.MountRoots...)
	out.Orb.DefaultPacks = append([]string(nil), cfg.Orb.DefaultPacks...)
	out.Sync.Excludes = append([]string(nil), cfg.Sync.Excludes...)
	if cfg.Providers != nil {
		out.Providers = make(map[string]ProviderConfig, len(cfg.Providers))
		for k, v := range cfg.Providers {
			v.ExtraArgs = append([]string(nil), v.ExtraArgs...)
			out.Providers[k] = v
		}
	}
	return out
}

// stderrIsTTY returns true if stderr is a terminal — used for color
// fallback decisions in non-interactive contexts.
func stderrIsTTY() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
