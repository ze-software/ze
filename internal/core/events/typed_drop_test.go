package events

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
)

// TestTypedDropRaisesInternalDefect proves a typed event dropped because its
// payload is not the registered type is logged under doctor-internal-defect
// and raised on the report bus under it, so the recent-events doctor check
// reports the drop. Method: an AsString wrapper handed a non-string payload,
// then the log and the bus are read.
//
// VALIDATES: AC-42 (spec-doctor-coverage-12-plugin-infra), the drops half.
// PREVENTS: a payload mismatch only a debug reader of the log would ever see.
func TestTypedDropRaisesInternalDefect(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var buf bytes.Buffer
	SetLogger(captureLogger(&buf))
	t.Cleanup(func() { SetLogger(slog.Default) })

	wrap := AsString(func(string) {})
	// Red: the raiseDrop call in AsString's wrong-type branch.
	wrap(&asStringPayload{N: 1})

	if !strings.Contains(buf.String(), "code="+report.CodeInternalDefect) {
		t.Errorf("drop logged without its code: %q", buf.String())
	}
	var found *report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == report.CodeInternalDefect && issue.Source == eventBusSource {
			found = &issue
		}
	}
	if found == nil {
		t.Fatalf("no internal-defect event on the bus: %v", report.Errors(0))
	}
	if !strings.Contains(found.Message, "asStringPayload") {
		t.Errorf("event message %q does not name the payload type", found.Message)
	}
}
