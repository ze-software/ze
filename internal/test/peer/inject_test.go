package peer

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"
)

// TestBuildUpdatesV4Exact verifies byte-for-byte RFC 4271 §4.3 output for a
// 2-prefix IPv4 unicast stream. Reference hex was cross-validated against
// the scapy-based upstream bgpupdate generator on 2026-04-16.
func TestBuildUpdatesV4Exact(t *testing.T) {
	// VALIDATES: IPv4 unicast UPDATE wire format (ORIGIN + AS_PATH 4-byte ASN
	// + NEXT_HOP + inline NLRI) + End-of-RIB marker.
	// PREVENTS: wire-format drift (byte ordering, attr flags, NLRI packing).
	spec := InjectSpec{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Count:    2,
		NextHop:  netip.MustParseAddr("172.31.0.3"),
		ASN:      65100,
		EndOfRIB: true,
	}
	got, nMsgs, err := BuildUpdates(spec)
	if err != nil {
		t.Fatalf("BuildUpdates: %v", err)
	}
	if nMsgs != 2 { // 1 UPDATE with 2 NLRI + 1 EOR
		t.Errorf("msg count: got %d, want 2", nMsgs)
	}
	const wantHex = "ffffffffffffffffffffffffffffffff" + // marker
		"0033" + "02" + // len=51, type=UPDATE
		"0000" + // withdrawn routes length
		"0014" + // total path attrs length
		"40010100" + // ORIGIN IGP
		"4002060201" + "0000fe4c" + // AS_PATH AS_SEQUENCE 1x 65100
		"400304" + "ac1f0003" + // NEXT_HOP 172.31.0.3
		"180a0000" + // NLRI 10.0.0.0/24
		"180a0001" + // NLRI 10.0.1.0/24
		"ffffffffffffffffffffffffffffffff" + // EOR marker
		"001702" + "00000000" // EOR: len=23, type=UPDATE, wdr=0, attr=0
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("byte image mismatch\n got: %s\nwant: %s", hex.EncodeToString(got), wantHex)
	}
}

// TestBuildUpdatesV6Exact verifies byte-for-byte RFC 4760 §3 MP_REACH output
// for a 2-prefix IPv6 unicast stream.
func TestBuildUpdatesV6Exact(t *testing.T) {
	// VALIDATES: IPv6 unicast via MP_REACH_NLRI extended-length attribute.
	// PREVENTS: MP_REACH byte-layout drift (flags, AFI/SAFI, NH length, NLRI).
	spec := InjectSpec{
		Prefix:   netip.MustParsePrefix("2001:db8::/48"),
		Count:    2,
		NextHop:  netip.MustParseAddr("2001:db8::3"),
		ASN:      65100,
		EndOfRIB: true,
	}
	got, nMsgs, err := BuildUpdates(spec)
	if err != nil {
		t.Fatalf("BuildUpdates: %v", err)
	}
	if nMsgs != 2 {
		t.Errorf("msg count: got %d, want 2", nMsgs)
	}
	const wantHex = "ffffffffffffffffffffffffffffffff" + // marker
		"004b" + "02" + // len=75, type=UPDATE
		"0000" + // withdrawn
		"0034" + // attr_len=52
		"40010100" + // ORIGIN IGP
		"4002060201" + "0000fe4c" + // AS_PATH
		"900e" + "0023" + // MP_REACH flags=0x90 type=14 ext-len=35
		"0002" + "01" + // AFI=2 SAFI=1
		"10" + // NH length = 16
		"20010db8000000000000000000000003" + // NH 2001:db8::3
		"00" + // reserved
		"30" + "20010db80000" + // NLRI /48 2001:0db8:0000
		"30" + "20010db80001" + // NLRI /48 2001:0db8:0001
		"ffffffffffffffffffffffffffffffff" + // EOR
		"001702" + "00000000"
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("byte image mismatch\n got: %s\nwant: %s", hex.EncodeToString(got), wantHex)
	}
}

// TestBuildUpdatesLarge builds a 100k-prefix image and re-parses it,
// verifying every NLRI arrives and message framing is self-consistent.
func TestBuildUpdatesLarge(t *testing.T) {
	// VALIDATES: structural consistency of the byte image across many messages.
	// PREVENTS: off-by-one in full/partial message accounting or NLRI packing.
	spec := InjectSpec{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Count:    100_000,
		NextHop:  netip.MustParseAddr("172.31.0.3"),
		ASN:      65100,
		EndOfRIB: true,
	}
	buf, nMsgs, err := BuildUpdates(spec)
	if err != nil {
		t.Fatalf("BuildUpdates: %v", err)
	}
	msgs, nlri := countNLRI(t, buf)
	if nlri != spec.Count {
		t.Errorf("NLRI count: got %d, want %d", nlri, spec.Count)
	}
	if msgs != nMsgs {
		t.Errorf("msg count: got %d (parse), want %d (builder)", msgs, nMsgs)
	}
}

// TestBuildUpdatesZero handles count=0: no UPDATEs, just End-of-RIB.
func TestBuildUpdatesZero(t *testing.T) {
	spec := InjectSpec{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Count:    0,
		NextHop:  netip.MustParseAddr("172.31.0.3"),
		ASN:      65100,
		EndOfRIB: true,
	}
	buf, nMsgs, err := BuildUpdates(spec)
	if err != nil {
		t.Fatalf("BuildUpdates: %v", err)
	}
	if nMsgs != 1 {
		t.Errorf("msg count: got %d, want 1 (EOR only)", nMsgs)
	}
	if len(buf) != 23 {
		t.Errorf("byte length: got %d, want 23 (EOR only)", len(buf))
	}
}

// TestBuildUpdatesFamilyMismatch rejects cross-family next-hop.
func TestBuildUpdatesFamilyMismatch(t *testing.T) {
	spec := InjectSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		Count:   1,
		NextHop: netip.MustParseAddr("2001:db8::1"),
		ASN:     65100,
	}
	_, _, err := BuildUpdates(spec)
	if err == nil {
		t.Fatal("expected error for family mismatch, got nil")
	}
}

// TestInjectEndToEnd pairs a ModeInject peer with a minimal BGP client that
// does the OPEN handshake and reads the injected stream, verifying every
// byte arrives in order.
func TestInjectEndToEnd(t *testing.T) {
	// VALIDATES: ze-test peer --mode inject writes the BuildUpdates output
	// to the socket after OPEN, without reordering or interleaving.
	// PREVENTS: wrong dispatch on mode, truncated writes, keepalive
	// interleaved in the middle of the stream.
	const (
		count = 1000
		asn   = 65100
	)
	spec := &InjectSpec{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Count:    count,
		NextHop:  netip.MustParseAddr("127.0.0.1"),
		ASN:      asn,
		EndOfRIB: true,
	}
	port := reservePort(t)
	p, err := New(&Config{
		Port:     port,
		BindAddr: "127.0.0.1",
		Mode:     ModeInject,
		Inject:   spec,
		Output:   io.Discard,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan Result, 1)
	go func() { done <- p.Run(ctx) }()

	select {
	case <-p.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("peer failed to bind")
	}

	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { closeIgnoreErr(conn) })

	if _, err := conn.Write(minimalOpenMsg(asn, "127.0.0.1")); err != nil {
		t.Fatalf("write OPEN: %v", err)
	}
	if _, _, err := ReadMessage(conn); err != nil {
		t.Fatalf("read peer OPEN: %v", err)
	}
	if _, _, err := ReadMessage(conn); err != nil {
		t.Fatalf("read peer KEEPALIVE: %v", err)
	}
	if _, err := conn.Write(KeepaliveMsg()); err != nil {
		t.Fatalf("write KEEPALIVE: %v", err)
	}

	want, _, err := BuildUpdates(*spec)
	if err != nil {
		t.Fatalf("BuildUpdates: %v", err)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("byte image mismatch: got %d bytes, want %d bytes", len(got), len(want))
	}

	closeIgnoreErr(conn)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("peer did not return after cancel")
	}
}

// countNLRI walks a raw BGP UPDATE byte stream and counts NLRI across both
// inline-NLRI fields and MP_REACH_NLRI attributes.
func countNLRI(t *testing.T, buf []byte) (int, int) {
	t.Helper()
	nMsgs, nNLRI := 0, 0
	for off := 0; off < len(buf); {
		if !bytes.Equal(buf[off:off+16], Marker) {
			t.Fatalf("bad marker at offset %d", off)
		}
		msgLen := int(binary.BigEndian.Uint16(buf[off+16 : off+18]))
		if buf[off+18] != MsgUPDATE {
			t.Fatalf("non-UPDATE type %d at offset %d", buf[off+18], off)
		}
		body := buf[off+HeaderLen : off+msgLen]
		wdrLen := int(binary.BigEndian.Uint16(body[0:2]))
		pos := 2 + wdrLen
		attrLen := int(binary.BigEndian.Uint16(body[pos : pos+2]))
		attrs := body[pos+2 : pos+2+attrLen]
		nlri := body[pos+2+attrLen:]
		nNLRI += countNLRIField(nlri)
		nNLRI += countMPReachNLRI(attrs)
		nMsgs++
		off += msgLen
	}
	return nMsgs, nNLRI
}

func countNLRIField(buf []byte) int {
	n := 0
	for i := 0; i < len(buf); {
		plen := int(buf[i])
		i += 1 + (plen+7)/8
		n++
	}
	return n
}

func countMPReachNLRI(attrs []byte) int {
	n := 0
	for i := 0; i < len(attrs); {
		flags := attrs[i]
		typ := attrs[i+1]
		var hdr, attrLen int
		if flags&0x10 != 0 {
			attrLen = int(binary.BigEndian.Uint16(attrs[i+2 : i+4]))
			hdr = 4
		} else {
			attrLen = int(attrs[i+2])
			hdr = 3
		}
		value := attrs[i+hdr : i+hdr+attrLen]
		if typ == 14 {
			nhLen := int(value[3])
			pos := 4 + nhLen + 1
			for j := pos; j < len(value); {
				plen := int(value[j])
				j += 1 + (plen+7)/8
				n++
			}
		}
		i += hdr + attrLen
	}
	return n
}

// minimalOpenMsg builds a syntactically-valid OPEN advertising IPv4-unicast
// and 4-byte ASN capabilities. Used only by the end-to-end test as the
// active peer; ze-test peer mirrors the capabilities it sees.
func minimalOpenMsg(asn uint32, routerID string) []byte {
	rid := net.ParseIP(routerID).To4()
	capIPv4 := []byte{0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01}
	capASN4 := []byte{0x02, 0x06, 0x41, 0x04, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(capASN4[4:8], asn)
	optParams := make([]byte, 0, len(capIPv4)+len(capASN4))
	optParams = append(optParams, capIPv4...)
	optParams = append(optParams, capASN4...)
	body := make([]byte, 10+len(optParams))
	body[0] = 4
	//nolint:gosec // test uses a 2-byte-representable ASN
	binary.BigEndian.PutUint16(body[1:3], uint16(asn))
	binary.BigEndian.PutUint16(body[3:5], 180)
	copy(body[5:9], rid)
	body[9] = byte(len(optParams))
	copy(body[10:], optParams)

	msg := make([]byte, HeaderLen+len(body))
	copy(msg, Marker)
	//nolint:gosec // fits in uint16
	binary.BigEndian.PutUint16(msg[16:18], uint16(HeaderLen+len(body)))
	msg[18] = MsgOPEN
	copy(msg[HeaderLen:], body)
	return msg
}

// reservePort binds to :0 to get a free port, closes, and returns the number.
func reservePort(t *testing.T) int {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen :0: %v", err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listen addr is %T, not *net.TCPAddr", ln.Addr())
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close :0: %v", err)
	}
	return addr.Port
}

func closeIgnoreErr(c io.Closer) {
	if err := c.Close(); err != nil {
		_ = err // best-effort teardown in tests
	}
}
