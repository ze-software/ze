// Design: docs/architecture/core-design.md -- changed-file verification report.
// Overview: docwiring.go -- the run that fills this report.
//
// The payload is a document because a reader needs judged files, selected
// actions, results, and failure groups.

package docwiring

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CheckResult is what one sub-check found.
// Message is the verdict line, and Violations contains the offending lines.
// Output contains the linked owner's prose rendering. Skipped distinguishes an
// unnecessary check from a clean executed check. Empty JSON fields cannot show
// that distinction.
type CheckResult struct {
	Name       string   `json:"name"`
	Failed     bool     `json:"failed"`
	Skipped    bool     `json:"skipped"`
	Code       int      `json:"code"`
	Message    string   `json:"message"`
	Violations []string `json:"violations"`
	Output     string   `json:"output"`
}

// Report is the whole answer of one run.
type Report struct {
	Changed        []string      `json:"changed"`
	Actions        []string      `json:"actions"`
	Checks         []CheckResult `json:"checks"`
	Advisory       string        `json:"advisory"`
	DryRun         bool          `json:"dry-run"`
	Groups         []Group       `json:"failure-groups"`
	DeclaredGroups int           `json:"declared-groups"`
	Failed         bool          `json:"failed"`
	Error          string        `json:"error"`
}

// The names of checks implemented directly by this package.
const (
	checkSleepRatchetName       = "ci-sleep-ratchet"
	checkSleepJustificationName = "ci-sleep-justification"
	checkLoadExcuseName         = "known-failure-load-excuse"
	checkLogSubsystemName       = "ci-log-subsystem-key"
	checkDesignRefsName         = "design-refs"
	checkDocDriftName           = "doc-drift"
)

// advice is the fixed block printed after a failed check's violations. The map
// removes per-check branches from rendering. A message states what happened,
// while this text tells the reader what to do.
var advice = map[string][]string{
	checkSleepRatchetName: {
		"  Replace the new sleep with a compiled fixture.Poll, SDK callback,",
		"  or declarative runner wait. Raise the ceiling only with explicit user",
		"  approval (append a `+N` line).",
	},
	checkSleepJustificationName: {
		"  Add a `#` comment (poll interval, deliberate timer, needs-linux effect,",
		"  or no queryable readiness signal). See ai/rules/testing.md.",
	},
	checkLoadExcuseName: {
		"  Fix the test to wait on the condition through fixture.Poll, an SDK",
		"  event, or a condition-specific helper, then delete the shard. Raising",
		"  a timeout is not a fix. See ai/rules/completion.md.",
	},
	checkLogSubsystemName: {
		"  An internal plugin's logger name is CanonicalSubsystemName of its",
		"  registry name (plugin/inprocess.go): every hyphen becomes a dot.",
	},
	checkDocDriftName: {
		"  Update each page in THIS work, beside the code that made it wrong,",
		"  and keep its <!-- source: --> anchor valid. When the change leaves",
		"  the claim true, the page still owes the edit that says so.",
		"  See ai/rules/documentation.md.",
	},
}

// preamble is what a failing check prints between its verdict line and its
// violations. Only two checks have one, and each explains why the violations
// below it matter.
var preamble = map[string][]string{
	checkSleepJustificationName: {
		"  Every time.sleep( in a changed .ci test must carry a comment on the",
		"  line directly above it (or trailing it) explaining why it is there /",
		"  why it was not converted to a deterministic wait. Unjustified sleeps:",
	},
	checkLoadExcuseName: {
		"  A shard may not attribute a red to host load. That attribution IS",
		"  the diagnosis: the test asserts on elapsed time instead of on state.",
	},
	checkLogSubsystemName: {
		"  A hyphenated ze.log.<subsystem> key only works when that exact",
		"  subsystem is declared literally in Go. These match nothing, so the",
		"  level silently stays at the WARN default:",
	},
	checkDocDriftName: {
		"  A page carries a <!-- source: --> anchor over each claim it makes",
		"  about code. These claims name a symbol this diff changed, and their",
		"  page did not change with it:",
	},
}

// Text renders the whole run for a person, in the words the script prints. It
// ends in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer

	if r.Error != "" {
		return tb.Str("error: ").Str(r.Error).Byte('\n').String()
	}
	if r.DryRun {
		return r.dryRunText()
	}

	for _, check := range r.Checks {
		renderCheck(&tb, check)
	}

	if len(r.Actions) == 0 {
		tb.Str("No wiring/doc/inventory checks needed\n")
	}
	if r.Advisory != "" {
		tb.Str(r.Advisory).Byte('\n')
	}

	for _, group := range r.Groups {
		tb.Str(group.Line()).Byte('\n')
	}
	if r.DeclaredGroups > 0 || r.Failed {
		tb.Str(failureGroupsCompletePrefix).Byte(' ').Int(int64(r.DeclaredGroups)).Byte('\n')
	}
	if !r.Failed && len(r.Actions) > 0 {
		tb.Str("Wiring/doc/inventory gates passed\n")
	}
	return tb.String()
}

// renderCheck writes one sub-check's verdict, its violations and its advice.
//
// Preamble and advice appear only with VIOLATIONS. They explain those lines and
// their repair. Printing them for an unreadable tree would hide the actual
// failure and answer an unrelated question.
func renderCheck(tb *textbuf.Buffer, check CheckResult) {
	if check.Skipped {
		return
	}
	if check.Message != "" {
		tb.Str(check.Message).Byte('\n')
	}
	explain := check.Failed && len(check.Violations) > 0
	if explain {
		for _, line := range preamble[check.Name] {
			tb.Str(line).Byte('\n')
		}
	}
	for _, line := range check.Violations {
		tb.Str("    ").Str(line).Byte('\n')
	}
	if explain {
		for _, line := range advice[check.Name] {
			tb.Str(line).Byte('\n')
		}
	}
	if check.Output != "" {
		tb.Str(check.Output)
		if !strings.HasSuffix(check.Output, "\n") {
			tb.Byte('\n')
		}
	}
}

// dryRunText renders what a dry run answers: the selected gates, one per line,
// or the sentence that says there are none.
func (r Report) dryRunText() string {
	var tb textbuf.Buffer
	if len(r.Actions) == 0 {
		tb.Str("No wiring/doc/inventory checks needed\n")
	} else {
		tb.Join(r.Actions, "\n").Byte('\n')
	}
	if r.Advisory != "" {
		tb.Str(r.Advisory).Byte('\n')
	}
	return tb.String()
}
