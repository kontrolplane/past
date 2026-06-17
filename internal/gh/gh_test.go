package gh

import "testing"

func TestParseRemote(t *testing.T) {
	cases := []struct {
		url         string
		ok          bool
		owner, name string
	}{
		{"git@github.com:kontrolplane/past.git", true, "kontrolplane", "past"},
		{"https://github.com/kontrolplane/past.git", true, "kontrolplane", "past"},
		{"https://github.com/kontrolplane/past", true, "kontrolplane", "past"},
		{"ssh://git@github.com/owner/repo.git", true, "owner", "repo"},
		{"https://gitlab.com/owner/repo.git", false, "", ""},
		{"not-a-url", false, "", ""},
	}
	for _, c := range cases {
		r, ok := parseRemote(c.url)
		if ok != c.ok {
			t.Errorf("parseRemote(%q) ok = %v, want %v", c.url, ok, c.ok)
			continue
		}
		if ok && (r.Owner != c.owner || r.Name != c.name) {
			t.Errorf("parseRemote(%q) = %s/%s, want %s/%s", c.url, r.Owner, r.Name, c.owner, c.name)
		}
	}
}
