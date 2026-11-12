package dataplane

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"
	"time"
)

// natTarget is the target of an SA installed with the RFC 3948 ports, UDP 4500 on both
// sides, the tuple the tests' SAs are watched with unless they name other ports.
func natTarget(peer, local netip.Addr) espFormTarget {
	return espFormTarget{peer: peer, local: local, peerPort: espFormUDPPort, localPort: espFormUDPPort}
}

// VALIDATES: only an SPI whose inbound state carries a template is re-presented, and
// Forget reports when the last one is gone so the sockets can be released.
// PREVENTS: re-presenting bare ESP for a template-free SA. XFRM ACCEPTS that traffic, so
// re-presenting it would inject a duplicate of every packet the kernel already decrypted.
func TestESPFormRegistryWatchesOnlyTemplatedSPIs(t *testing.T) {
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	var reg espFormRegistry

	if reg.watching(0x1111) {
		t.Error("an empty registry reported a watched SPI")
	}

	if first := reg.watch(0x1111, natTarget(peer, local)); !first {
		t.Error("the first watch did not report that the receiver must start")
	}
	if first := reg.watch(0x2222, natTarget(peer, local)); first {
		t.Error("a second watch reported that the receiver must start again")
	}

	got, ok := reg.target(0x1111, local)
	if !ok {
		t.Fatal("a watched SPI was not found")
	}
	if got.peer != peer || got.local != local {
		t.Errorf("target %v -> %v, want %v -> %v", got.peer, got.local, peer, local)
	}
	if reg.watching(0x3333) {
		t.Error("an SPI that was never watched was reported as watched")
	}

	if last := reg.forget(0x1111, local); last {
		t.Error("forgetting one of two SPIs reported the last one was gone")
	}
	if reg.watching(0x1111) {
		t.Error("a forgotten SPI is still watched")
	}
	if last := reg.forget(0x2222, local); !last {
		t.Error("forgetting the final SPI did not report that the sockets can be released")
	}
	if last := reg.forget(0x2222, local); last {
		t.Error("forgetting an absent SPI reported that the sockets can be released")
	}

	// forgetAll is what a backend shutdown uses. A registry it left populated would make
	// the NEXT watch report that it is not the first, so the receiver would never restart
	// and that SA would silently receive one ESP form only.
	reg.watch(0x4444, natTarget(peer, local))
	reg.watch(0x5555, natTarget(peer, local))
	reg.forgetAll()
	if reg.watching(0x4444) {
		t.Error("forgetAll left an SPI watched")
	}
	if first := reg.watch(0x6666, natTarget(peer, local)); !first {
		t.Error("a watch after forgetAll did not report that the receiver must start again")
	}
}

// TestESPFormRegistryHoldsTwoSAsOnOneSPI proves the registry holds two Child SAs that
// carry one inbound SPI to two local addresses, the pair XFRM keys an inbound state on,
// and that forget removes only the SA whose local address it names.
//
// VALIDATES: a second watch of an SPI on another local address keeps the first SA's
// target; the fast path serves the first SA watched; a forget naming a local address no
// SA holds removes nothing; forgetting the first hands the fast path to the second,
// which stays watched, and only the last forget reports the set empty.
// PREVENTS: the second SA's watch replacing the first SA's target, so the complete path
// drops the first SA's bare ESP, and a teardown of one SA (or of an outbound SA whose
// peer-chosen SPI matches) ending the other's service.
// MUTATION: in watch, replace the SPI's list with the new SA rather than append it: the
// first SA's target is gone.
// MUTATION: in forget, drop the local address test: the forget naming no SA removes the
// first one.
// MUTATION: in fastTarget, answer the last SA rather than the first: the fast path
// serves the second SA while the first is still watched.
func TestESPFormRegistryHoldsTwoSAsOnOneSPI(t *testing.T) {
	const spi = 0x7777
	peerA, localA := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	peerB, localB := netip.MustParseAddr("198.51.100.8"), netip.MustParseAddr("203.0.113.10")
	var reg espFormRegistry

	if first := reg.watch(spi, natTarget(peerA, localA)); !first {
		t.Error("the first watch did not report that the receiver must start")
	}
	if first := reg.watch(spi, natTarget(peerB, localB)); first {
		t.Error("the second SA on the SPI reported that the receiver must start again")
	}
	for _, want := range []espFormTarget{
		{peer: peerA, local: localA, peerPort: espFormUDPPort, localPort: espFormUDPPort},
		{peer: peerB, local: localB, peerPort: espFormUDPPort, localPort: espFormUDPPort},
	} {
		if got, ok := reg.target(spi, want.local); !ok || got != want {
			t.Errorf("target(%#x, %v) = %+v, %v, want %+v", spi, want.local, got, ok, want)
		}
	}
	if got, ok := reg.fastTarget(spi); !ok || got.local != localA {
		t.Errorf("the fast path serves %+v, %v, want the first SA, on %v", got, ok, localA)
	}

	if last := reg.forget(spi, netip.MustParseAddr("203.0.113.11")); last {
		t.Error("a forget naming no SA reported the set empty")
	}
	if _, ok := reg.target(spi, localA); !ok {
		t.Error("a forget naming no SA removed the first SA")
	}

	if last := reg.forget(spi, localA); last {
		t.Error("forgetting one of two SAs on the SPI reported the set empty")
	}
	if _, ok := reg.target(spi, localA); ok {
		t.Error("the forgotten SA is still watched")
	}
	if _, ok := reg.target(spi, localB); !ok {
		t.Fatal("forgetting the first SA removed the second, which carries the same SPI elsewhere")
	}
	if got, ok := reg.fastTarget(spi); !ok || got.local != localB {
		t.Errorf("after the first SA went, the fast path serves %+v, %v, want the second, on %v", got, ok, localB)
	}
	if last := reg.forget(spi, localB); !last {
		t.Error("forgetting the last SA did not report that the sockets can be released")
	}
	if _, ok := reg.fastTarget(spi); ok {
		t.Error("the fast path still serves an SPI no SA carries")
	}
}

// TestESPFormRegistryListsSharedSPIs proves the registry records each SA the fast path
// does not serve because an SA watched before it carries the same SPI to another local
// address, and that the record goes with the Forget of either SA.
//
// VALIDATES: one SA on an SPI is no entry; the second SA's watch is one entry naming
// the SPI, the first SA's local address as served and its own as unserved; a third SA
// is a second entry, sorted by its local address; an SPI carried by one SA elsewhere is
// no entry; forgetting an unserved SA takes its entry out; forgetting the served SA
// hands the fast path to the next one, whose entry goes, and the entries left name the
// new served address.
// PREVENTS: a doctor that never reports the collision, or that still reports it after
// either SA went, telling the operator to clear a healthy peer.
// MUTATION: in shared, start the inner loop at 0 rather than 1: the lone SA on 0x8888
// and the served SA are each listed, so the first check is red.
// MUTATION: in shared, record sas[i].local as Served: every entry names the unserved
// address twice.
// MUTATION: in shared, drop the sort: the two entries come out in map and list order,
// which for 203.0.113.12 watched before 203.0.113.11 is the wrong order.
func TestESPFormRegistryListsSharedSPIs(t *testing.T) {
	const spi, lone = 0x7777, 0x8888
	peer := netip.MustParseAddr("198.51.100.7")
	localA := netip.MustParseAddr("203.0.113.9")
	localB := netip.MustParseAddr("203.0.113.12")
	localC := netip.MustParseAddr("203.0.113.11")
	var reg espFormRegistry
	check := func(when string, want []ESPFormSharedSPI) {
		t.Helper()
		if got := reg.shared(); !slices.Equal(got, want) {
			t.Errorf("%s: shared = %+v, want %+v", when, got, want)
		}
	}

	reg.watch(spi, natTarget(peer, localA))
	reg.watch(lone, natTarget(peer, localB))
	check("one SA per SPI", nil)

	reg.watch(spi, natTarget(peer, localB))
	check("after the second SA on the SPI", []ESPFormSharedSPI{{SPI: spi, Served: localA, Unserved: localB}})

	reg.watch(spi, natTarget(peer, localC))
	check("after the third SA on the SPI", []ESPFormSharedSPI{
		{SPI: spi, Served: localA, Unserved: localC},
		{SPI: spi, Served: localA, Unserved: localB},
	})

	reg.forget(spi, localC)
	check("after the third SA was forgotten", []ESPFormSharedSPI{{SPI: spi, Served: localA, Unserved: localB}})

	reg.watch(spi, natTarget(peer, localC))
	reg.forget(spi, localA)
	check("after the served SA was forgotten", []ESPFormSharedSPI{{SPI: spi, Served: localB, Unserved: localC}})

	reg.forget(spi, localC)
	check("after the last unserved SA was forgotten", nil)
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
	n := writeESPForm(buf, peer, local, 0, esp)
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
	// A TUN write is received as it stands, and ip_rcv_core drops a header whose checksum
	// is wrong. Summed independently of the producer: the one's complement sum of a valid
	// header, its checksum field included, is all ones (RFC 791 Section 3.1).
	if sum := ipv4HeaderOnesSum(buf[:espFormIPv4HeaderLen]); sum != 0xFFFF {
		t.Errorf("IPv4 header sums to %#04x, want 0xffff; the stack drops the datagram on ze-espform (checksum field %#04x)",
			sum, binary.BigEndian.Uint16(buf[10:12]))
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

// ipv4HeaderOnesSum folds the one's complement sum of every 16-bit word of header. It is
// the test's own arithmetic, written apart from espFormIPv4Checksum so the assertions
// below do not grade the producer with itself.
func ipv4HeaderOnesSum(header []byte) uint32 {
	var sum uint32
	for i := 0; i+1 < len(header); i += 2 {
		sum += uint32(header[i])<<8 | uint32(header[i+1])
	}
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return sum
}

// VALIDATES: espFormIPv4Checksum produces the known checksum of a published header.
// PREVENTS: a checksum that is self-consistent but wrong, such as one that sums bytes
// instead of 16-bit words or forgets the final complement. The vector is the common
// textbook header 4500 0073 0000 4000 4011 xxxx c0a8 0001 c0a8 00c7, whose checksum is
// 0xb861.
func TestESPFormIPv4ChecksumKnownHeader(t *testing.T) {
	header := []byte{
		0x45, 0x00, 0x00, 0x73, 0x00, 0x00, 0x40, 0x00,
		0x40, 0x11, 0x00, 0x00, 0xc0, 0xa8, 0x00, 0x01,
		0xc0, 0xa8, 0x00, 0xc7,
	}
	if got := espFormIPv4Checksum(header); got != 0xb861 {
		t.Fatalf("checksum %#04x, want 0xb861", got)
	}
	binary.BigEndian.PutUint16(header[10:12], 0xb861)
	if sum := ipv4HeaderOnesSum(header); sum != 0xFFFF {
		t.Fatalf("the header with its checksum sums to %#04x, want 0xffff", sum)
	}
}

// VALIDATES: writeESPForm writes a valid IPv4 header checksum into a buffer that already
// holds an earlier datagram's header.
// PREVENTS: a checksum summed over the previous datagram's checksum field. The reader
// reuses one output buffer for every datagram, so a builder that forgot to clear the
// field before summing would write a valid header once and a wrong one ever after.
func TestWriteESPFormChecksumOverReusedBuffer(t *testing.T) {
	esp := make([]byte, 40)
	binary.BigEndian.PutUint32(esp[:4], 0xDEADBEEF)
	buf := make([]byte, espFormPacketLen(esp))

	if n := writeESPForm(buf, netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9"), 0, esp); n == 0 {
		t.Fatal("writeESPForm refused the first fixture")
	}
	first := binary.BigEndian.Uint16(buf[10:12])
	if n := writeESPForm(buf, netip.MustParseAddr("192.0.2.200"), netip.MustParseAddr("203.0.113.9"), 0, esp); n == 0 {
		t.Fatal("writeESPForm refused the second fixture")
	}
	if sum := ipv4HeaderOnesSum(buf[:espFormIPv4HeaderLen]); sum != 0xFFFF {
		t.Errorf("the second header sums to %#04x, want 0xffff", sum)
	}
	if second := binary.BigEndian.Uint16(buf[10:12]); second == first {
		t.Errorf("both headers carry checksum %#04x, though their source addresses differ", second)
	}
}

// VALIDATES: writeESPForm writes the TOS octet it is given into the header it builds, CE
// and a DSCP with 0b10 (ECN-capable) alike, and the header checksum covers it.
// PREVENTS: the complete path discarding ECN congestion indications, against RFC 7296
// Section 2.24: decapsulators "MUST implement the tunnel encapsulation and decapsulation
// processing specified in [IPSECARCH] to prevent discarding of ECN congestion
// indications." XFRM copies CE into the inner header from the outer header this builder
// writes, so a builder that wrote TOS 0 discarded every mark.
// MUTATION: writeESPForm writing buf[1] = 0 turns both rows red; summing the checksum
// before writing buf[1] turns the checksum check red.
func TestWriteESPFormCarriesTheTOS(t *testing.T) {
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	esp := make([]byte, 40)
	binary.BigEndian.PutUint32(esp[:4], 0xDEADBEEF)

	cases := []struct {
		name string
		tos  uint8
	}{
		{"ECN field CE, DSCP 0", 0x03},
		{"DSCP AF41 with ECN field 0b10 (ECN-capable)", 0x8a},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, espFormPacketLen(esp))
			if n := writeESPForm(buf, peer, local, tc.tos, esp); n != len(buf) {
				t.Fatalf("wrote %d octets, want %d", n, len(buf))
			}
			if buf[1] != tc.tos {
				t.Errorf("TOS octet %#02x, want %#02x", buf[1], tc.tos)
			}
			if sum := ipv4HeaderOnesSum(buf[:espFormIPv4HeaderLen]); sum != 0xFFFF {
				t.Errorf("IPv4 header sums to %#04x, want 0xffff; the checksum does not cover the TOS", sum)
			}
		})
	}
}

// VALIDATES: espFormBareIPv4 answers the TOS octet and the ESP payload of a whole IPv4
// datagram, after options when the header carries them and up to the total length, and
// refuses every read that is not a whole IPv4 datagram.
// PREVENTS: a payload boundary or a TOS guessed for a malformed read. The reader socket
// hands run the IPv4 header with the payload, so a wrong split would re-present header
// octets as ESP, and a refused read must never be served with a TOS of zero.
func TestESPFormBareIPv4(t *testing.T) {
	esp := make([]byte, espFormMinESPLen)
	binary.BigEndian.PutUint32(esp[:4], 0x0102ABCD)
	datagram := func(versionIHL, tos byte, totalLen int, options, payload []byte, trailing int) []byte {
		out := make([]byte, espFormIPv4HeaderLen, espFormIPv4HeaderLen+len(options)+len(payload)+trailing)
		out[0] = versionIHL
		out[1] = tos
		binary.BigEndian.PutUint16(out[2:4], uint16(totalLen))
		out[9] = 50
		out = append(out, options...)
		out = append(out, payload...)
		return append(out, make([]byte, trailing)...)
	}
	plainLen := espFormIPv4HeaderLen + len(esp)
	options := []byte{1, 1, 1, 1} // four NOP options, one 32-bit word (RFC 791 Section 3.1)

	cases := []struct {
		name    string
		in      []byte
		wantOK  bool
		wantTOS uint8
	}{
		{"plain header, CE", datagram(0x45, 0x03, plainLen, nil, esp, 0), true, 0x03},
		{"header with one word of options", datagram(0x46, 0x8a, plainLen+len(options), options, esp, 0), true, 0x8a},
		{"octets past the total length are not ESP", datagram(0x45, 0x02, plainLen, nil, esp, 4), true, 0x02},
		{"shorter than a header", make([]byte, espFormIPv4HeaderLen-1), false, 0},
		{"IPv6 version nibble", datagram(0x65, 0, plainLen, nil, esp, 0), false, 0},
		{"header length below five words", datagram(0x44, 0, plainLen, nil, esp, 0), false, 0},
		{"total length beyond the read", datagram(0x45, 0, plainLen+1, nil, esp, 0), false, 0},
		{"total length inside the header", datagram(0x46, 0, espFormIPv4HeaderLen, options, esp, 0), false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tos, _, payload, ok := espFormBareIPv4(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if tos != tc.wantTOS {
				t.Errorf("TOS %#02x, want %#02x", tos, tc.wantTOS)
			}
			if !bytes.Equal(payload, esp) {
				t.Errorf("payload %x, want the ESP octets %x", payload, esp)
			}
		})
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
			n := writeESPForm(buf, tc.src, tc.dst, 0, tc.esp)
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
