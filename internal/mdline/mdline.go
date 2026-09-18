// Package mdline turns Markdown into short styled lines that fit inside a
// canvas node.
//
// This deliberately is not a Markdown renderer. Glamour does that job well in
// the focus pane, where there is a comfortable width to work with, but inside
// a box that may be twelve cells wide its margins, fixed width and cost per
// render all work against us. What a node actually needs is narrower: drop the
// syntax, keep the emphasis, so a heading reads as bold text instead of
// "## Heading".
package mdline

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/dgcnz/cnvw/internal/mathtex"
)

// Style is a set of text attributes applied to a span.
type Style uint8

// The attributes a span can carry. Accent marks text the theme should tint
// with the node's color, which is how headings are distinguished.
const (
	Bold Style = 1 << iota
	Italic
	Code
	Underline
	Dim
	Accent
)

// Has reports whether every attribute in s is present.
func (s Style) Has(o Style) bool { return s&o == o }

// Span is a run of text sharing one style.
type Span struct {
	Text  string
	Style Style
}

// Line is a single rendered line, split into styled runs.
type Line []Span

// Text returns the line's plain text, with styling discarded.
func (l Line) Text() string {
	var b strings.Builder
	for _, s := range l {
		b.WriteString(s.Text)
	}
	return b.String()
}

// Width returns the display width of the line in cells.
func (l Line) Width() int { return ansi.StringWidth(l.Text()) }

// block is one source line after its leading Markdown marker is understood but
// before wrapping.
type block struct {
	prefix  string // literal text placed at the start of the first line
	indent  string // repeated on continuation lines to keep the prefix hanging
	content string // the inline Markdown still to be parsed
	base    style2 // attributes applied to the whole block
	rule    bool   // draw a horizontal rule instead of text
	literal bool   // fenced code: emit as-is, no inline parsing
}

type style2 = Style

// Render formats Markdown to fit width cells, returning at most one Line per
// visual row. A width of zero or less yields nothing.
func Render(md string, width int) []Line {
	if width <= 0 {
		return nil
	}
	var out []Line
	for _, b := range blocks(mathtex.Render(md)) {
		out = append(out, b.render(width)...)
	}
	return out
}

// blocks classifies each source line by its leading marker.
func blocks(md string) []block {
	var out []block
	inFence := false
	for _, raw := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)

		// Fences toggle literal mode and are themselves never drawn; showing
		// the ``` markers inside a small box wastes two of very few rows.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			out = append(out, block{content: line, base: Code, literal: true})
			continue
		}

		switch {
		case trimmed == "":
			out = append(out, block{})

		case isRule(trimmed):
			out = append(out, block{rule: true})

		case strings.HasPrefix(trimmed, "#"):
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			rest := strings.TrimSpace(trimmed[level:])
			// Every heading is bold; the deeper ones lose the accent tint so
			// the top-level headings still stand out against them.
			base := Bold | Accent
			if level >= 3 {
				base = Bold | Dim
			}
			out = append(out, block{content: rest, base: base})

		case strings.HasPrefix(trimmed, ">"):
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			out = append(out, block{prefix: "│ ", indent: "  ", content: rest, base: Dim})

		default:
			if pre, rest, ok := listItem(line); ok {
				out = append(out, block{
					prefix:  pre,
					indent:  strings.Repeat(" ", ansi.StringWidth(pre)),
					content: rest,
				})
				continue
			}
			out = append(out, block{content: trimmed})
		}
	}
	return out
}

// isRule matches a thematic break: three or more of - * _ and nothing else.
func isRule(s string) bool {
	if len(s) < 3 {
		return false
	}
	c := s[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	return strings.Trim(s, string(c)) == ""
}

// listItem recognises bullet, numbered and task list markers, returning the
// replacement prefix and the remaining content.
func listItem(line string) (prefix, rest string, ok bool) {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	indent = strings.ReplaceAll(indent, "\t", "  ")
	s := strings.TrimLeft(line, " \t")

	for _, m := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(s, m) {
			body := strings.TrimPrefix(s, m)
			// Task list items keep their checkbox, redrawn as a glyph.
			switch {
			case strings.HasPrefix(body, "[ ] "):
				return indent + "☐ ", strings.TrimPrefix(body, "[ ] "), true
			case strings.HasPrefix(body, "[x] "), strings.HasPrefix(body, "[X] "):
				return indent + "☒ ", body[4:], true
			}
			return indent + "• ", body, true
		}
	}

	// An ordered item keeps its number, which carries meaning a bullet loses.
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			continue
		}
		if i > 0 && (s[i] == '.' || s[i] == ')') && i+1 < len(s) && s[i+1] == ' ' {
			return indent + s[:i+1] + " ", s[i+2:], true
		}
		break
	}
	return "", "", false
}

// render wraps one block to width cells.
func (b block) render(width int) []Line {
	if b.rule {
		return []Line{{{Text: strings.Repeat("─", width), Style: Dim}}}
	}
	if b.content == "" && b.prefix == "" {
		return []Line{{}}
	}
	if b.literal {
		return []Line{{{Text: truncate(b.content, width), Style: b.base}}}
	}

	spans := inline(b.content, b.base)
	avail := width - ansi.StringWidth(b.prefix)
	if avail < 1 {
		avail = 1
	}
	wrapped := wrap(spans, avail)

	out := make([]Line, 0, len(wrapped))
	for i, l := range wrapped {
		lead := b.indent
		if i == 0 {
			lead = b.prefix
		}
		if lead != "" {
			l = append(Line{{Text: lead, Style: b.base}}, l...)
		}
		out = append(out, l)
	}
	return out
}

// inline parses emphasis, code, links and wiki links into styled spans.
func inline(s string, base Style) []Span {
	var out []Span
	var buf strings.Builder
	cur := base

	flush := func() {
		if buf.Len() > 0 {
			out = append(out, Span{Text: buf.String(), Style: cur})
			buf.Reset()
		}
	}
	// toggle ends the current run and flips an attribute for what follows.
	toggle := func(attr Style) {
		flush()
		cur ^= attr
	}

	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "**"), strings.HasPrefix(s[i:], "__"):
			toggle(Bold)
			i += 2

		case strings.HasPrefix(s[i:], "=="):
			toggle(Bold | Accent)
			i += 2

		case s[i] == '*' || s[i] == '_':
			toggle(Italic)
			i++

		case s[i] == '`':
			// Code spans are literal, so consume to the closing tick rather
			// than re-entering the parser and stripping markers inside them.
			end := strings.IndexByte(s[i+1:], '`')
			if end < 0 {
				buf.WriteByte(s[i])
				i++
				continue
			}
			flush()
			out = append(out, Span{Text: s[i+1 : i+1+end], Style: cur | Code})
			i += end + 2

		case strings.HasPrefix(s[i:], "![["), strings.HasPrefix(s[i:], "[["):
			open := 2
			if s[i] == '!' {
				open = 3
			}
			end := strings.Index(s[i:], "]]")
			if end < 0 {
				buf.WriteByte(s[i])
				i++
				continue
			}
			flush()
			out = append(out, Span{Text: WikiTarget(s[i+open : i+end]), Style: cur | Underline})
			i += end + 2

		case s[i] == '[':
			text, rest, ok := mdLink(s[i:])
			if !ok {
				buf.WriteByte(s[i])
				i++
				continue
			}
			flush()
			out = append(out, Span{Text: text, Style: cur | Underline})
			i += len(s[i:]) - len(rest)

		default:
			buf.WriteByte(s[i])
			i++
		}
	}
	flush()
	return out
}

// mdLink matches "[text](url)" at the start of s, returning the display text
// and whatever follows the link. The URL is dropped: in a box this narrow the
// text is all that fits, and the focus pane can show the target.
func mdLink(s string) (text, rest string, ok bool) {
	close := strings.IndexByte(s, ']')
	if close < 0 || close+1 >= len(s) || s[close+1] != '(' {
		return "", "", false
	}
	end := strings.IndexByte(s[close+1:], ')')
	if end < 0 {
		return "", "", false
	}
	return s[1:close], s[close+1+end+1:], true
}

// wrap breaks styled spans at word boundaries to fit width cells, splitting
// any single word that is too long to fit on a line by itself.
func wrap(spans []Span, width int) []Line {
	var lines []Line
	var cur Line
	curW := 0

	push := func() {
		lines = append(lines, cur)
		cur, curW = nil, 0
	}
	add := func(text string, st Style) {
		if n := len(cur); n > 0 && cur[n-1].Style == st {
			cur[n-1].Text += text
			return
		}
		cur = append(cur, Span{Text: text, Style: st})
	}

	for _, sp := range spans {
		for _, word := range splitKeepSpaces(sp.Text) {
			w := ansi.StringWidth(word)
			if strings.TrimSpace(word) == "" {
				// Never start a line with the space that ended the last one.
				if curW > 0 && curW+w <= width {
					add(word, sp.Style)
					curW += w
				}
				continue
			}
			if curW+w > width && curW > 0 {
				push()
			}
			for w > width {
				// A word longer than the line gets hard-split rather than
				// pushed off the edge.
				head := truncate(word, width)
				add(head, sp.Style)
				push()
				word = word[len(head):]
				w = ansi.StringWidth(word)
			}
			add(word, sp.Style)
			curW += w
		}
	}
	if len(cur) > 0 {
		push()
	}
	if len(lines) == 0 {
		lines = append(lines, Line{})
	}
	return lines
}

// splitKeepSpaces splits into words and the whitespace runs between them, so
// the wrapper can decide per gap whether to keep it.
func splitKeepSpaces(s string) []string {
	var out []string
	start, inSpace := 0, false
	for i, r := range s {
		sp := r == ' ' || r == '\t'
		if i > 0 && sp != inSpace {
			out = append(out, s[start:i])
			start = i
		}
		inSpace = sp
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// truncate cuts s to at most width cells.
func truncate(s string, width int) string {
	if ansi.StringWidth(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "")
}

// WikiTarget reduces a wiki link target to what should be shown for it.
//
// "[[page|alias]]" displays the alias, as Obsidian does. Without one, a bare
// vault path is nearly all directory, so only the final segment is kept:
// "100 Reference notes/101 Literature/2ndMatch - Finetuning..." says much less
// than "2ndMatch - Finetuning..." in the space available.
func WikiTarget(target string) string {
	if bar := strings.IndexByte(target, '|'); bar >= 0 {
		return strings.TrimSpace(target[bar+1:])
	}
	if slash := strings.LastIndexByte(target, '/'); slash >= 0 {
		target = target[slash+1:]
	}
	// A heading or block reference is part of the name, but the separator
	// reads better as a space.
	return strings.TrimSpace(strings.ReplaceAll(target, "#", " § "))
}

// ResolveWikiLinks rewrites wiki links in Markdown source to their display
// text. Glamour knows nothing of Obsidian's syntax, so a reader pane fed the
// raw source shows the whole vault path inside double brackets.
func ResolveWikiLinks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		open := 2
		switch {
		case strings.HasPrefix(s[i:], "![["):
			open = 3
		case strings.HasPrefix(s[i:], "[["):
		default:
			b.WriteByte(s[i])
			i++
			continue
		}
		end := strings.Index(s[i:], "]]")
		if end < 0 {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteString(WikiTarget(s[i+open : i+end]))
		i += end + 2
	}
	return b.String()
}

// Truncate shortens a plain string to width cells, marking the cut with an
// ellipsis when there is room for one.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return ansi.Truncate(s, width-1, "") + "…"
}
