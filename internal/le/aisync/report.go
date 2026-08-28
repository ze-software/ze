// Design: ai/rules/repo-maintenance.md -- one canonical source, several tool mirrors
// Overview: aisync.go -- the run this report describes
//
// report.go holds what a sync, a check and a preview ANSWER, apart from what
// produced them. One payload serves all three, because they are one question
// asked in three moods: which sources exist, and what is the tree's state
// against them.

package aisync

import "github.com/ze-software/ze/internal/core/textbuf"

// A run has one of three modes. The payload contains the mode because the same
// fields have different meanings in each mode. An empty Stale means a clean
// tree after a check. It says nothing after a sync.
const (
	modeSync    = "sync"
	modeCheck   = "check"
	modePreview = "preview"
)

// Report is what one run of this area answers.
type Report struct {
	Mode   string   `json:"mode"`
	Skills []string `json:"skills"`
	Agents []string `json:"agents"`
	// Stale names every generated path the checkout and a fresh generation
	// disagree about, repository-relative and sorted. A check fills it, and
	// nothing else does.
	Stale []string `json:"stale,omitempty"`
}

// Fresh reports whether the checkout's mirrors match their sources. It is the
// check's verdict, so the command's exit code reads it rather than re-deriving
// it from the page.
func (r Report) Fresh() bool { return len(r.Stale) == 0 }

// Text renders the report for a person, in the words the script printed before
// it was a command. It ends in a newline.
func (r Report) Text() string {
	var tb textbuf.Buffer
	switch r.Mode {
	case modePreview:
		for _, name := range r.Skills {
			tb.Str("would sync: ").Str(name).Byte('\n')
		}
		return tb.String()
	case modeCheck:
		if r.Fresh() {
			return tb.Str("generated agent files in sync\n").String()
		}
		for _, path := range r.Stale {
			tb.Str("stale: ").Str(path).Byte('\n')
		}
		tb.Str("generated agent files are stale -- run: ./le ai skills-sync\n")
		return tb.String()
	default:
		return tb.Str("synced ").Int(int64(len(r.Skills))).Str(" skill(s) + ").
			Int(int64(len(r.Agents))).Str(" agent(s) + CLAUDE.md + AGENTS.md\n").String()
	}
}
