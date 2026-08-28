// Design: docs/architecture/testing/verify-freshness-scope.md -- what a scoped run covers
// Overview: changed.go -- the selection these render
//
// report.go holds the two renderings of one selection.
//
// Group names and package patterns are the same selection in two default
// renderings. `| json` answers the same document for either verb.

package changed

import "github.com/ze-software/ze/internal/core/textbuf"

// groupNames renders a selection as the group names the native verifier uses.
type groupNames struct {
	Selection
}

// Text writes one group per line. It then writes `rest` when a changed package
// belongs to no group. The output ends in a newline. An empty selection renders
// an empty string rather than a blank line because a caller reading `$(...)`
// cannot distinguish a blank line from a name.
func (g groupNames) Text() string {
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

// groupPackages renders a selection as the Go package patterns a test run takes
// as arguments.
type groupPackages struct {
	Selection
}

// Text names one package pattern per line: the pattern of every hit group, then
// every unmapped package directory. It ends in a newline, and an empty
// selection renders the empty string.
func (g groupPackages) Text() string {
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
