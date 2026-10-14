// Design: docs/architecture/testing/ci-format.md — CI test format parsing
//
// Package ciformat provides shared utilities for parsing .ci test files.
package ci

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The three shapes ParseKVPairs refuses. Each error it returns wraps one, so a
// caller can tell the cause apart without matching text.
var (
	ErrSegmentWithoutKey = errors.New("a segment has no key")
	ErrKeyGivenTwice     = errors.New("a key is given twice")
	ErrKeyGluedToValue   = errors.New("key glued to a value")
)

// ParseKVPairs parses key=value pairs from colon-separated parts.
// Special handling for known keys that may contain colons in values (json, text, hex).
//
// It refuses three shapes it used to settle in silence, because a key the
// splitter never surfaces cannot be checked by the directive parser above it
// (docs/architecture/testing/ci-format.md, "An Unknown Directive or Key Is a
// Parse Error"): a segment with no `=` (`option=asn:65000`), a key given twice
// (the last one used to win), and a key that cannot open a key token, which the
// boundary split glues into the value before it (`path=x:!contains=y`). The
// error names the offending text; the caller adds the directive and the line.
func ParseKVPairs(parts []string) (map[string]string, error) {
	kv := make(map[string]string)

	// Rejoin parts to handle values containing colons
	joined := textbuf.Join(parts, ":")

	// Known keys that may have complex values containing colons
	complexKeys := []string{"json=", "text=", "hex=", "pattern="}

	for _, ck := range complexKeys {
		idx := strings.Index(joined, ck)
		if idx == -1 {
			continue
		}
		key := ck[:len(ck)-1] // Remove trailing =
		value := joined[idx+len(ck):]
		kv[key] = value
		// Remove this from joined for further parsing
		joined = joined[:idx]
		break
	}

	// Parse remaining simple key=value pairs.
	//
	// Split on a colon ONLY where a new `key=` token begins. A naive
	// SplitSeq(joined, ":") cuts every colon, which silently truncates any
	// value that legitimately contains one. `expect=stdout:contains=error: no
	// such peer` became `contains=error`, an assertion that passes on almost
	// any output. A tree-wide sweep on 2026-07-29 found 203 `.ci` assertions
	// weakened exactly that way across 15 suites. Those tests cannot fail.
	//
	// splitOnKeyBoundary keeps the 33 legitimate uses working: a colon followed
	// by `timeout=` IS a new key and still splits, so the engine-step
	// `contains=aes-cbc:timeout=25` form is unaffected.
	// A complex key extracted above leaves the separator that preceded it, so
	// `conn=1:seq=1:text=...` becomes `conn=1:seq=1:`. The old split-on-every-colon
	// dropped that as an empty part. A boundary-aware split would instead carry it
	// into the final value as `seq=1:`. Trim it before the split.
	joined = strings.TrimSuffix(joined, ":")

	for _, part := range splitOnKeyBoundary(joined) {
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			var b textbuf.Buffer
			b.Quoted(part).Str(" has no \"=\"; write each field as key=value")
			return nil, fmt.Errorf("%w: %s", ErrSegmentWithoutKey, b.String())
		}
		if _, held := kv[key]; held {
			var b textbuf.Buffer
			b.Quoted(key).Str("; keep one")
			return nil, fmt.Errorf("%w: %s", ErrKeyGivenTwice, b.String())
		}
		if glued, found := gluedKey(value); found {
			var b textbuf.Buffer
			b.Quoted(glued).Str(" was read as part of the value of ").Str(key).
				Str("=, because a key starts with a letter; rename the key or move the text into pattern=")
			return nil, fmt.Errorf("%w: %s", ErrKeyGluedToValue, b.String())
		}
		kv[key] = value
	}
	return kv, nil
}

// gluedKey reports a `:key=` span inside value whose key starts with a byte
// that cannot open a key token. splitOnKeyBoundary splits only before a letter,
// so such a key stays inside the previous value, where no parser can refuse it.
// The span is a colon, then key bytes or `!` with a non-letter first, then `=`:
// a colon in prose, a port (`:8080/`) or a JSON pair (`"a":1`) is not one.
func gluedKey(value string) (string, bool) {
	for i := range len(value) {
		if value[i] != ':' {
			continue
		}
		rest := value[i+1:]
		if rest == "" || isKeyLeadByte(rest[0]) {
			continue
		}
		end := 0
		for end < len(rest) && (isKeyByte(rest[end]) || rest[end] == '!') {
			end++
		}
		if end > 0 && end < len(rest) && rest[end] == '=' {
			return rest[:end], true
		}
	}
	return "", false
}

// splitOnKeyBoundary splits s on each colon that introduces a new `key=` token,
// leaving colons inside a value intact.
//
// A boundary is a colon followed by an identifier and then `=`. The identifier
// is the shape a .ci directive key actually takes: a letter, then letters,
// digits, `-` or `_`. `:contains=` and `:timeout=` are boundaries. `: no such
// peer` and `:8080/path` are not.
//
// The remaining ambiguity is a value containing a colon followed by something
// that looks like a key, for example `contains=note:level=high`. That splits,
// because nothing in the format distinguishes it from a real key. Authors who
// need such a value use `pattern=`, which is consumed whole before this runs.
func splitOnKeyBoundary(s string) []string {
	var parts []string
	start := 0
	for i := range len(s) {
		if s[i] != ':' || !startsKeyToken(s[i+1:]) {
			continue
		}
		parts = append(parts, s[start:i])
		start = i + 1
	}
	return append(parts, s[start:])
}

// startsKeyToken reports whether s begins with a directive key followed by `=`.
func startsKeyToken(s string) bool {
	if s == "" || !isKeyLeadByte(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ { //nolint:intrange // starts at 1, not 0
		switch {
		case s[i] == '=':
			return true
		case isKeyByte(s[i]):
			continue
		default:
			return false
		}
	}
	return false
}

func isKeyLeadByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isKeyByte(c byte) bool {
	return isKeyLeadByte(c) || (c >= '0' && c <= '9') || c == '-' || c == '_'
}
