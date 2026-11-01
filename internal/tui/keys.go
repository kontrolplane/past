package tui

// helpSection is one column of the key reference.
type helpSection struct {
	title string
	rows  [][2]string
}

// helpSections lists the keys grouped by what they act on, for the ? card.
var helpSections = []helpSection{
	{"navigate", [][2]string{
		{"↑/k ↓/j", "move"},
		{"←/→", "fold / unfold"},
		{"pgup/pgdn", "page"},
		{"home/end", "first / last"},
		{"enter", "reveal / fold"},
		{"/", "filter"},
	}},
	{"entries", [][2]string{
		{"c", "copy a field"},
		{"a", "add (pick a type)"},
		{"g", "add a login"},
		{"p", "new password"},
		{"e", "edit in $EDITOR"},
		{"r", "rename / move"},
		{"y", "duplicate"},
		{"d", "delete"},
	}},
	{"store", [][2]string{
		{"s", "pull and push"},
		{"h", "github"},
		{"u", "unlock gpg key"},
		{"?", "help"},
		{"q", "quit"},
	}},
	{"dialogs", [][2]string{
		{"tab", "next field"},
		{"y / n", "yes / no"},
		{"←/→", "choose"},
		{"enter", "confirm"},
		{"esc", "cancel"},
	}},
}
