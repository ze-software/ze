//go:build integration && linux

// Design: docs/guide/flowspec-protected-router.md -- selected packet-rate zero enforcement.
package flowspecfirewall

import (
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft"
)

// TestSelectedFlowSpecZeroPacketRateKernel enters at the selected-route bridge,
// installs literal subtype 0x800c with rate zero, and observes real UDP delivery
// and nft counters. The other destination remains reachable; replacing the same
// rule with a positive packet rate and withdrawing it restore target delivery.
// It does not claim to test RIB authorization, which precedes this boundary.
// RFC 8955 Section 7.2: "A traffic-rate-packets of 0 should result in all traffic
// for the particular flow to be discarded."
// RFC requirement: RFC8955-7.2-3 positive -- selected subtype 0x800c rate zero installs an nft rule that discards four target UDP packets and counts exactly four packets and 116 bytes.
// RFC requirement: RFC8955-7.2-3 negative -- zero packet-rate does not drop another destination, and positive-rate replacement and withdrawal restore target delivery.
func TestSelectedFlowSpecZeroPacketRateKernel(t *testing.T) {
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("network namespace unavailable: %v", err)
	}
	namespace, err := netns.New()
	if err != nil {
		if closeErr := original.Close(); closeErr != nil {
			t.Errorf("close namespace handle: %v", closeErr)
		}
		runtime.UnlockOSThread()
		t.Skipf("requires CAP_SYS_ADMIN and CAP_NET_ADMIN: %v", err)
	}
	t.Cleanup(func() {
		if err := firewall.RegisterTables("flowspec", nil); err != nil {
			t.Errorf("remove FlowSpec tables: %v", err)
		}
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("close nft backend: %v", err)
		}
		if err := netns.Set(original); err != nil {
			t.Errorf("restore namespace: %v", err)
		}
		if err := namespace.Close(); err != nil {
			t.Errorf("close test namespace: %v", err)
		}
		if err := original.Close(); err != nil {
			t.Errorf("close original namespace: %v", err)
		}
		runtime.UnlockOSThread()
	})
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatal(err)
	}
	if err := firewall.LoadBackend("nft"); err != nil {
		t.Fatal(err)
	}
	family.RegisterTestFamilies()
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := receiver.Close(); err != nil {
			t.Error(err)
		}
	})
	address, ok := receiver.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("receiver address is %T", receiver.LocalAddr())
	}
	target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: address.Port}
	control := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 3), Port: address.Port}
	sender, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sender.Close(); err != nil {
			t.Error(err)
		}
	})
	send := func(to *net.UDPAddr, token byte) {
		t.Helper()
		if n, err := sender.WriteToUDP([]byte{token}, to); err != nil || n != 1 {
			t.Fatalf("send token %x: n=%d err=%v", token, n, err)
		}
	}
	receive := func(token byte) {
		t.Helper()
		if err := receiver.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		var data [8]byte
		n, _, err := receiver.ReadFromUDP(data[:])
		if err != nil {
			t.Fatalf("receive token %x: %v", token, err)
		}
		if n != 1 || data[0] != token {
			t.Fatalf("received %x, want token %x", data[:n], token)
		}
	}
	send(target, 1)
	receive(1)
	send(control, 2)
	receive(2)

	b := testBridge(t)
	t.Cleanup(func() { b.stopped = true })
	port := address.Port
	change := ribevents.FlowSpecChange{
		Family:              family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
		NLRI:                []byte{13, 1, 32, 127, 0, 0, 1, 3, 0x81, 17, 5, 0x91, byte(port >> 8), byte(port)},
		ExtendedCommunities: []byte{0x80, 0x0c, 0, 0, 0, 0, 0, 0},
	}
	// RFC 8955 Section 7.2: only this selected packet-rate action programs nft.
	b.handleSelected(&change)
	for range 4 {
		send(target, 3)
	}
	for range 4 {
		send(control, 4)
	}
	for range 4 {
		receive(4) // A missing discard would deliver the earlier target tokens.
	}
	if err := receiver.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var data [8]byte
	if n, _, err := receiver.ReadFromUDP(data[:]); err == nil {
		t.Fatalf("zero-rate target leaked packet %x", data[:n])
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("receive failed for a reason other than packet discard: %v", err)
	}
	counters, err := firewall.GetBackend().GetCounters(tableName)
	if err != nil {
		t.Fatal(err)
	}
	var packets, octets uint64
	var inputTerms int
	for _, chain := range counters {
		if chain.Chain != "flowspec-in" {
			continue
		}
		inputTerms += len(chain.Terms)
		for _, term := range chain.Terms {
			packets += term.Packets
			octets += term.Bytes
		}
	}
	if inputTerms != 1 || packets != 4 || octets != 4*29 {
		t.Fatalf("installed input terms=%d packets=%d bytes=%d, want 1/4/116; control packets must not match", inputTerms, packets, octets)
	}

	// A positive rate is not an unconditional drop merely because it uses 0x800c.
	change.ExtendedCommunities = []byte{0x80, 0x0c, 0, 0, 0x3f, 0x80, 0, 0}
	b.handleSelected(&change)
	send(target, 5)
	receive(5)

	// Restore and observe an active zero-rate discard before withdrawal.
	// Withdrawing the positive limiter alone cannot distinguish removal from
	// reinstallation with a fresh token that happens to admit one packet.
	// RFC 8955 Section 7.2: the selected zero packet-rate must discard again.
	change.ExtendedCommunities = []byte{0x80, 0x0c, 0, 0, 0, 0, 0, 0}
	b.handleSelected(&change)
	send(target, 7)
	send(control, 8)
	receive(8)
	if err := receiver.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if n, _, err := receiver.ReadFromUDP(data[:]); err == nil {
		t.Fatalf("reinstalled zero-rate target leaked packet %x", data[:n])
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("reinstalled discard probe failed unexpectedly: %v", err)
	}
	change.Withdraw = true
	b.handleSelected(&change)
	send(target, 6)
	receive(6)
}
