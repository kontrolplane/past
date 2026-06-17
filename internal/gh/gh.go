// Package gh wraps the GitHub CLI to provide past's github integration:
// provisioning a private store repo, listing secret-scanning alerts, and
// listing recent Actions runs.
package gh

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kontrolplane/past/internal/git"
	pgit "github.com/kontrolplane/past/internal/pass"
	"github.com/kontrolplane/past/internal/sh"
)

// Alert is one secret-scanning alert.
type Alert struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Type   string `json:"secret_type_display_name"`
}

// Run is one Actions workflow run.
type Run struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// Repo identifies the origin GitHub repository in owner/name form.
type Repo struct {
	Owner string
	Name  string
}

func (r Repo) Slug() string { return r.Owner + "/" + r.Name }

// OriginRepo derives owner/name from the store's `origin` remote url. It
// returns ok=false when there is no origin remote.
func OriginRepo() (Repo, bool) {
	out, err := gitConfig("config", "--get", "remote.origin.url")
	if err != nil {
		return Repo{}, false
	}
	return parseRemote(strings.TrimSpace(out))
}

// parseRemote handles both ssh (git@github.com:owner/name.git) and https
// (https://github.com/owner/name.git) forms.
func parseRemote(url string) (Repo, bool) {
	url = strings.TrimSuffix(url, ".git")
	var path string
	switch {
	case strings.HasPrefix(url, "git@"):
		if i := strings.Index(url, ":"); i >= 0 {
			path = url[i+1:]
		}
	case strings.Contains(url, "github.com/"):
		path = url[strings.Index(url, "github.com/")+len("github.com/"):]
	default:
		return Repo{}, false
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repo{}, false
	}
	return Repo{Owner: parts[0], Name: parts[1]}, true
}

// Provision initializes the store as a git repo (if needed) and creates a
// private GitHub repository from it, pushing the initial contents.
func Provision(name string) error {
	if err := git.Init(); err != nil {
		return err
	}
	_, err := run("repo", "create", name,
		"--private", "--source", pgit.StoreDir(), "--remote", "origin", "--push")
	return err
}

// SecretScanningAlerts returns open secret-scanning alerts for the repo.
func SecretScanningAlerts(r Repo) ([]Alert, error) {
	out, err := run("api",
		fmt.Sprintf("repos/%s/secret-scanning/alerts", r.Slug()),
		"--method", "GET", "-f", "state=open")
	if err != nil {
		return nil, err
	}
	var alerts []Alert
	if err := json.Unmarshal([]byte(out), &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

// Runs returns the most recent Actions workflow runs for the repo.
func Runs(r Repo, limit int) ([]Run, error) {
	out, err := run("run", "list", "--repo", r.Slug(),
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "name,status,conclusion,createdAt")
	if err != nil {
		return nil, err
	}
	var runs []Run
	if err := json.Unmarshal([]byte(out), &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

func run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Env = os.Environ()
	return sh.Run("gh "+strings.Join(args, " "), cmd)
}

func gitConfig(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", pgit.StoreDir()}, args...)...)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	return string(out), err
}
