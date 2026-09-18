package app

import (
	"fmt"
	"strings"

	"github.com/dgcnz/cnvw/internal/canvas"
	"github.com/dgcnz/cnvw/internal/render"
)

// DumpOpts describes a single non-interactive frame.
type DumpOpts struct {
	W, H  int
	Zoom  float64 // 0 fits the whole canvas
	Theme string
}

// Dump renders one frame and returns it, without starting a terminal session.
// It exists so that rendering can be checked in a pipe or a test, where an
// interactive program cannot be inspected.
func Dump(doc *canvas.Canvas, path, vault string, o DumpOpts) string {
	m := New(doc, path, vault, o.Theme)
	m.w, m.h = o.W, o.H
	m = m.open()
	m.notice = ""
	if o.Zoom > 0 {
		m.cam.Zoom = o.Zoom
		m.cam = m.cam.Clamped()
	}
	m.selected = ""
	return render.Render(m.doc, m.th, m.view()) + "\n" + m.statusView()
}

// ParseSize reads a "WxH" geometry string.
func ParseSize(s string) (w, h int, err error) {
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("size must look like WxH, got %q", s)
	}
	if _, err = fmt.Sscanf(parts[0], "%d", &w); err != nil {
		return 0, 0, fmt.Errorf("bad width in %q", s)
	}
	if _, err = fmt.Sscanf(parts[1], "%d", &h); err != nil {
		return 0, 0, fmt.Errorf("bad height in %q", s)
	}
	if w < 1 || h < 2 {
		return 0, 0, fmt.Errorf("size %q is too small", s)
	}
	return w, h, nil
}
