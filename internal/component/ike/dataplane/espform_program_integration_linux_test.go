// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: espform_program_linux.go -- espFormFastPathSpec, the program under test
//
// VALIDATES: the hand-assembled fast path program (AC-6 of
// spec-ipsec-dual-form-receive-spd-bypass): a whole bare ESP datagram for a watched
// SPI, from the SA's peer to its local address, comes out as ESP-in-UDP with the
// template's ports, the right lengths and, over IPv4, a valid header checksum, and
// is counted; every other packet comes out byte-identical and uncounted. With the
// firewall gate shut, that same watched datagram comes out byte-identical and
// uncounted too. The program answers TCX_NEXT on every one of them.
// PREVENTS: a wrong offset or byte order in hand-written bytecode (no compiler checks
// it), a rewrite while the gate is shut (a rule that drops ESP would then see UDP),
// a rewrite of a fragment, of a header with options or extension headers, of
// traffic from another source or to another destination, a laundered header
// checksum, a CHECKSUM_COMPLETE sum left stale, and an IPv6 rewrite on a skb whose
// sum would catch the constant UDP checksum.
// METHOD: BPF_PROG_TEST_RUN over crafted frames. The expected bytes are built here
// from first principles, with this file's own one's complement checksum, never by
// calling the code under test. BPF_PROG_TEST_RUN builds the skb through
// eth_type_trans and pushes the Ethernet header back before the program runs
// (net/bpf/test_run.c, bpf_prog_test_run_skb), so only the Ethernet variant runs
// here; loading the collection proves the verifier accepts the L3 variant, and a
// real L3 link proves its rewrite later.
//
// A tag held out of band (vlan_present) has no row: BPF_PROG_TEST_RUN cannot build
// such a skb. convert___skb_to_skb (net/bpf/test_run.c, v7.2) refuses a ctx_in with
// any non-zero octet from the end of mark to the start of priority ("if
// (!range_is_zero(__skb, offsetofend(struct __sk_buff, mark), offsetof(struct
// __sk_buff, priority))) return -EINVAL;"), which covers protocol, vlan_present,
// vlan_tci and vlan_proto, and the skb it builds goes through eth_type_trans alone,
// which leaves a tag in the frame. The in-band rows are refused under this harness by
// the protocol branch too, because eth_type_trans sets the protocol from the frame's
// ether type: they prove the tagged datagram comes out untouched, not which of the
// two tests refused it.

//go:build linux

package dataplane

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"os"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"golang.org/x/sys/unix"
)

// fastPathTCXNext is TCX_NEXT (-1, include/uapi/linux/bpf.h) as BPF_PROG_TEST_RUN
// reports it, a u32.
const fastPathTCXNext = 0xffffffff

// The SA the maps watch, and the values a packet differs by.
const (
	fastPathSPI       = 0x0a0b0c0d
	fastPathOtherSPI  = 0x01020304
	fastPathPeerPort  = 4501 // a translated peer port, so a swap with the local port shows
	fastPathLocalPort = 4500
)

var (
	fastPathPeer4  = netip.MustParseAddr("192.0.2.1")
	fastPathLocal4 = netip.MustParseAddr("192.0.2.2")
	fastPathOther4 = netip.MustParseAddr("192.0.2.99")
	fastPathPeer6  = netip.MustParseAddr("2001:db8::1")
	fastPathLocal6 = netip.MustParseAddr("2001:db8::2")
	fastPathOther6 = netip.MustParseAddr("2001:db8::99")
)

// fastPathGate is the gate a row runs under. Its zero value is no choice, which the
// test refuses, so a row never runs shut by omission and passes for the wrong reason.
type fastPathGate uint8

const (
	fastPathGateUnset fastPathGate = iota
	fastPathGateShut               // the manager wrote espFormFastPathGateClosed
	fastPathGateOpen               // the manager wrote espFormFastPathGateOpen
)

// value is what the manager writes into the gate map for g.
func (g fastPathGate) value(t *testing.T) uint32 {
	t.Helper()
	switch g {
	case fastPathGateShut:
		return espFormFastPathGateClosed
	case fastPathGateOpen:
		return espFormFastPathGateOpen
	case fastPathGateUnset:
		t.Fatal("the row names no gate: set gate to fastPathGateShut or fastPathGateOpen")
	}
	t.Fatalf("the row names gate %d, which is not a gate", g)
	return 0
}

// fastPathCase is one frame and what the program must make of it.
type fastPathCase struct {
	gate  fastPathGate
	name  string
	frame []byte
	flags uint32
	want  []byte // nil: the frame comes out unchanged
	ipv4  uint64 // the rewritten-IPv4 counter's expected step
	ipv6  uint64 // the rewritten-IPv6 counter's expected step
}

// TestESPFormRewriteProgram loads the fast path collection, watches one SPI in each
// family, and runs the Ethernet program over every row of AC-6.
//
//ze:guest-root
func TestESPFormRewriteProgram(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: loading a SchedCLS program and running it through BPF_PROG_TEST_RUN need CAP_BPF and CAP_NET_ADMIN in the initial user namespace, which userns.Enter cannot grant; run `./le test qemu all`")
	}
	if err := rlimit.RemoveMemlock(); err != nil {
		t.Fatalf("remove the memlock rlimit: %v", err)
	}
	coll, err := ebpf.NewCollection(espFormFastPathSpec())
	if err != nil {
		if ve, ok := errors.AsType[*ebpf.VerifierError](err); ok {
			t.Fatalf("the verifier refused the fast path:\n%+v", ve)
		}
		t.Fatalf("load the fast path collection: %v", err)
	}
	t.Cleanup(coll.Close)
	fastPathWatch(t, coll)

	prog := coll.Programs[espFormFastPathEthernetProg]
	counter := coll.Maps[espFormFastPathCounterMap]
	gate := coll.Maps[espFormFastPathGateMap]
	for _, tc := range fastPathCases() {
		t.Run(tc.name, func(t *testing.T) {
			if err := gate.Put(uint32(espFormFastPathGateKey), tc.gate.value(t)); err != nil {
				t.Fatalf("write the gate: %v", err)
			}
			before4 := fastPathCount(t, counter, espFormFastPathSlotIPv4)
			before6 := fastPathCount(t, counter, espFormFastPathSlotIPv6)
			opts := &ebpf.RunOptions{
				Data:    tc.frame,
				DataOut: make([]byte, len(tc.frame)+64),
				Flags:   tc.flags,
			}
			ret, err := prog.Run(opts)
			if err != nil {
				t.Fatalf("BPF_PROG_TEST_RUN: %v", err)
			}
			if ret != fastPathTCXNext {
				t.Errorf("the program answered %#x, want TCX_NEXT (%#x)", ret, uint32(fastPathTCXNext))
			}
			want := tc.want
			if want == nil {
				want = tc.frame
			}
			if !bytes.Equal(opts.DataOut, want) {
				t.Errorf("output frame\n got %x\nwant %x", opts.DataOut, want)
			}
			if got := fastPathCount(t, counter, espFormFastPathSlotIPv4) - before4; got != tc.ipv4 {
				t.Errorf("the IPv4 rewrite counter moved %d, want %d", got, tc.ipv4)
			}
			if got := fastPathCount(t, counter, espFormFastPathSlotIPv6) - before6; got != tc.ipv6 {
				t.Errorf("the IPv6 rewrite counter moved %d, want %d", got, tc.ipv6)
			}
		})
	}
}

// fastPathWatch puts the watched SA of each family into its map, through the layout
// helpers the installer uses.
func fastPathWatch(t *testing.T, coll *ebpf.Collection) {
	t.Helper()
	key := newESPFormFastPathKey(fastPathSPI)
	v4, err := newESPFormFastPathValue4(fastPathPeer4, fastPathLocal4, fastPathPeerPort, fastPathLocalPort)
	if err != nil {
		t.Fatalf("lay out the IPv4 target: %v", err)
	}
	if err := coll.Maps[espFormFastPathWatched4Map].Put(key, v4); err != nil {
		t.Fatalf("watch the IPv4 SPI: %v", err)
	}
	v6, err := newESPFormFastPathValue6(fastPathPeer6, fastPathLocal6, fastPathPeerPort, fastPathLocalPort)
	if err != nil {
		t.Fatalf("lay out the IPv6 target: %v", err)
	}
	if err := coll.Maps[espFormFastPathWatched6Map].Put(key, v6); err != nil {
		t.Fatalf("watch the IPv6 SPI: %v", err)
	}
}

// fastPathCount sums one counter slot over every CPU.
func fastPathCount(t *testing.T, counter *ebpf.Map, slot uint32) uint64 {
	t.Helper()
	var perCPU []uint64
	if err := counter.Lookup(slot, &perCPU); err != nil {
		t.Fatalf("read counter slot %d: %v", slot, err)
	}
	var total uint64
	for _, n := range perCPU {
		total += n
	}
	return total
}

// fastPathCases is AC-6's table. A watched row rewrites; every other row differs
// from a watched one by the one field it names.
func fastPathCases() []fastPathCase {
	esp := fastPathESP(fastPathSPI, 16)
	v4 := fastPathIPv4Header{id: 0x1234, ttl: 64, proto: unix.IPPROTO_ESP, src: fastPathPeer4, dst: fastPathLocal4}
	v6 := fastPathIPv6Header{trafficClass: 0x28, flow: 0xabcde, next: unix.IPPROTO_ESP, hop: 64, src: fastPathPeer6, dst: fastPathLocal6}

	with4 := func(edit func(*fastPathIPv4Header)) fastPathIPv4Header {
		h := v4
		edit(&h)
		return h
	}
	with6 := func(edit func(*fastPathIPv6Header)) fastPathIPv6Header {
		h := v6
		edit(&h)
		return h
	}
	padding := make([]byte, 18)
	shortESP := fastPathESP(fastPathSPI, 0)[:7]
	hopByHop := append([]byte{unix.IPPROTO_ESP, 0, 1, 4, 0, 0, 0, 0}, esp...)
	fragment := append([]byte{unix.IPPROTO_ESP, 0, 0x00, 0x01, 0x12, 0x34, 0x56, 0x78}, esp...)
	arp := make([]byte, 28)

	return []fastPathCase{
		{
			gate:  fastPathGateOpen,
			name:  "ipv4 watched SPI from the peer to the local address",
			frame: fastPathEthernet(unix.ETH_P_IP, v4.packet(esp)),
			want:  fastPathEthernet(unix.ETH_P_IP, fastPathWantIPv4(v4, esp)),
			ipv4:  1,
		},
		{
			gate:  fastPathGateOpen,
			name:  "ipv4 watched SPI with DF set, which is not a fragment",
			frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.flagsFrag = 0x4000 }).packet(esp)),
			want:  fastPathEthernet(unix.ETH_P_IP, fastPathWantIPv4(with4(func(h *fastPathIPv4Header) { h.flagsFrag = 0x4000 }), esp)),
			ipv4:  1,
		},
		{
			gate:  fastPathGateOpen,
			name:  "ipv4 watched SPI on a CHECKSUM_COMPLETE skb keeps the sum right",
			frame: fastPathEthernet(unix.ETH_P_IP, v4.packet(esp)),
			flags: unix.BPF_F_TEST_SKB_CHECKSUM_COMPLETE,
			want:  fastPathEthernet(unix.ETH_P_IP, fastPathWantIPv4(v4, esp)),
			ipv4:  1,
		},
		{
			gate:  fastPathGateOpen,
			name:  "ipv4 watched SPI with Ethernet padding keeps the padding after the datagram",
			frame: append(fastPathEthernet(unix.ETH_P_IP, v4.packet(fastPathESP(fastPathSPI, 0))), padding...),
			want:  append(fastPathEthernet(unix.ETH_P_IP, fastPathWantIPv4(v4, fastPathESP(fastPathSPI, 0))), padding...),
			ipv4:  1,
		},
		{
			gate:  fastPathGateOpen,
			name:  "ipv4 ESP of eight octets, the shortest that carries an SPI and a sequence number",
			frame: fastPathEthernet(unix.ETH_P_IP, v4.packet(fastPathESP(fastPathSPI, 0))),
			want:  fastPathEthernet(unix.ETH_P_IP, fastPathWantIPv4(v4, fastPathESP(fastPathSPI, 0))),
			ipv4:  1,
		},
		{
			gate:  fastPathGateOpen,
			name:  "ipv6 watched SPI from the peer to the local address",
			frame: fastPathEthernet(unix.ETH_P_IPV6, v6.packet(esp)),
			want:  fastPathEthernet(unix.ETH_P_IPV6, fastPathWantIPv6(v6, esp)),
			ipv6:  1,
		},
		{
			gate:  fastPathGateOpen,
			name:  "ipv6 watched SPI on a CHECKSUM_COMPLETE skb is left untouched",
			frame: fastPathEthernet(unix.ETH_P_IPV6, v6.packet(esp)),
			flags: unix.BPF_F_TEST_SKB_CHECKSUM_COMPLETE,
		},
		// The gate shut: a datagram every check above lets through, untouched.
		{gate: fastPathGateShut, name: "ipv4 watched SPI with the gate shut", frame: fastPathEthernet(unix.ETH_P_IP, v4.packet(esp))},
		{gate: fastPathGateShut, name: "ipv6 watched SPI with the gate shut", frame: fastPathEthernet(unix.ETH_P_IPV6, v6.packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 unwatched SPI", frame: fastPathEthernet(unix.ETH_P_IP, v4.packet(fastPathESP(fastPathOtherSPI, 16)))},
		{gate: fastPathGateOpen, name: "ipv4 source other than the peer", frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.src = fastPathOther4 }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 destination other than the local address", frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.dst = fastPathOther4 }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 first fragment, MF set", frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.flagsFrag = 0x2000 }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 last fragment, offset non-zero", frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.flagsFrag = 0x0003 }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 IHL 6, one word of options", frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.options = []byte{1, 1, 1, 0} }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 header checksum wrong", frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.breakChecksum = true }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 total length beyond the frame", frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.totalLen = 20 + len(esp) + 1 }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 ESP of seven octets", frame: fastPathEthernet(unix.ETH_P_IP, v4.packet(shortESP))},
		{gate: fastPathGateOpen, name: "ipv4 protocol UDP, not ESP", frame: fastPathEthernet(unix.ETH_P_IP, with4(func(h *fastPathIPv4Header) { h.proto = unix.IPPROTO_UDP }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv4 truncated after the SPI", frame: fastPathEthernet(unix.ETH_P_IP, v4.packet(esp[:4]))},
		{gate: fastPathGateOpen, name: "ipv6 unwatched SPI", frame: fastPathEthernet(unix.ETH_P_IPV6, v6.packet(fastPathESP(fastPathOtherSPI, 16)))},
		{gate: fastPathGateOpen, name: "ipv6 source other than the peer", frame: fastPathEthernet(unix.ETH_P_IPV6, with6(func(h *fastPathIPv6Header) { h.src = fastPathOther6 }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv6 destination other than the local address", frame: fastPathEthernet(unix.ETH_P_IPV6, with6(func(h *fastPathIPv6Header) { h.dst = fastPathOther6 }).packet(esp))},
		{gate: fastPathGateOpen, name: "ipv6 hop-by-hop header before ESP", frame: fastPathEthernet(unix.ETH_P_IPV6, with6(func(h *fastPathIPv6Header) { h.next = unix.IPPROTO_HOPOPTS }).packet(hopByHop))},
		{gate: fastPathGateOpen, name: "ipv6 fragment header before ESP", frame: fastPathEthernet(unix.ETH_P_IPV6, with6(func(h *fastPathIPv6Header) { h.next = unix.IPPROTO_FRAGMENT }).packet(fragment))},
		{gate: fastPathGateOpen, name: "ipv6 truncated after the SPI", frame: fastPathEthernet(unix.ETH_P_IPV6, v6.packet(esp[:4]))},
		{gate: fastPathGateOpen, name: "not IP, an ARP frame", frame: fastPathEthernet(unix.ETH_P_ARP, arp)},
		// A watched datagram whose VLAN tag is still in the frame. A rule on the
		// parent link can match the tag, so the datagram must stay ESP.
		{gate: fastPathGateOpen, name: "ipv4 watched SPI behind an in-band 802.1Q tag", frame: fastPathEthernet(unix.ETH_P_8021Q, append([]byte{0x00, 0x64, 0x08, 0x00}, v4.packet(esp)...))},
		{gate: fastPathGateOpen, name: "ipv4 watched SPI behind an in-band 802.1ad tag", frame: fastPathEthernet(unix.ETH_P_8021AD, append([]byte{0x00, 0x64, 0x08, 0x00}, v4.packet(esp)...))},
	}
}

// fastPathESP is an ESP payload: the SPI, a sequence number, then extra octets of a
// pattern standing in for the ciphertext and the ICV.
func fastPathESP(spi uint32, extra int) []byte {
	esp := make([]byte, 8+extra)
	binary.BigEndian.PutUint32(esp[0:4], spi)
	binary.BigEndian.PutUint32(esp[4:8], 7)
	for i := range extra {
		esp[8+i] = byte(0xa0 + i)
	}
	return esp
}

// fastPathEthernet frames an L3 packet in an Ethernet II header.
func fastPathEthernet(ethertype uint16, l3 []byte) []byte {
	frame := make([]byte, 14+len(l3))
	copy(frame[0:6], []byte{0x02, 0, 0, 0, 0, 0x02})
	copy(frame[6:12], []byte{0x02, 0, 0, 0, 0, 0x01})
	binary.BigEndian.PutUint16(frame[12:14], ethertype)
	copy(frame[14:], l3)
	return frame
}

// fastPathIPv4Header is the IPv4 header a row sends (RFC 791 Section 3.1).
type fastPathIPv4Header struct {
	id            uint16
	flagsFrag     uint16
	ttl           byte
	proto         byte
	src, dst      netip.Addr
	options       []byte
	totalLen      int  // 0: the header and the payload
	breakChecksum bool // write a checksum one bit pattern off
}

// packet writes the header, with its checksum computed here, followed by payload.
func (h fastPathIPv4Header) packet(payload []byte) []byte {
	ihl := 20 + len(h.options)
	b := make([]byte, ihl+len(payload))
	b[0] = 0x40 | byte(ihl/4)
	total := len(b)
	if h.totalLen != 0 {
		total = h.totalLen
	}
	binary.BigEndian.PutUint16(b[2:4], uint16(total))
	binary.BigEndian.PutUint16(b[4:6], h.id)
	binary.BigEndian.PutUint16(b[6:8], h.flagsFrag)
	b[8] = h.ttl
	b[9] = h.proto
	src, dst := h.src.As4(), h.dst.As4()
	copy(b[12:16], src[:])
	copy(b[16:20], dst[:])
	copy(b[20:ihl], h.options)
	sum := fastPathChecksum(b[:ihl])
	if h.breakChecksum {
		sum ^= 0x0101
	}
	binary.BigEndian.PutUint16(b[10:12], sum)
	copy(b[ihl:], payload)
	return b
}

// fastPathIPv6Header is the IPv6 header a row sends (RFC 8200 Section 3).
type fastPathIPv6Header struct {
	trafficClass byte
	flow         uint32
	next         byte
	hop          byte
	src, dst     netip.Addr
}

// packet writes the header followed by payload, whose length it carries.
func (h fastPathIPv6Header) packet(payload []byte) []byte {
	b := make([]byte, 40+len(payload))
	binary.BigEndian.PutUint32(b[0:4], 6<<28|uint32(h.trafficClass)<<20|h.flow&0xfffff)
	binary.BigEndian.PutUint16(b[4:6], uint16(len(payload)))
	b[6] = h.next
	b[7] = h.hop
	src, dst := h.src.As16(), h.dst.As16()
	copy(b[8:24], src[:])
	copy(b[24:40], dst[:])
	copy(b[40:], payload)
	return b
}

// fastPathUDP is a UDP header from the peer's IKE port to the local one (RFC 3948
// Section 2.1), its length covering itself and esp.
func fastPathUDP(esp []byte, checksum uint16) []byte {
	udp := make([]byte, 8, 8+len(esp))
	binary.BigEndian.PutUint16(udp[0:2], fastPathPeerPort)
	binary.BigEndian.PutUint16(udp[2:4], fastPathLocalPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(esp)))
	binary.BigEndian.PutUint16(udp[6:8], checksum)
	return append(udp, esp...)
}

// fastPathWantIPv4 is what the sent header and esp must become: the same header
// naming UDP, its total length and checksum recomputed by packet, then a UDP header
// with checksum zero, then esp unchanged.
func fastPathWantIPv4(sent fastPathIPv4Header, esp []byte) []byte {
	sent.proto = unix.IPPROTO_UDP
	return sent.packet(fastPathUDP(esp, 0))
}

// fastPathWantIPv6 is what the sent header and esp must become: the same header
// naming UDP over a payload eight octets longer, then a UDP header carrying 0xffff,
// then esp unchanged.
func fastPathWantIPv6(sent fastPathIPv6Header, esp []byte) []byte {
	sent.next = unix.IPPROTO_UDP
	return sent.packet(fastPathUDP(esp, 0xffff))
}

// fastPathChecksum is the RFC 1071 Internet checksum of b: the one's complement of
// the one's complement sum of its 16-bit big-endian words.
func fastPathChecksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(b[i])<<8 | uint32(b[i+1])
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	for sum > 0xffff {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}
