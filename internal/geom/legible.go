package geom

import "sort"

// LegibleTitleCells is how many cells a node needs across before its title
// carries meaning rather than just marking that something is there. Fourteen
// or so characters is roughly where a title stops being an abbreviation.
const LegibleTitleCells = 16

// Size is a node's extent in canvas pixels.
type Size struct {
	W, H int
}

// LegibleZoom returns the zoom at which half the nodes can show a useful part
// of their title.
//
// A purely geometric fit answers "how do I get everything on screen", which on
// a large canvas shrinks every node past the point where it can say anything:
// the result is an accurate picture of the layout that carries no information.
// This answers the other question, "how close do I have to be for the cards to
// mean something", and the opening view takes whichever is closer.
//
// The median is what makes this safe to apply automatically. Requiring every
// node to be legible would let one narrow outlier zoom the whole canvas in to
// nothing; the median keeps the answer tied to the canvas as a whole.
func LegibleZoom(sizes []Size, titleCells int) float64 {
	if len(sizes) == 0 || titleCells <= 0 {
		return 0
	}
	zooms := make([]float64, 0, len(sizes))
	for _, s := range sizes {
		if s.W <= 0 || s.H <= 0 {
			continue
		}
		// Wide enough for the title, and tall enough to occupy a row at all.
		z := float64(titleCells) * PxPerCellX / float64(s.W)
		if byHeight := PxPerCellY / float64(s.H); byHeight > z {
			z = byHeight
		}
		zooms = append(zooms, z)
	}
	if len(zooms) == 0 {
		return 0
	}
	sort.Float64s(zooms)
	return zooms[len(zooms)/2]
}
