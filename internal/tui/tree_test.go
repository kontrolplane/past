package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// visiblePaths returns the path (or folder label + "/") of each visible row,
// in order, so tests can assert on structure without rendering styles.
func visiblePaths(t *tree) []string {
	out := make([]string, len(t.rows))
	for i, n := range t.rows {
		if n.path == "" {
			out[i] = n.label + "/"
		} else {
			out[i] = n.path
		}
	}
	return out
}

func TestSetEntriesNestsAndOrders(t *testing.T) {
	tr := newTree()
	// deliberately unsorted; folders should sort before sibling leaves.
	tr.setEntries([]string{"email/work", "github", "email/personal", "servers/web"})

	got := strings.Join(visiblePaths(&tr), ",")
	want := "email/,email/personal,email/work,servers/,servers/web,github"
	if got != want {
		t.Fatalf("visible rows = %q, want %q", got, want)
	}
}

func TestToggleCollapsesSubtree(t *testing.T) {
	tr := newTree()
	tr.setEntries([]string{"email/work", "email/personal", "github"})

	// cursor starts on the "email" folder; collapsing hides its children.
	if n := tr.selected(); n == nil || n.label != "email" || n.path != "" {
		t.Fatalf("expected cursor on email folder, got %+v", n)
	}
	tr.toggle()

	got := strings.Join(visiblePaths(&tr), ",")
	if want := "email/,github"; got != want {
		t.Fatalf("after collapse = %q, want %q", got, want)
	}

	tr.toggle() // expand again
	if got := strings.Join(visiblePaths(&tr), ","); !strings.Contains(got, "email/work") {
		t.Fatalf("after re-expand children missing: %q", got)
	}
}

func TestFilterFlattensMatches(t *testing.T) {
	tr := newTree()
	tr.setEntries([]string{"email/work", "email/personal", "servers/web"})

	tr.startFilter()
	for _, r := range "work" {
		tr.updateFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	got := visiblePaths(&tr)
	if len(got) != 1 || got[0] != "email/work" {
		t.Fatalf("filtered rows = %v, want [email/work]", got)
	}
	if sel := tr.selected(); sel == nil || sel.path != "email/work" {
		t.Fatalf("selected after filter = %+v, want email/work", sel)
	}

	tr.clearFilter()
	if got := visiblePaths(&tr); len(got) != 5 { // 2 folders + 3 leaves
		t.Fatalf("after clear = %v, want all rows back", got)
	}
}

func TestNavigationClamps(t *testing.T) {
	tr := newTree()
	tr.setEntries([]string{"a", "b"})
	tr.up() // already at top
	if tr.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", tr.cursor)
	}
	tr.down()
	tr.down() // past the end
	if tr.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (clamped)", tr.cursor)
	}
}
