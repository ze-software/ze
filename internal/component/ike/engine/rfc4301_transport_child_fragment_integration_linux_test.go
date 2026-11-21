//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- transport fragment boundary.
// Related: rfc4301_transport_output_fragment_integration_linux_test.go -- the raw IPv4 sender, packet builders and the legacy-state proof of the kernel check.
// Related: child.go -- installChildSA, whose transport-mode install this test drives.
// RFC naming: untagged -- the tag waits for a recorded red (`./le rfc record`).

package engine

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// transportChildFragmentID is the IPv4 identification of the plaintext fragment pair.
const transportChildFragmentID = 0x4306

// TestRFC4301TransportChildSANeverAppliesESPToAnIPv4Fragment proves the transport-mode
// half of RFC 4301 Section 4.1 on what Ze itself installs. The two tests of
// rfc4301_transport_output_fragment_integration_linux_test.go hand the backend a legacy
// state with a nil Scope, so they prove the kernel check and not the Child SA Ze builds.
//
// RFC 4301 Section 4.1: "IKE creates pairs of SAs, so for simplicity, we choose to
// require that both SAs in a pair be of the same mode, transport or tunnel."
// RFC 4301 Section 4.1: "Note: AH and ESP cannot be applied using transport mode to IPv4
// packets that are fragments."
//
// Method: createFirstChildSA, and through it installChildSA, installs a transport-mode
// Child SA for every protocol and port between Ze's and the peer's outer addresses on the
// real XFRM backend in Ze's namespace. The kernel's two states and the Child SA's policies
// are read back as transport mode, with no forward policy. A whole UDP datagram from a raw
// IPv4 socket must then advance the outbound SA by exactly one packet with
// XfrmOutStateModeError unchanged. The two plaintext fragments of a second datagram must
// not each be protected: either both fail at the output transform
// (XfrmOutStateModeError +2, the outbound SA unchanged), or the kernel gathered them first
// and the SA protected one whole datagram (the SA +1, XfrmOutStateModeError unchanged).
// A whole datagram after them must still be protected, so the SA was live throughout.
//
//ze:guest-root
func TestRFC4301TransportChildSANeverAppliesESPToAnIPv4Fragment(t *testing.T) {
	lab := newEspPolLab(t)
	lab.enter(t, lab.ze)
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		lab.enter(t, lab.ze)
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the transport Child SA backend: %v", err)
		}
	})
	child := transportChildInstall(t, lab)
	transportChildWantModes(t, child)

	source, target := net.ParseIP(espPolZeOuter), net.ParseIP(espPolPeerOuter)
	fd := transportOutputSender(t, source)
	whole := func(label string, want uint64) {
		t.Helper()
		before := espPolStat(t, "XfrmOutStateModeError")
		udp := transportOutputUDP(source, []byte("transport child whole datagram "+label))
		transportOutputSend(t, fd, target, transportOutputIPv4(source, target, udp, 0, 0), false)
		if got := transportChildPackets(t, child); got != want {
			t.Fatalf("%s: the outbound transport SA processed %d packets, want %d", label, got, want)
		}
		if got := espPolStat(t, "XfrmOutStateModeError"); got != before {
			t.Fatalf("%s: XfrmOutStateModeError moved %d -> %d on a whole datagram", label, before, got)
		}
	}
	whole("before the fragments", 1)

	udp := transportOutputUDP(source, []byte("transport child plaintext fragment pair"))
	refusedBefore := espPolStat(t, "XfrmOutStateModeError")
	// The first fragment carries More Fragments (0x2000) at offset 0; the second carries
	// the rest at offset 3, in units of eight octets.
	transportOutputSend(t, fd, target, transportOutputIPv4(source, target, udp[:24], 0x2000, transportChildFragmentID), true)
	transportOutputSend(t, fd, target, transportOutputIPv4(source, target, udp[24:], 3, transportChildFragmentID), true)
	refused := espPolStat(t, "XfrmOutStateModeError") - refusedBefore
	protected := transportChildPackets(t, child) - 1
	switch {
	case refused == 2 && protected == 0:
	case refused == 0 && protected == 1:
	default:
		t.Errorf("plaintext fragment pair: XfrmOutStateModeError moved by %d and the transport SA protected %d packets; "+
			"want both fragments refused at the transform (2, 0) or one gathered datagram protected (0, 1)", refused, protected)
	}
	whole("after the fragments", 2+protected)
}

// transportChildInstall installs one transport-mode Child SA through createFirstChildSA
// over the loaded backend, negotiated for every protocol and port between the two outer
// hosts, so every datagram the raw sender writes meets its outbound policy. The caller
// MUST be in Ze's namespace.
func transportChildInstall(t *testing.T, lab *espPolLab) *ChildSA {
	t.Helper()
	local := mustCIDRNet(t, espPolZeOuter+"/32")
	remote := mustCIDRNet(t, espPolPeerOuter+"/32")
	sa := testSA()
	sa.IsInitiator = true
	sa.UseTransportMode = true
	sa.NegotiatedTSi, sa.NegotiatedTSr = local, remote
	sa.NegotiatedPairs = []tsPair{{
		I: tsSelector{Net: local, Port: ipsec.AnyPort()},
		R: tsSelector{Net: remote, Port: ipsec.AnyPort()},
	}}
	group := ipsec.ESPGroup{
		Name: "transport-child-gcm", Lifetime: 3600, PFS: ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}},
	}
	// RFC 4301 Section 4.1
	child, err := createFirstChildSA(sa, group, espPolZeOuter, espPolPeerOuter, 0,
		dataplane.Get(), slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("install the transport-mode Child SA: %v", err)
	}
	t.Cleanup(child.Clear)
	t.Cleanup(func() {
		lab.enter(t, lab.ze)
		removeChildSA(child, dataplane.Get(), slogutil.DiscardLogger())
	})
	if !child.ESPInstalled {
		t.Fatal("the transport-mode Child SA was not installed in the kernel")
	}
	return child
}

// transportChildWantModes reads back the kernel's two states and the Child SA's policies
// and requires every one to be transport mode, with no forward policy: a transport SA
// carries this host's own traffic only.
func transportChildWantModes(t *testing.T, child *ChildSA) {
	t.Helper()
	for _, state := range []struct {
		name string
		spi  uint32
		dst  string
	}{{"inbound", child.InboundSPI, espPolZeOuter}, {"outbound", child.OutboundSPI, espPolPeerOuter}} {
		got, err := netlink.XfrmStateGet(&netlink.XfrmState{
			Dst: net.ParseIP(state.dst), Spi: int(state.spi), Proto: netlink.XFRM_PROTO_ESP,
		})
		if err != nil {
			t.Fatalf("read the %s state %#x: %v", state.name, state.spi, err)
		}
		if got.Mode != netlink.XFRM_MODE_TRANSPORT {
			t.Errorf("the %s state %#x is mode %v, want transport like its pair", state.name, state.spi, got.Mode)
		}
	}
	policies, err := netlink.XfrmPolicyList(netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("read the kernel's policies: %v", err)
	}
	directions := map[netlink.Dir]int{}
	for i := range policies {
		for _, template := range policies[i].Tmpls {
			if template.Reqid != int(child.ReqID) {
				continue
			}
			directions[policies[i].Dir]++
			if template.Mode != netlink.XFRM_MODE_TRANSPORT {
				t.Errorf("the Child SA's %v policy template is mode %v, want transport", policies[i].Dir, template.Mode)
			}
		}
	}
	want := map[netlink.Dir]int{netlink.XFRM_DIR_IN: 1, netlink.XFRM_DIR_OUT: 1}
	if len(directions) != len(want) || directions[netlink.XFRM_DIR_IN] != 1 || directions[netlink.XFRM_DIR_OUT] != 1 {
		t.Errorf("the Child SA's policies by direction = %v, want one inbound and one outbound and no forward policy (%v)", directions, want)
	}
}

// transportChildPackets answers how many packets the Child SA's outbound state processed.
func transportChildPackets(t *testing.T, child *ChildSA) uint64 {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: net.ParseIP(espPolPeerOuter), Spi: int(child.OutboundSPI), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read the outbound transport SA %#x: %v", child.OutboundSPI, err)
	}
	return state.Statistics.Packets
}
