// Package canvas parses and models JSON Canvas 1.0 documents.
//
// Spec: https://jsoncanvas.org/spec/1.0/
package canvas

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dgcnz/cnvw/internal/mathtex"
)

// Node types defined by the spec.
const (
	TypeText  = "text"
	TypeFile  = "file"
	TypeLink  = "link"
	TypeGroup = "group"
)

// Side is the side of a node an edge attaches to.
type Side int

// Sides, with SideAuto meaning the document left it unspecified and we infer
// it from the relative position of the two nodes.
const (
	SideAuto Side = iota
	SideTop
	SideRight
	SideBottom
	SideLeft
)

func parseSide(s string) Side {
	switch s {
	case "top":
		return SideTop
	case "right":
		return SideRight
	case "bottom":
		return SideBottom
	case "left":
		return SideLeft
	}
	return SideAuto
}

func (s Side) String() string {
	switch s {
	case SideTop:
		return "top"
	case SideRight:
		return "right"
	case SideBottom:
		return "bottom"
	case SideLeft:
		return "left"
	}
	return "auto"
}

// Opposite returns the side facing this one.
func (s Side) Opposite() Side {
	switch s {
	case SideTop:
		return SideBottom
	case SideRight:
		return SideLeft
	case SideBottom:
		return SideTop
	case SideLeft:
		return SideRight
	}
	return SideAuto
}

// Horizontal reports whether the side is on a vertical edge of the box, so an
// edge leaving it travels horizontally.
func (s Side) Horizontal() bool { return s == SideLeft || s == SideRight }

// Node is a single canvas node. The spec gives every node a position and size
// in pixels; type-specific fields are populated only for the matching type.
type Node struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Color  string `json:"color,omitempty"`

	Text string `json:"text,omitempty"` // type text

	File    string `json:"file,omitempty"`    // type file
	Subpath string `json:"subpath,omitempty"` // type file

	URL string `json:"url,omitempty"` // type link

	Label           string `json:"label,omitempty"`           // type group
	Background      string `json:"background,omitempty"`      // type group
	BackgroundStyle string `json:"backgroundStyle,omitempty"` // type group
}

// Right and Bottom are the exclusive far edges of the node in canvas pixels.
func (n *Node) Right() int  { return n.X + n.Width }
func (n *Node) Bottom() int { return n.Y + n.Height }

// CenterX and CenterY locate the middle of the node in canvas pixels.
func (n *Node) CenterX() float64 { return float64(n.X) + float64(n.Width)/2 }
func (n *Node) CenterY() float64 { return float64(n.Y) + float64(n.Height)/2 }

// Title is a short single-line name for the node, used in the status bar,
// search results and at the title level of detail. Markdown markers are
// stripped so a heading does not show up as "## Heading".
func (n *Node) Title() string {
	switch n.Type {
	case TypeGroup:
		if n.Label != "" {
			return n.Label
		}
		return "(group)"
	case TypeFile:
		return n.File + n.Subpath
	case TypeLink:
		return n.URL
	}
	for _, line := range strings.Split(mathtex.Render(n.Text), "\n") {
		if t := stripMarkers(strings.TrimSpace(line)); t != "" {
			return t
		}
	}
	return "(empty)"
}

// stripMarkers removes the leading block markers and inline emphasis runs that
// would otherwise clutter a one-line title.
func stripMarkers(s string) string {
	s = strings.TrimLeft(s, "#> \t")
	for _, bullet := range []string{"- ", "* ", "+ "} {
		s = strings.TrimPrefix(s, bullet)
	}
	s = strings.TrimPrefix(s, "[ ] ")
	s = strings.TrimPrefix(s, "[x] ")
	s = strings.NewReplacer("**", "", "__", "", "`", "", "==", "").Replace(s)
	return strings.TrimSpace(s)
}

// SearchText is everything in the node worth matching a query against.
func (n *Node) SearchText() string {
	return strings.ToLower(strings.Join([]string{n.Text, n.File, n.Subpath, n.URL, n.Label}, "\n"))
}

// Edge connects two nodes. FromEnd and ToEnd carry the spec defaults already
// applied: no arrow at the start, an arrow at the end.
type Edge struct {
	ID       string `json:"id"`
	FromNode string `json:"fromNode"`
	FromSide string `json:"fromSide,omitempty"`
	FromEnd  string `json:"fromEnd,omitempty"`
	ToNode   string `json:"toNode"`
	ToSide   string `json:"toSide,omitempty"`
	ToEnd    string `json:"toEnd,omitempty"`
	Color    string `json:"color,omitempty"`
	Label    string `json:"label,omitempty"`
}

// From and To give the parsed attachment sides.
func (e *Edge) From() Side { return parseSide(e.FromSide) }
func (e *Edge) To() Side   { return parseSide(e.ToSide) }

// ArrowAtStart and ArrowAtEnd apply the spec's endpoint defaults.
func (e *Edge) ArrowAtStart() bool { return e.FromEnd == "arrow" }
func (e *Edge) ArrowAtEnd() bool   { return e.ToEnd != "none" }

// Canvas is a parsed document. Nodes are kept in document order, which the
// spec defines as ascending z-index.
type Canvas struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`

	byID map[string]*Node

	// Warnings collects non-fatal problems found while loading, such as edges
	// pointing at nodes that do not exist. Shown once on startup.
	Warnings []string `json:"-"`
}

// Load reads and parses a .canvas file.
func Load(path string) (*Canvas, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse builds a Canvas from raw JSON. Both top-level arrays are optional per
// the spec, so an empty object is a valid, empty canvas.
func Parse(data []byte) (*Canvas, error) {
	var c Canvas
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse canvas: %w", err)
	}
	c.index()
	return &c, nil
}

// index builds the id lookup and drops entries the rest of the program cannot
// make sense of, recording why.
func (c *Canvas) index() {
	c.byID = make(map[string]*Node, len(c.Nodes))
	// Duplicates are tracked separately: the id map cannot serve as the seen
	// set because compacting the slice below invalidates any pointer taken
	// before it settles.
	seen := make(map[string]bool, len(c.Nodes))
	kept := c.Nodes[:0]
	for i := range c.Nodes {
		n := &c.Nodes[i]
		if n.ID == "" {
			c.Warnings = append(c.Warnings, "dropped a node with no id")
			continue
		}
		if seen[n.ID] {
			c.Warnings = append(c.Warnings, fmt.Sprintf("dropped duplicate node id %q", n.ID))
			continue
		}
		seen[n.ID] = true
		if n.Type == "" {
			n.Type = TypeText
		}
		kept = append(kept, *n)
	}
	c.Nodes = kept
	// Re-index after the slice settles, so the pointers stay valid.
	for i := range c.Nodes {
		c.byID[c.Nodes[i].ID] = &c.Nodes[i]
	}

	edges := c.Edges[:0]
	for _, e := range c.Edges {
		if c.byID[e.FromNode] == nil || c.byID[e.ToNode] == nil {
			c.Warnings = append(c.Warnings, fmt.Sprintf("dropped edge %q: unknown endpoint", e.ID))
			continue
		}
		edges = append(edges, e)
	}
	c.Edges = edges
}

// Node looks up a node by id, returning nil when it is absent.
func (c *Canvas) Node(id string) *Node { return c.byID[id] }

// Bounds returns the bounding box of every node in canvas pixels. The second
// result is false for an empty canvas, where no box is meaningful.
func (c *Canvas) Bounds() (minX, minY, maxX, maxY int, ok bool) {
	for i := range c.Nodes {
		n := &c.Nodes[i]
		if !ok {
			minX, minY, maxX, maxY, ok = n.X, n.Y, n.Right(), n.Bottom(), true
			continue
		}
		minX = min(minX, n.X)
		minY = min(minY, n.Y)
		maxX = max(maxX, n.Right())
		maxY = max(maxY, n.Bottom())
	}
	return
}
