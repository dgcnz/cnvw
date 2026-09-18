package render

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/geom"
	"github.com/dgcnz/cnvw/internal/mdline"
)

// NodeState carries the view's opinion of a node, separate from anything the
// document says about it.
type NodeState struct {
	Selected bool
	Matched  bool
}

// Box characters for a normal node and for the selected one. Selection is
// shown by thickening the border rather than by color alone, so it survives a
// node that already carries a strong color of its own.
var (
	roundBorder = [6]string{"╭", "╮", "╰", "╯", "─", "│"}
	thickBorder = [6]string{"┏", "┓", "┗", "┛", "━", "┃"}
)

// Type markers prefixed to the title of non-text nodes.
const (
	fileMarker = "▤ "
	linkMarker = "↗ "
)

// NodeBox renders a node as exactly r.H() lines of r.W() cells. Returning a
// fixed-size block is what lets the caller place it as an opaque layer that
// cleanly covers the edges routed underneath it.
func NodeBox(n *canvas.Node, r geom.Rect, th Theme, st NodeState) string {
	w, h := r.W(), r.H()
	if w <= 0 || h <= 0 {
		return ""
	}

	fg := th.Resolve(n.ParsedColor(), th.Border)
	if n.Type == canvas.TypeGroup {
		fg = th.Resolve(n.ParsedColor(), th.Group)
	}
	if st.Matched {
		fg = th.Match
	}

	tier := geom.TierFor(r)
	if n.Type == canvas.TypeGroup {
		tier = geom.TierForGroup(r)
		return groupBox(n, w, h, th, fg, st)
	}

	switch tier {
	case geom.TierHidden:
		return ""
	case geom.TierBlock:
		return blockBox(w, h, fg)
	case geom.TierMini:
		return miniBox(n, w, h, th, fg)
	default:
		return fullBox(n, w, h, th, fg, st, tier)
	}
}

// blockBox is the smallest representation: a solid mass of color that says
// only that something is there.
func blockBox(w, h int, fg color.Color) string {
	style := lipgloss.NewStyle().Foreground(fg)
	row := style.Render(strings.Repeat("█", w))
	rows := make([]string, h)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}

// miniBox drops the border, which at this size would consume every row
// available, and spends the space on the title instead.
//
// The title sits on a filled bar rather than under a drawn rule: a rule made
// of box characters is indistinguishable from an edge passing behind the node,
// which is exactly the confusion this size of node invites.
func miniBox(n *canvas.Node, w, h int, th Theme, fg color.Color) string {
	bar := lipgloss.NewStyle().Foreground(th.Bg).Background(fg).Bold(true)
	title := bar.Render(fitTo(" "+mdline.Truncate(marker(n)+n.Title(), w-1), w))

	// The bar goes on the row Anchor points at, not the first row. Otherwise
	// an edge into a two-row node arrives beside the title rather than at it.
	rows := make([]string, h)
	at := h / 2
	for i := range rows {
		rows[i] = strings.Repeat(" ", w)
	}
	rows[at] = title
	return strings.Join(rows, "\n")
}

// fullBox draws a bordered node. At the title tier only the first line is
// shown; at the full tier the body is wrapped into whatever rows remain.
func fullBox(n *canvas.Node, w, h int, th Theme, fg color.Color, st NodeState, tier geom.Tier) string {
	b := roundBorder
	if st.Selected {
		b = thickBorder
	}
	bs := lipgloss.NewStyle().Foreground(fg)
	if st.Selected {
		bs = bs.Bold(true)
	}

	// A single space of breathing room inside the border, but only when the
	// node is wide enough that giving up two cells still leaves useful text.
	padding := 0
	if w >= 10 {
		padding = 1
	}
	inner := w - 2 - padding*2
	rows := h - 2

	out := make([]string, 0, h)
	out = append(out, bs.Render(b[0]+strings.Repeat(b[4], w-2)+b[1]))

	for i, line := range nodeLines(n, th, fg, inner, rows, tier) {
		if i >= rows {
			break
		}
		pad := strings.Repeat(" ", padding)
		out = append(out, bs.Render(b[5])+pad+fitTo(line, inner)+pad+bs.Render(b[5]))
	}
	for len(out) < h-1 {
		out = append(out, bs.Render(b[5])+strings.Repeat(" ", w-2)+bs.Render(b[5]))
	}
	out = append(out, bs.Render(b[2]+strings.Repeat(b[4], w-2)+b[3]))
	return strings.Join(out[:h], "\n")
}

// nodeLines produces the styled inner lines of a node, at most rows of them.
func nodeLines(n *canvas.Node, th Theme, accent color.Color, width, rows int, tier geom.Tier) []string {
	if width < 1 || rows < 1 {
		return nil
	}

	// Below the full tier there is only ever room for a name.
	if tier < geom.TierFull {
		s := lipgloss.NewStyle().Foreground(th.Fg).Bold(true)
		return []string{s.Render(mdline.Truncate(marker(n)+n.Title(), width))}
	}

	switch n.Type {
	case canvas.TypeFile:
		out := []string{lipgloss.NewStyle().Foreground(th.Fg).Bold(true).
			Render(mdline.Truncate(fileMarker+n.File, width))}
		if n.Subpath != "" && rows > 1 {
			out = append(out, lipgloss.NewStyle().Foreground(th.Muted).
				Render(mdline.Truncate("  "+n.Subpath, width)))
		}
		return out

	case canvas.TypeLink:
		return wrapPlain(linkMarker+n.URL, width, rows,
			lipgloss.NewStyle().Foreground(th.Accent).Underline(true))
	}

	lines := mdline.Render(n.Text, width)
	out := make([]string, 0, min(len(lines), rows))
	for i, l := range lines {
		if i >= rows {
			// Mark that the node holds more than the box can show, so it is
			// clear the text was cut rather than simply ending there.
			if len(out) > 0 {
				out[len(out)-1] = fitTo(out[len(out)-1], width-1) +
					lipgloss.NewStyle().Foreground(th.Muted).Render("…")
			}
			break
		}
		out = append(out, styleLine(l, th, accent))
	}
	return out
}

// wrapPlain breaks unstyled text across rows with a single style throughout.
func wrapPlain(s string, width, rows int, style lipgloss.Style) []string {
	var out []string
	for len(s) > 0 && len(out) < rows {
		cut := ansi.Truncate(s, width, "")
		if cut == "" {
			break
		}
		out = append(out, style.Render(cut))
		s = s[len(cut):]
	}
	return out
}

// styleLine converts one formatted Markdown line into a styled string.
func styleLine(l mdline.Line, th Theme, accent color.Color) string {
	var b strings.Builder
	for _, sp := range l {
		b.WriteString(spanStyle(sp.Style, th, accent).Render(sp.Text))
	}
	return b.String()
}

// spanStyle maps the formatter's attributes onto terminal styling.
func spanStyle(s mdline.Style, th Theme, accent color.Color) lipgloss.Style {
	st := lipgloss.NewStyle().Foreground(th.Fg)
	if s.Has(mdline.Dim) {
		st = st.Foreground(th.Muted)
	}
	if s.Has(mdline.Accent) {
		st = st.Foreground(accent)
	}
	if s.Has(mdline.Code) {
		st = st.Foreground(th.Muted).Italic(false)
	}
	if s.Has(mdline.Bold) {
		st = st.Bold(true)
	}
	if s.Has(mdline.Italic) {
		st = st.Italic(true)
	}
	if s.Has(mdline.Underline) {
		st = st.Underline(true)
	}
	return st
}

// groupBox draws a group as an outline with its label set into the top border,
// leaving the interior untouched so the nodes it contains stay visible.
func groupBox(n *canvas.Node, w, h int, th Theme, fg color.Color, st NodeState) string {
	if w < 2 || h < 2 {
		return ""
	}
	bs := lipgloss.NewStyle().Foreground(fg)
	if st.Selected {
		bs = bs.Bold(true)
	}

	top := "╭"
	if label := n.Label; label != "" && w >= 8 {
		label = mdline.Truncate(label, w-6)
		top += "─ " + label + " "
		top += strings.Repeat("─", max(0, w-2-ansi.StringWidth(top)+1))
	} else {
		top += strings.Repeat("─", w-2)
	}
	top = ansi.Truncate(top, w-1, "") + "╮"

	out := []string{bs.Render(top)}
	for i := 1; i < h-1; i++ {
		// The interior is transparent by intent: a filled row here would
		// erase the child nodes that sit inside the group.
		out = append(out, bs.Render("│")+strings.Repeat(" ", w-2)+bs.Render("│"))
	}
	out = append(out, bs.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(out, "\n")
}

// marker is the glyph identifying a node's type in a one-line summary.
func marker(n *canvas.Node) string {
	switch n.Type {
	case canvas.TypeFile:
		return fileMarker
	case canvas.TypeLink:
		return linkMarker
	}
	return ""
}

// padTo right-pads a styled string to width cells, measuring the visible text
// rather than the escape sequences around it.
func padTo(s string, width int) string {
	if n := width - ansi.StringWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// fitTo forces a styled string to exactly width cells, padding or cutting as
// needed. Padding alone is not enough where the result has to leave room for a
// following glyph, or the row ends up wider than the box around it.
func fitTo(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > width {
		return ansi.Truncate(s, width, "")
	}
	return padTo(s, width)
}
