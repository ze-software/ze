// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)
// Overview: espform.go -- the watched-SPI registry whose entries the fast path's maps mirror
// Related: override_guard_linux.go -- the other program this package assembles from asm.Instructions

//go:build linux

package dataplane

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"golang.org/x/sys/unix"
)

// The fast path is a tc-ingress program that turns bare ESP for a watched SPI into
// the ESP-in-UDP form its inbound XFRM state carries, in place, before the IP layer
// sees it. The kernel then receives the state's own form on the link it arrived on,
// where the inbound policy check runs. An IPv4 datagram the program cannot rewrite (a
// fragment, IPv4 options) or must not (a frame that carries a VLAN tag, which a rule
// on the parent link can match) is left byte-identical for the complete path, which reads
// it off the raw ESP socket ("ip4:50") once XFRM has refused it. An IPv6 datagram it
// skips (an extension header, an skb whose checksum the device already summed) is
// left untouched too, but no IPv6 path serves it: the complete path reads IPv4 only
// until plan/post-release/features/spec-ike-ipv6-transport.md.
//
// IPv4 rewrite, offsets from the IP header (an Ethernet link carries 14 octets of
// L2 header before offset 0, an L3 link none):
//
//	   before (bare ESP)                      after (ESP-in-UDP)
//	 0 +-------+-------+---------------+    0 +-------+-------+---------------+
//	   | 0x45  |  TOS  | Total Length  |      | 0x45  |  TOS  | Total Len + 8 |
//	 4 +-------+-------+---------------+    4 +-------+-------+---------------+
//	   |  Identification | Flags+Offset|      |  Identification | Flags+Offset|
//	 8 +-------+-------+---------------+    8 +-------+-------+---------------+
//	   |  TTL  |  50   | Hdr Checksum  |      |  TTL  |  17   | Hdr Checksum' |
//	12 +-------+-------+---------------+   12 +-------+-------+---------------+
//	   |        Source Address         |      |        Source Address         |
//	16 +-------------------------------+   16 +-------------------------------+
//	   |      Destination Address      |      |      Destination Address      |
//	20 +-------------------------------+   20 +---------------+---------------+
//	   |              SPI              |      | UDP Src (peer)| UDP Dst(local)|
//	24 +-------------------------------+   24 +---------------+---------------+
//	   |        Sequence Number        |      | Len (TL-20+8) | Checksum 0    |
//	28 +-------------------------------+   28 +---------------+---------------+
//	   |          ESP payload ...      |      |              SPI              |
//	                                       32 +-------------------------------+
//	                                          |        Sequence Number        |
//	                                       36 +-------------------------------+
//	                                          |          ESP payload ...      |
//
// IPv6 rewrite, offsets from the IP header:
//
//	   before (bare ESP)                      after (ESP-in-UDP)
//	 0 +-------------------------------+    0 +-------------------------------+
//	   | 6 | Traffic Class | Flow Label|      | 6 | Traffic Class | Flow Label|
//	 4 +---------------+-------+-------+    4 +---------------+-------+-------+
//	   | Payload Length|  50   |HopLim |      | Payload Len+8 |  17   |HopLim |
//	 8 +---------------+-------+-------+    8 +---------------+-------+-------+
//	   |   Source Address (16 octets)  |      |   Source Address (16 octets)  |
//	24 +-------------------------------+   24 +-------------------------------+
//	   | Destination Address (16 oct.) |      | Destination Address (16 oct.) |
//	40 +-------------------------------+   40 +---------------+---------------+
//	   |              SPI              |      | UDP Src (peer)| UDP Dst(local)|
//	44 +-------------------------------+   44 +---------------+---------------+
//	   |        Sequence Number        |      | Len (PL + 8)  | Checksum ffff |
//	48 +-------------------------------+   48 +---------------+---------------+
//	   |          ESP payload ...      |      |              SPI              |
//	                                       52 +-------------------------------+
//	                                          |  Sequence Number, payload ... |
//
// bpf_skb_adjust_room(BPF_ADJ_ROOM_NET) opens the eight octets right after the fixed
// header: it inserts at skb_mac_header_len + bpf_skb_net_base_len, which is
// sizeof(struct iphdr) or sizeof(struct ipv6hdr) (net/core/filter.c,
// bpf_skb_adjust_room, bpf_skb_net_base_len, v7.0). That is why an IPv4 header with
// options is left to the complete path, and an IPv6 header followed by extension
// headers is left untouched, for no IPv6 path until spec-ike-ipv6-transport.
//
// The gate. A rewritten datagram reaches every netfilter hook as UDP, so a rule that
// drops ESP on the input path would never see it. The program therefore reads a
// one-element gate map before it reads the packet, and leaves every packet untouched
// while the gate holds 0: bare ESP then stays ESP until the complete path reads it
// after NF_INET_LOCAL_IN, where the firewall decides as it would without Ze. The
// fast path's manager (espform_fastpath_linux.go) writes 1 only while no rule on the
// input path can drop ESP (espform_firewall_linux.go). A new collection's gate reads
// 0, so the program is shut from the moment it loads.

// Names of the fast path's kernel objects. A BPF object name is at most 15
// characters.
const (
	espFormFastPathGateMap      = "ze_esp_gate"
	espFormFastPathWatched4Map  = "ze_esp_watch4"
	espFormFastPathWatched6Map  = "ze_esp_watch6"
	espFormFastPathCounterMap   = "ze_esp_count"
	espFormFastPathEthernetProg = "ze_esp_form_eth"
	espFormFastPathL3Prog       = "ze_esp_form_l3"

	// espFormFastPathLicense is GPL for the reason overrideGuardLicense is: every
	// helper the program calls is offered to any license today, and one license keeps
	// it loadable if a GPL-only helper is added.
	espFormFastPathLicense = "GPL"
)

// L2 offsets the two program variants are built for. tcx_run pushes skb->mac_len
// back onto the skb before it runs a program (net/core/dev.c, tcx_run:
// "__skb_push(skb, skb->mac_len)"), so the IP header starts after the Ethernet
// header on an Ethernet link and at offset 0 on an L3 device.
const (
	espFormFastPathEthernetL2 = 14 // ETH_HLEN
	espFormFastPathL3L2       = 0
)

// espFormFastPathWatchedMax bounds each watched map. The maps are created with
// BPF_F_NO_PREALLOC, so an unused slot costs nothing, and a full map refuses the
// next insert, which the installer reports.
const espFormFastPathWatchedMax = 65536

// The gate map's one key, and the two values the manager writes into it. The program
// rewrites only under a value other than espFormFastPathGateClosed.
const (
	espFormFastPathGateKey    = 0
	espFormFastPathGateClosed = 0
	espFormFastPathGateOpen   = 1
)

// The counter map's slots: rewritten IPv4 and rewritten IPv6 datagrams, one u64
// per CPU each.
const (
	espFormFastPathSlotIPv4 = 0
	espFormFastPathSlotIPv6 = 1
	espFormFastPathSlots    = 2
)

// espFormFastPathKey is the watched maps' key: the SPI in the four octets the ESP
// header carries it in (RFC 4303 Section 2.1), so the program looks the packet's
// own bytes up with no conversion.
type espFormFastPathKey [4]byte

// newESPFormFastPathKey writes spi the way the wire carries it, network order.
func newESPFormFastPathKey(spi uint32) espFormFastPathKey {
	var key espFormFastPathKey
	binary.BigEndian.PutUint32(key[:], spi)
	return key
}

// The watched maps' values: the peer address, the local address, then the peer's
// and the local UDP port in network order, with no padding. The program reads
// them at these offsets.
const (
	espFormFastPathValue4Len = 2*4 + 2 + 2
	espFormFastPathValue6Len = 2*16 + 2 + 2
)

// espFormFastPathValue4 is the target of one watched IPv4 SPI:
//
//	0        4        8      10      12
//	+--------+--------+-------+-------+
//	| peer   | local  | peer  | local |
//	| IPv4   | IPv4   | port  | port  |
//	+--------+--------+-------+-------+
type espFormFastPathValue4 [espFormFastPathValue4Len]byte

// espFormFastPathValue6 is the target of one watched IPv6 SPI:
//
//	0          16         32      34      36
//	+----------+----------+-------+-------+
//	| peer     | local    | peer  | local |
//	| IPv6     | IPv6     | port  | port  |
//	+----------+----------+-------+-------+
type espFormFastPathValue6 [espFormFastPathValue6Len]byte

// newESPFormFastPathValue4 lays out an IPv4 target. It refuses an address of the
// other family, which only a Ze defect produces: the registry keeps one family per
// SA.
func newESPFormFastPathValue4(peer, local netip.Addr, peerPort, localPort uint16) (espFormFastPathValue4, error) {
	var value espFormFastPathValue4
	if !peer.Is4() {
		return value, fmt.Errorf("ESP form fast path: IPv4 target peer %s is not an IPv4 address", peer)
	}
	if !local.Is4() {
		return value, fmt.Errorf("ESP form fast path: IPv4 target local address %s is not an IPv4 address", local)
	}
	peer4, local4 := peer.As4(), local.As4()
	copy(value[0:4], peer4[:])
	copy(value[4:8], local4[:])
	binary.BigEndian.PutUint16(value[8:10], peerPort)
	binary.BigEndian.PutUint16(value[10:12], localPort)
	return value, nil
}

// newESPFormFastPathValue6 lays out an IPv6 target. It refuses an IPv4 or an
// IPv4-mapped address: a bare ESP datagram over IPv6 never carries one as its
// source or destination, so such a target could never match and only a Ze defect
// produces it.
func newESPFormFastPathValue6(peer, local netip.Addr, peerPort, localPort uint16) (espFormFastPathValue6, error) {
	var value espFormFastPathValue6
	if !peer.Is6() {
		return value, fmt.Errorf("ESP form fast path: IPv6 target peer %s is not an IPv6 address", peer)
	}
	if peer.Is4In6() {
		return value, fmt.Errorf("ESP form fast path: IPv6 target peer %s is an IPv4-mapped address", peer)
	}
	if !local.Is6() {
		return value, fmt.Errorf("ESP form fast path: IPv6 target local address %s is not an IPv6 address", local)
	}
	if local.Is4In6() {
		return value, fmt.Errorf("ESP form fast path: IPv6 target local address %s is an IPv4-mapped address", local)
	}
	peer16, local16 := peer.As16(), local.As16()
	copy(value[0:16], peer16[:])
	copy(value[16:32], local16[:])
	binary.BigEndian.PutUint16(value[32:34], peerPort)
	binary.BigEndian.PutUint16(value[34:36], localPort)
	return value, nil
}

// espFormFastPathSpec is the fast path's collection: the gate, the two watched maps,
// the counter and one program per L2 offset. The programs name the maps by reference,
// so ebpf.NewCollection creates the maps once and both programs share them.
func espFormFastPathSpec() *ebpf.CollectionSpec {
	return &ebpf.CollectionSpec{
		Maps: map[string]*ebpf.MapSpec{
			// An array element exists from creation and reads zero, so the gate is
			// shut until the manager opens it.
			espFormFastPathGateMap: {
				Name:       espFormFastPathGateMap,
				Type:       ebpf.Array,
				KeySize:    4,
				ValueSize:  4,
				MaxEntries: 1,
			},
			espFormFastPathWatched4Map: {
				Name:       espFormFastPathWatched4Map,
				Type:       ebpf.Hash,
				KeySize:    uint32(len(espFormFastPathKey{})),
				ValueSize:  espFormFastPathValue4Len,
				MaxEntries: espFormFastPathWatchedMax,
				Flags:      unix.BPF_F_NO_PREALLOC,
			},
			espFormFastPathWatched6Map: {
				Name:       espFormFastPathWatched6Map,
				Type:       ebpf.Hash,
				KeySize:    uint32(len(espFormFastPathKey{})),
				ValueSize:  espFormFastPathValue6Len,
				MaxEntries: espFormFastPathWatchedMax,
				Flags:      unix.BPF_F_NO_PREALLOC,
			},
			espFormFastPathCounterMap: {
				Name:       espFormFastPathCounterMap,
				Type:       ebpf.PerCPUArray,
				KeySize:    4,
				ValueSize:  8,
				MaxEntries: espFormFastPathSlots,
			},
		},
		Programs: map[string]*ebpf.ProgramSpec{
			espFormFastPathEthernetProg: {
				Name:         espFormFastPathEthernetProg,
				Type:         ebpf.SchedCLS,
				License:      espFormFastPathLicense,
				Instructions: espFormFastPathInstructions(espFormFastPathEthernetL2),
			},
			espFormFastPathL3Prog: {
				Name:         espFormFastPathL3Prog,
				Type:         ebpf.SchedCLS,
				License:      espFormFastPathLicense,
				Instructions: espFormFastPathInstructions(espFormFastPathL3L2),
			},
		},
	}
}

// Return codes of a TCX program (include/uapi/linux/bpf.h, enum tcx_action_base:
// "TCX_NEXT = -1", "TCX_DROP = 2"). tcx_run stops at the first program that answers
// anything but TCX_NEXT, and the fast path attaches at the head, so every path but
// one answers TCX_NEXT and the programs behind it still run.
const (
	espFormFastPathNext = -1
	espFormFastPathDrop = 2
)

// __sk_buff field offsets (include/uapi/linux/bpf.h, v7.0, struct __sk_buff: len,
// pkt_type, mark, queue_mapping, protocol, vlan_present, each a __u32). protocol
// holds the network-order ethertype in its low 16 bits. vlan_present reads 1 while
// the skb holds a VLAN tag out of band: the verifier rewrites the load
// (bpf_convert_ctx_access, net/core/filter.c, v7.0) as "BPF_LDX_MEM(BPF_W,
// si->dst_reg, si->src_reg, bpf_target_off(struct sk_buff, vlan_all, 4,
// target_size));" then "BPF_JMP_IMM(BPF_JEQ, si->dst_reg, 0, 1);" and
// "BPF_ALU32_IMM(BPF_MOV, si->dst_reg, 1);", so it is non-zero exactly when the
// tag's protocol or TCI is.
const (
	espFormFastPathSkbLen         = 0
	espFormFastPathSkbProtocol    = 16
	espFormFastPathSkbVLANPresent = 20
)

// espFormFastPathEtherType is the octet offset of the ether type in an Ethernet
// header, which the Ethernet variant reads from the frame.
const espFormFastPathEtherType = 12

// The program's stack. The packet's header and the first eight ESP octets are
// copied to the buffer, checked, rewritten there in place to the new header plus
// the UDP header, and stored back in one bpf_skb_store_bytes call. Every access is
// aligned to its size, which the verifier requires of the stack.
//
//	| fp offset  | octets | holds                                                  |
//	|------------|--------|--------------------------------------------------------|
//	| -48 .. -21 | 28     | IPv4: header (20) + SPI and sequence, then header + UDP |
//	| -48 .. -1  | 48     | IPv6: header (40) + SPI and sequence, then header + UDP |
//	| -56 .. -53 | 4      | the counter slot to bump                               |
//	| -60 .. -57 | 4      | the gate map's key, espFormFastPathGateKey             |
//	| -64 .. -63 | 2      | Ethernet variant: the frame's ether type               |
//
// Registers that must survive helper calls are callee-saved: R6 the context, R7 the
// watched map value, R8 the length the ESP rides in (IPv4 total length or IPv6
// payload length, host order), R9 skb->len until the bounds check, then on the IPv6
// path whether this program raised the checksum level and must lower it again. The
// gate's value lives in R0 between its lookup and its test, and no helper runs between
// them.
const (
	espFormFastPathBuf       = -48
	espFormFastPathSlotKey   = -56
	espFormFastPathGateKeyAt = -60
	espFormFastPathEtherAt   = -64
)

// IPv4 header fields, octet offsets in the buffer (RFC 791 Section 3.1).
const (
	espFormFastPathV4VersionIHL = 0
	espFormFastPathV4TotalLen   = 2
	espFormFastPathV4FragOff    = 6
	espFormFastPathV4Protocol   = 9
	espFormFastPathV4Checksum   = 10
	espFormFastPathV4Source     = 12
	espFormFastPathV4Dest       = 16

	// espFormFastPathV4Plain is version 4 with an IHL of 5: no options.
	espFormFastPathV4Plain = 0x45

	// espFormFastPathV4FragMask covers the MF flag (0x2000) and the 13-bit fragment
	// offset; DF (0x4000) does not make a fragment.
	espFormFastPathV4FragMask = 0x3fff
)

// IPv6 header fields, octet offsets in the buffer (RFC 8200 Section 3).
const (
	espFormFastPathV6HeaderLen  = 40
	espFormFastPathV6PayloadLen = 4
	espFormFastPathV6NextHeader = 6
	espFormFastPathV6Source     = 8
	espFormFastPathV6Dest       = 24

	// espFormFastPathV6MaxESPLen keeps the UDP length field, ESP plus the UDP
	// header, inside 16 bits.
	espFormFastPathV6MaxESPLen = 0xFFFF - espFormUDPHeaderLen
)

// UDP header fields, octet offsets from the start of the inserted header (RFC 768).
const (
	espFormFastPathUDPSource   = 0
	espFormFastPathUDPDest     = 2
	espFormFastPathUDPLen      = 4
	espFormFastPathUDPChecksum = 6
)

// espFormFastPathIPv6Checksum is the UDP checksum the IPv6 rewrite writes. See
// espFormFastPathIPv6 for why a constant is correct.
const espFormFastPathIPv6Checksum = 0xffff

// Jump labels. A program is one symbol space.
const (
	espFormFastPathLabelV4      = "esp_v4"
	espFormFastPathLabelV6      = "esp_v6"
	espFormFastPathLabelV6Room  = "esp_v6_room"
	espFormFastPathLabelV6Store = "esp_v6_store"
	espFormFastPathLabelCount   = "esp_count"
	espFormFastPathLabelNext    = "esp_next"
	espFormFastPathLabelDrop    = "esp_drop"
)

// espFormFastPathNetwork16 answers what a BPF load of the network-order 16-bit
// value v yields on this host: a load is in host order (docs/architecture/traffic/
// traffic-usage.md, "eBPF traps").
func espFormFastPathNetwork16(v uint16) int32 {
	var wire [2]byte
	binary.BigEndian.PutUint16(wire[:], v)
	return int32(binary.NativeEndian.Uint16(wire[:]))
}

// espFormFastPathInstructions is the rewrite program for one L2 offset. It reads the
// gate first and answers TCX_NEXT, before it reads anything of the packet, while the
// gate is shut or cannot be read. It then answers TCX_NEXT, untouched, for every skb
// that carries a VLAN tag, and only then branches on the skb's ethertype into the
// IPv4 or the IPv6 rewrite, answering TCX_NEXT for anything else.
//
// A tagged datagram always takes the complete path, after the input hook. A rule on
// the VLAN's parent link can match on the tag: nft_payload_eval puts a tag the
// device took off back into a link-layer load (net/netfilter/nft_payload.c, v7.0,
// nft_payload_copy_vlan), while the skb's protocol already names the inner IPv4, so
// a rewrite here would hand that rule UDP where it matches ESP. Two tests cover the
// two forms a tag takes:
//
//   - Out of band: vlan_present. skb_vlan_untag runs before TCX in
//     __netif_receive_skb_core (net/core/dev.c: "if (eth_type_vlan(skb->protocol))
//     { skb = skb_vlan_untag(skb);" ahead of sch_handle_ingress), so the outer tag of
//     a received frame is out of band by the time the program runs.
//   - In band, on the Ethernet variant: the frame's ether type must be the skb's
//     protocol. An 802.1Q or 802.1ad tag left in the frame puts its TPID there. With
//     the untag above, the protocol then reads the TPID too and the branch below
//     refuses it already; this compare keeps the refusal from resting on that.
//
// RFC 7296 Section 2.23: "all devices MUST be able to receive and process both
// UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time." A Linux
// inbound state receives one form (LL-XFRM-STATE-ONE-ESP-FORM in
// docs/architecture/linux-limitations.md); the program hands it that form.
func espFormFastPathInstructions(l2 int32) asm.Instructions {
	insns := asm.Instructions{
		asm.Mov.Reg(asm.R6, asm.R1),

		// The gate. An array lookup of an index in range does not fail; a failure
		// still shuts the gate rather than opening it.
		asm.StoreImm(asm.RFP, espFormFastPathGateKeyAt, espFormFastPathGateKey, asm.Word),
		asm.LoadMapPtr(asm.R1, 0).WithReference(espFormFastPathGateMap),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, espFormFastPathGateKeyAt),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, espFormFastPathLabelNext),
		asm.LoadMem(asm.R0, asm.R0, 0, asm.Word),
		asm.JEq.Imm(asm.R0, espFormFastPathGateClosed, espFormFastPathLabelNext),

		asm.LoadMem(asm.R0, asm.R6, espFormFastPathSkbVLANPresent, asm.Word),
		asm.JNE.Imm(asm.R0, 0, espFormFastPathLabelNext),
	}
	if l2 == espFormFastPathEthernetL2 {
		// The frame's ether type, copied to the stack: a frame too short to hold it
		// fails the copy and goes on untouched.
		insns = append(insns,
			asm.Mov.Reg(asm.R1, asm.R6),
			asm.Mov.Imm(asm.R2, espFormFastPathEtherType),
			asm.Mov.Reg(asm.R3, asm.RFP),
			asm.Add.Imm(asm.R3, espFormFastPathEtherAt),
			asm.Mov.Imm(asm.R4, 2),
			asm.FnSkbLoadBytes.Call(),
			asm.JNE.Imm(asm.R0, 0, espFormFastPathLabelNext),
		)
	}
	insns = append(insns,
		asm.LoadMem(asm.R9, asm.R6, espFormFastPathSkbLen, asm.Word),
		asm.LoadMem(asm.R2, asm.R6, espFormFastPathSkbProtocol, asm.Word),
	)
	if l2 == espFormFastPathEthernetL2 {
		// Both are the network-order ether type read in host order, so they compare
		// equal exactly when the octets do.
		insns = append(insns,
			asm.LoadMem(asm.R1, asm.RFP, espFormFastPathEtherAt, asm.Half),
			asm.JNE.Reg(asm.R1, asm.R2, espFormFastPathLabelNext),
		)
	}
	insns = append(insns,
		asm.JEq.Imm(asm.R2, espFormFastPathNetwork16(unix.ETH_P_IP), espFormFastPathLabelV4),
		asm.JEq.Imm(asm.R2, espFormFastPathNetwork16(unix.ETH_P_IPV6), espFormFastPathLabelV6),
		asm.Ja.Label(espFormFastPathLabelNext),
	)
	insns = append(insns, espFormFastPathIPv4(l2)...)
	insns = append(insns, espFormFastPathIPv6(l2)...)
	return append(insns,
		// Bump the slot staged at espFormFastPathSlotKey. A failed lookup cannot
		// happen for an in-range array index; it skips the count, never the packet.
		asm.LoadMapPtr(asm.R1, 0).WithReference(espFormFastPathCounterMap).WithSymbol(espFormFastPathLabelCount),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, espFormFastPathSlotKey),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, espFormFastPathLabelNext),
		asm.Mov.Imm(asm.R1, 1),
		asm.StoreXAdd(asm.R0, asm.R1, asm.DWord),

		asm.Mov.Imm(asm.R0, espFormFastPathNext).WithSymbol(espFormFastPathLabelNext),
		asm.Return(),

		// Only a datagram whose room was opened and whose header could not be written
		// arrives here: it is half-rewritten and must reach neither XFRM nor the IP
		// layer.
		asm.Mov.Imm(asm.R0, espFormFastPathDrop).WithSymbol(espFormFastPathLabelDrop),
		asm.Return(),
	)
}

// espFormFastPathIPv4 rewrites a whole, option-free bare ESP datagram from a
// watched SPI's peer to its local address. Every check that fails jumps to
// TCX_NEXT with the packet untouched.
//
// The TOS octet is kept, which is what RFC 7296 Section 2.24 needs of this path:
// decapsulators "MUST implement the tunnel encapsulation and decapsulation processing
// specified in [IPSECARCH] to prevent discarding of ECN congestion indications."
// espFormFastPathLoad copies the received header to espFormFastPathBuf, the program
// writes only the total length (offset 2), the protocol (offset 9), the header checksum
// (offset 10) and the UDP header (offsets 20 to 27), and espFormFastPathStore writes the
// buffer back over the header. The TOS at offset 1 goes back as it arrived, under a
// checksum summed over it, so XFRM's decapsulation reads the received ECN field and
// copies a CE mark into the inner header (ipip_ecn_decapsulate, net/xfrm/xfrm_input.c).
func espFormFastPathIPv4(l2 int32) asm.Instructions {
	const (
		headerLen = espFormIPv4HeaderLen
		bufLen    = headerLen + espFormMinESPLen // the header, the SPI and the sequence number
		udp       = espFormFastPathBuf + headerLen
		spi       = espFormFastPathBuf + headerLen
		minTotal  = headerLen + espFormMinESPLen
		maxTotal  = headerLen + espFormMaxESPLen
	)
	insns := espFormFastPathLoad(l2, bufLen)
	insns[0] = insns[0].WithSymbol(espFormFastPathLabelV4)
	insns = append(insns,
		asm.LoadMem(asm.R0, asm.RFP, espFormFastPathBuf+espFormFastPathV4VersionIHL, asm.Byte),
		asm.JNE.Imm(asm.R0, espFormFastPathV4Plain, espFormFastPathLabelNext),
		asm.LoadMem(asm.R0, asm.RFP, espFormFastPathBuf+espFormFastPathV4Protocol, asm.Byte),
		asm.JNE.Imm(asm.R0, unix.IPPROTO_ESP, espFormFastPathLabelNext),

		// A fragment carries no SPI unless it is the first, and no first fragment can
		// carry the UDP length of the whole datagram: IP reassembly runs after tc
		// ingress, so fragments go to the complete path, which reads the reassembled
		// datagram.
		asm.LoadMem(asm.R0, asm.RFP, espFormFastPathBuf+espFormFastPathV4FragOff, asm.Half),
		asm.HostTo(asm.BE, asm.R0, asm.Half),
		asm.And.Imm(asm.R0, espFormFastPathV4FragMask),
		asm.JNE.Imm(asm.R0, 0, espFormFastPathLabelNext),

		// The total length must hold the SPI and the sequence number, must fit the
		// frame (a frame longer than it is link padding, which ip_rcv trims), and must
		// leave room for the UDP header inside 16 bits.
		asm.LoadMem(asm.R8, asm.RFP, espFormFastPathBuf+espFormFastPathV4TotalLen, asm.Half),
		asm.HostTo(asm.BE, asm.R8, asm.Half),
		asm.JLT.Imm(asm.R8, minTotal, espFormFastPathLabelNext),
		asm.JGT.Imm(asm.R8, maxTotal, espFormFastPathLabelNext),
		asm.Mov.Reg(asm.R0, asm.R8),
		asm.Add.Imm(asm.R0, l2),
		asm.JGT.Reg(asm.R0, asm.R9, espFormFastPathLabelNext),
	)
	insns = append(insns, espFormFastPathLookup(espFormFastPathWatched4Map, spi)...)
	insns = append(insns,
		asm.LoadMem(asm.R1, asm.RFP, espFormFastPathBuf+espFormFastPathV4Source, asm.Word),
		asm.LoadMem(asm.R2, asm.R7, 0, asm.Word),
		asm.JNE.Reg(asm.R1, asm.R2, espFormFastPathLabelNext),
		asm.LoadMem(asm.R1, asm.RFP, espFormFastPathBuf+espFormFastPathV4Dest, asm.Word),
		asm.LoadMem(asm.R2, asm.R7, 4, asm.Word),
		asm.JNE.Reg(asm.R1, asm.R2, espFormFastPathLabelNext),
	)

	// RFC 791 Section 3.1: "Since some header fields change (e.g., time to live), this
	// is recomputed and verified at each point that the internet header is processed."
	// The program writes a fresh checksum, so it verifies the received one first: a
	// corrupted header is left for ip_rcv to drop, never given a valid checksum.
	insns = append(insns, espFormFastPathIPv4HeaderSum()...)
	insns = append(insns,
		asm.JNE.Imm(asm.R0, 0xffff, espFormFastPathLabelNext),

		// RFC 3948 Section 3.4: "The Total Length, Protocol, and Header Checksum (for
		// IPv4) fields in the new IP header are edited to match the resulting IP
		// packet."
		asm.Mov.Reg(asm.R1, asm.R8),
		asm.Add.Imm(asm.R1, espFormUDPHeaderLen),
		asm.HostTo(asm.BE, asm.R1, asm.Half),
		asm.StoreMem(asm.RFP, espFormFastPathBuf+espFormFastPathV4TotalLen, asm.R1, asm.Half),
		asm.StoreImm(asm.RFP, espFormFastPathBuf+espFormFastPathV4Protocol, unix.IPPROTO_UDP, asm.Byte),
		asm.StoreImm(asm.RFP, espFormFastPathBuf+espFormFastPathV4Checksum, 0, asm.Half),
	)
	insns = append(insns, espFormFastPathPorts(udp, 2*4)...)
	insns = append(insns,
		// RFC 768: "Length  is the length  in octets  of this user datagram  including  this
		// header  and the data." The datagram is the ESP payload, the total length
		// less the fixed header, plus the UDP header.
		asm.Mov.Reg(asm.R1, asm.R8),
		asm.Add.Imm(asm.R1, espFormUDPHeaderLen-headerLen),
		asm.HostTo(asm.BE, asm.R1, asm.Half),
		asm.StoreMem(asm.RFP, udp+espFormFastPathUDPLen, asm.R1, asm.Half),

		// RFC 3948 Section 2.1: "the IPv4 UDP Checksum SHOULD be transmitted as a zero
		// value". RFC 768: "An all zero  transmitted checksum  value means that the
		// transmitter  generated  no checksum  (for debugging or for higher level
		// protocols that don't care)." The ESP ICV protects the payload.
		asm.StoreImm(asm.RFP, udp+espFormFastPathUDPChecksum, 0, asm.Half),
	)

	// RFC 791 Section 3.1: "The checksum field is the 16 bit one's complement of the
	// one's complement sum of all 16 bit words in the header." bpf_csum_diff sums the
	// header in host-order words, and the complement stored back in host order lands
	// in the right octets, because a one's complement sum is the same in either byte
	// order once its two octets are swapped back.
	insns = append(insns, espFormFastPathIPv4HeaderSum()...)
	insns = append(insns,
		asm.Xor.Imm(asm.R0, 0xffff),
		asm.StoreMem(asm.RFP, espFormFastPathBuf+espFormFastPathV4Checksum, asm.R0, asm.Half),

		// Flags 0: on success the helper turns a CHECKSUM_UNNECESSARY skb into
		// CHECKSUM_NONE, harmless for an IPv4 UDP checksum of zero, which nothing
		// verifies. A refusal (a GSO skb, a datagram past SKB_MAX_ALLOC) has changed
		// nothing, so the datagram goes on untouched to the complete path.
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, espFormUDPHeaderLen),
		asm.Mov.Imm(asm.R3, unix.BPF_ADJ_ROOM_NET),
		asm.Mov.Imm(asm.R4, 0),
		asm.FnSkbAdjustRoom.Call(),
		asm.JNE.Imm(asm.R0, 0, espFormFastPathLabelNext),
	)
	insns = append(insns, espFormFastPathStore(l2, bufLen, espFormFastPathSlotIPv4)...)
	return insns
}

// espFormFastPathIPv6 rewrites a bare ESP datagram whose fixed header names ESP as
// its next header, from a watched SPI's peer to its local address.
//
// Nothing reaches this half of the program until Ze has an IPv6 IKE transport: the IKE
// listener opens udp4 only, so no Child SA over IPv6 is negotiated, InstallSA refuses one
// that asks for both ESP forms, and no IPv6 SPI is ever written into its map. The gap,
// owed after the first release, is plan/post-release/features/spec-ike-ipv6-transport.md.
//
// The UDP checksum. IPv6 refuses a zero one: RFC 8200 Section 8.1: "IPv6 receivers
// must discard UDP packets containing a zero checksum and should log the error." Linux
// does so in __udp6_lib_rcv ("if (!uh->check && !udp_get_no_check6_rx(sk))",
// net/ipv6/udp.c), and the program cannot sum a payload of any length to write a real
// one. It marks the skb CHECKSUM_UNNECESSARY instead and writes the constant
// 0xffff: bpf_csum_level(BPF_CSUM_LEVEL_INC) runs __skb_incr_checksum_unnecessary,
// which turns CHECKSUM_NONE into CHECKSUM_UNNECESSARY at level 0
// (include/linux/skbuff.h). That UDP receive then skips the checksum is UNPROVEN: the
// reading is that udp6_csum_init reaches __skb_checksum_validate_needed, which on
// skb_csum_unnecessary sets csum_valid and returns false, and that
// udp_lib_checksum_complete before the encapsulation hook sees csum_valid and skips
// the sum, but no test has delivered such a datagram to an IPv6 UDP encapsulation
// socket; spec-ike-ipv6-transport owes that proof before an IPv6 SPI is ever
// written. The non-zero constant only gets past the zero-checksum refusal. A
// CHECKSUM_COMPLETE skb, where the device summed the packet, is not moved by INC
// (__skb_incr_checksum_unnecessary changes only NONE and UNNECESSARY), and its sum
// would then catch the constant, so the program leaves it untouched; no IPv6 path
// serves it until spec-ike-ipv6-transport. The adjustment passes
// BPF_F_ADJ_ROOM_NO_CSUM_RESET so the mark survives it.
func espFormFastPathIPv6(l2 int32) asm.Instructions {
	const (
		headerLen = espFormFastPathV6HeaderLen
		bufLen    = headerLen + espFormMinESPLen
		udp       = espFormFastPathBuf + headerLen
		spi       = espFormFastPathBuf + headerLen
		addrLen   = 16
	)
	insns := espFormFastPathLoad(l2, bufLen)
	insns[0] = insns[0].WithSymbol(espFormFastPathLabelV6)
	insns = append(insns,
		asm.LoadMem(asm.R0, asm.RFP, espFormFastPathBuf, asm.Byte),
		asm.RSh.Imm(asm.R0, 4),
		asm.JNE.Imm(asm.R0, 6, espFormFastPathLabelNext),

		// ESP directly after the fixed header. An extension header (hop-by-hop, a
		// fragment header) sits where the UDP header would be inserted, so that
		// datagram is left untouched; no IPv6 path serves it until
		// spec-ike-ipv6-transport.
		asm.LoadMem(asm.R0, asm.RFP, espFormFastPathBuf+espFormFastPathV6NextHeader, asm.Byte),
		asm.JNE.Imm(asm.R0, unix.IPPROTO_ESP, espFormFastPathLabelNext),

		// The payload length must hold the SPI and the sequence number, must fit the
		// frame, and must leave room for the UDP header inside 16 bits. Zero (a
		// jumbogram) needs a hop-by-hop header and has already gone.
		asm.LoadMem(asm.R8, asm.RFP, espFormFastPathBuf+espFormFastPathV6PayloadLen, asm.Half),
		asm.HostTo(asm.BE, asm.R8, asm.Half),
		asm.JLT.Imm(asm.R8, espFormMinESPLen, espFormFastPathLabelNext),
		asm.JGT.Imm(asm.R8, espFormFastPathV6MaxESPLen, espFormFastPathLabelNext),
		asm.Mov.Reg(asm.R0, asm.R8),
		asm.Add.Imm(asm.R0, l2+headerLen),
		asm.JGT.Reg(asm.R0, asm.R9, espFormFastPathLabelNext),
	)
	insns = append(insns, espFormFastPathLookup(espFormFastPathWatched6Map, spi)...)
	for word := int16(0); word < addrLen; word += 4 {
		insns = append(insns,
			asm.LoadMem(asm.R1, asm.RFP, espFormFastPathBuf+espFormFastPathV6Source+word, asm.Word),
			asm.LoadMem(asm.R2, asm.R7, word, asm.Word),
			asm.JNE.Reg(asm.R1, asm.R2, espFormFastPathLabelNext),
			asm.LoadMem(asm.R1, asm.RFP, espFormFastPathBuf+espFormFastPathV6Dest+word, asm.Word),
			asm.LoadMem(asm.R2, asm.R7, addrLen+word, asm.Word),
			asm.JNE.Reg(asm.R1, asm.R2, espFormFastPathLabelNext),
		)
	}
	insns = append(insns,
		asm.Mov.Reg(asm.R1, asm.R8),
		asm.Add.Imm(asm.R1, espFormUDPHeaderLen),
		asm.HostTo(asm.BE, asm.R1, asm.Half),
		asm.StoreMem(asm.RFP, espFormFastPathBuf+espFormFastPathV6PayloadLen, asm.R1, asm.Half),
		asm.StoreImm(asm.RFP, espFormFastPathBuf+espFormFastPathV6NextHeader, unix.IPPROTO_UDP, asm.Byte),
	)
	insns = append(insns, espFormFastPathPorts(udp, 2*addrLen)...)
	insns = append(insns,
		// RFC 768: "Length  is the length  in octets  of this user datagram  including  this
		// header  and the data." The payload length is the ESP payload here.
		asm.Mov.Reg(asm.R1, asm.R8),
		asm.Add.Imm(asm.R1, espFormUDPHeaderLen),
		asm.HostTo(asm.BE, asm.R1, asm.Half),
		asm.StoreMem(asm.RFP, udp+espFormFastPathUDPLen, asm.R1, asm.Half),
		asm.StoreImm(asm.RFP, udp+espFormFastPathUDPChecksum, espFormFastPathIPv6Checksum, asm.Half),

		// R9 records whether this program raised the checksum level, so a refused
		// adjustment can lower it again and leave the skb as it came.
		asm.Mov.Imm(asm.R9, 0),
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, unix.BPF_CSUM_LEVEL_QUERY),
		asm.FnCsumLevel.Call(),
		asm.JSGE.Imm(asm.R0, 0, espFormFastPathLabelV6Room),
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, unix.BPF_CSUM_LEVEL_INC),
		asm.FnCsumLevel.Call(),
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, unix.BPF_CSUM_LEVEL_QUERY),
		asm.FnCsumLevel.Call(),
		asm.JSLT.Imm(asm.R0, 0, espFormFastPathLabelNext),
		asm.Mov.Imm(asm.R9, 1),

		asm.Mov.Reg(asm.R1, asm.R6).WithSymbol(espFormFastPathLabelV6Room),
		asm.Mov.Imm(asm.R2, espFormUDPHeaderLen),
		asm.Mov.Imm(asm.R3, unix.BPF_ADJ_ROOM_NET),
		asm.Mov.Imm(asm.R4, unix.BPF_F_ADJ_ROOM_NO_CSUM_RESET),
		asm.FnSkbAdjustRoom.Call(),
		asm.JEq.Imm(asm.R0, 0, espFormFastPathLabelV6Store),
		asm.JEq.Imm(asm.R9, 0, espFormFastPathLabelNext),
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, unix.BPF_CSUM_LEVEL_DEC),
		asm.FnCsumLevel.Call(),
		asm.Ja.Label(espFormFastPathLabelNext),
	)
	store := espFormFastPathStore(l2, bufLen, espFormFastPathSlotIPv6)
	store[0] = store[0].WithSymbol(espFormFastPathLabelV6Store)
	return append(insns, store...)
}

// espFormFastPathLoad copies bufLen octets from the IP header into the stack buffer,
// and answers TCX_NEXT for a packet too short to hold them: bpf_skb_load_bytes
// fails with -EFAULT and zeroes the buffer (net/core/filter.c).
func espFormFastPathLoad(l2, bufLen int32) asm.Instructions {
	return asm.Instructions{
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, l2),
		asm.Mov.Reg(asm.R3, asm.RFP),
		asm.Add.Imm(asm.R3, espFormFastPathBuf),
		asm.Mov.Imm(asm.R4, bufLen),
		asm.FnSkbLoadBytes.Call(),
		asm.JNE.Imm(asm.R0, 0, espFormFastPathLabelNext),
	}
}

// espFormFastPathLookup looks the SPI at stack offset spi up in the named watched
// map, answers TCX_NEXT for an SPI no SA watches, and keeps the value in R7.
func espFormFastPathLookup(watched string, spi int32) asm.Instructions {
	return asm.Instructions{
		asm.LoadMapPtr(asm.R1, 0).WithReference(watched),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, spi),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, espFormFastPathLabelNext),
		asm.Mov.Reg(asm.R7, asm.R0),
	}
}

// espFormFastPathPorts writes the UDP ports from the watched value in R7, whose
// ports follow the two addresses at portsOffset, into the UDP header at stack offset
// udp. They are copied as the octets they are, network order.
//
// RFC 3948 Section 2.1: "the Source Port and Destination Port MUST be the same as that
// used by IKE traffic". The datagram travels from the peer to Ze, so its source is
// the peer's IKE port and its destination Ze's. The source port is the state's
// encap_sport: esp_input_done2 (net/ipv4/esp4.c) raises km_new_mapping for any other.
func espFormFastPathPorts(udp, portsOffset int16) asm.Instructions {
	return asm.Instructions{
		asm.LoadMem(asm.R1, asm.R7, portsOffset, asm.Half),
		asm.StoreMem(asm.RFP, udp+espFormFastPathUDPSource, asm.R1, asm.Half),
		asm.LoadMem(asm.R1, asm.R7, portsOffset+2, asm.Half),
		asm.StoreMem(asm.RFP, udp+espFormFastPathUDPDest, asm.R1, asm.Half),
	}
}

// espFormFastPathIPv4HeaderSum leaves in R0 the one's complement sum, folded to 16 bits,
// of the IPv4 header at the start of the stack buffer. bpf_csum_diff with no "from"
// block sums the "to" block (net/core/filter.c, bpf_csum_diff: "from_size == 0,
// to_size > 0,  seed := csum --> pushing data"). It folds its answer to 16 bits at
// v7.0 (csum_from32to16); the program folds again, which changes nothing then and
// keeps the sum right on a kernel that answers the 32-bit sum.
func espFormFastPathIPv4HeaderSum() asm.Instructions {
	return asm.Instructions{
		asm.Mov.Imm(asm.R1, 0),
		asm.Mov.Imm(asm.R2, 0),
		asm.Mov.Reg(asm.R3, asm.RFP),
		asm.Add.Imm(asm.R3, espFormFastPathBuf),
		asm.Mov.Imm(asm.R4, espFormIPv4HeaderLen),
		asm.Mov.Imm(asm.R5, 0),
		asm.FnCsumDiff.Call(),
		asm.Mov.Reg32(asm.R0, asm.R0),
		asm.Mov.Reg(asm.R1, asm.R0),
		asm.RSh.Imm(asm.R1, 16),
		asm.And.Imm(asm.R0, 0xffff),
		asm.Add.Reg(asm.R0, asm.R1),
		asm.Mov.Reg(asm.R1, asm.R0),
		asm.RSh.Imm(asm.R1, 16),
		asm.And.Imm(asm.R0, 0xffff),
		asm.Add.Reg(asm.R0, asm.R1),
	}
}

// espFormFastPathStore writes the rewritten header and the UDP header, bufLen octets,
// over the IP header and the room just opened, then counts the datagram in slot.
// BPF_F_RECOMPUTE_CSUM keeps a CHECKSUM_COMPLETE skb's sum right: the helper
// subtracts the old octets and adds the new (bpf_skb_store_bytes, net/core/filter.c).
// A failed store leaves a datagram whose room is open and whose header is stale, so
// it is dropped.
func espFormFastPathStore(l2, bufLen int32, slot int64) asm.Instructions {
	return asm.Instructions{
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, l2),
		asm.Mov.Reg(asm.R3, asm.RFP),
		asm.Add.Imm(asm.R3, espFormFastPathBuf),
		asm.Mov.Imm(asm.R4, bufLen),
		asm.Mov.Imm(asm.R5, unix.BPF_F_RECOMPUTE_CSUM),
		asm.FnSkbStoreBytes.Call(),
		asm.JNE.Imm(asm.R0, 0, espFormFastPathLabelDrop),
		asm.StoreImm(asm.RFP, espFormFastPathSlotKey, slot, asm.Word),
		asm.Ja.Label(espFormFastPathLabelCount),
	}
}
