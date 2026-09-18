package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dgcnz/cnvw/internal/geom"
)

// panDivisor sets the pan step as a fraction of the viewport, so a keypress
// moves the same proportion of the screen at any zoom. A quarter is large
// enough that crossing a screen is four taps, which is why there is no
// separate half-screen or full-screen paging key: on a canvas you cover
// distance by zooming out, not by paging.
const panDivisor = 4

// handleKey dispatches a keypress to the handler for the current mode.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Any keypress clears a startup notice, which has been read by then.
	m.notice = ""

	switch m.mode {
	case modeSearch:
		return m.searchKey(msg)
	case modeFocus:
		return m.focusKey(msg)
	case modeHelp:
		if k := msg.String(); k == "ctrl+c" || k == "q" {
			return m, tea.Quit
		}
		m.mode = modeCanvas
		return m, nil
	}
	return m.canvasKey(msg)
}

// canvasKey handles navigation over the canvas.
func (m Model) canvasKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	vp := m.viewport()
	stepX := max(1, vp.W/panDivisor)
	stepY := max(1, vp.H/panDivisor)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "left":
		m.cam = m.cam.PanCells(-stepX, 0, vp)
	case "right":
		m.cam = m.cam.PanCells(stepX, 0, vp)
	case "up":
		m.cam = m.cam.PanCells(0, -stepY, vp)
	case "down":
		m.cam = m.cam.PanCells(0, stepY, vp)

	// Shift-direction jumps to the nearest node that way. The letter forms
	// stay as a fallback: not every terminal reports a shifted arrow, and
	// without them the jump would simply be unreachable there.
	case "shift+left", "H":
		m = m.jump(geom.DirLeft)
	case "shift+right", "L":
		m = m.jump(geom.DirRight)
	case "shift+up", "K":
		m = m.jump(geom.DirUp)
	case "shift+down", "J":
		m = m.jump(geom.DirDown)

	case "tab":
		m = m.cycle(1)
	case "shift+tab":
		m = m.cycle(-1)

	case "+", "=":
		m.cam = m.cam.ZoomBy(geom.ZoomStep, float64(vp.W)/2, float64(vp.H)/2, vp)
	case "-", "_":
		m.cam = m.cam.ZoomBy(1/geom.ZoomStep, float64(vp.W)/2, float64(vp.H)/2, vp)

	case "f":
		m = m.fit()
	case "z":
		if m.selected != "" {
			m.cam.Zoom = 1
			m = m.center(m.selected)
		}

	case "enter":
		if n := m.doc.Node(m.selected); n != nil {
			m.focus = newFocus(n, m.doc, m.vault, m.path, m.th, max(20, m.w-4))
			m.mode = modeFocus
		}

	case "/":
		m.mode = modeSearch
		m.query = ""

	case "n":
		m = m.step(1)
	case "N":
		m = m.step(-1)

	case "r":
		m = m.reload()
	case "e":
		m.edgeLabels = !m.edgeLabels
	case "?":
		m.mode = modeHelp

	case "esc":
		m.query = ""
		m.matches = map[string]bool{}
		m.matchIDs = nil
	}
	return m, nil
}

// focusKey scrolls the reader pane.
func (m Model) focusKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	height := m.h - statusHeight
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q", "enter":
		m.mode = modeCanvas
		m.focus = nil
	case "down":
		m.focus.scroll(1, height)
	case "up":
		m.focus.scroll(-1, height)
	case "ctrl+d":
		m.focus.scroll(height/2, height)
	case "ctrl+u":
		m.focus.scroll(-height/2, height)
	case " ", "pgdown":
		m.focus.scroll(height, height)
	case "pgup":
		m.focus.scroll(-height, height)
	case "g":
		m.focus.off = 0
	case "G":
		m.focus.scroll(len(m.focus.lines), height)
	}
	return m, nil
}

// searchKey edits the query, matching as it is typed so the highlight tracks
// the input without waiting for Enter.
func (m Model) searchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.mode = modeCanvas
		m.query = ""
		m.matches = map[string]bool{}
		m.matchIDs = nil
		return m, nil

	case "enter":
		m.mode = modeCanvas
		if len(m.matchIDs) > 0 {
			m.matchIdx = 0
			m.selected = m.matchIDs[0]
			m = m.center(m.selected)
		}
		return m, nil

	case "backspace":
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
		}
		return m.runSearch(), nil
	}

	if t := msg.Key().Text; t != "" {
		m.query += t
		return m.runSearch(), nil
	}
	return m, nil
}

// runSearch recomputes the match set for the current query.
func (m Model) runSearch() Model {
	m.matches = map[string]bool{}
	m.matchIDs = nil
	m.matchIdx = 0
	q := strings.ToLower(strings.TrimSpace(m.query))
	if q == "" {
		return m
	}
	// Walk in reading order so n and N move through the canvas predictably
	// rather than in whatever order the document happened to store nodes.
	for _, id := range m.order {
		if strings.Contains(m.doc.Node(id).SearchText(), q) {
			m.matches[id] = true
			m.matchIDs = append(m.matchIDs, id)
		}
	}
	return m
}

// step moves to the next or previous search match, wrapping around.
func (m Model) step(delta int) Model {
	if len(m.matchIDs) == 0 {
		return m
	}
	m.matchIdx = (m.matchIdx + delta + len(m.matchIDs)) % len(m.matchIDs)
	m.selected = m.matchIDs[m.matchIdx]
	return m.center(m.selected)
}

// cycle moves the selection through the reading order.
func (m Model) cycle(delta int) Model {
	if len(m.order) == 0 {
		return m
	}
	idx := 0
	for i, id := range m.order {
		if id == m.selected {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(m.order)) % len(m.order)
	m.selected = m.order[idx]
	return m.center(m.selected)
}

// jump selects the nearest node in a direction and brings it into view.
func (m Model) jump(dir geom.Dir) Model {
	cur := m.doc.Node(m.selected)
	if cur == nil {
		return m.cycle(1)
	}
	// Candidates are every other node, indexed so the winner maps back.
	pts := make([]geom.Point, 0, len(m.order))
	ids := make([]string, 0, len(m.order))
	for _, id := range m.order {
		if id == m.selected {
			continue
		}
		n := m.doc.Node(id)
		pts = append(pts, geom.Point{X: n.CenterX(), Y: n.CenterY()})
		ids = append(ids, id)
	}
	i := geom.Nearest(geom.Point{X: cur.CenterX(), Y: cur.CenterY()}, pts, dir)
	if i < 0 {
		return m
	}
	m.selected = ids[i]
	return m.center(m.selected)
}

// helpKeys is the key reference, shown in the help overlay.
var helpKeys = [][2]string{
	{"arrows", "pan"},
	{"shift+arrows", "jump to the nearest node that way"},
	{"tab / shift+tab", "cycle nodes in reading order"},
	{"+ / -", "zoom in / out"},
	{"f", "fit the whole canvas"},
	{"z", "zoom to the selected node"},
	{"enter", "open the selected node"},
	{"/", "search"},
	{"n / N", "next / previous match"},
	{"r", "reload the file from disk"},
	{"e", "toggle edge labels"},
	{"mouse", "click to select, wheel to pan, ctrl+wheel to zoom"},
	{"?", "this help"},
	{"q", "quit"},
}

// helpView renders the key reference.
func (m Model) helpView() string {
	height := m.h - statusHeight
	key := lipgloss.NewStyle().Foreground(m.th.Accent).Bold(true)
	desc := lipgloss.NewStyle().Foreground(m.th.Fg)

	rows := []string{
		lipgloss.NewStyle().Foreground(m.th.Accent).Bold(true).Render("cnvw"),
		"",
	}
	for _, kv := range helpKeys {
		rows = append(rows, "  "+key.Render(pad(kv[0], 18))+desc.Render(kv[1]))
	}
	rows = append(rows, "", lipgloss.NewStyle().Foreground(m.th.Muted).
		Render("  any key to return"))
	return padRows(rows, height, m.w)
}

func pad(s string, w int) string {
	if n := w - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
