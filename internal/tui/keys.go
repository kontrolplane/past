package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the browse-mode keymap (lowercase, vim-ish).
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Filter key.Binding
	Show   key.Binding
	Reveal key.Binding
	Copy   key.Binding
	Gen    key.Binding
	Regen  key.Binding
	Add    key.Binding
	Edit   key.Binding
	Rename key.Binding
	Dup    key.Binding
	Delete key.Binding
	Sync   key.Binding
	Github key.Binding
	Unlock key.Binding
	Help   key.Binding
	Quit   key.Binding
}

var keys = keyMap{
	Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "move selection up")),
	Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "move selection down")),
	Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter entries by name")),
	Show:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "reveal password / fold folder")),
	Reveal: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "reveal password (same as enter)")),
	Copy:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy a field (password, or pick one)")),
	Gen:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "new login with generated password")),
	Regen:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "regenerate this entry's password")),
	Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add an entry (pick a type)")),
	Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit entry in $EDITOR")),
	Rename: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename / move entry")),
	Dup:    key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "duplicate entry")),
	Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete entry")),
	Sync:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync store (pull --rebase + push)")),
	Github: key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "github menu (commit, sync, repo)")),
	Unlock: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unlock the gpg key")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle this help")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// helpGroup is one labelled column in the ? popover.
type helpGroup struct {
	title string
	keys  []key.Binding
}

// helpGroups lists the bindings grouped by what they do, for the ? popover.
func (k keyMap) helpGroups() []helpGroup {
	return []helpGroup{
		{"navigate", []key.Binding{k.Up, k.Down, k.Filter, k.Show, k.Reveal}},
		{"entries", []key.Binding{k.Copy, k.Gen, k.Regen, k.Add, k.Edit, k.Rename, k.Dup, k.Delete}},
		{"store", []key.Binding{k.Sync, k.Github, k.Unlock}},
		{"app", []key.Binding{k.Help, k.Quit}},
	}
}
