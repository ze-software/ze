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
	mapPortIngress = "tu_port_in"
	mapPortEgress  = "tu_port_out"

	progIngress = "tu_ingress"
	progEgress  = "tu_egress"

	// bpfLicense is the license every trafficusage program declares: the
	// kernel refuses GPL-only helpers to a program that declares another.
	bpfLicense = "GPL"
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

// Header layout constants (IPv4 over Ethernet II).
const (
	ethHdrLen   = 14
	ipHdrLen    = 20    // fixed part; options handled via the IHL field
	ethTypeIPv4 = 0x008 // htons(ETH_P_IP=0x0800) on a little-endian host

	ipProtoOff = 9  // iphdr.protocol
	ipSrcOff   = 12 // iphdr.saddr
	ipDstOff   = 16 // iphdr.daddr

	protoTCP = 6
	protoUDP = 17

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
	slotIPKey     = -4  // u32 IP key
	slotPortKey   = -8  // u32 {port,proto,pad} key
	slotValue     = -16 // u64 byte count for map insert
	slotPortBytes = -24 // u16 L4 port in network order, as bpf_skb_load_bytes copies it
)

// bpfPortKey mirrors the kernel port_proto key {u16 port; u8 proto; u8 pad}.
// Port is host byte order (the program applies ntohs); the pad byte is always 0.
type bpfPortKey struct {
	Port     uint16
	Protocol uint8
	Pad      uint8
}

// newLRUMap returns a per-interface LRU hash map spec. Both the port_proto
// key ({u16 port; u8 proto; u8 pad}) and the IPv4 address key are 4 bytes.
func newLRUMap(maxEntries uint32) *ebpf.MapSpec {
	return &ebpf.MapSpec{
		Type:       ebpf.LRUHash,
		KeySize:    4,
		ValueSize:  8,
		MaxEntries: maxEntries,
	}
}

// buildCollectionSpec assembles the full eBPF collection in pure Go: per-(port,
// protocol) accounting always, and per-IP accounting only when trackIP is set
// (so disabling track-ip removes both the maps and the per-packet IP work).
func buildCollectionSpec(maxEntries uint32, trackIP bool) *ebpf.CollectionSpec {
	maps := map[string]*ebpf.MapSpec{
		mapPortIngress: newLRUMap(maxEntries),
		mapPortEgress:  newLRUMap(maxEntries),
	}
	if trackIP {
		maps[mapIPIngress] = newLRUMap(maxEntries)
		maps[mapIPEgress] = newLRUMap(maxEntries)
	}

	return &ebpf.CollectionSpec{
		Maps: maps,
		Programs: map[string]*ebpf.ProgramSpec{
			progIngress: {
				Type:         ebpf.SchedCLS,
				License:      bpfLicense,
				Instructions: directionInsns(ipSrcOff, l4DstOffset, mapIPIngress, mapPortIngress, trackIP),
			},
			progEgress: {
				Type:         ebpf.SchedCLS,
				License:      bpfLicense,
				Instructions: directionInsns(ipDstOff, l4SrcOffset, mapIPEgress, mapPortEgress, trackIP),
			},
		},
	}
}

// directionInsns builds one TCX program. ipKeyOff selects which IPv4 address is
// keyed (source for ingress, destination for egress); portOff selects which L4
// port is keyed (destination for ingress, source for egress).
//
// Structure: read the fixed-offset fields (EtherType, the IP key, the IHL byte
// and the protocol) through the packet pointer, then copy the L4 port to the
// stack with bpf_skb_load_bytes at the SCALAR offset 14 + ihl*4 + portOff, then
// run the map operations.
//
// The port is never read through a packet pointer, because that pointer would
// carry the variable offset ihl*4. Without CAP_PERFMON the verifier accepts
// arithmetic on a packet pointer only with an immediate operand. For a loader
// without bypass_spec_v1, can_skip_alu_sanitation skips the speculation
// masking only when "BPF_SRC(insn->code) == BPF_K" (or the verifier already
// fenced the instruction), so a register operand, even one holding a known
// constant, goes on to retrieve_ptr_limit, which answers REASON_TYPE for every
// pointer type but the stack and a map value. sanitize_err reports that as
// "pointer arithmetic with it prohibited for !root" (kernel/bpf/verifier.c
// v7.0). The systemd unit does not grant CAP_PERFMON, and a helper read with a
// scalar offset needs no pointer arithmetic at all.
//
// No packet pointer is dereferenced after a helper call: every packet-pointer
// read sits before bpf_skb_load_bytes, so the verifier never has to re-validate
// packet bounds. Every path returns TCX_NEXT (tcxNext); the program never drops
// and never ends the hook.
func directionInsns(ipKeyOff, portOff int16, ipMap, portMap string, trackIP bool) asm.Instructions {
	// Callee-saved registers (they survive helper calls): R6=skb (the helper's
	// context argument), R7=IPv4 protocol, R8=skb->len, R9=L4 header offset from
	// skb->data. R2=data and R3=data_end live only in the parse, which makes no
	// helper call.
	ins := asm.Instructions{
		asm.Mov.Reg(asm.R6, asm.R1),
		asm.LoadMem(asm.R8, asm.R1, skbLen, asm.Word),
		asm.LoadMem(asm.R2, asm.R1, skbData, asm.Word),
		asm.LoadMem(asm.R3, asm.R1, skbDataEnd, asm.Word),

		// Ethernet header bounds: data + 14 > data_end -> pass.
		asm.Mov.Reg(asm.R0, asm.R2),
		asm.Add.Imm(asm.R0, ethHdrLen),
		asm.JGT.Reg(asm.R0, asm.R3, "ret0"),

		// EtherType must be IPv4.
		asm.LoadMem(asm.R0, asm.R2, 12, asm.Half),
		asm.JNE.Imm(asm.R0, ethTypeIPv4, "ret0"),

		// IPv4 fixed-header bounds: data + 34 > data_end -> pass. Every
		// packet-pointer read below sits at a fixed offset inside these 34 bytes.
		asm.Mov.Reg(asm.R0, asm.R2),
		asm.Add.Imm(asm.R0, ethHdrLen+ipHdrLen),
		asm.JGT.Reg(asm.R0, asm.R3, "ret0"),
	}

	// Stage the IP key on the stack (only when track-ip accounts per-IP).
	if trackIP {
		ins = append(ins,
			asm.LoadMem(asm.R0, asm.R2, ethHdrLen+ipKeyOff, asm.Word), // raw network-order IPv4
			asm.StoreMem(asm.RFP, slotIPKey, asm.R0, asm.Word),
		)
	}

	// Compute the (port, protocol) key.
	ins = append(ins,
		asm.LoadMem(asm.R7, asm.R2, ethHdrLen+ipProtoOff, asm.Byte),

		// RFC 791 Section 3.1: "Internet Header Length is the length of the
		// internet header in 32 bit words, and thus points to the beginning of
		// the data."
		asm.LoadMem(asm.R9, asm.R2, ethHdrLen, asm.Byte), // version<<4 | ihl
		asm.And.Imm(asm.R9, 0x0F),
		asm.LSh.Imm(asm.R9, 2),         // ihl * 4 (0..60)
		asm.Add.Imm(asm.R9, ethHdrLen), // R9 = L4 header offset (14..74), a scalar

		// The port defaults to 0 (ICMP, GRE, a truncated L4 header).
		asm.StoreImm(asm.RFP, slotPortBytes, 0, asm.Half),

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

	// Per-IP accounting (optional). The first instruction carries "ip_ops" so the
	// port-insert branch lands here; it ends by falling through to ret0.
	if trackIP {
		ipOps := accountInsns(ipMap, slotIPKey, "ip_add", "ret0")
		ipOps[0] = ipOps[0].WithSymbol("ip_ops")
		ins = append(ins, ipOps...)
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
