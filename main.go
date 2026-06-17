// past is a terminal ui for the `pass` password manager with github integration.
//
// it shells out to the user's existing pass/git/gpg/gh installs and never
// re-implements crypto or stores secrets itself.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kontrolplane/past/internal/config"
	"github.com/kontrolplane/past/internal/git"
	"github.com/kontrolplane/past/internal/importer"
	"github.com/kontrolplane/past/internal/pass"
	"github.com/kontrolplane/past/internal/setup"
	"github.com/kontrolplane/past/internal/tui"
)

func main() {
	// non-interactive subcommand: `past import <file.csv|file.xml>`.
	if len(os.Args) > 1 && os.Args[1] == "import" {
		if err := runImport(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "past import:", err)
			os.Exit(1)
		}
		return
	}

	// when a prerequisite is missing, show the unfolding setup tree (no
	// alt-screen, so it stays on screen after quit) instead of launching.
	if steps := setup.Check(); !setup.Complete(steps) {
		if _, err := tea.NewProgram(setup.New(steps)).Run(); err != nil {
			fmt.Fprintln(os.Stderr, "past:", err)
		}
		os.Exit(1)
	}

	git.AutoCommit = config.Load().AutoCommit

	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "past:", err)
		os.Exit(1)
	}
}

// runImport reads a CSV/XML export and writes each record as a pass entry
// (replacing any existing one of the same name). Like every past write it does
// not auto-commit; stage and commit the new entries from the UI afterwards.
func runImport(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: past import <file.csv|file.xml>")
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	entries, err := importer.Parse(args[0], f)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := pass.Insert(e.Name, e.Content()); err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
	}
	fmt.Printf("imported %d entries\n", len(entries))
	return nil
}
