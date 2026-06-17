package pass

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestList verifies the store-tree parser: it returns *.gpg files as slash-separated
// names (minus the suffix), sorted, skipping .git and non-.gpg files.
func TestList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PASSWORD_STORE_DIR", dir)

	for _, rel := range []string{
		"github/token.gpg",
		"email/personal.gpg",
		"wifi.gpg",
		".git/config",     // must be skipped
		"email/notes.txt", // not a .gpg, ignored
		".gpg-id",         // not a .gpg entry
	} {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"email/personal", "github/token", "wifi"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %v, want %v", got, want)
	}
}

func TestStoreDirEnv(t *testing.T) {
	t.Setenv("PASSWORD_STORE_DIR", "/tmp/custom-store")
	if got := StoreDir(); got != "/tmp/custom-store" {
		t.Fatalf("StoreDir() = %q, want /tmp/custom-store", got)
	}
}

// TestValidName guards the trust boundary: leading-dash names (which pass's
// getopt would treat as flags) and path-escaping names must be rejected, while
// ordinary folder/entry paths pass.
func TestValidName(t *testing.T) {
	bad := []string{"", "-c", "--help", "/etc/passwd", "../escape", "a/../../b"}
	for _, n := range bad {
		if err := validName(n); err == nil {
			t.Errorf("validName(%q) = nil, want error", n)
		}
	}
	good := []string{"wifi", "github/work", "email/personal", "a-b", "x_y.z"}
	for _, n := range good {
		if err := validName(n); err != nil {
			t.Errorf("validName(%q) = %v, want nil", n, err)
		}
	}
}
