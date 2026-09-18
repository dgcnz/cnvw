// Package mathtex renders LaTeX math spans as the nearest plain Unicode.
//
// Glamour is goldmark-based with no math extension, so a note written with
// "$J_x^\top J_x$" reaches the screen with its dollar signs and backslashes
// intact. A terminal cannot typeset mathematics and nothing upstream is going
// to change that, but most math in a research note is a handful of symbols,
// sub- and superscripts: "Jₓᵀ Jₓ" is a fair rendering of that example and a
// great deal more readable than the source.
//
// Where Unicode has no equivalent the LaTeX notation is kept as written. A
// half-translated expression is still readable; a wrong symbol is not.
package mathtex

import (
	"strings"
	"unicode"
)

// Render converts every math span in s, leaving the rest of the text alone.
//
// A span is "$...$" or "$$...$$". To avoid mangling prose about money, an
// opening delimiter must be followed by a non-space and the closing one
// preceded by a non-space, and a span may not cross a blank line.
func Render(s string) string {
	if !strings.ContainsRune(s, '$') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' {
			b.WriteByte(s[i])
			i++
			continue
		}
		delim := 1
		if strings.HasPrefix(s[i:], "$$") {
			delim = 2
		}
		body, width, ok := span(s[i:], delim)
		if !ok {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteString(Expr(body))
		i += width
	}
	return b.String()
}

// span matches a math span at the start of s, returning its body and the
// number of bytes it occupies including both delimiters.
func span(s string, delim int) (body string, width int, ok bool) {
	open := strings.Repeat("$", delim)
	rest := s[delim:]
	if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
		return "", 0, false
	}
	end := strings.Index(rest, open)
	if end <= 0 {
		return "", 0, false
	}
	body = rest[:end]
	if strings.Contains(body, "\n\n") {
		return "", 0, false
	}
	if last := body[len(body)-1]; last == ' ' || last == '\t' {
		return "", 0, false
	}
	return body, delim*2 + end, true
}

// Expr converts the body of a math span.
func Expr(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '\\':
			text, width := command(s[i:])
			b.WriteString(text)
			i += width

		case c == '^' || c == '_':
			arg, width := argument(s[i+1:])
			raised, ok := shift(Expr(arg), c == '^')
			if !ok {
				// No Unicode form: keep the notation rather than drop a
				// character or silently substitute the wrong one.
				b.WriteByte(c)
				b.WriteString(arg)
			} else {
				b.WriteString(raised)
			}
			i += 1 + width

		case c == '{' || c == '}':
			i++ // grouping braces carry no meaning once flattened

		default:
			b.WriteByte(c)
			i++
		}
	}
	return collapseSpaces(b.String())
}

// command converts one control sequence, returning its text and the bytes it
// consumed.
func command(s string) (string, int) {
	// A non-letter after the backslash is an escape such as "\{" or "\,".
	if len(s) > 1 && !isLetter(rune(s[1])) {
		if sym, ok := commands[string(s[1])]; ok {
			return sym, 2
		}
		return string(s[1]), 2
	}

	end := 1
	for end < len(s) && isLetter(rune(s[end])) {
		end++
	}
	name := s[1:end]

	switch name {
	case "frac", "dfrac", "tfrac":
		num, n := argument(s[end:])
		den, d := argument(s[end+n:])
		return "(" + Expr(num) + ")/(" + Expr(den) + ")", end + n + d
	case "mathbb", "mathbf", "mathrm", "mathcal", "mathit", "text", "textrm", "operatorname":
		arg, n := argument(s[end:])
		return alphabet(name, arg), end + n
	case "left", "right", "big", "Big", "bigg", "Bigg", "displaystyle", "limits":
		return "", end
	case "sqrt":
		arg, n := argument(s[end:])
		if n == 0 {
			return "√", end
		}
		return "√(" + Expr(arg) + ")", end + n
	}

	if sym, ok := commands[name]; ok {
		return sym, end
	}
	// Unknown command: keep it legible as written.
	return "\\" + name, end
}

// alphabet applies a font command to its argument.
func alphabet(name, arg string) string {
	var table map[string]string
	switch name {
	case "mathbb":
		table = blackboard
	case "mathcal":
		table = script
	default:
		return Expr(arg) // the other fonts have no plain-text distinction
	}
	if sym, ok := table[arg]; ok {
		return sym
	}
	return Expr(arg)
}

// argument reads a braced group, or the single character that follows when
// there are no braces, as LaTeX does.
func argument(s string) (string, int) {
	if s == "" {
		return "", 0
	}
	if s[0] != '{' {
		if s[0] == '\\' {
			end := 1
			for end < len(s) && isLetter(rune(s[end])) {
				end++
			}
			if end == 1 && end < len(s) {
				end++
			}
			return s[:end], end
		}
		r := []rune(s)
		return string(r[0]), len(string(r[0]))
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return s[1:i], i + 1
			}
		}
	}
	return s[1:], len(s) // unbalanced; take what there is
}

// shift raises or lowers text, reporting false when any character has no such
// form. It is all or nothing: a partly raised expression reads as a typo.
func shift(s string, up bool) (string, bool) {
	if s == "" {
		return "", false
	}
	table := subscripts
	if up {
		table = superscripts
	}
	var b strings.Builder
	for _, r := range s {
		shifted, ok := table[r]
		if !ok {
			return "", false
		}
		b.WriteRune(shifted)
	}
	return b.String(), true
}

// collapseSpaces tidies the gaps left where commands carried no glyph.
func collapseSpaces(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

func isLetter(r rune) bool { return unicode.IsLetter(r) }
