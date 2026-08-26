// Design: docs/architecture/core-design.md -- what a job answers once it has run
// Related: lejob.go -- the run this report describes
//
// A wrapped job's OUTPUT is the child's output, streamed to the terminal as it
// occurs. The payload contains facts that a reader cannot recover from that
// stream. It states whether this job ran, waited, or used another session's
// verdict. It also states which tree it used and what it decided.

package lejob

// Report contains the answer from one admitted job.
//
// The fields answer the questions that a session asks about an unobserved job.
// They state whether it ran or shared, the admission duration, the tree that
// was judged, and the result. Tree and Key are the two fingerprints that
// support a shared verdict. Thus, a caller that reads `le job run ... | json`
// can determine WHY two jobs shared or did not share.
type Report struct {
	Label     string   `json:"label"`
	Command   []string `json:"command"`
	Admission string   `json:"admission"`
	Tree      string   `json:"tree,omitempty"`
	Key       string   `json:"key,omitempty"`
	// WaitedSeconds is how long admission took, which is the number that says
	// whether the machine is oversubscribed.
	WaitedSeconds int `json:"waited-seconds"`
	Code          int `json:"code"`
}

// Text intentionally renders no text.
//
// The command's answer already reached the terminal. The child's output was
// streamed as it occurred, and the banners went to stderr. A final summary
// line would add output that the shell half never wrote to every wrapped
// recipe in the repository. The payload remains structured, so
// `le job run ... | json` still returns the report. Therefore, this is a Prose
// rendering with no text instead of a nil payload (letools/leroot, Prose).
func (r Report) Text() string { return "" }
