// Design: docs/research/vpp-deployment-reference.md -- VPP interface query and MAC operations
// Related: query.go -- inheritsParentMAC and fillParentMAC
// Related: apply_test.go -- dumpChannel supplies VPP replies

package ifacevpp

import (
	"testing"

	"go.fd.io/govpp/binapi/ethernet_types"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
)

// parentMACChannel answers interface dumps as VPP does for a physical port
// and its VLAN: the port carries its MAC, the sub-interface a zero
// l2_address (send_sw_interface_details fills it only for a physical
// Ethernet interface).
func parentMACChannel() *dumpChannel {
	return &dumpChannel{details: []interfaces.SwInterfaceDetails{
		{SwIfIndex: 1, SupSwIfIndex: 1, InterfaceName: asciiName("host-eth0"),
			L2Address: ethernet_types.MacAddress{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}},
		{SwIfIndex: 4, SupSwIfIndex: 1, Type: interface_types.IF_API_TYPE_SUB, SubOuterVlanID: 100,
			InterfaceName: asciiName("host-eth0.100")},
	}}
}

// TestVPPSubInterfaceReportsParentMAC proves every read of a VLAN
// sub-interface on VPP answers its parent's MAC, the one it sends with. A
// lab run stopped Ze at startup with "VPP access source MAC is absent" for
// host-eth0.100, because the zero VPP reports was passed through.
func TestVPPSubInterfaceReportsParentMAC(t *testing.T) {
	const want = "02:00:00:00:00:01"
	b := &vppBackendImpl{ch: parentMACChannel(), names: newNameMap()}
	if err := b.ensureChannel(); err != nil {
		t.Fatal(err)
	}
	link, err := b.GetLink("host-eth0.100")
	if err != nil {
		t.Fatal(err)
	}
	if link.MAC != want {
		t.Fatalf("GetLink MAC = %q, want the parent's %q", link.MAC, want)
	}
	info, err := b.GetInterface("host-eth0.100")
	if err != nil {
		t.Fatal(err)
	}
	if info.MAC != want {
		t.Fatalf("GetInterface MAC = %q, want the parent's %q", info.MAC, want)
	}
	list, err := b.ListInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range list {
		if entry.MAC != want {
			t.Fatalf("ListInterfaces %s MAC = %q, want %q", entry.Name, entry.MAC, want)
		}
	}
}

// TestVPPSubInterfaceWithoutParentRefusesRead proves a sub-interface whose
// parent VPP cannot report fails the read instead of answering a zero MAC.
func TestVPPSubInterfaceWithoutParentRefusesRead(t *testing.T) {
	ch := parentMACChannel()
	ch.details = ch.details[1:]
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if err := b.ensureChannel(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetLink("host-eth0.100"); err == nil {
		t.Fatal("a sub-interface with no readable parent answered a MAC")
	}
	if _, err := b.ListInterfaces(); err == nil {
		t.Fatal("a list holding a sub-interface with no readable parent succeeded")
	}
}
