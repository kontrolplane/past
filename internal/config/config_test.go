package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// no file → defaults (auto-commit on).
	t.Setenv("PAST_CONFIG", filepath.Join(t.TempDir(), "missing"))
	if got := Load(); !got.AutoCommit {
		t.Fatalf("missing file: AutoCommit = false, want true (default)")
	}

	// a dotfile with comments, blanks, and an override turns auto-commit off.
	path := filepath.Join(t.TempDir(), "config")
	body := "# past config\n\nauto_commit = false\nunknown = 1\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAST_CONFIG", path)
	if got := Load(); got.AutoCommit {
		t.Fatalf("auto_commit=false not applied: AutoCommit = true")
	}
}
