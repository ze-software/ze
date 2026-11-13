//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact negotiated selectors.
// Related: rfc4301_espform_policy_integration_linux_test.go -- private Ze/peer namespaces and authenticated ESP sender.
// Related: child.go -- childPairPolicy converts IKE's MH high octet at the XFRM boundary.
// RFC naming: untagged -- investigate actual inbound and forward MH classification on Ze's kernel.

package engine

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	mhBoundaryLocal      = "2001:db8:122::1"
	mhBoundaryHost       = "2001:db8:122::2"
	mhBoundaryRouter     = "2001:db8:122::fe"
	mhBoundaryRemote     = "2001:db8:121::7"
	mhBoundaryRouterLink = "zemhrt0"
	mhBoundaryHostLink   = "zemhhost0"
)

// mhBoundaryAddress installs an address without DAD in the private topology.
func mhBoundaryAddress(t *testing.T, name, prefix string) {
	t.Helper()
	addr, err := netlink.ParseAddr(prefix)
	if err != nil {
		t.Fatal(err)
	}
	addr.Flags = unix.IFA_F_NODAD
	if err := netlink.AddrAdd(espPolLink(t, name), addr); err != nil {
		t.Fatalf("add private MH destination %s: %v", prefix, err)
	}
}

func mhBoundaryListener(t *testing.T, address string) *net.IPConn {
	t.Helper()
	conn, err := net.ListenIP("ip6:135", &net.IPAddr{IP: net.ParseIP(address)})
	if err != nil {
		t.Fatalf("open the actual MH receiver at %s: %v", address, err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close the MH receiver: %v", err)
		}
	})
	return conn
}

// mhBoundaryReceivers adds a nonlocal protected host as well as a local IPv6
// destination. Both raw sockets run the real rawv6_rcv XFRM input check; the
// host socket additionally proves the packet crossed Ze's forward check.
// The calling locked thread MUST return to Ze before this function returns.
func mhBoundaryReceivers(t *testing.T, lab *espPolLab) (*net.IPConn, *net.IPConn) {
	t.Helper()
	defer lab.enter(t, lab.ze)
	for _, setting := range []struct{ path, value string }{
		{"/proc/sys/net/ipv6/conf/all/disable_ipv6", "0"},
		{"/proc/sys/net/ipv6/conf/default/disable_ipv6", "0"},
		{"/proc/sys/net/ipv6/conf/lo/disable_ipv6", "0"},
		{"/proc/sys/net/ipv6/conf/all/forwarding", "1"},
	} {
		if err := os.WriteFile(setting.path, []byte(setting.value), 0o600); err != nil {
			t.Fatalf("set private MH router %s: %v", setting.path, err)
		}
	}
	mhBoundaryAddress(t, "lo", mhBoundaryLocal+"/128")
	local := mhBoundaryListener(t, mhBoundaryLocal)
	host, err := netns.New()
	if err != nil {
		t.Fatalf("make private MH protected host: %v", err)
	}
	t.Cleanup(func() {
		if err := host.Close(); err != nil {
			t.Errorf("close MH protected host namespace: %v", err)
		}
	})
	lab.enter(t, lab.ze)
	link := &netlink.Veth{
		Name: mhBoundaryRouterLink, PeerName: mhBoundaryHostLink,
		PeerNamespace: netlink.NsFd(int(host)),
	}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("join private MH protected host: %v", err)
	}
	mhBoundaryAddress(t, mhBoundaryRouterLink, mhBoundaryRouter+"/64")
	espPolUp(t, mhBoundaryRouterLink)
	routerLink := espPolLink(t, mhBoundaryRouterLink)
	lab.enter(t, host)
	espPolUp(t, "lo")
	mhBoundaryAddress(t, mhBoundaryHostLink, mhBoundaryHost+"/64")
	espPolUp(t, mhBoundaryHostLink)
	hostLink := espPolLink(t, mhBoundaryHostLink)
	forwarded := mhBoundaryListener(t, mhBoundaryHost)
	lab.enter(t, lab.ze)
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: routerLink.Attrs().Index, Family: unix.AF_INET6,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(mhBoundaryHost),
		HardwareAddr: hostLink.Attrs().HardwareAddr,
	}); err != nil {
		t.Fatalf("install private MH protected-host neighbor: %v", err)
	}
	return local, forwarded
}

// mhBoundaryLANEgress gives decrypted MH packets an explicit cleartext route
// to the protected LAN through the real operator-policy producer. Only OUT is
// bypassed: IN and FWD still require the Child SA's exact selectors. This keeps
// the separate unmatched-discard forwarding-default investigation from deciding
// a test of MH classification before any MH field is examined.
func mhBoundaryLANEgress(t *testing.T) {
	t.Helper()
	entry := ipsec.SPDPolicy{
		Name: "mh-protected-lan", Action: dataplane.SPActionBypass,
		Direction: ipsec.SPDDirOut, Order: 1500, Protocol: ipsec.MobilityHeaderProtocol,
		LocalPrefix: mustCIDRNet(t, "2001:db8:121::/64"), LocalPort: ipsec.AnyPort(),
		RemotePrefix: mustCIDRNet(t, "2001:db8:122::/64"), RemotePort: ipsec.AnyPort(),
	}
	entries := map[string]ipsec.SPDPolicy{entry.Name: entry}
	log := slogutil.DiscardLogger()
	installSPDPolicies(dataplane.Get(), nil, entries, log)
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, log) })
	actual, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("read back the private protected-LAN egress policy: %v", err)
	}
	for _, policy := range actual {
		if policy.Dir != dataplane.SADirOut || policy.Action != dataplane.SPActionBypass {
			continue
		}
		if policy.UpperProto != ipsec.MobilityHeaderProtocol || policy.Priority != int(entry.Order) {
			continue
		}
		if policy.Src == nil {
			continue
		}
		if policy.Dst == nil {
			continue
		}
		if policy.Src.String() == entry.LocalPrefix.String() && policy.Dst.String() == entry.RemotePrefix.String() {
			return
		}
	}
	t.Fatal("the exact private MH protected-LAN OUT BYPASS did not reach the kernel")
}

// mhBoundaryChild uses the real IKE Child SA creator and XFRM backend. The
// remote side constrains received types; local remains ANY. Outer ESP uses
// the ordinary UDP-encapsulated path so the test measures inner IPv6 selectors.
func mhBoundaryChild(t *testing.T, port ipsec.PortSelector, natt *transport.UDPTransport) (*ChildSA, error) {
	t.Helper()
	sa := testSA()
	sa.IsInitiator = true
	sa.bindSockets(nil, natt)
	sa.floatToNATTPort()
	sa.NATDetected = true
	sa.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.NATTPort}
	local := mustCIDRNet(t, "2001:db8:122::/64")
	remote := mustCIDRNet(t, "2001:db8:121::/64")
	sa.NegotiatedTSi, sa.NegotiatedTSr = local, remote
	sa.NegotiatedPairs = []tsPair{{
		I: tsSelector{Net: local, Port: ipsec.AnyPort(), Proto: ipsec.MobilityHeaderProtocol},
		R: tsSelector{Net: remote, Port: port, Proto: ipsec.MobilityHeaderProtocol},
	}}
	group := ipsec.ESPGroup{
		Name: "mh-gcm", Lifetime: 3600, PFS: ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}},
	}
	child, err := createFirstChildSA(sa, group, espPolZeOuter, espPolPeerOuter, 0,
		dataplane.Get(), slogutil.DiscardLogger())
	if err != nil {
		return nil, err
	}
	if !child.ESPInstalled {
		t.Fatal("the MH Child SA did not reach XFRM")
	}
	t.Cleanup(child.Clear)
	return child, nil
}

// mhBoundarySender rekeys the authenticated packet builder from the actual
// inbound state while retaining the single peer socket across the two stages.
func mhBoundarySender(t *testing.T, child *ChildSA, conn *net.UDPConn) *espPolSender {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: net.ParseIP(espPolZeOuter), Spi: int(child.InboundSPI), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read back actual MH inbound state: %v", err)
	}
	if state.Aead == nil || len(state.Aead.Key) != 36 {
		t.Fatal("the MH state carries no AES-256-GCM key")
	}
	block, err := aes.NewCipher(state.Aead.Key[:32])
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return &espPolSender{spi: child.InboundSPI, aead: aead, salt: state.Aead.Key[32:], encap: conn}
}

// mhBoundaryPacket is a checksummed IPv6 MH datagram, not a raw6 outbound
// socket flow (raw6 outbound alone fills the MH flow key). Header byte 6 is
// protocol135; MH byte2 is Type and bytes4..5 carry the IPv6 pseudoheader
// checksum. The test claims no MIPv6 binding-management implementation.
func mhBoundaryPacket(destination string, typ byte) []byte {
	mh := make([]byte, 24)
	mh[0], mh[1], mh[2] = 59, 2, typ
	copy(mh[8:24], net.ParseIP(mhBoundaryRemote).To16())
	pseudo := make([]byte, 40, 40+len(mh))
	copy(pseudo[:16], net.ParseIP(mhBoundaryRemote).To16())
	copy(pseudo[16:32], net.ParseIP(destination).To16())
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(mh)))
	pseudo[39] = ipsec.MobilityHeaderProtocol
	binary.BigEndian.PutUint16(mh[4:6], sadSelChecksum(append(pseudo, mh...)))
	packet := make([]byte, 40, 40+len(mh))
	packet[0], packet[6], packet[7] = 0x60, ipsec.MobilityHeaderProtocol, 64
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(mh)))
	copy(packet[8:24], net.ParseIP(mhBoundaryRemote).To16())
	copy(packet[24:40], net.ParseIP(destination).To16())
	return append(packet, mh...)
}

type mhBoundaryDropCounts struct {
	policy, state, missingPolicy int
}

func mhBoundaryDrops(t *testing.T) mhBoundaryDropCounts {
	t.Helper()
	return mhBoundaryDropCounts{
		policy:        sadSelStat(t, "XfrmInPolBlock"),
		state:         sadSelStat(t, "XfrmInStateMismatch"),
		missingPolicy: sadSelStat(t, "XfrmInNoPols"),
	}
}

// mhBoundaryDelivered waits for one of the two actual outcomes: socket
// delivery or a counted XFRM selector rejection. A bounded read deadline multiplexes those
// observations; the verdict never follows from elapsed time alone.
func mhBoundaryDelivered(t *testing.T, conn *net.IPConn, typ byte, before mhBoundaryDropCounts) bool {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var buf [256]byte
	for range 64 {
		if !time.Now().Before(deadline) {
			break
		}
		if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		n, _, err := conn.ReadFromIP(buf[:])
		if err == nil {
			if n < 8 || buf[2] != typ {
				t.Fatalf("received unexpected MH bytes: %x", buf[:n])
			}
			if got := mhBoundaryDrops(t); got != before {
				t.Fatalf("MH delivery also counted XFRM rejection: %+v -> %+v", before, got)
			}
			return true
		}
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("read actual MH receiver: %v", err)
		}
		if got := mhBoundaryDrops(t); got != before {
			t.Logf("MH Type %d actual XFRM rejection counters: %+v -> %+v", typ, before, got)
			return false
		}
	}
	t.Fatalf("MH Type %d produced neither delivery nor counted XFRM rejection", typ)
	return false
}

// TestMHChildPoliciesSelectIncomingIPv6Packets preserves actual ANY Type5/7
// delivery on IN and FWD, then requires constrained MH to be refused before
// the real kernel's existing states and policies change. The earlier matching
// exact-Type5 packet drops established the absent classifier; they are not
// accepted here as conformant product delivery.
//
//ze:guest-root
func TestMHChildPoliciesSelectIncomingIPv6Packets(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	local, forwarded := mhBoundaryReceivers(t, lab)
	mhBoundaryLANEgress(t)
	natt := mbTransport(t, net.JoinHostPort(espPolZeOuter, "4500"), true)
	if err := transport.EnableESPInUDP(natt.Conn()); err != nil {
		t.Fatalf("enable ordinary ESP-in-UDP reception: %v", err)
	}
	lab.enter(t, lab.peer)
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.NATTPort})
	lab.enter(t, lab.ze)
	if err != nil {
		t.Fatalf("open the single authenticated peer socket: %v", err)
	}
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Errorf("close the MH peer socket: %v", err)
		}
	})
	{
		child, err := mhBoundaryChild(t, ipsec.AnyPort(), natt)
		if err != nil {
			t.Fatalf("create the ANY MH Child SA: %v", err)
		}
		t.Cleanup(func() { removeChildSA(child, dataplane.Get(), slogutil.DiscardLogger()) })
		sender := mhBoundarySender(t, child, peer)
		for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirFwd} {
			want := childPolicies(child, dir)
			if len(want) != 1 {
				t.Fatalf("MH direction %d produced %d policies", dir, len(want))
			}
			kernelDir := netlink.XFRM_DIR_IN
			if dir == dataplane.SADirFwd {
				kernelDir = netlink.XFRM_DIR_FWD
			}
			got, err := netlink.XfrmPolicyGet(&netlink.XfrmPolicy{
				Src: want[0].Src, Dst: want[0].Dst, Proto: netlink.Proto(ipsec.MobilityHeaderProtocol),
				SrcPort: int(want[0].SrcPort.Port), SrcPortMask: want[0].SrcPort.Mask,
				Dir: kernelDir,
			})
			if err != nil {
				t.Fatalf("read actual MH policy direction %d: %v", dir, err)
			}
			if got.SrcPort != int(want[0].SrcPort.Port) || got.SrcPortMask != want[0].SrcPort.Mask {
				t.Fatalf("MH readback direction %d: source %d/%#x, want %+v", dir, got.SrcPort, got.SrcPortMask, want[0].SrcPort)
			}
		}
		for _, receiver := range []struct {
			name, address string
			conn          *net.IPConn
		}{{"IN", mhBoundaryLocal, local}, {"FWD", mhBoundaryHost, forwarded}} {
			for _, typ := range []byte{5, 7} {
				before := mhBoundaryDrops(t)
				packet := sender.seal(41, mhBoundaryPacket(receiver.address, typ))
				if _, err := sender.encap.WriteToUDP(packet, &net.UDPAddr{IP: net.ParseIP(espPolZeOuter), Port: transport.NATTPort}); err != nil {
					t.Fatalf("send authenticated IPv6 MH packet: %v", err)
				}
				delivered := mhBoundaryDelivered(t, receiver.conn, typ, before)
				t.Logf("ANY MH path=%s type=%d delivered=%v", receiver.name, typ, delivered)
				if !delivered {
					t.Errorf("ANY MH path=%s Type %d was not delivered", receiver.name, typ)
				}
			}
		}
		statesBefore, err := dataplane.Get().ListSAs(0)
		if err != nil {
			t.Fatal(err)
		}
		policiesBefore, err := dataplane.Get().ListPolicies()
		if err != nil {
			t.Fatal(err)
		}
		exact, err := mhBoundaryChild(t, ipsec.PortSelector{Form: ipsec.PortSingle, Port: 1280}, natt)
		if !errors.Is(err, errTSUnacceptable) {
			t.Fatalf("exact MH error %v, want typed selector refusal", err)
		}
		if errors.Is(err, dataplane.ErrNotSupported) || exact != nil {
			t.Fatalf("exact MH refusal returned a degraded/established Child: child=%v error=%v", exact, err)
		}
		statesAfter, err := dataplane.Get().ListSAs(0)
		if err != nil {
			t.Fatal(err)
		}
		policiesAfter, err := dataplane.Get().ListPolicies()
		if err != nil {
			t.Fatal(err)
		}
		if len(statesAfter) != len(statesBefore) || len(policiesAfter) != len(policiesBefore) {
			t.Fatalf("refused exact MH changed kernel states %d->%d or policies %d->%d", len(statesBefore), len(statesAfter), len(policiesBefore), len(policiesAfter))
		}
	}
}
