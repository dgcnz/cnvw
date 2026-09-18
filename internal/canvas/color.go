package canvas

import "strings"

// Preset identifies one of the six colors the spec reserves as numbers. The
// spec deliberately leaves the actual values undefined, so the render theme
// picks them.
type Preset int

// The preset colors, in spec order. PresetNone means the node either had no
// color or gave an explicit hex value.
const (
	PresetNone Preset = iota
	PresetRed
	PresetOrange
	PresetYellow
	PresetGreen
	PresetCyan
	PresetPurple
)

// Color is a parsed canvasColor: either one of the six presets, or a literal
// hex string, or neither when the attribute was absent.
type Color struct {
	Preset Preset
	Hex    string // "#RGB" or "#RRGGBB", empty unless the document gave one
}

// ParseColor reads the canvasColor encoding: a digit "1".."6" selects a
// preset, a leading "#" is taken as a literal hex color, anything else is
// treated as no color at all.
func ParseColor(s string) Color {
	s = strings.TrimSpace(s)
	switch s {
	case "1":
		return Color{Preset: PresetRed}
	case "2":
		return Color{Preset: PresetOrange}
	case "3":
		return Color{Preset: PresetYellow}
	case "4":
		return Color{Preset: PresetGreen}
	case "5":
		return Color{Preset: PresetCyan}
	case "6":
		return Color{Preset: PresetPurple}
	}
	if strings.HasPrefix(s, "#") && (len(s) == 4 || len(s) == 7) {
		return Color{Hex: strings.ToUpper(s)}
	}
	return Color{}
}

// IsZero reports whether the document specified no usable color, in which case
// the theme's default for the node type applies.
func (c Color) IsZero() bool { return c.Preset == PresetNone && c.Hex == "" }

// Color returns the node's parsed color.
func (n *Node) ParsedColor() Color { return ParseColor(n.Color) }

// ParsedColor returns the edge's parsed color.
func (e *Edge) ParsedColor() Color { return ParseColor(e.Color) }
