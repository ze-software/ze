// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- receive path and the VRID check
// Related: engine.go -- dispatchRx, which routes a datagram to the instance that received it
// Related: rx_discard_test.go -- rxCheckItem, the advertisement builder reused here
package vrrp

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// VALIDATES: the VRID check is scoped to the interface that received the
// advertisement.
//
// TestRxVRIDCheckedOnTheReceivingInterface checks that an advertisement is
// judged against the groups of the interface it arrived on, not against every
// group of the router. The transport stamps each datagram with the key of the
// socket that received it, so the key names the receiving interface.
//
// Method: one VRRPv2 group on eth0 with VRID 10 and one on eth1 with VRID 20.
// A VRID 10 advertisement received on eth0 reaches eth0's FSM. The same bytes
// received on eth1, where VRID 10 is not configured, are discarded with the
// vrid reason, and neither eth1's nor eth0's FSM sees them.
//
// RFC requirement: RFC3768-7.1-4 positive -- a VRRPv2 advertisement whose VRID is configured on the interface that received it records no receive error and reaches that interface's FSM as AdvertReceived (dispatchRx engine.go, lookup instance.go).
// RFC requirement: RFC3768-7.1-4 negative -- the same advertisement received on another interface, where its VRID is not configured, is discarded with the vrid reason, and neither the receiving interface's FSM nor the FSM of the interface that configures the VRID sees it (dispatchRx engine.go, lookup instance.go).
func TestRxVRIDCheckedOnTheReceivingInterface(t *testing.T) {
	spec0 := testSpec()
	spec0.Version = versionV2
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
			t.Fatalf("an advertisement received on eth1 for eth0's VRID reached %s's FSM as %T", name, ev)
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
		t.Fatal("an advertisement for eth0's VRID received on eth0 never reached its FSM")
	}
}
