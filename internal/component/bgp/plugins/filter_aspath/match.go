// Design: docs/architecture/core-design.md -- AS-path regex filter matching
// Related: config.go -- AS-path list config parsing
// Related: filter_aspath.go -- SDK entry point and handleFilterUpdate
// Related: internal/component/bgp/filtertext -- the reader of the filter text format
//
// The matching algorithm walks an ordered list of compiled regex entries.
// filtertext.ASPath reads the UPDATE's AS-path attribute out of the filter text
// format as a space-separated decimal string, and each entry's regex is matched
// against it. First match wins; no match yields actionReject (implicit deny).
//
// Unlike prefix-list which operates per-prefix and can partition, AS-path
// filtering is attribute-level: the entire UPDATE shares one AS-path, so
// the result is always accept or reject (never modify).
package filter_aspath

import "regexp"

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
