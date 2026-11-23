//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions and Child SA selectors.
// Related: rfc4301_espform_policy_integration_linux_test.go -- isolated namespaces and authenticated sender.
// Related: rfc4301_fragment_linux_test.go -- the ordinary UDP packet and fragment builders.
// Packet-level conformance probes awaiting observation on Ze's kernel.

package engine

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/net/ipv4"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	fragmentBypassPort           = 6000
	fragmentDiscardPort          = 7000
	fragmentStatTemplateMismatch = "XfrmInTmplMismatch"
)

// fragmentPorts is one test Child SA's selector in local/remote orientation.
type fragmentPorts struct {
	local, remote ipsec.PortSelector
}

// fragmentChildren installs distinct Child SAs through the production creator.
// The caller MUST be in the Ze namespace made by newEspPolLab; each test returns
// there after creating a peer socket, before making another kernel operation.
func fragmentChildren(t *testing.T, selectors ...fragmentPorts) []*ChildSA {
	t.Helper()
	return fragmentChildrenWithDataplane(t, dataplane.Get(), selectors...)
}

// fragmentChildrenWithDataplane keeps the ordinary fixture while allowing an explicit
// test-only backend wrapper. Existing fragmentChildren callers remain unchanged.
func fragmentChildrenWithDataplane(t *testing.T, dp dataplane.Dataplane, selectors ...fragmentPorts) []*ChildSA {
	t.Helper()
	natt := mbTransport(t, net.JoinHostPort(espPolZeOuter, "4500"), true)
	if err := transport.EnableESPInUDP(natt.Conn()); err != nil {
		t.Fatalf("enable ESP-in-UDP for the private receiver: %v", err)
	}
	group := ipsec.ESPGroup{
		Name: "fragment-gcm", Lifetime: 3600, PFS: ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}},
	}
	children := make([]*ChildSA, 0, len(selectors))
	for index, ports := range selectors {
		sa := testSA()
		sa.IsInitiator = true
		sa.ChildInboundSPI = 0x6f010001 + uint32(index)*2
		sa.ChildOutboundSPI = sa.ChildInboundSPI + 1
		sa.bindSockets(nil, natt)
		sa.floatToNATTPort()
		sa.NATDetected = false
		sa.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.NATTPort}
		sa.NegotiatedTSi = mustCIDRNet(t, "10.2.0.0/16")
		sa.NegotiatedTSr = mustCIDRNet(t, "10.1.0.0/16")
		sa.NegotiatedPairs = []tsPair{{
			I: tsSelector{Net: sa.NegotiatedTSi, Port: ports.local, Proto: sadSelProtoUDP},
			R: tsSelector{Net: sa.NegotiatedTSr, Port: ports.remote, Proto: sadSelProtoUDP},
		}}
		// RFC 4301 Sections 4.4.2 and 5.2: install the negotiated SA and its policies.
		child, err := createFirstChildSA(sa, group, espPolZeOuter, espPolPeerOuter,
			0, dp, slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("create Child SA %d: %v", index, err)
		}
		t.Cleanup(child.Clear)
		// Cleanups run last-registered first, so the Child SA's states and policies leave
		// the kernel through the production teardown, on the same dp, before its keys are
		// cleared, rather than staying in force until the lab's namespace goes.
		t.Cleanup(func() { removeChildSA(child, dp, slogutil.DiscardLogger()) })
		if !child.ESPInstalled {
			t.Fatalf("Child SA %d was not installed in the kernel", index)
		}
		children = append(children, child)
	}
	return children
}

// fragmentPlainSender opens an ordinary private-peer UDP packet sender. Its source
// address belongs to that peer's loopback. Existing IPv4/UDP builders and fragSplit
// supply every packet; RawConn only preserves their normal fragmentation headers.
// It MUST return the calling thread to Ze's namespace before its caller continues.
func fragmentPlainSender(t *testing.T, lab *espPolLab) *ipv4.RawConn {
	t.Helper()
	lab.enter(t, lab.peer)
	defer lab.enter(t, lab.ze)
	espPolAddr(t, "lo", sadSelInnerRemote+"/32")
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: espPolLink(t, espPolPeerLink).Attrs().Index,
		Dst:       mustCIDRNet(t, "10.2.0.0/16"), Gw: net.ParseIP(espPolZeOuter),
	}); err != nil {
		t.Fatalf("route the private peer's UDP traffic through Ze: %v", err)
	}
	return fragmentRawSender(t, sadSelInnerRemote)
}

// fragmentRawSender MUST be called in the namespace that owns source. The socket
// keeps that namespace when the controller returns to Ze's namespace.
func fragmentRawSender(t *testing.T, source string) *ipv4.RawConn {
	t.Helper()
	var listen net.ListenConfig
	conn, err := listen.ListenPacket(context.Background(), "ip4:255", source)
	if err != nil {
		t.Fatalf("open the private peer's IPv4 sender: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close the private peer's IPv4 sender: %v", err)
		}
	})
	raw, err := ipv4.NewRawConn(conn)
	if err != nil {
		t.Fatalf("preserve the existing IPv4 fragment headers: %v", err)
	}
	return raw
}

func fragmentSend(t *testing.T, conn *ipv4.RawConn, packet []byte) {
	t.Helper()
	header, err := ipv4.ParseHeader(packet)
	if err != nil {
		t.Fatalf("read the test packet's IPv4 header: %v", err)
	}
	if err := conn.WriteTo(header, packet[header.Len:], nil); err != nil {
		t.Fatalf("send ordinary UDP in the private namespace: %v", err)
	}
}

// TestRFC4301CleartextFragmentsRespectOperatorDisposition observes local delivery
// after actual operator policies and a Child SA are installed. Whole and fragmented
// cleartext UDP for BYPASS must arrive; cleartext for DISCARD or PROTECT must not.
// Authenticated PROTECT delivery and readback of both operator entries are controls.
// RFC 4301 Section 7.4: "All implementations MUST support stateful fragment checking
// to accommodate BYPASS traffic for which a non-trivial port range is specified."
// This local-path probe does not claim the distinct forwarding obligation. It sends no
// lone non-initial fragment: local input reassembles a datagram before any policy check,
// so a lone tail is never delivered whatever the SPD holds, and its absence would prove
// nothing about the BYPASS entry.
//
// RFC requirement: RFC4301-7.4-2 positive -- on Ze's kernel, under unmatched discard,
// the kernel holds the inbound BYPASS and DISCARD entries Ze installs for operator
// entries on one IPv4 UDP local port each, and a whole and a fragmented cleartext
// datagram to the bypassed port are delivered locally, while a whole and a fragmented
// cleartext datagram to the discarded port (XfrmInPolBlock) or to the port a Child SA
// protects (XfrmInTmplMismatch) are not.
//
//ze:guest-root
func TestRFC4301CleartextFragmentsRespectOperatorDisposition(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	child := fragmentChildren(t, fragmentPorts{
		local: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort}, remote: ipsec.AnyPort(),
	})[0]
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	bypassed := sadSelListen(t, sadSelInnerLocal, fragmentBypassPort)
	discarded := sadSelListen(t, sadSelInnerLocal, fragmentDiscardPort)
	entries := map[string]ipsec.SPDPolicy{
		"fragment-bypass": {
			Name: "fragment-bypass", Action: dataplane.SPActionBypass, Order: 1000,
			Direction: ipsec.SPDDirIn, Protocol: sadSelProtoUDP,
			LocalPrefix:  mustCIDRNet(t, "10.2.0.0/16"),
			LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentBypassPort},
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"), RemotePort: ipsec.AnyPort(),
		},
		"fragment-discard": {
			Name: "fragment-discard", Action: dataplane.SPActionDiscard, Order: 1001,
			Direction: ipsec.SPDDirIn, Protocol: sadSelProtoUDP,
			LocalPrefix:  mustCIDRNet(t, "10.2.0.0/16"),
			LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentDiscardPort},
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"), RemotePort: ipsec.AnyPort(),
		},
	}
	// RFC 4301 Sections 4.4.1 and 7.4: use the same reconcile path as config apply.
	if err := reconcileForTest(dataplane.Get(), nil, entries, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("install the operator SPD entries: %v", err)
	}
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	fragmentWantOperatorPolicies(t, entries)
	authenticated := newEspPolSender(t, lab, child)
	authenticated.sendEncap(t, sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort)))
	if !sadSelDelivered(t, inside) {
		t.Fatal("the authenticated PROTECT control was not delivered")
	}
	plain := fragmentPlainSender(t, lab)
	// These cases stay on the lab's locked thread; t.Run would move namespace-sensitive reads.
	for _, tc := range []struct {
		name     string
		port     uint16
		listener *net.UDPConn
		counter  string
	}{
		{"BYPASS", fragmentBypassPort, bypassed, ""},
		{"DISCARD", fragmentDiscardPort, discarded, "XfrmInPolBlock"},
		{"PROTECT", sadSelPort, inside, fragmentStatTemplateMismatch},
	} {
		for _, split := range []bool{false, true} {
			packet := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(tc.port))
			before := 0
			if tc.counter != "" {
				before = sadSelStat(t, tc.counter)
			}
			if split {
				first, last := fragSplit(packet)
				fragmentSend(t, plain, first)
				fragmentSend(t, plain, last)
			} else {
				fragmentSend(t, plain, packet)
			}
			if tc.counter != "" {
				awaitForwardKernelCounter(t, tc.name, uint64(before+1),
					func(t *testing.T) uint64 { return uint64(sadSelStat(t, tc.counter)) })
			}
			delivered := sadSelDelivered(t, tc.listener)
			if delivered != (tc.counter == "") {
				t.Errorf("%s split=%v: delivered=%v, want %v", tc.name, split, delivered, tc.counter == "")
			}
		}
	}
}

func fragmentWantOperatorPolicies(t *testing.T, entries map[string]ipsec.SPDPolicy) {
	t.Helper()
	actual, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("read back the operator policies: %v", err)
	}
	for name, entry := range entries {
		wanted := spdPolicyParams(entry)
		for i := range wanted {
			want := &wanted[i]
			found := false
			for j := range actual {
				got := &actual[j]
				if got.Role == dataplane.PolicyRoleOperator && got.Dir == want.Dir &&
					got.Action == want.Action && got.SrcPort == want.SrcPort && got.DstPort == want.DstPort &&
					got.UpperProto == want.UpperProto && got.Priority == want.Priority &&
					sameIPNet(got.Src, want.Src) && sameIPNet(got.Dst, want.Dst) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("operator entry %s direction %v was not stored with its exact selector, action and order", name, want.Dir)
			}
		}
	}
}

// fragmentSiblingSender shares the existing peer socket and encoder with a second
// fixture SA. Both testSAs carry identical Ni, Nr and SK_d; verify the keys in the
// actual SAD agree before reusing the immutable encoder under another SPI.
func fragmentSiblingSender(t *testing.T, original *espPolSender, sibling *ChildSA) *espPolSender {
	t.Helper()
	var keys [][]byte
	for _, spi := range []uint32{original.spi, sibling.InboundSPI} {
		state, err := netlink.XfrmStateGet(&netlink.XfrmState{
			Dst: net.ParseIP(espPolZeOuter), Spi: int(spi), Proto: netlink.XFRM_PROTO_ESP,
		})
		if err != nil {
			t.Fatalf("read the sibling fixture's SAD state: %v", err)
		}
		if state.Aead == nil {
			t.Fatal("the sibling fixture has no AEAD key")
		}
		keys = append(keys, state.Aead.Key)
	}
	if !bytes.Equal(keys[0], keys[1]) {
		t.Fatal("the two identical key-derivation fixtures do not share an encoder key")
	}
	sender := *original
	sender.spi, sender.seq = sibling.InboundSPI, 0
	return &sender
}

// TestRFC4301SiblingSAsKeepInboundSelectorsBound observes two independently created
// Child SAs with equal endpoints and distinct exact local ports. Each own-SA control
// must arrive; whole and fragmented UDP sent through the other SA must not arrive, each
// refused by that SA's state selector (XfrmInStateMismatch +1, XfrmInNoPols and
// XfrmInPolBlock unchanged) and followed by an own-SA control that arrives.
// RFC 4301 Section 5.2: "Then match the packet against the inbound selectors identified
// by the SAD entry to verify that the received packet is appropriate for the SA via
// which it was received."
//
//ze:guest-root
func TestRFC4301SiblingSAsKeepInboundSelectorsBound(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	children := fragmentChildren(t,
		fragmentPorts{local: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort}, remote: ipsec.AnyPort()},
		fragmentPorts{local: ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentBypassPort}, remote: ipsec.AnyPort()},
	)
	first := newEspPolSender(t, lab, children[0])
	second := fragmentSiblingSender(t, first, children[1])
	firstPort := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	secondPort := sadSelListen(t, sadSelInnerLocal, fragmentBypassPort)
	packet := func(port uint16) []byte {
		return sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(port))
	}
	first.sendEncap(t, packet(sadSelPort))
	if !sadSelDelivered(t, firstPort) {
		t.Fatal("the first SA did not deliver its own selector control")
	}
	second.sendEncap(t, packet(fragmentBypassPort))
	if !sadSelDelivered(t, secondPort) {
		t.Fatal("the sibling SA did not deliver its own selector control")
	}
	// Each refusal is the SA's own state selector, after reassembly for the split form:
	// XfrmInStateMismatch moves by exactly one and no SPD counter moves, so a drop for
	// any other reason, or a packet lost before ESP, is a red. The own-SA control after
	// each refusal shows the SA still delivers, so the refusal was the selector's.
	for _, tc := range []struct {
		name        string
		sender      *espPolSender
		port, own   uint16
		listener    *net.UDPConn
		ownListener *net.UDPConn
	}{
		{"first SA carrying the sibling port", first, fragmentBypassPort, sadSelPort, secondPort, firstPort},
		{"sibling SA carrying the first port", second, sadSelPort, fragmentBypassPort, firstPort, secondPort},
	} {
		before := sadSelReadDropCounters(t, sadSelStat)
		tc.sender.sendEncap(t, packet(tc.port))
		if sadSelDelivered(t, tc.listener) {
			t.Errorf("%s: a whole packet outside that SA's selectors was delivered", tc.name)
		}
		sadSelWantDropCounters(t, tc.name+", whole", before, sadSelReadDropCounters(t, sadSelStat),
			sadSelDropCounters{stateMismatch: 1})
		tc.sender.sendEncap(t, packet(tc.own))
		if !sadSelDelivered(t, tc.ownListener) {
			t.Errorf("%s: the SA's own control after the whole refusal was not delivered", tc.name)
		}

		before = sadSelReadDropCounters(t, sadSelStat)
		initial, nonInitial := fragSplit(packet(tc.port))
		tc.sender.sendEncap(t, initial)
		tc.sender.sendEncap(t, nonInitial)
		if sadSelDelivered(t, tc.listener) {
			t.Errorf("%s: a fragmented packet outside that SA's selectors was delivered", tc.name)
		}
		sadSelWantDropCounters(t, tc.name+", fragmented", before, sadSelReadDropCounters(t, sadSelStat),
			sadSelDropCounters{stateMismatch: 1})
		tc.sender.sendEncap(t, packet(tc.own))
		if !sadSelDelivered(t, tc.ownListener) {
			t.Errorf("%s: the SA's own control after the fragmented refusal was not delivered", tc.name)
		}
	}
}
