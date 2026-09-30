// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA install into the dataplane
// Related: rfc4301_sad_selector_linux_test.go -- the namespace probe, ESP sender and listeners reused here
// Related: spd_policy.go -- spdPolicyDirection, the operator BYPASS and DISCARD entries
// Related: unmatched.go -- unmatchedPolicies, the catch-all ranked last
//
// VALIDATES: against a real Linux XFRM stack, the inbound steps of RFC 4301 Section 5.2 on
// the SAD and SPD Ze installs: an ESP packet is looked up in the SAD by its SPI, and is
// discarded when no SA matches (step 3a); a packet that is not ESP is looked up in the
// SPD-I and bypassed or discarded as the entry it matches says (step 3b); and an ESP packet
// is processed with the SA its SPI selected, so one failing that SA's integrity check is
// discarded (step 4). The catch-all's handling of a packet no SPD-I entry matches follows
// the `vpn ipsec unmatched` leaf, which is the RFC4301-4.4.1-3 deviation.
// PREVENTS: an inbound SA or SPD-I entry Ze installs that the kernel does not apply, so an
// ESP packet with an unknown SPI, a forged ESP packet, or a packet the operator discards
// reaches a local socket.
//
// No privilege is needed: sadSelOwnNamespace re-runs the unit in a user and network
// namespace of its own, as its sibling file explains.

//go:build linux

package engine

import (
	"crypto/aes"
	"crypto/cipher"
	"net"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	inStepPeer        = "10.3.0.5"
	inStepBypassPort  = 7101
	inStepDiscardPort = 7102
	inStepStrayPort   = 7109

	// inStepOperatorRank is an operator entry order ahead of the catch-all
	// (dataplane.PriorityUnmatched), so each operator entry is searched first.
	inStepOperatorRank = 1000

	inStepStatNoStates   = "XfrmInNoStates"
	inStepStatProtoError = "XfrmInStateProtoError"
	inStepStatPolBlock   = "XfrmInPolBlock"
)

// inStepNetns is sadSelNetns plus the address the clear peer sends from.
func inStepNetns(t *testing.T) {
	t.Helper()
	sadSelNetns(t)
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	addr, err := netlink.ParseAddr(inStepPeer + "/32")
	if err != nil {
		t.Fatalf("parse %s: %v", inStepPeer, err)
	}
	if err := netlink.AddrAdd(lo, addr); err != nil {
		t.Fatalf("add %s to lo: %v", inStepPeer, err)
	}
}

// inStepSPDI loads the XFRM backend and installs the SPD-I the clear-packet units measure,
// through the functions Ze's apply path calls: an operator BYPASS entry for UDP from
// 10.3.0.5 to local port 7101, an operator DISCARD entry for UDP to local port 7102, and
// the catch-all with the given disposition.
func inStepSPDI(t *testing.T, unmatched dataplane.SPAction) {
	t.Helper()
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load xfrm backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close xfrm backend: %v", err)
		}
	})
	dp := dataplane.Get()
	log := slogutil.DiscardLogger()

	anyPort := ipsec.PortSelector{Form: ipsec.PortAny}
	local := mustCIDRNet(t, sadSelInnerLocal+"/32")
	peer := mustCIDRNet(t, inStepPeer+"/32")
	entries := map[string]ipsec.SPDPolicy{
		"bypass-7101": {
			Name: "bypass-7101", Action: dataplane.SPActionBypass, Order: inStepOperatorRank,
			Direction: ipsec.SPDDirIn, Protocol: sadSelProtoUDP,
			LocalPrefix: local, LocalPort: ipsec.PortSelector{Form: ipsec.PortSingle, Port: inStepBypassPort},
			RemotePrefix: peer, RemotePort: anyPort,
		},
		"discard-7102": {
			Name: "discard-7102", Action: dataplane.SPActionDiscard, Order: inStepOperatorRank + 1,
			Direction: ipsec.SPDDirIn, Protocol: sadSelProtoUDP,
			LocalPrefix: local, LocalPort: ipsec.PortSelector{Form: ipsec.PortSingle, Port: inStepDiscardPort},
			RemotePrefix: peer, RemotePort: anyPort,
		},
	}
	installSPDPolicies(dp, nil, entries, log)
	t.Cleanup(func() { removeSPDPolicies(dp, entries, log) })
	if err := installUnmatched(dp, unmatched, log); err != nil {
		t.Fatalf("install the catch-all: %v", err)
	}
	t.Cleanup(func() { removeUnmatched(dp, log) })
}

// inStepSendClear sends one clear UDP datagram from the peer address to a local port.
func inStepSendClear(t *testing.T, targetPort int) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(inStepPeer)})
	if err != nil {
		t.Fatalf("bind %s: %v", inStepPeer, err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close sender: %v", err)
		}
	}()
	target := &net.UDPAddr{IP: net.ParseIP(sadSelInnerLocal), Port: targetPort}
	if _, err := conn.WriteToUDP([]byte("ze-rfc4301-inbound-step"), target); err != nil {
		t.Fatalf("send to %s: %v", target, err)
	}
}

// inStepInside returns an inner packet inside the Child SA's selectors: UDP from the peer's
// inner address to local port 5000.
func inStepInside() []byte {
	return sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort))
}

// TestRFC4301InboundESPIsProcessedWithTheSAItsSPISelects proves the positive of RFC 4301
// Section 5.2 steps 3a and 4. Method: a tunnel-mode Child SA installed by
// createFirstChildSA; an ESP packet carrying its inbound SPI, sealed with the key the
// kernel state holds, around an inner packet inside the selectors, must be found in the
// SAD, decrypted with that SA, and delivered, with no SAD miss and no integrity failure
// counted.
func TestRFC4301InboundESPIsProcessedWithTheSAItsSPISelects(t *testing.T) {
	// RFC requirement: RFC4301-5.2-6 positive -- an ESP packet addressed to Ze carrying the
	// inbound SPI of a Child SA Ze installed is found in the SAD (no XfrmInNoStates).
	// RFC requirement: RFC4301-5.2-7 positive -- an ESP packet whose SPI the SAD holds is not
	// discarded as a SAD miss: its inner packet is delivered.
	// RFC requirement: RFC4301-5.2-10 positive -- the packet is ESP-processed with the SA its SPI
	// selected: decrypted with that SA's key and delivered, no XfrmInStateProtoError.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)

	noStates := sadSelStat(t, inStepStatNoStates)
	protoErrors := sadSelStat(t, inStepStatProtoError)
	sender.send(t, sadSelProtoIPv4, inStepInside())
	if !sadSelDelivered(t, inside) {
		t.Fatal("an ESP packet on the Child SA's inbound SPI was not delivered")
	}
	if got := sadSelStat(t, inStepStatNoStates) - noStates; got != 0 {
		t.Fatalf("%s rose by %d: the SAD lookup missed the Child SA's inbound SPI", inStepStatNoStates, got)
	}
	if got := sadSelStat(t, inStepStatProtoError) - protoErrors; got != 0 {
		t.Fatalf("%s rose by %d: the packet failed ESP processing with its own SA", inStepStatProtoError, got)
	}
}

// TestRFC4301InboundESPWithNoSADMatchIsDiscarded proves the negative of RFC 4301 Section
// 5.2 step 3a. Method: the Child SA of the positive unit; an ESP packet sealed with the
// same key around the same inner packet, but carrying an SPI no SA holds, must be counted
// as XfrmInNoStates and never delivered.
func TestRFC4301InboundESPWithNoSADMatchIsDiscarded(t *testing.T) {
	// RFC requirement: RFC4301-5.2-6 negative -- the SAD lookup is keyed on the SPI: the same
	// packet under an SPI Ze installed no SA for finds nothing (XfrmInNoStates rises by one).
	// RFC requirement: RFC4301-5.2-7 negative -- an ESP packet whose SPI matches no SAD entry is
	// discarded: counted as XfrmInNoStates and never delivered.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)

	unknown := *sender
	unknown.spi = child.InboundSPI ^ 0x5a5a5a5a
	noStates := sadSelStat(t, inStepStatNoStates)
	unknown.send(t, sadSelProtoIPv4, inStepInside())
	if sadSelDelivered(t, inside) {
		t.Fatal("an ESP packet whose SPI no SA holds was delivered")
	}
	if got := sadSelStat(t, inStepStatNoStates) - noStates; got != 1 {
		t.Fatalf("%s rose by %d on an unknown SPI, want 1", inStepStatNoStates, got)
	}
}

// TestRFC4301InboundESPFailingIntegrityIsDiscarded proves the negative of RFC 4301 Section
// 5.2 step 4. Method: the Child SA of the positive unit; an ESP packet carrying its inbound
// SPI and a fresh sequence number but sealed with another key must fail ESP processing with
// that SA (XfrmInStateProtoError) and never be delivered.
func TestRFC4301InboundESPFailingIntegrityIsDiscarded(t *testing.T) {
	// RFC requirement: RFC4301-5.2-10 negative -- ESP processing uses the selected SA's key: a
	// packet on its SPI sealed with another key fails the ICV (XfrmInStateProtoError rises by
	// one) and is never delivered.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)

	block, err := aes.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	wrongKey, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	forged := *sender
	forged.aead = wrongKey
	forged.seq = sender.seq + 100
	protoErrors := sadSelStat(t, inStepStatProtoError)
	forged.send(t, sadSelProtoIPv4, inStepInside())
	if sadSelDelivered(t, inside) {
		t.Fatal("an ESP packet failing the SA's integrity check was delivered")
	}
	if got := sadSelStat(t, inStepStatProtoError) - protoErrors; got != 1 {
		t.Fatalf("%s rose by %d on a forged ICV, want 1", inStepStatProtoError, got)
	}
}

// TestRFC4301InboundClearPacketIsBypassedByTheSPDIEntryItMatches proves the positive of
// RFC 4301 Section 5.2 step 3b. Method: the SPD-I of inStepSPDI with a discarding
// catch-all; a clear UDP datagram from 10.3.0.5 to local port 7101, which the operator
// BYPASS entry names, must be delivered with no policy block counted.
func TestRFC4301InboundClearPacketIsBypassedByTheSPDIEntryItMatches(t *testing.T) {
	// RFC requirement: RFC4301-5.2-8 positive -- a packet addressed to Ze that is not ESP is
	// looked up in the SPD-I Ze installs, and one matching an operator BYPASS entry, ranked
	// ahead of a discarding catch-all, is delivered.
	if !sadSelOwnNamespace(t) {
		return
	}
	inStepNetns(t)
	inStepSPDI(t, dataplane.SPActionDiscard)
	bypassed := sadSelListen(t, sadSelInnerLocal, inStepBypassPort)

	blocked := sadSelStat(t, inStepStatPolBlock)
	inStepSendClear(t, inStepBypassPort)
	if !sadSelDelivered(t, bypassed) {
		t.Fatal("a clear datagram matching the operator BYPASS entry was not delivered")
	}
	if got := sadSelStat(t, inStepStatPolBlock) - blocked; got != 0 {
		t.Fatalf("%s rose by %d on the BYPASS entry", inStepStatPolBlock, got)
	}
}

// TestRFC4301InboundClearPacketIsDiscardedByTheSPDIEntryItMatches proves the negative of
// RFC 4301 Section 5.2 step 3b. Method: the SPD-I of inStepSPDI with a BYPASS catch-all,
// so only the operator entry can discard; a clear UDP datagram to local port 7102, which the
// operator DISCARD entry names, must be counted as XfrmInPolBlock and never delivered.
func TestRFC4301InboundClearPacketIsDiscardedByTheSPDIEntryItMatches(t *testing.T) {
	// RFC requirement: RFC4301-5.2-8 negative -- a packet addressed to Ze that is not ESP and
	// matches an operator DISCARD entry of the SPD-I is discarded (XfrmInPolBlock rises by
	// one) and never delivered, though the catch-all would bypass it.
	if !sadSelOwnNamespace(t) {
		return
	}
	inStepNetns(t)
	inStepSPDI(t, dataplane.SPActionBypass)
	discarded := sadSelListen(t, sadSelInnerLocal, inStepDiscardPort)

	blocked := sadSelStat(t, inStepStatPolBlock)
	inStepSendClear(t, inStepDiscardPort)
	if sadSelDelivered(t, discarded) {
		t.Fatal("a clear datagram matching the operator DISCARD entry was delivered")
	}
	if got := sadSelStat(t, inStepStatPolBlock) - blocked; got != 1 {
		t.Fatalf("%s rose by %d on the DISCARD entry, want 1", inStepStatPolBlock, got)
	}
}

// TestRFC4301InboundClearPacketMatchingNoEntryFollowsTheUnmatchedLeaf pins the step 3b
// sentence "If there is no match, discard the traffic." to the `vpn ipsec unmatched` leaf.
// It carries no RFC requirement tag: RFC4301-5.2-9 is a {gap}, because the leaf defaults to
// bypass (the RFC4301-4.4.1-3 deviation). Method: a clear datagram to local port 7109,
// which no operator entry names; with `unmatched discard` it must be counted as
// XfrmInPolBlock and never delivered, and with the default bypass it is delivered.
func TestRFC4301InboundClearPacketMatchingNoEntryFollowsTheUnmatchedLeaf(t *testing.T) {
	if !sadSelOwnNamespace(t) {
		return
	}
	inStepNetns(t)
	inStepSPDI(t, dataplane.SPActionDiscard)
	stray := sadSelListen(t, sadSelInnerLocal, inStepStrayPort)

	blocked := sadSelStat(t, inStepStatPolBlock)
	inStepSendClear(t, inStepStrayPort)
	if sadSelDelivered(t, stray) {
		t.Fatal("with unmatched discard, a clear datagram no SPD-I entry names was delivered")
	}
	if got := sadSelStat(t, inStepStatPolBlock) - blocked; got != 1 {
		t.Fatalf("with unmatched discard, %s rose by %d, want 1", inStepStatPolBlock, got)
	}

	// The default: installUnmatched replaces the catch-all under the same selector.
	if err := installUnmatched(dataplane.Get(), dataplane.SPActionBypass, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("install the catch-all: %v", err)
	}
	inStepSendClear(t, inStepStrayPort)
	if !sadSelDelivered(t, stray) {
		t.Fatal("with the default unmatched bypass, a clear datagram no SPD-I entry names was not delivered")
	}
}
