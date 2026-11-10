// Design: docs/contributing/rfc-conformance-gates.md -- what an interop observation must show
// Overview: discriminate_observe.go -- the observation this judgement serves
// Related: discriminate.go -- interopCitationState, the number a citation must be
//
// discriminate_interop.go hands an interop observation's output to the shared
// verdict readers (internal/le/test/overlay, interop.go) with what the record
// holds: the unit it names and the assertion it cites.

package rfc

import (
	testoverlay "github.com/ze-software/ze/internal/le/test/overlay"
)

// interopCleanUnobserved answers why a clean interop run did not observe the
// scenario passing, or the empty string when it did (testoverlay.CleanUnobserved).
func (o *observationRunner) interopCleanUnobserved(output string) string {
	return testoverlay.CleanUnobserved(output, o.scenario, o.record.Unit)
}

// interopBrokenUnobserved answers why a run under the break did not observe
// the scenario failing, or the empty string when it did
// (testoverlay.BrokenUnobserved).
func (o *observationRunner) interopBrokenUnobserved(output string) string {
	return testoverlay.BrokenUnobserved(output, o.scenario, o.record.Unit)
}

// interopAttribution answers why a failing interop run is NOT the cited
// assertion's red, or the empty string when it is (testoverlay.Attribution).
// The citation is the record's (interopCitationState).
func (o *observationRunner) interopAttribution(output string) string {
	return testoverlay.Attribution(output, o.scenario, o.record.Citation)
}
