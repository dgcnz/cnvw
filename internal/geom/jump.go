package geom

import "math"

// Dir is a direction for node-to-node navigation.
type Dir int

// The four jump directions.
const (
	DirUp Dir = iota
	DirDown
	DirLeft
	DirRight
)

// Point is a location in canvas pixels.
type Point struct {
	X, Y float64
}

// perpWeight biases the search towards nodes that are more or less straight
// ahead. A candidate off to the side has to be considerably closer to win.
const perpWeight = 2.0

// Nearest picks the index of the best candidate to jump to from `from` in the
// given direction, or -1 when nothing lies that way.
//
// Candidates inside a 45-degree cone along the direction of travel are
// preferred, and only if the cone is empty does the search widen to anything
// forward at all. Scoring by distance alone would send "up" to a node barely
// higher but far off to one side, which is never what the keypress meant;
// scoring by cone alone would refuse to move on a sparse canvas.
func Nearest(from Point, candidates []Point, dir Dir) int {
	best, bestScore := -1, math.Inf(1)
	wide, wideScore := -1, math.Inf(1)

	for i, c := range candidates {
		var along, perp float64
		switch dir {
		case DirUp:
			along, perp = from.Y-c.Y, math.Abs(c.X-from.X)
		case DirDown:
			along, perp = c.Y-from.Y, math.Abs(c.X-from.X)
		case DirLeft:
			along, perp = from.X-c.X, math.Abs(c.Y-from.Y)
		case DirRight:
			along, perp = c.X-from.X, math.Abs(c.Y-from.Y)
		}
		if along <= 0 {
			continue // behind us, or exactly level
		}
		score := along + perpWeight*perp
		if perp <= along {
			if score < bestScore {
				best, bestScore = i, score
			}
			continue
		}
		if score < wideScore {
			wide, wideScore = i, score
		}
	}
	if best >= 0 {
		return best
	}
	return wide
}
