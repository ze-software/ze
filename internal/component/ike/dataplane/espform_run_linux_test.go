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
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// errESPFormConnDrained ends the reader once the scripted datagrams are spent. run treats
// any read error as the end of the socket's life, which is what a closed socket produces.
var errESPFormConnDrained = errors.New("test: no more datagrams")

// fakeESPFormConn hands run a scripted sequence of bare ESP datagrams and then ends it.
// It stands in for the raw IPPROTO_ESP socket, which needs CAP_NET_RAW and cannot be fed
// a chosen datagram from a unit test.
type fakeESPFormConn struct {
	from  net.Addr
	reads [][]byte
	next  int
}

func (c *fakeESPFormConn) ReadFrom(p []byte) (int, net.Addr, error) {
	if c.next >= len(c.reads) {
		return 0, nil, errESPFormConnDrained
	}
	datagram := c.reads[c.next]
	c.next++
	return copy(p, datagram), c.from, nil
}

func (c *fakeESPFormConn) WriteTo([]byte, net.Addr) (int, error) { return 0, errESPFormConnDrained }
func (c *fakeESPFormConn) Close() error                          { return nil }
func (c *fakeESPFormConn) LocalAddr() net.Addr                   { return c.from }
func (c *fakeESPFormConn) SetDeadline(time.Time) error           { return nil }
func (c *fakeESPFormConn) SetReadDeadline(time.Time) error       { return nil }
func (c *fakeESPFormConn) SetWriteDeadline(time.Time) error      { return nil }

// recordedESPForm is one re-presented datagram. The bytes are copied because run hands the
// injector the same output buffer for every datagram it builds.
type recordedESPForm struct {
	packet []byte
	dst    netip.Addr
}

// recordingESPFormInjector stands in for the raw IP_HDRINCL socket and keeps what run asked
// it to send, so a test can assert on the datagram's CONTENT rather than on the fact that a
// call happened.
type recordingESPFormInjector struct {
	mu   sync.Mutex
	sent []recordedESPForm
}

func (i *recordingESPFormInjector) inject(pkt []byte, dst netip.Addr) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.sent = append(i.sent, recordedESPForm{packet: bytes.Clone(pkt), dst: dst})
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
// what it re-presented. run exits when the fake socket drains, which is why stop is left
// OPEN: the reader must genuinely read and classify every datagram to reach that point.
func driveESPFormRun(t *testing.T, r *espFormReceiver, datagrams [][]byte) *recordingESPFormInjector {
	t.Helper()

	inj := &recordingESPFormInjector{}
	conn := &fakeESPFormConn{from: &net.IPAddr{IP: net.IPv4(198, 51, 100, 7)}, reads: datagrams}
	stop := make(chan struct{})

	r.wg.Add(1) // run reports its own completion, exactly as startLocked arranges.
	r.run(conn, inj, stop)
	r.wg.Wait()

	if conn.next != len(datagrams) {
		t.Fatalf("the reader consumed %d of %d datagrams; it stopped early and the assertions below prove nothing",
			conn.next, len(datagrams))
	}
	return inj
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
	// Register through the receiver's own registry. Watch would open the raw sockets this
	// test exists to run without.
	r.reg.watch(spi, peer, local)

	esp := bareESP(spi, 3, 0xA5)
	inj := driveESPFormRun(t, r, [][]byte{esp})

	got := inj.records()
	if len(got) != 1 {
		t.Fatalf("re-presented %d datagrams, want 1; the bare ESP form of a watched SA is not served", len(got))
	}
	rec := got[0]

	if rec.dst != local {
		t.Errorf("sent to %v, want the local endpoint %v; __xfrm_state_lookup keys on the destination",
			rec.dst, local)
	}

	// The datagram must be byte-for-byte what writeESPForm builds for this payload.
	want := make([]byte, espFormPacketLen(esp))
	n := writeESPForm(want, peer, local, esp)
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
	r.reg.watch(0x11111111, peer, local)

	unwatched := bareESP(0x22222222, 1, 0x5A)
	runt := make([]byte, espFormMinESPLen-1)

	inj := driveESPFormRun(t, r, [][]byte{unwatched, runt})

	if got := inj.records(); len(got) != 0 {
		t.Fatalf("re-presented %d datagrams for traffic no watched SA owns, want 0; "+
			"XFRM already accepted that traffic and the state would receive a duplicate", len(got))
	}
}
