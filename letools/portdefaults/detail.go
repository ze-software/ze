// Design: docs/architecture/cli/color-system.md -- selftest failure detail lines
// Related: selftest.go -- the failure details these build
//
// A selftest detail is a sentence a person reads when a case fails, and it is
// built with textbuf rather than a format string because that is how a compiled
// Ze package writes its text.

package portdefaults

import "github.com/ze-software/ze/internal/core/textbuf"

// detail answers a lead-in followed by one number.
func detail(lead string, value int) string {
	var tb textbuf.Buffer
	return tb.Str(lead).Int(int64(value)).String()
}

// describe answers a lead-in followed by the drift list, one clause per drift,
// so a failing case says what the comparison actually produced.
func describe(lead string, drifts []Drift) string {
	var tb textbuf.Buffer
	tb.Str(lead).Byte('[')
	for i, drift := range drifts {
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Byte('{').Str(drift.Service).Byte(' ').Int(int64(drift.GoPort)).Byte(' ').
			Int(int64(drift.YANGPort)).Byte(' ').Str(drift.File).Byte(' ').Str(drift.Reason).Byte('}')
	}
	tb.Byte(']')
	return tb.String()
}

// describeStrings answers a lead-in followed by a list of words.
func describeStrings(lead string, words []string) string {
	var tb textbuf.Buffer
	return tb.Str(lead).Byte('[').Join(words, " ").Byte(']').String()
}
