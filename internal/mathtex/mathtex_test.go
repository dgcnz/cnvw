package mathtex

import "testing"

func TestRendersRealExpression(t *testing.T) {
	// The span that prompted this, from labeled-tree.canvas.
	in := `Match random projections of $J_x^\top J_x$ between models.`
	want := "Match random projections of Jₓᵀ Jₓ between models."
	if got := Render(in); got != want {
		t.Errorf("Render(%q)\n got %q\nwant %q", in, got, want)
	}
}

func TestSymbolsAndScripts(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`$\alpha + \beta$`, "α + β"},
		{`$x^2$`, "x²"},
		{`$x_i$`, "xᵢ"},
		{`$x^{n+1}$`, "xⁿ⁺¹"},
		{`$\sum_{i=1}^{n} x_i$`, "∑ᵢ₌₁ⁿ xᵢ"},
		{`$\mathbb{R}^d$`, "ℝᵈ"},
		{`$\mathcal{L}$`, "ℒ"},
		{`$\frac{a}{b}$`, "(a)/(b)"},
		{`$A \times B$`, "A × B"},
		{`$x \leq y$`, "x ≤ y"},
		{`$\nabla_\theta J$`, "∇_\\theta J"},
		{`$\sqrt{2}$`, "√(2)"},
		{`$$E = mc^2$$`, "E = mc²"},
	} {
		if got := Render(tc.in); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestKeepsNotationItCannotRender(t *testing.T) {
	// Unicode has no subscript b, and no glyph for an unknown command. Half a
	// translation is still readable; a wrong symbol is not.
	for _, tc := range []struct{ in, want string }{
		{`$x_b$`, "x_b"},
		{`$\weirdcommand$`, `\weirdcommand`},
		{`$x^{qq}$`, "x^qq"},
	} {
		if got := Render(tc.in); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLeavesProseAndMoneyAlone(t *testing.T) {
	// A span must open on a non-space and close on one, or ordinary text about
	// prices gets mangled into symbols.
	for _, in := range []string{
		"it costs $5 or $10 depending",
		"a lone $ sign",
		"$ spaced $",
		"no math here at all",
		"price is $9.99",
	} {
		if got := Render(in); got != in {
			t.Errorf("Render(%q) changed it to %q", in, got)
		}
	}
}

func TestDoesNotSpanAParagraphBreak(t *testing.T) {
	in := "cost $5\n\nlater $6 more"
	if got := Render(in); got != in {
		t.Errorf("a span crossed a blank line: %q", got)
	}
}

func TestMalformedInputIsSafe(t *testing.T) {
	for _, in := range []string{
		`$x^$`, `$_$`, `${$`, `$\frac{a}$`, `$\$`, `$$`, `$`, `$\mathbb{$`,
		`$x^{$`, `$\left($`,
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Render(%q) panicked: %v", in, r)
				}
			}()
			Render(in)
		}()
	}
}

func TestUnicodeInputSurvives(t *testing.T) {
	in := "日本語 $x^2$ テキスト"
	if got := Render(in); got != "日本語 x² テキスト" {
		t.Errorf("got %q", got)
	}
}
