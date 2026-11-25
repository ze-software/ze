// Design: docs/architecture/core-design.md -- what a job answers once it has run
// Related: job.go -- the run this report describes
//
// A wrapped job's OUTPUT is the child's output, streamed to the terminal as it
// occurs. The payload contains facts that a reader cannot recover from that
// stream. It states whether this job ran, waited, or used another session's
// verdict. It also states which tree it used and what it decided.

package job

import "github.com/ze-software/ze/internal/core/textbuf"

// Report contains the answer from one admitted job.
//
// The fields answer the questions that a session asks about an unobserved job.
// They state whether it ran or shared, the admission duration, the inputs that
// were judged, and the result. Tree and Key are the two fingerprints that
// support a shared verdict: Tree covers the inputs this label reads, and Key
// covers the work itself. Thus, a caller that reads `le job run ... | json`
// can determine WHY two jobs shared or did not share.
type Report struct {
	Label     string   `json:"label"`
	Command   []string `json:"command"`
	Admission string   `json:"admission"`
	Tree      string   `json:"tree,omitempty"`
	Key       string   `json:"key,omitempty"`
	// Log is the checkout-relative path of the log a quiet run wrote, which is
	// the whole of the child's output. It survives the run, which the registry
	// log does not (ticket.Release removes that one).
	Log string `json:"log,omitempty"`
	// KeyLines are the failure lines of that log, each with its line number
	// (internal/le/runlog). An empty list on a non-zero code says the child
	// failed without writing a line of that shape.
	KeyLines []string `json:"key-lines,omitempty"`
	// Quiet says the child's output went to Log instead of this job's stdout,
	// which is what makes a summary the right thing to render.
	Quiet bool `json:"quiet,omitempty"`
	// WaitedSeconds is how long admission took, which is the number that says
	// whether the machine is oversubscribed.
	WaitedSeconds int `json:"waited-seconds"`
	// RanSeconds is how long the run took after admission, the number that
	// tells a slow run from a run that queued. It is zero exactly when this job
	// ran nothing: an attached job took another run's verdict, and a refused or
	// canceled admission started nothing. A run always measures above zero, so
	// that zero is the guard WriteTiming relies on to leave the clause out
	// rather than print a run that did not happen. A run whose child could not
	// start (exit 127) still measures its attempt, and its code says the rest.
	RanSeconds float64 `json:"ran-seconds,omitempty"`
	Code       int     `json:"code"`
}

// Text renders the summary of a quiet run, and nothing for any other run.
//
// An ordinary run's answer already reached the terminal. The child's output was
// streamed as it occurred, and the banners went to stderr. A final summary
// line would add output that the shell half never wrote to every wrapped
// recipe in the repository. The payload remains structured, so
// `le job run ... | json` still returns the report. Therefore, this is a Prose
// rendering with no text instead of a nil payload (internal/le/le/root, Prose).
//
// A quiet run wrote that output to a file instead, so its reader has seen
// nothing yet. The summary is then the whole of what reaches the terminal:
// the verdict, how long admission waited and how long the child ran
// (WriteTiming), where the log is, and the lines that say what broke.
func (r Report) Text() string {
	if !r.Quiet {
		return ""
	}

	var text textbuf.Buffer
	text.Reset().Str("job ").Str(r.Label).Str(": exit ").Int(int64(r.Code))
	r.WriteTiming(&text)
	text.Str(", log ").Str(r.Log).Byte('\n')
	for _, line := range r.KeyLines {
		text.Str(line).Byte('\n')
	}
	return text.String()
}

// WriteTiming writes the clauses that say where the job's time went, each led
// by ", ": how long admission waited, when it waited a whole second or more,
// then how long the child ran after admission, to a tenth of a second, when a
// child ran here. The two together tell a run that queued from a run that was
// slow. Every quiet answer carries them, `le test unit pkg`'s included, so the
// clauses are written here once.
//
// The seconds are triage on the host in hand, never a performance figure.
func (r Report) WriteTiming(text *textbuf.Buffer) {
	if r.WaitedSeconds > 0 {
		text.Str(", waited ").Int(int64(r.WaitedSeconds)).Byte('s')
	}
	// A zero RanSeconds says no child ran here (see the field), so no run is
	// printed rather than a run of zero seconds.
	if r.RanSeconds > 0 {
		text.Str(", ran ").Float(r.RanSeconds, 1).Byte('s')
	}
}
