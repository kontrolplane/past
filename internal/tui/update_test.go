package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kontrolplane/past/internal/pass"
)

// step feeds one message through Update and returns the concrete Model.
func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func mkKey(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// TestErrMsgLockedOpensUnlock: a gpg "locked key" error with no passphrase yet
// should switch to the unlock prompt rather than just flashing.
func TestErrMsgLockedOpensUnlock(t *testing.T) {
	pass.Passphrase = "" // ensure the "no passphrase yet" branch
	m := New()
	m, _ = step(m, errMsg{errors.New("gpg: decryption failed: No secret key")})
	if m.mode != modeUnlock {
		t.Fatalf("mode = %v, want modeUnlock", m.mode)
	}
}

// TestErrMsgGenericFlashes: a non-lock error must surface as an error flash and
// leave the user in browse mode (not stuck on the pre-auth gate).
func TestErrMsgGenericFlashes(t *testing.T) {
	pass.Passphrase = ""
	m := New()
	m, _ = step(m, errMsg{errors.New("pass mv: some other failure")})
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v, want modeBrowse", m.mode)
	}
	if !m.isErr || m.message == "" {
		t.Fatalf("expected error flash, got isErr=%v message=%q", m.isErr, m.message)
	}
	if !m.ready {
		t.Fatal("expected ready=true so the gate is released")
	}
}

// TestPromptRejectsEmptyDest: confirming an empty/identity destination flashes
// and does not fire a rename command.
func TestPromptRejectsEmptyDest(t *testing.T) {
	m := New()
	m = m.openPrompt("github/work", false)
	m.promptInput.SetValue("   ") // trims to empty
	m, cmd := step(m, mkKey("enter"))
	if cmd != nil {
		t.Fatal("empty destination should not return a command")
	}
	if m.mode != modeBrowse || m.message == "" {
		t.Fatalf("want browse + flash, got mode=%v message=%q", m.mode, m.message)
	}
}

// TestPromptAcceptsDest: a real destination returns a (rename) command.
func TestPromptAcceptsDest(t *testing.T) {
	m := New()
	m = m.openPrompt("github/work", false)
	m.promptInput.SetValue("github/personal")
	_, cmd := step(m, mkKey("enter"))
	if cmd == nil {
		t.Fatal("valid destination should return a rename command")
	}
}

// TestConfirmDeleteOnlyOnY: any key other than 'y' cancels the delete.
func TestConfirmDeleteOnlyOnY(t *testing.T) {
	m := New()
	m.mode = modeConfirmDelete
	m.current = "github/work"
	m, cmd := step(m, mkKey("n"))
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v, want modeBrowse", m.mode)
	}
	if cmd != nil {
		t.Fatal("a non-'y' key must not return a delete command")
	}

	m = New()
	m.mode = modeConfirmDelete
	m.current = "github/work"
	m, cmd = step(m, mkKey("y"))
	if cmd == nil {
		t.Fatal("'y' should return a delete command")
	}
	if m.current != "" {
		t.Fatalf("current = %q, want cleared after delete", m.current)
	}
}

// TestWindowSizeTiny: a 1x1 resize must populate dimensions without panicking.
func TestWindowSizeTiny(t *testing.T) {
	m := New()
	m, _ = step(m, tea.WindowSizeMsg{Width: 1, Height: 1})
	if m.width != 1 || m.height != 1 {
		t.Fatalf("size = %dx%d, want 1x1", m.width, m.height)
	}
}
