package setup

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "enter", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	return tea.NewView(m.render())
}

func (m Model) render() string {
	var b strings.Builder
	b.WriteString(theme.Render(theme.B("past", theme.ToneText), theme.S(" › ", theme.ToneFaint), theme.B("setup", theme.ToneText)) + "\n")
	b.WriteString(theme.Muted("checking what past needs before it opens the store") + "\n\n")

	rule := lipgloss.NewStyle().Foreground(theme.P.RuleBold)
	for i := 0; i < m.shown; i++ {
		s := m.steps[i]
		connector := "├─ "
		if i == len(m.steps)-1 {
			connector = "└─ "
		}

		glyph, tone := "✗", theme.ToneDanger
		switch {
		case s.OK:
			glyph, tone = "✓", theme.ToneSuccess
		case s.Optional:
			glyph, tone = "○", theme.ToneFaint
		}

		name := theme.B(pad(s.Name), theme.ToneText)
		if s.OK {
			name = theme.S(pad(s.Name), theme.ToneBody)
		}
		line := rule.Render(connector) + theme.Render(theme.S(glyph+" ", tone), name)
		if !s.OK {
			line += "  " + theme.Muted(s.Hint)
		}
		b.WriteString(line + "\n")
	}

	if m.shown >= len(m.steps) {
		b.WriteString("\n" + theme.Muted("install what is marked ") + theme.Render(theme.S("✗", theme.ToneDanger)) + theme.Muted(", then run past again") + "\n\n")
		b.WriteString(theme.Key("q", "quit"))
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
