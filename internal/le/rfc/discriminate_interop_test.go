package rfc

import (
	"strings"
	"testing"
)

// interopJudgeFixture is an interop observation of one scenario citing one
// assertion.
func interopJudgeFixture(citation string) *observationRunner {
	return &observationRunner{
		carrier:  Carrier{Kind: kindInterop},
		scenario: "relay-shape",
		record: DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityNegative,
			Unit:  "internal/le/interoplab/bgp/check_rfc.go::checkRelayWithdrawalShape",
			Route: RouteMutant, Producer: selftestProducerUnit, Break: selftestBreak,
			Citation: citation},
	}
}

// The output of a run whose ze cross-compile failed: the lab never started a
// scenario, and the suite still printed its summary.
const interopBuildFailure = "interop: stage ze: go build: ./peer.go:12:2: declared and not used: x\n" +
	"interop: 0 passed, 0 failed\n"

// VALIDATES: a clean interop run counts as green only when the selected
// scenario printed its own pass line.
// METHOD: cleanRunUnobserved over a run that executed nothing and over one that
// passed the scenario, a sibling whose name extends it included.
// PREVENTS: the RFC4271-5.1.2-3 record of 2026-10-23, sealed over a run whose
// ze cross-compile failed: `0 passed, 0 failed` was read as a result
// (plan/journal/green-that-could-not-have-been-red.md).
func TestInteropCleanRunMustShowTheScenarioPass(t *testing.T) {
	runner := interopJudgeFixture("6")
	if why := runner.cleanRunUnobserved("interop: 0 passed, 0 failed\n"); why == "" {
		t.Fatal("a clean run that executed no scenario was accepted as green")
	}
	sibling := "interop: [1/1] relay-shape-v6: pass (40s)\n"
	if why := runner.cleanRunUnobserved(sibling); why == "" {
		t.Fatal("a sibling scenario's pass was accepted as relay-shape's")
	}
	passed := "interop: images ready (3s)\ninterop: [1/1] relay-shape: running\n" +
		"interop: [1/1] relay-shape: pass (41s)\n"
	if why := runner.cleanRunUnobserved(passed); why != "" {
		t.Fatalf("a clean run that passed the scenario was refused: %s", why)
	}
}

// VALIDATES: a broken interop run counts as the record's red only when the
// selected scenario FAILED and its FAIL line names the cited assertion.
// METHOD: judgeRed over a build failure, a different assertion, an assertion
// whose number extends the cited one, and the cited assertion itself.
// PREVENTS: a negative record sealed by a mutant that never compiled, and a red
// from another assertion of the same checker published as the claim's proof.
func TestInteropRedMustNameTheCitedAssertion(t *testing.T) {
	runner := interopJudgeFixture("6")
	err := runner.judgeRed(false, interopBuildFailure)
	if err == nil {
		t.Fatal("a run whose build failed before any scenario ran was recorded as a red")
	}
	if !strings.Contains(err.Error(), "did not run under the break") {
		t.Errorf("the refusal does not say the scenario never ran: %v", err)
	}

	other := "interop: [1/1] relay-shape: FAIL (30s): scenario relay-shape assertion 2: no route\n" +
		"interop: 0 passed, 1 failed: relay-shape\n"
	err = runner.judgeRed(false, other)
	if err == nil {
		t.Fatal("a red from assertion 2 was recorded as assertion 6's proof")
	}
	if !strings.Contains(err.Error(), "assertion 6") {
		t.Errorf("the refusal does not name the cited assertion: %v", err)
	}

	longer := "interop: [1/1] relay-shape: FAIL (30s): scenario relay-shape assertion 61: no route\n"
	if runner.judgeRed(false, longer) == nil {
		t.Fatal("assertion 61 was read as assertion 6")
	}

	cited := "interop: [1/1] relay-shape: FAIL (30s): scenario relay-shape assertion 6: " +
		"withdrawal carried an AS_PATH\ninterop: 0 passed, 1 failed: relay-shape\n"
	if err := runner.judgeRed(false, cited); err != nil {
		t.Fatalf("a red naming the cited assertion was refused: %v", err)
	}

	stayed := "interop: [1/1] relay-shape: pass (41s)\n"
	err = runner.judgeRed(true, stayed)
	if err == nil || !strings.Contains(err.Error(), "GREEN") {
		t.Fatalf("a scenario that passed under the break was not refused as green: %v", err)
	}
}

// VALIDATES: R2-4 -- the cited assertion is read where the checker writes it,
// `scenario <name> assertion <N>:` (checkerFailure in
// internal/le/interoplab/bgp/check_engine.go), and nowhere else on the line.
// METHOD: judgeRed over FAIL lines that carry `assertion 6` in the cause of
// another assertion, under another scenario's name, with no scenario anchor,
// and without the colon that closes the number, then over the anchored form.
// PREVENTS: a red from assertion 2 whose cause text happens to quote
// "assertion 6" being recorded as assertion 6's proof.
func TestInteropCitedAssertionIsReadAtTheCheckerAnchor(t *testing.T) {
	runner := interopJudgeFixture("6")
	const head = "interop: [1/1] relay-shape: FAIL (30s): "
	refused := map[string]string{
		"the cause of another assertion": head +
			"scenario relay-shape assertion 2: wait timed out after assertion 6 setup\n",
		"another scenario's name": head + "scenario relay-other assertion 6: no route\n",
		"no scenario anchor":      head + "lab: assertion 6: no route\n",
		"no closing colon":        head + "scenario relay-shape assertion 6 no route\n",
	}
	for name, output := range refused {
		if err := runner.judgeRed(false, output); err == nil {
			t.Errorf("a FAIL line with %s was recorded as assertion 6's red", name)
		}
	}
	anchored := head + "scenario relay-shape assertion 6: FRR saw NEXT_HOP missing\n"
	if err := runner.judgeRed(false, anchored); err != nil {
		t.Fatalf("a red at the anchored assertion was refused: %v", err)
	}
	// The shape the relay checker's red reaches the suite in (2026-10-24): an
	// outer checkerFailure wraps the inner one, and the inner one is cited.
	wrapped := head + "scenario relay-shape assertion 0: scenario relay-shape assertion 6: " +
		"FRR refused the attributes of an UPDATE ze relayed\n"
	if err := runner.judgeRed(false, wrapped); err != nil {
		t.Fatalf("a red at the wrapped anchored assertion was refused: %v", err)
	}
	outer := head + "scenario relay-shape assertion 6: scenario relay-shape assertion 2: no route\n"
	if runner.judgeRed(false, outer) == nil {
		t.Fatal("a wrapper's number was read in place of the inner assertion 2")
	}
}
