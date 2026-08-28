// Design: docs/architecture/testing/tracked-build-gate.md -- the report's number columns
//
// format.go holds the two number renderings the flavor table needs and nothing
// else. They are here rather than inline because both are right-aligned in a
// fixed column, and a column that shifts by a character makes six rows unreadable.

package repositorytrackedbuild

import (
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// seconds renders a duration to one decimal place, which is the column the
// script printed with %5.1f.
func seconds(value float64) string {
	var tb textbuf.Buffer
	return tb.Float(value, 1).String()
}

// count renders a package count, which is the column the script printed with
// %4d.
func count(value int) string { return strconv.Itoa(value) }

// splitLines yields one entry per line of a tool's output, so a multi-line
// compiler complaint is indented line by line.
func splitLines(text string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for line := range strings.SplitSeq(text, "\n") {
			if !yield(line) {
				return
			}
		}
	}
}
