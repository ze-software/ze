package dataplane

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"
)

// VALIDATES: only an SPI whose inbound state carries a template is re-presented, and
// Forget reports when the last one is gone so the sockets can be released.
// PREVENTS: re-presenting bare ESP for a template-free SA. XFRM ACCEPTS that traffic, so
// re-presenting it would inject a duplicate of every packet the kernel already decrypted.
func TestESPFormRegistryWatchesOnlyTemplatedSPIs(t *testing.T) {
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	var reg espFormRegistry

	if _, ok := reg.target(0x1111); ok {
		t.Error("an empty registry reported a watched SPI")
	}

	if first := reg.watch(0x1111, peer, local); !first {
		t.Error("the first watch did not report that the receiver must start")
	}
	if first := reg.watch(0x2222, peer, local); first {
		t.Error("a second watch reported that the receiver must start again")
	}

	got, ok := reg.target(0x1111)
	if !ok {
		t.Fatal("a watched SPI was not found")
	}
	if got.peer != peer || got.local != local {
		t.Errorf("target %v -> %v, want %v -> %v", got.peer, got.local, peer, local)
	}
	if _, ok := reg.target(0x3333); ok {
		t.Error("an SPI that was never watched was reported as watched")
	}

	if last := reg.forget(0x1111); last {
		t.Error("forgetting one of two SPIs reported the last one was gone")
	}
	if _, ok := reg.target(0x1111); ok {
		t.Error("a forgotten SPI is still watched")
	}
	if last := reg.forget(0x2222); !last {
		t.Error("forgetting the final SPI did not report that the sockets can be released")
	}
	if last := reg.forget(0x2222); last {
		t.Error("forgetting an absent SPI reported that the sockets can be released")
	}

	// forgetAll is what a backend shutdown uses. A registry it left populated would make
	// the NEXT watch report that it is not the first, so the receiver would never restart
	// and that SA would silently receive one ESP form only.
	reg.watch(0x4444, peer, local)
	reg.watch(0x5555, peer, local)
	reg.forgetAll()
	if _, ok := reg.target(0x4444); ok {
		t.Error("forgetAll left an SPI watched")
	}
	if first := reg.watch(0x6666, peer, local); !first {
		t.Error("a watch after forgetAll did not report that the receiver must start again")
	}
}

// VALIDATES: the re-presentation rate bound refills over time and stops handing out
// tokens once the burst is spent.
// PREVENTS: an off-path flood aimed at a watched SPI spending an unbounded share of CPU
// reaching the crypto check.
func TestESPFormLimiter(t *testing.T) {
	start := time.Unix(1700000000, 0)
	lim := newESPFormLimiter(start)

	for i := range espFormBurst {
		if !lim.allow(start) {
			t.Fatalf("refused datagram %d of the initial burst of %d", i, espFormBurst)
		}
	}
	if lim.allow(start) {
		t.Fatal("handed out a token past the burst with no time elapsed")
	}

	// A full second refills the whole burst.
	if !lim.allow(start.Add(time.Second)) {
		t.Error("a full second did not refill the bound")
	}

	// A partial second refills proportionally, and never past the burst.
	lim = newESPFormLimiter(start)
	for range espFormBurst {
		lim.allow(start)
	}
	half := start.Add(500 * time.Millisecond)
	if !lim.allow(half) {
		t.Error("half a second refilled no tokens at all")
	}
	drained := 1
	for lim.allow(half) {
		drained++
		if drained > espFormBurst {
			t.Fatal("a partial refill exceeded the burst")
		}
	}
	if drained > espFormRate/2+1 {
		t.Errorf("half a second yielded %d tokens, want at most %d", drained, espFormRate/2+1)
	}
}

// VALIDATES: writeESPForm re-presents a bare ESP payload as a well-formed IPv4/UDP
// datagram addressed from the peer to the local endpoint, on the port pair RFC 3948
// Section 2.1 requires.
// PREVENTS: a re-presentation that substitutes a local source address. XFRM matches an
// inbound state on the OUTER addresses, so a datagram carrying the wrong source reaches
// no state and the ESP form the RFC requires is silently lost.
func TestWriteESPFormBuildsEncapsulatedDatagram(t *testing.T) {
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	esp := make([]byte, 40)
	binary.BigEndian.PutUint32(esp[:4], 0xDEADBEEF)
	esp[7] = 3 // sequence number

	buf := make([]byte, espFormPacketLen(esp))
	n := writeESPForm(buf, peer, local, esp)
	if n != espFormPacketLen(esp) {
		t.Fatalf("wrote %d bytes, want %d", n, espFormPacketLen(esp))
	}
	if n != espFormHeaderLen+len(esp) {
		t.Fatalf("wrote %d bytes, want a %d-octet header plus %d of ESP", n, espFormHeaderLen, len(esp))
	}

	if buf[0] != 0x45 {
		t.Errorf("version/IHL octet %#x, want 0x45 (IPv4, five-word header)", buf[0])
	}
	if got := binary.BigEndian.Uint16(buf[2:4]); int(got) != n {
		t.Errorf("IPv4 total length %d, want %d", got, n)
	}
	if buf[9] != 17 {
		t.Errorf("IPv4 protocol %d, want 17 (UDP)", buf[9])
	}
	if buf[8] == 0 {
		t.Error("TTL is zero; the datagram would be discarded before it reached XFRM")
	}

	wantSrc, wantDst := peer.As4(), local.As4()
	if got := netip.AddrFrom4([4]byte(buf[12:16])); got != peer {
		t.Errorf("source address %v, want the peer %v; XFRM matches on the outer addresses", got, peer)
	}
	if got := netip.AddrFrom4([4]byte(buf[16:20])); got != local {
		t.Errorf("destination address %v, want the local endpoint %v", got, local)
	}
	if !bytes.Equal(buf[12:16], wantSrc[:]) || !bytes.Equal(buf[16:20], wantDst[:]) {
		t.Error("address octets do not round-trip")
	}

	// RFC 3948 Section 2.1: both ports are the IKE port pair, and RFC 7296 Section 2.23
	// forbids encapsulation on port 500, so 4500 is the only legal pair.
	if got := binary.BigEndian.Uint16(buf[20:22]); got != espFormUDPPort {
		t.Errorf("UDP source port %d, want %d", got, espFormUDPPort)
	}
	if got := binary.BigEndian.Uint16(buf[22:24]); got != espFormUDPPort {
		t.Errorf("UDP destination port %d, want %d", got, espFormUDPPort)
	}
	if got := binary.BigEndian.Uint16(buf[24:26]); int(got) != espFormUDPHeaderLen+len(esp) {
		t.Errorf("UDP length %d, want %d", got, espFormUDPHeaderLen+len(esp))
	}

	if !bytes.Equal(buf[espFormHeaderLen:n], esp) {
		t.Error("the ESP payload was not copied verbatim; the SA would fail its integrity check")
	}
}

// VALIDATES: writeESPForm reports 0 for every input it cannot render, and writes nothing.
// PREVENTS: a partial header reaching the wire. A caller that ignored the length would
// otherwise transmit a truncated datagram built from a short payload or a small buffer.
func TestWriteESPFormRefusesUnusableInput(t *testing.T) {
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	v6 := netip.MustParseAddr("2001:db8::1")
	good := make([]byte, 32)

	cases := []struct {
		name      string
		src, dst  netip.Addr
		esp       []byte
		bufLen    int
		wantWrite bool
	}{
		{"payload one octet below the ESP header", peer, local, make([]byte, espFormMinESPLen-1), 512, false},
		{"payload exactly the ESP header", peer, local, make([]byte, espFormMinESPLen), 512, true},
		{"buffer one octet short", peer, local, good, espFormPacketLen(good) - 1, false},
		{"buffer exactly large enough", peer, local, good, espFormPacketLen(good), true},
		{"IPv6 source", v6, local, good, 512, false},
		{"IPv6 destination", peer, v6, good, 512, false},
		{"empty payload", peer, local, nil, 512, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, tc.bufLen)
			n := writeESPForm(buf, tc.src, tc.dst, tc.esp)
			if tc.wantWrite {
				if n == 0 {
					t.Fatalf("wrote nothing, want a datagram of %d bytes", espFormPacketLen(tc.esp))
				}
				return
			}
			if n != 0 {
				t.Fatalf("wrote %d bytes for an input that cannot be rendered", n)
			}
			for i, b := range buf {
				if b != 0 {
					t.Fatalf("buffer octet %d is %#x; a refused input must leave the buffer untouched", i, b)
				}
			}
		})
	}
}

// VALIDATES: espFormSPI reads the SPI from the first four octets and refuses a payload
// too short to carry one.
// PREVENTS: a truncated datagram being read as SPI zero and matched against a watched SA.
func TestESPFormSPI(t *testing.T) {
	esp := make([]byte, espFormMinESPLen)
	binary.BigEndian.PutUint32(esp[:4], 0x0102ABCD)
	got, ok := espFormSPI(esp)
	if !ok {
		t.Fatal("refused a payload long enough to carry an SPI")
	}
	if got != 0x0102ABCD {
		t.Errorf("SPI %#x, want %#x", got, 0x0102ABCD)
	}

	// Boundary: one octet below the minimum must be refused, and must not read as zero.
	if spi, ok := espFormSPI(make([]byte, espFormMinESPLen-1)); ok {
		t.Errorf("accepted a %d-octet payload and read SPI %#x; a short read must fail closed",
			espFormMinESPLen-1, spi)
	}
	if _, ok := espFormSPI(nil); ok {
		t.Error("accepted a nil payload")
	}
}
