// Package pass is a thin wrapper around the `pass` binary and the on-disk
// password store. It is the only layer that touches decrypted secrets, and it
// never re-implements crypto: every operation shells out to `pass`/`gpg`.
//
// Secrets are returned to callers as strings only for as long as the UI needs
// them and are never logged.
package pass

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kontrolplane/past/internal/sh"
)

// StoreDir resolves the password store directory, honouring PASSWORD_STORE_DIR
// and falling back to ~/.password-store.
func StoreDir() string {
	if d := os.Getenv("PASSWORD_STORE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".password-store"
	}
	return filepath.Join(home, ".password-store")
}

// Initialized reports whether the store directory exists and contains a .gpg-id.
func Initialized() bool {
	if _, err := os.Stat(filepath.Join(StoreDir(), ".gpg-id")); err != nil {
		return false
	}
	return true
}

// List walks the store tree and returns entry names (path minus the .gpg
// suffix), sorted. Parsing filenames directly is cheaper and more reliable than
// parsing the tree art emitted by `pass ls`.
func List() ([]string, error) {
	root := StoreDir()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// skip the git metadata directory.
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".gpg") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(rel, ".gpg")
		entries = append(entries, filepath.ToSlash(name))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	return entries, nil
}

// SecretKeyAvailable reports whether the local gpg keyring holds a secret key
// for at least one of the store's recipients (the ids listed in .gpg-id).
// Without one, `pass show` fails with "gpg: decryption failed: No secret key".
func SecretKeyAvailable() bool {
	data, err := os.ReadFile(filepath.Join(StoreDir(), ".gpg-id"))
	if err != nil {
		return false
	}
	for _, id := range strings.Fields(string(data)) {
		if exec.Command("gpg", "--list-secret-keys", id).Run() == nil {
			return true
		}
	}
	return false
}

// NoAutoCommitEnv, added to a pass child's environment, points git at a path
// that is not a repository so pass's git detection fails and it skips its
// built-in `git add` + `git commit`. The file is still written; past stages and
// commits explicitly from the UI instead. Callers running `pass` outside this
// package (e.g. `pass edit`) must add it too.
const NoAutoCommitEnv = "GIT_DIR=/dev/null"

// Passphrase, when non-empty, is the user's gpg key passphrase, supplied to gpg
// via loopback pinentry so past can unlock the key from its own prompt (pinentry
// can't grab the terminal while the TUI's alt-screen owns it).
//
// ponytail: this is the one place past holds a secret in its own memory — a
// deliberate departure from the "let gpg-agent own the passphrase" design,
// chosen for in-app unlock. It is set once per session and never persisted.
var Passphrase string

// CheckAuth verifies the gpg key can actually decrypt the store, by decrypting
// the first entry and discarding the plaintext. It returns nil when the store
// is empty or the key is already unlocked (agent-cached or Passphrase set), and
// the gpg error when the key is locked — which the UI turns into an unlock
// prompt before the user starts browsing.
func CheckAuth() error {
	entries, err := List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	_, err = runDecrypt("show", entries[0])
	return err
}

// Show returns the full decrypted contents of an entry.
func Show(name string) (string, error) {
	out, err := runDecrypt("show", name)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

// Password returns just the first line of an entry (the password by convention).
func Password(name string) (string, error) {
	full, err := Show(name)
	if err != nil {
		return "", err
	}
	if i := strings.IndexByte(full, '\n'); i >= 0 {
		return full[:i], nil
	}
	return full, nil
}

// Copy copies the entry's password to the clipboard via `pass -c`, so that pass
// owns the auto-clear timeout (PASSWORD_STORE_CLIP_TIME). past never touches the
// clipboard itself for the password.
func Copy(name string) error {
	_, err := runDecrypt("-c", name)
	return err
}

// CopyValue writes a non-secret field value (a username, url, …) straight to
// the system clipboard. Unlike Copy, which routes the password through `pass -c`
// for its auto-clear, this writes directly and does NOT auto-clear — it is only
// for the metadata fields the user explicitly picks, never the password.
func CopyValue(s string) error {
	cmd := clipCmd()
	if cmd == nil {
		return fmt.Errorf("no clipboard tool found (need pbcopy, wl-copy, xclip, or xsel)")
	}
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

// clipCmd returns the first available OS clipboard-copy command (stdin → board),
// or nil when none is installed.
func clipCmd() *exec.Cmd {
	for _, c := range [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "-b", "-i"},
	} {
		if _, err := exec.LookPath(c[0]); err == nil {
			return exec.Command(c[0], c[1:]...)
		}
	}
	return nil
}

// validName rejects names that pass's CLI would misparse as a flag or that
// could escape the store. pass's own check_sneaky_paths catches "..", but we
// reject at the boundary so the leading-dash / absolute cases fail clearly here
// rather than misfiring as a pass option. Covers TUI writes and `past import`.
func validName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("name required")
	case strings.HasPrefix(name, "-"):
		return fmt.Errorf("name %q may not start with '-'", name)
	case strings.HasPrefix(name, "/") || strings.Contains(name, ".."):
		return fmt.Errorf("name %q may not be absolute or contain '..'", name)
	}
	return nil
}

// Generate creates a new entry with a freshly generated password.
func Generate(name string, length int, symbols bool) error {
	if err := validName(name); err != nil {
		return err
	}
	args := []string{"generate"}
	if !symbols {
		args = append(args, "--no-symbols")
	}
	args = append(args, name, fmt.Sprintf("%d", length))
	_, err := run(nil, args...)
	return err
}

// Regenerate re-rolls an existing entry's password in place (pass generate
// --in-place), keeping the entry's other lines. This is the common "rotate this
// password" path; Generate is for brand-new entries.
func Regenerate(name string, length int, symbols bool) error {
	if err := validName(name); err != nil {
		return err
	}
	args := []string{"generate", "--in-place"}
	if !symbols {
		args = append(args, "--no-symbols")
	}
	args = append(args, name, fmt.Sprintf("%d", length))
	_, err := run(nil, args...)
	return err
}

// Rename moves an entry to a new path (pass mv). The destination may include
// folders, so this also covers moving an entry between folders.
func Rename(old, dst string) error {
	if err := validName(old); err != nil {
		return err
	}
	if err := validName(dst); err != nil {
		return err
	}
	_, err := run(nil, "mv", "--force", old, dst)
	return err
}

// Duplicate copies an entry to a new path (pass cp).
func Duplicate(old, dst string) error {
	if err := validName(old); err != nil {
		return err
	}
	if err := validName(dst); err != nil {
		return err
	}
	_, err := run(nil, "cp", "--force", old, dst)
	return err
}

// Insert writes a multi-line entry, replacing any existing one. The first line
// should be the password; subsequent lines are arbitrary metadata.
func Insert(name, content string) error {
	if err := validName(name); err != nil {
		return err
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	_, err := run(strings.NewReader(content), "insert", "--multiline", "--force", name)
	return err
}

// Remove deletes an entry.
func Remove(name string) error {
	_, err := run(nil, "rm", "--force", name)
	return err
}

// runDecrypt runs a decrypting pass command (show / -c). When Passphrase is set
// it adds loopback pinentry and feeds the passphrase on stdin (fd 0) — these
// commands don't otherwise use stdin, and gpg reads the ciphertext from the file
// argument, so fd 0 is free for the passphrase.
func runDecrypt(args ...string) (string, error) {
	env := os.Environ()
	var stdin io.Reader
	if Passphrase != "" {
		env = append(env, "PASSWORD_STORE_GPG_OPTS=--pinentry-mode loopback --passphrase-fd 0")
		stdin = strings.NewReader(Passphrase + "\n")
	}
	return runEnv(env, stdin, args...)
}

// run executes pass with explicit args (never a shell string, so no injection),
// optionally feeding stdin. PASSWORD_STORE_DIR is propagated from the
// environment automatically.
func run(stdin io.Reader, args ...string) (string, error) {
	return runEnv(os.Environ(), stdin, args...)
}

func runEnv(env []string, stdin io.Reader, args ...string) (string, error) {
	cmd := exec.Command("pass", args...)
	cmd.Env = append(env, NoAutoCommitEnv)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	return sh.Run("pass "+strings.Join(args, " "), cmd)
}
