// Design: docs/architecture/core-design.md -- AS-path regex filter matching
// Related: config.go -- AS-path list config parsing
// Related: condition.go -- the as-path condition of a match instance
//
// The matching algorithm walks an ordered list of compiled regex entries.
// The BGP view answers the UPDATE's AS path as a space-separated decimal text,
// and each entry's regex is matched against it. First match wins; no match
// yields actionReject (implicit deny).
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

// evaluateASPathText walks the entries in order and returns the action of the
// first entry whose regex matches the AS-path text, in the bytes the BGP view
// answers (condition.go), so no string is built for the regex. Returns
// actionReject if no entry matches (implicit deny).
func evaluateASPathText(entries []aspathEntry, text []byte) action {
	for i := range entries {
		e := &entries[i]
		if e.regex.Match(text) {
			return e.action
		}
	}
	return actionReject
}
