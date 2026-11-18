// Design: docs/architecture/traffic/cos-dynamic.md -- original-target readback
// Related: original_target.go -- creator lease and exact effective maps
//go:build ze_l2tp

package cos

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/report"
)

// This models the retained lease boundary, never a kernel custody receipt.
// Actual original/replacement kernel questions remain in the Linux carrier.
type coSReadbackLease struct {
	snapshot   iface.NativeVLANMaps
	readErr    error
	replaceErr error
	reads      int
	writes     int
	joins      int
	closes     int
	onReplace  func(iface.NativeVLANMaps)
}

func (*coSReadbackLease) Identity() (iface.NativeVLANIdentity, error) {
	return iface.NativeVLANIdentity{Device: sharedCoSTestInterface, Index: 317}, nil
}

func (lease *coSReadbackLease) Borrow(context.Context) (iface.NativeVLANLease, error) {
	return lease, nil
}

func (lease *coSReadbackLease) Snapshot(context.Context) (iface.NativeVLANMaps, error) {
	lease.reads++
	return iface.NativeVLANMaps{Ingress: maps.Clone(lease.snapshot.Ingress), Egress: maps.Clone(lease.snapshot.Egress)}, lease.readErr
}

func (lease *coSReadbackLease) Replace(_ context.Context, requested iface.NativeVLANMaps) error {
	lease.writes++
	if lease.onReplace != nil {
		lease.onReplace(requested)
	}
	return lease.replaceErr
}

func (lease *coSReadbackLease) Close() error {
	lease.closes++
	return nil
}

func (lease *coSReadbackLease) JoinNativeVLAN(context.Context) error {
	lease.joins++
	return nil
}

// Goal: an ACK without exact original readback cannot confirm restoration.
// Method: keep the same lease, vary only its actual returned maps/read error,
// and require the original matching effective maps as a productive control.
func TestCoSOriginalTargetRequiresExactPostWriteReadback(t *testing.T) {
	readFailure := errors.New("original map readback failed")
	desired := iface.NativeVLANMaps{Ingress: map[uint32]uint32{2: 5}, Egress: map[uint32]uint32{5: 2}}
	for _, test := range []struct {
		name        string
		actual      iface.NativeVLANMaps
		failure     error
		wantSuccess bool
		requested   *iface.NativeVLANMaps
	}{
		{name: "different-ingress", actual: iface.NativeVLANMaps{Ingress: map[uint32]uint32{2: 4}, Egress: desired.Egress}},
		{name: "different-egress", actual: iface.NativeVLANMaps{Ingress: desired.Ingress, Egress: map[uint32]uint32{5: 3}}},
		{name: "original-read-error", actual: desired, failure: readFailure},
		{name: "exact-original", actual: desired, wantSuccess: true},
		{name: "equivalent-defaults", actual: iface.NativeVLANMaps{Ingress: map[uint32]uint32{0: 0, 2: 5}, Egress: map[uint32]uint32{0: 0, 5: 2}}, wantSuccess: true},
		{name: "equivalent-nonzero-key-zero-defaults", actual: iface.NativeVLANMaps{Ingress: map[uint32]uint32{2: 5, 6: 0}, Egress: map[uint32]uint32{5: 2, 6: 0}}, wantSuccess: true},
		{name: "nil-ingress-readback-zero-default", requested: &iface.NativeVLANMaps{Ingress: map[uint32]uint32{6: 0}, Egress: desired.Egress}, actual: iface.NativeVLANMaps{Egress: desired.Egress}, wantSuccess: true},
		{name: "nil-ingress-readback-nonzero-mismatch", requested: &iface.NativeVLANMaps{Ingress: map[uint32]uint32{6: 6}, Egress: desired.Egress}, actual: iface.NativeVLANMaps{Egress: desired.Egress}},
		{name: "nil-egress-readback-zero-default", requested: &iface.NativeVLANMaps{Ingress: desired.Ingress, Egress: map[uint32]uint32{6: 0}}, actual: iface.NativeVLANMaps{Ingress: desired.Ingress}, wantSuccess: true},
		{name: "nil-egress-readback-nonzero-mismatch", requested: &iface.NativeVLANMaps{Ingress: desired.Ingress, Egress: map[uint32]uint32{6: 6}}, actual: iface.NativeVLANMaps{Ingress: desired.Ingress}},
	} {
		t.Run(test.name, func(t *testing.T) {
			lease := &coSReadbackLease{snapshot: test.actual, readErr: test.failure}
			target := &creatorCoSTarget{name: sharedCoSTestInterface, lease: lease, join: lease}
			requested := desired
			if test.requested != nil {
				requested = *test.requested
			}
			err := target.write(sharedCoSTestInterface, requested.Ingress, requested.Egress)
			if test.wantSuccess {
				if err != nil {
					t.Fatalf("matching original readback refused: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("native ACK without confirmed original maps admitted cleanup")
				}
			}
			if test.failure != nil {
				if !errors.Is(err, test.failure) {
					t.Fatalf("original readback cause lost: %v", err)
				}
			}
			if lease.writes != 1 {
				t.Fatalf("original native writes=%d, want one", lease.writes)
			}
			if lease.reads != 1 {
				t.Fatalf("original post-write reads=%d, want one", lease.reads)
			}
			if lease.joins != 1 {
				t.Fatalf("original operation joins=%d, want one", lease.joins)
			}
		})
	}
}

// Goal: the real owner entrypoint cannot clear custody on a native ACK alone.
// Method: the registered handler retains a creator target over a deliberately
// modeled lease, rejects wrong/failed readback on Down, then the same target
// recovers through the actual registered cleanup handler. No kernel custody is
// claimed: separate Linux questions retain genuine creator/lease requirements.
func TestCoSRegisteredCleanupRequiresOriginalReadback(t *testing.T) {
	for _, mode := range []string{"different-original-maps", "original-read-failure"} {
		t.Run(mode, func(t *testing.T) {
			f := newRegisteredCoSReportFixture(t)
			lease := &coSReadbackLease{snapshot: iface.NativeVLANMaps{
				Ingress: maps.Clone(f.original.IngressMap), Egress: maps.Clone(f.original.EgressMap)}}
			install := func(requested iface.NativeVLANMaps) {
				lease.snapshot = iface.NativeVLANMaps{Ingress: maps.Clone(requested.Ingress), Egress: maps.Clone(requested.Egress)}
			}
			lease.onReplace = install
			target := &creatorCoSTarget{name: sharedCoSTestInterface,
				identity: iface.NativeVLANIdentity{Device: sharedCoSTestInterface, Index: 317}, lease: lease, join: lease}
			f.handler.captureTarget = func(string) (originalCoSTarget, error) { return target, nil }
			const session = uint16(64105)
			f.up(session, sharedCoSTestFirst)
			if lease.writes != 1 {
				t.Fatal("productive initial policy did not reach the retained creator target")
			}
			original := f.handler.accesses[sharedCoSTestInterface].sharedOwner()
			readFailure := errors.New("original registered readback failed")
			if mode == "different-original-maps" {
				lease.onReplace = func(iface.NativeVLANMaps) {}
			} else {
				lease.readErr = readFailure
			}
			f.down(session)
			state := f.handler.accesses[sharedCoSTestInterface]
			if state == nil {
				t.Fatal("registered Down released custody without original map readback")
			}
			if state.sharedOwner() != original {
				t.Fatal("readback failure reselected another original owner")
			}
			if _, retained := state.(*sharedCoSRestore); !retained {
				t.Fatal("unconfirmed original maps became a report-only completion")
			}
			if lease.closes != 0 {
				t.Fatal("unconfirmed original maps released their native lease")
			}
			if warnings := report.Warnings(); len(warnings) != 1 {
				t.Fatalf("unconfirmed original maps lost their resource warning: %+v", warnings)
			}
			lease.readErr = nil
			lease.onReplace = install
			before := lease.writes
			status, _, err := executeCoSCleanup(nil, "")
			if err != nil {
				t.Fatal(err)
			}
			if status != "done" {
				t.Fatalf("matching original cleanup status=%q", status)
			}
			if lease.writes != before+1 {
				t.Fatal("matching original cleanup did not make one final restore")
			}
			if lease.closes != 1 {
				t.Fatal("matching original cleanup did not release its original lease exactly once")
			}
			if f.handler.accesses[sharedCoSTestInterface] != nil {
				t.Fatal("matching original readback retained finished custody")
			}
			if warnings := report.Warnings(); len(warnings) != 0 {
				t.Fatalf("matching original readback retained its warning: %+v", warnings)
			}
		})
	}
}
