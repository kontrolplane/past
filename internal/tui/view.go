package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kontrolplane/past/internal/theme"
)

// Outer padding around the whole view, matching the kontrolplane terminal look.
const (
	padX = 2
	padY = 1
)

var appStyle = lipgloss.NewStyle().Padding(padY, padX, padY, padX)

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}

	switch m.mode {
	case modeCreate:
		title := "new entry"
		if !m.genPicking {
			title = m.preset().name
		}
		return m.overlay(title, m.createView())
	case modeConfirmDelete:
		return m.overlay("delete", m.confirmView())
	case modeGithub:
		title := "github"
		switch m.ghView {
		case ghAlerts:
			title = "secret-scanning alerts"
		case ghRuns:
			title = "actions runs"
		}
		return m.overlay(title, m.githubView())
	case modeUnlock:
		return m.overlay("unlock", m.unlockView())
	case modePrompt:
		title := "rename / move"
		if m.promptCopy {
			title = "duplicate"
		}
		return m.overlay(title, m.promptView())
	case modeCommit:
		return m.overlay("commit", m.commitView())
	case modeCopy:
		return m.overlay("copy "+m.current, m.copyView())
	}

	// don't render the store until the startup auth probe resolves, so the tree
	// never flashes before the unlock prompt.
	if !m.ready {
		return m.overlay("past", theme.Label.Render("checking key…"))
	}

	if m.showHelp {
		return m.overlay("help", helpView(m.width-2*padX, m.height-2*padY))
	}

	// dnsquire-style screen: a navy title badge with a dim status subtitle, the
	// tree (which inlines the open entry's detail), then a dim footer.
	head := theme.Badge.Render("past") + "  " + m.headerStatus()
	return appStyle.Render(head + "\n\n" + m.tree.view() + "\n\n" + m.footer())
}

// overlay renders a modal as the same padded screen: a navy title badge, a
// blank line, then the modal content (which carries its own key hints).
func (m Model) overlay(title, content string) string {
	return appStyle.Render(theme.Badge.Render(title) + "\n\n" + content)
}

// headerStatus is the dim subtitle beside the title badge: repo + sync state +
// any transient message.
func (m Model) headerStatus() string {
	var sync string
	switch {
	case m.busy:
		sync = m.spinner.View() + theme.Label.Render("working")
	case !m.hasRepo:
		sync = theme.Label.Render("no repo")
	case !m.status.HasRemote:
		s := "local only"
		if m.status.Dirty {
			s += " ●"
		}
		sync = theme.Label.Render(s)
	default:
		s := "synced"
		if m.status.Ahead > 0 || m.status.Behind > 0 {
			s = fmt.Sprintf("↑%d ↓%d", m.status.Ahead, m.status.Behind)
		}
		if m.status.Dirty {
			s += " ●"
		}
		sync = theme.Label.Render(s)
	}

	repo := "no remote"
	if m.repo.Owner != "" {
		repo = m.repo.Slug()
	}
	out := theme.Label.Render(repo) + "   " + sync
	if m.message != "" {
		st := theme.Success
		if m.isErr {
			st = theme.Error
		}
		out += "   " + st.Render(m.message)
	}
	return out
}

// footer is the dim key-hint line under the tree.
func (m Model) footer() string {
	return theme.Label.Render("↑/↓ move · enter open · / filter · c copy · a add · ? help · q quit")
}

// createView shows the type picker, or the field form once a type is chosen.
func (m Model) createView() string {
	if m.genPicking {
		var b strings.Builder
		for i, p := range presets {
			// same two-space indent for every row; the selection only adds the
			// highlight (with a trailing space for the bar), never a left shift.
			label := p.name
			if i == m.genPreset {
				label = theme.Selected.Render(p.name + " ")
			}
			b.WriteString("  " + label + "\n")
		}
		b.WriteString("\n" + theme.Label.Render("j/k pick · enter · esc cancel"))
		return b.String()
	}

	p := m.preset()
	pad := func(s string) string {
		return theme.Label.Render(s + strings.Repeat(" ", p.labelWidth()-len(s)+2))
	}
	var rows []string
	ii := 0
	for _, f := range p.fields {
		if f.kind == fSecret && f.gen {
			rows = append(rows, pad(f.label)+theme.Label.Render("(generated)"))
			continue
		}
		rows = append(rows, pad(f.label)+m.genInputs[ii].View())
		ii++
	}
	if p.hasGen() {
		sym := "[ ]"
		if m.symbols {
			sym = "[x]"
		}
		rows = append(rows, pad("length")+m.genLen.View())
		rows = append(rows, pad("symbols")+sym+theme.Label.Render("  (ctrl+s)"))
	}
	rows = append(rows, "", theme.Label.Render("tab next · enter create · esc cancel"))
	return strings.Join(rows, "\n")
}

func (m Model) unlockView() string {
	return strings.Join([]string{
		theme.Label.Render("passphrase ") + m.passInput.View(),
		"",
		theme.Label.Render("enter unlock · esc cancel"),
	}, "\n")
}

func (m Model) promptView() string {
	verb := "move"
	if m.promptCopy {
		verb = "copy"
	}
	return strings.Join([]string{
		theme.Label.Render(verb+" ") + theme.Body.Render(m.promptSrc),
		theme.Label.Render("to ") + m.promptInput.View(),
		"",
		theme.Label.Render("enter confirm · esc cancel"),
	}, "\n")
}

func (m Model) commitView() string {
	return strings.Join([]string{
		theme.Label.Render("message ") + m.promptInput.View(),
		"",
		theme.Label.Render("enter commit · esc cancel"),
	}, "\n")
}

// copyView lists the selected entry's fields so the user can pick one to copy.
// The password row stays masked; the cursor field is accented.
func (m Model) copyView() string {
	fields := parseFields(m.detailRaw)
	kw := 0
	for _, f := range fields {
		if w := lipgloss.Width(f.key); w > kw {
			kw = w
		}
	}
	var b strings.Builder
	for i, f := range fields {
		key := f.key
		if key == "" {
			key = "note"
		}
		val := f.val
		if i == 0 {
			val = strings.Repeat("•", max(len([]rune(val)), 1))
		}
		cursor, ks := "  ", theme.Body
		if i == m.copyCursor {
			cursor, ks = theme.Accent.Render("› "), theme.Accent
		}
		b.WriteString(cursor + ks.Render(key+strings.Repeat(" ", kw-lipgloss.Width(f.key))) +
			"  " + theme.Label.Render(val) + "\n")
	}
	b.WriteString("\n" + theme.Label.Render("j/k move · enter copy · esc cancel"))
	return b.String()
}

func (m Model) confirmView() string {
	return strings.Join([]string{
		theme.Error.Render(m.current + "?"),
		"",
		theme.Label.Render("y confirm · any other key cancel"),
	}, "\n")
}

func (m Model) githubView() string {
	switch m.ghView {
	case ghAlerts:
		return m.alertsView()
	case ghRuns:
		return m.runsView()
	}

	items := []string{"commit staged changes", "sync (pull --rebase + push)", "provision private repo", "secret-scanning alerts", "actions runs"}
	var b strings.Builder
	for i, it := range items {
		num := theme.Label.Render(fmt.Sprintf("%02d ", i+1))
		label := it
		if i == m.ghCursor {
			label = theme.Selected.Render(it + " ")
		}
		b.WriteString(num + label + "\n")
	}
	b.WriteString("\n" + theme.Label.Render("j/k move · enter select · esc close"))
	return b.String()
}

func (m Model) alertsView() string {
	var b strings.Builder
	if len(m.alerts) == 0 {
		b.WriteString(theme.Success.Render("no open alerts — repo is clean") + "\n")
		b.WriteString(theme.Label.Render("(store holds encrypted .gpg blobs; empty is expected)") + "\n")
	}
	for _, a := range m.alerts {
		b.WriteString(theme.Label.Render(fmt.Sprintf("#%d ", a.Number)) +
			theme.Body.Render(a.Type) + theme.Label.Render(" "+a.State) + "\n")
	}
	b.WriteString("\n" + theme.Label.Render("esc back"))
	return b.String()
}

func (m Model) runsView() string {
	var b strings.Builder
	if len(m.runs) == 0 {
		b.WriteString(theme.Label.Render("no runs") + "\n")
	}
	for _, r := range m.runs {
		state := r.Conclusion
		if state == "" {
			state = r.Status
		}
		st := theme.Body
		switch state {
		case "success":
			st = theme.Success
		case "failure":
			st = theme.Error
		}
		b.WriteString(st.Render(state) + theme.Label.Render("  "+r.Name) + "\n")
	}
	b.WriteString("\n" + theme.Label.Render("esc back"))
	return b.String()
}

// helpView renders the key bindings as labelled columns, packing as many
// columns per row as the width allows and wrapping the rest onto further rows,
// then dropping the inter-row spacing when height is tight — so it lays out
// against both dimensions instead of one wide strip.
func helpView(width, height int) string {
	const gutter = "      "
	availW := width - 2 // a little breathing room inside the padded screen
	if availW < 1 {
		availW = 1
	}

	type column struct {
		block string
		w     int
	}
	var cols []column
	for _, g := range keys.helpGroups() {
		kw := 0
		for _, b := range g.keys {
			if w := lipgloss.Width(b.Help().Key); w > kw {
				kw = w
			}
		}
		lines := []string{theme.Accent.Render(g.title), ""}
		w := lipgloss.Width(g.title)
		for _, b := range g.keys {
			h := b.Help()
			line := theme.Label.Render(h.Key+strings.Repeat(" ", kw-lipgloss.Width(h.Key))) + "  " + h.Desc
			lines = append(lines, line)
			if lw := lipgloss.Width(line); lw > w {
				w = lw
			}
		}
		cols = append(cols, column{strings.Join(lines, "\n"), w})
	}

	// pad every column to the widest so the grid's columns line up vertically.
	maxW := 0
	for _, c := range cols {
		if c.w > maxW {
			maxW = c.w
		}
	}
	for i, c := range cols {
		var ls []string
		for _, ln := range strings.Split(c.block, "\n") {
			ls = append(ls, ln+strings.Repeat(" ", maxW-lipgloss.Width(ln)))
		}
		cols[i].block, cols[i].w = strings.Join(ls, "\n"), maxW
	}

	// fixed 2-per-row grid; a row that would overflow the width still wraps early.
	var rows []string
	var row []string
	cnt, rowW := 0, 0
	for _, c := range cols {
		add := c.w
		if cnt > 0 {
			add += len(gutter)
		}
		if cnt > 0 && (cnt >= 2 || rowW+add > availW) {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
			row, cnt, rowW = nil, 0, 0
			add = c.w
		}
		if cnt > 0 {
			row = append(row, gutter)
		}
		row = append(row, c.block)
		cnt++
		rowW += add
	}
	if len(row) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}

	// blank line between wrapped rows for breathing room, dropped when the
	// content would otherwise overflow the available height.
	sep := "\n\n"
	total := 2 * (len(rows) - 1)
	for _, r := range rows {
		total += strings.Count(r, "\n") + 1
	}
	if height > 0 && total > height-4 {
		sep = "\n"
	}
	return strings.Join(rows, sep)
}
