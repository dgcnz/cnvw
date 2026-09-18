package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/mathtex"
	"github.com/dgcnz/cnvw/internal/mdline"
	"github.com/dgcnz/cnvw/internal/render"
)

// maxPreviewBytes caps how much of a file node's target is read. A canvas can
// point at anything, and the pane only ever shows a screenful at a time.
const maxPreviewBytes = 512 << 10

// focusState is the scrollable reader opened over a single node. This is where
// a node's full text lives: no box on the canvas can show a long note, and
// this is the one place with enough width to justify real Markdown rendering.
type focusState struct {
	title string
	lines []string
	off   int
}

// newFocus builds the reader for a node at the given content width.
func newFocus(n *canvas.Node, doc *canvas.Canvas, vault, canvasPath string, th render.Theme, width int) *focusState {
	f := &focusState{title: n.Title()}
	switch n.Type {
	case canvas.TypeText:
		f.lines = markdownLines(n.Text, th, width)
	case canvas.TypeFile:
		f.lines = fileLines(n, vault, canvasPath, th, width)
	case canvas.TypeLink:
		f.lines = []string{
			lipgloss.NewStyle().Foreground(th.Accent).Underline(true).Render(n.URL),
		}
	case canvas.TypeGroup:
		f.lines = groupLines(n, doc, th)
	}
	if len(f.lines) == 0 {
		f.lines = []string{lipgloss.NewStyle().Foreground(th.Muted).Render("(empty)")}
	}
	return f
}

// markdownLines renders Markdown with Glamour, which is worth its cost here
// where the pane is wide, unlike inside a canvas node.
func markdownLines(md string, th render.Theme, width int) []string {
	// Glamour is goldmark-based and knows neither LaTeX math nor Obsidian's
	// wiki links, so both are resolved before it ever sees the source.
	md = mdline.ResolveWikiLinks(mathtex.Render(md))

	style := "dark"
	if th.Name == "light" {
		style = "light"
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return strings.Split(md, "\n")
	}
	out, err := r.Render(md)
	if err != nil {
		return strings.Split(md, "\n")
	}
	return strings.Split(strings.TrimRight(out, "\n"), "\n")
}

// fileLines previews the target of a file node, or explains why it cannot.
func fileLines(n *canvas.Node, vault, canvasPath string, th render.Theme, width int) []string {
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	path, ok := ResolveFile(n.File, vault, canvasPath)
	head := []string{muted.Render(path), ""}
	if !ok {
		return append(head, lipgloss.NewStyle().Foreground(th.Match).Render("file not found"))
	}

	st, err := os.Stat(path)
	if err != nil {
		return append(head, lipgloss.NewStyle().Foreground(th.Match).Render(err.Error()))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return append(head, lipgloss.NewStyle().Foreground(th.Match).Render(err.Error()))
	}
	truncated := false
	if len(data) > maxPreviewBytes {
		data, truncated = data[:maxPreviewBytes], true
	}

	if isBinary(data) {
		return append(head, muted.Render(fmt.Sprintf("binary file, %d bytes", st.Size())))
	}

	var body []string
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".md" || ext == ".markdown" {
		body = markdownLines(string(data), th, width)
	} else {
		body = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	}
	if truncated {
		body = append(body, "", muted.Render("… preview truncated"))
	}
	return append(head, body...)
}

// groupLines lists what a group spatially contains, which is the only thing a
// group really has to say.
func groupLines(g *canvas.Node, doc *canvas.Canvas, th render.Theme) []string {
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	var out []string
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		if n.ID == g.ID || !contains(g, n) {
			continue
		}
		out = append(out, fmt.Sprintf("%s %s",
			muted.Render(typeLabel(n.Type)), n.Title()))
	}
	if len(out) == 0 {
		return []string{muted.Render("(empty group)")}
	}
	return append([]string{muted.Render(fmt.Sprintf("%d nodes", len(out))), ""}, out...)
}

// contains reports whether the group's rectangle covers the node's center.
func contains(g, n *canvas.Node) bool {
	cx, cy := n.CenterX(), n.CenterY()
	return cx >= float64(g.X) && cx <= float64(g.Right()) &&
		cy >= float64(g.Y) && cy <= float64(g.Bottom())
}

func typeLabel(t string) string {
	switch t {
	case canvas.TypeFile:
		return "file "
	case canvas.TypeLink:
		return "link "
	case canvas.TypeGroup:
		return "group"
	}
	return "text "
}

// scroll moves the viewport by delta lines, stopping at the ends.
func (f *focusState) scroll(delta, height int) {
	f.off += delta
	if maxOff := len(f.lines) - height; f.off > maxOff {
		f.off = maxOff
	}
	if f.off < 0 {
		f.off = 0
	}
}

// view returns the visible slice of the rendered content.
func (f *focusState) view(height int) []string {
	if f.off >= len(f.lines) {
		return nil
	}
	end := min(f.off+height, len(f.lines))
	return f.lines[f.off:end]
}
