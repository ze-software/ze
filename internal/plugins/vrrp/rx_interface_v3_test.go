// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- receive path and the VRID check
// Related: engine.go -- dispatchRx, which routes a datagram to the instance that received it
// Related: rx_interface_test.go -- the VRRPv2 counterpart
package vrrp

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// VALIDATES: the VRRPv3 VRID check is scoped to the interface that received
// the advertisement, through the production lookup.
//
// TestRxV3VRIDCheckedOnTheReceivingInterface is the VRRPv3 run of the check
// TestRxVRIDCheckedOnTheReceivingInterface makes for VRRPv2. The decode uses
// the receiving instance's own lookup (instance.go), not a lookup the test
// supplies, so a lookup that accepted any VRID reddens it.
//
// Method: one VRRPv3 group on eth0 with VRID 10 and one on eth1 with VRID 20.
// The same VRID 10 advertisement is received first on eth1, where it must be
// discarded with the vrid reason and reach neither FSM, then on eth0, where it
// must reach eth0's FSM as AdvertReceived with no receive error.
//
// RFC requirement: RFC9568-7.1-4 positive -- a VRRPv3 advertisement whose VRID is configured on the interface that received it records no receive error and reaches that interface's FSM as AdvertReceived (dispatchRx engine.go, lookup instance.go).
// RFC requirement: RFC9568-7.1-4 negative -- the same VRRPv3 advertisement received on another interface, where its VRID is not configured, is discarded with the vrid reason, and neither the receiving interface's FSM nor the FSM of the interface that configures the VRID sees it (dispatchRx engine.go, lookup instance.go).
func TestRxV3VRIDCheckedOnTheReceivingInterface(t *testing.T) {
	spec0 := testSpec()
	if spec0.Version != versionV3 {
		t.Fatalf("testSpec version = %v, want VRRPv3", spec0.Version)
	}
	spec1 := spec0
	spec1.Interface = "eth1"
	spec1.ParentDevice = "eth1"
	spec1.VRID = 20

	in0, f0, _ := newTestInstance(t, spec0)
	in1, f1, _ := newTestInstance(t, spec1)
	in0.dispatch(fsm.Startup{Config: in0.fsmConfig()})
	in1.dispatch(fsm.Startup{Config: in1.fsmConfig()})
	eng := &engine{instances: map[string]*instance{"eth0": in0, "eth1": in1}}

	onEth1 := rxCheckItem(spec0, nil)
	onEth1.Key = in1.key
	eng.dispatchRx(onEth1)
	if got := f1.snapshot().rxErrors; len(got) != 1 || got[0] != "vrid" {
		t.Fatalf("eth1 rx errors = %v, want exactly [vrid] for a VRID it does not configure", got)
	}
	for name, in := range map[string]*instance{"eth1": in1, "eth0": in0} {
		select {
		case ev := <-in.events:
			t.Fatalf("a VRRPv3 advertisement received on eth1 for eth0's VRID reached %s's FSM as %T", name, ev)
		default:
		}
	}
	if got := f0.snapshot().rxErrors; len(got) != 0 {
		t.Fatalf("eth0 recorded %v for a datagram it did not receive", got)
	}

	onEth0 := rxCheckItem(spec0, nil)
	onEth0.Key = in0.key
	eng.dispatchRx(onEth0)
	if got := f0.snapshot().rxErrors; len(got) != 0 {
		t.Fatalf("eth0 rx errors = %v, want none for its own VRID", got)
	}
	select {
	case ev := <-in0.events:
		if _, ok := ev.(fsm.AdvertReceived); !ok {
			t.Fatalf("eth0 event = %T, want fsm.AdvertReceived", ev)
		}
	default:
		t.Fatal("a VRRPv3 advertisement for eth0's VRID received on eth0 never reached its FSM")
	}
}
