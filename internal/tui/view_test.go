package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func keyEnter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

func TestLooksLocked(t *testing.T) {
	locked := []string{
		"pass show x: gpg: decryption failed: No secret key",
		"gpg: public key decryption failed: Inappropriate ioctl for device",
		"gpg: pinentry error",
	}
	for _, s := range locked {
		if !looksLocked(errors.New(s)) {
			t.Errorf("looksLocked(%q) = false, want true", s)
		}
	}
	notLocked := []string{"pass show x: no such file or directory", "git: not a repository"}
	for _, s := range notLocked {
		if looksLocked(errors.New(s)) {
			t.Errorf("looksLocked(%q) = true, want false", s)
		}
	}
}

// the prompt guards: an empty or unchanged destination must cancel back to
// browse with an error flash, never fire a move/copy command.
func TestPromptGuards(t *testing.T) {
	for _, dst := range []string{"", "  ", "vault/x"} { // last == src after trim
		m := New().openPrompt("vault/x", false)
		m.promptInput.SetValue(dst)
		next, cmd := m.handlePromptKey(keyEnter())
		nm := next.(Model)
		if nm.mode != modeBrowse {
			t.Errorf("dst %q: mode = %v, want modeBrowse", dst, nm.mode)
		}
		if !nm.isErr || cmd != nil {
			t.Errorf("dst %q: want error flash and no command, got isErr=%v cmd!=nil=%v", dst, nm.isErr, cmd != nil)
		}
	}
	// a real destination fires a command and clears the modal.
	m := New().openPrompt("vault/x", true)
	m.promptInput.SetValue("vault/y")
	next, cmd := m.handlePromptKey(keyEnter())
	if cmd == nil || next.(Model).mode != modeBrowse {
		t.Fatalf("valid dst: want a command and modeBrowse, got cmd!=nil=%v mode=%v", cmd != nil, next.(Model).mode)
	}
}

// a long password/value must wrap, never overflow the detail pane (which would
// push the box's right border out of alignment).
func TestDetailWraps(t *testing.T) {
	m := New()
	m.width, m.height = 80, 24
	m.layout()
	m.tree.setEntries([]string{"x"})
	m.current = "x"
	m.detailRaw = "user: " + strings.Repeat("a", 500) // no spaces, must hard-wrap
	m.masked = false
	m.tree.toggle() // open the leaf so its detail inlines into the tree
	m.renderDetail()
	for _, ln := range strings.Split(m.tree.view(), "\n") {
		if w := lipgloss.Width(ln); w > m.tree.width {
			t.Fatalf("tree line width %d exceeds tree width %d", w, m.tree.width)
		}
	}
}

func TestParseFields(t *testing.T) {
	got := parseFields("s3cret\nuser: levi@x\nurl: https://x\nbare note")
	want := []field{
		{"password", "s3cret"},
		{"user", "levi@x"},
		{"url", "https://x"},
		{"", "bare note"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d fields, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
