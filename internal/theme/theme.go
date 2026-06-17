// Package theme holds past's terminal styles, following the kontrolplane.dev
// palette: a navy/cream base with gold, green, and blue accents.
package theme

import "github.com/charmbracelet/lipgloss"

// kontrolplane.dev palette.
const (
	cream = "#FFF1CC"
	navy  = "#14325C"
	gold  = "#F5C76A"
	green = "#8EE39E"
	tan   = "#9A917A"
	red   = "#F58A8A"
	blue  = "#8FB4E8"
)

var (
	// Title is emphasised heading text.
	Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cream))

	// Badge is the inverse pill used for the app/section title: cream on navy.
	Badge = lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(cream)).
		Background(lipgloss.Color(navy)).
		Padding(0, 1)

	// Selected is the highlighted row: cream on navy, matching the badge.
	Selected = lipgloss.NewStyle().
			Foreground(lipgloss.Color(cream)).
			Background(lipgloss.Color(navy))

	// Item is an unselected leaf row (a stored entry), in host blue.
	Item = lipgloss.NewStyle().Foreground(lipgloss.Color(blue))

	// Label is dimmed metadata.
	Label = lipgloss.NewStyle().Foreground(lipgloss.Color(tan))

	// Accent carries gold emphasis (folders, headings, branches).
	Accent = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(gold))

	// Rule is the dim tree connector / hairline.
	Rule = lipgloss.NewStyle().Foreground(lipgloss.Color(tan))

	// Success / Error in the palette's green / red.
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color(green))
	Error   = lipgloss.NewStyle().Foreground(lipgloss.Color(red))

	// Body is default detail text, in cream.
	Body = lipgloss.NewStyle().Foreground(lipgloss.Color(cream))
)
