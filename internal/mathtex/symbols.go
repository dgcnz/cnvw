package mathtex

// commands maps LaTeX control sequences to their closest single Unicode
// character. Only entries that genuinely read better as a symbol are listed;
// anything absent is left as written, which is more useful than a wrong guess.
var commands = map[string]string{
	// Greek, lower case.
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε",
	"varepsilon": "ε", "zeta": "ζ", "eta": "η", "theta": "θ", "vartheta": "ϑ",
	"iota": "ι", "kappa": "κ", "lambda": "λ", "mu": "μ", "nu": "ν", "xi": "ξ",
	"pi": "π", "rho": "ρ", "sigma": "σ", "tau": "τ", "upsilon": "υ",
	"phi": "φ", "varphi": "φ", "chi": "χ", "psi": "ψ", "omega": "ω",

	// Greek, upper case.
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ",
	"Pi": "Π", "Sigma": "Σ", "Upsilon": "Υ", "Phi": "Φ", "Psi": "Ψ",
	"Omega": "Ω",

	// Operators and relations.
	"times": "×", "div": "÷", "cdot": "·", "pm": "±", "mp": "∓",
	"leq": "≤", "le": "≤", "geq": "≥", "ge": "≥", "neq": "≠", "ne": "≠",
	"approx": "≈", "equiv": "≡", "sim": "∼", "simeq": "≃", "propto": "∝",
	"ll": "≪", "gg": "≫", "circ": "∘", "ast": "∗", "star": "⋆",
	"oplus": "⊕", "otimes": "⊗", "odot": "⊙",

	// Sets and logic.
	"in": "∈", "notin": "∉", "ni": "∋", "subset": "⊂", "subseteq": "⊆",
	"supset": "⊃", "supseteq": "⊇", "cup": "∪", "cap": "∩",
	"emptyset": "∅", "varnothing": "∅", "forall": "∀", "exists": "∃",
	"neg": "¬", "land": "∧", "lor": "∨", "setminus": "∖",

	// Big operators and calculus.
	"sum": "∑", "prod": "∏", "int": "∫", "iint": "∬", "oint": "∮",
	"partial": "∂", "nabla": "∇", "infty": "∞", "sqrt": "√",
	"lim": "lim", "max": "max", "min": "min", "arg": "arg",
	"log": "log", "exp": "exp", "ln": "ln", "sin": "sin", "cos": "cos",
	"tan": "tan", "det": "det", "dim": "dim", "deg": "deg",

	// Arrows.
	"to": "→", "rightarrow": "→", "leftarrow": "←", "leftrightarrow": "↔",
	"Rightarrow": "⇒", "Leftarrow": "⇐", "Leftrightarrow": "⇔",
	"mapsto": "↦", "uparrow": "↑", "downarrow": "↓", "implies": "⇒",

	// Linear algebra and miscellany.
	"top": "⊤", "bot": "⊥", "perp": "⊥", "parallel": "∥", "angle": "∠",
	"hbar": "ℏ", "ell": "ℓ", "Re": "ℜ", "Im": "ℑ", "aleph": "ℵ",
	"dots": "…", "ldots": "…", "cdots": "⋯", "vdots": "⋮", "ddots": "⋱",
	"prime": "′", "degree": "°", "checkmark": "✓",

	// Spacing commands, which have no glyph of their own.
	"quad": " ", "qquad": "  ", ",": " ", ";": " ", ":": " ", "!": "",
	" ": " ", "&": " ", "\\": " ",
}

// blackboard and script alphabets, used by \mathbb and \mathcal.
var blackboard = map[string]string{
	"R": "ℝ", "N": "ℕ", "Z": "ℤ", "Q": "ℚ", "C": "ℂ", "P": "ℙ",
	"E": "𝔼", "H": "ℍ", "F": "𝔽", "1": "𝟙",
}

var script = map[string]string{
	"A": "𝒜", "B": "ℬ", "C": "𝒞", "D": "𝒟", "E": "ℰ", "F": "ℱ", "G": "𝒢",
	"H": "ℋ", "I": "ℐ", "J": "𝒥", "K": "𝒦", "L": "ℒ", "M": "ℳ", "N": "𝒩",
	"O": "𝒪", "P": "𝒫", "Q": "𝒬", "R": "ℛ", "S": "𝒮", "T": "𝒯", "U": "𝒰",
	"V": "𝒱", "W": "𝒲", "X": "𝒳", "Y": "𝒴", "Z": "𝒵",
}

// superscripts and subscripts hold the raised and lowered forms Unicode
// actually provides. The coverage is incomplete by design of the standard:
// there is no subscript "b" or "c", so an expression needing one keeps its
// LaTeX notation rather than losing a character silently.
var superscripts = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴', '5': '⁵',
	'6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾', 'n': 'ⁿ', 'i': 'ⁱ',
	'a': 'ᵃ', 'b': 'ᵇ', 'c': 'ᶜ', 'd': 'ᵈ', 'e': 'ᵉ', 'f': 'ᶠ', 'g': 'ᵍ',
	'h': 'ʰ', 'j': 'ʲ', 'k': 'ᵏ', 'l': 'ˡ', 'm': 'ᵐ', 'o': 'ᵒ', 'p': 'ᵖ',
	'r': 'ʳ', 's': 'ˢ', 't': 'ᵗ', 'u': 'ᵘ', 'v': 'ᵛ', 'w': 'ʷ', 'x': 'ˣ',
	'y': 'ʸ', 'z': 'ᶻ',
	'A': 'ᴬ', 'B': 'ᴮ', 'D': 'ᴰ', 'E': 'ᴱ', 'G': 'ᴳ', 'H': 'ᴴ', 'I': 'ᴵ',
	'J': 'ᴶ', 'K': 'ᴷ', 'L': 'ᴸ', 'M': 'ᴹ', 'N': 'ᴺ', 'O': 'ᴼ', 'P': 'ᴾ',
	'R': 'ᴿ', 'T': 'ᵀ', 'U': 'ᵁ', 'V': 'ⱽ', 'W': 'ᵂ',
	// Transpose is written \top but means a raised T, and reads as one.
	'⊤': 'ᵀ', '∗': '*', '′': '′',
}

var subscripts = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄', '5': '₅',
	'6': '₆', '7': '₇', '8': '₈', '9': '₉',
	'+': '₊', '-': '₋', '=': '₌', '(': '₍', ')': '₎',
	'a': 'ₐ', 'e': 'ₑ', 'h': 'ₕ', 'i': 'ᵢ', 'j': 'ⱼ', 'k': 'ₖ', 'l': 'ₗ',
	'm': 'ₘ', 'n': 'ₙ', 'o': 'ₒ', 'p': 'ₚ', 'r': 'ᵣ', 's': 'ₛ', 't': 'ₜ',
	'u': 'ᵤ', 'v': 'ᵥ', 'x': 'ₓ',
}
