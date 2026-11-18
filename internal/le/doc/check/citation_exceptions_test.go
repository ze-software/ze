// Design: docs/architecture/core-design.md -- command-citation judgment and declared line exceptions
package doccheck

import "testing"

func TestCitationNonCommandExceptionsKeepOtherLinesJudged(t *testing.T) {
	// VALIDATES: the nearest-name example and executable-construction data
	// need their precise declared file and line. The same text elsewhere is
	// still judged, as is a live unknown command beside each exception.
	// PREVENTS: a legitimate non-command sample exempting a whole source file.
	for _, sample := range []struct {
		file string
		line string
	}{
		{"docs/architecture/core-design.md", "(`le unit test` names `le test unit`)"},
		{"internal/le/feature/recordrun.go", `testqemu.KeywordGuestLe, le, "timeout", guestCommandTimeout,`},
		{"internal/le/rfc/weekly_source.go", `fromLe, err := endpointLeOf(ctx, "from", events.FromDir, events.WorkDir)`},
		{"internal/le/rfc/weekly_source.go", `toLe, err := endpointLeOf(ctx, "to", events.ToDir, events.WorkDir)`},
	} {
		t.Run(sample.file+"/"+sample.line, func(t *testing.T) {
			if got := unheldCitations(sample.file, 1, sample.line); len(got) != 0 {
				t.Errorf("declared non-command sample was judged: %+v", got)
			}
			if got := unheldCitations("docs/undeclared.md", 1, sample.line); len(got) != 1 {
				t.Errorf("same text elsewhere yielded %+v, want one finding", got)
			}
			if got := unheldCitations(sample.file, 2, `"le absent-citation-probe"`); len(got) != 1 {
				t.Errorf("live unknown command beside exception yielded %+v, want one finding", got)
			}
		})
	}
}
