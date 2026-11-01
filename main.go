// past is a terminal ui for the `pass` password manager with github integration.
//
// it shells out to the user's existing pass/git/gpg/gh installs and never
// re-implements crypto or stores secrets itself.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kontrolplane/past/internal/config"
	"github.com/kontrolplane/past/internal/git"
	"github.com/kontrolplane/past/internal/importer"
	"github.com/kontrolplane/past/internal/pass"
	"github.com/kontrolplane/past/internal/setup"
	"github.com/kontrolplane/past/internal/theme"
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

	fs := flag.NewFlagSet("past", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: past [--theme auto|dark|light]")
		fmt.Fprintln(os.Stderr, "       past import <file.csv|file.xml>")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	themeName := fs.String("theme", "auto", "colour theme: auto follows the terminal, dark or light also paint the background")
	_ = fs.Parse(os.Args[1:])
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "past: unexpected argument %q (see past --help)\n", fs.Arg(0))
		os.Exit(2)
	}
	switch *themeName {
	case "dark":
		theme.Use(true)
		theme.Paint = true
	case "light":
		theme.Use(false)
		theme.Paint = true
	case "auto":
		theme.Use(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))
	default:
		fmt.Fprintf(os.Stderr, "past: unknown theme %q, use auto, dark or light\n", *themeName)
		os.Exit(2)
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

	p := tea.NewProgram(tui.New())
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
