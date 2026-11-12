//go:build linux

package dataplane

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// errESPFormConnDrained ends the reader once the scripted datagrams are spent. run treats
// any read error as the end of the socket's life, which is what a closed socket produces.
var errESPFormConnDrained = errors.New("test: no more datagrams")

// fakeESPFormConn hands run a scripted sequence of bare ESP datagrams and then ends it.
// It stands in for the raw IPPROTO_ESP socket, which needs CAP_NET_RAW and cannot be fed
// a chosen datagram from a unit test.
//
// Each entry of reads is an ESP payload. ReadMsgIP, the read run makes, delivers it the
// way the kernel's raw socket does, behind the IPv4 header the datagram arrived with
// (raw_rcv pushes the skb back to its network header, and ReadMsgIP strips nothing):
// version 4, no options, protocol 50, the entry's source and the local address
// 203.0.113.9.
//
// froms, when set, names the source of each datagram in reads, position for position;
// otherwise every datagram comes from from. ReadMsgIP delivers a source that is not a
// *net.IPAddr as no address at all, which is what run gets from a socket that could not
// report an IP sender. toss, when set, names the TOS octet of each datagram's header,
// position for position; otherwise every header carries TOS 0.
type fakeESPFormConn struct {
	from  net.Addr
	froms []net.Addr
	toss  []uint8
	// dsts, when set, is each datagram's outer destination; 203.0.113.9 otherwise.
	dsts  []netip.Addr
	reads [][]byte
	next  int
}

func (c *fakeESPFormConn) ReadMsgIP(p, _ []byte) (n, oobn, flags int, addr *net.IPAddr, err error) {
	if c.next >= len(c.reads) {
		return 0, 0, 0, nil, errESPFormConnDrained
	}
	// i is the datagram this read answers: every per-datagram slice is indexed by it,
	// never by next, which already names the following one.
	i := c.next
	c.next++
	esp := c.reads[i]
	source := c.from
	if c.froms != nil {
		source = c.froms[i]
	}
	var tos uint8
	if c.toss != nil {
		tos = c.toss[i]
	}

	header := [espFormIPv4HeaderLen]byte{0: 0x45, 1: tos, 8: 64, 9: ProtoESP}
	binary.BigEndian.PutUint16(header[2:4], uint16(espFormIPv4HeaderLen+len(esp)))
	copy(header[16:20], net.IPv4(203, 0, 113, 9).To4())
	if c.dsts != nil {
		dst := c.dsts[i].As4()
		copy(header[16:20], dst[:])
	}
	ip, isIP := source.(*net.IPAddr)
	if isIP {
		copy(header[12:16], ip.IP.To4())
	}
	n = copy(p, header[:])
	n += copy(p[n:], esp)
	if !isIP {
		return n, 0, 0, nil, nil
	}
	return n, 0, 0, ip, nil
}

// Close stays only because `./le test weakened approve` cannot record its removal: three
// test types in this package each declare a Close, so the unit name dataplane.Close is ambiguous.
func (c *fakeESPFormConn) Close() error { return nil }

// recordedESPForm is one re-presented datagram. The bytes are copied because run hands the
// injector the same output buffer for every datagram it builds. dst is the destination the
// datagram's own IPv4 header carries, which is the only destination a TUN write has.
type recordedESPForm struct {
	packet []byte
	dst    netip.Addr
}

// recordingESPFormInjector stands in for the ze-espform device and keeps what run asked it
// to write, so a test can assert on the datagram's CONTENT rather than on the fact that a
// call happened.
type recordingESPFormInjector struct {
	mu   sync.Mutex
	sent []recordedESPForm
}

func (i *recordingESPFormInjector) inject(pkt []byte) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	rec := recordedESPForm{packet: bytes.Clone(pkt)}
	if len(pkt) >= espFormIPv4HeaderLen {
		rec.dst = netip.AddrFrom4([4]byte(pkt[16:20]))
	}
	i.sent = append(i.sent, rec)
	return nil
}

func (i *recordingESPFormInjector) records() []recordedESPForm {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]recordedESPForm(nil), i.sent...)
}

// bareESP builds one ESP payload. RFC 4303 Section 2 puts the SPI in the first four octets
// and the sequence number in the next four. The remaining octets stand for the encrypted
// body, which this receiver carries verbatim and never decrypts.
func bareESP(spi, seq uint32, body byte) []byte {
	esp := make([]byte, 48)
	binary.BigEndian.PutUint32(esp[0:4], spi)
	binary.BigEndian.PutUint32(esp[4:8], seq)
	for i := espFormMinESPLen; i < len(esp); i++ {
		esp[i] = body
	}
	return esp
}

// driveESPFormRun runs the reader over a scripted sequence of inbound datagrams and returns
// what it re-presented. run exits only when the fake socket drains: the reader must
// genuinely read and classify every datagram to reach that point.
// Every datagram comes from 198.51.100.7, the peer these tests install their SAs with.
func driveESPFormRun(t *testing.T, r *espFormReceiver, datagrams [][]byte) *recordingESPFormInjector {
	t.Helper()
	return driveESPFormRunConn(t, r, &fakeESPFormConn{from: &net.IPAddr{IP: net.IPv4(198, 51, 100, 7)}, reads: datagrams})
}

// driveESPFormRunConn is driveESPFormRun over a fake socket the caller scripted.
func driveESPFormRunConn(t *testing.T, r *espFormReceiver, conn *fakeESPFormConn) *recordingESPFormInjector {
	t.Helper()
	datagrams := conn.reads

	inj := &recordingESPFormInjector{}

	if err := r.run(conn, inj); !errors.Is(err, errESPFormConnDrained) {
		t.Fatalf("the reader ended with %v, want the drained socket's %v", err, errESPFormConnDrained)
	}

	if conn.next != len(datagrams) {
		t.Fatalf("the reader consumed %d of %d datagrams; it stopped early and the assertions below prove nothing",
			conn.next, len(datagrams))
	}
	return inj
}

// TestESPFormRunServesTwoSAsOnOneSPI proves the complete path serves two Child SAs
// that carry one inbound SPI to two local addresses, each from its own peer to its own
// local address, which XFRM keys the two states on: the random inbound SPI
// (generateESPSPI, engine/child.go) is never checked against the SPIs in use.
//
// METHOD: two SAs are watched through Watch's registry on one SPI; the reader runs over
// three datagrams, one per SA and one from the first SA's peer to the second SA's
// address, which neither SA was installed for.
// MUTATION: key espFormRegistry on the SPI alone (watch replaces the SPI's list): the
// first SA's datagram finds the second SA's target, its source is not that peer, and
// it is dropped.
// MUTATION: in run, look the SA up by SPI alone (the first of the SPI's list) rather
// than by SPI and destination: the second SA's datagram is dropped.
func TestESPFormRunServesTwoSAsOnOneSPI(t *testing.T) {
	const spi = 0x5A5A0001
	peerA, localA := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	peerB, localB := netip.MustParseAddr("198.51.100.8"), netip.MustParseAddr("203.0.113.10")
	r := newESPFormReceiver(slogutil.DiscardLogger())
	r.reg.watch(spi, natTarget(peerA, localA))
	r.reg.watch(spi, natTarget(peerB, localB))

	conn := &fakeESPFormConn{
		froms: []net.Addr{
			&net.IPAddr{IP: net.IP(peerA.AsSlice())},
			&net.IPAddr{IP: net.IP(peerB.AsSlice())},
			&net.IPAddr{IP: net.IP(peerA.AsSlice())},
		},
		dsts:  []netip.Addr{localA, localB, localB},
		reads: [][]byte{bareESP(spi, 1, 0xA1), bareESP(spi, 2, 0xB2), bareESP(spi, 3, 0xC3)},
	}
	got := driveESPFormRunConn(t, r, conn).records()
	if len(got) != 2 {
		t.Fatalf("re-presented %d datagrams, want two: one for each SA that carries the SPI", len(got))
	}
	for i, want := range []struct{ peer, local netip.Addr }{{peerA, localA}, {peerB, localB}} {
		source := netip.AddrFrom4([4]byte(got[i].packet[12:16]))
		if got[i].dst != want.local || source != want.peer {
			t.Errorf("datagram %d re-presented %v -> %v, want %v -> %v", i, source, got[i].dst, want.peer, want.local)
		}
	}
}

// VALIDATES: run re-presents a bare ESP datagram whose SPI is watched, and hands the
// injector the UDP-encapsulated datagram writeESPForm builds, addressed from the peer to
// the local endpoint the SA was installed with.
// PREVENTS: the headline claim of RFC 7296 Section 2.23 -- one Child SA receiving BOTH ESP
// wire forms -- resting on nothing. Ze's own producer for the form the kernel refuses is
// this reader, and before this test nothing drove it. Deleting run's build-and-send block
// left the whole linux test binary green: the socket-lifecycle tests never feed the reader
// a datagram, the hybrid integration test re-implements read-write-inject with its own
// descriptors and therefore measures the KERNEL trick rather than this code, the engine
// tests assert the AcceptBothESPForms flag and never its effect, and every integration test
// builds &xfrmBackend{} with espForms nil.
func TestESPFormRunRepresentsWatchedSPI(t *testing.T) {
	const spi = 0xDEADBEEF
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")

	r := newESPFormReceiver(slogutil.DiscardLogger())
	// Register through the receiver's own registry. Watch would open the reader socket,
	// ze-espform and the fast path, which this test exists to run without.
	r.reg.watch(spi, natTarget(peer, local))

	esp := bareESP(spi, 3, 0xA5)
	inj := driveESPFormRun(t, r, [][]byte{esp})

	got := inj.records()
	if len(got) != 1 {
		t.Fatalf("re-presented %d datagrams, want 1; the bare ESP form of a watched SA is not served", len(got))
	}
	rec := got[0]

	if rec.dst != local {
		t.Errorf("written into the device addressed to %v, want the local endpoint %v; __xfrm_state_lookup keys on the destination",
			rec.dst, local)
	}

	// The datagram must be byte-for-byte what writeESPForm builds for this payload.
	want := make([]byte, espFormPacketLen(esp))
	n := writeESPForm(want, peer, local, 0, esp)
	if n == 0 {
		t.Fatal("writeESPForm refused the fixture, so this test cannot assert on its output")
	}
	if !bytes.Equal(rec.packet, want[:n]) {
		t.Fatalf("re-presented datagram\n got %x\nwant %x", rec.packet, want[:n])
	}

	// Spelled out as well, so a failure names the field instead of showing a hex diff.
	if len(rec.packet) != espFormHeaderLen+len(esp) {
		t.Fatalf("datagram is %d bytes, want a %d-octet header plus %d of ESP",
			len(rec.packet), espFormHeaderLen, len(esp))
	}
	if addr := netip.AddrFrom4([4]byte(rec.packet[12:16])); addr != peer {
		t.Errorf("outer source %v, want the peer %v; XFRM matches an inbound state on the outer addresses",
			addr, peer)
	}
	if addr := netip.AddrFrom4([4]byte(rec.packet[16:20])); addr != local {
		t.Errorf("outer destination %v, want the local endpoint %v", addr, local)
	}
	if rec.packet[9] != 17 {
		t.Errorf("IPv4 protocol %d, want 17 (UDP); a bare re-presentation is the form XFRM already refused",
			rec.packet[9])
	}
	if port := binary.BigEndian.Uint16(rec.packet[22:24]); port != espFormUDPPort {
		t.Errorf("UDP destination port %d, want %d; only that port carries UDP_ENCAP_ESPINUDP",
			port, espFormUDPPort)
	}
	if !bytes.Equal(rec.packet[espFormHeaderLen:], esp) {
		t.Error("the ESP payload was not carried verbatim; the SA would fail its integrity check")
	}
}

// VALIDATES: run re-presents nothing for an SPI no watched SA owns, and nothing for a
// datagram too short to carry an SPI at all.
// PREVENTS: injecting a duplicate of traffic XFRM already accepted. A template-free SA's
// bare ESP is delivered by the kernel on its own fast path, so re-presenting it would hand
// that state a second copy of every packet it just decrypted. It also prevents a runt being
// read as SPI zero and matched against a watched SA.
func TestESPFormRunIgnoresUnwatchedTraffic(t *testing.T) {
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")

	r := newESPFormReceiver(slogutil.DiscardLogger())
	r.reg.watch(0x11111111, natTarget(peer, local))

	unwatched := bareESP(0x22222222, 1, 0x5A)
	runt := make([]byte, espFormMinESPLen-1)

	inj := driveESPFormRun(t, r, [][]byte{unwatched, runt})

	if got := inj.records(); len(got) != 0 {
		t.Fatalf("re-presented %d datagrams for traffic no watched SA owns, want 0; "+
			"XFRM already accepted that traffic and the state would receive a duplicate", len(got))
	}
}

// VALIDATES: run re-presents a watched SPI's datagram only when its outer source is the
// SA's peer, and carries that real source; a datagram from another address, or from an
// address the socket did not report as IP, is skipped.
// PREVENTS: source laundering. The reader once wrote every watched datagram under the
// peer's address whatever its sender, so a datagram from anywhere reached the SA's crypto
// check looking like the peer's. The state lookup ignores the source, so nothing after
// the reader would have told the two apart.
//
// The peer's datagram in the middle is the positive control: a reader that dropped
// everything would also re-present nothing for the other two.
func TestESPFormRunServesOnlyThePeersSource(t *testing.T) {
	const spi = 0xCAFEF00D
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")

	r := newESPFormReceiver(slogutil.DiscardLogger())
	r.reg.watch(spi, natTarget(peer, local))

	other := bareESP(spi, 1, 0x11)
	fromPeer := bareESP(spi, 2, 0x22)
	notIP := bareESP(spi, 3, 0x33)
	conn := &fakeESPFormConn{
		froms: []net.Addr{
			&net.IPAddr{IP: net.IPv4(192, 0, 2, 66)},
			&net.IPAddr{IP: net.IPv4(198, 51, 100, 7)},
			&net.UDPAddr{IP: net.IPv4(198, 51, 100, 7), Port: 4500},
		},
		reads: [][]byte{other, fromPeer, notIP},
	}
	inj := driveESPFormRunConn(t, r, conn)

	got := inj.records()
	if len(got) != 1 {
		t.Fatalf("re-presented %d datagrams, want 1: only the one from the peer %v", len(got), peer)
	}
	if !bytes.Equal(got[0].packet[espFormHeaderLen:], fromPeer) {
		t.Fatalf("re-presented the payload %x, want the peer's datagram %x", got[0].packet[espFormHeaderLen:], fromPeer)
	}
	if addr := netip.AddrFrom4([4]byte(got[0].packet[12:16])); addr != peer {
		t.Errorf("outer source %v, want the peer %v the datagram came from", addr, peer)
	}
}

// VALIDATES: run re-presents only the peer's bare ESP sent to the SA's local address: a
// datagram from the peer to another address of this host, or to the limited broadcast
// address, is not re-presented, as the fast path does not rewrite it.
// PREVENTS: the complete path re-addressing the peer's bare ESP sent anywhere onto the
// watched state. The reader rebuilds every datagram addressed to target.local, so a
// lookup that ignored the destination let a datagram to 192.0.2.200 or 255.255.255.255
// reach the SA as if sent to it.
// MUTATION: in run, look the SA up by SPI alone (fastTarget) rather than by (SPI, dst):
// all three are re-presented, and this test turns red. The datagram to the local
// address is the positive control.
func TestESPFormRunServesOnlyTheSAsLocalAddress(t *testing.T) {
	const spi = 0xCAFED00D
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")

	r := newESPFormReceiver(slogutil.DiscardLogger())
	r.reg.watch(spi, natTarget(peer, local))

	otherLocal := bareESP(spi, 1, 0x11)
	toLocal := bareESP(spi, 2, 0x22)
	broadcast := bareESP(spi, 3, 0x33)
	conn := &fakeESPFormConn{
		from: &net.IPAddr{IP: net.IPv4(198, 51, 100, 7)},
		dsts: []netip.Addr{
			netip.MustParseAddr("192.0.2.200"),
			local,
			netip.MustParseAddr("255.255.255.255"),
		},
		reads: [][]byte{otherLocal, toLocal, broadcast},
	}
	inj := driveESPFormRunConn(t, r, conn)

	got := inj.records()
	if len(got) != 1 {
		t.Fatalf("re-presented %d datagrams, want 1: only the one sent to the SA's local address %v", len(got), local)
	}
	if !bytes.Equal(got[0].packet[espFormHeaderLen:], toLocal) {
		t.Fatalf("re-presented the payload %x, want the datagram sent to %v (%x)",
			got[0].packet[espFormHeaderLen:], local, toLocal)
	}
}

// VALIDATES: run carries the TOS octet a refused bare datagram arrived with onto the
// ESP-in-UDP header it re-presents, ECN field and DSCP alike, under a header checksum that
// covers it. One datagram arrives marked CE (0x03), one with DSCP AF41 and 0b10 (ECN-capable) (0x8a).
// PREVENTS: the complete path discarding congestion indications, against RFC 7296 Section
// 2.24: decapsulators "MUST implement the tunnel encapsulation and decapsulation
// processing specified in [IPSECARCH] to prevent discarding of ECN congestion
// indications." XFRM copies CE into the inner header from the outer header it
// decapsulates, which on this path is the one run builds; a header rebuilt with TOS 0
// hands XFRM no CE to copy.
// MUTATION: writeESPForm writing buf[1] = 0 turns both rows red.
func TestESPFormRunKeepsTheOuterTOS(t *testing.T) {
	const spi = 0x0EC40001
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")

	r := newESPFormReceiver(slogutil.DiscardLogger())
	r.reg.watch(spi, natTarget(peer, local))

	const (
		tosCE          = 0x03 // ECN field CE (RFC 3168 Section 5), DSCP 0
		tosAF41Capable = 0x8a // DSCP AF41 (34) and ECN field 0b10 (ECN-capable)
	)
	conn := &fakeESPFormConn{
		from:  &net.IPAddr{IP: peer.AsSlice()},
		toss:  []uint8{tosCE, tosAF41Capable},
		reads: [][]byte{bareESP(spi, 1, 0x11), bareESP(spi, 2, 0x22)},
	}
	inj := driveESPFormRunConn(t, r, conn)

	got := inj.records()
	if len(got) != 2 {
		t.Fatalf("re-presented %d datagrams, want 2", len(got))
	}
	for i, want := range []uint8{tosCE, tosAF41Capable} {
		pkt := got[i].packet
		if len(pkt) < espFormIPv4HeaderLen {
			t.Fatalf("datagram %d is %d octets, shorter than an IPv4 header", i, len(pkt))
		}
		if pkt[1] != want {
			t.Errorf("datagram %d carries TOS %#02x, want %#02x as it arrived; XFRM copies CE into the inner header only from the outer header it decapsulates",
				i, pkt[1], want)
		}
		if sum := ipv4HeaderOnesSum(pkt[:espFormIPv4HeaderLen]); sum != 0xFFFF {
			t.Errorf("datagram %d header sums to %#04x, want 0xffff; ip_rcv_core drops it on ze-espform", i, sum)
		}
		if !bytes.Equal(pkt[espFormHeaderLen:], conn.reads[i]) {
			t.Errorf("datagram %d does not carry its ESP payload verbatim", i)
		}
	}
}
