// Design: docs/architecture/l2tp/bng-5-pppoe.md -- exact retained ownership

package vpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/memclnt"
)

// Process proof must reach an active session's captured API before cleanup
// transfers into the pending ledger. An unrelated API remains live, and the
// covered owner later closes its socket without sending obsolete native work.
func TestVPPReconcileRetiresActiveReservationBeforePendingTransfer(t *testing.T) {
	isolateReconciliation(t)
	harness := &factoryHarness{}
	opened, err := openWithOps(t.Context(), factoryRequest(), harness.ops())
	if err != nil {
		t.Fatal(err)
	}
	owner, ok := opened.Dataplane().(*session)
	if !ok {
		t.Fatal("factory did not return its session owner")
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	retireCoveredSessions(func(client *binaryAPI) bool { return false })
	if owner.api.processExited() {
		t.Fatal("unrelated process proof retired the active API")
	}
	retireCoveredSessions(func(client *binaryAPI) bool { return client == owner.api })
	if !owner.api.processExited() {
		t.Fatal("active reservation lost the process proof before pending transfer")
	}
	requests := len(harness.requests)
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if len(harness.requests) != requests {
		t.Fatal("proven process exit sent native cleanup")
	}
	if harness.stream.closes != 1 {
		t.Fatalf("socket closes=%d", harness.stream.closes)
	}
}

// Retired aliases refuse both mutations and queries before choosing a native
// stream, including a constructing owner which has not reached session.Close.
func TestVPPReconcileRetiredAPIRefusesNativeSelection(t *testing.T) {
	client := &binaryAPI{open: func(context.Context) (api.Stream, error) {
		t.Error("retired API selected a native stream")
		return nil, errors.New("retired API selected a native stream")
	}}
	client.retired.Store(true)
	if _, err := client.exchange(t.Context(), &memclnt.ControlPing{}); err == nil {
		t.Fatal("retired API exchange succeeded")
	}
	if err := client.dump(t.Context(), &memclnt.ControlPing{}, 1, func(api.Message) error { return nil }); err == nil {
		t.Fatal("retired API dump succeeded")
	}
}
