// Design: docs/architecture/traffic/cos-dynamic.md -- retained original target
//go:build ze_l2tp

package cos

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// This fixture explicitly constructs a modeled original recipient BEFORE
// admission. It proves callback/state routing only, never kernel custody. The
// Linux questions use captureCreatorCoSTarget and the actual backend creator.
type modeledOriginalCoSTarget struct {
	name          string
	read          func(string) (*iface.InterfaceInfo, error)
	update        updateQoSFunc
	closed        bool
	finishFailure error
}

func (target *modeledOriginalCoSTarget) readLink(name string) (*iface.InterfaceInfo, error) {
	if target.closed {
		return nil, errors.New("modeled original resource is closed")
	}
	if name != target.name {
		return nil, errors.New("modeled original resource names another access")
	}
	return target.read(name)
}

func (target *modeledOriginalCoSTarget) write(name string, ingress, egress map[uint32]uint32) error {
	if target.closed {
		return errors.New("modeled original resource is closed")
	}
	if name != target.name {
		return errors.New("modeled original resource names another access")
	}
	return target.update(name, ingress, egress)
}

func (*modeledOriginalCoSTarget) allowRecovery() error { return nil }
func (*modeledOriginalCoSTarget) stopOwner()           {}
func (target *modeledOriginalCoSTarget) finish() error {
	if target.finishFailure != nil {
		return target.finishFailure
	}
	target.closed = true
	return nil
}

func captureModeledOriginalCoSTarget(h *cosHandler) originalCoSTargetFactory {
	return func(name string) (originalCoSTarget, error) {
		return &modeledOriginalCoSTarget{name: name, read: h.readLink, update: h.updateQoS}, nil
	}
}

// Goal: NEW operator retry never replays a name-only writer. The warning and
// original departure remain, even if the replacement would accept the maps.
// MUTATION: omit allowRecovery, or clear the episode after the refused retry.
func TestCoSOriginalTargetUnsupportedRecoveryRetainsWarning(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.handler.captureTarget = nil
	if iface.GetBackend() != nil {
		t.Fatal("name-only refusal fixture has an unrelated selected backend")
	}
	var clears int
	var fix string
	f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
		if input.Action == rpc.ReportClear {
			clears++
		} else if input.Action == rpc.ReportRaise {
			fix = input.Fix
		}
		return nil
	}
	const session = uint16(63001)
	f.up(session, sharedCoSTestFirst)
	f.failWrites = 1
	f.down(session)
	access := f.handler.accesses[sharedCoSTestInterface].sharedOwner()
	before := f.writes
	result, err := f.handler.retrySharedCleanup()
	var unavailable *originalCoSTargetUnavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("unowned retry did not report original-recipient refusal: %v", err)
	}
	if result.Attempted != 1 || result.Recovered != 0 {
		t.Fatalf("refused retry counts: %+v", result)
	}
	if f.writes != before || clears != 0 {
		t.Fatalf("refusal wrote or cleared an unknown episode: writes=%d/%d clears=%d", f.writes, before, clears)
	}
	if fix != originalCoSTargetFix {
		t.Fatalf("refusal did not publish the exact original-owner recovery fix: %q", fix)
	}
	if f.handler.accesses[sharedCoSTestInterface].sharedOwner() != access {
		t.Fatal("refusal discarded original custody")
	}
	if _, retained := access.departures[fixtureCoSKey(sharedCoSTestTunnel, session)]; !retained {
		t.Fatal("refusal discarded the original departing session")
	}
	if _, retained := access.warning.(sharedCoSWarningAttempted); !retained {
		t.Fatal("refusal cleared the original unknown warning")
	}
}

// Goal: stopped-runner transfer retains an explicit unsupported variant; its
// old callbacks cannot run after transfer, even if they themselves resolve a
// newly selected recipient. The replacement handler supplies no new authority.
func TestCoSOriginalTargetTransferRefusesUnownedCallbacks(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	f.handler.captureTarget = nil
	if iface.GetBackend() != nil {
		t.Fatal("name-only transfer fixture has an unrelated backend")
	}
	const session = uint16(63002)
	f.up(session, sharedCoSTestFirst)
	access := f.handler.accesses[sharedCoSTestInterface].sharedOwner()
	before := f.writes
	replacement := newCosHandlerFromStopped(newTestBus(), func(string, map[uint32]uint32, map[uint32]uint32) error {
		t.Fatal("transferred access used the replacement writer")
		return nil
	}, discardCoSReportForTest, f.handler.stopOwners())
	t.Cleanup(replacement.stop)
	err := replacement.departSharedSession(fixtureCoSKey(sharedCoSTestTunnel, session))
	var unavailable *originalCoSTargetUnavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("transferred unowned departure did not refuse: %v", err)
	}
	if f.writes != before || replacement.accesses[sharedCoSTestInterface].sharedOwner() != access {
		t.Fatal("transfer rewrote a recipient or lost the original owner")
	}
}

// Goal: an unknown first Snapshot keeps the already captured target without
// a guessed baseline. Only joined release can finish that cleanup episode.
func TestCoSOriginalTargetFirstSnapshotRetainsCustodyUntilRelease(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	target := &modeledOriginalCoSTarget{name: sharedCoSTestInterface,
		read: func(string) (*iface.InterfaceInfo, error) {
			return nil, errors.New("original Snapshot return withheld")
		},
		update: f.handler.updateQoS, finishFailure: errors.New("original native operation has not joined")}
	f.handler.captureTarget = func(string) (originalCoSTarget, error) { return target, nil }
	if _, err := f.handler.captureSharedAccess(sharedCoSTestInterface); err == nil {
		t.Fatal("unknown Snapshot admitted a baseline")
	}
	state := f.handler.accesses[sharedCoSTestInterface]
	if _, retained := state.(*sharedCoSCapturePending); !retained || len(state.sharedOwner().users) != 0 {
		t.Fatal("unknown Snapshot lost custody or admitted a subscriber")
	}
	if result, err := f.handler.retrySharedCleanup(); err == nil || result.Recovered != 0 || target.closed {
		t.Fatalf("unknown operation was released or counted recovered: %+v %v", result, err)
	}
	target.finishFailure = nil
	if result, err := f.handler.retrySharedCleanup(); err != nil || result.Recovered != 1 {
		t.Fatalf("joined original Snapshot did not release: %+v %v", result, err)
	}
	if !target.closed || f.handler.accesses[sharedCoSTestInterface] != nil || f.writes != 0 {
		t.Fatal("release did not end custody, or wrote a nonexistent baseline")
	}
}

// Goal: after the final original restore, a failed join/release retries no
// map write. Warning clearance follows the original release confirmation.
func TestCoSOriginalTargetReleaseRetryDoesNotRepeatRestore(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	var target *modeledOriginalCoSTarget
	f.handler.captureTarget = func(name string) (originalCoSTarget, error) {
		target = &modeledOriginalCoSTarget{name: name, read: f.handler.readLink, update: f.handler.updateQoS}
		return target, nil
	}
	const session = uint16(63003)
	f.up(session, sharedCoSTestFirst)
	target.finishFailure = errors.New("original release is unconfirmed")
	f.down(session)
	if _, retained := f.handler.accesses[sharedCoSTestInterface].(*sharedCoSReleasePending); !retained {
		t.Fatal("unconfirmed release lost the original lease")
	}
	f.requireMaps(f.original)
	before := f.writes
	target.finishFailure = nil
	if result, err := f.handler.retrySharedCleanup(); err != nil || result.Recovered != 1 {
		t.Fatalf("original release did not finish: %+v %v", result, err)
	}
	if f.writes != before || !target.closed || f.handler.accesses[sharedCoSTestInterface] != nil {
		t.Fatal("release retry repeated restore or retained completed custody")
	}
}

// Goal: once joining/releasing succeeds, a failed CLEAR remains report-only.
// It is never raised again as a native failure or used to repeat a map write.
func TestCoSOriginalTargetReleaseClearFailureStaysReportOnly(t *testing.T) {
	f := newSharedCoSTestOwner(t)
	target := &modeledOriginalCoSTarget{name: sharedCoSTestInterface,
		read: func(string) (*iface.InterfaceInfo, error) {
			return nil, errors.New("original Snapshot return withheld")
		},
		update: f.handler.updateQoS}
	f.handler.captureTarget = func(string) (originalCoSTarget, error) { return target, nil }
	clearFailure := errors.New("original warning CLEAR was not acknowledged")
	var raises, clears int
	f.handler.reportIssue = func(input rpc.ReportIssueInput) error {
		if input.Action == rpc.ReportRaise {
			raises++
		}
		if input.Action == rpc.ReportClear {
			clears++
			return clearFailure
		}
		return nil
	}
	if _, err := f.handler.captureSharedAccess(sharedCoSTestInterface); err == nil {
		t.Fatal("unknown Snapshot admitted a baseline")
	}
	if result, err := f.handler.retrySharedCleanup(); !errors.Is(err, clearFailure) || result.Recovered != 0 {
		t.Fatalf("failed original CLEAR was acknowledged: %+v %v", result, err)
	}
	state := f.handler.accesses[sharedCoSTestInterface]
	if _, reportOnly := state.(*sharedCoSReportPending); !reportOnly || !target.closed {
		t.Fatal("confirmed release plus failed CLEAR did not retain report-only state")
	}
	if _, pending := state.sharedOwner().warning.(*sharedCoSWarningClearPending); !pending || raises != 1 || clears != 1 {
		t.Fatalf("failed CLEAR was relabeled or republished as native failure: raises=%d clears=%d", raises, clears)
	}
	clearFailure = nil
	if result, err := f.handler.retrySharedCleanup(); err != nil || result.Recovered != 1 {
		t.Fatalf("report-only retry did not finish: %+v %v", result, err)
	}
	if f.writes != 0 || raises != 1 || clears != 2 || f.handler.accesses[sharedCoSTestInterface] != nil {
		t.Fatal("report-only retry repeated native work or retained completed warning")
	}
}
