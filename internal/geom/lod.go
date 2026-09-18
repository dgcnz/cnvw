package geom

// Tier is the level of detail a node is drawn at, chosen from how many cells
// the node occupies on screen rather than from the zoom alone. A large node
// stays readable when a small one beside it has already collapsed.
type Tier int

// The tiers, in increasing order of detail.
const (
	TierHidden Tier = iota // too small to occupy a cell
	TierBlock              // a solid colored block
	TierMini               // borderless: a title line and an underline
	TierTitle              // a border and one truncated line
	TierFull               // a border and wrapped body text
)

// Cell thresholds for each tier. A bordered box needs three rows before a line
// of text fits between its sides, so anything shorter drops the border rather
// than render an empty frame.
const (
	fullMinW  = 12
	titleMinW = 6
	borderH   = 3
	miniMinW  = 3
	miniMinH  = 1
)

// TierFor picks the level of detail for a node occupying r.
func TierFor(r Rect) Tier {
	w, h := r.W(), r.H()
	switch {
	case w >= fullMinW && h >= borderH:
		return TierFull
	case w >= titleMinW && h >= borderH:
		return TierTitle
	case w >= miniMinW && h >= miniMinH:
		return TierMini
	case w >= 1 && h >= 1:
		return TierBlock
	}
	return TierHidden
}

// TierForGroup picks the level of detail for a group. A group never collapses
// to a solid block the way a leaf node does, because filling its rectangle
// would bury the nodes it contains; it degrades to a bare outline instead.
func TierForGroup(r Rect) Tier {
	if t := TierFor(r); t >= TierTitle {
		return t
	}
	if r.W() >= 2 && r.H() >= 2 {
		return TierTitle
	}
	return TierHidden
}
