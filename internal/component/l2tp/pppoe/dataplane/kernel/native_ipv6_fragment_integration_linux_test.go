//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- original IPv6 carrier custody
// Related: native_ipv6_carrier_fixture_integration_linux_test.go -- real PPPoE wire
// Related: native_ipv6_fragment_readiness_r2_integration_linux_test.go -- FIFO and route readiness
// RFC 8200 Section 4.5 -- fragment reassembly, text in rfc/full/rfc8200.txt
// Additive current-API tests. These tests do not activate a kernel patch.
package kernel

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestNativeIPv6FragmentUseful delivers an actual fragmented UDP datagram
// through the admitted PPP owner. A destination-options header stays in the
// native IPv6 receive path; the test never parses or reassembles its input.
func TestNativeIPv6FragmentUseful(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeV6FragmentOrdering(t)
	f := newNativeV6Fixture(t)
	nativeV6FragmentIngress(t, f.route, f.access.Device())
	opened, endpoint := f.open(t, 821)
	nativeV6FragmentGlobal(t, f, opened)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 91)
	nativeV6FragmentVector(t)
	before := nativeV6FragmentCounters(t)
	body := nativeV6FragmentUDP(92)
	endpoint.inject(t, nativeV6FragmentPacket(t, 0x10203040, body, 0, 16, true, true))
	nativeV6FragmentBarrier(t, carrier, endpoint, 120)
	nativeV6FragmentQueued(t, before)
	endpoint.inject(t, nativeV6FragmentPacket(t, 0x10203040, body, 16, len(body), false, true))
	nativeV6FragmentRead(t, carrier, 92)
	nativeV6FragmentCompleted(t, before, 1)
	nativeV6Useful(t, carrier, endpoint, 93)
}

// TestNativeIPv6FragmentDistinctOwners gives two live PPP units the exact same
// global tuple and fragment ID. Both halves contain bytes from one identical
// checksum-valid datagram: a cross-owner completion cannot hide as bad UDP.
func TestNativeIPv6FragmentDistinctOwners(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeV6FragmentOrdering(t)
	f := newNativeV6Fixture(t)
	nativeV6FragmentIngress(t, f.route, f.access.Device())
	first, firstEndpoint := f.open(t, 822)
	second, secondEndpoint := f.open(t, 823)
	if first.Dataplane().Identity().NativeIndex == second.Dataplane().Identity().NativeIndex {
		t.Fatal("fragment ownership control did not create distinct live PPP units")
	}
	nativeV6FragmentGlobal(t, f, first)
	nativeV6FragmentGlobal(t, f, second)
	firstCarrier := nativeV6Acquire(t, first)
	secondCarrier := nativeV6Acquire(t, second)
	nativeV6Useful(t, firstCarrier, firstEndpoint, 94)
	nativeV6Useful(t, secondCarrier, secondEndpoint, 95)
	before := nativeV6FragmentCounters(t)
	body := nativeV6FragmentUDP(96)
	firstEndpoint.inject(t, nativeV6FragmentPacket(t, 0x20304050, body, 0, 16, true, false))
	nativeV6FragmentBarrier(t, firstCarrier, firstEndpoint, 121)
	nativeV6FragmentQueued(t, before)
	secondEndpoint.inject(t, nativeV6FragmentPacket(t, 0x20304050, body, 16, len(body), false, false))
	nativeV6FragmentRequests(t, before.requests+2)
	// Each real socket must receive its later complete control first. Receiving
	// marker 96 on either owner fails, even though its checksum would be valid.
	nativeV6FragmentBarrier(t, firstCarrier, firstEndpoint, 97)
	nativeV6FragmentBarrier(t, secondCarrier, secondEndpoint, 98)
	nativeV6FragmentUnchanged(t, before)
	firstEndpoint.inject(t, nativeV6FragmentPacket(t, 0x20304050, body, 16, len(body), false, false))
	secondEndpoint.inject(t, nativeV6FragmentPacket(t, 0x20304050, body, 0, 16, true, false))
	nativeV6FragmentRead(t, firstCarrier, 96)
	nativeV6FragmentRead(t, secondCarrier, 96)
	nativeV6FragmentCompleted(t, before, 2)
	nativeV6Useful(t, firstCarrier, firstEndpoint, 99)
	nativeV6Useful(t, secondCarrier, secondEndpoint, 100)
}

// TestNativeIPv6FragmentRetiredOwner queues the first half on real PPP, then
// gives its exact native index and name to useful ordinary Ethernet. The old
// queue must neither complete on the replacement nor satisfy the stale carrier.
func TestNativeIPv6FragmentRetiredOwner(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeV6FragmentOrdering(t)
	f := newNativeV6Fixture(t)
	nativeV6FragmentIngress(t, f.route, f.access.Device())
	opened, endpoint := f.open(t, 824)
	nativeV6FragmentGlobal(t, f, opened)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 101)
	before := nativeV6FragmentCounters(t)
	body := nativeV6FragmentUDP(102)
	endpoint.inject(t, nativeV6FragmentPacket(t, 0x30405060, body, 0, 16, true, false))
	nativeV6FragmentBarrier(t, carrier, endpoint, 122)
	nativeV6FragmentQueued(t, before)
	replacement := f.retire(t, opened)
	nativeV6FragmentEthernetGlobal(t, f.route, replacement.link)
	replacement.useful(t, 103)
	replacement.endpoint.inject(t, nativeV6FragmentPacket(t, 0x30405060, body, 16, len(body), false, false))
	nativeV6FragmentRequests(t, before.requests+2)
	nativeV6FragmentReplacementBarrier(t, replacement, 104)
	nativeV6FragmentUnchanged(t, before)
	ctx, cancel := context.WithTimeout(t.Context(), nativeV6Wait)
	defer cancel()
	var buffer [128]byte
	if n, peer, err := carrier.ReadDHCPv6(ctx, buffer[:]); err == nil {
		t.Fatalf("retired carrier accepted global replacement payload %x from %s", buffer[:n], peer)
	}
	// The replacement's own first half completes its independently held last
	// half. It must not need another ID, another tuple, or a new listener.
	replacement.endpoint.inject(t, nativeV6FragmentPacket(t, 0x30405060, body, 0, 16, true, false))
	nativeV6FragmentReplacementRead(t, replacement, 102)
	nativeV6FragmentCompleted(t, before, 1)
	replacement.useful(t, 105)
}

// TestNativeIPv6FragmentRename keeps one queue on the same real PPP object
// across a physical rename while the old name belongs to useful Ethernet.
func TestNativeIPv6FragmentRename(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeV6FragmentOrdering(t)
	f := newNativeV6Fixture(t)
	nativeV6FragmentIngress(t, f.route, f.access.Device())
	opened, endpoint := f.open(t, 825)
	nativeV6FragmentGlobal(t, f, opened)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 106)
	before := nativeV6FragmentCounters(t)
	body := nativeV6FragmentUDP(107)
	endpoint.inject(t, nativeV6FragmentPacket(t, 0x40506070, body, 0, 16, true, false))
	nativeV6FragmentBarrier(t, carrier, endpoint, 123)
	nativeV6FragmentQueued(t, before)
	replacement := f.rename(t, opened)
	nativeV6FragmentIngress(t, f.route, replacement.link.Attrs().Name)
	replacement.useful(t, 108)
	endpoint.inject(t, nativeV6FragmentPacket(t, 0x40506070, body, 16, len(body), false, false))
	nativeV6FragmentRead(t, carrier, 107)
	nativeV6FragmentCompleted(t, before, 1)
	nativeV6Useful(t, carrier, endpoint, 109)
	replacement.useful(t, 110)
}

// TestNativeIPv6FragmentOrdinaryEthernet preserves untagged native IPv6
// reassembly on a real veth with independent ordinary UDP sockets.
func TestNativeIPv6FragmentOrdinaryEthernet(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeV6FragmentOrdering(t)
	nativeV6Defaults(t)
	route, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(route.Close)
	replacement := nativeV6ReplacementNew(t, route, "v6ordinary", 0)
	nativeV6FragmentEthernetGlobal(t, route, replacement.link)
	replacement.useful(t, 111)
	before := nativeV6FragmentCounters(t)
	body := nativeV6FragmentUDP(112)
	replacement.endpoint.inject(t, nativeV6FragmentPacket(t, 0x50607080, body, 0, 16, true, true))
	nativeV6FragmentReplacementBarrier(t, replacement, 124)
	nativeV6FragmentQueued(t, before)
	replacement.endpoint.inject(t, nativeV6FragmentPacket(t, 0x50607080, body, 16, len(body), false, true))
	nativeV6FragmentReplacementRead(t, replacement, 112)
	nativeV6FragmentCompleted(t, before, 1)
	replacement.useful(t, 113)
}

func nativeV6FragmentGlobal(t *testing.T, f *nativeV6Fixture, opened dataplane.OpenedSession) {
	t.Helper()
	handle := opened.Dataplane()
	if err := handle.AddAddressP2P(netip.MustParsePrefix("2001:db8:6::1/128"), netip.MustParsePrefix("2001:db8:6::2/128")); err != nil {
		t.Fatalf("install original global IPv6 pair through public data plane: %v", err)
	}
	if err := handle.Connect(); err != nil {
		t.Fatal(err)
	}
	link, err := f.route.LinkByIndex(int(handle.Identity().NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	nativeV6FragmentTupleReady(t, f.route, link)
}

func nativeV6FragmentEthernetGlobal(t *testing.T, route *netlink.Handle, link netlink.Link) {
	t.Helper()
	nativeV6FragmentIngress(t, route, link.Attrs().Name)
	address, err := netlink.ParseAddr("2001:db8:6::1/128")
	if err != nil {
		t.Fatal(err)
	}
	address.Flags = unix.IFA_F_NODAD
	peer, err := netlink.ParseAddr("2001:db8:6::2/128")
	if err != nil {
		t.Fatal(err)
	}
	address.Peer = peer.IPNet
	if err := route.AddrAdd(link, address); err != nil {
		t.Fatal(err)
	}
	nativeV6FragmentTupleReady(t, route, link)
}

func nativeV6FragmentUDP(marker byte) []byte {
	payload := nativeV6DHCP(1, marker)
	body := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint16(body[:2], 546)
	binary.BigEndian.PutUint16(body[2:4], 547)
	binary.BigEndian.PutUint16(body[4:6], uint16(len(body)))
	copy(body[8:], payload)
	checksum := nativeV6Checksum(netip.MustParseAddr("2001:db8:6::2"), netip.MustParseAddr("2001:db8:6::1"), 17, body)
	if checksum == 0 {
		checksum = 0xffff
	}
	binary.BigEndian.PutUint16(body[6:8], checksum)
	return body
}

// Offset and M are controlled independently from the complete UDP checksum.
// The optional eight-byte Destination Options header precedes Fragment and
// contains only Pad1 options; the kernel owns extension-chain validation.
func nativeV6FragmentPacket(t *testing.T, id uint32, body []byte, start, end int, more, options bool) []byte {
	t.Helper()
	if start < 0 {
		t.Fatal("negative test fragment offset")
	}
	if start%8 != 0 {
		t.Fatal("unaligned test fragment offset")
	}
	if end <= start {
		t.Fatal("empty test fragment")
	}
	if end > len(body) {
		t.Fatal("test fragment exceeds its UDP datagram")
	}
	if more {
		if (end-start)%8 != 0 {
			t.Fatal("non-final test fragment is not eight-byte aligned")
		}
	}
	header := 8
	next := byte(44)
	if options {
		header = 16
		next = 60
	}
	payload := make([]byte, header+end-start)
	fragment := payload[:8]
	if options {
		payload[0] = 44
		fragment = payload[8:16]
	}
	fragment[0] = 17
	bits := uint16(start)
	if more {
		bits |= 1
	}
	binary.BigEndian.PutUint16(fragment[2:4], bits)
	binary.BigEndian.PutUint32(fragment[4:8], id)
	copy(payload[header:], body[start:end])
	return nativeV6Packet(netip.MustParseAddr("2001:db8:6::2"), netip.MustParseAddr("2001:db8:6::1"), next, 64, payload)
}

func nativeV6FragmentVector(t *testing.T) {
	t.Helper()
	nativeV6CheckVector(t)
	body := nativeV6FragmentUDP(92)
	// Independently summed global pseudoheader and complete UDP datagram:
	// 0x2dc1 + 0x2dc0 + 32 + 17 + 0x0ddc = 0x698e; complement 0x9671.
	want := []byte{2, 0x22, 2, 0x23, 0, 32, 0x96, 0x71,
		1, 0, 0, 92, 0, 1, 0, 10, 0, 3, 0, 1, 2, 0, 6, 0, 0, 2,
		0, 8, 0, 2, 0, 0}
	if !bytes.Equal(body, want) {
		t.Fatalf("global UDP datagram differs from its literal checksum vector: %x", body)
	}
	first := nativeV6FragmentPacket(t, 0x10203040, body, 0, 16, true, true)
	last := nativeV6FragmentPacket(t, 0x10203040, body, 16, len(body), false, true)
	if !bytes.Equal(first[40:56], []byte{44, 0, 0, 0, 0, 0, 0, 0, 17, 0, 0, 1, 0x10, 0x20, 0x30, 0x40}) {
		t.Fatal("first fragment extension header differs from its literal wire vector")
	}
	if !bytes.Equal(last[48:56], []byte{17, 0, 0, 16, 0x10, 0x20, 0x30, 0x40}) {
		t.Fatal("last fragment header differs from its literal wire vector")
	}
}

func nativeV6FragmentRead(t *testing.T, carrier ppp.IPv6Carrier, marker byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), nativeV6Wait)
	defer cancel()
	var buffer [128]byte
	n, peer, err := carrier.ReadDHCPv6(ctx, buffer[:])
	if err != nil {
		t.Fatalf("original carrier did not receive useful global DHCPv6: %v", err)
	}
	nativeV6FragmentPayload(t, buffer[:n], peer, marker)
}

func nativeV6FragmentReplacementRead(t *testing.T, replacement *nativeV6Replacement, marker byte) {
	t.Helper()
	if err := replacement.dhcp.SetReadDeadline(time.Now().Add(nativeV6Wait)); err != nil {
		t.Fatal(err)
	}
	var buffer [128]byte
	n, address, err := replacement.dhcp.ReadFrom(buffer[:])
	if err != nil {
		t.Fatalf("independent Ethernet did not receive useful global DHCPv6: %v", err)
	}
	peer, ok := address.(*net.UDPAddr)
	if !ok {
		t.Fatalf("ordinary global UDP source=%T", address)
	}
	nativeV6FragmentPayload(t, buffer[:n], peer.AddrPort(), marker)
}

func nativeV6FragmentPayload(t *testing.T, payload []byte, peer netip.AddrPort, marker byte) {
	t.Helper()
	if !bytes.Equal(payload, nativeV6DHCP(1, marker)) {
		t.Fatalf("unexpected global DHCPv6 payload %x, want marker %d", payload, marker)
	}
	if peer.Addr() != netip.MustParseAddr("2001:db8:6::2") {
		t.Fatalf("global DHCPv6 source=%s", peer)
	}
	if peer.Port() != 546 {
		t.Fatalf("global DHCPv6 source port=%d", peer.Port())
	}
}

func nativeV6FragmentBarrier(t *testing.T, carrier ppp.IPv6Carrier, endpoint *nativeV6Endpoint, marker byte) {
	t.Helper()
	endpoint.inject(t, nativeV6Packet(netip.MustParseAddr("2001:db8:6::2"), netip.MustParseAddr("2001:db8:6::1"), 17, 64, nativeV6FragmentUDP(marker)))
	nativeV6FragmentRead(t, carrier, marker)
}

func nativeV6FragmentReplacementBarrier(t *testing.T, replacement *nativeV6Replacement, marker byte) {
	t.Helper()
	replacement.endpoint.inject(t, nativeV6Packet(netip.MustParseAddr("2001:db8:6::2"), netip.MustParseAddr("2001:db8:6::1"), 17, 64, nativeV6FragmentUDP(marker)))
	nativeV6FragmentReplacementRead(t, replacement, marker)
}

type nativeV6FragmentState struct {
	requests uint64
	complete uint64
	queues   uint64
}

// These are per-network-namespace native counters, not test-supplied owners or
// queue IDs. Polling ends on the requested native state; the timer only fails.
func nativeV6FragmentCounters(t *testing.T) nativeV6FragmentState {
	t.Helper()
	data, err := os.ReadFile("/proc/net/snmp6")
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]uint64)
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != "Ip6ReasmReqds" {
			if fields[0] != "Ip6ReasmOKs" {
				continue
			}
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		values[fields[0]] = value
	}
	requests, ok := values["Ip6ReasmReqds"]
	if !ok {
		t.Fatal("native IPv6 reassembly request counter is absent")
	}
	complete, ok := values["Ip6ReasmOKs"]
	if !ok {
		t.Fatal("native IPv6 reassembly completion counter is absent")
	}
	data, err = os.ReadFile("/proc/net/sockstat6")
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] != "FRAG6:" {
			continue
		}
		if fields[1] != "inuse" {
			t.Fatalf("unexpected native FRAG6 counter layout: %q", line)
		}
		queues, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return nativeV6FragmentState{requests: requests, complete: complete, queues: queues}
	}
	t.Fatal("native IPv6 fragment queue counter is absent")
	return nativeV6FragmentState{}
}

func nativeV6FragmentQueued(t *testing.T, before nativeV6FragmentState) {
	t.Helper()
	// Caller first received its later useful UDP barrier on the pinned FIFO
	// receive path. These counters then require the earlier retained queue
	// to remain present; allocation alone is not an insertion acknowledgment.
	nativeV6FragmentPoll(t, func(actual nativeV6FragmentState) bool {
		if actual.requests < before.requests+1 {
			return false
		}
		return actual.queues > before.queues
	})
}

func nativeV6FragmentRequests(t *testing.T, count uint64) {
	t.Helper()
	nativeV6FragmentPoll(t, func(actual nativeV6FragmentState) bool { return actual.requests >= count })
}

func nativeV6FragmentCompleted(t *testing.T, before nativeV6FragmentState, count uint64) {
	t.Helper()
	nativeV6FragmentPoll(t, func(actual nativeV6FragmentState) bool { return actual.complete >= before.complete+count })
	actual := nativeV6FragmentCounters(t)
	if actual.complete != before.complete+count {
		t.Fatalf("native reassembly completions=%d, want exact %d", actual.complete-before.complete, count)
	}
}

func nativeV6FragmentUnchanged(t *testing.T, before nativeV6FragmentState) {
	t.Helper()
	actual := nativeV6FragmentCounters(t)
	if actual.complete != before.complete {
		t.Fatalf("fragments from different native owners completed a datagram: before=%+v after=%+v", before, actual)
	}
}

func nativeV6FragmentPoll(t *testing.T, ready func(nativeV6FragmentState) bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), nativeV6Wait)
	defer cancel()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		actual := nativeV6FragmentCounters(t)
		if ready(actual) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("native IPv6 reassembly state did not reach the required condition: %+v: %v", actual, ctx.Err())
		case <-ticker.C:
		}
	}
}
