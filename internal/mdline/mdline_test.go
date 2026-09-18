package mdline

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// text joins the plain text of every rendered line.
func text(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Text()
	}
	return out
}

// styleOf returns the style covering the first span containing want.
func styleOf(lines []Line, want string) (Style, bool) {
	for _, l := range lines {
		for _, sp := range l {
			if strings.Contains(sp.Text, want) {
				return sp.Style, true
			}
		}
	}
	return 0, false
}

func TestSyntaxIsStrippedNotShown(t *testing.T) {
	// The whole point of this package: a node should read as "Ingest", never
	// as "## Ingest".
	for _, tc := range []struct{ in, want string }{
		{"## Ingest", "Ingest"},
		{"**bold**", "bold"},
		{"*italic*", "italic"},
		{"`code`", "code"},
		{"[label](https://example.com)", "label"},
		{"[[schema-registry]]", "schema-registry"},
		{"[[page|alias]]", "alias"},
		{"![[image.png]]", "image.png"},
		// A bare vault path is nearly all directory; the name is the part
		// worth the cells.
		{"[[100 Reference notes/101 Literature/2ndMatch]]", "2ndMatch"},
		{"==highlight==", "highlight"},
	} {
		got := text(Render(tc.in, 60))
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("Render(%q) = %q, want [%q]", tc.in, got, tc.want)
		}
	}
}

func TestHeadingsAreBold(t *testing.T) {
	st, ok := styleOf(Render("## Ingest", 40), "Ingest")
	if !ok || !st.Has(Bold) {
		t.Errorf("heading style = %v, want bold", st)
	}
	// Deeper headings drop the accent so the top levels still stand out.
	deep, _ := styleOf(Render("#### Detail", 40), "Detail")
	if deep.Has(Accent) {
		t.Errorf("h4 should not carry the accent, got %v", deep)
	}
}

func TestEmphasisCarriesThrough(t *testing.T) {
	lines := Render("plain **bold** more", 40)
	if st, _ := styleOf(lines, "bold"); !st.Has(Bold) {
		t.Errorf("bold span not marked bold")
	}
	if st, _ := styleOf(lines, "plain"); st.Has(Bold) {
		t.Errorf("surrounding text picked up bold")
	}
}

func TestCodeSpansAreLiteral(t *testing.T) {
	// Markers inside a code span must survive; stripping them would change
	// the code being shown.
	got := text(Render("run `a **b** c` now", 40))
	if len(got) != 1 || !strings.Contains(got[0], "a **b** c") {
		t.Errorf("code span was reformatted: %q", got)
	}
}

func TestListMarkers(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"- item", "• item"},
		{"* item", "• item"},
		{"1. first", "1. first"},
		{"- [ ] todo", "☐ todo"},
		{"- [x] done", "☒ done"},
	} {
		got := text(Render(tc.in, 40))
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("Render(%q) = %q, want [%q]", tc.in, got, tc.want)
		}
	}
}

func TestBlockQuoteAndRule(t *testing.T) {
	if got := text(Render("> quoted", 40)); got[0] != "│ quoted" {
		t.Errorf("quote = %q", got)
	}
	got := text(Render("---", 10))
	if got[0] != strings.Repeat("─", 10) {
		t.Errorf("rule = %q", got)
	}
}

func TestFencesAreDropped(t *testing.T) {
	// The ``` markers would waste two of very few rows.
	got := text(Render("```sql\nSELECT 1\n```", 40))
	if len(got) != 1 || got[0] != "SELECT 1" {
		t.Errorf("fenced block = %q", got)
	}
}

func TestNoLineExceedsTheWidth(t *testing.T) {
	const width = 14
	md := "## A heading that will not fit\n\n- a bullet item long enough to wrap twice over\n\nsupercalifragilisticexpialidocious"
	for _, l := range Render(md, width) {
		if w := l.Width(); w > width {
			t.Errorf("line %q is %d cells, limit is %d", l.Text(), w, width)
		}
	}
}

func TestWrapHangsListContinuations(t *testing.T) {
	got := text(Render("- alpha beta gamma delta", 12))
	if len(got) < 2 {
		t.Fatalf("expected a wrap, got %q", got)
	}
	if !strings.HasPrefix(got[0], "• ") {
		t.Errorf("first line missing the bullet: %q", got[0])
	}
	if !strings.HasPrefix(got[1], "  ") {
		t.Errorf("continuation is not indented under the bullet: %q", got[1])
	}
}

func TestOverlongWordIsSplitNotDropped(t *testing.T) {
	got := text(Render("supercalifragilistic", 6))
	if strings.Join(got, "") != "supercalifragilistic" {
		t.Errorf("hard split lost characters: %q", got)
	}
}

func TestUnterminatedMarkersDoNotPanic(t *testing.T) {
	for _, in := range []string{"**unclosed", "`unclosed", "[[unclosed", "[text](", "==", "*"} {
		if got := Render(in, 20); got == nil {
			t.Errorf("Render(%q) returned nil", in)
		}
	}
}

func TestResolveWikiLinks(t *testing.T) {
	// Glamour knows nothing of Obsidian syntax, so the reader pane is fed
	// source that has already had these rewritten.
	for _, tc := range []struct{ in, want string }{
		{"see [[notes/deep/page|the page]] now", "see the page now"},
		{"see [[notes/deep/page]] now", "see page now"},
		{"![[img/diagram.png]]", "diagram.png"},
		{"[[page#Heading]]", "page § Heading"},
		{"no links here", "no links here"},
		{"unterminated [[link", "unterminated [[link"},
	} {
		if got := ResolveWikiLinks(tc.in); got != tc.want {
			t.Errorf("ResolveWikiLinks(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMathIsRenderedInsideNodes(t *testing.T) {
	// A canvas box gets the same treatment as the reader pane.
	got := text(Render("energy is $E = mc^2$", 40))
	if len(got) != 1 || got[0] != "energy is E = mc²" {
		t.Errorf("got %q", got)
	}
}

func TestZeroWidthRendersNothing(t *testing.T) {
	if got := Render("anything", 0); got != nil {
		t.Errorf("expected nil at width 0, got %q", text(got))
	}
}

func TestTruncateMarksTheCut(t *testing.T) {
	for _, tc := range []struct {
		in   string
		w    int
		want string
	}{
		{"short", 10, "short"},
		{"truncate me", 5, "trun…"},
		{"x", 1, "x"},
		{"xy", 1, "…"},
		{"anything", 0, ""},
	} {
		if got := Truncate(tc.in, tc.w); got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.w, got, tc.want)
		}
	}
}

func TestTruncateRespectsWideRunes(t *testing.T) {
	if w := ansi.StringWidth(Truncate("日本語テキスト", 5)); w > 5 {
		t.Errorf("truncated to %d cells, limit is 5", w)
	}
}
