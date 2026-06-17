// Package setup checks the tools and store past depends on and renders the
// result as a tree that unfolds, shown in place of a crash when a prerequisite
// is missing. It never re-implements crypto: it only probes $PATH and the store.
package setup

import (
	"os/exec"

	"github.com/kontrolplane/past/internal/pass"
)

// Step is a single prerequisite node in the setup tree. Optional steps (gh) do
// not block past from launching; their absence only disables github features.
type Step struct {
	Name     string
	OK       bool
	Optional bool
	Hint     string // install command / detail, shown when not OK
}

// Check probes every prerequisite and returns one Step each, in display order.
func Check() []Step {
	bins := []struct{ name, hint string }{
		{"pass", "brew install pass"},
		{"git", "usually preinstalled"},
		{"gpg", "brew install gnupg"},
	}
	var steps []Step
	for _, b := range bins {
		_, err := exec.LookPath(b.name)
		steps = append(steps, Step{Name: b.name, OK: err == nil, Hint: b.hint})
	}
	storeOK := pass.Initialized()
	steps = append(steps, Step{
		Name: "store",
		OK:   storeOK,
		Hint: "pass init <gpg-id> → " + pass.StoreDir(),
	})
	// only meaningful once the store exists (.gpg-id names the recipients).
	if storeOK {
		steps = append(steps, Step{
			Name: "key",
			OK:   pass.SecretKeyAvailable(),
			Hint: "no secret key for the store's gpg-id — import it (gpg --import)",
		})
	}
	_, err := exec.LookPath("gh")
	steps = append(steps, Step{
		Name:     "gh",
		OK:       err == nil,
		Optional: true,
		Hint:     "brew install gh (github features only)",
	})
	return steps
}

// Complete reports whether every required (non-optional) step is satisfied.
func Complete(steps []Step) bool {
	for _, s := range steps {
		if !s.OK && !s.Optional {
			return false
		}
	}
	return true
}
