// Package sh runs an external command and turns a failure into an error that
// carries the command's stderr — the one piece every past shell-out shares.
package sh

import (
	"fmt"
	"os/exec"
	"strings"
)

// Run executes cmd, returning its stdout. On failure it returns an error of the
// form "<label>: <stderr>" (falling back to the exec error when stderr is empty).
// The caller builds cmd so it controls the binary, env, and stdin.
func Run(label string, cmd *exec.Cmd) (string, error) {
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s: %s", label, msg)
	}
	return string(out), nil
}
