//go:build linux

// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, the VRRP owner table beside an operator table
package vrrp

import (
	"net"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft" // registers the nft firewall backend
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/test/userns"
)

// VALIDATES: AC-16 of spec-firewall-existing-family-and-owner-correctness --
// an operator arp table whose output chain runs first and ACCEPTS the
// parent's ARP replies from the VRRP-owned address does not override the
// VRRP owner filter: the owner table's drop still removes the parent's
// physical-MAC reply, because a drop in any base chain at a hook is final and
// an accept ends only its own chain.
//
// Method: the operator table is registered through RegisterTables under the
// firewall engine's owner, as registerConfigTables does for a config table,
// at priority -10 so it sees each reply before the owner table at 0. The
// control phase, with the operator table alone, proves the capture sees the
// physical MAC when nothing drops it and that the operator table installs.
// With the owner filter added, the LAN host learns only the virtual MAC.
func TestVRRPOwnerSurvivesOperatorARPTable(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	w := newGatewayWire(t)
	if err := firewall.LoadBackend("nft"); err != nil {
		t.Skipf("nft firewall backend unavailable: %v", err)
	}
	t.Cleanup(func() { _ = firewall.CloseBackend() })

	parent := w.parent.Attrs().Name
	v4 := netip.MustParseAddr("192.0.2.254") // the parent's own address: the owner case
	vmac := packet.VirtualMAC(packet.V4, 10)
	if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
		Name: "zp4", Parent: parent, MAC: macString(vmac),
		Mode: iface.MacvlanModePrivate, Alias: "ze:owned:operator-arp-proof",
	}); err != nil {
		t.Fatal(err)
	}
	if err := applyDataplaneSysctls(parent, "zp4", familyIPv4); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { revertDataplaneSysctls(parent, "zp4", familyIPv4) })
	if err := w.backend.AddAddress("zp4", "192.0.2.254/32"); err != nil {
		t.Fatal(err)
	}

	physical := w.parent.Attrs().HardwareAddr
	virtual := net.HardwareAddr(vmac[:])
	arpFD := ownerSocket(t, w.peer, unix.ETH_P_ARP)

	operator := firewall.Table{
		Name:   "ze_lan_arp",
		Family: firewall.FamilyARP,
		Chains: []firewall.Chain{{
			Name:     "output",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookOutput,
			Priority: -10,
			Policy:   firewall.PolicyAccept,
			Terms: []firewall.Term{{
				Name: "accept-owned-replies",
				Matches: []firewall.Match{
					firewall.MatchARPOperation{Operation: firewall.ARPOperationReply},
					firewall.MatchARPSenderAddress{Addr: v4},
				},
				Actions: []firewall.Action{firewall.Accept{}},
			}},
		}},
	}
	if err := firewall.ValidateTables([]firewall.Table{operator}); err != nil {
		t.Fatalf("operator table refused at verify: %v", err)
	}
	if err := firewall.RegisterTables("firewall", []firewall.Table{operator}); err != nil {
		t.Fatalf("register operator table: %v", err)
	}
	t.Cleanup(func() {
		_ = firewall.RegisterTables("firewall", nil)
		_ = firewall.ApplyAll()
	})
	if err := firewall.ApplyAll(); err != nil {
		t.Fatalf("apply operator table: %v", err)
	}

	// Control: the operator accept alone, so the parent answers with its
	// physical MAC and the capture sees it.
	ownerExpect(t, "operator-only ARP", ownerResolveARP(t, w.peer, arpFD, v4), physical, virtual, true)

	if err := setOwnerFilter("vrrp:zp4", parent, []netip.Addr{v4}); err != nil {
		t.Fatalf("apply owner filter: %v", err)
	}
	t.Cleanup(func() { _ = clearOwnerFilter("vrrp:zp4") })

	// The operator chain accepts the reply first; the owner chain still drops it.
	ownerExpect(t, "operator and owner ARP", ownerResolveARP(t, w.peer, arpFD, v4), physical, virtual, false)
}
