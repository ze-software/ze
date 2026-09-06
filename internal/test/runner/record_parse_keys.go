// Design: docs/architecture/testing/ci-format.md -- the .ci directive key vocabulary
// Overview: record_parse.go -- the directive parser that calls checkKeys
// Related: internal/test/ci/ciformat.go -- the key=value splitter that produces the map

package runner

import (
	"errors"
	"slices"
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// checkKeys refuses a directive that carries a key the parser does not read.
//
// An unrecognized key used to be dropped in silence, so its whole assertion
// vanished and the test passed whatever the command printed.
// `expect=stdout:not-contains=doctor-platform-detect` asserted nothing at all,
// and nothing said so: `not-contains=` is the right spelling for
// `expect=file:`, so an author who learned it there wrote it on a stream and
// got a green bar. Ten such lines were live across seven files when this check
// was added. A parser that meets a key it cannot answer says so
// (ai/rules/principles.md).
//
// known MUST list every key the calling arm reads. The caller passes the
// directive text (`expect=stdout`) so the message names the line the author
// wrote rather than the switch arm that refused it.
func checkKeys(directive string, kv map[string]string, known ...string) error {
	var unknown []string
	for key := range kv {
		if slices.Contains(known, key) {
			continue
		}
		unknown = append(unknown, key)
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	accepted := slices.Clone(known)
	sort.Strings(accepted)

	var msg textbuf.Buffer
	msg.Str(directive).Str(": unknown key")
	if len(unknown) > 1 {
		msg.Byte('s')
	}
	msg.Byte(' ')
	for i, key := range unknown {
		if i > 0 {
			msg.Str(", ")
		}
		msg.Quoted(key)
	}
	msg.Str(" (accepts ").Join(accepted, ", ").Byte(')')
	for _, key := range unknown {
		hint, ok := retiredKeys[key]
		if !ok {
			continue
		}
		msg.Str("; ").Str(key).Str("= is ").Str(hint)
	}
	return errors.New(msg.String())
}

// retiredKeys maps a key an author is likely to write on a stream directive to
// the directive that carries that assertion instead. Both entries spell stream
// non-containment. `not-contains=` is the `expect=file:` spelling and was never
// read on a stream. `!contains=` WAS read on `expect=stdout` until the stream
// vocabulary was consolidated on `reject=`, which already carried the identical
// assertion as `reject=stdout:contains=` and is now the only spelling.
var retiredKeys = map[string]string{
	"not-contains": "reject=<stream>:contains=<text>",
	"!contains":    "reject=<stream>:contains=<text>",
}
