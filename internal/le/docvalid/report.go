// Design: docs/architecture/core-design.md -- the documentation gates' answers
// Detail: drift.go -- what produces the drift answer
// Detail: contract.go -- what produces the contract answer
//
// report.go holds what `le docvalid` ANSWERS, apart from what produced it.
// Each action has one payload, and each payload renders itself for a person
// (internal/le/leroot, Prose) while staying structured data for `| json`, `| yaml`
// and `| table`.
//
// The renderings reproduce what the scripts printed, line for line, because a
// port that changes the words changes every reader's habit and every grep in a
// rule file. The one thing they do not reproduce is the raw escape sequence: a
// compiled Ze package writes the semantic palette instead
// (docs/architecture/cli/color-system.md).

package docvalid

import "github.com/ze-software/ze/internal/core/textbuf"

// Issue is one documentation drift finding, and one ROW of the drift answer.
type Issue struct {
	// File is the path the claim lives in, relative to the tree.
	File string `json:"file"`
	// Line is the 1-based line, and it is ABSENT from the JSON when the
	// finding is about the file as a whole.
	Line int `json:"line,omitempty"`
	// Message says what the document claims and what the tree holds.
	Message string `json:"message"`
	// Detail carries what to write instead, when there is something to say.
	Detail string `json:"detail,omitempty"`
}

// DriftReport is the whole answer of one `ze-doc-drift-check` run.
//
// Issues is the only row set in it, so the row operators act on the findings
// (internal/component/command/answer_shape.go, rowsIn).
type DriftReport struct {
	Issues []Issue `json:"issues"`
}

// Text renders the drift report the way the script printed it: the clean line
// on its own, or a heading, one line per finding, and the command to run. It
// ends in a newline.
//
// The script wrote the finding list to stderr and the clean line to stdout. A
// command answers ONE payload and the caller chooses where it goes, so both
// land on stdout here. The verdict travels in the exit code either way.
func (r DriftReport) Text() string {
	if len(r.Issues) == 0 {
		return "No documentation drift detected.\n"
	}

	var tb textbuf.Buffer
	tb.SetColor(true)
	color := textbuf.C

	tb.Byte('\n').Colored(color.BrightYellow).Str("  Documentation drift detected (").
		Int(int64(len(r.Issues))).Str(" issues)").Colored(color.Reset).Str("\n\n")

	for _, iss := range r.Issues {
		tb.Str("  ").Colored(color.BoldRed).Byte('x').Colored(color.Reset).Byte(' ').
			Str(iss.File).Byte(':').Int(int64(iss.Line)).Str(": ").Str(iss.Message).Byte('\n')
		if iss.Detail != "" {
			tb.Str("    ").Colored(color.BrightYellow).Str("->").Colored(color.Reset).
				Byte(' ').Str(iss.Detail).Byte('\n')
		}
	}

	tb.Str("\n  Run: make ze-doc-drift-check\n\n")
	return tb.String()
}

// CommandEntry is one ze:command node found in the YANG tree.
type CommandEntry struct {
	WireMethod string `json:"wire-method"`
	YANGPath   string `json:"yang-path"`
	Module     string `json:"module"`
}

// ValidationResult holds the cross-check between the YANG command tree and the
// registered handlers.
//
// The JSON keys are the ones `make ze-command-contract-check-json` published
// before this was a command, so anything reading that output keeps reading it.
type ValidationResult struct {
	YANGCommands        []CommandEntry `json:"yang-commands"`
	Handlers            []string       `json:"handlers"`
	LocalHandlers       []string       `json:"local-handlers"`
	OrphanYANG          []CommandEntry `json:"orphan-yang"`
	OrphanHandlers      []string       `json:"orphan-handlers"`
	OrphanLocalHandlers []string       `json:"orphan-local-handlers"`
	SkippedHandlers     []string       `json:"skipped-handlers"`
	Total               int            `json:"total-yang"`
	TotalHandlers       int            `json:"total-handlers"`
	TotalLocal          int            `json:"total-local-handlers"`
	Valid               bool           `json:"valid"`
	// Warnings names a declared -cmd module the loader does not hold. The
	// script printed each one before its report and put none of them in its
	// JSON, so this field stays out of the JSON too.
	Warnings []string `json:"-"`
}

// Text renders the validation result the way the script printed it: the four
// counts, a section per orphan kind, the verdict, and the full command table.
// It ends in a newline.
func (r ValidationResult) Text() string {
	var tb textbuf.Buffer

	for _, warning := range r.Warnings {
		tb.Str("warning: module ").Str(warning).Str(" not found\n")
	}

	tb.Str("# Command Validation\n\n")
	tb.Str("YANG commands: ").Int(int64(r.Total)).Byte('\n')
	tb.Str("Registered handlers: ").Int(int64(r.TotalHandlers)).Byte('\n')
	tb.Str("Registered local handlers: ").Int(int64(r.TotalLocal)).Byte('\n')
	tb.Str("Skipped (editor-internal): ").Int(int64(len(r.SkippedHandlers))).Str("\n\n")

	if len(r.OrphanYANG) > 0 {
		tb.Str("## YANG commands with no handler (").Int(int64(len(r.OrphanYANG))).Str(")\n\n")
		for _, cmd := range r.OrphanYANG {
			tb.Str("  ").Str(cmd.WireMethod).Str("  (").Str(cmd.YANGPath).
				Str(" in ").Str(cmd.Module).Str(")\n")
		}
		tb.Byte('\n')
	}

	if len(r.OrphanHandlers) > 0 {
		tb.Str("## Handlers with no YANG command (").Int(int64(len(r.OrphanHandlers))).Str(")\n\n")
		for _, wm := range r.OrphanHandlers {
			tb.Str("  ").Str(wm).Byte('\n')
		}
		tb.Byte('\n')
	}

	if len(r.OrphanLocalHandlers) > 0 {
		tb.Str("## Local handlers with no YANG command (").Int(int64(len(r.OrphanLocalHandlers))).Str(")\n\n")
		for _, path := range r.OrphanLocalHandlers {
			tb.Str("  ").Str(path).Byte('\n')
		}
		tb.Byte('\n')
	}

	if r.Valid {
		tb.Str("All commands validated.\n")
	} else {
		problems := len(r.OrphanYANG) + len(r.OrphanHandlers)
		tb.Str("FAILED: ").Int(int64(problems)).Str(" problem(s)\n")
	}

	tb.Str("\n## All YANG commands (").Int(int64(r.Total)).Str(")\n\n")
	tb.Str("| WireMethod | YANG Path | Module |\n")
	tb.Str("|------------|-----------|--------|\n")
	for _, cmd := range r.YANGCommands {
		tb.Str("| ").Str(cmd.WireMethod).Str(" | ").Str(cmd.YANGPath).
			Str(" | ").Str(cmd.Module).Str(" |\n")
	}

	if len(r.SkippedHandlers) > 0 {
		tb.Str("\n## Skipped handlers (editor-internal)\n\n")
		tb.Str("| WireMethod | Reason |\n")
		tb.Str("|------------|--------|\n")
		for _, wm := range r.SkippedHandlers {
			tb.Str("| ").Str(wm).Str(" | ").Str(skipReason(wm)).Str(" |\n")
		}
	}

	return tb.String()
}

// WriteReport is what the writing action answers: the file it rewrote.
type WriteReport struct {
	Path string `json:"path"`
}

// Text renders the writing action the way the script printed it.
func (w WriteReport) Text() string {
	var tb textbuf.Buffer
	return tb.Str("wrote ").Str(w.Path).Byte('\n').String()
}
