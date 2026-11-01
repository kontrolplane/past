package tui

import (
	"image/color"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kontrolplane/past/internal/theme"
)

// Layout constants, shared with relay and tablespace so the tools read as one family.
const (
	minContentWidth  = 72 // Below this, or minContentHeight, a notice asks for a larger terminal
	minContentHeight = 12
	chromeWidth      = 2 // Frame borders
	chromeHeight     = 8 // Header, spacing, frame edges and padding, footer
	pagePadX         = 2 // Room between the frame and the page
)

// The content area fills the terminal. setLayout resizes it and everything derived from it.
var (
	contentWidth  = 120
	contentHeight = 24
	frameWidth    = contentWidth + chromeWidth
)

func setLayout(width, height int) {
	contentWidth = max(minContentWidth, width-chromeWidth)
	contentHeight = max(minContentHeight, height-chromeHeight)
	frameWidth = contentWidth + chromeWidth
}

// pageWidth is the room the page has inside the frame's padding.
func pageWidth() int { return contentWidth - 2*pagePadX }

// truncate cuts s to width columns, ending it in an ellipsis when it does not fit.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// spread places left and right on one line of the given width.
func spread(left, right string, width int) string {
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

// padRight fills s with spaces up to width, cutting it when it is wider.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width {
		s = truncate(s, width)
		w = lipgloss.Width(s)
	}
	return s + strings.Repeat(" ", max(0, width-w))
}

// fill paints n blank cells on bg, or on the terminal's ground when bg is nil.
func fill(n int, bg color.Color) string {
	if n <= 0 {
		return ""
	}
	if bg == nil {
		return strings.Repeat(" ", n)
	}
	return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", n))
}

// line renders spans into exactly width cells over bg, cutting the last that does not fit with an
// ellipsis, so a selected row is one unbroken band of colour.
func line(spans []theme.Span, width int, bg color.Color) string {
	var b strings.Builder
	used := 0
	for i, s := range spans {
		w := ansi.StringWidth(s.Text)
		if used+w > width || (used+w == width && i < len(spans)-1 && restWidth(spans[i+1:]) > 0) {
			s.Text = ansi.Truncate(s.Text, max(0, width-used-1), "") + "…"
			b.WriteString(s.On(bg))
			used = width
			break
		}
		b.WriteString(s.On(bg))
		used += w
	}
	b.WriteString(fill(width-used, bg))
	return b.String()
}

func restWidth(spans []theme.Span) int {
	w := 0
	for _, s := range spans {
		w += ansi.StringWidth(s.Text)
	}
	return w
}

// place centres s in an area of width and height, every line on its own as lipgloss.Place does,
// and cuts what does not fit.
func place(width, height int, s string) string {
	lines := strings.Split(s, "\n")
	lines = lines[:min(len(lines), height)]
	blank := strings.Repeat(" ", width)
	top := (height - len(lines)) / 2

	var b strings.Builder
	for i := range height {
		if i > 0 {
			b.WriteByte('\n')
		}
		j := i - top
		if j < 0 || j >= len(lines) {
			b.WriteString(blank)
			continue
		}
		l := lines[j]
		w := lipgloss.Width(l)
		if w > width {
			l = truncate(l, width)
			w = lipgloss.Width(l)
		}
		gap := width - w
		b.WriteString(blank[:gap/2] + l + blank[:gap-gap/2])
	}
	return b.String()
}

// clip cuts s to height lines, ending lines wider than width in an ellipsis.
func clip(s string, width, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:max(height, 0)]
	}
	for i, l := range lines {
		if lipgloss.Width(l) > width {
			lines[i] = truncate(l, width)
		}
	}
	return strings.Join(lines, "\n")
}

// frame draws the rounded border around the content area, with a title set into the top edge
// and meta text into the right of the top one and the bottom one.
func frame(title, meta, foot, body string) string {
	border := lipgloss.NewStyle().Foreground(theme.P.Rule)
	edge := func(n int) string { return border.Render(strings.Repeat("─", max(1, n))) }

	metaWidth := 0
	if meta != "" {
		meta = " " + truncate(meta, frameWidth/3) + " "
		metaWidth = lipgloss.Width(meta)
	}
	title = truncate(title, frameWidth-7-metaWidth)
	top := border.Render("╭─ ") + title + " " +
		edge(frameWidth-5-lipgloss.Width(title)-1-metaWidth) + meta + border.Render("─╮")

	footWidth := 0
	if foot != "" {
		foot = " " + truncate(foot, frameWidth-6) + " "
		footWidth = lipgloss.Width(foot)
	}
	bottom := border.Render("╰") + edge(frameWidth-3-footWidth) + foot + border.Render("─╯")

	lines := strings.Split(body, "\n")
	lines = lines[:min(len(lines), contentHeight)]
	side := border.Render("│")
	blank := side + strings.Repeat(" ", contentWidth) + side

	var b strings.Builder
	b.WriteString(top + "\n" + blank)
	for i := range contentHeight {
		b.WriteString("\n" + side)
		if i < len(lines) {
			b.WriteString(padRight(lines[i], contentWidth))
		} else {
			b.WriteString(strings.Repeat(" ", contentWidth))
		}
		b.WriteString(side)
	}
	b.WriteString("\n" + blank + "\n" + bottom)
	return b.String()
}

// indent sets every line of s pagePadX columns in from the frame.
func indent(s string) string {
	pad := strings.Repeat(" ", pagePadX)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

// dialogPadX is the room a dialog card leaves between its edge and its text.
const dialogPadX = 4

// dialogTextWidth is the widest a line of dialog text gets before it wraps.
func dialogTextWidth() int {
	return min(contentWidth-4-2*dialogPadX, 72)
}

// dialog renders a card with a toned edge and heading, to be set over the page with overlay.
// Lines wider than the card wrap, and the card gives up its inner margin when it would not fit
// the height.
func dialog(title string, tone theme.Tone, glyph string, body ...string) string {
	heading := theme.Render(theme.B(title, tone))
	if glyph != "" {
		heading = theme.Render(theme.B(glyph+" ", tone)) + heading
	}
	width := dialogTextWidth()
	lines := []string{heading, ""}
	for _, block := range body {
		for _, l := range strings.Split(block, "\n") {
			if lipgloss.Width(l) > width {
				l = lipgloss.Wrap(l, width, "")
			}
			lines = append(lines, l)
		}
	}
	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	padY := 1
	if lipgloss.Height(content)+2+2*padY > contentHeight {
		padY = 0
	}
	edge := theme.P.Color(tone)
	if tone == theme.ToneText {
		edge = theme.P.RuleBold
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(edge).
		Padding(padY, dialogPadX).
		Render(content)
}

// overlay sets card in the middle of the content area, over the page, which recedes behind it so
// it is still clear what the card is about.
func overlay(page, card string) string {
	lines := strings.Split(ansi.Strip(page), "\n")
	dim := lipgloss.NewStyle().Foreground(theme.P.Rule)
	for i, l := range lines {
		lines[i] = dim.Render(l)
	}
	// A ring of blank cells keeps the page's text off the card's edge.
	card = lipgloss.NewStyle().Padding(0, 1).Render(card)
	w, h := lipgloss.Width(card), lipgloss.Height(card)
	c := lipgloss.NewCanvas(contentWidth, contentHeight)
	c.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(strings.Join(lines, "\n")),
		lipgloss.NewLayer(card).X(max(0, (contentWidth-w)/2)).Y(max(0, (contentHeight-h)/2)).Z(1),
	))
	return c.Render()
}

// confirmButtons renders a no/yes button pair. selected 0 highlights "no", 1 highlights "yes".
func confirmButtons(selected int, tone theme.Tone) string {
	return lipgloss.JoinHorizontal(lipgloss.Center,
		theme.Button("no", selected == 0, theme.ToneText),
		"    ",
		theme.Button("yes", selected == 1, tone),
	)
}

// hints renders key hints separated by faint dots.
func hints(pairs ...[2]string) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = theme.Key(p[0], p[1])
	}
	return strings.Join(parts, theme.Faint("  ·  "))
}

// fitHints renders the hints that fit width. Hints are dropped from the end but for the last two,
// help and quit, which stay.
func fitHints(width int, pairs ...[2]string) string {
	pairs = slices.Clone(pairs)
	for len(pairs) > 2 && lipgloss.Width(hints(pairs...)) > width {
		pairs = slices.Delete(pairs, len(pairs)-3, len(pairs)-2)
	}
	return hints(pairs...)
}
