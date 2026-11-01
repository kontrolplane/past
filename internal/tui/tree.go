package tui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kontrolplane/past/internal/theme"
)

// node is one node in the entry tree: a folder when path == "" (its contents
// live in children) or a leaf carrying the full pass entry path.
type node struct {
	label    string
	path     string
	dir      string // a folder's own path in the store, e.g. "bank/cards"
	expanded bool
	children []*node
}

// tree is the browse-pane widget: a collapsible hierarchy of store entries with
// a live filter. It replaces the flat list so that foo/bar renders nested under
// a foo/ folder. Folders start expanded, so a fresh store shows everything.
// The selected entry's detail is drawn beside the tree, not in it.
type tree struct {
	root     *node
	rows     []*node  // currently visible rows, recomputed by rebuild
	prefixes []string // tree-branch prefix per visible row, aligned with rows
	cursor   int
	offset   int // first visible row
	width    int
	height   int

	editing bool // the filter input has focus
	filter  string
	input   textinput.Model
}

func newTree() tree {
	in := textinput.New()
	in.Placeholder = "name or folder"
	in.Prompt = ""
	in.SetStyles(theme.TextInput())
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
			cur = childFolder(cur, seg, strings.Join(segs[:i+1], "/"))
		}
	}
	sortNode(root)
	t.root = root
	t.cursor = 0
	t.offset = 0
	t.rebuild()
}

// childFolder finds or creates the folder child of parent with the given label.
func childFolder(parent *node, label, dir string) *node {
	for _, c := range parent.children {
		if c.path == "" && c.label == label {
			return c
		}
	}
	n := &node{label: label, dir: dir, expanded: true}
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

func (t *tree) clampOffset() {
	h := max(1, t.height)
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+h {
		t.offset = t.cursor - h + 1
	}
	t.offset = max(0, min(t.offset, len(t.rows)-h))
}

// position reports where the cursor is among the visible rows, e.g. "3 of 20".
func (t *tree) position() string {
	if len(t.rows) == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", t.cursor+1, len(t.rows))
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

// jump moves the cursor by n rows, stopping at either end.
func (t *tree) jump(n int) {
	t.cursor = max(0, min(len(t.rows)-1, t.cursor+n))
	t.clampOffset()
}

// fold collapses (or with open, expands) the folder under the cursor. On an
// entry, or a folder that is already closed, collapsing steps out to the parent.
func (t *tree) fold(open bool) {
	n := t.selected()
	if n == nil || t.filter != "" {
		return
	}
	if n.path == "" && n.expanded != open {
		t.toggle()
		return
	}
	if open {
		return
	}
	depth := utf8.RuneCountInString(t.prefixes[t.cursor])
	for i := t.cursor - 1; i >= 0; i-- {
		if t.rows[i].path == "" && utf8.RuneCountInString(t.prefixes[i]) < depth {
			t.cursor = i
			t.clampOffset()
			return
		}
	}
}

// toggle folds or unfolds the folder under the cursor. Leaves have no fold
// state.
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
	return t.input.Focus()
}

// updateFilter feeds a key to the filter input; esc clears, enter confirms.
func (t *tree) updateFilter(msg tea.KeyPressMsg) tea.Cmd {
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

// leaves counts the entries under n, in every folder below it.
func leaves(n *node) int {
	c := 0
	for _, ch := range n.children {
		if ch.path == "" {
			c += leaves(ch)
		} else {
			c++
		}
	}
	return c
}

// view renders the visible window of rows, the one under the cursor on the selection band.
func (t *tree) view(focused bool) string {
	if len(t.rows) == 0 {
		if t.filter != "" {
			return "  " + theme.Muted("no entry matches ") + theme.Bold(theme.Clean(t.filter))
		}
		return "  " + theme.Muted("the store is empty") + "\n\n  " + theme.Key("a", "add your first entry")
	}
	end := min(len(t.rows), t.offset+t.height)
	lines := make([]string, 0, end-t.offset)
	for i := t.offset; i < end; i++ {
		lines = append(lines, t.renderRow(t.rows[i], t.prefixes[i], i == t.cursor, focused))
	}
	return strings.Join(lines, "\n")
}

// renderRow draws one row: a marker, the tree's branches, then the label. Filtered rows show the
// whole path with the match picked out. Folders end in a faint slash and count what they hide.
func (t *tree) renderRow(n *node, prefix string, sel, focused bool) string {
	var bg color.Color
	marker := []theme.Span{theme.S("  ", theme.ToneFaint)}
	if sel {
		bg = theme.P.Surface
		tone := theme.ToneMuted
		if focused {
			bg, tone = theme.P.Selection, theme.ToneAccent
		}
		marker = []theme.Span{theme.S("▌ ", tone)}
	}

	var spans []theme.Span
	switch {
	case t.filter != "":
		spans = matchSpans(n.path, t.filter, sel)
	case n.path == "":
		tone := theme.ToneMuted
		if sel {
			tone = theme.ToneText
		}
		spans = []theme.Span{theme.S(prefix, theme.ToneRule), theme.B(n.label, tone), theme.S("/", theme.ToneFaint)}
		if !n.expanded {
			spans = append(spans, theme.S(fmt.Sprintf(" +%d", leaves(n)), theme.ToneFaint))
		}
	default:
		label := theme.S(n.label, theme.ToneBody)
		if sel {
			label = theme.B(n.label, theme.ToneText)
		}
		spans = []theme.Span{theme.S(prefix, theme.ToneRule), label}
	}
	if sel {
		spans = lift(spans)
	}
	return line(append(marker, spans...), t.width, bg)
}

// matchSpans renders a path with the part matching the filter in the accent.
func matchSpans(path, filter string, sel bool) []theme.Span {
	tone := theme.ToneBody
	if sel {
		tone = theme.ToneText
	}
	lower := strings.ToLower(path)
	i := strings.Index(lower, strings.ToLower(filter))
	if i < 0 || len(lower) != len(path) {
		return theme.Path(path, tone)
	}
	j := i + len(filter)
	spans := theme.Path(path[:i], tone)
	if i == 0 {
		spans = nil
	}
	spans = append(spans, theme.B(path[i:j], theme.ToneAccent))
	if j < len(path) {
		spans = append(spans, theme.Path(path[j:], tone)...)
	}
	return spans
}

// lift raises faint spans on a selected row, where the ground has less contrast to spare.
func lift(spans []theme.Span) []theme.Span {
	out := make([]theme.Span, len(spans))
	for i, s := range spans {
		switch s.Tone {
		case theme.ToneFaint:
			s.Tone = theme.ToneMuted
		case theme.ToneRule:
			s.Tone = theme.ToneFaint
		}
		out[i] = s
	}
	return out
}
