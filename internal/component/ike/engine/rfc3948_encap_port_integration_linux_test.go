// VALIDATES: AC-1 against a real Linux XFRM stack. A Child SA whose IKE SA's peer
// authenticated from a NAT-translated port Y is installed in the kernel with
// XFRMA_ENCAP espinudp sport Y dport L on the inbound state and sport L dport Y on the
// outbound state, L being the port of the NAT-T socket the IKE SA sends from.
// PREVENTS: a unit test over SAParams agreeing with a model of the backend rather than
// with the kernel; the state read back over netlink is what decides where ESP goes.
//
// Needs root for the network namespace (delNetns skips without CLONE_NEWNET) and runs
// in its own process (delOwnProcess), as the sibling Child SA policy probe does.

//go:build integration && linux

package engine

import (
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestRFC3948InstalledStateCarriesTheIKEPeerPort(t *testing.T) {
	if !delOwnProcess(t) {
		return
	}
	delNetns(t)
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load the xfrm backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the xfrm backend: %v", err)
		}
	})
	dp := dataplane.Get()
	if dp == nil {
		t.Fatal("the xfrm backend loaded but Get returned nil")
	}

	sa := testSA()
	sa.NATDetected = true
	sa.floatToNATTPort()
	localPort := encapPath(t, sa, encapTestPeerPort)
	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	states, err := netlink.XfrmStateList(netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("xfrm state list: %v", err)
	}
	want := map[uint32][2]int{
		child.InboundSPI:  {int(encapTestPeerPort), int(localPort)},
		child.OutboundSPI: {int(localPort), int(encapTestPeerPort)},
	}
	seen := 0
	for i := range states {
		ports, ok := want[uint32(states[i].Spi)]
		if !ok {
			continue
		}
		seen++
		enc := states[i].Encap
		if enc == nil {
			t.Errorf("state SPI %#x carries no XFRMA_ENCAP", states[i].Spi)
			continue
		}
		if enc.Type != netlink.XFRM_ENCAP_ESPINUDP {
			t.Errorf("state SPI %#x encap type %v, want espinudp", states[i].Spi, enc.Type)
		}
		if [2]int{enc.SrcPort, enc.DstPort} != ports {
			t.Errorf("state SPI %#x encap (sport, dport) = (%d, %d), want %v", states[i].Spi, enc.SrcPort, enc.DstPort, ports)
		}
	}
	if seen != len(want) {
		t.Fatalf("the kernel holds %d of the Child SA's %d states", seen, len(want))
	}
}
