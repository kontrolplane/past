package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kontrolplane/past/internal/theme"
)

// node is one node in the entry tree: a folder when path == "" (its contents
// live in children) or a leaf carrying the full pass entry path.
type node struct {
	label    string
	path     string
	expanded bool
	children []*node
}

// tree is the browse-pane widget: a collapsible hierarchy of store entries with
// a live filter. It replaces the flat list so that foo/bar renders nested under
// a foo/ folder. Folders start expanded, so a fresh store shows everything.
type tree struct {
	root     *node
	rows     []*node  // currently visible rows, recomputed by rebuild
	prefixes []string // tree-branch prefix per visible row, aligned with rows
	cursor   int
	offset   int // scroll offset, in display lines (rows + any inlined detail)
	width    int
	height   int

	detail     string // pre-rendered detail block, inlined under the selected leaf
	detailPath string // entry the detail belongs to; only shown under its own row

	editing bool // the filter input has focus
	filter  string
	input   textinput.Model
}

func newTree() tree {
	in := textinput.New()
	in.Placeholder = "filter"
	in.Prompt = "/"
	return tree{root: &node{expanded: true}, input: in}
}

// setEntries rebuilds the tree from sorted slash-separated entry names.
func (t *tree) setEntries(entries []string) {
	root := &node{expanded: true}
	for _, e := range entries {
		segs := strings.Split(e, "/")
		cur := root
		for i, seg := range segs {
			if i == len(segs)-1 {
				cur.children = append(cur.children, &node{label: seg, path: e})
				continue
			}
			cur = childFolder(cur, seg)
		}
	}
	sortNode(root)
	t.root = root
	t.cursor = 0
	t.offset = 0
	t.rebuild()
}

// childFolder finds or creates the folder child of parent with the given label.
func childFolder(parent *node, label string) *node {
	for _, c := range parent.children {
		if c.path == "" && c.label == label {
			return c
		}
	}
	n := &node{label: label, expanded: true}
	parent.children = append(parent.children, n)
	return n
}

// sortNode orders each folder's children: folders first, then leaves, alpha.
func sortNode(n *node) {
	sort.SliceStable(n.children, func(i, j int) bool {
		a, b := n.children[i], n.children[j]
		if af, bf := a.path == "", b.path == ""; af != bf {
			return af
		}
		return a.label < b.label
	})
	for _, c := range n.children {
		if c.path == "" {
			sortNode(c)
		}
	}
}

func (t *tree) filtering() bool { return t.editing || t.filter != "" }

// rebuild recomputes the visible rows. With an active filter it flattens to the
// matching leaves; otherwise it walks the tree honouring fold state.
func (t *tree) rebuild() {
	t.rows = t.rows[:0]
	t.prefixes = t.prefixes[:0]
	if t.filter != "" {
		q := strings.ToLower(t.filter)
		var walk func(*node)
		walk = func(n *node) {
			for _, c := range n.children {
				if c.path == "" {
					walk(c)
				} else if strings.Contains(strings.ToLower(c.path), q) {
					t.rows = append(t.rows, c)
					t.prefixes = append(t.prefixes, "")
				}
			}
		}
		walk(t.root)
	} else {
		// draw box-drawing branches: "├─ "/"└─ " for each row, with "│  "/"   "
		// guides carried down from ancestors so the lines connect.
		var walk func(n *node, guide string)
		walk = func(n *node, guide string) {
			for i, c := range n.children {
				last := i == len(n.children)-1
				branch := "├─ "
				ext := "│  "
				if last {
					branch, ext = "└─ ", "   "
				}
				t.rows = append(t.rows, c)
				t.prefixes = append(t.prefixes, guide+branch)
				if c.path == "" && c.expanded {
					walk(c, guide+ext)
				}
			}
		}
		walk(t.root, "")
	}
	if t.cursor >= len(t.rows) {
		t.cursor = len(t.rows) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.clampOffset()
}

// visibleHeight is the row capacity, reserving a line for the filter indicator.
func (t *tree) visibleHeight() int {
	h := t.height
	if t.filtering() {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (t *tree) clampOffset() {
	_, cur := t.lines()
	h := t.visibleHeight()
	if cur < t.offset {
		t.offset = cur
	}
	if cur >= t.offset+h {
		t.offset = cur - h + 1
	}
	// pull the open leaf's detail block into view if it runs past the bottom,
	// but never by scrolling the cursor row itself off the top.
	if blockEnd := cur + t.detailHeight(); blockEnd >= t.offset+h {
		t.offset = blockEnd - h + 1
		if t.offset > cur {
			t.offset = cur
		}
	}
	if t.offset < 0 {
		t.offset = 0
	}
}

// detailHeight is the number of inlined detail lines under the cursor, or 0 when
// the cursor isn't the leaf whose detail is loaded.
func (t *tree) detailHeight() int {
	n := t.selected()
	if !t.showsDetail(n) {
		return 0
	}
	return len(t.detailLines(t.prefixes[t.cursor]))
}

// showsDetail reports whether n is the leaf whose loaded detail should be
// inlined: the detail always shows for the selected entry, but only under its
// own row, so an async reload never flashes the old entry's detail under a new
// one.
func (t *tree) showsDetail(n *node) bool {
	return n != nil && n.path != "" && n.path == t.detailPath && t.detail != ""
}

// lines renders every visible row to a display line, inlining the detail block
// under the selected leaf. It returns the lines and the display-line index of
// the cursor row.
func (t *tree) lines() ([]string, int) {
	var out []string
	cursorLine := 0
	for i, n := range t.rows {
		sel := i == t.cursor
		if sel {
			cursorLine = len(out)
		}
		out = append(out, t.renderRow(n, t.prefixes[i], sel))
		if sel && t.showsDetail(n) {
			out = append(out, t.detailLines(t.prefixes[i])...)
		}
	}
	return out, cursorLine
}

// detailLines wraps the stored detail block and leads each line with the open
// leaf's continuation guide ("├─ " → "│  ", "└─ " → "   "), so the detail nests
// under the label and the tree's vertical line carries past it to the next row.
func (t *tree) detailLines(prefix string) []string {
	guide := prefix
	switch {
	case strings.HasSuffix(prefix, "├─ "):
		guide = strings.TrimSuffix(prefix, "├─ ") + "│  "
	case strings.HasSuffix(prefix, "└─ "):
		guide = strings.TrimSuffix(prefix, "└─ ") + "   "
	}
	w := t.width - lipgloss.Width(guide)
	if w < 1 {
		w = 1
	}
	lead := theme.Rule.Render(guide)
	var out []string
	for _, ln := range strings.Split(lipgloss.NewStyle().Width(w).Render(t.detail), "\n") {
		out = append(out, lead+ln)
	}
	return out
}

// selected returns the node under the cursor, or nil when the tree is empty.
func (t *tree) selected() *node {
	if t.cursor < 0 || t.cursor >= len(t.rows) {
		return nil
	}
	return t.rows[t.cursor]
}

func (t *tree) up() {
	if t.cursor > 0 {
		t.cursor--
		t.clampOffset()
	}
}

func (t *tree) down() {
	if t.cursor < len(t.rows)-1 {
		t.cursor++
		t.clampOffset()
	}
}

// toggle folds or unfolds the folder under the cursor. Leaves have no fold
// state — their detail is always shown when selected.
func (t *tree) toggle() {
	if n := t.selected(); n != nil {
		n.expanded = !n.expanded
		t.rebuild()
	}
}

func (t *tree) startFilter() tea.Cmd {
	t.editing = true
	t.filter = ""
	t.input.SetValue("")
	t.cursor = 0
	t.offset = 0
	t.rebuild()
	t.input.Focus()
	return textinput.Blink
}

// updateFilter feeds a key to the filter input; esc clears, enter confirms.
func (t *tree) updateFilter(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		t.clearFilter()
		return nil
	case "enter":
		t.editing = false
		t.input.Blur()
		return nil
	}
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	t.filter = strings.TrimSpace(t.input.Value())
	t.cursor = 0
	t.offset = 0
	t.rebuild()
	return cmd
}

func (t *tree) clearFilter() {
	t.editing = false
	t.filter = ""
	t.input.SetValue("")
	t.input.Blur()
	t.cursor = 0
	t.offset = 0
	t.rebuild()
}

func (t *tree) setSize(w, h int) {
	t.width = w
	t.height = h
	t.clampOffset()
}

func (t *tree) view() string {
	var b strings.Builder
	if t.filtering() {
		if t.editing {
			b.WriteString(t.input.View())
		} else {
			b.WriteString(theme.Label.Render("filter: " + t.filter))
		}
		b.WriteByte('\n')
	}
	if len(t.rows) == 0 {
		if t.filter != "" {
			b.WriteString(theme.Label.Render("no match"))
		} else {
			b.WriteString(theme.Label.Render("no entries") + "\n" +
				theme.Label.Render("press a to add your first entry"))
		}
		return b.String()
	}
	lines, _ := t.lines()
	h := t.visibleHeight()
	start := t.offset
	if start > len(lines) {
		start = len(lines)
	}
	end := start + h
	if end > len(lines) {
		end = len(lines)
	}
	for i := start; i < end; i++ {
		b.WriteString(lines[i])
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// renderRow draws one row: filtered rows show the full path; otherwise the
// dim box-drawing prefix leads the styled label (folders accented with a
// trailing slash, leaves plain). The selected row reverses the whole line.
func (t *tree) renderRow(n *node, prefix string, sel bool) string {
	if t.filter != "" {
		row := truncate("  "+n.path, t.width)
		style := theme.Item
		if sel {
			style = theme.Selected
		}
		return style.Width(t.width).Render(row)
	}

	label := n.label
	style := theme.Item
	if n.path == "" {
		label = n.label + "/"
		style = theme.Label // folders in dim tan, like the connectors
	}
	avail := t.width - lipgloss.Width(prefix)
	if avail < 0 {
		avail = 0
	}
	label = truncate(label, avail)

	if sel {
		return theme.Selected.Width(t.width).Render(prefix + label)
	}
	line := theme.Label.Render(prefix) + style.Render(label)
	return lipgloss.NewStyle().Width(t.width).Render(line)
}

// truncate trims s to w runes, adding an ellipsis when it overflows.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return string(r[:1])
	}
	return string(r[:w-1]) + "…"
}
