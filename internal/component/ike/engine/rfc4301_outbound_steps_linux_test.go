// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA install into the dataplane
// Related: rfc4301_sad_selector_linux_test.go -- the namespace probe, Child SA install and listeners reused here
// Related: spd_policy.go -- spdPolicyDirection, the operator BYPASS and DISCARD entries
// Related: unmatched.go -- unmatchedPolicies, the catch-all ranked last
//
// VALIDATES: against a real Linux XFRM stack, the outbound steps of RFC 4301 Section 5.1
// on the SPD Ze installs: each outbound packet's headers are matched against the SPD, and
// the packet is then processed as the matching entry says. A packet matching a Child SA's
// PROTECT entry leaves as ESP under that SA's SPI, one matching an operator BYPASS entry
// leaves in the clear, one matching an operator DISCARD entry is dropped, and one whose
// headers match no entry falls to the catch-all.
// PREVENTS: an outbound entry Ze installs whose disposition the kernel does not apply, so
// traffic the configuration protects leaves in the clear, or traffic it discards leaves.
//
// No privilege is needed: sadSelOwnNamespace re-runs the unit in a user and network
// namespace of its own, as its sibling file explains. Linux keeps no SPD cache (the flow
// cache was removed in Linux 4.14): every outbound packet is looked up in the SPD itself,
// so the cache hit of step 3a and the SPD search of step 3b are the same kernel lookup,
// and the probe proves the external behavior both steps describe.

//go:build linux

package engine

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	outStepProtectTarget = "10.1.0.5"
	outStepBypassTarget  = "10.3.0.5"
	outStepDiscardTarget = "10.4.0.5"
	outStepProtectPort   = 7000
	outStepBypassPort    = 7001
	outStepDiscardPort   = 7002
	outStepStrayPort     = 7009

	// outStepOperatorRank is an operator entry order ahead of the catch-all
	// (dataplane.PriorityUnmatched), so each operator entry is searched first.
	outStepOperatorRank = 1000

	outStepStatPolBlock = "XfrmOutPolBlock"
	outStepStatNoStates = "XfrmOutNoStates"
)

// outStepNetns brings loopback up with every address the probe uses, lets packets routed
// over it take the outbound policy lookup, and routes the protected remote range out of it.
func outStepNetns(t *testing.T) {
	t.Helper()
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}
	// Linux creates loopback with disable_xfrm set, so a packet routed over lo skips the
	// outbound policy lookup entirely (DST_NOXFRM on its route). Every other interface
	// defaults to 0, which is the setting outbound traffic meets, so the probe clears it.
	// Left set, every packet below would leave in the clear whatever the SPD said.
	// disable_policy stays set: the inbound check is not what this probe measures.
	if err := os.WriteFile("/proc/sys/net/ipv4/conf/lo/disable_xfrm", []byte("0"), 0o600); err != nil {
		t.Fatalf("clear lo disable_xfrm: %v", err)
	}
	for _, a := range []string{sadSelLocalOuter, sadSelRemoteOuter, sadSelInnerLocal, outStepBypassTarget, outStepDiscardTarget} {
		addr, err := netlink.ParseAddr(a + "/32")
		if err != nil {
			t.Fatalf("parse %s: %v", a, err)
		}
		if err := netlink.AddrAdd(lo, addr); err != nil {
			t.Fatalf("add %s to lo: %v", a, err)
		}
	}
	// The protected range is not local: a packet the PROTECT entry misses is routed out of
	// lo, comes back not addressed to this namespace, and is dropped (forwarding is off).
	route := &netlink.Route{LinkIndex: lo.Attrs().Index, Dst: mustCIDRNet(t, "10.1.0.0/16"), Scope: netlink.SCOPE_LINK}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route 10.1.0.0/16 over lo: %v", err)
	}
}

// outStepSPD installs the outbound SPD the probe measures, through the functions Ze's
// apply path calls: a tunnel-mode Child SA (PROTECT) for UDP from local port 5000 of
// 10.2.0.0/16 to 10.1.0.0/16, an operator BYPASS entry for UDP to port 7001 of 10.3.0.5,
// an operator DISCARD entry for 10.4.0.5, and the catch-all set to discard.
func outStepSPD(t *testing.T) *ChildSA {
	t.Helper()
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	dp := dataplane.Get()
	log := slogutil.DiscardLogger()

	anyPort := ipsec.PortSelector{Form: ipsec.PortAny}
	local := mustCIDRNet(t, sadSelInnerLocal+"/32")
	entries := map[string]ipsec.SPDPolicy{
		"bypass-7001": {
			Name: "bypass-7001", Action: dataplane.SPActionBypass, Order: outStepOperatorRank,
			Direction: ipsec.SPDDirOut, Protocol: sadSelProtoUDP,
			LocalPrefix: local, LocalPort: anyPort,
			RemotePrefix: mustCIDRNet(t, outStepBypassTarget+"/32"),
			RemotePort:   ipsec.PortSelector{Form: ipsec.PortSingle, Port: outStepBypassPort},
		},
		"discard-all": {
			Name: "discard-all", Action: dataplane.SPActionDiscard, Order: outStepOperatorRank + 1,
			Direction:   ipsec.SPDDirOut,
			LocalPrefix: local, LocalPort: anyPort,
			RemotePrefix: mustCIDRNet(t, outStepDiscardTarget+"/32"), RemotePort: anyPort,
		},
	}
	installSPDPolicies(dp, nil, entries, log)
	t.Cleanup(func() { removeSPDPolicies(dp, entries, log) })
	installUnmatched(dp, dataplane.SPActionDiscard, log)
	t.Cleanup(func() { removeUnmatched(dp, log) })
	return child
}

// outStepSend writes one UDP datagram from the local inner address and source port to
// target, and returns the error the kernel gave the write.
func outStepSend(t *testing.T, sourcePort int, target string, targetPort int) error {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(sadSelInnerLocal), Port: sourcePort})
	if err != nil {
		t.Fatalf("bind %s:%d: %v", sadSelInnerLocal, sourcePort, err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close sender: %v", err)
		}
	}()
	_, err = conn.WriteToUDP([]byte("ze-rfc4301-outbound-step"), &net.UDPAddr{IP: net.ParseIP(target), Port: targetPort})
	return err
}

// outStepESP opens a raw ESP socket on the remote tunnel endpoint. A raw socket gets a
// copy of every ESP packet that arrives for its address, before the ESP handler runs.
func outStepESP(t *testing.T) *net.IPConn {
	t.Helper()
	conn, err := net.ListenIP("ip4:50", &net.IPAddr{IP: net.ParseIP(sadSelRemoteOuter)})
	if err != nil {
		t.Fatalf("raw ESP socket: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close raw ESP socket: %v", err)
		}
	})
	return conn
}

// outStepESPSPI returns the SPI of the next ESP packet the raw socket reads, and false
// when none arrives within a short wait.
func outStepESPSPI(t *testing.T, conn *net.IPConn) (uint32, bool) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 512)
	n, _, err := conn.ReadFromIP(buf)
	if err != nil {
		return 0, false
	}
	if n < 8 {
		t.Fatalf("ESP packet of %d octets is shorter than its SPI and sequence number", n)
	}
	return binary.BigEndian.Uint32(buf[0:4]), true
}

// TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches proves the PROTECT and
// BYPASS halves of RFC 4301 Section 5.1 steps 2, 3a and 3b on the SPD Ze installs. Method:
// the SPD of outStepSPD; a datagram from port 5000 to 10.1.0.5 (the Child SA's PROTECT
// entry) must reach the remote tunnel endpoint as ESP carrying the Child SA's outbound SPI,
// with no state lookup failing, and must not be sent in the clear; a datagram to port 7001
// of 10.3.0.5 (the operator BYPASS entry, ranked ahead of the discarding catch-all) must be
// delivered in the clear.
func TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches(t *testing.T) {
	// RFC requirement: RFC4301-5.1-4 positive -- each outbound packet's headers are matched
	// against the SPD Ze installs: two datagrams from one source, differing in destination and
	// port, meet the Child SA's PROTECT entry and the operator BYPASS entry respectively.
	// RFC requirement: RFC4301-5.1-5 positive -- a packet matching the Child SA's PROTECT entry
	// leaves as ESP under that SA's outbound SPI, and one matching an operator BYPASS entry is
	// delivered in the clear.
	// RFC requirement: RFC4301-5.1-6 positive -- the first packet of each flow, which no cache
	// can hold, is processed by the SPD entry its headers select.
	if !sadSelOwnNamespace(t) {
		return
	}
	outStepNetns(t)
	child := outStepSPD(t)
	esp := outStepESP(t)
	bypassed := sadSelListen(t, outStepBypassTarget, outStepBypassPort)

	noStates := sadSelStat(t, outStepStatNoStates)
	if err := outStepSend(t, sadSelPort, outStepProtectTarget, outStepProtectPort); err != nil {
		t.Fatalf("send on the PROTECT entry: %v", err)
	}
	spi, sent := outStepESPSPI(t, esp)
	if !sent {
		t.Fatal("a datagram matching the Child SA's PROTECT entry did not reach the tunnel endpoint as ESP")
	}
	if spi != child.OutboundSPI {
		t.Fatalf("the ESP packet carries SPI %#x, want the Child SA's outbound SPI %#x", spi, child.OutboundSPI)
	}
	if got := sadSelStat(t, outStepStatNoStates) - noStates; got != 0 {
		t.Fatalf("XfrmOutNoStates rose by %d: the PROTECT entry did not resolve to the Child SA's state", got)
	}

	if err := outStepSend(t, 0, outStepBypassTarget, outStepBypassPort); err != nil {
		t.Fatalf("send on the BYPASS entry: %v", err)
	}
	if !sadSelDelivered(t, bypassed) {
		t.Fatal("a datagram matching the operator BYPASS entry was not delivered in the clear")
	}
	if _, again := outStepESPSPI(t, esp); again {
		t.Fatal("the BYPASS datagram was sent as ESP")
	}
}

// TestRFC4301OutboundPacketIsDiscardedByTheEntryItMatches proves the DISCARD half of RFC
// 4301 Section 5.1 steps 2, 3a and 3b on the SPD Ze installs. Method: the SPD of
// outStepSPD; a datagram to 10.4.0.5 (the operator DISCARD entry) must be refused at the
// write, counted as XfrmOutPolBlock and never delivered; a datagram to 10.3.0.5 on port
// 7009, which differs from the BYPASS entry only by its destination port, must match no
// operator entry, fall to the discarding catch-all, and never be delivered.
func TestRFC4301OutboundPacketIsDiscardedByTheEntryItMatches(t *testing.T) {
	// RFC requirement: RFC4301-5.1-4 negative -- the match reads the packet's headers, not its
	// destination alone: a datagram to the BYPASS entry's address on another port is not
	// bypassed, and the catch-all discards it.
	// RFC requirement: RFC4301-5.1-5 negative -- a packet matching an operator DISCARD entry is
	// refused at the write, counted as XfrmOutPolBlock, and never delivered.
	// RFC requirement: RFC4301-5.1-6 negative -- a packet whose headers select no operator entry
	// takes the disposition of the SPD's catch-all entry, not that of a neighboring entry.
	if !sadSelOwnNamespace(t) {
		return
	}
	outStepNetns(t)
	outStepSPD(t)
	discarded := sadSelListen(t, outStepDiscardTarget, outStepDiscardPort)
	stray := sadSelListen(t, outStepBypassTarget, outStepStrayPort)

	blocked := sadSelStat(t, outStepStatPolBlock)
	err := outStepSend(t, 0, outStepDiscardTarget, outStepDiscardPort)
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("send on the DISCARD entry returned %v, want EPERM from the blocking policy", err)
	}
	if got := sadSelStat(t, outStepStatPolBlock) - blocked; got != 1 {
		t.Fatalf("XfrmOutPolBlock rose by %d on the DISCARD entry, want 1", got)
	}
	if sadSelDelivered(t, discarded) {
		t.Fatal("a datagram matching the operator DISCARD entry was delivered")
	}

	blocked = sadSelStat(t, outStepStatPolBlock)
	err = outStepSend(t, 0, outStepBypassTarget, outStepStrayPort)
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("send on a port the BYPASS entry does not name returned %v, want EPERM from the catch-all", err)
	}
	if got := sadSelStat(t, outStepStatPolBlock) - blocked; got != 1 {
		t.Fatalf("XfrmOutPolBlock rose by %d on the unmatched port, want 1", got)
	}
	if sadSelDelivered(t, stray) {
		t.Fatal("a datagram on a port the BYPASS entry does not name was delivered")
	}
}
