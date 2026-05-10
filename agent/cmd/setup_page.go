package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// pageModel wraps any tea.Model child so every wizard step renders
// centered in the alternate screen buffer with consistent breathing
// room. Centralizing the frame here means the welcome page, detection
// card, and every huh form share the same layout — no per-screen sizing
// code.
type pageModel struct {
	child         tea.Model
	width, height int
}

func newPage(child tea.Model) *pageModel {
	return &pageModel{child: child}
}

func (m *pageModel) Init() tea.Cmd {
	return m.child.Init()
}

func (m *pageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = ws.Width, ws.Height
	}

	var cmd tea.Cmd
	m.child, cmd = m.child.Update(msg)

	// huh.Form doesn't emit tea.Quit on its own when used as a child
	// model — it just transitions State. Watch for that and quit so
	// runForm returns.
	if f, ok := m.child.(*huh.Form); ok {
		if f.State == huh.StateCompleted || f.State == huh.StateAborted {
			return m, tea.Quit
		}
	}

	return m, cmd
}

func (m *pageModel) View() string {
	view := m.child.View()
	if m.width == 0 || m.height == 0 {
		return view
	}
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center, view)
}
