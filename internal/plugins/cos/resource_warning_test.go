// Design: docs/architecture/traffic/cos-dynamic.md -- original cleanup warning custody
// Related: diagnostic_shared.go -- concrete report reservation and episode
//go:build ze_l2tp

package cos

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: an old completed rollback remains idempotent after its released access
// name is reused. Method: B joins A, B rolls back, A leaves, C owns a fresh
// target; replay B's exact token while C is healthy or retains its own failure.
func TestCoSCompletedRollbackDoesNotTouchReplacementOwner(t *testing.T) {
	for _, failedReplacement := range []bool{false, true} {
		t.Run(fmt.Sprintf("replacement-failed-%t", failedReplacement), func(t *testing.T) {
			f := newSharedCoSTestOwner(t)
			f.handler.captureTarget = captureModeledOriginalCoSTarget(f.handler)
			const first, joined, replacement = uint16(64110), uint16(64111), uint16(64112)
			f.up(first, sharedCoSTestFirst)
			old := f.handler.accesses[sharedCoSTestInterface].sharedOwner()
			peer := netip.MustParseAddr("192.0.2.93")
			f.handler.mu.Lock()
			prepared, err := f.handler.prepareSessionChange(sessionTransactionRequest{
				SubscriberID: fixtureCoSID(sharedCoSTestTunnel, joined), TunnelID: sharedCoSTestTunnel,
				SessionID: joined, Interface: sharedCoSTestInterface, Profile: sharedCoSTestFirst}, peer)
			f.handler.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			f.handler.mu.Lock()
			err = f.handler.applySessionChange(prepared.Token, peer)
			f.handler.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			f.handler.mu.Lock()
			err = f.handler.rollbackSessionToken(prepared.Token, peer)
			f.handler.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			f.down(first)
			f.up(replacement, sharedCoSTestFirst)
			t.Cleanup(func() {
				f.failWrites = 0
				f.down(replacement)
			})
			current := f.handler.accesses[sharedCoSTestInterface].sharedOwner()
			if current == old {
				t.Fatal("replacement fixture did not capture a fresh original target")
			}
			if failedReplacement {
				f.failWrites = 1
				f.down(replacement)
			}
			warnings := report.Warnings()
			if failedReplacement {
				if len(warnings) != 1 {
					t.Fatal("replacement fixture did not retain its own cleanup warning")
				}
			}
			before := f.writes
			request, err := json.Marshal(sessionTransactionRequest{Operation: "rollback", Token: prepared.Token})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := f.handler.executeSessionTransaction([]string{"transaction", string(request)}, "192.0.2.94"); err == nil {
				t.Fatal("foreign peer acquired an original completed rollback acknowledgement")
			}
			status, _, err := f.handler.executeSessionTransaction([]string{"transaction", string(request)}, peer.String())
			if err != nil {
				t.Fatalf("completed old rollback lost idempotence after access reuse: %v", err)
			}
			if status != rpc.StatusDone {
				t.Fatalf("completed old rollback status=%q", status)
			}
			if f.writes != before {
				t.Fatal("old rollback replay wrote the replacement target")
			}
			if actual := f.handler.accesses[sharedCoSTestInterface].sharedOwner(); actual != current {
				t.Fatal("old rollback replay replaced current native custody")
			}
			if actual := report.Warnings(); !reflect.DeepEqual(actual, warnings) {
				t.Fatal("old rollback replay cleared or changed the replacement warning")
			}
		})
	}
}

// Goal: stopping the original event owner also fences its command callbacks.
// Method: prove a live transaction writes, then prepare another original token,
// stop that same handler and require its delayed apply to issue no write.
func TestCoSTransactionRefusesStoppedOriginalRunner(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.handler.captureTarget = captureModeledOriginalCoSTarget(f.handler)
	const session = uint16(64104)
	f.up(session, sharedCoSTestFirst)
	peer := "192.0.2.92"
	command := func(operation, token string) (any, error) {
		t.Helper()
		request := sessionTransactionRequest{Operation: operation, Token: token}
		if operation == "prepare" {
			request.SubscriberID = fixtureCoSID(sharedCoSTestTunnel, session)
			request.TunnelID = sharedCoSTestTunnel
			request.SessionID = session
			request.Interface = sharedCoSTestInterface
			request.Profile = sharedCoSTestDifferent
		}
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		_, answer, err := f.handler.executeSessionTransaction([]string{"transaction", string(data)}, peer)
		return answer, err
	}
	answer, err := command("prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	live, ok := answer.(sessionTransactionResult)
	if !ok {
		t.Fatalf("live prepare returned %T", answer)
	}
	before := f.writes
	if _, err := command("apply", live.Token); err != nil {
		t.Fatal(err)
	}
	if f.writes != before+1 {
		t.Fatal("live original transaction did not make one productive write")
	}
	if _, err := command("commit", live.Token); err != nil {
		t.Fatal(err)
	}
	answer, err = command("prepare", "")
	if err != nil {
		t.Fatal(err)
	}
	delayed, ok := answer.(sessionTransactionResult)
	if !ok {
		t.Fatalf("delayed prepare returned %T", answer)
	}
	f.handler.stop()
	before = f.writes
	if _, err := command("apply", delayed.Token); err == nil {
		t.Fatal("stopped original runner admitted a delayed native transaction")
	}
	if f.writes != before {
		t.Fatal("stopped original command wrote native maps")
	}
	// Teardown joins only this test's pending original before releasing fixture
	// state; it does not convert the refused command into product acceptance.
	f.handler.mu.Lock()
	if err := f.handler.rollbackSessionChange(); err != nil {
		f.handler.mu.Unlock()
		t.Fatal(err)
	}
	if err := f.handler.departSharedSession(fixtureCoSKey(sharedCoSTestTunnel, session)); err != nil {
		f.handler.mu.Unlock()
		t.Fatal(err)
	}
	f.handler.mu.Unlock()
}

// Goal: actual source shutdown cannot erase the original failed resource.
// Method: fail the registered owner's restore, clean its report source, then
// transfer the same target and complete only its original cleanup command.
func TestCoSCleanupResourceWarningSurvivesRunnerTransfer(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.handler.captureTarget = captureModeledOriginalCoSTarget(f.handler)
	const session = uint16(64101)
	f.up(session, sharedCoSTestFirst)
	f.failWrites = 1
	f.down(session)
	before := report.Warnings()
	if len(before) != 1 {
		t.Fatalf("original failed restore has %d warnings, want one", len(before))
	}
	access := f.handler.accesses[sharedCoSTestInterface].sharedOwner()
	report.ClearSource(Name)
	if actual := report.Warnings(); !reflect.DeepEqual(actual, before) {
		t.Fatalf("runner cleanup erased or changed the original resource warning: %+v", actual)
	}
	if err := report.TryClearWarning(Name, before[0].Code, before[0].Subject); !errors.Is(err, report.ErrResourceWarningOwned) {
		t.Fatalf("ordinary publisher acquired original warning clearance: %v", err)
	}
	registration := registry.Lookup(Name)
	registration.ConfigureEventBus(newTestBus())
	f.handler = dynamicHandler
	if actual := f.handler.accesses[sharedCoSTestInterface].sharedOwner(); actual != access {
		t.Fatal("runner replacement re-created the original map or warning owner")
	}
	f.handler.updateQoS = func(string, map[uint32]uint32, map[uint32]uint32) error {
		t.Fatal("original cleanup selected the replacement callback")
		return nil
	}
	beforeWrites := f.writes
	status, _, err := executeCoSCleanup(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if status != "done" {
		t.Fatalf("original cleanup status=%q", status)
	}
	if f.writes != beforeWrites+1 {
		t.Fatalf("original cleanup writes=%d, want exactly one", f.writes-beforeWrites)
	}
	f.requireMaps(f.original)
	if warnings := report.Warnings(); len(warnings) != 0 {
		t.Fatalf("confirmed original cleanup retained warning: %+v", warnings)
	}
	if f.handler.accesses[sharedCoSTestInterface] != nil {
		t.Fatal("confirmed target and warning release retained its owner")
	}
}

// Goal: a completed old recovery cannot clear a later failure on the same
// live resource. Method: actual failed apply/compensation, actual rollback,
// then another failed change while retaining the original admitted member.
func TestCoSCleanupOldEpisodeCannotClearNewFailure(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.handler.captureTarget = captureModeledOriginalCoSTarget(f.handler)
	const session = uint16(64102)
	f.up(session, sharedCoSTestFirst)
	h := f.handler
	h.mu.Lock()
	defer h.mu.Unlock()
	request := sessionTransactionRequest{SubscriberID: fixtureCoSID(sharedCoSTestTunnel, session),
		TunnelID: sharedCoSTestTunnel, SessionID: session, Interface: sharedCoSTestInterface,
		Profile: sharedCoSTestDifferent}
	peer := netip.MustParseAddr("192.0.2.91")
	first, err := h.prepareSessionChange(request, peer)
	if err != nil {
		t.Fatal(err)
	}
	f.failWrites = 2
	if err := h.applySessionChange(first.Token, peer); err == nil {
		t.Fatal("failed original apply and compensation were acknowledged")
	}
	access := h.accesses[sharedCoSTestInterface].sharedOwner()
	oldEpisode := access.episode
	if oldEpisode == nil {
		t.Fatal("actual cleanup failure retained no original report episode")
	}
	if err := h.rollbackSessionCleanup(); err != nil {
		t.Fatal(err)
	}
	f.requireMaps(f.shared)
	if warnings := report.Warnings(); len(warnings) != 0 {
		t.Fatalf("confirmed rollback retained its old warning: %+v", warnings)
	}
	second, err := h.prepareSessionChange(request, peer)
	if err != nil {
		t.Fatal(err)
	}
	f.failWrites = 2
	if err := h.applySessionChange(second.Token, peer); err == nil {
		t.Fatal("second original failure was acknowledged")
	}
	if access.episode == nil {
		t.Fatal("new failure retained no report episode")
	}
	if access.episode == oldEpisode {
		t.Fatal("new failure re-used a cleared diagnostic episode")
	}
	before := report.Warnings()
	if len(before) != 1 {
		t.Fatalf("new failure has %d warnings, want one", len(before))
	}
	if err := oldEpisode.Clear(); err != nil {
		t.Fatal(err)
	}
	if actual := report.Warnings(); !reflect.DeepEqual(actual, before) {
		t.Fatal("completed old recovery cleared or changed the new failure")
	}
	if err := h.rollbackSessionCleanup(); err != nil {
		t.Fatal(err)
	}
	f.requireMaps(f.shared)
}

// Goal: exhausted diagnostic custody refuses admission before its baseline
// read or write. Method: fill the actual store's declared bounded slots, then
// release those exact reservations and prove the original admission succeeds.
func TestCoSCleanupReservationPrecedesOriginalBaseline(t *testing.T) {
	f := newRegisteredCoSReportFixture(t)
	f.handler.captureTarget = captureModeledOriginalCoSTarget(f.handler)
	_, _, capacity := report.SourceWarnings(Name, "", "")
	if capacity <= 0 {
		t.Fatal("actual report store declared no positive capacity")
	}
	reservations := make([]*report.ResourceWarning, 0, capacity)
	t.Cleanup(func() {
		for _, owner := range reservations {
			if err := owner.Close(); err != nil {
				t.Errorf("close exact capacity fixture reservation: %v", err)
			}
		}
	})
	for index := range capacity {
		owner, err := report.NewResourceWarning("cos-capacity-fixture", fmt.Sprintf("owner-%d", index))
		if err != nil {
			t.Fatal(err)
		}
		reservations = append(reservations, owner)
	}
	const session = uint16(64103)
	f.up(session, sharedCoSTestFirst)
	if f.reads != 0 {
		t.Fatalf("original baseline read before warning reservation: %d reads", f.reads)
	}
	if f.writes != 0 {
		t.Fatalf("native map written before warning reservation: %d writes", f.writes)
	}
	if f.handler.accesses[sharedCoSTestInterface] != nil {
		t.Fatal("failed reservation admitted an original access owner")
	}
	for _, owner := range reservations {
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
	}
	f.up(session, sharedCoSTestFirst)
	if f.writes != 1 {
		t.Fatalf("released capacity did not admit one productive original policy: %d writes", f.writes)
	}
	f.requireMaps(f.shared)
	f.down(session)
	f.requireMaps(f.original)
}
