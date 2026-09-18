package geom

import (
	"math"
	"testing"
)

var vp = Viewport{W: 80, H: 24}

func TestScreenWorldRoundTrip(t *testing.T) {
	c := Camera{X: 123.5, Y: -40.25, Zoom: 0.7}
	for _, p := range [][2]float64{{0, 0}, {40, 12}, {79, 23}, {-5, 100}} {
		wx, wy := c.ScreenToWorld(p[0], p[1], vp)
		sx, sy := c.WorldToScreen(wx, wy, vp)
		if math.Abs(sx-p[0]) > 1e-9 || math.Abs(sy-p[1]) > 1e-9 {
			t.Errorf("round trip of (%v,%v) gave (%v,%v)", p[0], p[1], sx, sy)
		}
	}
}

func TestProjectKeepsAdjacentNodesFlush(t *testing.T) {
	// Rounding position and size independently would leave a one-cell gap or
	// overlap between abutting nodes at some camera offsets. Taking the
	// difference of two rounded corners cannot.
	for off := 0.0; off < 40; off += 0.37 {
		c := Camera{X: off, Y: off / 3, Zoom: 0.63}
		left := c.Project(0, 0, 250, 60, vp)
		right := c.Project(250, 0, 250, 60, vp)
		if left.MaxX != right.MinX {
			t.Fatalf("offset %.2f: left ends at %d but right starts at %d",
				off, left.MaxX, right.MinX)
		}
	}
}

func TestProjectSizeIsStableWhilePanning(t *testing.T) {
	// A node must not change size by a cell purely because the camera crossed
	// a sub-cell boundary; that shows up as visible twitching when panning.
	const zoom = 1.0
	want := -1
	for off := 0.0; off < 20; off += 0.1 {
		c := Camera{X: off, Zoom: zoom}
		r := c.Project(0, 0, 8*13, 16*4, vp)
		if want == -1 {
			want = r.W()
		}
		if r.W() != want {
			t.Fatalf("offset %.2f: width changed from %d to %d", off, want, r.W())
		}
	}
}

func TestZoomHoldsAnchorStill(t *testing.T) {
	c := Camera{X: 500, Y: 300, Zoom: 1}
	const ax, ay = 10.0, 5.0
	wx, wy := c.ScreenToWorld(ax, ay, vp)

	z := c.ZoomBy(ZoomStep, ax, ay, vp)
	gx, gy := z.ScreenToWorld(ax, ay, vp)
	if math.Abs(gx-wx) > 1e-6 || math.Abs(gy-wy) > 1e-6 {
		t.Errorf("anchor drifted from (%v,%v) to (%v,%v)", wx, wy, gx, gy)
	}
}

func TestZoomClamps(t *testing.T) {
	c := Camera{Zoom: MaxZoom}
	for range 10 {
		c = c.ZoomBy(ZoomStep, 0, 0, vp)
	}
	if c.Zoom > MaxZoom {
		t.Errorf("zoom %v exceeded the maximum", c.Zoom)
	}
	c = Camera{Zoom: MinZoom}
	for range 10 {
		c = c.ZoomBy(1/ZoomStep, 0, 0, vp)
	}
	if c.Zoom < MinZoom {
		t.Errorf("zoom %v fell below the minimum", c.Zoom)
	}
}

func TestFitFramesEverything(t *testing.T) {
	c := Fit(-100, -50, 900, 450, vp)
	r := c.Project(-100, -50, 1000, 500, vp)
	if r.MinX < 0 || r.MinY < 0 || r.MaxX > vp.W || r.MaxY > vp.H {
		t.Fatalf("fitted canvas %+v spills out of the %dx%d viewport", r, vp.W, vp.H)
	}
}

func TestFitHandlesDegenerateBounds(t *testing.T) {
	// A single zero-size node, or an empty canvas, must not divide by zero.
	c := Fit(10, 10, 10, 10, vp)
	if c.Zoom <= 0 || math.IsNaN(c.X) || math.IsNaN(c.Y) {
		t.Fatalf("degenerate fit produced %+v", c)
	}
}

func TestProjectClampsExtremeCoordinates(t *testing.T) {
	// A far-off node at a high zoom must not overflow the int conversion.
	c := Camera{X: 0, Y: 0, Zoom: MaxZoom}
	r := c.Project(1e17, 1e17, 10, 10, vp)
	if r.MinX != cellLimit || r.MinY != cellLimit {
		t.Fatalf("expected clamping to %d, got %+v", cellLimit, r)
	}
}

func TestLegibleZoomTakesTheMedian(t *testing.T) {
	// One narrow outlier must not drag the whole canvas in: requiring every
	// node to be legible would zoom to the worst card on the board.
	sizes := []Size{
		{W: 500, H: 300}, {W: 500, H: 300}, {W: 500, H: 300},
		{W: 500, H: 300}, {W: 20, H: 300},
	}
	got := LegibleZoom(sizes, 16)
	want := 16 * PxPerCellX / 500.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("LegibleZoom = %v, want the median %v", got, want)
	}
}

func TestLegibleZoomAccountsForHeight(t *testing.T) {
	// A node wide enough for its title but under a cell tall still shows
	// nothing, so height sets the floor when it is the binding constraint.
	sizes := []Size{{W: 4000, H: 8}}
	got := LegibleZoom(sizes, 16)
	if want := PxPerCellY / 8.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("LegibleZoom = %v, want %v from the height", got, want)
	}
}

func TestLegibleZoomDegenerateInput(t *testing.T) {
	if got := LegibleZoom(nil, 16); got != 0 {
		t.Errorf("no nodes should give 0, got %v", got)
	}
	if got := LegibleZoom([]Size{{W: 0, H: 0}}, 16); got != 0 {
		t.Errorf("zero-size nodes should give 0, got %v", got)
	}
	if got := LegibleZoom([]Size{{W: 100, H: 100}}, 0); got != 0 {
		t.Errorf("no title budget should give 0, got %v", got)
	}
}

func TestClampToBoundsKeepsTheViewportOnTheCanvas(t *testing.T) {
	// Centering on a node at the edge of a large canvas would otherwise spend
	// half the screen on empty space beyond it.
	c := Camera{X: 0, Y: 0, Zoom: 1}
	c = c.ClampToBounds(0, 0, 100000, 100000, vp)
	minX, minY, _, _ := c.VisibleWorld(vp, 0)
	if minX < -1e-6 || minY < -1e-6 {
		t.Errorf("viewport starts at (%v,%v), outside the canvas", minX, minY)
	}
}

func TestClampToBoundsCentersWhatAlreadyFits(t *testing.T) {
	// Nothing to slide along an axis the canvas does not fill.
	c := Camera{X: 9999, Y: 9999, Zoom: 0.01}
	c = c.ClampToBounds(0, 0, 100, 60, vp)
	if math.Abs(c.X-50) > 1e-9 || math.Abs(c.Y-30) > 1e-9 {
		t.Errorf("expected the canvas centered, got (%v,%v)", c.X, c.Y)
	}
}

func TestTiers(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		want Tier
	}{
		{40, 10, TierFull},
		{12, 3, TierFull},
		{11, 3, TierTitle},
		{6, 3, TierTitle},
		{5, 3, TierMini},
		{11, 2, TierMini},
		{3, 2, TierMini},
		// Wide but a single row: the common shape in a real canvas once it is
		// zoomed out at all. A title still fits across; a block says nothing.
		{20, 1, TierMini},
		{3, 1, TierMini},
		{2, 1, TierBlock},
		{1, 1, TierBlock},
		{0, 5, TierHidden},
	} {
		got := TierFor(Rect{MaxX: tc.w, MaxY: tc.h})
		if got != tc.want {
			t.Errorf("TierFor(%dx%d) = %v, want %v", tc.w, tc.h, got, tc.want)
		}
	}
}

func TestGroupNeverCollapsesToABlock(t *testing.T) {
	// Filling a group's rectangle would bury the nodes inside it.
	for _, r := range []Rect{{MaxX: 4, MaxY: 2}, {MaxX: 2, MaxY: 2}, {MaxX: 30, MaxY: 2}} {
		if got := TierForGroup(r); got == TierBlock {
			t.Errorf("group %+v collapsed to a block", r)
		}
	}
}

func TestNearestPrefersStraightAhead(t *testing.T) {
	from := Point{0, 0}
	cands := []Point{
		{100, 0},  // straight right
		{60, 200}, // closer in raw distance, but well off the line
	}
	if got := Nearest(from, cands, DirRight); got != 0 {
		t.Errorf("expected the node straight ahead, got index %d", got)
	}
}

func TestNearestPrefersTheConeOverRawDistance(t *testing.T) {
	// A node barely above but far to the side is not what "up" meant, even
	// when it is the closest by the weighted score.
	from := Point{0, 0}
	cands := []Point{
		{800, -15}, // 15 up, 800 across: outside the cone
		{40, -300}, // straight up, much further away
	}
	if got := Nearest(from, cands, DirUp); got != 1 {
		t.Errorf("expected the node in the cone, got index %d", got)
	}
}

func TestNearestFallsBackOutsideTheCone(t *testing.T) {
	// On a sparse canvas a jump should still find something rather than
	// refusing to move because nothing sits straight ahead.
	from := Point{0, 0}
	cands := []Point{{800, -15}}
	if got := Nearest(from, cands, DirUp); got != 0 {
		t.Errorf("expected the only candidate, got index %d", got)
	}
}

func TestNearestIgnoresWhatIsBehind(t *testing.T) {
	from := Point{0, 0}
	cands := []Point{{-10, 0}, {0, 0}}
	if got := Nearest(from, cands, DirRight); got != -1 {
		t.Errorf("expected no candidate, got index %d", got)
	}
}
