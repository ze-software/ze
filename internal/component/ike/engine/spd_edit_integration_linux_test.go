//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: spd_policy.go -- installSPDPolicies, whose write sequence this probes
// Related: fragment_boundary_integration_linux_test.go -- the plain sender and the inbound DISCARD counter

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// editDiscardPort is the port the edited discard entry names. No other probe uses it.
const editDiscardPort = 6100

// probingDP is the loaded backend with a probe after every SPD write, so a test can
// observe the packet disposition at each intermediate database state of a reconcile.
type probingDP struct {
	dataplane.Dataplane
	after func(what string)
}

func (d *probingDP) InstallPolicy(p dataplane.SPParams) error {
	err := d.Dataplane.InstallPolicy(p)
	d.after("install " + dirLabel(p.Dir) + " " + p.Src.String() + " -> " + p.Dst.String())
	return err
}

func (d *probingDP) RemovePolicyParams(p dataplane.SPParams) error {
	err := d.Dataplane.RemovePolicyParams(p)
	d.after("remove " + dirLabel(p.Dir) + " " + p.Src.String() + " -> " + p.Dst.String())
	return err
}

// VALIDATES: AC-14, an edited DISCARD entry holds. An inbound discard entry for one UDP
// port drops matching cleartext traffic after EVERY kernel write of an edit: one that
// keeps its selector and changes its order (replaced in place), and one that moves it
// to another selector that still covers the probe (installed before the old one goes).
// METHOD: in the two-namespace lab on the real kernel, a bypass floor entry at order
// 5000 passes all UDP from the peer, so a probe that meets no discard entry is
// DELIVERED (the positive control, observed first). The discard entry is then
// installed, and each edit is reconciled through installSPDPolicies on a backend that
// sends one probe after each write and requires XfrmInPolBlock to count it and the
// listener to receive nothing.
// PREVENTS: the D-1 reconcile order, which removed the old form before installing the
// new one, so between the two writes the traffic the operator discards crossed the
// boundary under the next entry down.
//
// RFC requirement: RFC4301-4.4.1-7 positive -- an inbound discard entry edited through the reconcile path, once to a new order on the same selector and once to a new selector still covering the probe, blocks a matching cleartext packet after every kernel write of each edit, while the same packet with no discard entry is delivered.
//
//ze:guest-root
func TestRFC4301DiscardHoldsAcrossAnEditedEntry(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	listener := sadSelListen(t, sadSelInnerLocal, editDiscardPort)
	plain := fragmentPlainSender(t, lab)
	log := slogutil.DiscardLogger()
	packet := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(editDiscardPort))

	floor := ipsec.SPDPolicy{
		Name: "floor", Action: dataplane.SPActionBypass, Order: 5000,
		Direction: ipsec.SPDDirIn, Protocol: sadSelProtoUDP,
		LocalPrefix: mustCIDRNet(t, "10.2.0.0/16"), LocalPort: ipsec.AnyPort(),
		RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"), RemotePort: ipsec.AnyPort(),
	}
	drop := ipsec.SPDPolicy{
		Name: "drop", Action: dataplane.SPActionDiscard, Order: 1000,
		Direction: ipsec.SPDDirIn, Protocol: sadSelProtoUDP,
		LocalPrefix:  mustCIDRNet(t, "10.2.0.0/16"),
		LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: editDiscardPort},
		RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"), RemotePort: ipsec.AnyPort(),
	}
	installed := map[string]ipsec.SPDPolicy{"floor": floor}
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), installed, log) })

	if err := installSPDPolicies(dataplane.Get(), nil, installed, log); err != nil {
		t.Fatalf("install the bypass floor: %v", err)
	}
	fragmentSend(t, plain, packet)
	if !sadSelDelivered(t, listener) {
		t.Fatal("positive control: with only the bypass floor installed the probe was not delivered, so a drop below proves nothing")
	}

	probeDropped := func(t *testing.T, stage string) {
		t.Helper()
		before := sadSelStat(t, "XfrmInPolBlock")
		fragmentSend(t, plain, packet)
		awaitForwardKernelCounter(t, stage, uint64(before+1),
			func(t *testing.T) uint64 { return uint64(sadSelStat(t, "XfrmInPolBlock")) })
		if sadSelDelivered(t, listener) {
			t.Fatalf("%s: the probe the discard entry names was delivered", stage)
		}
	}

	next := map[string]ipsec.SPDPolicy{"floor": floor, "drop": drop}
	if err := installSPDPolicies(dataplane.Get(), installed, next, log); err != nil {
		t.Fatalf("install the discard entry: %v", err)
	}
	installed = next
	probeDropped(t, "after the discard entry was installed")

	reorder := drop
	reorder.Order = 1200
	reselect := reorder
	reselect.LocalPrefix = mustCIDRNet(t, "10.2.0.0/24")
	for _, edit := range []struct {
		name  string
		entry ipsec.SPDPolicy
	}{
		{"new order, same selector", reorder},
		{"new selector", reselect},
	} {
		probe := &probingDP{Dataplane: dataplane.Get(), after: func(what string) {
			probeDropped(t, edit.name+", after "+what)
		}}
		next := map[string]ipsec.SPDPolicy{"floor": floor, "drop": edit.entry}
		if err := installSPDPolicies(probe, installed, next, log); err != nil {
			t.Fatalf("%s: reconcile the edit: %v", edit.name, err)
		}
		installed = next
		probeDropped(t, edit.name+", after the reconcile")
	}
	held, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("list the SPD: %v", err)
	}
	for i := range held {
		if held[i].Role == dataplane.PolicyRoleOperator && held[i].Action == dataplane.SPActionDiscard &&
			held[i].Dst != nil && held[i].Dst.String() == "10.2.0.0/16" {
			t.Fatalf("the discard entry's replaced selector 10.2.0.0/16 is still installed: %+v", held[i])
		}
	}
}
