// Design: docs/architecture/core-design.md -- what the vendored-web commands answer
//
// report.go holds what the three commands ANSWER, apart from what produced it.
//
// Each answer is structured data with one row set in it, so `| json` feeds a
// script, `| match DRIFT` keeps one kind of problem and `| count` says how many.
// Each also renders ITSELF (Text), because the two scripts these replace print
// a walk rather than a table, and the walk is what a person reads. That
// rendering is the default and nothing more: the data is the same either way
// (internal/le/leroot, Prose).
//
// The renderings here are BYTE-IDENTICAL to what the scripts print. No color is
// involved on either side, so unlike the consistency port there is no palette
// to trade away, and internal/le/vendorweb/vendorweb_test.go compares the two streams
// exactly.

package vendorweb

import (
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The four verdicts the drift comparison can reach about one file. They are the
// first word of the line a reader sees, and they are what `| match` selects on.
const (
	// ProblemUnreadable is a source file under third_party/web/ that could not
	// be read. Nothing can be compared against it.
	ProblemUnreadable = "UNREADABLE"
	// ProblemMissing is a consumer that subscribes to a package and holds no
	// copy of one of its files. The sync was never told to write it.
	ProblemMissing = "MISSING"
	// ProblemDrift is a consumer copy whose bytes differ from the source.
	ProblemDrift = "DRIFT"
	// ProblemUnsynced is a vendored package that reaches no consumer at all.
	ProblemUnsynced = "UNSYNCED"
)

// Problem is one thing the drift comparison found, and it is one ROW of the
// check's answer.
type Problem struct {
	// Kind is one of the four constants above.
	Kind string `json:"kind"`
	// File is the consumer copy the problem is about, relative to the tree. For
	// ProblemUnsynced it is the vendored package directory instead, because
	// that verdict is about a package no consumer copied.
	File string `json:"file"`
	// Source is the third_party/web/ file the copy is compared against. It is
	// absent for ProblemUnsynced, which names no single file.
	Source string `json:"source,omitempty"`
	// Detail is the read error, for ProblemUnreadable alone.
	Detail string `json:"detail,omitempty"`
}

// PackageVersion is what the registry query found about one vendored npm
// package. Current is what MANIFEST.md records and Latest is what the registry
// answered.
//
// The four states a reader sees are DERIVED from these three fields rather than
// stored beside them, because a stored state and the fields it summarizes can
// disagree: no current version means the manifest did not name one, no latest
// version means the query did not answer (and Err says why), equal versions
// mean up to date, and anything else is an upgrade.
type PackageVersion struct {
	Package string `json:"package"`
	Current string `json:"current,omitempty"`
	Latest  string `json:"latest,omitempty"`
	Err     string `json:"error,omitempty"`
}

// UpdateReport is the registry query's answer. It is nested under the check's
// report rather than beside its rows, so the answer keeps ONE row set and the
// row operators know what they act on.
type UpdateReport struct {
	Packages []PackageVersion `json:"packages"`
}

// CheckReport is the whole answer of one `vendor-web-check` run.
//
// Problems is the only row set in it, which is what lets the engine find the
// rows and act on them with `| match`, `| first` and `| count`
// (internal/component/command/answer_shape.go, rowsIn). Skipped is a map from
// path to reason rather than a second list for the same reason: two row sets in
// one answer are ambiguous, and the engine reports no rows at all for them.
type CheckReport struct {
	// Updates is the registry query's answer, present only when it ran.
	Updates *UpdateReport `json:"updates,omitempty"`
	// Skipped maps each vendor-directory entry that was passed over to the
	// reason it was passed over.
	Skipped map[string]string `json:"skipped"`
	// Compared is how many consumer copies were read and compared. A run that
	// compared none has proven nothing, which is why the verdict reads it.
	Compared int `json:"compared"`
	// Problems is every problem found, in the order the walk found them.
	Problems []Problem `json:"problems"`
	// DriftChecked says the consumer-copy comparison was reached. It is false
	// only when the registry query failed first and the run ended there.
	DriftChecked bool `json:"drift-checked"`
}

// Text renders the check for a person: the registry report when one ran, then
// the consumer-copy walk, then one line per problem, then the verdict. It ends
// in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer

	if r.Updates != nil {
		tb.Str("checking vendored web assets against npm registry...\n\n")
		for _, pkg := range r.Updates.Packages {
			tb.Str("  ").Str(pkg.Package).Str(": ")
			switch {
			case pkg.Current == "":
				tb.Str("version not found in MANIFEST.md")
			case pkg.Latest == "":
				tb.Str("could not fetch latest version (").Str(pkg.Err).Byte(')')
			case pkg.Current == pkg.Latest:
				tb.Str(pkg.Current).Str(" (up to date)")
			default:
				tb.Str(pkg.Current).Str(" -> ").Str(pkg.Latest).Str(" available")
			}
			tb.Byte('\n')
		}
		tb.Byte('\n')
	}

	if !r.DriftChecked {
		return tb.String()
	}

	tb.Str("checking consumer copies...\n")

	// Sorted, which is the order os.ReadDir hands the entries over and so the
	// order the script printed them in: every key is a sibling under one
	// directory, so sorting the paths sorts the names.
	for _, dir := range sortedSkips(r.Skipped) {
		tb.Str("  skipped ").Str(dir).Str(": ").Str(r.Skipped[dir]).Byte('\n')
	}

	for _, problem := range r.Problems {
		switch problem.Kind {
		case ProblemUnreadable:
			tb.Str("  UNREADABLE: ").Str(problem.Source).Str(" (").Str(problem.Detail).Str(")\n")
		case ProblemMissing:
			tb.Str("  MISSING: ").Str(problem.File).Byte('\n')
		case ProblemDrift:
			tb.Str("  DRIFT: ").Str(problem.File).Str(" differs from ").Str(problem.Source).Byte('\n')
		case ProblemUnsynced:
			tb.Str("  UNSYNCED: ").Str(problem.File).
				Str(" reaches no consumer; add it to internal/le/vendorweb/actions.go\n")
		}
	}

	// The verdict line belongs to a run that read something and found nothing.
	// A run that compared no copy proved nothing and says so through its exit
	// code instead (Check, the fail-closed guard).
	if len(r.Problems) == 0 && r.Compared > 0 {
		tb.Str("  all ").Int(int64(r.Compared)).Str(" consumer copies match their ").
			Str(vendorDir).Str("/ source\n")
	}

	return tb.String()
}

// The two things one sync visit can produce. A visit that found the copy
// already correct produces neither and is not a row.
const (
	// SyncWritten is a consumer copy this run wrote.
	SyncWritten = "written"
	// SyncWarned is a source or a destination the run could not use. The
	// script printed these on stderr and carried on, and so does the command.
	SyncWarned = "warning"
)

// SyncedFile is one visit that produced output, and it is one ROW of the sync's
// answer. Keeping the warnings in the same list as the writes is what keeps the
// answer to ONE row set, and it keeps them in the order they happened, which a
// map keyed by path could not: one missing source file warns once per consumer.
type SyncedFile struct {
	// Kind is SyncWritten or SyncWarned.
	Kind string `json:"kind"`
	// Path is the file that was written, or the file or directory the warning
	// is about.
	Path string `json:"path"`
	// Message is the warning text. It is absent for a write.
	Message string `json:"message,omitempty"`
}

// SyncReport is the whole answer of one `vendor-web-sync` run.
type SyncReport struct {
	// Files is every visit that produced output, in the order they happened.
	Files []SyncedFile `json:"files"`
	// Changed is how many consumer copies were written.
	Changed int `json:"changed"`
	// Readable is how many vendor source files the run read. Zero means the
	// run wrote nothing because it found nothing, which is not the same fact
	// as a tree already up to date, and Sync refuses to report it as one.
	Readable int `json:"readable"`
}

// Text renders one line for each file written. A clean run reports that all
// consumer copies are current. Warnings use the stderr rendering.
func (r SyncReport) Text() string {
	var tb textbuf.Buffer

	for _, file := range r.Files {
		if file.Kind != SyncWritten {
			continue
		}
		tb.Str("synced: ").Str(file.Path).Byte('\n')
	}

	// A run that read no source wrote nothing because it found nothing, and it
	// must not print what an already-synced tree prints. Sync answers that case
	// as an error instead (Sync, the fail-closed guard).
	if r.Readable == 0 {
		return tb.String()
	}

	if r.Changed == 0 {
		tb.Str("all consumer copies are up to date\n")
		return tb.String()
	}
	return tb.String()
}

// Warnings answers the lines the sync writes to stderr, in order. They are a
// second READING of the rows the answer already carries, never a second list.
func (r SyncReport) Warnings() []string {
	var out []string
	for _, file := range r.Files {
		if file.Kind != SyncWarned {
			continue
		}
		out = append(out, file.Message)
	}
	return out
}

// sortedSkips answers the skipped paths in order, so a rendering built from a
// map is stable.
func sortedSkips(skipped map[string]string) []string {
	out := make([]string, 0, len(skipped))
	for dir := range skipped {
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}
