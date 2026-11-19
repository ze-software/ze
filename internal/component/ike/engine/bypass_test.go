package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// bypassDP records both halves of the policy traffic, which mockDP (child_test.go)
// does not: it drops removals on the floor, and this file has to assert that what
// shutdown releases is exactly what start installed.
type bypassDP struct {
	installed  []dataplane.SPParams
	removed    []dataplane.SPParams
	installErr error
	live       map[spKey]dataplane.SPParams

	// removedPolicies records RemovePolicy (the three-argument form the Child SA
	// teardown uses). mockDP in child_test.go drops those on the floor, and whether
	// a policy removal HAPPENS is the whole subject of the rekey tests.
	removedPolicies []removedPolicy
	removedSAs      []uint32

	// guardOps records the application override guard calls in order ("attach",
	// "detach"), and guardErr refuses an attach with that error, so the apply's
	// refusal and its order against the catch-all can be asserted.
	guardOps []string
	guardErr error
}

func (d *bypassDP) AttachOverrideGuard() error {
	if d.guardErr != nil {
		return d.guardErr
	}
	d.guardOps = append(d.guardOps, "attach")
	return nil
}

func (d *bypassDP) DetachOverrideGuard() error {
	d.guardOps = append(d.guardOps, "detach")
	return nil
}

// OverrideGuardState answers from the last recorded call.
func (d *bypassDP) OverrideGuardState() (dataplane.OverrideGuardState, error) {
	if len(d.guardOps) > 0 && d.guardOps[len(d.guardOps)-1] == "attach" {
		return dataplane.OverrideGuardState{Status: dataplane.OverrideGuardAttached}, nil
	}
	return dataplane.OverrideGuardState{Status: dataplane.OverrideGuardDetached}, nil
}

type removedPolicy struct {
	src, dst string
	dir      dataplane.SADir
}

func (d *bypassDP) RequiresForwardPolicy() bool { return false }

func (d *bypassDP) InstallSA(_ dataplane.SAParams) error          { return nil }
func (d *bypassDP) ListSAs(_ uint32) ([]dataplane.SAInfo, error)  { return nil, nil }
func (d *bypassDP) ListPolicies() ([]dataplane.PolicyInfo, error) { return nil, nil }
func (d *bypassDP) Close() error                                  { return nil }

func (d *bypassDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	d.removedSAs = append(d.removedSAs, spi)
	return nil
}

func (d *bypassDP) RemovePolicy(src, dst *net.IPNet, dir dataplane.SADir) error {
	d.removedPolicies = append(d.removedPolicies, removedPolicy{
		src: src.String(), dst: dst.String(), dir: dir,
	})
	return nil
}

func (d *bypassDP) InstallPolicy(p dataplane.SPParams) error {
	if d.installErr != nil {
		return d.installErr
	}
	d.installed = append(d.installed, p)
	if d.live == nil {
		d.live = make(map[spKey]dataplane.SPParams)
	}
	d.live[keyOf(p)] = p
	return nil
}

func (d *bypassDP) RemovePolicyParams(p dataplane.SPParams) error {
	d.removed = append(d.removed, p)
	delete(d.live, keyOf(p))
	return nil
}

// spKey is the identity of a policy for set comparison: everything the kernel uses
// to tell one policy from another (vendor netlink selFromPolicy + Dir), and the if_id,
// which the kernel keys a policy by too (xfrm_policy_match): it names the SPD.
type spKey struct {
	src, dst         string
	dir              dataplane.SADir
	upperProto       uint8
	srcPort, dstPort dataplane.PortMatch
	ifIndex          int
	ifID             uint32
}

func keyOf(p dataplane.SPParams) spKey {
	return spKey{
		src:        p.Src.String(),
		dst:        p.Dst.String(),
		dir:        p.Dir,
		upperProto: p.UpperProto,
		srcPort:    p.SrcPort,
		dstPort:    p.DstPort,
		ifIndex:    p.IfIndex,
		ifID:       p.IfID,
	}
}

// VALIDATES: the Child SA policies the install emits carry PriorityChildSA, an
// explicit non-zero rank an operator order can be placed on either side of.
// PREVENTS: a Child SA policy at the implicit 0, which no operator order can beat.
func TestChildSAPoliciesCarryChildPriority(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 3, dp, log); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.policies) == 0 {
		t.Fatal("no policies installed; the sweep below would assert nothing")
	}
	for i, p := range dp.policies {
		if p.Action != dataplane.SPActionProtect {
			t.Errorf("child policy[%d] action = %d, want SPActionProtect", i, p.Action)
		}
		if p.Priority != dataplane.PriorityChildSA {
			t.Errorf("child policy[%d] priority = %d, want PriorityChildSA (%d)",
				i, p.Priority, dataplane.PriorityChildSA)
		}
	}
}
