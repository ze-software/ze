// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: vppevidence.go -- the run that fills this report
//
// The report keeps one scenario row for each producer function and one check row
// for each claim that function makes. The text view is derived from these rows.
package deployment

import "github.com/ze-software/ze/internal/core/textbuf"

// VPPProofVerdict is the result of one scenario or check.
//
// The zero value is invalid. Thus, a scenario that the run did not reach cannot
// appear as proof.
type VPPProofVerdict uint8

const (
	// VPPProofUnspecified is a scenario or check that did not run.
	VPPProofUnspecified VPPProofVerdict = iota
	// VPPProofPass is proof that VPP reported the required state.
	VPPProofPass
	// VPPProofFail is a completed query that did not report the required state.
	VPPProofFail
)

// String answers the report word for a verdict.
func (v VPPProofVerdict) String() string {
	switch v {
	case VPPProofPass:
		return "pass"
	case VPPProofFail:
		return "fail"
	case VPPProofUnspecified:
		return reportValueUnspecified
	}
	return reportValueUnspecified
}

// MarshalJSON writes the report word instead of its numeric identity.
func (v VPPProofVerdict) MarshalJSON() ([]byte, error) {
	var tb textbuf.Buffer
	return []byte(tb.Quoted(v.String()).String()), nil
}

// VPPCheck is one observable claim inside a scenario.
type VPPCheck struct {
	Check    string          `json:"check"`
	Verdict  VPPProofVerdict `json:"verdict"`
	Detail   string          `json:"detail"`
	Evidence []string        `json:"evidence,omitempty"`
	LogTail  []string        `json:"log-tail,omitempty"`
}

// VPPScenarioReport is one producer scenario and its checks.
type VPPScenarioReport struct {
	Scenario string          `json:"scenario"`
	Verdict  VPPProofVerdict `json:"verdict"`
	Checks   []VPPCheck      `json:"checks"`
}

// VPPReport is one run of the real VPP deployment proof.
type VPPReport struct {
	Image     string              `json:"image"`
	Container string              `json:"container"`
	Version   string              `json:"vpp-version"`
	Interface string              `json:"interface"`
	Scenarios []VPPScenarioReport `json:"scenarios"`
	Passed    bool                `json:"passed"`
}

// Text renders the report in the producer's order. It writes each check from the
// structured payload and never keeps a second list of scenario results.
func (r VPPReport) Text() string {
	var tb textbuf.Buffer
	if r.Interface != "" {
		tb.Str("OK: created real VPP loopback interface ").Str(r.Interface).Byte('\n')
	}
	for i := range r.Scenarios {
		scenario := &r.Scenarios[i]
		for j := range scenario.Checks {
			check := &scenario.Checks[j]
			if check.Verdict == VPPProofPass {
				tb.Str("OK: ").Str(check.Detail).Byte('\n')
			} else {
				tb.Str("FAIL: ").Str(check.Detail).Byte('\n')
			}
			for _, line := range check.Evidence {
				tb.Str(line).Byte('\n')
			}
			if len(check.LogTail) == 0 {
				continue
			}
			tb.Str("ze log tail:\n")
			for _, line := range check.LogTail {
				tb.Str(line).Byte('\n')
			}
		}
	}
	return tb.String()
}
