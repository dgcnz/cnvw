// Package geom converts between canvas pixel coordinates and terminal cells,
// and holds the camera that decides which part of the canvas is on screen.
package geom

import "math"

// A terminal cell stands in for this many canvas pixels at zoom 1.0. The
// values track Obsidian's text metrics rather than the visual aspect ratio of
// a cell: a canvas node tall enough for three lines of text in Obsidian should
// be tall enough for three lines here too. Deriving the vertical scale from a
// cell's visual 2:1 shape instead would squash a default 250x60 node into two
// rows, leaving no room for text inside its border.
const (
	PxPerCellX = 8.0
	PxPerCellY = 16.0
)

// Zoom limits and step. The step is geometric so each keypress feels the same
// at every scale.
const (
	MinZoom  = 0.02
	MaxZoom  = 4.0
	ZoomStep = 1.25
)

// Camera is the viewport over the canvas. X and Y are the canvas pixel
// coordinates at the center of the terminal viewport, which makes zooming
// about the center fall out for free.
type Camera struct {
	X, Y float64
	Zoom float64
}

// NewCamera returns a camera at the origin and natural zoom.
func NewCamera() Camera { return Camera{Zoom: 1} }

// Viewport is the size of the drawing area in terminal cells.
type Viewport struct {
	W, H int
}

// scaleX and scaleY are cells per canvas pixel along each axis.
func (c Camera) scaleX() float64 { return c.Zoom / PxPerCellX }
func (c Camera) scaleY() float64 { return c.Zoom / PxPerCellY }

// WorldToScreen converts a canvas point to fractional cell coordinates within
// the viewport.
func (c Camera) WorldToScreen(wx, wy float64, vp Viewport) (sx, sy float64) {
	sx = (wx-c.X)*c.scaleX() + float64(vp.W)/2
	sy = (wy-c.Y)*c.scaleY() + float64(vp.H)/2
	return
}

// ScreenToWorld converts cell coordinates back to canvas pixels. It is the
// inverse of WorldToScreen and is used for mouse hit tests and for keeping a
// point fixed while zooming.
func (c Camera) ScreenToWorld(sx, sy float64, vp Viewport) (wx, wy float64) {
	wx = (sx-float64(vp.W)/2)/c.scaleX() + c.X
	wy = (sy-float64(vp.H)/2)/c.scaleY() + c.Y
	return
}

// Rect is an axis-aligned rectangle in terminal cells, with Max exclusive.
type Rect struct {
	MinX, MinY, MaxX, MaxY int
}

// W and H are the rectangle's size in cells.
func (r Rect) W() int { return r.MaxX - r.MinX }
func (r Rect) H() int { return r.MaxY - r.MinY }

// Empty reports whether the rectangle covers no cells.
func (r Rect) Empty() bool { return r.W() <= 0 || r.H() <= 0 }

// Overlaps reports whether two rectangles share any cell.
func (r Rect) Overlaps(o Rect) bool {
	return r.MinX < o.MaxX && o.MinX < r.MaxX && r.MinY < o.MaxY && o.MinY < r.MaxY
}

// Project maps a canvas-pixel rectangle onto cells.
//
// Both corners are rounded independently and the size is taken as their
// difference, rather than rounding the position and the size separately. That
// keeps abutting nodes flush against each other and stops a box from
// twitching by a cell as the camera pans across a sub-cell boundary.
func (c Camera) Project(wx, wy, ww, wh float64, vp Viewport) Rect {
	x0, y0 := c.WorldToScreen(wx, wy, vp)
	x1, y1 := c.WorldToScreen(wx+ww, wy+wh, vp)
	return Rect{
		MinX: roundCell(x0),
		MinY: roundCell(y0),
		MaxX: roundCell(x1),
		MaxY: roundCell(y1),
	}
}

// cellLimit bounds projected coordinates. A node far outside the viewport is
// clipped anyway, so pinning it to a large but finite distance costs nothing
// and keeps a canvas with extreme coordinates, or a very low zoom, from
// overflowing the integer conversion.
const cellLimit = 1 << 20

func roundCell(v float64) int {
	switch {
	case math.IsNaN(v):
		return 0
	case v > cellLimit:
		return cellLimit
	case v < -cellLimit:
		return -cellLimit
	}
	return int(math.Round(v))
}

// Clamped returns the camera with its zoom forced back into range.
func (c Camera) Clamped() Camera {
	c.Zoom = math.Min(MaxZoom, math.Max(MinZoom, c.Zoom))
	return c
}

// ZoomBy scales the zoom by factor while holding the canvas point under
// (anchorX, anchorY), given in cells, stationary on screen.
func (c Camera) ZoomBy(factor float64, anchorX, anchorY float64, vp Viewport) Camera {
	wx, wy := c.ScreenToWorld(anchorX, anchorY, vp)
	out := c
	out.Zoom *= factor
	out = out.Clamped()
	// Re-place the camera so the anchor lands back where it started.
	nx, ny := out.ScreenToWorld(anchorX, anchorY, vp)
	out.X += wx - nx
	out.Y += wy - ny
	return out
}

// PanCells moves the camera by a whole number of cells, which keeps panning
// aligned to the grid regardless of zoom.
func (c Camera) PanCells(dx, dy int, vp Viewport) Camera {
	c.X += float64(dx) / c.scaleX()
	c.Y += float64(dy) / c.scaleY()
	return c
}

// Fit centers the camera on the given canvas rectangle and picks the largest
// zoom at which the whole thing fits, leaving a small margin.
func Fit(minX, minY, maxX, maxY int, vp Viewport) Camera {
	c := Camera{
		X:    float64(minX+maxX) / 2,
		Y:    float64(minY+maxY) / 2,
		Zoom: 1,
	}
	w := float64(maxX - minX)
	h := float64(maxY - minY)
	if w <= 0 || h <= 0 || vp.W <= 0 || vp.H <= 0 {
		return c
	}
	const margin = 0.94
	zx := float64(vp.W) * PxPerCellX / w
	zy := float64(vp.H) * PxPerCellY / h
	c.Zoom = math.Min(zx, zy) * margin
	return c.Clamped()
}

// ClampToBounds slides the camera so the viewport stays over the canvas.
//
// Centering on a node near an edge of the canvas otherwise spends half the
// screen on empty space beyond it. Along an axis where the canvas is smaller
// than the viewport there is nothing to slide, so it centers instead.
func (c Camera) ClampToBounds(minX, minY, maxX, maxY int, vp Viewport) Camera {
	halfW := float64(vp.W) / 2 / c.scaleX()
	halfH := float64(vp.H) / 2 / c.scaleY()

	if float64(maxX-minX) <= halfW*2 {
		c.X = float64(minX+maxX) / 2
	} else {
		c.X = math.Min(math.Max(c.X, float64(minX)+halfW), float64(maxX)-halfW)
	}
	if float64(maxY-minY) <= halfH*2 {
		c.Y = float64(minY+maxY) / 2
	} else {
		c.Y = math.Min(math.Max(c.Y, float64(minY)+halfH), float64(maxY)-halfH)
	}
	return c
}

// VisibleWorld returns the canvas rectangle currently on screen, grown by pad
// cells on every side so that nodes just off the edge are still considered.
func (c Camera) VisibleWorld(vp Viewport, pad int) (minX, minY, maxX, maxY float64) {
	minX, minY = c.ScreenToWorld(float64(-pad), float64(-pad), vp)
	maxX, maxY = c.ScreenToWorld(float64(vp.W+pad), float64(vp.H+pad), vp)
	return
}
