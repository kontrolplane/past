package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kontrolplane/past/internal/gh"
	"github.com/kontrolplane/past/internal/git"
	"github.com/kontrolplane/past/internal/pass"
)

// kv is one `key: value` metadata line for a created entry.
type kv struct{ k, v string }

// async result messages.
type (
	entriesMsg     []string
	detailMsg      struct{ name, content string }
	clearDetailMsg struct{}
	statusMsg      git.SyncStatus
	copiedMsg      struct {
		name   string
		clears bool // password copy auto-clears; a field value does not
	}
	syncDoneMsg  struct{}
	provDoneMsg  struct{ name string }
	alertsMsg    []gh.Alert
	runsMsg      []gh.Run
	editDoneMsg  struct{ err error }
	movedMsg     struct{ name string }
	regenMsg     struct{ name string }
	committedMsg struct{}
	changedMsg   struct{} // store mutated: reload entries + sync status
	authOKMsg    struct{}
	errMsg       struct{ err error }
)

// checkAuth probes decryption at startup so a locked key opens the unlock
// prompt before the store is browsed, rather than only when a leaf is selected.
func checkAuth() tea.Msg {
	if err := pass.CheckAuth(); err != nil {
		return errMsg{err}
	}
	return authOKMsg{}
}

func loadEntries() tea.Msg {
	entries, err := pass.List()
	if err != nil {
		return errMsg{err}
	}
	return entriesMsg(entries)
}

func loadDetail(name string) tea.Cmd {
	return func() tea.Msg {
		content, err := pass.Show(name)
		if err != nil {
			return errMsg{err}
		}
		return detailMsg{name: name, content: content}
	}
}

func loadStatus() tea.Msg {
	s, err := git.Status()
	if err != nil {
		return errMsg{err}
	}
	return statusMsg(s)
}

func copyEntry(name string) tea.Cmd {
	return func() tea.Msg {
		if err := pass.Copy(name); err != nil {
			return errMsg{err}
		}
		return copiedMsg{name: name + " password", clears: true}
	}
}

// copyField copies a non-secret field value straight to the clipboard (no
// auto-clear). The password is never copied this way — it goes through copyEntry.
func copyField(name string, f field) tea.Cmd {
	return func() tea.Msg {
		if err := pass.CopyValue(f.val); err != nil {
			return errMsg{err}
		}
		field := f.key
		if field == "" {
			field = "value"
		}
		return copiedMsg{name: name + " " + field, clears: false}
	}
}

// createEntry builds a pass entry from the preset form. When genPw is set, it
// generates the password via pass and reads it back; otherwise it uses the
// typed secret. Metadata becomes `key: value` lines under the secret. A second
// write only happens when there's metadata (or a typed secret) to add, so a
// plain generated login stays a single `pass generate`.
func createEntry(name string, genPw bool, length int, symbols bool, secret string, metas []kv) tea.Cmd {
	return func() tea.Msg {
		pw := secret
		if genPw {
			if err := pass.Generate(name, length, symbols); err != nil {
				return errMsg{err}
			}
			got, err := pass.Password(name)
			if err != nil {
				return errMsg{err}
			}
			pw = got
		}
		if len(metas) > 0 || !genPw {
			var b strings.Builder
			b.WriteString(pw + "\n")
			for _, m := range metas {
				b.WriteString(m.k + ": " + m.v + "\n")
			}
			if err := pass.Insert(name, b.String()); err != nil {
				return errMsg{err}
			}
		}
		if err := git.Record("past: add " + name); err != nil {
			return errMsg{err}
		}
		return changedMsg{}
	}
}

func renameEntry(src, dst string, cp bool) tea.Cmd {
	return func() tea.Msg {
		op, verb := pass.Rename, "move"
		if cp {
			op, verb = pass.Duplicate, "copy"
		}
		if err := op(src, dst); err != nil {
			return errMsg{err}
		}
		if err := git.Record("past: " + verb + " " + src + " to " + dst); err != nil {
			return errMsg{err}
		}
		return movedMsg{dst}
	}
}

func regenEntry(name string, length int, symbols bool) tea.Cmd {
	return func() tea.Msg {
		if err := pass.Regenerate(name, length, symbols); err != nil {
			return errMsg{err}
		}
		if err := git.Record("past: regenerate " + name); err != nil {
			return errMsg{err}
		}
		return regenMsg{name}
	}
}

func removeEntry(name string) tea.Cmd {
	return func() tea.Msg {
		if err := pass.Remove(name); err != nil {
			return errMsg{err}
		}
		if err := git.Record("past: remove " + name); err != nil {
			return errMsg{err}
		}
		return changedMsg{}
	}
}

// recordEdit stages (and, with auto-commit on, commits) the working tree after
// an external `pass edit`, then refreshes the sync status.
func recordEdit(name string) tea.Cmd {
	return func() tea.Msg {
		if err := git.Record("past: edit " + name); err != nil {
			return errMsg{err}
		}
		return loadStatus()
	}
}

// commitStore commits the staged changes (git add already ran per-op). An empty
// message falls back to a generic one.
func commitStore(msg string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(msg) == "" {
			msg = "past: update store"
		}
		if err := git.Commit(msg); err != nil {
			return errMsg{err}
		}
		return committedMsg{}
	}
}

func syncStore() tea.Msg {
	if err := git.Sync(); err != nil {
		return errMsg{err}
	}
	return syncDoneMsg{}
}

func provision(name string) tea.Cmd {
	return func() tea.Msg {
		if err := gh.Provision(name); err != nil {
			return errMsg{err}
		}
		return provDoneMsg{name}
	}
}

func loadAlerts(r gh.Repo) tea.Cmd {
	return func() tea.Msg {
		a, err := gh.SecretScanningAlerts(r)
		if err != nil {
			return errMsg{err}
		}
		return alertsMsg(a)
	}
}

func loadRuns(r gh.Repo) tea.Cmd {
	return func() tea.Msg {
		runs, err := gh.Runs(r, 10)
		if err != nil {
			return errMsg{err}
		}
		return runsMsg(runs)
	}
}
