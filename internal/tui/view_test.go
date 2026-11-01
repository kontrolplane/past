package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func keyEnter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }

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
// push the frame's right border out of alignment).
func TestDetailWraps(t *testing.T) {
	m := sized(t, 100, 30)
	m.tree.setEntries([]string{"x"})
	m.ready = true
	m.current = "x"
	m.detailRaw = strings.Repeat("p", 300) + "\nuser: " + strings.Repeat("a", 500) // no spaces, must hard-wrap
	m.masked = false
	assertFits(t, m)
}

// every screen, dialogs and help included, must fit the terminal exactly.
func TestScreensFit(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 60}, {40, 10}} {
		m := sized(t, size[0], size[1])
		m.ready = true
		m.tree.setEntries([]string{"bank/amex", "bank/visa", "cloud/aws", "email", strings.Repeat("long/", 30) + "leaf"})
		m.tree.down()
		m.current = "bank/amex"
		m.detailRaw = "hunter2\nusername: levi\nurl: https://example.com\na note"
		assertFits(t, m)
		m.showHelp = true
		assertFits(t, m)
		m.showHelp = false
		for _, md := range []mode{modeCreate, modeConfirmDelete, modeGithub, modeUnlock, modePrompt, modeCommit, modeCopy} {
			m.mode = md
			m.genPicking = true
			assertFits(t, m)
		}
		m.mode = modeCreate
		m.startForm(2)
		assertFits(t, m)
	}
}

func sized(t *testing.T, w, h int) Model {
	t.Helper()
	m, _ := step(New(), tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func assertFits(t *testing.T, m Model) {
	t.Helper()
	lines := strings.Split(m.render(), "\n")
	if len(lines) != m.height {
		t.Fatalf("%dx%d mode %d: %d lines, want %d", m.width, m.height, m.mode, len(lines), m.height)
	}
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w != m.width {
			t.Fatalf("%dx%d mode %d: line %d is %d wide, want %d: %q", m.width, m.height, m.mode, i, w, m.width, ln)
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
