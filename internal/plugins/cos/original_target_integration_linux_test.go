// Design: docs/architecture/traffic/cos-dynamic.md -- actual original recipient
//go:build integration && linux && ze_l2tp

package cos

import (
	"errors"
	"maps"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/test/userns"
)

// Observation wraps the genuine creator resource. It never replaces native
// read/write with a name-based callback or manufactures a native lease.
type observedCreatorCoSTarget struct {
	originalCoSTarget
	writes         *int
	failAfterWrite error
}

func (target *observedCreatorCoSTarget) write(name string, ingress, egress map[uint32]uint32) error {
	(*target.writes)++
	if err := target.originalCoSTarget.write(name, ingress, egress); err != nil {
		return err
	}
	return target.failAfterWrite
}

func observeCreatorCoSTargets(f *nativeCoSSharedFixture) originalCoSTargetFactory {
	return func(name string) (originalCoSTarget, error) {
		target, err := captureCreatorCoSTarget(name)
		if err != nil {
			return nil, err
		}
		return &observedCreatorCoSTarget{originalCoSTarget: target, writes: &f.writes}, nil
	}
}

// Test teardown joins issued operations and releases this fixture's leases
// before its backend and isolated namespace disappear. This is not a product
// cleanup success or a warning-clear assertion; those are tested above teardown.
func closeNativeCoSTestTargets(t *testing.T, h *cosHandler) {
	t.Helper()
	h.stop()
	for _, state := range h.accesses {
		if err := state.sharedOwner().target.finish(); err != nil {
			t.Errorf("join and release fixture's original native target: %v", err)
		}
	}
}

// Goal: actual original cleanup can retry and restore the real kernel maps.
// MUTATION: refusing all recovery, or bypassing the original lease, fails this
// positive control. The existing survivor/CoA questions keep all assertions.
func TestCoSOriginalTargetNativeCleanupRestoresOriginalKernelMaps(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeCoSSharedFixture(t)
	f.up(63011, "shared-first")
	access := f.handler.accesses[f.device].sharedOwner()
	target, genuine := access.target.(*observedCreatorCoSTarget)
	if !genuine {
		t.Fatalf("native admission retained %T instead of the genuine creator target", access.target)
	}
	target.failAfterWrite = errors.New("original restore acknowledgement lost")
	emitSessionDown(t, f.bus, &l2tpevents.SessionDownPayload{TunnelID: 31, SessionID: 63011})
	if _, retained := f.handler.accesses[f.device].(*sharedCoSRestore); !retained {
		t.Fatal("unconfirmed original restore did not retain its original recipient")
	}
	target.failAfterWrite = nil
	result, err := f.handler.retrySharedCleanup()
	if err != nil || result.Attempted != 1 || result.Recovered != 1 {
		t.Fatalf("genuine original cleanup did not recover: %+v %v", result, err)
	}
	f.requireMaps(f.original)
	if f.handler.accesses[f.device] != nil {
		t.Fatal("confirmed final restore and release retained native custody")
	}
}

// Goal: the old baseline cannot reach an externally replaced kernel device.
// This deliberately bypasses Ze's delete guard to simulate external netlink
// replacement. The retained native lease checks its original creator resource.
func TestCoSOriginalTargetNativeReplacementRetainsUnknown(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeCoSSharedFixture(t)
	f.up(63012, "shared-first")
	access := f.handler.accesses[f.device].sharedOwner()
	original, err := netlink.LinkByName(f.device)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.backend.DeleteInterface(f.device); err == nil {
		t.Fatal("Ze deleted the genuinely leased original access")
	}
	if err := iface.CloseBackend(); err == nil || iface.GetBackend() != f.backend {
		t.Fatal("backend close discarded the genuinely leased original owner")
	}
	if err := netlink.LinkDel(original); err != nil {
		t.Fatal(err)
	}
	parent, err := netlink.LinkByName("cosshare0")
	if err != nil {
		t.Fatal(err)
	}
	replacementMaps := coreCos.Profile{IngressMap: map[uint32]uint32{7: 6}, EgressMap: map[uint32]uint32{6: 7}}
	replacement := &netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: f.device, ParentIndex: parent.Attrs().Index}, VlanId: 317,
		IngressQosMap: maps.Clone(replacementMaps.IngressMap), EgressQosMap: maps.Clone(replacementMaps.EgressMap)}
	if err := netlink.LinkAdd(replacement); err != nil {
		t.Fatal(err)
	}
	emitSessionDown(t, f.bus, &l2tpevents.SessionDownPayload{TunnelID: 31, SessionID: 63012})
	result, err := f.handler.retrySharedCleanup()
	if err == nil || result.Recovered != 0 {
		t.Fatalf("replacement confirmed another resource's cleanup: %+v %v", result, err)
	}
	f.requireMaps(replacementMaps)
	if f.handler.accesses[f.device].sharedOwner() != access {
		t.Fatal("replacement discarded the original owner and warning")
	}
	if _, retained := access.warning.(sharedCoSWarningAttempted); !retained {
		t.Fatal("replacement cleared the original unconfirmed warning")
	}
}
