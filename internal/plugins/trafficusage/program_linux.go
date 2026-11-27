//go:build linux

// Design: docs/architecture/traffic/traffic-usage.md -- pure-Go eBPF TCX accounting programs (asm.Instructions)

package trafficusage

import (
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
)

// Map and program names. BPF object names must be <= 15 chars. Maps are not
// pinned; cilium/ebpf keeps the FDs alive in-process.
const (
	mapIPIngress   = "tu_ip_in"
	mapIPEgress    = "tu_ip_out"
	mapIP6Ingress  = "tu_ip6_in"
	mapIP6Egress   = "tu_ip6_out"
	mapPortIngress = "tu_port_in"
	mapPortEgress  = "tu_port_out"

	progIngress = "tu_ingress"
	progEgress  = "tu_egress"

	// bpfLicense is the license every trafficusage program declares: the
	// kernel refuses GPL-only helpers to a program that declares another.
	bpfLicense = "GPL"

	keySizePort = 4
	keySizeIPv4 = 4
	keySizeIPv6 = 16
)

// tcxNext is TCX_NEXT (include/uapi/linux/bpf.h, enum tcx_action_base), the
// verdict every trafficusage program returns: it leaves the packet's fate to the
// next program on the hook and then to the legacy tc filters. The programs only
// count, so they must not end the hook.
//
// TCX_PASS (0, the value of TC_ACT_OK) would end it. tcx_run stops at the first
// program that answers anything but TCX_NEXT, so a TCX program attached after
// this one would not run (the IPsec ESP form fast path is one), and
// sch_handle_ingress skips the legacy clsact filters on any TCX verdict but
// TC_ACT_UNSPEC, which equals TCX_NEXT (net/core/dev.c v7.2, lines 4485-4490):
//
//	if (static_branch_unlikely(&tcx_needed_key)) {
//		sch_ret = tcx_run(entry, skb, true);
//		if (sch_ret != TC_ACT_UNSPEC)
//			goto ingress_verdict;
//	}
//	sch_ret = tc_run(tcx_entry(entry), skb, &drop_reason);
//
// sch_handle_egress carries the same test (lines 4544-4549). On a link with
// traffic-usage attached, a pass therefore turned off the traffic-control
// ingress policer, the flow-export sFlow sampler and the interface mirror, which
// are all clsact filters (internal/plugins/traffic/netlink/policer_linux.go,
// internal/plugins/flowexport/sampling/tc_linux.go,
// internal/plugins/iface/netlink/mirror_linux.go), and on egress the mirror.
const tcxNext = -1

// Header layout constants (IPv4 and IPv6 over Ethernet II).
const (
	ethHdrLen   = 14
	ipHdrLen    = 20     // fixed part; options handled via the IHL field
	ipv6HdrLen  = 40     // fixed IPv6 base header (RFC 8200 Section 3)
	ethTypeIPv4 = 0x008  // htons(ETH_P_IP=0x0800) on a little-endian host
	ethTypeIPv6 = 0xDD86 // htons(ETH_P_IPV6=0x86DD) on a little-endian host

	ipProtoOff = 9  // iphdr.protocol
	ipSrcOff   = 12 // iphdr.saddr
	ipDstOff   = 16 // iphdr.daddr

	ipv6NextHdrOff = 6  // ipv6hdr.nexthdr
	ipv6SrcOff     = 8  // ipv6hdr.saddr (16 bytes)
	ipv6DstOff     = 24 // ipv6hdr.daddr (16 bytes)

	protoTCP    = 6
	protoUDP    = 17
	protoICMPv6 = 58

	tcpHdrLen = 20
	udpHdrLen = 8

	// __sk_buff field offsets.
	skbLen      = 0
	skbData     = 76
	skbDataEnd  = 80
	l4DstOffset = 2 // TCP/UDP destination port (ingress key)
	l4SrcOffset = 0 // TCP/UDP source port (egress key)

	// l4PortLen is the size of a TCP or UDP port, the bytes bpf_skb_load_bytes
	// copies.
	l4PortLen = 2

	// Stack slots (all below the frame pointer, non-overlapping, aligned).
	slotIPKey     = -4  // u32 IPv4 key (-4..-1)
	slotPortKey   = -8  // u32 {port,proto,pad} key (-8..-5)
	slotValue     = -16 // u64 byte count for map insert (-16..-9)
	slotPortBytes = -24 // u16 L4 port in network order (-24..-23), as bpf_skb_load_bytes copies it
	slotIP6Key    = -40 // 16-byte IPv6 key (-40..-25), 8-byte aligned
	slotIsIPv6    = -44 // u32 flag: 0 for IPv4, 1 for IPv6 (-44..-41; track-ip only)
)

// bpfPortKey mirrors the kernel port_proto key {u16 port; u8 proto; u8 pad}.
// Port is host byte order (the program applies ntohs); the pad byte is always 0.
type bpfPortKey struct {
	Port     uint16
	Protocol uint8
	Pad      uint8
}

// newLRUMap returns a per-interface LRU hash map spec with the given key size
// (4 bytes for port_proto and IPv4 keys, 16 bytes for IPv6 keys).
func newLRUMap(maxEntries, keySize uint32) *ebpf.MapSpec {
	return &ebpf.MapSpec{
		Type:       ebpf.LRUHash,
		KeySize:    keySize,
		ValueSize:  8,
		MaxEntries: maxEntries,
	}
}

// buildCollectionSpec assembles the full eBPF collection in pure Go: per-(port,
// protocol) accounting always, and per-IP (IPv4 and IPv6) accounting only when
// trackIP is set (so disabling track-ip removes all four IP maps and the
// per-packet IP work).
func buildCollectionSpec(maxEntries uint32, trackIP bool) *ebpf.CollectionSpec {
	maps := map[string]*ebpf.MapSpec{
		mapPortIngress: newLRUMap(maxEntries, keySizePort),
		mapPortEgress:  newLRUMap(maxEntries, keySizePort),
	}
	if trackIP {
		maps[mapIPIngress] = newLRUMap(maxEntries, keySizeIPv4)
		maps[mapIPEgress] = newLRUMap(maxEntries, keySizeIPv4)
		maps[mapIP6Ingress] = newLRUMap(maxEntries, keySizeIPv6)
		maps[mapIP6Egress] = newLRUMap(maxEntries, keySizeIPv6)
	}

	return &ebpf.CollectionSpec{
		Maps: maps,
		Programs: map[string]*ebpf.ProgramSpec{
			progIngress: {
				Type:         ebpf.SchedCLS,
				License:      bpfLicense,
				Instructions: directionInsns(ipSrcOff, ipv6SrcOff, l4DstOffset, mapIPIngress, mapIP6Ingress, mapPortIngress, trackIP),
			},
			progEgress: {
				Type:         ebpf.SchedCLS,
				License:      bpfLicense,
				Instructions: directionInsns(ipDstOff, ipv6DstOff, l4SrcOffset, mapIPEgress, mapIP6Egress, mapPortEgress, trackIP),
			},
		},
	}
}

// directionInsns builds one TCX program covering IPv4 and IPv6. ipKeyOff and
// ip6KeyOff select which address is keyed (source for ingress, destination for
// egress); portOff selects which L4 port is keyed (destination for ingress,
// source for egress).
//
// Structure: read the fixed-offset L2/L3 fields (EtherType, IP key, IPv4 IHL
// and protocol or IPv6 Next Header) through the packet pointer, then copy the
// L4 port to the stack with bpf_skb_load_bytes at the SCALAR offset
// (14 + ihl*4 + portOff for IPv4, 54 + portOff for IPv6), then run the map
// operations. IPv6 parses only the fixed 40-byte base header; packets carrying
// extension headers count per-IP and under {port: 0, proto: first NextHeader}.
//
// The port is never read through a packet pointer, because IPv4 carries the
// variable offset ihl*4. Without CAP_PERFMON the verifier accepts arithmetic on
// a packet pointer only with an immediate operand. For a loader without
// bypass_spec_v1, can_skip_alu_sanitation skips the speculation masking only
// when "BPF_SRC(insn->code) == BPF_K" (or the verifier already fenced the
// instruction), so a register operand, even one holding a known constant, goes
// on to retrieve_ptr_limit, which answers REASON_TYPE for every pointer type
// but the stack and a map value. sanitize_err reports that as "pointer
// arithmetic with it prohibited for !root" (kernel/bpf/verifier.c v7.0). The
// systemd unit does not grant CAP_PERFMON, and a helper read with a scalar
// offset needs no pointer arithmetic at all.
//
// No packet pointer is dereferenced after a helper call: every packet-pointer
// read sits before bpf_skb_load_bytes, so the verifier never has to re-validate
// packet bounds. Every path returns TCX_NEXT (tcxNext); the program never drops
// and never ends the hook.
func directionInsns(ipKeyOff, ip6KeyOff, portOff int16, ipMap, ip6Map, portMap string, trackIP bool) asm.Instructions {
	// Callee-saved registers (they survive helper calls): R6=skb (the helper's
	// context argument), R7=IPv4 protocol or IPv6 Next Header, R8=skb->len,
	// R9=L4 header offset from skb->data. R2=data and R3=data_end live only in
	// the parse, which makes no helper call.
	ins := asm.Instructions{
		asm.Mov.Reg(asm.R6, asm.R1),
		asm.LoadMem(asm.R8, asm.R1, skbLen, asm.Word),
		asm.LoadMem(asm.R2, asm.R1, skbData, asm.Word),
		asm.LoadMem(asm.R3, asm.R1, skbDataEnd, asm.Word),

		// Ethernet header bounds: data + 14 > data_end -> pass.
		asm.Mov.Reg(asm.R0, asm.R2),
		asm.Add.Imm(asm.R0, ethHdrLen),
		asm.JGT.Reg(asm.R0, asm.R3, "ret0"),

		// Dispatch on EtherType: IPv4 falls through, IPv6 jumps to parse_v6,
		// everything else passes untouched.
		asm.LoadMem(asm.R0, asm.R2, 12, asm.Half),
		asm.JEq.Imm(asm.R0, ethTypeIPv6, "parse_v6"),
		asm.JNE.Imm(asm.R0, ethTypeIPv4, "ret0"),

		// IPv4 fixed-header bounds: data + 34 > data_end -> pass. Every
		// packet-pointer read below sits at a fixed offset inside these 34 bytes.
		asm.Mov.Reg(asm.R0, asm.R2),
		asm.Add.Imm(asm.R0, ethHdrLen+ipHdrLen),
		asm.JGT.Reg(asm.R0, asm.R3, "ret0"),
	}

	// Stage the IPv4 key on the stack (only when track-ip accounts per-IP).
	// Zeroing slotIP6Key keeps every stack byte initialized on both branches so
	// the !CAP_PERFMON verifier accepts speculative exploration of ip6_ops.
	if trackIP {
		ins = append(ins,
			asm.LoadMem(asm.R0, asm.R2, ethHdrLen+ipKeyOff, asm.Word), // raw network-order IPv4
			asm.StoreMem(asm.RFP, slotIPKey, asm.R0, asm.Word),
			asm.Mov.Imm(asm.R0, 0),
			asm.StoreMem(asm.RFP, slotIP6Key, asm.R0, asm.DWord),
			asm.StoreMem(asm.RFP, slotIP6Key+8, asm.R0, asm.DWord),
			asm.StoreMem(asm.RFP, slotIsIPv6, asm.R0, asm.Word),
		)
	}

	// Compute the IPv4 protocol and scalar L4 offset, then join shared l4_parse.
	ins = append(ins,
		asm.LoadMem(asm.R7, asm.R2, ethHdrLen+ipProtoOff, asm.Byte),

		// RFC 791 Section 3.1: "Internet Header Length is the length of the
		// internet header in 32 bit words, and thus points to the beginning of
		// the data."
		asm.LoadMem(asm.R9, asm.R2, ethHdrLen, asm.Byte), // version<<4 | ihl
		asm.And.Imm(asm.R9, 0x0F),
		asm.LSh.Imm(asm.R9, 2),         // ihl * 4 (0..60)
		asm.Add.Imm(asm.R9, ethHdrLen), // R9 = L4 header offset (14..74), a scalar
		asm.Ja.Label("l4_parse"),

		// IPv6 fixed base-header bounds: data + 54 > data_end -> pass. Every
		// packet-pointer read below sits at a fixed offset inside these 54 bytes.
		asm.Mov.Reg(asm.R0, asm.R2).WithSymbol("parse_v6"),
		asm.Add.Imm(asm.R0, ethHdrLen+ipv6HdrLen),
		asm.JGT.Reg(asm.R0, asm.R3, "ret0"),
	)

	// Stage the 16-byte IPv6 key on the stack via two fixed-offset DWord loads
	// (zero pointer arithmetic). Zeroing slotIPKey keeps every stack byte
	// initialized on both branches for !CAP_PERFMON speculative verification.
	if trackIP {
		ins = append(ins,
			asm.LoadMem(asm.R0, asm.R2, ethHdrLen+ip6KeyOff, asm.DWord),
			asm.StoreMem(asm.RFP, slotIP6Key, asm.R0, asm.DWord),
			asm.LoadMem(asm.R0, asm.R2, ethHdrLen+ip6KeyOff+8, asm.DWord),
			asm.StoreMem(asm.RFP, slotIP6Key+8, asm.R0, asm.DWord),
			asm.StoreImm(asm.RFP, slotIPKey, 0, asm.Word),
			asm.StoreImm(asm.RFP, slotIsIPv6, 1, asm.Word),
		)
	}

	// IPv6 Next Header (first header after the fixed 40-byte base header) and
	// scalar L4 offset (54), falling through to shared l4_parse.
	ins = append(ins,
		asm.LoadMem(asm.R7, asm.R2, ethHdrLen+ipv6NextHdrOff, asm.Byte),
		asm.Mov.Imm(asm.R9, ethHdrLen+ipv6HdrLen),

		// Shared L4 parse: the port defaults to 0 (ICMP, ICMPv6, GRE, IPv6
		// extension headers, or a truncated L4 header).
		asm.StoreImm(asm.RFP, slotPortBytes, 0, asm.Half).WithSymbol("l4_parse"),

		// TCP? The whole header must lie inside skb->len, or the port stays 0.
		asm.JNE.Imm(asm.R7, protoTCP, "check_udp"),
		asm.Mov.Reg(asm.R0, asm.R9),
		asm.Add.Imm(asm.R0, tcpHdrLen),
		asm.JGT.Reg(asm.R0, asm.R8, "build_key"),
		asm.Ja.Label("load_port"),

		// UDP? The same rule for its 8-byte header.
		asm.JNE.Imm(asm.R7, protoUDP, "build_key").WithSymbol("check_udp"),
		asm.Mov.Reg(asm.R0, asm.R9),
		asm.Add.Imm(asm.R0, udpHdrLen),
		asm.JGT.Reg(asm.R0, asm.R8, "build_key"),

		// bpf_skb_load_bytes(skb, l4 + portOff, fp + slotPortBytes, 2). It reads
		// the skb's paged data as well as its linear part, and writes only the
		// stack: no allocation.
		asm.Mov.Reg(asm.R1, asm.R6).WithSymbol("load_port"),
		asm.Mov.Reg(asm.R2, asm.R9),
		asm.Add.Imm(asm.R2, int32(portOff)),
		asm.Mov.Reg(asm.R3, asm.RFP),
		asm.Add.Imm(asm.R3, slotPortBytes),
		asm.Mov.Imm(asm.R4, l4PortLen),
		asm.FnSkbLoadBytes.Call(),
		asm.JEq.Imm(asm.R0, 0, "build_key"),
		// A refused read leaves the port 0. The helper zeroes the buffer when
		// it fails; this store states it rather than relying on that.
		asm.StoreImm(asm.RFP, slotPortBytes, 0, asm.Half),

		// key = ntohs(port) | (proto << 16); pad byte (bits 24..31) stays 0.
		asm.LoadMem(asm.R4, asm.RFP, slotPortBytes, asm.Half).WithSymbol("build_key"),
		asm.HostTo(asm.BE, asm.R4, asm.Half), // ntohs
		asm.Mov.Reg(asm.R0, asm.R7),
		asm.LSh.Imm(asm.R0, 16),
		asm.Or.Reg(asm.R4, asm.R0),
		asm.StoreMem(asm.RFP, slotPortKey, asm.R4, asm.Word),
	)

	// Map operations: no packet pointer is read past this point, only the
	// stack. Port accounting first.
	portInsertCont := "ret0"
	if trackIP {
		portInsertCont = "ip_ops"
	}
	ins = append(ins, accountInsns(portMap, slotPortKey, "port_add", portInsertCont)...)

	// Per-IP accounting (optional). Branches on slotIsIPv6 between the 4-byte
	// IPv4 map and the 16-byte IPv6 map. accountInsns falls through after its
	// StoreXAdd, so an explicit jump to ret0 stops IPv4 from entering ip6_ops.
	if trackIP {
		ins = append(ins,
			asm.LoadMem(asm.R0, asm.RFP, slotIsIPv6, asm.Word).WithSymbol("ip_ops"),
			asm.JNE.Imm(asm.R0, 0, "ip6_ops"),
		)
		ins = append(ins, accountInsns(ipMap, slotIPKey, "ip_add", "ret0")...)
		ins = append(ins, asm.Ja.Label("ret0"))

		ip6Ops := accountInsns(ip6Map, slotIP6Key, "ip6_add", "ret0")
		ip6Ops[0] = ip6Ops[0].WithSymbol("ip6_ops")
		ins = append(ins, ip6Ops...)
	}

	// The one exit, labeled ret0 for the branches above: TCX_NEXT, so the next
	// TCX program and the clsact filters still run (tcxNext).
	ins = append(ins,
		asm.Mov.Imm(asm.R0, tcxNext).WithSymbol("ret0"),
		asm.Return(),
	)
	return ins
}

// accountInsns emits the lookup/atomic-add-or-insert idiom for one map: look up
// the key staged at keySlot; if present, atomically add skb->len (R8); otherwise
// insert skb->len. addLabel marks the atomic-add instruction; contLabel is where
// the insert branch jumps to continue.
func accountInsns(mapName string, keySlot int16, addLabel, contLabel string) asm.Instructions {
	return asm.Instructions{
		asm.LoadMapPtr(asm.R1, 0).WithReference(mapName),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, int32(keySlot)),
		asm.FnMapLookupElem.Call(),
		asm.JNE.Imm(asm.R0, 0, addLabel),

		// not found: *(u64*)(fp+slotValue) = len; map_update_elem(map, &key, &val, BPF_ANY)
		asm.StoreMem(asm.RFP, slotValue, asm.R8, asm.DWord),
		asm.LoadMapPtr(asm.R1, 0).WithReference(mapName),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, int32(keySlot)),
		asm.Mov.Reg(asm.R3, asm.RFP),
		asm.Add.Imm(asm.R3, slotValue),
		asm.Mov.Imm(asm.R4, 0), // BPF_ANY
		asm.FnMapUpdateElem.Call(),
		asm.Ja.Label(contLabel),

		// found: *ptr += len.
		asm.StoreXAdd(asm.R0, asm.R8, asm.DWord).WithSymbol(addLabel),
	}
}
