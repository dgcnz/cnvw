package render

import (
	"image/color"
	"math"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/geom"
)

// Pt is a point in terminal cells.
type Pt struct {
	X, Y int
}

// Connection bits for a cell, used to pick the right box-drawing glyph.
const (
	connN uint8 = 1 << iota
	connE
	connS
	connW
)

// glyphs maps a set of connection bits to the box-drawing rune that shows
// exactly those connections.
var glyphs = [16]rune{
	0:                             ' ',
	connN:                         '│',
	connE:                         '─',
	connN | connE:                 '└',
	connS:                         '│',
	connN | connS:                 '│',
	connE | connS:                 '┌',
	connN | connE | connS:         '├',
	connW:                         '─',
	connN | connW:                 '┘',
	connE | connW:                 '─',
	connN | connE | connW:         '┴',
	connS | connW:                 '┐',
	connN | connS | connW:         '┤',
	connE | connS | connW:         '┬',
	connN | connE | connS | connW: '┼',
}

// edgeCell is one cell of the edge layer. Keeping the connection mask next to
// the glyph is what lets two edges that cross merge into a proper junction
// instead of one simply overwriting the other.
type edgeCell struct {
	r     rune
	mask  uint8
	col   color.Color
	fixed bool // label text and arrowheads, which must not be merged over
}

// EdgeLayer is a viewport-sized scratch buffer that edges are drawn into
// before being composited under the nodes.
//
// Edges are drawn here rather than straight onto the shared canvas so that
// crossings can be merged by connection mask, and so routing can run in screen
// coordinates that fall outside the viewport and simply get clipped.
type EdgeLayer struct {
	W, H  int
	cells []edgeCell
}

// NewEdgeLayer allocates a layer for a viewport of w by h cells.
func NewEdgeLayer(w, h int) *EdgeLayer {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &EdgeLayer{W: w, H: h, cells: make([]edgeCell, w*h)}
}

// At returns the cell at x, y and whether the position is inside the layer.
func (l *EdgeLayer) At(x, y int) (edgeCell, bool) {
	if x < 0 || y < 0 || x >= l.W || y >= l.H {
		return edgeCell{}, false
	}
	return l.cells[y*l.W+x], true
}

// Rune returns the glyph at x, y, or 0 when the cell is empty or off-layer.
func (l *EdgeLayer) Rune(x, y int) rune {
	c, ok := l.At(x, y)
	if !ok {
		return 0
	}
	return c.r
}

// connect adds connection bits to a cell, merging with whatever is already
// there. Writes outside the layer are dropped, which is what allows an edge
// between two off-screen nodes to be routed without special cases.
func (l *EdgeLayer) connect(x, y int, mask uint8, col color.Color) {
	c, ok := l.At(x, y)
	if !ok || c.fixed {
		return
	}
	c.mask |= mask
	c.r = glyphs[c.mask]
	if c.col == nil {
		c.col = col
	}
	l.cells[y*l.W+x] = c
}

// set writes a literal glyph that later strokes must not merge into. An
// existing fixed glyph wins, so a label can never bury an arrowhead.
func (l *EdgeLayer) set(x, y int, r rune, col color.Color) {
	c, ok := l.At(x, y)
	if !ok || c.fixed {
		return
	}
	l.cells[y*l.W+x] = edgeCell{r: r, col: col, fixed: true}
}

// Reserve marks the cells a node will occupy.
//
// Nodes are composited over the edge layer, so anything drawn here beneath one
// is hidden. For a line that costs nothing, but a label half-covered by a node
// leaves a fragment of a word stranded on screen. Reserving the node's cells
// keeps labels out of them, and lets a line that would pass behind the node
// simply stop at its border.
func (l *EdgeLayer) Reserve(r geom.Rect) {
	for y := max(0, r.MinY); y < min(l.H, r.MaxY); y++ {
		for x := max(0, r.MinX); x < min(l.W, r.MaxX); x++ {
			l.cells[y*l.W+x] = edgeCell{fixed: true}
		}
	}
}

// Stroke draws the straight segments of a polyline.
func (l *EdgeLayer) Stroke(pts []Pt, col color.Color) {
	for i := 0; i+1 < len(pts); i++ {
		l.segment(pts[i], pts[i+1], col)
	}
}

// segment draws one axis-aligned run, giving each cell the connection bits
// pointing at its neighbours in the run so that corners form automatically
// where two runs meet.
func (l *EdgeLayer) segment(a, b Pt, col color.Color) {
	if a == b {
		// A degenerate run has no direction to contribute. Giving it one
		// would invent a junction wherever two waypoints coincide.
		return
	}
	switch {
	case a.Y == b.Y:
		lo, hi := a.X, b.X
		if lo > hi {
			lo, hi = hi, lo
		}
		for x := lo; x <= hi; x++ {
			var m uint8
			if x > lo {
				m |= connW
			}
			if x < hi {
				m |= connE
			}
			l.connect(x, a.Y, m, col)
		}
	case a.X == b.X:
		lo, hi := a.Y, b.Y
		if lo > hi {
			lo, hi = hi, lo
		}
		for y := lo; y <= hi; y++ {
			var m uint8
			if y > lo {
				m |= connN
			}
			if y < hi {
				m |= connS
			}
			l.connect(a.X, y, m, col)
		}
	}
}

// arrowGlyph is the arrowhead pointing the way the edge is travelling.
func arrowGlyph(dx, dy int) rune {
	switch {
	case dx > 0:
		return '▶'
	case dx < 0:
		return '◀'
	case dy > 0:
		return '▼'
	}
	return '▲'
}

// Arrow draws an arrowhead at the end of a polyline, pointing along its final
// segment.
func (l *EdgeLayer) Arrow(pts []Pt, atStart bool, col color.Color) {
	if len(pts) < 2 {
		return
	}
	var tip, prev Pt
	if atStart {
		tip, prev = pts[0], pts[1]
	} else {
		tip, prev = pts[len(pts)-1], pts[len(pts)-2]
	}
	l.set(tip.X, tip.Y, arrowGlyph(sign(tip.X-prev.X), sign(tip.Y-prev.Y)), col)
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// Anchor returns the point on the given side of r where an edge attaches.
func Anchor(r geom.Rect, s canvas.Side) Pt {
	midX := r.MinX + r.W()/2
	midY := r.MinY + r.H()/2
	switch s {
	case canvas.SideTop:
		return Pt{midX, r.MinY}
	case canvas.SideBottom:
		return Pt{midX, r.MaxY - 1}
	case canvas.SideLeft:
		return Pt{r.MinX, midY}
	case canvas.SideRight:
		return Pt{r.MaxX - 1, midY}
	}
	return Pt{midX, midY}
}

// outward is the unit step leaving a node through the given side.
func outward(s canvas.Side) (dx, dy int) {
	switch s {
	case canvas.SideTop:
		return 0, -1
	case canvas.SideBottom:
		return 0, 1
	case canvas.SideLeft:
		return -1, 0
	case canvas.SideRight:
		return 1, 0
	}
	return 0, 0
}

// InferSides picks attachment sides for an edge whose document left them
// unspecified, choosing the pair of facing sides along whichever axis the two
// nodes are furthest apart on. This mirrors what Obsidian does when an edge is
// drawn without pinned sides.
func InferSides(from, to geom.Rect) (canvas.Side, canvas.Side) {
	fx := float64(from.MinX+from.MaxX) / 2
	fy := float64(from.MinY+from.MaxY) / 2
	tx := float64(to.MinX+to.MaxX) / 2
	ty := float64(to.MinY+to.MaxY) / 2

	dx, dy := tx-fx, ty-fy
	// Cells are about twice as tall as they are wide, so compare the vertical
	// gap against a halved horizontal one to judge the dominant axis the way
	// it looks on screen.
	if abs(dx) >= abs(dy)*2 {
		if dx >= 0 {
			return canvas.SideRight, canvas.SideLeft
		}
		return canvas.SideLeft, canvas.SideRight
	}
	if dy >= 0 {
		return canvas.SideBottom, canvas.SideTop
	}
	return canvas.SideTop, canvas.SideBottom
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// clearance is how far an edge runs on past its first cell before it is
// allowed to turn, so the line visibly leaves the correct side.
const clearance = 1

// Route lays out an orthogonal path between two anchor points.
//
// Each end steps one cell clear of its border and then away by the clearance,
// which guarantees
// the edge leaves and enters through the intended sides. The two resulting
// points are then joined with a single elbow when the ends run on different
// axes, or with a Z through the midpoint when they run on the same axis. Ends
// that face away from each other need no special case: the clearance puts the
// turn outside the node, and the Z or elbow routes back around.
func Route(from Pt, fromSide canvas.Side, to Pt, toSide canvas.Side) []Pt {
	fdx, fdy := outward(fromSide)
	tdx, tdy := outward(toSide)

	// The path begins one cell outside the border rather than on it. Nodes are
	// composited over the edges, so anything drawn on the border itself is
	// hidden -- which would swallow the arrowhead.
	start := Pt{from.X + fdx, from.Y + fdy}
	end := Pt{to.X + tdx, to.Y + tdy}

	a := Pt{start.X + fdx*clearance, start.Y + fdy*clearance}
	b := Pt{end.X + tdx*clearance, end.Y + tdy*clearance}

	pts := []Pt{start, a}
	fromHoriz := fdx != 0
	toHoriz := tdx != 0

	switch {
	case fromHoriz && toHoriz:
		mid := connector(a.X, b.X, fdx, tdx)
		pts = append(pts, Pt{mid, a.Y}, Pt{mid, b.Y})
	case !fromHoriz && !toHoriz:
		mid := connector(a.Y, b.Y, fdy, tdy)
		pts = append(pts, Pt{a.X, mid}, Pt{b.X, mid})
	case fromHoriz:
		pts = append(pts, Pt{b.X, a.Y})
	default:
		pts = append(pts, Pt{a.X, b.Y})
	}

	return simplify(append(pts, b, end))
}

// connector picks the coordinate of the crossbar joining two ends that run on
// the same axis.
//
// The midpoint is the natural choice, but it is only valid when it lies on the
// outward side of both ends. Two nodes whose edges leave through the same side
// have a midpoint that sits between them, which would make the path doubles
// back into its own destination and point the arrowhead the wrong way. Pinning
// the crossbar beyond both ends turns that case into the U it should be.
func connector(a, b, fromOut, toOut int) int {
	lo, hi := math.MinInt/2, math.MaxInt/2
	if fromOut > 0 {
		lo = max(lo, a)
	} else if fromOut < 0 {
		hi = min(hi, a)
	}
	if toOut > 0 {
		lo = max(lo, b)
	} else if toOut < 0 {
		hi = min(hi, b)
	}
	if lo > hi {
		// The two ends want the crossbar on opposite sides of each other.
		// Favour the destination, so at least the arrival stays correct.
		return b
	}
	return min(max((a+b)/2, lo), hi)
}

// simplify drops repeated waypoints and those that merely continue a run. Without it a route that
// happens to be a straight line still arrives as four separate runs, which
// leaves no single segment long enough to hold a label.
func simplify(pts []Pt) []Pt {
	out := pts[:0:0]
	for _, p := range pts {
		if n := len(out); n > 0 && out[n-1] == p {
			continue
		}
		out = append(out, p)
	}
	for i := 1; i+1 < len(out); {
		a, b, c := out[i-1], out[i], out[i+1]
		// Only drop b when the run genuinely passes through it. A point where
		// the path reverses is collinear too, and collapsing that would hide
		// the turn and mis-aim the arrowhead at the end of the route.
		if (a.X == b.X && b.X == c.X && between(a.Y, b.Y, c.Y)) ||
			(a.Y == b.Y && b.Y == c.Y && between(a.X, b.X, c.X)) {
			out = append(out[:i], out[i+1:]...)
			continue
		}
		i++
	}
	return out
}

// between reports whether b lies within the closed span from a to c.
func between(a, b, c int) bool {
	return (a <= b && b <= c) || (c <= b && b <= a)
}

// canPlace reports whether n cells starting at x, y can take label text.
//
// Placement is all or nothing. Writing cell by cell and letting each one fail
// on its own is what turned a row of labels into "Which layers, when? s?":
// three labels partly overwriting each other, every one of them unreadable.
func (l *EdgeLayer) canPlace(x, y, n int, needEmpty bool) bool {
	for i := range n {
		c, ok := l.At(x+i, y)
		if !ok || c.fixed {
			return false
		}
		if needEmpty && c.r != 0 {
			return false
		}
	}
	return true
}

// write places label text, assuming canPlace already approved the run.
func (l *EdgeLayer) write(x, y int, runes []rune, col color.Color) {
	for i, r := range runes {
		l.set(x+i, y, r, col)
	}
}

// Label draws an edge label on the longest straight run of its route.
//
// A horizontal run takes the text inline, sitting in a gap in the line. A
// vertical run cannot: text written down a column one character per row is
// unreadable, so the label goes beside the line instead, level with its
// middle. Vertical placement is the common case in practice, because a canvas
// laid out top to bottom connects nodes bottom-to-top and leaves the long run
// on the vertical axis.
//
// A label that does not fit, or that would land on another label or an
// arrowhead, is dropped. At low zoom that silently thins them out, which is
// what you want: labels crowding into the same few cells say less than no
// labels at all.
func (l *EdgeLayer) Label(pts []Pt, text string, col color.Color) {
	if text == "" {
		return
	}
	runes := []rune(text)

	bestH, bestHLen := -1, 0
	bestV, bestVLen := -1, 0
	for i := 0; i+1 < len(pts); i++ {
		a, b := pts[i], pts[i+1]
		switch {
		case a.Y == b.Y:
			if n := abs2(b.X - a.X); n > bestHLen {
				bestH, bestHLen = i, n
			}
		case a.X == b.X:
			if n := abs2(b.Y - a.Y); n > bestVLen {
				bestV, bestVLen = i, n
			}
		}
	}

	// Prefer whichever run is longer, then fall back to the other.
	if bestHLen >= bestVLen {
		if l.labelHorizontal(pts, bestH, bestHLen, runes, col) {
			return
		}
		l.labelVertical(pts, bestV, bestVLen, runes, col)
		return
	}
	if l.labelVertical(pts, bestV, bestVLen, runes, col) {
		return
	}
	l.labelHorizontal(pts, bestH, bestHLen, runes, col)
}

// labelHorizontal centers the text inside a horizontal run, with a space of
// padding on each side so the line does not run straight into the letters.
func (l *EdgeLayer) labelHorizontal(pts []Pt, seg, length int, runes []rune, col color.Color) bool {
	if seg < 0 || length < len(runes)+2 {
		return false
	}
	a, b := pts[seg], pts[seg+1]
	lo, hi := a.X, b.X
	if lo > hi {
		lo, hi = hi, lo
	}
	start := lo + (hi-lo-len(runes))/2
	if !l.canPlace(start-1, a.Y, len(runes)+2, false) {
		return false
	}
	l.set(start-1, a.Y, ' ', col)
	l.write(start, a.Y, runes, col)
	l.set(start+len(runes), a.Y, ' ', col)
	return true
}

// labelVertical sets the text beside a vertical run, level with its middle.
// The cells it needs must be genuinely empty, since unlike the horizontal case
// it is not replacing its own line but borrowing space next to it.
func (l *EdgeLayer) labelVertical(pts []Pt, seg, length int, runes []rune, col color.Color) bool {
	if seg < 0 || length < 1 {
		return false
	}
	a, b := pts[seg], pts[seg+1]
	lo, hi := a.Y, b.Y
	if lo > hi {
		lo, hi = hi, lo
	}
	row := lo + (hi-lo)/2
	// Try the right of the line first, then the left.
	for _, start := range []int{a.X + 2, a.X - len(runes) - 1} {
		if l.canPlace(start, row, len(runes), true) {
			l.write(start, row, runes, col)
			return true
		}
	}
	return false
}

func abs2(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
