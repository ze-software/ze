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
)

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

	// Stack slots (all below the frame pointer, non-overlapping, aligned).
	slotIPKey   = -4  // u32 IP key
	slotPortKey = -8  // u32 {port,proto,pad} key
	slotValue   = -16 // u64 byte count for map insert
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
				License:      "GPL",
				Instructions: directionInsns(ipSrcOff, l4DstOffset, mapIPIngress, mapPortIngress, trackIP),
			},
			progEgress: {
				Type:         ebpf.SchedCLS,
				License:      "GPL",
				Instructions: directionInsns(ipDstOff, l4SrcOffset, mapIPEgress, mapPortEgress, trackIP),
			},
		},
	}
}

// directionInsns builds one TCX program. ipKeyOff selects which IPv4 address is
// keyed (source for ingress, destination for egress); portOff selects which L4
// port is keyed (destination for ingress, source for egress).
//
// Structure: parse the packet and stage both keys on the stack FIRST, then run
// all map operations. No packet pointer is dereferenced after a helper call, so
// the verifier never has to re-validate packet bounds. Every path returns
// TC_ACT_OK (0); the program never drops.
func directionInsns(ipKeyOff, portOff int16, ipMap, portMap string, trackIP bool) asm.Instructions {
	// callee-saved (survive helper calls): R6=data, R7=data_end, R8=skb->len,
	// R9=iphdr. Packet reads all happen before the first call.
	ins := asm.Instructions{
		asm.LoadMem(asm.R8, asm.R1, skbLen, asm.Word),
		asm.LoadMem(asm.R6, asm.R1, skbData, asm.Word),
		asm.LoadMem(asm.R7, asm.R1, skbDataEnd, asm.Word),

		// Ethernet header bounds: data + 14 > data_end -> pass.
		asm.Mov.Reg(asm.R0, asm.R6),
		asm.Add.Imm(asm.R0, ethHdrLen),
		asm.JGT.Reg(asm.R0, asm.R7, "ret0"),

		// EtherType must be IPv4.
		asm.LoadMem(asm.R0, asm.R6, 12, asm.Half),
		asm.JNE.Imm(asm.R0, ethTypeIPv4, "ret0"),

		// IPv4 fixed-header bounds: data + 34 > data_end -> pass.
		asm.Mov.Reg(asm.R0, asm.R6),
		asm.Add.Imm(asm.R0, ethHdrLen+ipHdrLen),
		asm.JGT.Reg(asm.R0, asm.R7, "ret0"),

		// R9 = iphdr.
		asm.Mov.Reg(asm.R9, asm.R6),
		asm.Add.Imm(asm.R9, ethHdrLen),
	}

	// Stage the IP key on the stack (only when track-ip accounts per-IP).
	if trackIP {
		ins = append(ins,
			asm.LoadMem(asm.R0, asm.R9, ipKeyOff, asm.Word), // raw network-order IPv4
			asm.StoreMem(asm.RFP, slotIPKey, asm.R0, asm.Word),
		)
	}

	// Compute the (port, protocol) key. transport = iphdr + ihl*4.
	ins = append(ins,
		asm.LoadMem(asm.R1, asm.R9, 0, asm.Byte), // version<<4 | ihl
		asm.And.Imm(asm.R1, 0x0F),
		asm.LSh.Imm(asm.R1, 2), // ihl * 4 (0..60)
		asm.Mov.Reg(asm.R2, asm.R9),
		asm.Add.Reg(asm.R2, asm.R1), // R2 = transport header
		asm.LoadMem(asm.R3, asm.R9, ipProtoOff, asm.Byte),
		asm.Mov.Imm(asm.R4, 0), // port defaults to 0 (ICMP/GRE/truncated)

		// TCP?
		asm.JNE.Imm(asm.R3, protoTCP, "check_udp"),
		asm.Mov.Reg(asm.R0, asm.R2),
		asm.Add.Imm(asm.R0, tcpHdrLen),
		asm.JGT.Reg(asm.R0, asm.R7, "build_key"), // truncated L4 -> port stays 0
		asm.LoadMem(asm.R4, asm.R2, portOff, asm.Half),
		asm.HostTo(asm.BE, asm.R4, asm.Half), // ntohs
		asm.Ja.Label("build_key"),

		// UDP?
		asm.JNE.Imm(asm.R3, protoUDP, "build_key").WithSymbol("check_udp"),
		asm.Mov.Reg(asm.R0, asm.R2),
		asm.Add.Imm(asm.R0, udpHdrLen),
		asm.JGT.Reg(asm.R0, asm.R7, "build_key"),
		asm.LoadMem(asm.R4, asm.R2, portOff, asm.Half),
		asm.HostTo(asm.BE, asm.R4, asm.Half),

		// key = port | (proto << 16); pad byte (bits 24..31) stays 0.
		asm.Mov.Reg(asm.R0, asm.R3).WithSymbol("build_key"),
		asm.LSh.Imm(asm.R0, 16),
		asm.Or.Reg(asm.R4, asm.R0),
		asm.StoreMem(asm.RFP, slotPortKey, asm.R4, asm.Word),
	)

	// Map operations (helper calls only past this point). Port accounting first.
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

	ins = append(ins,
		asm.Mov.Imm(asm.R0, 0).WithSymbol("ret0"),
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
