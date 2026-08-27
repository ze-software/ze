// Design: docs/architecture/config/yang-config-design.md -- the coercion guard's answers
//
// report.go holds what the two actions of `le config-coercion` ANSWER, apart
// from what produced them.
//
// Each answer IS its rows, so each payload is a slice rather than a struct
// wrapping one: `| json` renders the array the script's --json rendered, and
// `| count` says how many. Each slice also renders ITSELF (Text), because a
// violation list with the remedy under it is what a person reads here
// (internal/le/leroot, Prose).

package configcoercion

import "github.com/ze-software/ze/internal/core/textbuf"

// The two shapes a coercion bug takes. They are the script's own spellings, and
// they are what `| match type-assert` selects on.
const (
	KindTypeSwitch = "type-switch"
	KindTypeAssert = "type-assert"
)

// Finding is one native-type coercion of a delivered config value, and it is
// one ROW of the check's answer. The keys are the script's, unchanged.
type Finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
	Code string `json:"code"`
}

// Findings is the whole answer of one check.
type Findings []Finding

// Text renders the findings for a person: the count, one line per site, and the
// remedy. A run that found nothing renders the verdict the script printed. It
// ends in a newline.
func (f Findings) Text() string {
	var tb textbuf.Buffer
	if len(f) == 0 {
		return tb.Str("config-string-coercion: OK\n").String()
	}

	tb.Str("config-string-coercion: ").Int(int64(len(f))).
		Str(" native-type config coercion(s) that ignore the delivered string form:\n")
	for _, finding := range f {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
			Str(" (").Str(finding.Kind).Str("): ").Str(finding.Code).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("The config framework delivers every YANG leaf value as a JSON STRING, so a\n")
	tb.Str("native-type assertion (v.(bool)/v.(float64)) or a numeric/bool type switch with\n")
	tb.Str("no `case string:` arm always fails -> the leaf silently reverts to its default\n")
	tb.Str("(a bool `enabled` gate disables the whole feature). Coerce via a helper that\n")
	tb.Str("accepts the string form (see internal/plugins/trafficusage/config.go cfgBool and\n")
	tb.Str("the `case string:` arms in toInt/toFloat). Allowlist only genuine non-config uses.\n")
	return tb.String()
}
