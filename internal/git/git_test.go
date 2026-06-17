package git

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestCommitGuards: Commit must be a no-op when nothing is staged (no empty
// commits), and must record exactly one commit once a change is staged.
func TestCommitGuards(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PASSWORD_STORE_DIR", dir)
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t",
		"GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t",
	} {
		t.Setenv(k, v)
	}
	mustGit(t, dir, "init")
	write(t, dir, "seed", "x")
	mustGit(t, dir, "add", "-A")
	mustGit(t, dir, "commit", "-m", "seed")

	before := count(t, dir)
	if err := Commit("noop"); err != nil {
		t.Fatal(err)
	}
	if got := count(t, dir); got != before {
		t.Fatalf("empty commit created: %d -> %d", before, got)
	}

	write(t, dir, "a.gpg", "y")
	if err := Stage(); err != nil {
		t.Fatal(err)
	}
	if err := Commit("add a"); err != nil {
		t.Fatal(err)
	}
	if got := count(t, dir); got != before+1 {
		t.Fatalf("expected one new commit, got %d -> %d", before, got)
	}
}

func write(t *testing.T, dir, name, body string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, dir string) int {
	out, err := git(dir, "rev-list", "--count", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

func mustGit(t *testing.T, dir string, args ...string) {
	if _, err := git(dir, args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}
