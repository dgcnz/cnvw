package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/geom"
	"github.com/dgcnz/cnvw/internal/render"
)

func sample(t testing.TB) (*canvas.Canvas, string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "sample.canvas")
	c, err := canvas.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return c, path
}

// sized returns a model that has already received a window size.
func sized(t testing.TB, w, h int) Model {
	t.Helper()
	c, path := sample(t)
	m := New(c, path, filepath.Join("..", "..", "testdata", "vault"), "dark")
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model)
}

// press feeds a keystroke to the model.
func press(t testing.TB, m Model, key string) Model {
	t.Helper()
	var msg tea.KeyPressMsg
	if r := []rune(key); len(r) == 1 {
		msg = tea.KeyPressMsg{Code: r[0], Text: key}
	} else {
		msg = tea.KeyPressMsg{Code: keyCode(key)}
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

func keyCode(name string) rune {
	switch name {
	case "enter":
		return tea.KeyEnter
	case "esc":
		return tea.KeyEscape
	case "tab":
		return tea.KeyTab
	case "backspace":
		return tea.KeyBackspace
	case "left":
		return tea.KeyLeft
	case "right":
		return tea.KeyRight
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	}
	return 0
}

// checkView asserts the rendered view exactly fills the window. A short line
// leaves stale content on screen; a long one wraps and shifts everything down.
func checkView(t *testing.T, m Model, what string) {
	t.Helper()
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != m.h {
		t.Fatalf("%s: got %d lines, want %d", what, len(lines), m.h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != m.w {
			t.Errorf("%s: line %d is %d cells, want %d: %q", what, i, got, m.w, ansi.Strip(l))
		}
	}
}

func TestViewIsExactlyTheWindowSize(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {40, 10}} {
		m := sized(t, size[0], size[1])
		checkView(t, m, "canvas")
	}
}

func TestEveryModeFillsTheWindow(t *testing.T) {
	base := sized(t, 90, 26)

	m := press(t, base, "?")
	checkView(t, m, "help")

	m = base
	m.selected = "n6" // a note long enough to scroll
	m = press(t, m, "enter")
	checkView(t, m, "focus")
	m = press(t, m, "j")
	checkView(t, m, "focus scrolled")

	m = press(t, base, "/")
	for _, r := range "queue" {
		m = press(t, m, string(r))
	}
	checkView(t, m, "search")
}

func TestNarrowWindowDoesNotOverflow(t *testing.T) {
	// The status line has to survive a window too narrow to hold it.
	for _, w := range []int{12, 20, 30} {
		m := sized(t, w, 8)
		checkView(t, m, "narrow")
	}
}

func TestFitsOnFirstWindowSize(t *testing.T) {
	// Fitting has to wait for a real viewport; doing it at construction would
	// fit to a zero-sized screen.
	c, path := sample(t)
	m := New(c, path, "", "dark")
	if m.cam.Zoom != 1 {
		t.Fatalf("unfitted camera should sit at zoom 1, got %v", m.cam.Zoom)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if next.(Model).cam.Zoom == 1 {
		t.Error("camera was not fitted after the first window size")
	}
}

func TestOpeningViewIsTheFitWhenThatIsAlreadyLegible(t *testing.T) {
	// A small canvas must not be zoomed in past its own overview.
	m := sized(t, 120, 30)
	fitted := m.fit()
	if m.cam.Zoom != fitted.cam.Zoom {
		t.Errorf("opening zoom %v differs from the fit %v", m.cam.Zoom, fitted.cam.Zoom)
	}
}

func TestOpeningViewZoomsInUntilTitlesRead(t *testing.T) {
	// labeled-tree.canvas fits at a zoom where no node can show anything, which
	// is an accurate picture carrying no information.
	c, err := canvas.Load(filepath.Join("..", "..", "testdata", "labeled-tree.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(c, "labeled-tree.canvas", "", "dark")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = next.(Model)

	if geometric := m.fit(); m.cam.Zoom <= geometric.cam.Zoom {
		t.Fatalf("opening zoom %v did not rise above the fit %v",
			m.cam.Zoom, geometric.cam.Zoom)
	}

	// The median node has to reach a tier that can show a title.
	sc := render.Layout(c, m.view())
	titled := 0
	for _, id := range sc.Visible {
		if geom.TierFor(sc.Rects[id]) >= geom.TierMini {
			titled++
		}
	}
	if titled == 0 {
		t.Error("no visible node can show a title at the opening zoom")
	}
}

func TestOpeningViewKeepsTheSelectionOnScreen(t *testing.T) {
	c, err := canvas.Load(filepath.Join("..", "..", "testdata", "labeled-tree.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(c, "labeled-tree.canvas", "", "dark")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = next.(Model)

	sc := render.Layout(c, m.view())
	found := false
	for _, id := range sc.Visible {
		if id == m.selected {
			found = true
		}
	}
	if !found {
		t.Error("the selected node is off screen in the opening view")
	}
}

func TestOpeningViewDoesNotStrandTheCameraOffTheCanvas(t *testing.T) {
	// Centering on the first node, which sits at a corner, would leave half
	// the screen beyond the edge of the canvas.
	c, err := canvas.Load(filepath.Join("..", "..", "testdata", "labeled-tree.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(c, "labeled-tree.canvas", "", "dark")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = next.(Model)

	if len(render.Layout(c, m.view()).Visible) < 3 {
		t.Error("the opening view shows almost nothing")
	}
}

func TestFitStillGivesTheWholeCanvas(t *testing.T) {
	// Whatever the opening view decides, f has to be the way back out.
	c, err := canvas.Load(filepath.Join("..", "..", "testdata", "labeled-tree.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(c, "labeled-tree.canvas", "", "dark")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = press(t, next.(Model), "f")

	if got := len(render.Layout(c, m.view()).Visible); got != len(c.Nodes) {
		t.Errorf("f showed %d of %d nodes", got, len(c.Nodes))
	}
}

func TestPanningDoesNotChangeSelection(t *testing.T) {
	m := sized(t, 100, 30)
	before := m.selected
	x, y := m.cam.X, m.cam.Y
	m = press(t, m, "right")
	m = press(t, m, "down")
	if m.selected != before {
		t.Error("panning changed the selection")
	}
	if m.cam.X == x && m.cam.Y == y {
		t.Error("camera did not move")
	}
}

func TestZoomKeysStayInRange(t *testing.T) {
	m := sized(t, 100, 30)
	for range 40 {
		m = press(t, m, "+")
	}
	if m.cam.Zoom > geom.MaxZoom {
		t.Errorf("zoom ran past the maximum: %v", m.cam.Zoom)
	}
	for range 80 {
		m = press(t, m, "-")
	}
	if m.cam.Zoom < geom.MinZoom {
		t.Errorf("zoom ran past the minimum: %v", m.cam.Zoom)
	}
}

func TestTabCyclesEveryNodeAndWraps(t *testing.T) {
	m := sized(t, 100, 30)
	seen := map[string]bool{m.selected: true}
	for range len(m.order) {
		m = press(t, m, "tab")
		seen[m.selected] = true
	}
	if len(seen) != len(m.order) {
		t.Errorf("visited %d of %d nodes", len(seen), len(m.order))
	}
}

func TestSearchMatchesAndJumps(t *testing.T) {
	m := sized(t, 100, 30)
	m = press(t, m, "/")
	if m.mode != modeSearch {
		t.Fatal("did not enter search mode")
	}
	for _, r := range "normalize" {
		m = press(t, m, string(r))
	}
	if len(m.matchIDs) != 1 {
		t.Fatalf("expected one match, got %d", len(m.matchIDs))
	}
	m = press(t, m, "enter")
	if m.mode != modeCanvas {
		t.Error("search did not close on enter")
	}
	if got := m.doc.Node(m.selected).Title(); got != "Normalize" {
		t.Errorf("jumped to %q", got)
	}
}

func TestSearchIsCaseInsensitiveAndCoversAllFields(t *testing.T) {
	m := sized(t, 100, 30)
	for _, q := range []string{"WAREHOUSE", "jsoncanvas.org", "Ingest pipeline"} {
		m.query = q
		if got := m.runSearch(); len(got.matchIDs) == 0 {
			t.Errorf("query %q matched nothing", q)
		}
	}
}

func TestEscapeClearsSearchState(t *testing.T) {
	m := sized(t, 100, 30)
	m.query = "queue"
	m = m.runSearch()
	if len(m.matchIDs) == 0 {
		t.Fatal("fixture no longer matches")
	}
	m = press(t, m, "esc")
	if len(m.matchIDs) != 0 || len(m.matches) != 0 {
		t.Error("escape left search state behind")
	}
}

func TestJumpMovesTowardsTheRequestedDirection(t *testing.T) {
	m := sized(t, 100, 30)
	// Start from the leftmost node in the top row.
	m.selected = "n1"
	start := m.doc.Node("n1")
	m = m.jump(geom.DirRight)
	if m.selected == "n1" {
		t.Fatal("jump did not move")
	}
	if got := m.doc.Node(m.selected); got.CenterX() <= start.CenterX() {
		t.Errorf("jumped right but landed at x=%v from x=%v", got.CenterX(), start.CenterX())
	}
}

func TestJumpAtTheEdgeIsANoop(t *testing.T) {
	m := sized(t, 100, 30)
	m.selected = "n4" // the highest node center on the canvas
	m = m.jump(geom.DirUp)
	if m.selected != "n4" {
		t.Errorf("jumped off the top of the canvas to %q", m.selected)
	}
}

func TestFocusPaneOpensAndScrolls(t *testing.T) {
	m := sized(t, 100, 30)
	m.selected = "n6" // the long note
	m = press(t, m, "enter")
	if m.mode != modeFocus || m.focus == nil {
		t.Fatal("focus pane did not open")
	}
	m = press(t, m, "j")
	m = press(t, m, "esc")
	if m.mode != modeCanvas || m.focus != nil {
		t.Error("focus pane did not close")
	}
}

func TestFocusPaneReadsFileNodes(t *testing.T) {
	m := sized(t, 100, 30)
	m.selected = "n4" // notes/warehouse.md, present in the test vault
	m = press(t, m, "enter")
	body := strings.Join(m.focus.lines, "\n")
	if !strings.Contains(ansi.Strip(body), "event_date") {
		t.Errorf("file contents were not read:\n%s", ansi.Strip(body))
	}
}

func TestFocusScrollStaysInBounds(t *testing.T) {
	f := &focusState{lines: make([]string, 10)}
	f.scroll(1000, 5)
	if f.off > 5 {
		t.Errorf("scrolled past the end to %d", f.off)
	}
	f.scroll(-1000, 5)
	if f.off != 0 {
		t.Errorf("scrolled above the start to %d", f.off)
	}
	// Fewer lines than the window: there is nowhere to scroll to.
	short := &focusState{lines: make([]string, 2)}
	short.scroll(5, 20)
	if short.off != 0 {
		t.Errorf("scrolled a pane that fits, to %d", short.off)
	}
}

func TestHelpOverlayIsDismissedByAnyKey(t *testing.T) {
	m := sized(t, 100, 30)
	m = press(t, m, "?")
	if m.mode != modeHelp {
		t.Fatal("help did not open")
	}
	m = press(t, m, "x")
	if m.mode != modeCanvas {
		t.Error("help did not close")
	}
}

func TestResolveFilePrefersTheVault(t *testing.T) {
	vault := filepath.Join("..", "..", "testdata", "vault")
	got, ok := ResolveFile("notes/warehouse.md", vault, "somewhere/else.canvas")
	if !ok {
		t.Fatalf("did not resolve, got %q", got)
	}
	if !strings.Contains(filepath.ToSlash(got), "testdata/vault/notes") {
		t.Errorf("resolved to %q", got)
	}
}

func TestResolveFileReportsMissingTargets(t *testing.T) {
	if _, ok := ResolveFile("nope.md", "", "testdata/x.canvas"); ok {
		t.Error("reported a missing file as found")
	}
	if _, ok := ResolveFile("", "", "x.canvas"); ok {
		t.Error("an empty path should not resolve")
	}
}

func TestParseSize(t *testing.T) {
	if w, h, err := ParseSize("120x40"); err != nil || w != 120 || h != 40 {
		t.Errorf("got %d %d %v", w, h, err)
	}
	for _, bad := range []string{"", "120", "axb", "0x10", "10x1"} {
		if _, _, err := ParseSize(bad); err == nil {
			t.Errorf("ParseSize(%q) should have failed", bad)
		}
	}
}

func TestDumpProducesAFullFrame(t *testing.T) {
	c, path := sample(t)
	out := Dump(c, path, "", DumpOpts{W: 90, H: 20, Theme: "dark"})
	lines := strings.Split(out, "\n")
	if len(lines) != 20 {
		t.Fatalf("got %d lines, want 20", len(lines))
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != 90 {
			t.Errorf("line %d is %d cells", i, got)
		}
	}
}

func TestReloadPicksUpChangesAndKeepsThePlace(t *testing.T) {
	// Having the canvas open in Obsidian and in cnvw at once is the normal
	// case, so a stale view is a bug rather than an inconvenience.
	dir := t.TempDir()
	path := filepath.Join(dir, "work.canvas")
	write := func(text string) {
		doc := `{"nodes":[
			{"id":"a","type":"text","x":0,"y":0,"width":400,"height":200,"text":"` + text + `"},
			{"id":"b","type":"text","x":800,"y":0,"width":400,"height":200,"text":"second"}
		]}`
		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("before")

	c, err := canvas.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := New(c, path, "", "dark")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	m.selected = "b"
	m = press(t, m, "right") // move the camera off centre
	camX := m.cam.X

	write("after")
	m = press(t, m, "r")

	if got := m.doc.Node("a").Text; got != "after" {
		t.Errorf("reload did not pick up the change, text is %q", got)
	}
	if m.selected != "b" {
		t.Errorf("reload lost the selection, now %q", m.selected)
	}
	if m.cam.X != camX {
		t.Errorf("reload moved the camera from %v to %v", camX, m.cam.X)
	}
}

func TestReloadFallsBackWhenTheSelectionIsGone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work.canvas")
	os.WriteFile(path, []byte(`{"nodes":[
		{"id":"a","type":"text","x":0,"y":0,"width":400,"height":200,"text":"a"},
		{"id":"gone","type":"text","x":800,"y":0,"width":400,"height":200,"text":"g"}
	]}`), 0o644)

	c, _ := canvas.Load(path)
	m := New(c, path, "", "dark")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	m.selected = "gone"

	os.WriteFile(path, []byte(`{"nodes":[
		{"id":"a","type":"text","x":0,"y":0,"width":400,"height":200,"text":"a"}
	]}`), 0o644)
	m = press(t, m, "r")

	if m.doc.Node(m.selected) == nil {
		t.Errorf("selection %q survived its node being deleted", m.selected)
	}
}

func TestReloadOfAnUnreadableFileKeepsTheOldCanvas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work.canvas")
	os.WriteFile(path, []byte(`{"nodes":[
		{"id":"a","type":"text","x":0,"y":0,"width":400,"height":200,"text":"ok"}
	]}`), 0o644)

	c, _ := canvas.Load(path)
	m := New(c, path, "", "dark")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	os.WriteFile(path, []byte(`{ this is not json`), 0o644)
	m = press(t, m, "r")

	if len(m.doc.Nodes) != 1 {
		t.Error("a failed reload discarded the canvas that was already open")
	}
	if m.notice == "" {
		t.Error("a failed reload said nothing")
	}
}
