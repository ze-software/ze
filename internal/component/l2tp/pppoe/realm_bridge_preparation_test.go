// Design: docs/architecture/l2tp/followup-l2tp-call.md -- pre-bridge original release.
package pppoe

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/callsink"
)

func prepareRealmBridgeForTest(t *testing.T, harness *realmHarness) {
	t.Helper()
	preparation, ok := harness.handoff().(callsink.BridgePreparation)
	if !ok {
		t.Fatal("realm handoff has no original release preparation")
	}
	if err := preparation.PrepareBridge(); err != nil {
		t.Fatal(err)
	}
}

// Preparation releases once before fresh bridge attachment, and a later
// native failure ends the exact access admission instead of attempting an
// authentication reply through already closed PPP files.
func TestRealmPreparedBridgeFailureEndsOriginalAdmission(t *testing.T) {
	harness := newRealmHarness(t)
	prepareRealmBridgeForTest(t, harness)
	prepareRealmBridgeForTest(t, harness)
	if released, _ := harness.driver.counts(); released != 1 {
		t.Fatalf("releases=%d", released)
	}
	harness.handoff().Bridged(errors.New("original native bridge refused"))
	if sid := waitSignal(t, harness.ended, "the prepared subscriber end"); sid != 5 {
		t.Fatalf("ended session=%d", sid)
	}
	harness.relay.stopAll()
	if released, refusals := harness.driver.counts(); released != 1 || len(refusals) != 0 {
		t.Fatalf("released=%d refusals=%v", released, refusals)
	}
}

// This private callback-error model checks retained preparation errors and
// captured End. Actual attempt release-state guards are exercised separately
// by TestRealmPreparationChecksActualRetiredReleaseState.
func TestRealmPreparationRetainsInjectedReleaseFailure(t *testing.T) {
	harness := newRealmHarness(t)
	want := errors.New("original native release is pending")
	harness.relay.released = func(_ ppp.EventAuthRequest) error { return want }
	preparation := harness.handoff().(callsink.BridgePreparation)
	if err := preparation.PrepareBridge(); !errors.Is(err, want) {
		t.Fatalf("prepare=%v", err)
	}
	if err := preparation.PrepareBridge(); !errors.Is(err, want) {
		t.Fatalf("repeat prepare=%v", err)
	}
	harness.handoff().Ended("setup refused before peer capture")
	if sid := waitSignal(t, harness.ended, "pending original admission end"); sid != 5 {
		t.Fatalf("ended session=%d", sid)
	}
	harness.relay.stopAll()
}
