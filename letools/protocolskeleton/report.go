// Design: ai/rules/protocol.md -- what the protocol-skeleton lens answers
// Overview: protocolskeleton.go -- the walk that fills this in
//
// report.go holds what `le protocol-skeleton report` ANSWERS, apart from what
// produced it.
//
// The page is a SUMMARY and the payload is the detail. The script printed one
// line by default and a per-protocol table under --verbose; a command has one
// answer and several renderings, so the detail is always in the payload and
// `| json` is what --verbose was.

package protocolskeleton

import "github.com/ze-software/ze/internal/core/textbuf"

// Module is one subpackage of a protocol and the class it falls in.
type Module struct {
	Name  string `json:"name"`
	Class string `json:"class"`
}

// Protocol is one manifest row: the name, the root it names, and what that root
// holds. Missing says the root is gone, which is a fact about the MANIFEST
// rather than about the tree, and it is reported in every mode because the
// manifest is hand-maintained.
type Protocol struct {
	Name          string   `json:"name"`
	Root          string   `json:"root"`
	Missing       bool     `json:"missing"`
	SinglePackage bool     `json:"single-package"`
	Modules       []Module `json:"modules"`
}

// Counts is how many modules fell in each class.
type Counts struct {
	Canonical       int `json:"canonical"`
	RFCState        int `json:"rfc-state"`
	Version         int `json:"version"`
	Domain          int `json:"domain"`
	LegacyException int `json:"legacy-exception"`
}

// Report is the whole answer of one run.
type Report struct {
	Protocols []Protocol `json:"protocols"`
	Counts    Counts     `json:"counts"`
}

// Missing answers the manifest rows whose root is gone, in manifest order.
func (r Report) Missing() []string {
	var gone []string
	for _, protocol := range r.Protocols {
		if protocol.Missing {
			gone = append(gone, protocol.Name)
		}
	}
	return gone
}

// Text renders the summary line for a person, in the words the script printed.
// It ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("protocol-skeleton advisory: ").Int(int64(len(r.Protocols))).Str(" protocols; ").
		Str("canonical ").Int(int64(r.Counts.Canonical)).
		Str(", rfc-state ").Int(int64(r.Counts.RFCState)).
		Str(", version ").Int(int64(r.Counts.Version)).
		Str(", domain ").Int(int64(r.Counts.Domain)).
		Str(", legacy ").Int(int64(r.Counts.LegacyException))

	if gone := r.Missing(); len(gone) > 0 {
		tb.Str("; MISSING roots: ").Join(gone, ", ")
	}
	// The detail is a pipe operator away rather than a flag, which is the one
	// place this page differs from the script's.
	return tb.Str(" (ai/rules/protocol.md; | json for detail)\n").String()
}
