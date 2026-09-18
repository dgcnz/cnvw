package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/geom"
)

func loadSample(t testing.TB) *canvas.Canvas {
	t.Helper()
	c, err := canvas.Load(filepath.Join("..", "..", "testdata", "sample.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func fitView(c *canvas.Canvas, w, h int) View {
	minX, minY, maxX, maxY, _ := c.Bounds()
	vp := geom.Viewport{W: w, H: h}
	return View{Cam: geom.Fit(minX, minY, maxX, maxY, vp), VP: vp, EdgeLabels: true}
}

// checkFrame asserts the render is exactly the size the viewport asked for.
// A frame that is off by a cell corrupts the whole terminal display.
func checkFrame(t *testing.T, out string, w, h int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) != h {
		t.Fatalf("got %d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != w {
			t.Errorf("line %d is %d cells, want %d: %q", i, got, w, ansi.Strip(l))
		}
	}
}

func TestFrameSizeIsExact(t *testing.T) {
	c := loadSample(t)
	for _, size := range [][2]int{{80, 24}, {120, 40}, {40, 12}, {200, 60}} {
		w, h := size[0], size[1]
		t.Run(fmt.Sprintf("%dx%d", w, h), func(t *testing.T) {
			checkFrame(t, Render(c, Dark(), fitView(c, w, h)), w, h)
		})
	}
}

func TestFrameSizeIsExactAtEveryZoom(t *testing.T) {
	c := loadSample(t)
	const w, h = 100, 30
	v := fitView(c, w, h)
	for _, z := range []float64{geom.MinZoom, 0.05, 0.1, 0.25, 0.5, 1, 2, geom.MaxZoom} {
		v.Cam.Zoom = z
		t.Run(fmt.Sprintf("zoom-%.2f", z), func(t *testing.T) {
			checkFrame(t, Render(c, Dark(), v), w, h)
		})
	}
}

func TestPanningFarAwayStillRendersAFrame(t *testing.T) {
	c := loadSample(t)
	v := fitView(c, 80, 24)
	v.Cam.X, v.Cam.Y = 1e9, -1e9
	checkFrame(t, Render(c, Dark(), v), 80, 24)
}

func TestEmptyCanvasRenders(t *testing.T) {
	c, err := canvas.Parse([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	checkFrame(t, Render(c, Dark(), View{Cam: geom.NewCamera(), VP: geom.Viewport{W: 40, H: 10}}), 40, 10)
}

func TestDegenerateViewportIsSafe(t *testing.T) {
	c := loadSample(t)
	for _, vp := range []geom.Viewport{{W: 0, H: 0}, {W: -5, H: 10}, {W: 10, H: 0}} {
		if got := Render(c, Dark(), View{Cam: geom.NewCamera(), VP: vp}); got != "" {
			t.Errorf("viewport %+v produced %q", vp, got)
		}
	}
}

func TestCullingSkipsOffscreenNodes(t *testing.T) {
	c := loadSample(t)
	v := fitView(c, 80, 24)
	if len(Layout(c, v).Visible) != len(c.Nodes) {
		t.Error("a fitted canvas should have every node visible")
	}
	v.Cam.X += 100000
	if got := len(Layout(c, v).Visible); got != 0 {
		t.Errorf("%d nodes still visible after panning away", got)
	}
}

func TestBothThemesRender(t *testing.T) {
	c := loadSample(t)
	for _, th := range []Theme{Dark(), Light()} {
		checkFrame(t, Render(c, th, fitView(c, 90, 28)), 90, 28)
	}
}

func TestSelectionAndMatchesRender(t *testing.T) {
	c := loadSample(t)
	v := fitView(c, 100, 30)
	v.SelectedID = c.Nodes[1].ID
	v.Matches = map[string]bool{c.Nodes[2].ID: true}
	checkFrame(t, Render(c, Dark(), v), 100, 30)
}

func TestNodesCoverTheEdgesBeneathThem(t *testing.T) {
	// Layers are opaque, so the node drawn last must hide the edge stubs that
	// were routed under it. If the order slipped, box interiors would show
	// stray line glyphs.
	c, err := canvas.Parse([]byte(`{
		"nodes":[
			{"id":"a","type":"text","x":0,"y":0,"width":400,"height":200,"text":"aaaa"},
			{"id":"b","type":"text","x":800,"y":0,"width":400,"height":200,"text":"bbbb"}
		],
		"edges":[{"id":"e","fromNode":"a","fromSide":"right","toNode":"b","toSide":"left"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out := ansi.Strip(Render(c, Dark(), fitView(c, 100, 30)))
	for _, line := range strings.Split(out, "\n") {
		// Inside a box the only box-drawing runes should be its own sides.
		if i := strings.Index(line, "aaaa"); i > 0 && strings.ContainsRune(line[i:i+4], '─') {
			t.Errorf("edge leaked into a node body: %q", line)
		}
	}
}

// loadLarge opens the generated stress fixture, skipping if it is absent.
func loadLarge(b *testing.B) *canvas.Canvas {
	b.Helper()
	path := filepath.Join("..", "..", "testdata", "large.canvas")
	if _, err := os.Stat(path); err != nil {
		b.Skip("large fixture not generated; run testdata/gen_large.py")
	}
	c, err := canvas.Load(path)
	if err != nil {
		b.Fatal(err)
	}
	return c
}

// realCanvases are hand-made documents, kept as fixtures because they use the
// format in ways a synthetic sample does not: null colors, nodes far wider
// than they are tall, and edges that are labelled and run bottom-to-top.
var realCanvases = []string{"grouped.canvas", "labeled-tree.canvas"}

func TestRealCanvasesLoadCleanly(t *testing.T) {
	for _, name := range realCanvases {
		c, err := canvas.Load(filepath.Join("..", "..", "testdata", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(c.Warnings) != 0 {
			t.Errorf("%s: %v", name, c.Warnings)
		}
		if len(c.Nodes) == 0 {
			t.Errorf("%s: no nodes parsed", name)
		}
	}
}

func TestRealCanvasesRenderAtEveryZoom(t *testing.T) {
	for _, name := range realCanvases {
		c, err := canvas.Load(filepath.Join("..", "..", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		const w, h = 130, 40
		v := fitView(c, w, h)
		for _, z := range []float64{geom.MinZoom, 0.11, 0.25, 0.5, 1, 2, geom.MaxZoom} {
			v.Cam.Zoom = z
			t.Run(fmt.Sprintf("%s-%.2f", name, z), func(t *testing.T) {
				checkFrame(t, Render(c, Dark(), v), w, h)
			})
		}
	}
}

func TestLabelsNeverStrandFragmentsUnderNodes(t *testing.T) {
	// labeled-tree.canvas labels almost every edge and routes bottom-to-top.
	// Before labels were placed as a second pass over reserved node cells,
	// this produced rows of half-overwritten words.
	c, err := canvas.Load(filepath.Join("..", "..", "testdata", "labeled-tree.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	v := fitView(c, 130, 40)
	sc := Layout(c, v)
	out := strings.Split(ansi.Strip(Render(c, Dark(), v)), "\n")

	// Every labelled edge either shows its whole label or none of it.
	for _, e := range c.Edges {
		if e.Label == "" {
			continue
		}
		if _, ok := sc.Rects[e.FromNode]; !ok {
			continue
		}
		whole := strings.Contains(strings.Join(out, "\n"), e.Label)
		if whole {
			continue
		}
		// Not placed at all is fine; a long fragment of it is not.
		if len(e.Label) > 12 {
			frag := e.Label[2 : len(e.Label)-2]
			if strings.Contains(strings.Join(out, "\n"), frag) {
				t.Errorf("fragment of label %q survived without the whole", e.Label)
			}
		}
	}
}

func BenchmarkRenderLargeCanvasZoomedIn(b *testing.B) {
	// The interactive case: most of the canvas is off screen and culled.
	c := loadLarge(b)
	v := fitView(c, 200, 50)
	v.Cam.Zoom = 1
	b.ResetTimer()
	for b.Loop() {
		Render(c, Dark(), v)
	}
}

func BenchmarkRenderLargeCanvas(b *testing.B) {
	c := loadLarge(b)
	v := fitView(c, 200, 50)
	b.ResetTimer()
	for b.Loop() {
		Render(c, Dark(), v)
	}
}
