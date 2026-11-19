package fleetui

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The style rule of the fleet interface, as this file enforces it over every
// stylesheet under assets/.
//
// Colour. A colour is written as a literal (a hex value, a colour function
// such as rgb() or hsl(), or a colour keyword such as white) only as the
// value of a custom property, and only in fleet-shell.css, which declares
// the theme. Everywhere else a colour is var(--name). The keywords
// transparent, currentColor, inherit and none are not colours of their own
// and stay allowed.
//
// Gap. A gap is the value of gap, row-gap, column-gap, and of margin and
// padding in every long and short form. Each part of a gap is 0, auto,
// inherit, one of var(--space-1) to var(--space-6), or the negative of one,
// written calc(-1 * var(--space-N)). A size is not a gap: widths, heights,
// offsets (top, right, bottom, left, inset), border widths, radii, font
// sizes and percentages are free.
//
// A declaration is read as "property: value;", so every declaration ends
// with a semicolon, the last of a block included.

// styleTokenFile is the one stylesheet that declares literal colours, and
// stylePageFile stands for any other sheet in the cases below.
const (
	styleTokenFile = "fleet-shell.css"
	stylePageFile  = "page.css"
)

var (
	styleComment     = regexp.MustCompile(`(?s)/\*.*?\*/`)
	styleDeclaration = regexp.MustCompile(`(--[A-Za-z0-9-]+|[A-Za-z][A-Za-z-]*)\s*:\s*([^;{}]+);`)
	styleColorValue  = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|\b(?:rgb|rgba|hsl|hsla|hwb|lab|lch|oklab|oklch|color|color-mix)\(`)
	styleWord        = regexp.MustCompile(`[A-Za-z]+`)
	styleGapPart     = regexp.MustCompile(`^(?:0|auto|inherit|var\(--space-[1-6]\)|calc\(-1 \* var\(--space-[1-6]\)\))$`)
	styleGapSplit    = regexp.MustCompile(`calc\([^)]*\)\)|\S+`)
)

// styleColorWords holds the colour keywords a stylesheet is most likely to
// reach for. A keyword outside the set is still a literal colour by the rule
// above; the set is what this test can see.
var styleColorWords = map[string]bool{
	"black": true, "white": true, "gray": true, "grey": true, "silver": true,
	"red": true, "green": true, "blue": true, "yellow": true, "orange": true,
	"purple": true, "pink": true, "brown": true, "cyan": true, "magenta": true,
	"lime": true, "navy": true, "teal": true, "aqua": true, "maroon": true,
	"olive": true, "fuchsia": true, "gold": true, "crimson": true, "tomato": true,
	"lightgray": true, "lightgrey": true, "darkgray": true, "darkgrey": true,
	"whitesmoke": true, "gainsboro": true, "dimgray": true, "dimgrey": true,
}

// styleCounts is what one stylesheet held, for the floors that keep the walk
// from passing over nothing.
type styleCounts struct {
	declarations int
	gaps         int
	literals     int
}

// isGapProperty reports whether a property's value is a gap by the rule.
func isGapProperty(property string) bool {
	switch property {
	case "gap", "row-gap", "column-gap":
		return true
	}
	if strings.HasPrefix(property, "margin") {
		return true
	}

	return strings.HasPrefix(property, "padding")
}

// holdsColorLiteral reports whether a value writes a colour as a literal.
func holdsColorLiteral(value string) bool {
	if styleColorValue.MatchString(value) {
		return true
	}
	for _, word := range styleWord.FindAllString(value, -1) {
		if styleColorWords[strings.ToLower(word)] {
			return true
		}
	}

	return false
}

// styleFindings reads one stylesheet and answers one line per declaration
// that breaks the rule, with what the sheet held.
func styleFindings(name, css string) ([]string, styleCounts) {
	var (
		findings []string
		counts   styleCounts
	)

	body := styleComment.ReplaceAllString(css, "")
	for _, match := range styleDeclaration.FindAllStringSubmatch(body, -1) {
		property, value := match[1], strings.TrimSpace(match[2])
		counts.declarations++

		if strings.HasPrefix(property, "--") {
			if !holdsColorLiteral(value) {
				continue
			}
			counts.literals++
			if name != styleTokenFile {
				findings = append(findings, name+": "+property+": "+value+
					" declares a literal colour; only "+styleTokenFile+" declares colours")
			}

			continue
		}

		if holdsColorLiteral(value) {
			findings = append(findings, name+": "+property+": "+value+
				" writes a literal colour; use a theme variable")
		}

		if !isGapProperty(property) {
			continue
		}
		counts.gaps++

		parts := styleGapSplit.FindAllString(strings.TrimSuffix(value, "!important"), -1)
		for _, part := range parts {
			if !styleGapPart.MatchString(part) {
				findings = append(findings, name+": "+property+": "+value+
					" holds the gap "+part+"; a gap is 0, auto or var(--space-1) to var(--space-6)")
			}
		}
	}

	return findings, counts
}

// TestFleetStylesUseVariablesOnly reads every stylesheet the package embeds.
//
// VALIDATES: AC-5 -- outside the variable declarations of fleet-shell.css no
// fleet style writes a colour as a literal, and every gap is one of the six
// spacing variables, 0 or auto.
// PREVENTS: a page family adding a colour or a spacing value of its own,
// which is how the pages of the interface drift apart.
func TestFleetStylesUseVariablesOnly(t *testing.T) {
	sheets, err := fs.Glob(assetsFS, "assets/*.css")
	require.NoError(t, err)
	require.Contains(t, sheets, "assets/"+styleTokenFile, "the token stylesheet must be among the sheets read")

	var total styleCounts
	for _, sheet := range sheets {
		css, readErr := fs.ReadFile(assetsFS, sheet)
		require.NoError(t, readErr)

		findings, counts := styleFindings(strings.TrimPrefix(sheet, "assets/"), string(css))
		for _, finding := range findings {
			t.Error(finding)
		}

		total.declarations += counts.declarations
		total.gaps += counts.gaps
		total.literals += counts.literals
	}

	// The floors make a walk that read nothing a failure: the shell sheet
	// alone holds more than these.
	assert.GreaterOrEqual(t, total.declarations, 200, "declarations read")
	assert.GreaterOrEqual(t, total.gaps, 40, "gap declarations read")
	assert.GreaterOrEqual(t, total.literals, 25, "literal colours read in the variable declarations")
}

// TestFleetStyleRuleRefuses feeds the rule declarations it must refuse and
// declarations it must accept.
//
// VALIDATES: the rule finds each kind of literal colour and each gap outside
// the spacing variables, and passes what it allows.
// PREVENTS: TestFleetStylesUseVariablesOnly passing because its patterns
// match nothing.
func TestFleetStyleRuleRefuses(t *testing.T) {
	cases := []struct {
		name     string
		sheet    string
		css      string
		findings int
	}{
		{"hex colour", stylePageFile, ".a { color: #fff; }", 1},
		{"colour function", stylePageFile, ".a { background: rgba(0, 0, 0, 0.5); }", 1},
		{"colour keyword", stylePageFile, ".a { border: 1px solid white; }", 1},
		{"literal in a variable of another sheet", stylePageFile, ":root { --mine: #123456; }", 1},
		{"literal in a variable of the token sheet", styleTokenFile, ":root { --mine: #123456; }", 0},
		{"variable colour", stylePageFile, ".a { color: var(--accent); border: 1px solid transparent; }", 0},
		{"gap in rem", stylePageFile, ".a { gap: 0.5rem; }", 1},
		{"padding in px", stylePageFile, ".a { padding: 0 12px; }", 1},
		{"margin short form with one bad part", stylePageFile, ".a { margin: var(--space-2) 3px 0 auto; }", 1},
		{"long form", stylePageFile, ".a { padding-left: 1em; }", 1},
		{"spacing variables", stylePageFile, ".a { margin: 0 auto; padding: var(--space-1) var(--space-6); gap: var(--space-3); }", 0},
		{"negative spacing", stylePageFile, ".a { margin-left: calc(-1 * var(--space-2)); }", 0},
		{"a size is not a gap", stylePageFile, ".a { width: 22rem; top: 2.5rem; border-radius: 6px; font-size: 0.85rem; }", 0},
		{"a comment is not read", stylePageFile, "/* color: #fff; */ .a { color: var(--accent); }", 0},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			findings, _ := styleFindings(test.sheet, test.css)
			assert.Len(t, findings, test.findings, "findings: %v", findings)
		})
	}
}
