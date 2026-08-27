// Design: docs/contributing/documentation-testing.md -- what the digest check answers
//
// Overview: digest.go -- the anchor scan behind these answers
//
// report.go holds what `le digest` ANSWERS, apart from what produced it.
//
// The script keeps its two output streams separate. A clean verdict goes to
// stdout (Text). The failure page goes to stderr (Diagnosis), where a person
// expects a gate to print it. This split lets the migration prove both streams
// byte for byte. A runner cannot order one merged capture.
package digest

import "github.com/ze-software/ze/internal/core/textbuf"

// Problem is one anchor that did not resolve, and it is one row of the failure
// page. The keys are the script's --json keys, unchanged.
type Problem struct {
	// Digest is the tree-relative digest the anchor was written in.
	Digest string `json:"digest"`
	// Anchor is the anchor as the digest wrote it.
	Anchor string `json:"anchor"`
	// Detail says what is wrong with it. The key stays `problem`, which is
	// what the script's --json answered.
	Detail string `json:"problem"`
}

// Resolution is one anchor that did resolve, and the file it reached. It is
// what the script's --list printed, carried in the payload instead: a
// rendering is a pipe operator, so `le digest | json` is the listing.
type Resolution struct {
	Digest string `json:"digest"`
	Anchor string `json:"anchor"`
	File   string `json:"file"`
}

// Report is the whole answer of one run.
type Report struct {
	// Digests is how many digests were read.
	Digests int `json:"digests"`
	// Anchors is how many anchors were judged, resolved and failed together.
	Anchors int `json:"anchors"`
	// Errors are the anchors that did not resolve.
	Errors []Problem `json:"errors"`
	// Resolved are the anchors that did.
	Resolved []Resolution `json:"resolved"`
}

// Text renders the stdout half: the verdict a clean run prints. A run holding
// problems prints its page on stderr, exactly as the script does, so there is
// nothing for stdout to carry.
func (r Report) Text() string {
	if len(r.Errors) > 0 {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str("checked ").Int(int64(len(r.Resolved))).Str(" anchors across ").
		Int(int64(r.Digests)).Str(" digests, all resolve\n").String()
}

// Diagnosis renders the stderr half: the failure heading, one line per bad
// anchor, and the remedy. It is empty for a run that found nothing wrong.
func (r Report) Diagnosis() string {
	if len(r.Errors) == 0 {
		return ""
	}

	digests := make(map[string]bool, len(r.Errors))
	for _, problem := range r.Errors {
		digests[problem.Digest] = true
	}

	var tb textbuf.Buffer
	tb.Str("digest anchor check FAILED: ").Int(int64(len(r.Errors))).
		Str(" bad anchor(s) in ").Int(int64(len(digests))).Str(" digest(s)\n")
	for _, problem := range r.Errors {
		tb.Str("  ").Str(problem.Digest).Str(": `").Str(problem.Anchor).
			Str("` -- ").Str(problem.Detail).Byte('\n')
	}
	tb.Str("Fix the anchor or run the file to find the moved line, then update the digest (these are hand-maintained: ai/digests/README.md).\n")
	return tb.String()
}
