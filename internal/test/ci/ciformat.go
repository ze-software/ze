// Design: docs/architecture/testing/ci-format.md — CI test format parsing
//
// Package ciformat provides shared utilities for parsing .ci test files.
package ci

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ParseKVPairs parses key=value pairs from colon-separated parts.
// Special handling for known keys that may contain colons in values (json, text, hex).
func ParseKVPairs(parts []string) map[string]string {
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
		if before, after, ok := strings.Cut(part, "="); ok {
			key := before
			value := after
			kv[key] = value
		}
	}
	return kv
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
