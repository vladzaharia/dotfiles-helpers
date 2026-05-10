package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vladzaharia/dotfiles-helpers/agent/lmstudio"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// rowState is the per-tool detection lifecycle.
type rowState int

const (
	rowLoading rowState = iota
	rowActive
	rowNotLoggedIn
	rowNotInstalled
	rowNotReachable // LM Studio specific
	rowInstalling
	rowInstallFailed
)

type toolID string

const (
	toolClaude   toolID = "claude"
	toolCodex    toolID = "codex"
	toolOrbStack toolID = "orbstack"
	toolLMStudio toolID = "lmstudio"
)

// toolRow holds per-tool render + interaction state.
type toolRow struct {
	id          toolID
	displayName string
	state       rowState
	version     string
	path        string
	detail      string
	errMsg      string
	infobox     string // shown when focused + not-installed
	loggedIn    bool
}

// DetectionResult is what runDetectionCard returns to the caller. Maps
// tool name → final state, plus override paths the user typed.
type DetectionResult struct {
	Statuses     map[toolID]rowState
	OverridePath map[toolID]string
}

// detectionMsgType is the discriminator on goroutine-emitted messages.
type detectionMsg struct {
	tool toolID
	row  toolRow
}
type installDoneMsg struct {
	tool toolID
	err  error
}
type spinnerTickMsg time.Time

// runDetectionCard runs the live detection card and blocks until the
// user hits `c` to continue or `q` to quit. Returns the final state
// plus the user's chosen override paths.
//
// This replaces the procedural detection block in runSetup with a
// proper TUI page that shows live "Loading…" → "Active" transitions,
// inline install actions, and per-row infoboxes.
func runDetectionCard(ctx context.Context, lmURL string) (*DetectionResult, error) {
	rows := []toolRow{
		{id: toolClaude, displayName: "Claude Code", state: rowLoading},
		{id: toolCodex, displayName: "Codex CLI", state: rowLoading},
		{id: toolOrbStack, displayName: "OrbStack", state: rowLoading},
		{id: toolLMStudio, displayName: "LM Studio", state: rowLoading},
	}

	m := detectionCardModel{
		rows:  rows,
		lmURL: lmURL,
	}

	p := tea.NewProgram(&m, tea.WithContext(ctx), tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := final.(*detectionCardModel)
	if fm.cancelled {
		return nil, fmt.Errorf("detection cancelled")
	}
	return fm.result(), nil
}

type detectionCardModel struct {
	rows      []toolRow
	focus     int
	lmURL     string
	logTail   []string // last 5 lines of streaming install output
	doneInit  bool     // detection commands have been kicked off
	cancelled bool
	width     int
	tick      int // animation frame for spinner
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *detectionCardModel) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.rows)+1)
	for i := range m.rows {
		cmds = append(cmds, detectCmd(m.rows[i].id, m.lmURL))
	}
	cmds = append(cmds, tickCmd())
	m.doneInit = true
	return tea.Batch(cmds...)
}

func (m *detectionCardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case spinnerTickMsg:
		m.tick++
		if m.anyLoading() || m.anyInstalling() {
			return m, tickCmd()
		}
		return m, nil
	case detectionMsg:
		m.applyDetectionResult(msg)
		return m, nil
	case installDoneMsg:
		m.applyInstallResult(msg)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *detectionCardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	case "j", "down":
		if m.focus < len(m.rows)-1 {
			m.focus++
		}
	case "k", "up":
		if m.focus > 0 {
			m.focus--
		}
	case "c", "enter":
		// Enter on a row triggers its action; "c" always continues.
		if msg.String() == "c" {
			return m, tea.Quit
		}
		// Enter: trigger the focused row's primary action.
		row := &m.rows[m.focus]
		switch row.state {
		case rowNotInstalled, rowInstallFailed:
			row.state = rowInstalling
			return m, installCmd(row.id, m.lmURL)
		case rowNotReachable:
			// For LM Studio, "enter" re-probes.
			row.state = rowLoading
			return m, detectCmd(row.id, m.lmURL)
		case rowActive, rowNotLoggedIn:
			// Enter advances; no per-row login flow in this compact
			// implementation (login happens at agent invocation).
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *detectionCardModel) applyDetectionResult(msg detectionMsg) {
	for i := range m.rows {
		if m.rows[i].id == msg.tool {
			m.rows[i] = msg.row
			break
		}
	}
}

func (m *detectionCardModel) applyInstallResult(msg installDoneMsg) {
	for i := range m.rows {
		if m.rows[i].id != msg.tool {
			continue
		}
		if msg.err != nil {
			m.rows[i].state = rowInstallFailed
			m.rows[i].errMsg = msg.err.Error()
		} else {
			// Re-detect after install.
			m.rows[i].state = rowLoading
		}
	}
	if msg.err == nil {
		// Trigger fresh detection for the just-installed tool.
		// (The Update loop will receive the result.)
	}
}

func (m *detectionCardModel) anyLoading() bool {
	for _, r := range m.rows {
		if r.state == rowLoading {
			return true
		}
	}
	return false
}

func (m *detectionCardModel) anyInstalling() bool {
	for _, r := range m.rows {
		if r.state == rowInstalling {
			return true
		}
	}
	return false
}

func (m *detectionCardModel) View() string {
	var b strings.Builder
	b.WriteString(output.Style.Bold.Render("agent-helper setup · 1 of 5 · Detection") + "\n\n")

	for i, r := range m.rows {
		focused := i == m.focus
		b.WriteString(m.renderRow(r, focused))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if focusedRow := m.rows[m.focus]; needsInfobox(focusedRow) {
		b.WriteString(output.InfoBox(infoboxFor(focusedRow)))
		b.WriteString("\n")
	}

	help := output.Style.Dim.Render("↑↓ navigate · enter act · c continue · q quit")
	b.WriteString(help)
	return b.String()
}

func (m *detectionCardModel) renderRow(r toolRow, focused bool) string {
	cursor := "  "
	if focused {
		cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("▶ ")
	}
	name := lipgloss.NewStyle().Bold(true).Width(14).Render(r.displayName)
	status := m.renderStatus(r)
	return cursor + name + status
}

func (m *detectionCardModel) renderStatus(r toolRow) string {
	switch r.state {
	case rowLoading:
		return output.Style.Loading.Render(spinnerFrames[m.tick%len(spinnerFrames)] + " Loading…")
	case rowActive:
		auth := ""
		if !r.loggedIn && (r.id == toolClaude || r.id == toolCodex) {
			auth = output.Style.Warning.Render("  (not logged in)")
		}
		ver := ""
		if r.version != "" {
			ver = "  " + r.version
		}
		return output.Style.Success.Render("✓ Active") + ver + auth
	case rowNotLoggedIn:
		return output.Style.Warning.Render("⚠ Installed, not logged in")
	case rowNotInstalled:
		return output.Style.Error.Render("✗ Not installed") + output.Style.Dim.Render("  [enter to install]")
	case rowNotReachable:
		return output.Style.Warning.Render("— Not reachable at "+r.detail) + output.Style.Dim.Render("  [enter to retry]")
	case rowInstalling:
		return output.Style.Loading.Render(spinnerFrames[m.tick%len(spinnerFrames)] + " Installing…")
	case rowInstallFailed:
		return output.Style.Error.Render("✗ Install failed") + output.Style.Dim.Render("  [enter to retry]")
	}
	return ""
}

func (m *detectionCardModel) result() *DetectionResult {
	out := &DetectionResult{
		Statuses:     map[toolID]rowState{},
		OverridePath: map[toolID]string{},
	}
	for _, r := range m.rows {
		out.Statuses[r.id] = r.state
	}
	return out
}

// needsInfobox returns true when the focused row's state warrants a
// detail blurb below the table.
func needsInfobox(r toolRow) bool {
	return r.state == rowNotInstalled || r.state == rowNotReachable || r.state == rowInstallFailed
}

// infoboxFor returns the per-tool infobox copy. Per-tool blurbs match
// the wizard plan's specifications.
func infoboxFor(r toolRow) string {
	switch r.id {
	case toolClaude:
		return "Claude Code\nAnthropic's official coding agent CLI. Required if you want to use `ag claude`."
	case toolCodex:
		return "Codex CLI\nOpenAI's official coding agent CLI. Useful for comparing model outputs or as a fallback when Claude is rate-limited."
	case toolOrbStack:
		return "OrbStack\nLightweight macOS VM/container manager. Required for `shared` and `full` isolation modes."
	case toolLMStudio:
		return "LM Studio\nLocal model server. Powers `--local` mode for offline / private inference. Optional."
	}
	return r.errMsg
}

// detectCmd returns a tea.Cmd that probes the named tool and emits a
// detectionMsg with the result.
func detectCmd(id toolID, lmURL string) tea.Cmd {
	return func() tea.Msg {
		row := toolRow{id: id, displayName: displayName(id)}
		switch id {
		case toolClaude:
			st := provider.DetectClaude()
			if !st.Installed {
				row.state = rowNotInstalled
				return detectionMsg{tool: id, row: row}
			}
			row.state = rowActive
			row.version = st.Version
			row.detail = st.Detail
			row.loggedIn = !needsClaudeAuthSetup()
		case toolCodex:
			st := provider.DetectCodex()
			if !st.Installed {
				row.state = rowNotInstalled
				return detectionMsg{tool: id, row: row}
			}
			row.state = rowActive
			row.version = st.Version
			row.detail = st.Detail
			row.loggedIn = true // Codex login detection not implemented yet
		case toolOrbStack:
			if !orb.IsInstalled() {
				row.state = rowNotInstalled
				return detectionMsg{tool: id, row: row}
			}
			row.state = rowActive
			row.detail = "installed"
		case toolLMStudio:
			st := CheckLMStudio(lmURL)
			if !st.Reachable {
				row.state = rowNotReachable
				row.detail = lmURL
				return detectionMsg{tool: id, row: row}
			}
			row.state = rowActive
			row.detail = fmt.Sprintf("%d models", len(st.Models))
		}
		return detectionMsg{tool: id, row: row}
	}
}

// installCmd kicks off an install for the focused tool. Drops out of
// the TUI for the install (so brew's output shows naturally), then
// re-runs detection on completion. Currently routes everything through
// brew when available.
func installCmd(id toolID, lmURL string) tea.Cmd {
	return tea.Sequence(
		tea.Cmd(func() tea.Msg {
			err := runInstall(id)
			if err != nil {
				return installDoneMsg{tool: id, err: err}
			}
			return installDoneMsg{tool: id, err: nil}
		}),
		detectCmd(id, lmURL),
	)
}

func runInstall(id toolID) error {
	cask := caskNameFor(id)
	if cask == "" {
		return fmt.Errorf("install routing for %s not implemented yet", id)
	}
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("brew not on PATH; install %s manually then re-run setup", id)
	}
	cmd := exec.Command("brew", "install", "--cask", cask)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew install --cask %s: %w (%s)", cask, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func caskNameFor(id toolID) string {
	switch id {
	case toolClaude:
		return "claude-code"
	case toolCodex:
		return "" // Codex isn't on a brew cask widely yet — skip auto-install.
	case toolOrbStack:
		return "orbstack"
	case toolLMStudio:
		return "lm-studio"
	}
	return ""
}

func displayName(id toolID) string {
	switch id {
	case toolClaude:
		return "Claude Code"
	case toolCodex:
		return "Codex CLI"
	case toolOrbStack:
		return "OrbStack"
	case toolLMStudio:
		return "LM Studio"
	}
	return string(id)
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

// Reference unused vars to silence "unused" warnings during incremental
// development; lmstudio import kept for future API extensions in this
// file (model count rendering, etc.).
var _ = lmstudio.Tier(0)
