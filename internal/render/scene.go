package render

import (
	"image/color"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/geom"
)

// View is everything the renderer needs to know that is not in the document.
type View struct {
	Cam        geom.Camera
	VP         geom.Viewport
	SelectedID string
	Matches    map[string]bool
	EdgeLabels bool
}

// cullPad is how far outside the viewport, in cells, a node is still laid out.
// Keeping a margin means a node straddling the edge is drawn and clipped
// rather than popping in once its top-left corner arrives.
const cullPad = 4

// Scene holds the per-render layout so the view and the input handling agree
// on exactly where each node ended up.
type Scene struct {
	Rects   map[string]geom.Rect
	Visible []string
}

// Layout projects every node onto the viewport and notes which ones are close
// enough to be worth drawing.
func Layout(c *canvas.Canvas, v View) Scene {
	s := Scene{Rects: make(map[string]geom.Rect, len(c.Nodes))}
	vpRect := geom.Rect{
		MinX: -cullPad, MinY: -cullPad,
		MaxX: v.VP.W + cullPad, MaxY: v.VP.H + cullPad,
	}
	for i := range c.Nodes {
		n := &c.Nodes[i]
		r := v.Cam.Project(float64(n.X), float64(n.Y), float64(n.Width), float64(n.Height), v.VP)
		s.Rects[n.ID] = r
		if r.Overlaps(vpRect) {
			s.Visible = append(s.Visible, n.ID)
		}
	}
	return s
}

// Render draws the canvas to a string of v.VP.H lines.
//
// The order is fixed by how layers composite: a layer clears the cells it
// covers, so groups go down first, then edges, then the nodes that should hide
// the edge stubs running underneath them. That happens to match how Obsidian
// stacks them.
func Render(c *canvas.Canvas, th Theme, v View) string {
	if v.VP.W <= 0 || v.VP.H <= 0 {
		return ""
	}
	sc := Layout(c, v)
	cv := lipgloss.NewCanvas(v.VP.W, v.VP.H)

	groups, leaves := splitByKind(c, sc)
	compose(cv, c, sc, th, v, groups)
	drawEdges(cv, c, sc, th, v, leaves)
	compose(cv, c, sc, th, v, leaves)

	return padFrame(cv.Render(), v.VP.W, v.VP.H)
}

// padFrame squares the rendered frame off to exactly w by h cells.
//
// The canvas trims trailing spaces from each line and drops empty lines at the
// end, which is the right default for a block of text but wrong for a viewport
// that has to be a fixed rectangle: a short line leaves whatever the terminal
// drew there before, and missing lines push the status bar up the screen.
func padFrame(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	out := make([]string, h)
	blank := strings.Repeat(" ", w)
	for i := range out {
		if i >= len(lines) {
			out[i] = blank
			continue
		}
		l := lines[i]
		if n := w - ansi.StringWidth(l); n > 0 {
			l += strings.Repeat(" ", n)
		}
		out[i] = l
	}
	return strings.Join(out, "\n")
}

// splitByKind separates groups from ordinary nodes, keeping document order
// within each since the spec defines that order as ascending z-index. Groups
// are additionally drawn largest first so a nested group's outline lands on
// top of its parent's.
func splitByKind(c *canvas.Canvas, sc Scene) (groups, leaves []string) {
	for _, id := range sc.Visible {
		if c.Node(id).Type == canvas.TypeGroup {
			groups = append(groups, id)
		} else {
			leaves = append(leaves, id)
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := c.Node(groups[i]), c.Node(groups[j])
		return a.Width*a.Height > b.Width*b.Height
	})
	return
}

// compose places each node as a layer on the canvas.
func compose(cv *lipgloss.Canvas, c *canvas.Canvas, sc Scene, th Theme, v View, ids []string) {
	layers := make([]*lipgloss.Layer, 0, len(ids))
	for _, id := range ids {
		n := c.Node(id)
		r := sc.Rects[id]
		content := NodeBox(n, r, th, NodeState{
			Selected: id == v.SelectedID,
			Matched:  v.Matches[id],
		})
		if content == "" {
			continue
		}
		layers = append(layers, lipgloss.NewLayer(content).X(r.MinX).Y(r.MinY).ID(id))
	}
	if len(layers) > 0 {
		cv.Compose(lipgloss.NewCompositor(layers...))
	}
}

// drawEdges routes every edge whose path could cross the viewport and blits
// the result onto the canvas.
func drawEdges(cv *lipgloss.Canvas, c *canvas.Canvas, sc Scene, th Theme, v View, leaves []string) {
	layer := NewEdgeLayer(v.VP.W, v.VP.H)
	drew := false

	// Claim the cells the nodes will cover before routing anything, so labels
	// are never placed where a node is about to be drawn on top of them.
	for _, id := range leaves {
		layer.Reserve(sc.Rects[id])
	}

	// Labels are held back until every line is down. Placed as each edge is
	// routed, a label can only avoid the lines that happen to exist already,
	// and the next edge routed will run straight through it.
	type pending struct {
		pts   []Pt
		text  string
		color color.Color
	}
	var labels []pending

	for i := range c.Edges {
		e := &c.Edges[i]
		from, okF := sc.Rects[e.FromNode]
		to, okT := sc.Rects[e.ToNode]
		if !okF || !okT {
			continue
		}

		fs, ts := e.From(), e.To()
		if fs == canvas.SideAuto || ts == canvas.SideAuto {
			inferF, inferT := InferSides(from, to)
			if fs == canvas.SideAuto {
				fs = inferF
			}
			if ts == canvas.SideAuto {
				ts = inferT
			}
		}

		a := Anchor(from, fs)
		b := Anchor(to, ts)
		if !spanRect(a, b).Overlaps(geom.Rect{MaxX: v.VP.W, MaxY: v.VP.H}) {
			continue
		}

		col := th.Resolve(e.ParsedColor(), th.Edge)
		if v.Matches[e.FromNode] || v.Matches[e.ToNode] {
			col = th.Match
		}
		pts := Route(a, fs, b, ts)
		layer.Stroke(pts, col)
		if e.ArrowAtEnd() {
			layer.Arrow(pts, false, col)
		}
		if e.ArrowAtStart() {
			layer.Arrow(pts, true, col)
		}
		if v.EdgeLabels && e.Label != "" {
			labels = append(labels, pending{pts: pts, text: e.Label, color: col})
		}
		drew = true
	}

	for _, p := range labels {
		layer.Label(p.pts, p.text, p.color)
	}

	if drew {
		layer.Blit(cv)
	}
}

// spanRect is the bounding box of two points, grown by the routing clearance
// so that the detour an edge takes around a node is included in the test.
func spanRect(a, b Pt) geom.Rect {
	r := geom.Rect{
		MinX: min(a.X, b.X) - clearance,
		MinY: min(a.Y, b.Y) - clearance,
		MaxX: max(a.X, b.X) + clearance + 1,
		MaxY: max(a.Y, b.Y) + clearance + 1,
	}
	return r
}

// Blit copies the edge layer onto a canvas, leaving untouched cells alone so
// that whatever was composed underneath shows through.
func (l *EdgeLayer) Blit(cv *lipgloss.Canvas) {
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			c := l.cells[y*l.W+x]
			if c.r == 0 {
				continue
			}
			cv.SetCell(x, y, &uv.Cell{
				Content: string(c.r),
				Width:   1,
				Style:   uv.Style{Fg: c.col},
			})
		}
	}
}
