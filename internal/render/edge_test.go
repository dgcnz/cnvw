package render

import (
	"strings"
	"testing"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/geom"
)

// arrivalDir is the direction of the final step of a route.
func arrivalDir(pts []Pt) (int, int) {
	n := len(pts)
	a, b := pts[n-2], pts[n-1]
	return sign(b.X - a.X), sign(b.Y - a.Y)
}

func TestRouteArrivesThroughTheRequestedSide(t *testing.T) {
	// Whatever the layout, the last step has to travel into the target node,
	// or the arrowhead ends up pointing away from it.
	cases := []struct {
		name             string
		from, to         geom.Rect
		fromSide, toSide canvas.Side
	}{
		{"facing", geom.Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 4}, geom.Rect{MinX: 40, MinY: 0, MaxX: 50, MaxY: 4}, canvas.SideRight, canvas.SideLeft},
		{"perpendicular", geom.Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 4}, geom.Rect{MinX: 40, MinY: 20, MaxX: 50, MaxY: 24}, canvas.SideBottom, canvas.SideLeft},
		{"same side", geom.Rect{MinX: 40, MinY: 0, MaxX: 50, MaxY: 4}, geom.Rect{MinX: 0, MinY: 20, MaxX: 10, MaxY: 24}, canvas.SideLeft, canvas.SideLeft},
		{"facing away", geom.Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 4}, geom.Rect{MinX: 40, MinY: 0, MaxX: 50, MaxY: 4}, canvas.SideLeft, canvas.SideRight},
		{"backwards", geom.Rect{MinX: 40, MinY: 0, MaxX: 50, MaxY: 4}, geom.Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 4}, canvas.SideRight, canvas.SideLeft},
		{"vertical same side", geom.Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 4}, geom.Rect{MinX: 30, MinY: 30, MaxX: 40, MaxY: 34}, canvas.SideTop, canvas.SideTop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pts := Route(Anchor(tc.from, tc.fromSide), tc.fromSide,
				Anchor(tc.to, tc.toSide), tc.toSide)
			if len(pts) < 2 {
				t.Fatalf("degenerate route %v", pts)
			}
			wantX, wantY := outward(tc.toSide)
			gotX, gotY := arrivalDir(pts)
			// Travelling into the node is the opposite of its outward normal.
			if gotX != -wantX || gotY != -wantY {
				t.Errorf("arrives heading (%d,%d), want (%d,%d); route %v",
					gotX, gotY, -wantX, -wantY, pts)
			}
		})
	}
}

func TestRouteLeavesThroughTheRequestedSide(t *testing.T) {
	from := geom.Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 4}
	to := geom.Rect{MinX: 40, MinY: 30, MaxX: 50, MaxY: 34}
	for _, side := range []canvas.Side{canvas.SideTop, canvas.SideRight, canvas.SideBottom, canvas.SideLeft} {
		pts := Route(Anchor(from, side), side, Anchor(to, canvas.SideLeft), canvas.SideLeft)
		wantX, wantY := outward(side)
		gotX, gotY := sign(pts[1].X-pts[0].X), sign(pts[1].Y-pts[0].Y)
		if gotX != wantX || gotY != wantY {
			t.Errorf("side %v: leaves heading (%d,%d), want (%d,%d)", side, gotX, gotY, wantX, wantY)
		}
	}
}

func TestRouteStaysOrthogonal(t *testing.T) {
	pts := Route(Pt{0, 0}, canvas.SideRight, Pt{37, 21}, canvas.SideTop)
	for i := 0; i+1 < len(pts); i++ {
		if pts[i].X != pts[i+1].X && pts[i].Y != pts[i+1].Y {
			t.Fatalf("segment %v -> %v is diagonal", pts[i], pts[i+1])
		}
	}
}

func TestRouteClearsTheBorder(t *testing.T) {
	// Nodes are composited over the edges, so a path that starts on the border
	// itself has its first cell, and its arrowhead, hidden.
	r := geom.Rect{MinX: 10, MinY: 10, MaxX: 30, MaxY: 20}
	pts := Route(Anchor(r, canvas.SideRight), canvas.SideRight, Pt{80, 15}, canvas.SideLeft)
	if pts[0].X < r.MaxX {
		t.Errorf("route starts at %v, inside the node ending at x=%d", pts[0], r.MaxX)
	}
}

func TestSimplifyKeepsReversals(t *testing.T) {
	// A point where the path doubles back is collinear with its neighbours but
	// dropping it would erase the turn and mis-aim the arrowhead.
	pts := simplify([]Pt{{0, 0}, {10, 0}, {5, 0}})
	if len(pts) != 3 {
		t.Fatalf("reversal was collapsed: %v", pts)
	}
	// A genuine pass-through should still go.
	if got := simplify([]Pt{{0, 0}, {5, 0}, {10, 0}}); len(got) != 2 {
		t.Fatalf("collinear midpoint survived: %v", got)
	}
}

func TestCrossingEdgesMergeIntoAJunction(t *testing.T) {
	l := NewEdgeLayer(11, 11)
	l.Stroke([]Pt{{0, 5}, {10, 5}}, nil)
	l.Stroke([]Pt{{5, 0}, {5, 10}}, nil)
	if got := l.Rune(5, 5); got != '┼' {
		t.Errorf("crossing rendered as %q, want ┼", got)
	}
	if got := l.Rune(2, 5); got != '─' {
		t.Errorf("horizontal run rendered as %q", got)
	}
}

func TestCornersFormAtTurns(t *testing.T) {
	l := NewEdgeLayer(11, 11)
	l.Stroke([]Pt{{1, 1}, {5, 1}, {5, 8}}, nil)
	if got := l.Rune(5, 1); got != '┐' {
		t.Errorf("corner rendered as %q, want ┐", got)
	}
}

func TestZeroLengthSegmentDrawsNothing(t *testing.T) {
	// Coincident waypoints used to contribute opposing connection bits, which
	// invented junctions in the middle of a plain straight run.
	l := NewEdgeLayer(5, 5)
	l.Stroke([]Pt{{2, 2}, {2, 2}}, nil)
	if got := l.Rune(2, 2); got != 0 {
		t.Errorf("degenerate segment drew %q", got)
	}
}

func TestStrokeClipsOutsideTheLayer(t *testing.T) {
	// Routing happens in screen space even when both nodes are off screen.
	l := NewEdgeLayer(4, 4)
	l.Stroke([]Pt{{-100, 2}, {100, 2}}, nil)
	if got := l.Rune(0, 2); got != '─' {
		t.Errorf("visible part missing, got %q", got)
	}
}

func TestArrowPointsAlongTheFinalSegment(t *testing.T) {
	for _, tc := range []struct {
		pts  []Pt
		want rune
	}{
		{[]Pt{{0, 0}, {5, 0}}, '▶'},
		{[]Pt{{5, 0}, {0, 0}}, '◀'},
		{[]Pt{{0, 0}, {0, 5}}, '▼'},
		{[]Pt{{0, 5}, {0, 0}}, '▲'},
	} {
		l := NewEdgeLayer(10, 10)
		l.Arrow(tc.pts, false, nil)
		tip := tc.pts[len(tc.pts)-1]
		if got := l.Rune(tip.X, tip.Y); got != tc.want {
			t.Errorf("route %v gave %q, want %q", tc.pts, got, tc.want)
		}
	}
}

func TestLabelNeverBuriesAnArrowhead(t *testing.T) {
	l := NewEdgeLayer(40, 5)
	pts := []Pt{{0, 2}, {20, 2}}
	l.Arrow(pts, false, nil)
	l.Label(pts, "events", nil)
	if got := l.Rune(20, 2); got != '▶' {
		t.Errorf("arrowhead was overwritten, got %q", got)
	}
}

func TestLabelSkippedWhenItCannotFit(t *testing.T) {
	l := NewEdgeLayer(40, 5)
	pts := []Pt{{0, 2}, {3, 2}}
	l.Stroke(pts, nil)
	l.Label(pts, "much too long for this", nil)
	var b strings.Builder
	for x := range 40 {
		if r := l.Rune(x, 2); r != 0 {
			b.WriteRune(r)
		}
	}
	if strings.ContainsAny(b.String(), "much") {
		t.Errorf("label was drawn into a segment too short for it: %q", b.String())
	}
}

// rowText reads back a row of the layer as a string.
func rowText(l *EdgeLayer, y int) string {
	var b strings.Builder
	for x := range l.W {
		if r := l.Rune(x, y); r != 0 {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func TestLabelOnAVerticalRunGoesBesideTheLine(t *testing.T) {
	// A canvas laid out top to bottom connects bottom-to-top, so the long run
	// is vertical. Writing the text down the column, one character per row,
	// would be unreadable.
	l := NewEdgeLayer(40, 21)
	pts := []Pt{{5, 0}, {5, 20}}
	l.Stroke(pts, nil)
	l.Label(pts, "cites", nil)

	if got := rowText(l, 10); !strings.Contains(got, "cites") {
		t.Fatalf("label not placed level with the middle of the run: %q", got)
	}
	// The line itself must survive.
	if got := l.Rune(5, 10); got != '│' {
		t.Errorf("label overwrote the line, found %q", got)
	}
}

func TestLabelIsAllOrNothing(t *testing.T) {
	// Placing cell by cell and letting each one fail on its own is what turned
	// a row of labels into unreadable fragments of overlapping words.
	l := NewEdgeLayer(30, 5)
	pts := []Pt{{0, 2}, {28, 2}}
	l.Stroke(pts, nil)
	l.set(14, 2, '▶', nil) // an arrowhead in the middle of the run

	l.Label(pts, "a label that spans the middle", nil)
	got := rowText(l, 2)
	if strings.ContainsAny(got, "label") && !strings.Contains(got, "a label that spans the middle") {
		t.Errorf("a partial label was written: %q", got)
	}
}

func TestLabelAvoidsCellsANodeWillCover(t *testing.T) {
	// Nodes composite over the edge layer. A label under one leaves a stranded
	// fragment of a word wherever the node does not quite reach.
	l := NewEdgeLayer(40, 5)
	l.Reserve(geom.Rect{MinX: 10, MinY: 0, MaxX: 30, MaxY: 5})
	pts := []Pt{{0, 2}, {38, 2}}
	l.Stroke(pts, nil)
	l.Label(pts, "covered", nil)

	for x := 10; x < 30; x++ {
		if r := l.Rune(x, 2); r != 0 {
			t.Fatalf("drew %q at x=%d, inside the reserved node", r, x)
		}
	}
}

func TestReservedCellsTakeNoInk(t *testing.T) {
	l := NewEdgeLayer(20, 5)
	l.Reserve(geom.Rect{MinX: 5, MinY: 1, MaxX: 15, MaxY: 4})
	l.Stroke([]Pt{{0, 2}, {19, 2}}, nil)
	if got := l.Rune(10, 2); got != 0 {
		t.Errorf("line drawn under a node: %q", got)
	}
	if got := l.Rune(2, 2); got != '─' {
		t.Errorf("line missing outside the node: %q", got)
	}
}

func TestReserveClipsToTheLayer(t *testing.T) {
	l := NewEdgeLayer(10, 10)
	l.Reserve(geom.Rect{MinX: -50, MinY: -50, MaxX: 500, MaxY: 500})
	l.Stroke([]Pt{{0, 5}, {9, 5}}, nil)
	if got := rowText(l, 5); got != "" {
		t.Errorf("expected everything reserved, got %q", got)
	}
}

func TestLabelPrefersTheLongerRun(t *testing.T) {
	// An L-shaped route with a long vertical arm and a stub of a horizontal
	// one should label the arm, not the stub.
	l := NewEdgeLayer(40, 25)
	pts := []Pt{{4, 0}, {4, 20}, {8, 20}}
	l.Stroke(pts, nil)
	l.Label(pts, "next", nil)
	if got := rowText(l, 10); !strings.Contains(got, "next") {
		t.Errorf("label did not go on the long vertical run: %q", got)
	}
}

func TestInferSidesFacesTheOtherNode(t *testing.T) {
	left := geom.Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 4}
	right := geom.Rect{MinX: 60, MinY: 0, MaxX: 70, MaxY: 4}
	f, to := InferSides(left, right)
	if f != canvas.SideRight || to != canvas.SideLeft {
		t.Errorf("got %v -> %v, want right -> left", f, to)
	}
	below := geom.Rect{MinX: 0, MinY: 40, MaxX: 10, MaxY: 44}
	f, to = InferSides(left, below)
	if f != canvas.SideBottom || to != canvas.SideTop {
		t.Errorf("got %v -> %v, want bottom -> top", f, to)
	}
}
