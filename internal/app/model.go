// Package app wires the canvas renderer to a Bubble Tea event loop.
package app

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/geom"
	"github.com/dgcnz/cnvw/internal/render"
)

type mode int

const (
	modeCanvas mode = iota
	modeSearch
	modeFocus
	modeHelp
)

// statusHeight is the single row reserved at the bottom for status and search.
const statusHeight = 1

// Model is the whole application state. The camera and the selection are kept
// independent: panning never changes what is selected, and jumping between
// nodes moves the camera without disturbing the zoom.
type Model struct {
	doc   *canvas.Canvas
	path  string
	vault string

	th        render.Theme
	themeName string // "auto", "dark" or "light"

	cam    geom.Camera
	w, h   int
	fitted bool

	selected string
	order    []string // node ids top-to-bottom, left-to-right

	mode     mode
	query    string
	matches  map[string]bool
	matchIDs []string
	matchIdx int

	edgeLabels bool
	focus      *focusState
	notice     string
}

// New builds a model for an already-parsed canvas.
func New(doc *canvas.Canvas, path, vault, themeName string) Model {
	m := Model{
		doc:        doc,
		path:       path,
		vault:      vault,
		themeName:  themeName,
		th:         render.ByName(themeName),
		cam:        geom.NewCamera(),
		matches:    map[string]bool{},
		edgeLabels: true,
	}
	m.order = readingOrder(doc)
	if len(m.order) > 0 {
		m.selected = m.order[0]
	}
	if len(doc.Warnings) > 0 {
		m.notice = doc.Warnings[0]
		if n := len(doc.Warnings); n > 1 {
			m.notice += fmt.Sprintf(" (+%d more)", n-1)
		}
	}
	return m
}

// readingOrder sorts nodes top-to-bottom then left-to-right, which is the
// order Tab walks through them in.
func readingOrder(doc *canvas.Canvas) []string {
	ids := make([]string, 0, len(doc.Nodes))
	for i := range doc.Nodes {
		ids = append(ids, doc.Nodes[i].ID)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := doc.Node(ids[i]), doc.Node(ids[j])
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	return ids
}

// Init requests the terminal background color so that "auto" can pick a theme.
func (m Model) Init() tea.Cmd {
	if m.themeName == "auto" {
		return tea.RequestBackgroundColor
	}
	return nil
}

// viewport is the drawing area, which is the window minus the status row.
func (m Model) viewport() geom.Viewport {
	return geom.Viewport{W: m.w, H: max(0, m.h-statusHeight)}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		// The opening view waits for the first real size, when the viewport
		// is finally known; fitting at construction would fit to nothing.
		if !m.fitted && m.w > 0 && m.h > 0 {
			m.fitted = true
			m = m.open()
		}
		return m, nil

	case tea.BackgroundColorMsg:
		if m.themeName == "auto" {
			if msg.IsDark() {
				m.th = render.Dark()
			} else {
				m.th = render.Light()
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseWheelMsg:
		return m.handleWheel(msg), nil

	case tea.MouseClickMsg:
		return m.handleClick(msg), nil
	}
	return m, nil
}

// handleWheel scrolls the focus pane, or pans the canvas; with ctrl held it
// zooms about the pointer.
func (m Model) handleWheel(msg tea.MouseWheelMsg) Model {
	mouse := msg.Mouse()
	up := mouse.Button == tea.MouseWheelUp
	down := mouse.Button == tea.MouseWheelDown

	if m.mode == modeFocus && m.focus != nil {
		switch {
		case up:
			m.focus.scroll(-3, m.h-statusHeight)
		case down:
			m.focus.scroll(3, m.h-statusHeight)
		}
		return m
	}

	if mouse.Mod&tea.ModCtrl != 0 {
		switch {
		case up:
			m.cam = m.cam.ZoomBy(geom.ZoomStep, float64(mouse.X), float64(mouse.Y), m.viewport())
		case down:
			m.cam = m.cam.ZoomBy(1/geom.ZoomStep, float64(mouse.X), float64(mouse.Y), m.viewport())
		}
		return m
	}

	switch {
	case up:
		m.cam = m.cam.PanCells(0, -3, m.viewport())
	case down:
		m.cam = m.cam.PanCells(0, 3, m.viewport())
	}
	return m
}

// handleClick selects whatever node is under the pointer.
func (m Model) handleClick(msg tea.MouseClickMsg) Model {
	mouse := msg.Mouse()
	if m.mode != modeCanvas || mouse.Button != tea.MouseLeft {
		return m
	}
	sc := render.Layout(m.doc, m.view())
	// Walk the visible nodes back to front so the topmost one wins, and prefer
	// a leaf over the group it sits inside.
	best := ""
	for _, id := range sc.Visible {
		r := sc.Rects[id]
		if mouse.X < r.MinX || mouse.X >= r.MaxX || mouse.Y < r.MinY || mouse.Y >= r.MaxY {
			continue
		}
		if best == "" || m.doc.Node(id).Type != canvas.TypeGroup {
			best = id
		}
	}
	if best != "" {
		m.selected = best
	}
	return m
}

// view assembles the render parameters for the current state.
func (m Model) view() render.View {
	return render.View{
		Cam:        m.cam,
		VP:         m.viewport(),
		SelectedID: m.selected,
		Matches:    m.matches,
		EdgeLabels: m.edgeLabels,
	}
}

// fit frames the whole canvas.
func (m Model) fit() Model {
	minX, minY, maxX, maxY, ok := m.doc.Bounds()
	if !ok {
		m.cam = geom.NewCamera()
		return m
	}
	m.cam = geom.Fit(minX, minY, maxX, maxY, m.viewport())
	return m
}

// reload re-reads the canvas from disk.
//
// The file is the one thing this program does not own: it is normal to have
// the canvas open in Obsidian and cnvw at the same time, and without this the
// view silently goes stale. The camera, zoom and selection are carried over so
// that reloading lands you where you were rather than back at the opening
// view; a selection whose node has since been deleted falls back to the first.
func (m Model) reload() Model {
	doc, err := canvas.Load(m.path)
	if err != nil {
		m.notice = "reload failed: " + err.Error()
		return m
	}

	next := New(doc, m.path, m.vault, m.themeName)
	next.th = m.th // keep the palette resolved for this terminal
	next.w, next.h = m.w, m.h
	next.fitted = m.fitted
	next.cam = m.cam
	next.edgeLabels = m.edgeLabels
	if doc.Node(m.selected) != nil {
		next.selected = m.selected
	}

	// The reload is worth confirming, but a problem with the file it just read
	// matters more.
	if next.notice == "" {
		next.notice = fmt.Sprintf("reloaded · %d nodes · %d edges",
			len(doc.Nodes), len(doc.Edges))
	}
	return next
}

// open picks the view the canvas is first shown at.
//
// It is the geometric fit unless that would shrink the nodes past the point of
// legibility, in which case it zooms in far enough for titles to read and
// centers on the first node instead. A canvas too large to be both complete
// and readable opens readable: the whole-canvas view is one keypress away at
// `f`, whereas an opening screen of anonymous blocks is a dead end.
func (m Model) open() Model {
	m = m.fit()

	sizes := make([]geom.Size, 0, len(m.doc.Nodes))
	for i := range m.doc.Nodes {
		n := &m.doc.Nodes[i]
		// Groups are containers, not cards; their size says nothing about
		// whether the canvas is readable.
		if n.Type != canvas.TypeGroup {
			sizes = append(sizes, geom.Size{W: n.Width, H: n.Height})
		}
	}

	legible := geom.LegibleZoom(sizes, geom.LegibleTitleCells)
	if legible <= m.cam.Zoom {
		return m
	}

	m.cam.Zoom = legible
	m.cam = m.cam.Clamped()
	// Zoomed past the fit, only part of the canvas is on screen, so start on
	// the node that is already selected rather than on empty space.
	if m.selected != "" {
		m = m.center(m.selected)
	}
	// That node is usually at a corner of the canvas, which would leave half
	// the screen showing nothing beyond it.
	if minX, minY, maxX, maxY, ok := m.doc.Bounds(); ok {
		m.cam = m.cam.ClampToBounds(minX, minY, maxX, maxY, m.viewport())
	}
	if m.notice == "" {
		m.notice = "zoomed in so titles read · f fits the whole canvas · ? help"
	}
	return m
}

// center moves the camera onto a node without changing the zoom.
func (m Model) center(id string) Model {
	n := m.doc.Node(id)
	if n == nil {
		return m
	}
	m.cam.X, m.cam.Y = n.CenterX(), n.CenterY()
	return m
}

// View renders the current mode.
func (m Model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	if m.w <= 0 || m.h <= 0 {
		return v
	}

	var body string
	switch m.mode {
	case modeHelp:
		body = m.helpView()
	case modeFocus:
		body = m.focusView()
	default:
		body = render.Render(m.doc, m.th, m.view())
	}

	v.Content = body + "\n" + m.statusView()
	return v
}

// focusView renders the reader pane over the whole window.
func (m Model) focusView() string {
	height := m.h - statusHeight
	title := lipgloss.NewStyle().Foreground(m.th.Accent).Bold(true).
		Render(truncate(m.focus.title, m.w))
	rows := []string{title, strings.Repeat("─", m.w)}
	rows = append(rows, m.focus.view(height-len(rows))...)
	return padRows(rows, height, m.w)
}

// statusView is the bottom row: either the search prompt or the state line.
func (m Model) statusView() string {
	st := lipgloss.NewStyle().Foreground(m.th.StatusFg).Background(m.th.StatusBg).Width(m.w)

	if m.mode == modeSearch {
		hits := ""
		if m.query != "" {
			hits = fmt.Sprintf("  %d matches", len(m.matchIDs))
		}
		return st.Render(truncate("/"+m.query+"▏"+hits, m.w))
	}
	if m.notice != "" {
		return lipgloss.NewStyle().Foreground(m.th.Match).Background(m.th.StatusBg).
			Width(m.w).Render(truncate(m.notice, m.w))
	}

	// In the reader the camera means nothing; where you are in the text does.
	if m.mode == modeFocus && m.focus != nil {
		shown := min(m.focus.off+m.h-statusHeight, len(m.focus.lines))
		return st.Render(truncate(fmt.Sprintf("%s  line %d of %d  ·  esc to return",
			m.focus.title, shown, len(m.focus.lines)), m.w))
	}

	left := fmt.Sprintf("%s  %d nodes  %d edges  zoom %.2f  (%.0f, %.0f)",
		filepath.Base(m.path), len(m.doc.Nodes), len(m.doc.Edges),
		m.cam.Zoom, m.cam.X, m.cam.Y)

	right := "? help"
	if n := m.doc.Node(m.selected); n != nil {
		right = truncate(n.Title(), max(0, m.w-ansi.StringWidth(left)-4))
	}
	if len(m.matchIDs) > 0 {
		right = fmt.Sprintf("%d/%d  %s", m.matchIdx+1, len(m.matchIDs), right)
	}

	gap := m.w - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return st.Render(truncate(left, m.w))
	}
	return st.Render(left + strings.Repeat(" ", gap) + right)
}

// padRows squares a block of lines off to exactly height rows of width cells.
// Every row is padded, not just the missing ones: a short row would otherwise
// leave whatever the terminal drew there on the previous frame.
func padRows(rows []string, height, width int) string {
	out := make([]string, height)
	for i := range out {
		var l string
		if i < len(rows) {
			l = rows[i]
		}
		switch n := width - ansi.StringWidth(l); {
		case n > 0:
			l += strings.Repeat(" ", n)
		case n < 0:
			l = ansi.Truncate(l, width, "")
		}
		out[i] = l
	}
	return strings.Join(out, "\n")
}

// truncate cuts a string to w display cells, counting wide runes as the two
// cells they actually occupy so the status row cannot overflow.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "")
}
