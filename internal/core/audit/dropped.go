// Design: docs/architecture/core-design.md -- audit component
// Related: audit.go -- the config-discard action this detail is written for

package audit

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DroppedDiff answers the part of a pending diff that a discard dropped: the
// lines of before, read just ahead of the discard, that after, read once it
// succeeded, no longer holds. A config-discard row records this, never before
// alone, because a discard at a path keeps every change outside that path, and
// a row that listed those would tell the operator a change was dropped that
// the next commit then applies.
//
// A whole discard is always exact: it leaves no pending change, so after is
// empty and the answer is before. A path discard is exact only when every
// diff line names the path of the change it renders, as the web's
// EditorManager.Diff does: each kept change then renders the same lines it
// rendered before, so only the dropped change's lines go missing. Lines are
// counted, not set: a line that before holds twice and after once was dropped
// once.
//
// The SSH editor's diff (cli Editor.Diff, computeDiff) carries no path and
// compares lines as sets, so two places holding the same text collapse into
// one line, and the SSH path-discard row can name a kept change as dropped.
// The journal class key-omits-a-fact-the-builder-uses records it.
//
// Both diffs are line-oriented text; a line's trailing newline is not part of
// its identity, so a last line with no newline still matches.
func DroppedDiff(before, after string) string {
	if after == "" {
		return before
	}

	kept := make(map[string]int)
	for line := range strings.Lines(after) {
		kept[strings.TrimSuffix(line, "\n")]++
	}

	var b textbuf.Buffer
	for line := range strings.Lines(before) {
		text := strings.TrimSuffix(line, "\n")
		if kept[text] > 0 {
			kept[text]--
			continue
		}
		b.Str(text).Byte('\n')
	}
	return b.String()
}
