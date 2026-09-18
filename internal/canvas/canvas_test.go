package canvas

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDropsDanglingEdges(t *testing.T) {
	c, err := Parse([]byte(`{
		"nodes": [{"id":"a","type":"text","x":0,"y":0,"width":10,"height":10,"text":"hi"}],
		"edges": [
			{"id":"e1","fromNode":"a","toNode":"a"},
			{"id":"e2","fromNode":"a","toNode":"ghost"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Edges) != 1 || c.Edges[0].ID != "e1" {
		t.Fatalf("expected only e1 to survive, got %+v", c.Edges)
	}
	if len(c.Warnings) != 1 {
		t.Fatalf("expected one warning, got %v", c.Warnings)
	}
}

func TestParseEmptyDocument(t *testing.T) {
	// Both top-level arrays are optional per the spec.
	c, err := Parse([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, ok := c.Bounds(); ok {
		t.Fatal("an empty canvas should have no bounds")
	}
}

func TestParseDropsDuplicateIDs(t *testing.T) {
	c, err := Parse([]byte(`{"nodes":[
		{"id":"a","type":"text","x":0,"y":0,"width":1,"height":1,"text":"first"},
		{"id":"a","type":"text","x":9,"y":9,"width":1,"height":1,"text":"second"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Nodes) != 1 || c.Node("a").Text != "first" {
		t.Fatalf("expected the first node to win, got %+v", c.Nodes)
	}
}

func TestNodePointersSurviveIndexing(t *testing.T) {
	// index() compacts the slice, so the id map must be rebuilt afterwards or
	// it ends up pointing at stale elements.
	c, err := Parse([]byte(`{"nodes":[
		{"id":"","type":"text","x":0,"y":0,"width":1,"height":1,"text":"dropped"},
		{"id":"b","type":"text","x":5,"y":6,"width":1,"height":1,"text":"kept"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	n := c.Node("b")
	if n == nil || n.Text != "kept" || n.X != 5 {
		t.Fatalf("id map points at the wrong node: %+v", n)
	}
}

func TestTitleStripsMarkdown(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"## Ingest pipeline\n\nbody", "Ingest pipeline"},
		{"\n\n- **bold** item", "bold item"},
		{"> quoted", "quoted"},
		{"plain", "plain"},
		{"", "(empty)"},
	} {
		n := Node{Type: TypeText, Text: tc.in}
		if got := n.Title(); got != tc.want {
			t.Errorf("Title(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEdgeEndpointDefaults(t *testing.T) {
	// The spec defaults are no arrow at the start and an arrow at the end.
	var e Edge
	if e.ArrowAtStart() {
		t.Error("start should default to no arrow")
	}
	if !e.ArrowAtEnd() {
		t.Error("end should default to an arrow")
	}
	e.ToEnd = "none"
	if e.ArrowAtEnd() {
		t.Error("an explicit none should suppress the end arrow")
	}
}

func TestParseColor(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Color
	}{
		{"1", Color{Preset: PresetRed}},
		{"6", Color{Preset: PresetPurple}},
		{"#ff0000", Color{Hex: "#FF0000"}},
		{"#abc", Color{Hex: "#ABC"}},
		{"7", Color{}},
		{"", Color{}},
		{"red", Color{}},
	} {
		if got := ParseColor(tc.in); got != tc.want {
			t.Errorf("ParseColor(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestLoadSample(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "testdata", "sample.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Warnings) != 0 {
		t.Errorf("sample should load cleanly, got %v", c.Warnings)
	}
	minX, minY, maxX, maxY, ok := c.Bounds()
	if !ok || maxX <= minX || maxY <= minY {
		t.Fatalf("bad bounds: %d %d %d %d", minX, minY, maxX, maxY)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(os.TempDir(), "definitely-not-here.canvas")); err == nil {
		t.Fatal("expected an error")
	}
}
