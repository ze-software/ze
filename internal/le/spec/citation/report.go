// Design: docs/architecture/core-design.md -- le's native development gates
// Related: speccitation.go -- the citation population and scan rules

package speccitation

import "github.com/ze-software/ze/internal/core/textbuf"

// DocumentLocation identifies the document and line that carries a citation.
type DocumentLocation struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// SourceLocation identifies the source line that a token is expected to occupy.
// Line stays decimal text because the producer accepts citation line numbers of
// arbitrary size and reports them without narrowing them to a machine integer.
type SourceLocation struct {
	Path string `json:"path"`
	Line string `json:"line"`
}

// DanglingFinding identifies an active spec citation whose target is absent.
type DanglingFinding struct {
	Citer  DocumentLocation `json:"citer"`
	Target string           `json:"target-path"`
}

// DriftFinding identifies a citation whose neighboring token is absent from
// the cited source line. Drift is advisory and never changes the exit code.
type DriftFinding struct {
	Citer       DocumentLocation `json:"citer"`
	Source      SourceLocation   `json:"source"`
	SourceToken string           `json:"source-token"`
}

// Report is one citation scan. Baseline carries the unique allowlisted targets
// so a structured caller can distinguish a checked exception from a count.
type Report struct {
	Specs    int               `json:"specs"`
	Baseline []string          `json:"baseline"`
	Dangling []DanglingFinding `json:"dangling"`
	Warnings []DriftFinding    `json:"warnings"`
}

// Text renders the native citation verdict and its actionable diagnostics.
func (r Report) Text() string {
	var tb textbuf.Buffer
	for _, warning := range r.Warnings {
		tb.Str("WARN ").Str(warning.Citer.Path).Byte(':').Int(int64(warning.Citer.Line)).
			Str(": citation `").Str(warning.Source.Path).Byte(':').Str(warning.Source.Line).
			Str("` no longer shows token `").Str(warning.SourceToken).
			Str("` on that line (line-token drift)\n")
	}

	if len(r.Dangling) > 0 {
		tb.Str("./le spec citation FAILED: dangling plan/spec-*.md references\n")
		for _, finding := range r.Dangling {
			tb.Str("  ").Str(finding.Citer.Path).Byte(':').Int(int64(finding.Citer.Line)).
				Str(": references ").Str(finding.Target).
				Str(" which is absent on disk (not in baseline)\n")
		}
		tb.Byte('\n').Int(int64(len(r.Dangling))).
			Str(" dangling reference(s). Either fix the citing reference,").
			Str(" or -- if the target is legitimately gone -- add it to ").
			Str(baselinePath).
			Str(".\n")
		return tb.String()
	}

	tb.Str("./le spec citation OK (").Int(int64(r.Specs)).Str(" specs, ").
		Int(int64(len(r.Baseline))).Str(" baselined dangling")
	if len(r.Warnings) > 0 {
		tb.Str(", ").Int(int64(len(r.Warnings))).Str(" line-token WARN")
	}
	tb.Str(")\n")
	return tb.String()
}

func verdict(report Report) int {
	if len(report.Dangling) > 0 {
		return 1
	}
	return 0
}
