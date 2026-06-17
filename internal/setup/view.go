package setup

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kontrolplane/past/internal/theme"
)

// unfoldMsg advances the tree by one node.
type unfoldMsg struct{}

// Model renders the setup tree, revealing one branch per tick.
type Model struct {
	steps []Step
	shown int
}

func New(steps []Step) Model { return Model{steps: steps} }

func (m Model) Init() tea.Cmd { return unfold() }

// unfold schedules the next branch reveal.
func unfold() tea.Cmd {
	return tea.Tick(110*time.Millisecond, func(time.Time) tea.Msg { return unfoldMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case unfoldMsg:
		if m.shown < len(m.steps) {
			m.shown++
		}
		if m.shown < len(m.steps) {
			return m, unfold()
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "enter", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("past · setup") + "\n\n")

	for i := 0; i < m.shown; i++ {
		s := m.steps[i]
		connector := "├─ "
		if i == len(m.steps)-1 {
			connector = "└─ "
		}

		glyph, gstyle := "✗", theme.Error
		switch {
		case s.OK:
			glyph, gstyle = "✓", theme.Success
		case s.Optional:
			glyph, gstyle = "○", theme.Label
		}

		line := theme.Rule.Render(connector) +
			gstyle.Render(glyph) + " " +
			theme.Accent.Render(pad(s.Name))
		if !s.OK {
			line += "  " + theme.Label.Render(s.Hint)
		}
		b.WriteString(line + "\n")
	}

	if m.shown >= len(m.steps) {
		b.WriteString("\n")
		b.WriteString(theme.Label.Render("install what's marked ✗, then run past again") + "\n")
		b.WriteString(theme.Label.Render("q quit"))
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// pad left-aligns the tool name into a fixed column so hints line up.
func pad(s string) string {
	const w = 6
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
