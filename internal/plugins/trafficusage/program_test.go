//go:build integration && linux

// VALIDATES: the hand-assembled eBPF programs parse Ethernet/IPv4/TCP/UDP,
// account bytes per source/dest IP and per (port,protocol) into the right maps,
// pass non-IPv4 and truncated packets without touching the maps, and accumulate.
// There is no compiler for this assembly, so BPF_PROG_TEST_RUN is the only thing
// that proves the bytecode is correct (spec assumptions A-1/A-2, ACs 1-6, 9).
// PREVENTS: silent miscounting (wrong offset/byte-order), out-of-bounds reads,
// or dropping legitimate traffic.

package trafficusage

import (
	"encoding/binary"
	"errors"
	"syscall"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
)

func requireBPF(t *testing.T) {
	t.Helper()
	if err := rlimit.RemoveMemlock(); err != nil {
		t.Skipf("eBPF unavailable (need CAP_BPF/CAP_SYS_RESOURCE): %v", err)
	}
}

func loadColl(t *testing.T, trackIP bool) *ebpf.Collection {
	t.Helper()
	requireBPF(t)
	coll, err := ebpf.NewCollection(buildCollectionSpec(1024, trackIP))
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skipf("no permission to load BPF: %v", err)
		}
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			t.Fatalf("BPF verifier rejected program:\n%+v", ve)
		}
		t.Fatalf("load collection: %v", err)
	}
	return coll
}

func ipv4Key(ip [4]byte) uint32 { return binary.LittleEndian.Uint32(ip[:]) }

// ethIPv4 builds an Ethernet+IPv4 frame carrying l4, padded to at least 64 bytes
// so skb->len is unambiguous.
func ethIPv4(proto byte, src, dst [4]byte, l4 []byte) []byte {
	pkt := []byte{
		0, 0, 0, 0, 0, 1, // dst mac
		0, 0, 0, 0, 0, 2, // src mac
		0x08, 0x00, // ethertype IPv4
		0x45, 0x00, 0, 0, 0, 0, 0, 0, 64, proto, 0, 0, // IPv4 hdr (ver/ihl, ... ttl, proto, csum)
	}
	pkt = append(pkt, src[:]...)
	pkt = append(pkt, dst[:]...)
	pkt = append(pkt, l4...)
	for len(pkt) < 64 {
		pkt = append(pkt, 0)
	}
	return pkt
}

func tcpHdr(srcPort, dstPort uint16) []byte {
	b := make([]byte, 20)
	binary.BigEndian.PutUint16(b[0:], srcPort)
	binary.BigEndian.PutUint16(b[2:], dstPort)
	return b
}

func udpHdr(srcPort, dstPort uint16) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint16(b[0:], srcPort)
	binary.BigEndian.PutUint16(b[2:], dstPort)
	return b
}

func runProg(t *testing.T, prog *ebpf.Program, pkt []byte) {
	t.Helper()
	ret, _, err := prog.Test(pkt)
	if err != nil {
		t.Fatalf("BPF_PROG_TEST_RUN: %v", err)
	}
	if ret != 0 {
		t.Fatalf("program returned %d, want 0 (TC_ACT_OK)", ret)
	}
}

func lookupU64(t *testing.T, m *ebpf.Map, key any) (uint64, bool) {
	t.Helper()
	var v uint64
	err := m.Lookup(key, &v)
	if errors.Is(err, ebpf.ErrKeyNotExist) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("map lookup: %v", err)
	}
	return v, true
}

func TestProgram_Loads(t *testing.T) {
	coll := loadColl(t, true)
	defer coll.Close()
	for _, name := range []string{progIngress, progEgress} {
		if coll.Programs[name] == nil {
			t.Errorf("program %q missing", name)
		}
	}
	for _, name := range []string{mapIPIngress, mapIPEgress, mapPortIngress, mapPortEgress} {
		if coll.Maps[name] == nil {
			t.Errorf("map %q missing", name)
		}
	}
}

func TestProgram_TrackIPOffOmitsIPMaps(t *testing.T) {
	coll := loadColl(t, false)
	defer coll.Close()
	if coll.Maps[mapIPIngress] != nil || coll.Maps[mapIPEgress] != nil {
		t.Error("IP maps must not exist when track-ip is off")
	}
	if coll.Maps[mapPortIngress] == nil || coll.Maps[mapPortEgress] == nil {
		t.Error("port maps must always exist")
	}
}

func TestProgram_IngressSrcIP(t *testing.T) {
	coll := loadColl(t, true)
	defer coll.Close()
	src := [4]byte{10, 0, 0, 1}
	dst := [4]byte{10, 0, 0, 2}
	pkt := ethIPv4(protoTCP, src, dst, tcpHdr(1234, 80))
	runProg(t, coll.Programs[progIngress], pkt)

	got, ok := lookupU64(t, coll.Maps[mapIPIngress], ipv4Key(src))
	if !ok {
		t.Fatal("ingress source IP not counted")
	}
	if got != uint64(len(pkt)) {
		t.Errorf("ingress[src] = %d, want %d (skb->len)", got, len(pkt))
	}
	// Egress IP must be unaffected on an ingress run.
	if _, ok := lookupU64(t, coll.Maps[mapIPEgress], ipv4Key(dst)); ok {
		t.Error("egress IP map written by ingress program")
	}
}

func TestProgram_EgressDstIP(t *testing.T) {
	coll := loadColl(t, true)
	defer coll.Close()
	src := [4]byte{192, 168, 1, 5}
	dst := [4]byte{8, 8, 8, 8}
	pkt := ethIPv4(protoUDP, src, dst, udpHdr(5000, 53))
	runProg(t, coll.Programs[progEgress], pkt)

	got, ok := lookupU64(t, coll.Maps[mapIPEgress], ipv4Key(dst))
	if !ok {
		t.Fatal("egress destination IP not counted")
	}
	if got != uint64(len(pkt)) {
		t.Errorf("egress[dst] = %d, want %d", got, len(pkt))
	}
}

func TestProgram_IngressDstPortTCP(t *testing.T) {
	coll := loadColl(t, false)
	defer coll.Close()
	pkt := ethIPv4(protoTCP, [4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, tcpHdr(1234, 443))
	runProg(t, coll.Programs[progIngress], pkt)

	key := bpfPortKey{Port: 443, Protocol: protoTCP}
	got, ok := lookupU64(t, coll.Maps[mapPortIngress], key)
	if !ok {
		t.Fatal("ingress dst port/proto not counted")
	}
	if got != uint64(len(pkt)) {
		t.Errorf("ingress port[443,tcp] = %d, want %d", got, len(pkt))
	}
}

func TestProgram_EgressSrcPortUDP(t *testing.T) {
	coll := loadColl(t, false)
	defer coll.Close()
	pkt := ethIPv4(protoUDP, [4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, udpHdr(12345, 53))
	runProg(t, coll.Programs[progEgress], pkt)

	key := bpfPortKey{Port: 12345, Protocol: protoUDP}
	got, ok := lookupU64(t, coll.Maps[mapPortEgress], key)
	if !ok {
		t.Fatal("egress src port/proto not counted")
	}
	if got != uint64(len(pkt)) {
		t.Errorf("egress port[12345,udp] = %d, want %d", got, len(pkt))
	}
}

func TestProgram_ICMPPortZero(t *testing.T) {
	coll := loadColl(t, false)
	defer coll.Close()
	// ICMP (proto 1) has no L4 port; the key must be {port:0, proto:1}.
	icmp := []byte{8, 0, 0, 0} // type, code, csum
	pkt := ethIPv4(1, [4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, icmp)
	runProg(t, coll.Programs[progIngress], pkt)

	key := bpfPortKey{Port: 0, Protocol: 1}
	if _, ok := lookupU64(t, coll.Maps[mapPortIngress], key); !ok {
		t.Fatal("ICMP not counted under {port:0, proto:1}")
	}
}

func TestProgram_NonIPv4Pass(t *testing.T) {
	coll := loadColl(t, true)
	defer coll.Close()
	// ARP ethertype 0x0806 -> not IPv4.
	pkt := make([]byte, 64)
	copy(pkt[12:14], []byte{0x08, 0x06})
	runProg(t, coll.Programs[progIngress], pkt)

	for _, m := range []*ebpf.Map{coll.Maps[mapIPIngress], coll.Maps[mapPortIngress]} {
		var k, v []byte
		it := m.Iterate()
		if it.Next(&k, &v) {
			t.Error("non-IPv4 packet wrote to a map")
		}
	}
}

func TestProgram_TruncatedPass(t *testing.T) {
	coll := loadColl(t, false)
	defer coll.Close()
	// Ethernet + IPv4 ethertype but only a few IP bytes: fails the IP bounds
	// check, so the program must pass without writing or reading out of bounds.
	pkt := []byte{
		0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 2, 0x08, 0x00,
		0x45, 0x00, 0x00, 0x00, // 4 IP bytes only
	}
	runProg(t, coll.Programs[progIngress], pkt)

	m := coll.Maps[mapPortIngress]
	var k, v []byte
	if m.Iterate().Next(&k, &v) {
		t.Error("truncated packet wrote to a map")
	}
}

func TestProgram_ByteAccumulation(t *testing.T) {
	coll := loadColl(t, false)
	defer coll.Close()
	pkt := ethIPv4(protoTCP, [4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, tcpHdr(1111, 2222))
	prog := coll.Programs[progIngress]
	runProg(t, prog, pkt)
	runProg(t, prog, pkt)

	key := bpfPortKey{Port: 2222, Protocol: protoTCP}
	got, ok := lookupU64(t, coll.Maps[mapPortIngress], key)
	if !ok {
		t.Fatal("port/proto not counted")
	}
	if got != 2*uint64(len(pkt)) {
		t.Errorf("accumulated = %d, want %d (2 x skb->len)", got, 2*len(pkt))
	}
}
