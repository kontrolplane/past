// Package config loads past's user settings from a simple key=value dotfile,
// falling back to built-in defaults. Today it carries one setting (auto-commit);
// the format leaves room for more without a parser dependency.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is past's runtime settings.
type Config struct {
	// AutoCommit commits the store after every change made in the UI. On by
	// default; set "auto_commit = false" in the dotfile to stage only and commit
	// manually from the github menu.
	AutoCommit bool
}

// Default returns the built-in settings, used when the dotfile is absent or a
// key is unset.
func Default() Config {
	return Config{AutoCommit: true}
}

// Path is the config file location: $PAST_CONFIG, else
// $XDG_CONFIG_HOME/past/config, else ~/.config/past/config. Empty when no home
// can be resolved.
func Path() string {
	if p := os.Getenv("PAST_CONFIG"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "past", "config")
}

// Load reads the dotfile over the defaults. A missing file, unknown keys, and
// unparseable values are all ignored, so a stray line never stops past from
// launching.
func Load() Config {
	cfg := Default()
	path := Path()
	if path == "" {
		return cfg
	}
	f, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "auto_commit":
			if b, err := strconv.ParseBool(strings.TrimSpace(val)); err == nil {
				cfg.AutoCommit = b
			}
		}
	}
	return cfg
}
