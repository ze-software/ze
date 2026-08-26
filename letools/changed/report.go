// Design: docs/architecture/testing/verify-freshness-scope.md -- what a scoped run covers
// Overview: changed.go -- the selection these render
//
// report.go holds the two renderings of one selection.
//
// The shell half had them as a flag: `changed-groups.sh` printed group names and
// `changed-groups.sh --pkgs` printed package patterns. They are the same data,
// so they are one payload here and two default renderings, and `| json` answers
// the same document for either verb.

package changed

import "github.com/ze-software/ze/internal/core/textbuf"

// GroupNames renders a selection as the group names a Make target is spelled
// with, which is what `changed-groups.sh` printed with no flag.
type GroupNames struct {
	Selection
}

// Text writes one group per line. It then writes `rest` when a changed package
// belongs to no group. The output ends in a newline. An empty selection renders
// an empty string rather than a blank line because a caller reading `$(...)`
// cannot distinguish a blank line from a name.
func (g GroupNames) Text() string {
	if g.Empty() {
		return ""
	}
	var tb textbuf.Buffer
	for _, group := range g.Groups {
		tb.Str(group.Name).Byte('\n')
	}
	if len(g.Rest) > 0 {
		tb.Str(restGroup).Byte('\n')
	}
	return tb.String()
}

// GroupPackages renders a selection as the Go package patterns a test run takes
// as arguments, which is what `changed-groups.sh --pkgs` printed.
type GroupPackages struct {
	Selection
}

// Text names one package pattern per line: the pattern of every hit group, then
// every unmapped package directory. It ends in a newline, and an empty
// selection renders the empty string.
func (g GroupPackages) Text() string {
	if g.Empty() {
		return ""
	}
	var tb textbuf.Buffer
	for _, group := range g.Groups {
		tb.Str(group.Pattern).Byte('\n')
	}
	for _, pkg := range g.Rest {
		tb.Str(pkg).Byte('\n')
	}
	return tb.String()
}
