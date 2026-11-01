package tui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kontrolplane/past/internal/git"
	"github.com/kontrolplane/past/internal/pass"
	"github.com/kontrolplane/past/internal/theme"
)

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "past"
	v.ForegroundColor = theme.P.Text
	if theme.Paint {
		v.BackgroundColor = theme.P.Base
	}
	return v
}

// render draws the screen the way relay and tablespace do: a grid of facts on
// top, the framed page with a breadcrumb set into its edge, and the keys below.
func (m Model) render() string {
	if m.width == 0 {
		return ""
	}
	if m.tooSmall() {
		return m.tooSmallView()
	}
	meta, foot := m.frameMeta()
	screen := m.renderHeader() + "\n\n" +
		frame(theme.Render(m.breadcrumb()...), meta, foot, m.content()) + "\n" +
		m.renderFooter()
	return place(m.width, m.height, screen)
}

func (m Model) tooSmall() bool {
	return m.width < minContentWidth+chromeWidth || m.height < minContentHeight+chromeHeight
}

func (m Model) tooSmallView() string {
	notice := lipgloss.JoinVertical(lipgloss.Center,
		theme.Render(theme.B("terminal too small", theme.ToneWarning)),
		theme.Muted(fmt.Sprintf("%d×%d, needs %d×%d", m.width, m.height, minContentWidth+chromeWidth, minContentHeight+chromeHeight)),
		theme.Faint("ctrl+c to quit"),
	)
	return clip(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, notice), m.width, m.height)
}

// content renders what the frame holds: the page, with a dialog or the help
// set over it.
func (m Model) content() string {
	if !m.ready {
		return place(contentWidth, contentHeight,
			theme.Accent(m.spinner.View())+" "+theme.Muted("checking the gpg key…"))
	}
	page := m.page()
	if m.showHelp {
		return overlay(page, m.helpCard())
	}
	if card := m.dialogCard(); card != "" {
		return overlay(page, card)
	}
	return page
}

// --- header ---

// fact is one label and value in the header grid.
type fact struct {
	label string
	value []theme.Span
}

const headerGap = 4 // the least room between the fact columns

// renderHeader lays the facts out in columns across the frame, dropping
// columns from the right when they do not fit.
func (m Model) renderHeader() string {
	width := frameWidth - 1
	columns := [][]fact{m.storeFacts(), m.countFacts(), m.syncFacts(), m.settingFacts()}

	render := func(facts []fact) string {
		lw := 0
		for _, f := range facts {
			lw = max(lw, len(f.label))
		}
		lines := make([]string, len(facts))
		for i, f := range facts {
			lines[i] = theme.Faint(f.label+strings.Repeat(" ", lw-len(f.label)+2)) + theme.Render(f.value...)
		}
		return strings.Join(lines, "\n")
	}
	var blocks []string
	total := 0
	for _, c := range columns {
		b := render(c)
		if len(blocks) > 0 && total+headerGap*len(blocks)+lipgloss.Width(b) > width {
			break
		}
		blocks = append(blocks, b)
		total += lipgloss.Width(b)
	}
	// Spare room goes between the columns, so the grid spans the frame.
	gaps := len(blocks) - 1
	spare := max(0, width-total)
	out := make([]string, 0, 2*len(blocks))
	for i, b := range blocks {
		if i > 0 {
			gap := spare / gaps
			if i <= spare%gaps {
				gap++
			}
			out = append(out, strings.Repeat(" ", gap))
		}
		out = append(out, b)
	}
	// every line spans the frame, so centring the screen keeps the columns put.
	lines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, out...), "\n")
	for i, l := range lines {
		lines[i] = " " + padRight(l, width)
	}
	return strings.Join(lines, "\n")
}

// home shortens a path under the home directory to ~.
func home(p string) string {
	if h, err := os.UserHomeDir(); err == nil && h != "" && strings.HasPrefix(p, h) {
		return "~" + strings.TrimPrefix(p, h)
	}
	return p
}

func (m Model) storeFacts() []fact {
	remote := []theme.Span{theme.S("-", theme.ToneFaint)}
	switch {
	case m.repo.Owner != "":
		remote = []theme.Span{theme.S(m.repo.Owner, theme.ToneMuted), theme.S("/", theme.ToneFaint), theme.S(m.repo.Name, theme.ToneBody)}
	case !m.hasRepo:
		remote = []theme.Span{theme.S("not a git repo", theme.ToneFaint)}
	}
	return []fact{
		{"store", []theme.Span{theme.S(truncate(home(pass.StoreDir()), 40), theme.ToneBody)}},
		{"remote", remote},
	}
}

func (m Model) countFacts() []fact {
	entries, folders := 0, 0
	var walk func(*node)
	walk = func(n *node) {
		for _, c := range n.children {
			if c.path == "" {
				folders++
				walk(c)
			} else {
				entries++
			}
		}
	}
	walk(m.tree.root)
	n := func(v int) []theme.Span {
		if v == 0 {
			return []theme.Span{theme.S("0", theme.ToneFaint)}
		}
		return []theme.Span{theme.S(fmt.Sprint(v), theme.ToneBody)}
	}
	return []fact{{"entries", n(entries)}, {"folders", n(folders)}}
}

func (m Model) syncFacts() []fact {
	s := m.status
	var upstream, changes []theme.Span
	switch {
	case !m.hasRepo:
		upstream = []theme.Span{theme.S("-", theme.ToneFaint)}
		changes = []theme.Span{theme.S("-", theme.ToneFaint)}
	default:
		switch {
		case !s.HasRemote:
			upstream = []theme.Span{theme.S("no upstream", theme.ToneFaint)}
		case s.Ahead == 0 && s.Behind == 0:
			upstream = []theme.Span{theme.S("up to date", theme.ToneBody)}
		default:
			upstream = aheadBehind(s)
		}
		changes = []theme.Span{theme.S("none", theme.ToneFaint)}
		if s.Dirty {
			changes = []theme.Span{theme.S("uncommitted", theme.ToneWarning)}
		}
	}
	return []fact{{"upstream", upstream}, {"changes", changes}}
}

// aheadBehind renders the commits to push and to pull, the zero side faint.
func aheadBehind(s git.SyncStatus) []theme.Span {
	tone := func(n int) theme.Tone {
		if n == 0 {
			return theme.ToneFaint
		}
		return theme.ToneWarning
	}
	return []theme.Span{
		theme.S(fmt.Sprintf("↑%d", s.Ahead), tone(s.Ahead)),
		theme.S(" ", theme.ToneFaint),
		theme.S(fmt.Sprintf("↓%d", s.Behind), tone(s.Behind)),
	}
}

func (m Model) settingFacts() []fact {
	commit := []theme.Span{theme.S("auto", theme.ToneBody)}
	if !git.AutoCommit {
		commit = []theme.Span{theme.S("manual", theme.ToneBody), theme.S("  h to commit", theme.ToneFaint)}
	}
	clip := os.Getenv("PASSWORD_STORE_CLIP_TIME")
	if clip == "" {
		clip = "45"
	}
	return []fact{
		{"commits", commit},
		{"clipboard", []theme.Span{theme.S("clears after ", theme.ToneFaint), theme.S(clip+"s", theme.ToneBody)}},
	}
}

// --- frame ---

// maxCrumbWidth bounds the crumbs between the first and the last, so a long
// folder name leaves room for the entry it leads to.
const maxCrumbWidth = 24

// breadcrumb names where the cursor sits, e.g. store › bank › amex, and the
// dialog open over it.
func (m Model) breadcrumb() []theme.Span {
	trail := []string{"store"}
	n := m.tree.selected()
	switch {
	case m.mode == modeCreate:
		trail = append(trail, "new entry")
		if !m.genPicking {
			trail = append(trail, m.preset().name)
		}
	case m.mode == modeGithub:
		trail = append(trail, "github")
		switch m.ghView {
		case ghAlerts:
			trail = append(trail, "alerts")
		case ghRuns:
			trail = append(trail, "runs")
		}
	case m.mode == modeCommit:
		trail = append(trail, "commit")
	case m.mode == modeUnlock:
		trail = append(trail, "unlock")
	case m.tree.filtering():
		trail = append(trail, "filter")
	case n != nil && n.path != "":
		trail = append(trail, strings.Split(n.path, "/")...)
	case n != nil:
		trail = append(trail, strings.Split(n.dir, "/")...)
	}
	switch m.mode {
	case modeConfirmDelete:
		trail = append(trail, "delete")
	case modePrompt:
		if m.promptCopy {
			trail = append(trail, "duplicate")
		} else {
			trail = append(trail, "rename")
		}
	case modeCopy:
		trail = append(trail, "copy")
	}

	var spans []theme.Span
	for i, t := range trail {
		if i > 0 {
			spans = append(spans, theme.S(" › ", theme.ToneFaint))
		}
		if i == len(trail)-1 {
			spans = append(spans, theme.B(t, theme.ToneText))
			continue
		}
		if i > 0 {
			t = truncate(t, maxCrumbWidth)
		}
		spans = append(spans, theme.S(t, theme.ToneMuted))
	}
	if n != nil && n.path == "" && m.mode == modeBrowse && !m.tree.filtering() {
		spans = append(spans, theme.S(fmt.Sprintf(" %d", leaves(n)), theme.ToneFaint))
	}
	return spans
}

// frameMeta returns what is set into the frame's edges: the active filter on
// top, and at the bottom where the cursor is and the state of the git repo.
func (m Model) frameMeta() (string, string) {
	var meta string
	if f := m.tree.filter; f != "" && !m.tree.editing {
		meta = theme.Render(theme.S("filter ", theme.ToneFaint), theme.S(f, theme.ToneText))
	}
	var foot []string
	if p := m.tree.position(); p != "" && m.ready {
		foot = append(foot, theme.Faint(p))
	}
	foot = append(foot, m.syncState())
	return meta, strings.Join(foot, theme.Faint(" · "))
}

// syncState renders the state of the store's repo, like the connection state
// in relay: a dot in the tone of what needs doing.
func (m Model) syncState() string {
	s := m.status
	switch {
	case m.busy:
		return theme.Accent(m.spinner.View()) + " " + theme.Muted("working")
	case !m.hasRepo:
		return theme.Render(theme.S("○ ", theme.ToneFaint), theme.S("no repo", theme.ToneMuted))
	case s.Dirty:
		return theme.Render(theme.S("● ", theme.ToneWarning), theme.S("uncommitted", theme.ToneBody))
	case !s.HasRemote:
		return theme.Render(theme.S("● ", theme.ToneMuted), theme.S("local only", theme.ToneBody))
	case s.Ahead > 0 || s.Behind > 0:
		return theme.Render(append([]theme.Span{theme.S("● ", theme.ToneWarning)}, aheadBehind(s)...)...)
	}
	return theme.Render(theme.S("● ", theme.ToneSuccess), theme.S("synced", theme.ToneBody))
}

// --- footer ---

func (m Model) renderFooter() string {
	var status string
	if m.message != "" {
		if m.isErr {
			status = theme.Render(theme.S("✗ ", theme.ToneDanger), theme.S(m.message, theme.ToneDanger))
		} else {
			status = theme.Render(theme.S("✓ ", theme.ToneSuccess), theme.S(m.message, theme.ToneBody))
		}
	}
	if lipgloss.Width(status) > frameWidth/2 {
		status = truncate(status, frameWidth/2)
	}
	width := frameWidth - 2 - lipgloss.Width(status) - 2
	left := truncate(m.renderFooterLeft(width), width)
	return " " + spread(left, status+" ", frameWidth-1)
}

func (m Model) renderFooterLeft(width int) string {
	if m.mode == modeBrowse && m.tree.editing && !m.showHelp {
		return theme.Accent("/ ") + m.tree.input.View() + "  " +
			hints([2]string{"enter", "apply"}, [2]string{"esc", "clear"})
	}
	return fitHints(width, m.shortHelp()...)
}

var confirmHelp = [][2]string{{"y/n", "yes/no"}, {"←/→", "choose"}, {"enter", "confirm"}, {"esc", "cancel"}}

func (m Model) shortHelp() [][2]string {
	switch {
	case !m.ready:
		return nil
	case m.showHelp:
		return [][2]string{{"any key", "close"}}
	}
	switch m.mode {
	case modeCreate:
		if m.genPicking {
			return [][2]string{{"↑/↓", "choose"}, {"enter", "next"}, {"esc", "cancel"}}
		}
		h := [][2]string{{"tab", "next field"}, {"shift+tab", "previous"}}
		if m.preset().hasGen() {
			h = append(h, [2]string{"ctrl+s", "symbols"})
		}
		return append(h, [2]string{"enter", "create"}, [2]string{"esc", "cancel"})
	case modeConfirmDelete:
		return confirmHelp
	case modeGithub:
		if m.ghView != ghMenu {
			return [][2]string{{"esc", "back"}}
		}
		return [][2]string{{"↑/↓", "choose"}, {"enter", "run"}, {"esc", "close"}}
	case modeUnlock:
		return [][2]string{{"enter", "unlock"}, {"esc", "skip"}}
	case modePrompt:
		verb := "move"
		if m.promptCopy {
			verb = "duplicate"
		}
		return [][2]string{{"enter", verb}, {"esc", "cancel"}}
	case modeCommit:
		return [][2]string{{"enter", "commit"}, {"esc", "cancel"}}
	case modeCopy:
		return [][2]string{{"↑/↓", "choose"}, {"enter", "copy"}, {"esc", "cancel"}}
	}
	open := [2]string{"enter", "reveal"}
	if n := m.tree.selected(); n != nil && n.path == "" {
		open = [2]string{"enter", "fold"}
	} else if !m.masked {
		open = [2]string{"enter", "hide"}
	}
	h := [][2]string{open, {"c", "copy"}, {"a", "add"}, {"e", "edit"}, {"d", "delete"}, {"/", "filter"}, {"h", "github"}, {"?", "help"}, {"q", "quit"}}
	if m.tree.filter != "" {
		h[len(h)-1] = [2]string{"esc", "clear filter"}
	}
	return h
}

// --- page ---

// treeWidth is the width of the tree pane, the rest of the page shows the entry.
func treeWidth() int {
	return max(28, min(52, pageWidth()*2/5))
}

// page draws the tree beside the selected entry, split by a hairline.
func (m Model) page() string {
	tw := treeWidth()
	dw := pageWidth() - tw - 5
	left := strings.Split(m.tree.view(m.mode == modeBrowse), "\n")
	right := strings.Split(m.detailView(dw), "\n")
	divider := lipgloss.NewStyle().Foreground(theme.P.Rule).Render("│")

	lines := make([]string, contentHeight)
	for i := range lines {
		var l, r string
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines[i] = padRight(l, tw) + "  " + divider + "  " + r
	}
	return indent(strings.Join(lines, "\n"))
}

// detailView describes what the cursor is on: an entry with its fields, or a
// folder with what it holds.
func (m Model) detailView(width int) string {
	n := m.tree.selected()
	switch {
	case n == nil:
		return ""
	case n.path == "":
		return m.folderView(n, width)
	case n.path != m.current:
		return theme.Accent(m.spinner.View()) + " " + theme.Muted("decrypting…")
	}
	return m.entryView(width)
}

// attr is one aligned label and value in a detail section.
type attr struct {
	label string
	value string // already styled
}

// attrs aligns labels to the right of a column lw wide, values wrapping under
// themselves when they run past width.
func attrs(width, lw int, rows ...attr) []string {
	for _, r := range rows {
		lw = max(lw, lipgloss.Width(r.label))
	}
	vw := max(8, width-lw-2)
	var out []string
	for _, r := range rows {
		label := theme.Muted(strings.Repeat(" ", lw-lipgloss.Width(r.label)) + r.label)
		value := r.value
		if lipgloss.Width(value) > vw {
			value = lipgloss.Wrap(value, vw, "")
		}
		for i, l := range strings.Split(value, "\n") {
			if i == 0 {
				out = append(out, label+"  "+l)
			} else {
				out = append(out, strings.Repeat(" ", lw+2)+l)
			}
		}
	}
	return out
}

func (m Model) entryView(width int) string {
	fields := parseFields(m.detailRaw)
	secret := fields[0].val
	name := m.current
	dir, base := "", name
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		dir, base = name[:i], name[i+1:]
	}

	folder := theme.Faint("(top level)")
	if dir != "" {
		folder = theme.Render(theme.Path(dir, theme.ToneBody)...)
	}
	file := home(filepath.Join(pass.StoreDir(), name+".gpg"))
	out := []string{theme.SectionHeader("entry", "", width)}
	// every section's labels share one column, so the values line up.
	lw := len("strength")
	for _, f := range fields[1:] {
		lw = max(lw, lipgloss.Width(theme.Clean(f.key)))
	}
	lw = min(lw, width/3)
	out = append(out, attrs(width, lw,
		attr{"name", theme.Bold(theme.Clean(base))},
		attr{"folder", folder},
		attr{"file", theme.Faint(theme.Clean(file))},
	)...)

	// the secret, masked unless revealed, with how strong it is.
	runes := []rune(secret)
	meta := theme.Faint(fmt.Sprintf("%d chars", len(runes)))
	value := theme.Fg(theme.ToneBody).Render(strings.Repeat("•", min(max(len(runes), 1), 48)))
	if !m.masked {
		meta = theme.Render(theme.S("● ", theme.ToneWarning), theme.S("revealed", theme.ToneWarning)) + theme.Faint(" · ") + meta
		value = theme.Render(secretSpans(secret)...)
	}
	out = append(out, "", theme.SectionHeader("password", meta, width))
	label, tone, ratio := strength(secret)
	bar := theme.Render(theme.Bar(ratio, 16, tone)...) + "  " + theme.Render(theme.S(label, tone))
	out = append(out, attrs(width, lw, attr{"value", value}, attr{"strength", bar})...)

	var meta2, notes []field
	for _, f := range fields[1:] {
		if f.key == "" {
			notes = append(notes, f)
		} else {
			meta2 = append(meta2, f)
		}
	}
	if len(meta2) > 0 {
		out = append(out, "", theme.SectionHeader("fields", theme.Faint(fmt.Sprint(len(meta2))), width))
		rows := make([]attr, len(meta2))
		for i, f := range meta2 {
			rows[i] = attr{theme.Clean(f.key), fieldValue(f)}
		}
		out = append(out, attrs(width, lw, rows...)...)
	}
	if len(notes) > 0 {
		out = append(out, "", theme.SectionHeader("notes", "", width))
		for _, f := range notes {
			out = append(out, strings.Split(theme.Fg(theme.ToneBody).Width(width).Render(theme.Clean(f.val)), "\n")...)
		}
	}
	return clip(strings.Join(out, "\n"), width, contentHeight)
}

// fieldValue styles a metadata value: links in the info tone, the rest as body.
func fieldValue(f field) string {
	v := theme.Clean(f.val)
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return theme.Fg(theme.ToneInfo).Render(v)
	}
	return theme.Body(v)
}

// secretSpans colours a revealed secret by character class so it can be read
// back without mistaking 0 for O or l for 1: digits copper, symbols olive.
func secretSpans(s string) []theme.Span {
	var spans []theme.Span
	for _, r := range theme.Clean(s) {
		tone := theme.ToneText
		switch {
		case unicode.IsDigit(r):
			tone = theme.ToneWarm
		case !unicode.IsLetter(r):
			tone = theme.ToneAccent
		}
		if n := len(spans); n > 0 && spans[n-1].Tone == tone {
			spans[n-1].Text += string(r)
			continue
		}
		spans = append(spans, theme.Span{Text: string(r), Tone: tone})
	}
	if len(spans) == 0 {
		return []theme.Span{theme.S("(empty)", theme.ToneFaint)}
	}
	return spans
}

// strength estimates a secret's entropy from its length and the classes of
// characters it draws from, and names it.
func strength(s string) (string, theme.Tone, float64) {
	var lower, upper, digit, symbol, other bool
	n := 0
	for _, r := range s {
		n++
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		case r < 128:
			symbol = true
		default:
			other = true
		}
	}
	pool := 0
	for _, c := range []struct {
		on   bool
		size int
	}{{lower, 26}, {upper, 26}, {digit, 10}, {symbol, 33}, {other, 100}} {
		if c.on {
			pool += c.size
		}
	}
	if n == 0 || pool == 0 {
		return "empty", theme.ToneFaint, 0
	}
	bits := float64(n) * math.Log2(float64(pool))
	ratio := bits / 128
	switch {
	case bits < 40:
		return fmt.Sprintf("weak · %.0f bits", bits), theme.ToneDanger, ratio
	case bits < 64:
		return fmt.Sprintf("fair · %.0f bits", bits), theme.ToneWarning, ratio
	case bits < 100:
		return fmt.Sprintf("good · %.0f bits", bits), theme.ToneAccent, ratio
	}
	return fmt.Sprintf("strong · %.0f bits", bits), theme.ToneSuccess, ratio
}

func (m Model) folderView(n *node, width int) string {
	var subs, entries []*node
	for _, c := range n.children {
		if c.path == "" {
			subs = append(subs, c)
		} else {
			entries = append(entries, c)
		}
	}
	state := theme.Body("open")
	if !n.expanded {
		state = theme.Muted("folded")
	}
	out := []string{theme.SectionHeader("folder", "", width)}
	out = append(out, attrs(width, 0,
		attr{"path", theme.Render(theme.Path(n.dir, theme.ToneBody)...)},
		attr{"entries", theme.Body(fmt.Sprint(leaves(n)))},
		attr{"folders", theme.Body(fmt.Sprint(len(subs)))},
		attr{"state", state},
	)...)

	out = append(out, "", theme.SectionHeader("contents", theme.Faint(fmt.Sprint(len(n.children))), width))
	for _, c := range subs {
		out = append(out, theme.Render(theme.B(c.label, theme.ToneMuted), theme.S("/", theme.ToneFaint), theme.S(fmt.Sprintf("  %d", leaves(c)), theme.ToneFaint)))
	}
	for _, c := range entries {
		out = append(out, theme.Body(theme.Clean(c.label)))
	}
	if room := contentHeight - 7; len(out)-7 > room {
		more := len(out) - 7 - room + 1
		out = append(out[:7+room-1], theme.Faint(fmt.Sprintf("… +%d more", more)))
	}
	return clip(strings.Join(out, "\n"), width, contentHeight)
}

// --- dialogs ---

// block pads lines to one width so a dialog, which centres each line, keeps
// them aligned on the left.
func block(lines ...string) string {
	w := 0
	for _, l := range lines {
		w = max(w, lipgloss.Width(l))
	}
	for i, l := range lines {
		lines[i] = padRight(l, w)
	}
	return strings.Join(lines, "\n")
}

// choice renders one row of a picker, the chosen one marked and bright.
func choice(label, note string, on bool, lw int) string {
	pad := strings.Repeat(" ", max(0, lw-lipgloss.Width(label)))
	if on {
		return theme.Render(theme.S("› ", theme.ToneAccent), theme.B(label, theme.ToneText)) + pad + "  " + theme.Muted(note)
	}
	return "  " + theme.Muted(label) + pad + "  " + theme.Faint(note)
}

func (m Model) dialogCard() string {
	switch m.mode {
	case modeCreate:
		return m.createCard()
	case modeConfirmDelete:
		return m.deleteCard()
	case modeGithub:
		return m.githubCard()
	case modeUnlock:
		return m.unlockCard()
	case modePrompt:
		return m.promptCard()
	case modeCommit:
		return m.commitCard()
	case modeCopy:
		return m.copyCard()
	}
	return ""
}

// presetNote names the fields a preset asks for.
func presetNote(p preset) string {
	var parts []string
	for _, f := range p.fields {
		switch {
		case f.kind == fName:
		case f.kind == fSecret && f.gen:
			parts = append(parts, "generated password")
		default:
			parts = append(parts, f.label)
		}
	}
	return strings.Join(parts, ", ")
}

func (m Model) createCard() string {
	if m.genPicking {
		lw := 0
		for _, p := range presets {
			lw = max(lw, len(p.name))
		}
		rows := make([]string, len(presets))
		for i, p := range presets {
			rows[i] = choice(p.name, presetNote(p), i == m.genPreset, lw)
		}
		return dialog("new entry", theme.ToneText, "", theme.Muted("what kind of entry?"), "", block(rows...))
	}

	p := m.preset()
	lw := p.labelWidth()
	label := func(s string, on bool) string {
		s = strings.Repeat(" ", lw-len(s)) + s
		if on {
			return theme.Render(theme.B(s, theme.ToneAccent))
		}
		return theme.Muted(s)
	}
	var rows []string
	ii := 0
	for _, f := range p.fields {
		if f.kind == fSecret && f.gen {
			rows = append(rows, label(f.label, false)+"  "+theme.Faint("  generated when created"))
			continue
		}
		in := m.genInputs[ii]
		in.SetWidth(36)
		rows = append(rows, label(f.label, ii == m.genFocus)+"  "+in.View())
		ii++
	}
	if p.hasGen() {
		lenOn := m.genFocus == len(m.genInputs)
		sym := theme.Render(theme.S("□ ", theme.ToneFaint), theme.S("off", theme.ToneMuted))
		if m.symbols {
			sym = theme.Render(theme.S("■ ", theme.ToneAccent), theme.S("on", theme.ToneBody))
		}
		rows = append(rows, "",
			label("length", lenOn)+"  "+m.genLen.View(),
			label("symbols", false)+"    "+sym+theme.Faint("   ctrl+s"),
		)
	}
	return dialog("new "+p.name, theme.ToneText, "", block(rows...))
}

func (m Model) deleteCard() string {
	prompt := theme.Render(theme.S("delete ", theme.ToneBody), theme.B(truncate(m.current, 48), theme.ToneText), theme.S("?", theme.ToneBody))
	note := "the encrypted file is removed from the store."
	if m.hasRepo {
		note = "the encrypted file is removed; git history still has it."
	}
	return dialog("delete entry", theme.ToneDanger, "▲",
		prompt, theme.Muted(note), "", confirmButtons(m.confirmSel, theme.ToneDanger))
}

func (m Model) unlockCard() string {
	in := m.passInput
	in.SetWidth(32)
	return dialog("unlock", theme.ToneWarning, "▲",
		theme.Body("the gpg key for this store is locked."),
		theme.Muted("the passphrase stays in memory until past quits."),
		"", block(theme.Muted("passphrase")+"  "+in.View()))
}

func (m Model) promptCard() string {
	title, verb := "rename", "move"
	if m.promptCopy {
		title, verb = "duplicate", "copy"
	}
	in := m.promptInput
	in.SetWidth(40)
	return dialog(title, theme.ToneText, "",
		theme.Render(theme.S(verb+" ", theme.ToneMuted), theme.B(truncate(m.promptSrc, 48), theme.ToneText), theme.S(" to", theme.ToneMuted)),
		"", block(in.View()),
		"", theme.Faint("a new folder in the path is created"))
}

func (m Model) commitCard() string {
	in := m.promptInput
	in.Placeholder = "past: update store"
	in.SetWidth(40)
	return dialog("commit", theme.ToneText, "",
		theme.Muted("commit the staged changes to the store"),
		"", block(theme.Muted("message")+"  "+in.View()))
}

func (m Model) copyCard() string {
	fields := parseFields(m.detailRaw)
	lw := 0
	for _, f := range fields {
		lw = max(lw, lipgloss.Width(fieldKey(f)))
	}
	rows := make([]string, len(fields))
	for i, f := range fields {
		val := truncate(theme.Clean(f.val), 36)
		if i == 0 {
			val = strings.Repeat("•", min(max(len([]rune(f.val)), 1), 24))
		}
		rows[i] = choice(fieldKey(f), val, i == m.copyCursor, lw)
	}
	return dialog("copy", theme.ToneText, "",
		theme.Render(theme.S("from ", theme.ToneMuted), theme.B(truncate(m.current, 48), theme.ToneText)),
		"", block(rows...),
		"", theme.Faint("the password clears from the clipboard on its own"))
}

func fieldKey(f field) string {
	if f.key == "" {
		return "note"
	}
	return theme.Clean(f.key)
}

func (m Model) githubCard() string {
	switch m.ghView {
	case ghAlerts:
		return m.alertsCard()
	case ghRuns:
		return m.runsCard()
	}
	items := []struct{ label, note string }{
		{"commit", "commit the staged changes"},
		{"sync", "pull --rebase, then push"},
		{"provision", "create a private repo with gh"},
		{"alerts", "secret scanning alerts"},
		{"runs", "recent actions runs"},
	}
	lw := 0
	for _, it := range items {
		lw = max(lw, len(it.label))
	}
	rows := make([]string, len(items))
	for i, it := range items {
		rows[i] = choice(it.label, it.note, i == m.ghCursor, lw)
	}
	repo := theme.Faint("no origin remote")
	if m.repo.Owner != "" {
		repo = theme.Render(theme.S(m.repo.Owner, theme.ToneMuted), theme.S("/", theme.ToneFaint), theme.B(m.repo.Name, theme.ToneText))
	}
	return dialog("github", theme.ToneText, "", repo, "", block(rows...))
}

func (m Model) alertsCard() string {
	if len(m.alerts) == 0 {
		return dialog("secret scanning", theme.ToneSuccess, "✓",
			theme.Body("no open alerts"),
			theme.Muted("the store only holds encrypted files, so none is expected"))
	}
	rows := make([]string, len(m.alerts))
	for i, a := range m.alerts {
		rows[i] = theme.Render(theme.S(fmt.Sprintf("#%-4d", a.Number), theme.ToneFaint), theme.S(a.Type, theme.ToneBody), theme.S("  "+a.State, theme.ToneMuted))
	}
	return dialog("secret scanning", theme.ToneWarning, "▲", block(rows...))
}

func (m Model) runsCard() string {
	if len(m.runs) == 0 {
		return dialog("actions runs", theme.ToneText, "", theme.Muted("no runs yet"))
	}
	rows := make([]string, len(m.runs))
	for i, r := range m.runs {
		state := r.Conclusion
		if state == "" {
			state = r.Status
		}
		glyph, tone := "●", theme.ToneWarning
		switch state {
		case "success":
			glyph, tone = "✓", theme.ToneSuccess
		case "failure", "cancelled", "timed_out":
			glyph, tone = "✗", theme.ToneDanger
		}
		rows[i] = theme.Render(theme.S(glyph+" ", tone), theme.S(fmt.Sprintf("%-10s", state), tone), theme.S("  "+r.Name, theme.ToneBody))
	}
	return dialog("actions runs", theme.ToneText, "", block(rows...))
}

// helpCard draws the key reference in a card, as roomy as the content area allows.
func (m Model) helpCard() string {
	title := func(s string) string {
		return theme.Fg(theme.ToneAccent).Bold(true).MarginBottom(1).Render(s)
	}
	keyStyle := theme.Fg(theme.ToneText).Bold(true).Width(11)
	var columns []string
	for _, s := range helpSections {
		lines := []string{title(s.title)}
		for _, r := range s.rows {
			lines = append(lines, keyStyle.Render(r[0])+theme.Muted(r[1]))
		}
		columns = append(columns, lipgloss.JoinVertical(lipgloss.Left, lines...))
	}
	grid := func(perRow, gap int) string {
		spaced := lipgloss.NewStyle().MarginRight(gap)
		var rows []string
		for i := 0; i < len(columns); i += perRow {
			row := append([]string(nil), columns[i:min(i+perRow, len(columns))]...)
			for j := range row[:len(row)-1] {
				row[j] = spaced.Render(row[j])
			}
			if len(rows) > 0 {
				rows = append(rows, "")
			}
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
		}
		return lipgloss.JoinVertical(lipgloss.Left, rows...)
	}
	var card string
	for _, fit := range []struct{ perRow, gap, padY, padX int }{{4, 6, 1, 4}, {4, 3, 1, 2}, {2, 6, 1, 4}, {2, 3, 0, 2}} {
		body := lipgloss.JoinVertical(lipgloss.Center, grid(fit.perRow, fit.gap), "", theme.Faint("press any key to close"))
		card = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.P.RuleBold).
			Padding(fit.padY, fit.padX).
			Render(body)
		if lipgloss.Width(card) <= contentWidth-2 && lipgloss.Height(card) <= contentHeight {
			break
		}
	}
	return card
}
