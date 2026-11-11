// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon.
// Related: report.go -- the wire type and bounds under test

package rpc

import (
	"strconv"
	"strings"
	"testing"
)

// TestValidateReportIssue proves every bound of a report-issue request is
// refused one octet past its limit and accepted at it, and that each refusal
// names the field and the bound, so a plugin author can fix the call.
//
// The method starts each case from a valid raise or a valid clear, changes
// one field, and checks the answer.
func TestValidateReportIssue(t *testing.T) {
	raise := ReportIssueInput{
		Action:   ReportRaise,
		Severity: ReportWarning,
		Code:     "doctor-internal-defect",
		Subject:  "peer 192.0.2.1",
		Message:  "the decoder met a state only a defect produces",
		Fix:      "show warnings",
	}
	clearing := ReportIssueInput{Action: ReportClear, Code: "doctor-internal-defect", Subject: "peer 192.0.2.1"}
	cases := []struct {
		name   string
		base   ReportIssueInput
		change func(*ReportIssueInput)
		want   []string // substrings of the refusal; nil means accepted
	}{
		{"raise warning", raise, func(*ReportIssueInput) {}, nil},
		{"raise error", raise, func(in *ReportIssueInput) { in.Severity = ReportError }, nil},
		{"raise with no fix", raise, func(in *ReportIssueInput) { in.Fix = "" }, nil},
		{"clear", clearing, func(*ReportIssueInput) {}, nil},
		{"clear naming warning", clearing, func(in *ReportIssueInput) { in.Severity = ReportWarning }, nil},
		{"code at bound", raise, func(in *ReportIssueInput) { in.Code = strings.Repeat("a", ReportCodeMax) }, nil},
		{"code above bound", raise, func(in *ReportIssueInput) { in.Code = strings.Repeat("a", ReportCodeMax+1) },
			[]string{"code", strconv.Itoa(ReportCodeMax)}},
		{"subject at bound", raise, func(in *ReportIssueInput) { in.Subject = strings.Repeat("s", ReportSubjectMax) }, nil},
		{"subject above bound", raise, func(in *ReportIssueInput) { in.Subject = strings.Repeat("s", ReportSubjectMax+1) },
			[]string{"subject", strconv.Itoa(ReportSubjectMax)}},
		{"message at bound", raise, func(in *ReportIssueInput) { in.Message = strings.Repeat("m", ReportMessageMax) }, nil},
		{"message above bound", raise, func(in *ReportIssueInput) { in.Message = strings.Repeat("m", ReportMessageMax+1) },
			[]string{"message", strconv.Itoa(ReportMessageMax)}},
		{"fix at bound", raise, func(in *ReportIssueInput) { in.Fix = strings.Repeat("f", ReportFixMax) }, nil},
		{"fix above bound", raise, func(in *ReportIssueInput) { in.Fix = strings.Repeat("f", ReportFixMax+1) },
			[]string{"fix", strconv.Itoa(ReportFixMax)}},
		{"empty code", raise, func(in *ReportIssueInput) { in.Code = "" }, []string{"code", "empty"}},
		{"empty subject", clearing, func(in *ReportIssueInput) { in.Subject = "" }, []string{"subject", "empty"}},
		{"raise with empty message", raise, func(in *ReportIssueInput) { in.Message = "" }, []string{"message", "empty"}},
		{"unknown action", raise, func(in *ReportIssueInput) { in.Action = "publish" }, []string{"action", "publish"}},
		{"raise with no severity", raise, func(in *ReportIssueInput) { in.Severity = "" }, []string{"severity"}},
		{"raise with unknown severity", raise, func(in *ReportIssueInput) { in.Severity = "fatal" }, []string{"severity", "fatal"}},
		{"clear of an error", clearing, func(in *ReportIssueInput) { in.Severity = ReportError }, []string{"error", "cannot be cleared"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.base
			tc.change(&in)
			err := ValidateReportIssue(&in)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("ValidateReportIssue refused a valid request: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateReportIssue accepted %+v, want a refusal naming %v", in, tc.want)
			}
			for _, part := range tc.want {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("refusal %q does not name %q", err, part)
				}
			}
		})
	}
}
