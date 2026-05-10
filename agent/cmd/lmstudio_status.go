package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/vladzaharia/dotfiles-helpers/agent/lmstudio"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// LMStudioState is a snapshot of LM Studio's reachability and inventory
// at a point in time. Used by both the wizard and the --local pre-flight.
type LMStudioState struct {
	URL          string
	Reachable    bool
	Models       []lmstudio.Model
	LoadedModels []lmstudio.Model
	LMSAvailable bool   // `lms` CLI on PATH
	Error        string // human-readable cause when !Reachable
}

// CheckLMStudio probes the LM Studio REST endpoint and detects whether
// the `lms` CLI is on PATH (used to surface auto-troubleshooting
// affordances when the server isn't responding).
func CheckLMStudio(url string) LMStudioState {
	st := LMStudioState{URL: url}
	if _, err := exec.LookPath("lms"); err == nil {
		st.LMSAvailable = true
	}
	client := &lmstudio.Client{URL: url}
	models, err := client.Models()
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.Reachable = true
	st.Models = models
	for _, m := range models {
		if m.IsLoaded() && m.IsLocal() {
			st.LoadedModels = append(st.LoadedModels, m)
		}
	}
	return st
}

// RenderLMStudioPanel builds a styled multi-line status panel for the
// given LM Studio state. Reused by both the wizard's Page 4 LM Studio
// section and the --local pre-flight dialog.
func RenderLMStudioPanel(st LMStudioState) string {
	var b strings.Builder
	b.WriteString(output.SectionHeader("LM Studio", 56))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  URL  %s\n", st.URL))

	if st.Reachable {
		b.WriteString(output.Style.Success.Render(fmt.Sprintf("  ✓ %d models available · %d loaded\n",
			len(st.Models), len(st.LoadedModels))))
		return b.String()
	}

	b.WriteString(output.Style.Error.Render("  ✗ Could not reach LM Studio\n"))
	if st.Error != "" {
		b.WriteString(output.Style.Muted.Render("    " + truncErr(st.Error) + "\n"))
	}
	b.WriteString("\n")
	b.WriteString(output.InfoBox(troubleshootingTextFor(st)))
	return b.String()
}

// truncErr keeps the error to one line for compact rendering. Common
// Go HTTP error strings are long; first sentence usually says enough.
func truncErr(s string) string {
	if i := strings.IndexAny(s, "\n;"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 96 {
		s = s[:93] + "..."
	}
	return s
}

// troubleshootingTextFor returns context-appropriate help based on
// what's available on the host (lms CLI, etc.).
func troubleshootingTextFor(st LMStudioState) string {
	if st.LMSAvailable {
		return strings.Join([]string{
			"To get started:",
			"  1. lms server start         (start the local API server)",
			"  2. lms load <model-id>      (load a model into memory)",
			"  3. lms ls                   (list available models)",
			"",
			"Or use the LM Studio app's GUI: open it, switch to Developer mode, and start the server.",
		}, "\n")
	}
	return strings.Join([]string{
		"LM Studio doesn't appear to be installed. Two options:",
		"  • Install from https://lmstudio.ai (recommended).",
		"  • Install the lms CLI separately: https://lmstudio.ai/docs/cli",
		"",
		"After installing, open LM Studio and start the local server",
		"(or run `lms server start`), then re-run with --local.",
	}, "\n")
}

// RunLMStudioPreflight is invoked at the start of every --local dispatch.
// When LM Studio is reachable, it's a no-op (returns immediately so the
// happy path stays fast). When unreachable, it surfaces the status
// panel + interactive recovery options:
//
//   - "lms server start" — runs `lms server start` for the user (only
//     offered when the lms CLI is on PATH)
//   - "Retry"            — re-probes with a fresh CheckLMStudio call
//   - "Continue anyway"  — proceeds to dispatch (the agent itself will
//     surface a connection error to the user)
//   - "Cancel"           — aborts dispatch with a clean exit
//
// On first invocation per machine (tracked via a marker file), the
// dialog also explains that --local always runs on the host with no
// isolation — important context users need exactly once.
func RunLMStudioPreflight(ctx context.Context, url string) error {
	st := CheckLMStudio(url)
	firstRun := !localFirstRunMarkerExists()

	// Happy path: reachable AND not first run → no UI.
	if st.Reachable && !firstRun {
		return nil
	}

	// First-run explainer (shown once even when LM Studio is healthy).
	if firstRun {
		if err := showLocalFirstRunDialog(st); err != nil {
			return err
		}
		_ = touchLocalFirstRunMarker()
		if st.Reachable {
			return nil
		}
	}

	// LM Studio unreachable → recovery loop.
	for {
		options := []huh.Option[string]{}
		if st.LMSAvailable {
			options = append(options, huh.NewOption("Start LM Studio server (lms server start)", "start"))
		}
		options = append(options,
			huh.NewOption("Retry connection", "retry"),
			huh.NewOption("Continue anyway", "continue"),
			huh.NewOption("Cancel", "cancel"),
		)

		choice := options[0].Value
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title(output.Style.Bold.Render("LM Studio is required for --local mode")).
					Description(RenderLMStudioPanel(st)),
				huh.NewSelect[string]().
					Title("How would you like to proceed?").
					Options(options...).
					Value(&choice),
			),
		)
		if err := runForm(form); err != nil {
			return err
		}
		switch choice {
		case "start":
			if err := startLMSServer(ctx); err != nil {
				output.Warn("lms server start: %v", err)
			}
			// Wait briefly for the server to come up, then re-probe.
			time.Sleep(2 * time.Second)
			st = CheckLMStudio(url)
			if st.Reachable {
				return nil
			}
		case "retry":
			st = CheckLMStudio(url)
			if st.Reachable {
				return nil
			}
		case "continue":
			return nil
		case "cancel":
			return errors.New("--local cancelled (LM Studio unreachable)")
		}
	}
}

// startLMSServer runs `lms server start` with a spinner. The command
// returns once the server is listening (lms blocks on its health probe
// internally), so we don't have to add our own wait here beyond the 2s
// settle in the caller.
func startLMSServer(ctx context.Context) error {
	return spinner.New().
		Title("Starting LM Studio server (lms server start)…").
		ActionWithErr(func(c context.Context) error {
			cmd := exec.CommandContext(c, "lms", "server", "start")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
			}
			return nil
		}).Run()
}

// showLocalFirstRunDialog explains, exactly once per machine, that
// --local always runs on the host with no isolation, and surfaces the
// LM Studio status so the user knows whether the next dispatch will
// actually work.
func showLocalFirstRunDialog(st LMStudioState) error {
	body := output.InfoBox(strings.Join([]string{
		"--local routes Claude Code at LM Studio for offline / private",
		"inference. Two things to know:",
		"",
		"  • --local always runs on the host (no VM isolation, no",
		"    OrbStack involvement). The agent has the same filesystem",
		"    access your shell does.",
		"  • LM Studio must be running locally with at least one model",
		"    loaded so the agent can actually serve completions.",
	}, "\n")) + "\n\n" + RenderLMStudioPanel(st)

	var ack string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(output.Style.Bold.Render("Welcome to --local mode")).
				Description(body),
			huh.NewSelect[string]().
				Title("").
				Options(huh.NewOption("Got it, continue", "ok")).
				Value(&ack),
		),
	)
	return runForm(form)
}

// localFirstRunMarker is a zero-byte sentinel that records "the user
// has seen the --local explainer at least once on this machine".
func localFirstRunMarker() string {
	return filepath.Join(config.Dir("agent-helper"), ".local-first-run")
}

func localFirstRunMarkerExists() bool {
	_, err := os.Stat(localFirstRunMarker())
	return err == nil
}

func touchLocalFirstRunMarker() error {
	dir := config.Dir("agent-helper")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(localFirstRunMarker(), nil, 0o644)
}
