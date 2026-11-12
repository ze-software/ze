//go:build integration && linux

// VALIDATES: the hand-assembled eBPF programs parse Ethernet/IPv4/TCP/UDP,
// account bytes per source/dest IP and per (port,protocol) into the right maps,
// pass non-IPv4 and truncated packets without touching the maps, and accumulate.
// There is no compiler for this assembly, so running it is the only thing that
// proves the bytecode is correct (spec assumptions A-1/A-2, ACs 1-6, 9). Most
// tests run it with BPF_PROG_TEST_RUN; a frame that input refuses goes through
// a veth pair with the programs attached by TCX (TestProgram_TruncatedPass).
// PREVENTS: silent miscounting (wrong offset/byte-order), out-of-bounds reads,
// or dropping legitimate traffic.

package trafficusage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"golang.org/x/sys/unix"
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
		if ve, ok := errors.AsType[*ebpf.VerifierError](err); ok {
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
	if got := tcxVerdict(ret); got != tcxNext {
		t.Fatalf("program returned %d, want %d (TCX_NEXT)", got, tcxNext)
	}
}

// tcxVerdict reads the retval BPF_PROG_TEST_RUN reports as the program's verdict.
// The kernel stores the program's int return in a u32 (bpf_test_run, retval), so
// TCX_NEXT (-1) arrives as 0xffffffff.
func tcxVerdict(ret uint32) int32 {
	return int32(ret) //nolint:gosec // the u32 holds the program's int return, -1 for TCX_NEXT
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

// TestProgram_TruncatedPass proves that a frame which carries the IPv4
// ethertype and ends inside the IPv4 header passes both programs and is not
// counted: each such frame takes the "data + 34 > data_end" branch to ret0. A
// frame of 34 bytes or more is always counted (port 0 when no L4 port fits),
// so only a frame shorter than the Ethernet and IPv4 headers together can
// show the branch writes nothing.
//
// BPF_PROG_TEST_RUN cannot feed such a frame to the program on Ze's kernel.
// bpf_prog_test_run_skb (net/bpf/test_run.c, v7.2) refuses an input shorter
// than ETH_HLEN, and after eth_type_trans it refuses an ETH_P_IP frame whose
// linear part holds less than an IPv4 header:
//
//	if (skb_headlen(skb) < sizeof(struct iphdr)) {
//		ret = -EINVAL;
//
// The program's data_end is the end of that linear part, and a context
// data_end shrinks the linear part the same check reads, so every IPv4 frame
// that route runs reaches data + 34. The real datapath carries these frames,
// so this test sends them through a veth pair with the programs attached by
// TCX. AF_PACKET accepts a frame as short as the 14-byte Ethernet header
// (dev_validate_header), and veth_xmit forwards it unpadded.
//
// Egress: the program runs in __dev_queue_xmit before the frame reaches the
// veth driver, so a frame that arrives on the peer has passed the program and
// was not dropped, and each one proves the branch writes no entry. Ingress:
// frames sent from one pinned CPU join that CPU's receive backlog in order
// (enqueue_to_backlog), so once a whole frame sent after them is counted, the
// ingress program has run on every truncated frame the hook received before
// it. The ingress leg sees no verdict: the TCX hook runs after the AF_PACKET
// taps, and a pass and a drop look alike from user space.
//
// The "data + 14 > data_end" branch stays out of reach of both routes, which
// carry no frame shorter than ETH_HLEN. The verifier proves that branch at
// load (TestProgram_Loads): without it, the read of the ethertype at offset 12
// is refused.
// MUTATION: point the IPv4-header bound at a new TC_ACT_SHOT (2) return instead
// of ret0: no egress frame reaches the peer and the test goes red.
func TestProgram_TruncatedPass(t *testing.T) {
	requireBPF(t)
	ifindexA := createVethPair(t, "tuv2a", "tuv2b")

	att, err := tcxAttacher{}.Attach(ifindexA, "tuv2a", 1024, true)
	if err != nil {
		t.Fatalf("AttachTCX: %v", err)
	}
	defer func() {
		if err := att.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	restore := pinToOneCPU(t)
	defer restore()

	frames := truncatedIPv4Frames()

	// Egress: send every truncated frame out tuv2a; each must reach tuv2b.
	peer := packetReceiver(t, "tuv2b")
	for _, frame := range frames {
		injectFrame(t, "tuv2a", frame)
	}
	awaitFrames(t, peer, frames)
	if err := unix.Close(peer); err != nil {
		t.Errorf("close the tuv2b receiver: %v", err)
	}

	// Ingress: send every truncated frame out tuv2b to tuv2a's ingress, then a
	// whole frame that orders after them.
	sentinelSrc := [4]byte{10, 9, 9, 9}
	sentinel := ethIPv4(protoUDP, sentinelSrc, [4]byte{10, 9, 9, 10}, udpHdr(4000, 9))
	sentinelKey := portProto{port: 9, proto: protoUDP}
	for _, frame := range frames {
		injectFrame(t, "tuv2b", frame)
	}
	injectFrame(t, "tuv2b", sentinel)
	c := waitCounts(t, att, func(c counts) bool { return c.ingressPort[sentinelKey] > 0 })

	// The pair carries no IPv4 address, so the kernel sends no IPv4 frame of
	// its own on it: an entry beside the sentinel's came from a truncated frame.
	if len(c.egressPort) != 0 || len(c.egressIP) != 0 {
		t.Errorf("truncated egress frames were counted: ports %v, addresses %v", c.egressPort, c.egressIP)
	}
	if len(c.ingressPort) != 1 || c.ingressPort[sentinelKey] != uint64(len(sentinel)) {
		t.Errorf("ingress ports = %v, want only %+v counted %d bytes (the sentinel)", c.ingressPort, sentinelKey, len(sentinel))
	}
	if len(c.ingressIP) != 1 || c.ingressIP[ipv4Key(sentinelSrc)] != uint64(len(sentinel)) {
		t.Errorf("ingress addresses = %v, want only %v counted %d bytes (the sentinel)", c.ingressIP, sentinelSrc, len(sentinel))
	}
}

// truncatedIPv4Frames returns one frame for each length from the Ethernet
// header alone (14 bytes) to one byte short of the IPv4 fixed header (33
// bytes). Each is a prefix of a TCP frame, so it carries the IPv4 ethertype and
// stops inside the IPv4 header.
func truncatedIPv4Frames() [][]byte {
	whole := ethIPv4Unpadded(protoTCP, 5, nil, tcpHdr(1234, 80))
	frames := make([][]byte, 0, ipHdrLen)
	for size := ethHdrLen; size < ethHdrLen+ipHdrLen; size++ {
		frames = append(frames, whole[:size:size])
	}
	return frames
}

// cpuSetBits is CPU_SETSIZE, the number of CPUs a unix.CPUSet holds.
const cpuSetBits = 1024

// pinToOneCPU locks the calling goroutine to its thread, and that thread to
// the first CPU it may run on. A frame a veth transmits joins the receive
// backlog of the CPU that sent it, and one backlog is processed in order, so
// frames sent from the pinned thread reach the peer's ingress in the order
// they were sent. The caller MUST call the returned function, on the same
// goroutine, before the test returns: it restores the affinity and unlocks
// the thread.
func pinToOneCPU(t *testing.T) func() {
	t.Helper()
	runtime.LockOSThread()
	var saved unix.CPUSet
	if err := unix.SchedGetaffinity(0, &saved); err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("read this thread's CPU affinity: %v", err)
	}
	cpu := -1
	for i := range cpuSetBits {
		if saved.IsSet(i) {
			cpu = i
			break
		}
	}
	if cpu < 0 {
		runtime.UnlockOSThread()
		t.Fatal("this thread's CPU affinity holds no CPU")
	}
	var one unix.CPUSet
	one.Set(cpu)
	if err := unix.SchedSetaffinity(0, &one); err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("pin this thread to CPU %d: %v", cpu, err)
	}
	return func() {
		if err := unix.SchedSetaffinity(0, &saved); err != nil {
			t.Errorf("restore this thread's CPU affinity: %v", err)
		}
		runtime.UnlockOSThread()
	}
}

// packetReceiver opens an AF_PACKET socket bound to ifname that receives every
// frame arriving on it, with a 100 ms read timeout so a reader can keep a
// deadline. Opened before a frame is sent, it sees that frame. The caller MUST
// close the returned descriptor.
func packetReceiver(t *testing.T, ifname string) int {
	t.Helper()
	ifi, err := net.InterfaceByName(ifname)
	if err != nil {
		t.Fatalf("InterfaceByName %s: %v", ifname, err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		t.Skipf("cannot open AF_PACKET socket (need CAP_NET_RAW): %v", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_ALL), Ifindex: ifi.Index}); err != nil {
		_ = unix.Close(fd)
		t.Fatalf("bind the receiver to %s: %v", ifname, err)
	}
	timeout := unix.Timeval{Usec: 100_000}
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		_ = unix.Close(fd)
		t.Fatalf("set the receiver's read timeout: %v", err)
	}
	return fd
}

// awaitFrames reads fd until every frame in want has arrived byte for byte, and
// fails when one has not arrived within three seconds. Other frames, such as
// the IPv6 neighbor discovery a link sends when it comes up, are skipped.
func awaitFrames(t *testing.T, fd int, want [][]byte) {
	t.Helper()
	missing := make(map[int]bool, len(want))
	for i := range want {
		missing[i] = true
	}
	buf := make([]byte, 2048)
	deadline := time.Now().Add(3 * time.Second)
	for len(missing) > 0 && time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if errors.Is(err, unix.EAGAIN) {
			continue // the read timeout passed; check the deadline
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("receive a frame: %v", err)
		}
		for i := range missing {
			if bytes.Equal(buf[:n], want[i]) {
				delete(missing, i)
				break
			}
		}
	}
	if len(missing) == 0 {
		return
	}
	var lengths []int
	for i, frame := range want {
		if missing[i] {
			lengths = append(lengths, len(frame))
		}
	}
	t.Fatalf("%d of %d frames never reached the peer within 3s (the egress program drops them, "+
		"or the veth pair does not carry them): lengths %v", len(lengths), len(want), lengths)
}

// TestProgram_IPv4HeaderEndingTheFrameCounts proves the IPv4-header bound lets
// equality through: a frame that ends exactly where the 20-byte IPv4 header
// ends (34 bytes, data + 34 == data_end) is counted under port 0 and its
// source address, at skb->len. It is the shortest IPv4 frame BPF_PROG_TEST_RUN
// runs on Ze's kernel (see TestProgram_TruncatedPass), so it is the truncation
// boundary as that route reaches it; TestProgram_TruncatedPass proves the
// frames one byte and more below it.
// MUTATION: make the IPv4-header bound refuse at equality (JGE for JGT): the
// frame is not counted and the test goes red.
func TestProgram_IPv4HeaderEndingTheFrameCounts(t *testing.T) {
	coll := loadColl(t, true)
	defer coll.Close()
	pkt := ethIPv4Unpadded(protoTCP, 5, nil, nil)
	if len(pkt) != ethHdrLen+ipHdrLen {
		t.Fatalf("frame is %d bytes, want %d (Ethernet and IPv4 headers only)", len(pkt), ethHdrLen+ipHdrLen)
	}
	runProg(t, coll.Programs[progIngress], pkt)

	key, got := onlyPortEntry(t, coll.Maps[mapPortIngress])
	want := bpfPortKey{Port: 0, Protocol: protoTCP}
	if key != want {
		t.Errorf("keyed %+v, want %+v (no TCP header fits)", key, want)
	}
	if got != uint64(len(pkt)) {
		t.Errorf("counted %d bytes, want %d (skb->len)", got, len(pkt))
	}
	src := [4]byte{10, 0, 0, 1} // ethIPv4Unpadded's saddr
	counted, ok := lookupU64(t, coll.Maps[mapIPIngress], ipv4Key(src))
	if !ok {
		t.Fatalf("ingress source %v not counted", src)
	}
	if counted != uint64(len(pkt)) {
		t.Errorf("ingress[%v] = %d, want %d (skb->len)", src, counted, len(pkt))
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

// ethIPv4Unpadded builds an Ethernet+IPv4 frame whose IPv4 header carries ihl
// and options, followed by l4, with no padding: skb->len is exactly the frame,
// so a test controls where the packet ends.
func ethIPv4Unpadded(proto, ihl byte, options, l4 []byte) []byte {
	pkt := []byte{
		0, 0, 0, 0, 0, 1, // dst mac
		0, 0, 0, 0, 0, 2, // src mac
		0x08, 0x00, // ethertype IPv4
		0x40 | ihl, 0x00, 0, 0, 0, 0, 0, 0, 64, proto, 0, 0, // IPv4 hdr (ver/ihl, ... ttl, proto, csum)
		10, 0, 0, 1, // saddr
		10, 0, 0, 2, // daddr
	}
	pkt = append(pkt, options...)
	return append(pkt, l4...)
}

// onlyPortEntry returns the one entry of a port map, and fails unless the map
// holds exactly one.
func onlyPortEntry(t *testing.T, m *ebpf.Map) (bpfPortKey, uint64) {
	t.Helper()
	var k, key bpfPortKey
	var v, value uint64
	entries := 0
	it := m.Iterate()
	for it.Next(&k, &v) {
		entries++
		key, value = k, v
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterate port map: %v", err)
	}
	if entries != 1 {
		t.Fatalf("port map holds %d entries, want exactly 1", entries)
	}
	return key, value
}

// TestProgram_IPOptionsMoveThePort proves both programs read the L4 port at
// 14 + ihl*4: one 4-byte IPv4 option word (IHL 6) puts the TCP header at byte
// 38. Read at the fixed byte 34, either port would come from the option bytes
// (0x0101, 257).
// MUTATION: drop the ihl*4 term from the L4 offset (a fixed 14+20): both
// directions key port 257 and the test goes red.
func TestProgram_IPOptionsMoveThePort(t *testing.T) {
	coll := loadColl(t, false)
	defer coll.Close()
	pkt := ethIPv4Unpadded(protoTCP, 6, []byte{1, 1, 1, 1}, tcpHdr(1234, 8080))

	for _, dir := range []struct {
		prog, ports string
		want        uint16
	}{
		{progIngress, mapPortIngress, 8080}, // ingress keys the destination port
		{progEgress, mapPortEgress, 1234},   // egress keys the source port
	} {
		runProg(t, coll.Programs[dir.prog], pkt)
		key, got := onlyPortEntry(t, coll.Maps[dir.ports])
		want := bpfPortKey{Port: dir.want, Protocol: protoTCP}
		if key != want {
			t.Errorf("%s keyed %+v, want %+v (the port after the IPv4 options)", dir.prog, key, want)
		}
		if got != uint64(len(pkt)) {
			t.Errorf("%s counted %d bytes, want %d (skb->len)", dir.prog, got, len(pkt))
		}
	}
}

// TestProgram_TruncatedL4HeaderKeysPortZero proves the port is read only when
// the whole TCP (20-byte) or UDP (8-byte) header lies inside skb->len: a header
// one byte short keys port 0 under its protocol, and the whole one keys its
// port. Each frame is unpadded, so the packet ends where the row says.
// MUTATION: bound the L4 read by the 2-byte port (l4PortLen) instead of the
// header length: both one-byte-short rows key their port and go red.
func TestProgram_TruncatedL4HeaderKeysPortZero(t *testing.T) {
	for _, row := range []struct {
		name  string
		proto byte
		l4    []byte
		want  uint16
	}{
		{"tcp header one byte short", protoTCP, tcpHdr(1234, 443)[:tcpHdrLen-1], 0},
		{"tcp header whole", protoTCP, tcpHdr(1234, 443), 443},
		{"udp header one byte short", protoUDP, udpHdr(5000, 53)[:udpHdrLen-1], 0},
		{"udp header whole", protoUDP, udpHdr(5000, 53), 53},
	} {
		t.Run(row.name, func(t *testing.T) {
			coll := loadColl(t, false)
			defer coll.Close()
			pkt := ethIPv4Unpadded(row.proto, 5, nil, row.l4)
			runProg(t, coll.Programs[progIngress], pkt)

			key, got := onlyPortEntry(t, coll.Maps[mapPortIngress])
			want := bpfPortKey{Port: row.want, Protocol: row.proto}
			if key != want {
				t.Errorf("keyed %+v, want %+v", key, want)
			}
			if got != uint64(len(pkt)) {
				t.Errorf("counted %d bytes, want %d (skb->len)", got, len(pkt))
			}
		})
	}
}

// TestProgram_L4OffsetBeyondThePacketKeysPortZero proves an IHL that puts the
// L4 header past the end of the packet still counts the packet, under port 0:
// IHL 15 places the TCP header at byte 74 of a 64-byte frame, so no port byte
// exists to read.
// MUTATION: send the TCP branch's refused bound to ret0 instead of build_key:
// the packet is not counted and the test goes red.
func TestProgram_L4OffsetBeyondThePacketKeysPortZero(t *testing.T) {
	coll := loadColl(t, false)
	defer coll.Close()
	pkt := ethIPv4(protoTCP, [4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, tcpHdr(1234, 443))
	pkt[ethHdrLen] = 0x4F // version 4, IHL 15: a 60-byte IPv4 header
	runProg(t, coll.Programs[progIngress], pkt)

	key, got := onlyPortEntry(t, coll.Maps[mapPortIngress])
	want := bpfPortKey{Port: 0, Protocol: protoTCP}
	if key != want {
		t.Errorf("keyed %+v, want %+v", key, want)
	}
	if got != uint64(len(pkt)) {
		t.Errorf("counted %d bytes, want %d (skb->len)", got, len(pkt))
	}
}
