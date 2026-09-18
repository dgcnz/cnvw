package render

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/dgcnz/cnvw/internal/canvas"
)

// Theme is the palette used to draw the canvas. The JSON Canvas spec leaves
// the six preset colors undefined on purpose so that each application can pick
// values that suit it, so both themes carry their own set.
type Theme struct {
	Name string

	Fg       color.Color // body text
	Muted    color.Color // secondary text, such as a file subpath
	Dim      color.Color // borders and chrome with nothing to say
	Border   color.Color // default node border
	Bg       color.Color // background fill
	Accent   color.Color // selection
	Match    color.Color // search hits
	Edge     color.Color // default edge line
	Group    color.Color // group outlines and labels
	StatusBg color.Color
	StatusFg color.Color

	presets [7]color.Color
}

// Dark is the palette for dark terminal backgrounds.
func Dark() Theme {
	return Theme{
		Name:     "dark",
		Fg:       lipgloss.Color("#c9d1d9"),
		Muted:    lipgloss.Color("#8b949e"),
		Dim:      lipgloss.Color("#4d555e"),
		Border:   lipgloss.Color("#6e7681"),
		Bg:       lipgloss.Color("#0d1117"),
		Accent:   lipgloss.Color("#58a6ff"),
		Match:    lipgloss.Color("#f0c674"),
		Edge:     lipgloss.Color("#57606a"),
		Group:    lipgloss.Color("#484f58"),
		StatusBg: lipgloss.Color("#161b22"),
		StatusFg: lipgloss.Color("#c9d1d9"),
		presets: [7]color.Color{
			canvas.PresetNone:   nil,
			canvas.PresetRed:    lipgloss.Color("#ff6b6b"),
			canvas.PresetOrange: lipgloss.Color("#ffa94d"),
			canvas.PresetYellow: lipgloss.Color("#ffd43b"),
			canvas.PresetGreen:  lipgloss.Color("#51cf66"),
			canvas.PresetCyan:   lipgloss.Color("#3bc9db"),
			canvas.PresetPurple: lipgloss.Color("#cc5de8"),
		},
	}
}

// Light is the palette for light terminal backgrounds.
func Light() Theme {
	return Theme{
		Name:     "light",
		Fg:       lipgloss.Color("#24292f"),
		Muted:    lipgloss.Color("#57606a"),
		Dim:      lipgloss.Color("#afb8c1"),
		Border:   lipgloss.Color("#8c959f"),
		Bg:       lipgloss.Color("#ffffff"),
		Accent:   lipgloss.Color("#0969da"),
		Match:    lipgloss.Color("#9a6700"),
		Edge:     lipgloss.Color("#8c959f"),
		Group:    lipgloss.Color("#afb8c1"),
		StatusBg: lipgloss.Color("#eaeef2"),
		StatusFg: lipgloss.Color("#24292f"),
		presets: [7]color.Color{
			canvas.PresetNone:   nil,
			canvas.PresetRed:    lipgloss.Color("#d1242f"),
			canvas.PresetOrange: lipgloss.Color("#bc4c00"),
			canvas.PresetYellow: lipgloss.Color("#9a6700"),
			canvas.PresetGreen:  lipgloss.Color("#1a7f37"),
			canvas.PresetCyan:   lipgloss.Color("#0c8599"),
			canvas.PresetPurple: lipgloss.Color("#8250df"),
		},
	}
}

// ByName returns the named theme, defaulting to dark.
func ByName(name string) Theme {
	if name == "light" {
		return Light()
	}
	return Dark()
}

// Resolve turns a document color into a terminal color, falling back to the
// given default when the document did not specify one.
func (t Theme) Resolve(c canvas.Color, fallback color.Color) color.Color {
	if c.Hex != "" {
		return lipgloss.Color(c.Hex)
	}
	if c.Preset != canvas.PresetNone {
		return t.presets[c.Preset]
	}
	return fallback
}
