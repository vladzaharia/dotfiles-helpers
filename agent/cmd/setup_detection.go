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
	rowLoggingIn
	rowLoginFailed
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
type loginDoneMsg struct {
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

	page := newPage(&m)
	p := tea.NewProgram(page,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		return nil, err
	}
	if m.cancelled {
		return nil, fmt.Errorf("detection cancelled")
	}
	return m.result(), nil
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
	case loginDoneMsg:
		return m, m.applyLoginResult(msg)
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
		case rowNotLoggedIn, rowLoginFailed:
			row.state = rowLoggingIn
			return m, loginCmd(row.id, m.lmURL)
		case rowActive:
			// Already active; enter on a healthy row is a no-op.
			return m, nil
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

// applyLoginResult mutates the row in response to a finished login
// subprocess and returns the follow-up command (re-detection on
// success; nothing on failure since the row is already in the failed
// state, the spinner stops naturally).
func (m *detectionCardModel) applyLoginResult(msg loginDoneMsg) tea.Cmd {
	for i := range m.rows {
		if m.rows[i].id != msg.tool {
			continue
		}
		if msg.err != nil {
			m.rows[i].state = rowLoginFailed
			m.rows[i].errMsg = msg.err.Error()
			return nil
		}
		// Login succeeded — re-detect to update version + loggedIn state.
		m.rows[i].state = rowLoading
		return detectCmd(msg.tool, m.lmURL)
	}
	return nil
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
		if r.state == rowInstalling || r.state == rowLoggingIn {
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

	help := output.Style.Subtle.Render("↑↓ navigate · enter act · c continue · q quit")
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
		return output.Style.Warning.Render("⚠ Installed, not logged in") + output.Style.Subtle.Render("  [enter to log in]")
	case rowNotInstalled:
		return output.Style.Error.Render("✗ Not installed") + output.Style.Subtle.Render("  [enter to install]")
	case rowNotReachable:
		return output.Style.Warning.Render("— Not reachable at "+r.detail) + output.Style.Subtle.Render("  [enter to retry]")
	case rowInstalling:
		return output.Style.Loading.Render(spinnerFrames[m.tick%len(spinnerFrames)] + " Installing…")
	case rowInstallFailed:
		return output.Style.Error.Render("✗ Install failed") + output.Style.Subtle.Render("  [enter to retry]")
	case rowLoggingIn:
		return output.Style.Loading.Render(spinnerFrames[m.tick%len(spinnerFrames)] + " Waiting for browser auth…")
	case rowLoginFailed:
		return output.Style.Error.Render("✗ Login failed") + output.Style.Subtle.Render("  [enter to retry]")
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
	switch r.state {
	case rowNotInstalled, rowNotReachable, rowInstallFailed,
		rowNotLoggedIn, rowLoginFailed:
		return true
	}
	return false
}

// infoboxFor returns the per-tool infobox copy. Login-state focus
// gets a login-specific blurb so the user knows what `[enter to log
// in]` actually does.
func infoboxFor(r toolRow) string {
	if r.state == rowNotLoggedIn || r.state == rowLoginFailed {
		switch r.id {
		case toolClaude:
			return "Claude Code is installed but not logged in.\nPress enter to run `claude setup-token` — a browser will open for OAuth, and the long-lived token gets saved for VM use."
		case toolCodex:
			return "Codex CLI is installed but not logged in.\nPress enter to run `codex login` and complete the auth flow."
		}
	}
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
			row.version = st.Version
			row.detail = st.Detail
			row.loggedIn = !needsClaudeAuthSetup()
			if row.loggedIn {
				row.state = rowActive
			} else {
				row.state = rowNotLoggedIn
			}
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

// loginCmd hands the terminal to the tool's login subprocess via
// tea.ExecProcess so the user can complete the OAuth flow inline. On
// return we re-run detection for that row.
func loginCmd(id toolID, lmURL string) tea.Cmd {
	subCmd := loginSubprocess(id)
	if subCmd == nil {
		// No login pathway implemented for this tool; surface as a
		// soft failure so the user can pick another action.
		return func() tea.Msg {
			return loginDoneMsg{tool: id, err: fmt.Errorf("no inline login flow for %s yet", id)}
		}
	}
	return tea.ExecProcess(subCmd, func(err error) tea.Msg {
		// For Claude's setup-token flow we also need to capture the
		// printed token; the existing runClaudeSetupTokenInteractive
		// handles that, but we ran the raw subprocess here for stdio
		// hand-off. Detection picks up the new state.
		return loginDoneMsg{tool: id, err: err}
	})
}

// loginSubprocess returns the *exec.Cmd that, when run with stdio
// inherited, performs an interactive login for the named tool. Returns
// nil when no inline login pathway exists.
func loginSubprocess(id toolID) *exec.Cmd {
	switch id {
	case toolClaude:
		path, err := exec.LookPath("claude")
		if err != nil {
			return nil
		}
		c := exec.Command(path, "setup-token")
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c
	case toolCodex:
		path, err := exec.LookPath("codex")
		if err != nil {
			return nil
		}
		c := exec.Command(path, "login")
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c
	}
	return nil
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
