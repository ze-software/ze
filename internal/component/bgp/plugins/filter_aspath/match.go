// Design: docs/architecture/core-design.md -- AS-path regex filter matching
// Related: config.go -- AS-path list config parsing
// Related: filter_aspath.go -- SDK entry point and handleFilterUpdate
//
// The matching algorithm walks an ordered list of compiled regex entries.
// The UPDATE's AS-path attribute is extracted from the filter text format,
// normalized to a space-separated decimal string (brackets stripped), and
// each entry's regex is matched against it. First match wins; no match
// yields actionReject (implicit deny).
//
// Unlike prefix-list which operates per-prefix and can partition, AS-path
// filtering is attribute-level: the entire UPDATE shares one AS-path, so
// the result is always accept or reject (never modify).
package filter_aspath

import (
	"regexp"
	"strings"
)

// action is the per-entry decision applied when a regex matches.
type action int

const (
	actionAccept action = iota
	actionReject
)

func (a action) String() string {
	if a == actionAccept {
		return "accept"
	}
	return "reject"
}

// aspathEntry is one ordered regex entry inside an as-path-list.
// Order matters: first match wins.
type aspathEntry struct {
	regex  *regexp.Regexp
	action action
}

// aspathList is a named ordered list of regex entries.
type aspathList struct {
	name    string
	entries []aspathEntry
}

// evaluateASPath walks the entries in order and returns the action of the
// first entry whose regex matches the AS-path string. Returns actionReject
// if no entry matches (implicit deny).
func evaluateASPath(entries []aspathEntry, asPathStr string) action {
	for i := range entries {
		e := &entries[i]
		if e.regex.MatchString(asPathStr) {
			return e.action
		}
	}
	return actionReject
}

// extractASPathField extracts the AS-path string from the filter update text
// format. The text format emits "as-path 65001" (single ASN) or
// "as-path [65001 65002]" (multiple ASNs). Returns "" for empty/absent AS-path.
//
// The returned string is normalized: brackets stripped, space-separated decimal
// ASNs suitable for regex matching.
//
// Relies on the fixed attribute emission order in filter_format.go: as-path is
// always preceded by origin and followed by next-hop, so "as-path " cannot
// appear as a substring of another attribute's value.
func extractASPathField(updateText string) string {
	_, rest, ok := strings.Cut(updateText, "as-path ")
	if !ok {
		return ""
	}

	value := extractValueUntilNextAttr(rest)

	// Strip brackets: "[65001 65002]" -> "65001 65002"
	if len(value) >= 2 && value[0] == '[' && value[len(value)-1] == ']' {
		value = value[1 : len(value)-1]
	}
	return value
}

// extractValueUntilNextAttr returns the portion of s up to (but not including)
// the first space-separated token that is a known attribute keyword. If no
// keyword is found, returns the whole string (trimmed).
func extractValueUntilNextAttr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Bracketed value: find closing bracket
	if s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end >= 0 {
			return s[:end+1]
		}
		return s
	}
	// Non-bracketed: single token (the ASN number)
	before, _, found := strings.Cut(s, " ")
	if !found {
		return s
	}
	return before
}
