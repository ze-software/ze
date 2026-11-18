// Design: docs/architecture/web-interface.md -- the AS path search, in the engine's grammar
// Related: handler_ui.go -- handleUISearch sends the pattern after `path`
// Related: prefix_lookup.go -- the same parse-then-send rule for the prefix field

package lg

import (
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// asPathPatternLengthMax bounds the AS path field before it is read. A path
// of 200 characters holds about 30 AS numbers, longer than any run a search
// asks for.
const asPathPatternLengthMax = 200

// asPathPatternForm is what the AS path field takes, said once for every
// refusal. The grammar belongs to bgp-rib (matchASPath and
// validatePathPattern, internal/component/bgp/plugins/rib/rib_pipeline.go):
// AS numbers joined by commas, matched as a contiguous run anywhere in the
// path, and anchored at its start by a leading ^. The page reads spaces where
// the engine reads commas, because routers print AS paths with spaces.
const asPathPatternForm = "the field takes AS numbers from 0 to 4294967295, such as 65001 or 1.10, " +
	"separated by spaces and matched as a run anywhere in the path; " +
	"a leading ^ anchors the run at the start of the path"

// asPathPatternExample is the valid form every refusal ends on.
const asPathPatternExample = "65001 65002"

// asPathPattern is the AS path question one search field asks bgp-rib, in
// the plugin's own form: AS numbers in asplain, joined by commas, with an
// optional leading ^.
//
// Only parseASPathPattern builds one, so engine holds digits, commas and at
// most one leading ^, written by this package from numbers asn.Parse read. No
// text the user typed reaches the command line.
type asPathPattern struct {
	engine string
}

// parseASPathPattern reads what a user typed in the AS path field and writes
// it in the engine's grammar.
//
// Spaces around the value are dropped. The AS numbers may be separated by
// spaces, commas, or both; each one is
// read by asn.Parse, the reader the engine uses, so the three RFC 5396
// spellings are accepted and written back as asplain. One leading ^ anchors
// the run at the start of the path. Every other character (`$`, `|`,
// brackets, `*`, `+`, `?`, `_`, `-`) is a pattern the engine cannot match,
// and is refused with an operator error that names the value and the form
// the field takes.
//
// A field with no AS number is refused too. The engine reads an empty
// pattern as no filter at all, so `^` alone would list every route.
func parseASPathPattern(text string) (asPathPattern, error) {
	if len(text) > asPathPatternLengthMax {
		var problem textbuf.Buffer
		problem.Str("AS path of ").Int(int64(len(text))).Str(" characters is longer than the ").
			Int(asPathPatternLengthMax).Str(" the field takes")
		return asPathPattern{}, opererr.New(problem.String(), asPathPatternForm, asPathPatternExample)
	}

	rest := strings.Trim(text, " ")
	var tb textbuf.Buffer
	if strings.HasPrefix(rest, "^") {
		tb.Byte('^')
		rest = rest[1:]
	}

	count := 0
	for _, token := range strings.FieldsFunc(rest, isASPathSeparator) {
		number, err := asn.Parse(token)
		if err != nil {
			return asPathPattern{}, opererr.New(
				"AS path "+text+" holds "+token+", which is not an AS number",
				asPathPatternForm,
				asPathPatternExample)
		}
		if count > 0 {
			tb.Byte(',')
		}
		tb.Uint32(number)
		count++
	}
	if count == 0 {
		return asPathPattern{}, opererr.New(
			"AS path "+text+" holds no AS number",
			asPathPatternForm,
			asPathPatternExample)
	}
	return asPathPattern{engine: tb.String()}, nil
}

// isASPathSeparator reports whether r separates two AS numbers in the field:
// a space, as routers print a path, or a comma, as the engine reads one. A tab
// or a line break is not one: it is no part of a path anyone pastes, so it is
// refused with the AS number it is attached to.
func isASPathSeparator(r rune) bool {
	if r == ',' {
		return true
	}
	return r == ' '
}
