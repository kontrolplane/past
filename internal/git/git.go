// Package git reports sync status for the password store and drives sync
// through `pass git`, which passes through to git inside the store directory.
package git

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kontrolplane/past/internal/pass"
	"github.com/kontrolplane/past/internal/sh"
)

// SyncStatus is the snapshot rendered in the status bar.
type SyncStatus struct {
	HasRepo   bool // store is a git repository
	HasRemote bool // an upstream is configured
	Ahead     int  // commits local is ahead of upstream
	Behind    int  // commits local is behind upstream
	Dirty     bool // uncommitted changes present
}

// Status inspects the store's git state. A non-repo store yields a zero value
// with HasRepo=false rather than an error.
func Status() (SyncStatus, error) {
	dir := pass.StoreDir()
	var s SyncStatus

	if _, err := os.Stat(dir + "/.git"); err != nil {
		return s, nil
	}
	s.HasRepo = true

	if out, err := git(dir, "status", "--porcelain"); err == nil {
		s.Dirty = strings.TrimSpace(out) != ""
	}

	// upstream presence: rev-parse @{u} fails when no upstream is set.
	if _, err := git(dir, "rev-parse", "--abbrev-ref", "@{u}"); err != nil {
		return s, nil
	}
	s.HasRemote = true

	if out, err := git(dir, "rev-list", "--left-right", "--count", "@{u}...HEAD"); err == nil {
		fields := strings.Fields(strings.TrimSpace(out))
		if len(fields) == 2 {
			s.Behind, _ = strconv.Atoi(fields[0])
			s.Ahead, _ = strconv.Atoi(fields[1])
		}
	}
	return s, nil
}

// AutoCommit controls whether Record commits after staging. On by default; set
// from config at startup.
var AutoCommit = true

// Record stages all working-tree changes and, when AutoCommit is set, commits
// them under msg. With AutoCommit off it only stages, leaving the commit to the
// user (github menu). A non-repo store makes both steps no-ops.
func Record(msg string) error {
	if err := Stage(); err != nil {
		return err
	}
	if !AutoCommit {
		return nil
	}
	return Commit(msg)
}

// Stage adds all working-tree changes to the index (git add -A) without
// committing. pass's own auto-commit is disabled, so each mutating op stages
// here and the user commits explicitly. A non-repo store is a no-op.
func Stage() error {
	dir := pass.StoreDir()
	if _, err := os.Stat(dir + "/.git"); err != nil {
		return nil
	}
	_, err := git(dir, "add", "-A")
	return err
}

// Commit records the staged changes under msg. It is a no-op when the store is
// not a repo or nothing is staged (so the commit key never makes empty commits).
func Commit(msg string) error {
	dir := pass.StoreDir()
	if _, err := os.Stat(dir + "/.git"); err != nil {
		return nil
	}
	// `diff --cached --quiet` exits 0 when the index matches HEAD (nothing
	// staged), non-zero when there are staged changes to commit.
	if _, err := git(dir, "diff", "--cached", "--quiet"); err == nil {
		return nil
	}
	_, err := git(dir, "commit", "-m", msg)
	return err
}

// Init makes the store a git repository via `pass git init`, unless it already
// is one.
func Init() error {
	if _, err := os.Stat(pass.StoreDir() + "/.git"); err == nil {
		return nil
	}
	_, err := passGit("init")
	return err
}

// Sync pulls (rebasing) then pushes, via `pass git` so pass's git wrapper and
// hooks are respected.
func Sync() error {
	if _, err := passGit("pull", "--rebase"); err != nil {
		return err
	}
	if _, err := passGit("push"); err != nil {
		return err
	}
	return nil
}

func passGit(args ...string) (string, error) {
	cmd := exec.Command("pass", append([]string{"git"}, args...)...)
	cmd.Env = os.Environ()
	return sh.Run("pass git "+strings.Join(args, " "), cmd)
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	return string(out), err
}
