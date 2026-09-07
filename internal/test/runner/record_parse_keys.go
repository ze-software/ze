// Design: docs/architecture/testing/ci-format.md -- the .ci directive key vocabulary
// Overview: record_parse.go -- the directive parser that calls checkKeys
// Related: internal/test/ci/ciformat.go -- the key=value splitter that produces the map

package runner

import (
	"errors"
	"slices"

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
	slices.Sort(unknown)
	accepted := slices.Clone(known)
	slices.Sort(accepted)

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

// checkMarkerKeys refuses a marker-parsed directive line that carries a key its
// parser does not read. The known set is the ":key=" spellings the parser
// reads, and the refusal is checkKeys's, so an author meets one message shape
// on every directive.
//
// A marker parser needs its own check because it cannot drop an unknown key the
// way the key=value parsers do: each value runs from its own marker to the next
// KNOWN one, so an unknown key is swallowed into the value BEFORE it.
// `cmd=foreground:seq=2:exec=ze -:stdin=ze-bgp:timeout=15s:env=ZE_FWD_WRITE_DEADLINE=10s`
// parsed with timeout="15s:env=ZE_FWD_WRITE_DEADLINE=10s", which
// time.ParseDuration then refused in silence (startBackgroundLifetime). The
// line got neither the timeout it declared nor the environment variable, and
// nothing said so. It was the only such line among 3,197 cmd= lines when this
// check was written, and option=env:var=NAME:value=VALUE is how a .ci sets one.
//
// The scan reads the WHOLE line, the exec= value included, because that is
// where a swallowed key hides. So a cmd= line cannot carry a ":<word>=" span
// inside its command: put that command in a tmpfs= script and run the script.
func checkMarkerKeys(directive, line string, markers []string) error {
	found := make(map[string]string, len(markers))
	for i := range len(line) {
		if line[i] != ':' {
			continue
		}
		key, ok := markerKeyAt(line[i:])
		if !ok {
			continue
		}
		found[key] = ""
	}
	known := make([]string, 0, len(markers))
	for _, marker := range markers {
		known = append(known, marker[1:len(marker)-1])
	}
	return checkKeys(directive, found, known...)
}

// markerKeyAt reads the key of a ":key=" span at the start of s, and reports
// whether s opens with one. A key starts with a letter, so neither a port in a
// URL (":8080/path") nor a bare colon in a command is read as a key.
func markerKeyAt(s string) (string, bool) {
	if len(s) < 3 || !isMarkerKeyFirst(s[1]) {
		return "", false
	}
	for i := 2; i < len(s); i++ {
		if s[i] == '=' {
			return s[1:i], true
		}
		if isMarkerKeyByte(s[i]) {
			continue
		}
		return "", false
	}
	return "", false
}

// isMarkerKeyFirst reports whether c can open a directive key.
func isMarkerKeyFirst(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isMarkerKeyByte reports whether c can continue a directive key.
func isMarkerKeyByte(c byte) bool {
	return isMarkerKeyFirst(c) || (c >= '0' && c <= '9') || c == '-' || c == '_'
}
