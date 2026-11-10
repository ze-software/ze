// Design: docs/architecture/testing/ci-format.md -- generated bulk UPDATEs.
// Related: peer.go -- handleConnection and the active Run entry.
// Related: peer_connmap.go -- acceptConnMapBatch and processConnBatch.

package peer

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGeneratedBulkAttributesFollowOPEN captures complete frames from real TCP
// sessions, including both OPENs, KEEPALIVEs, every generated prefix, EOR and an
// intentionally external-forbidden raw LOCAL_PREF. No encoder result supplies
// the expected UPDATE, and no caller supplies the session-classification bool.
//
// The passive entries accept on an already-owned ephemeral listener: ordinary
// handleConnection and actual acceptConnMapBatch/processConnBatch. Active runs
// Peer.Run against an owned listener. Dial takes precedence over ConnMap, so
// there is deliberately NO active-connmap case or claim of such support.
//
// RFC 4271 Section 5.1.5: "LOCAL_PREF is a well-known attribute that SHALL be included in all UPDATE messages that a given BGP speaker sends to other internal peers."
// RFC 4271 Section 5.1.5: "A BGP speaker MUST NOT include this attribute in UPDATE messages it sends to external peers, except in the case of BGP Confederations [RFC3065]."
// RFC 4271 Section 5.1.2: "the originating speaker includes an empty AS_PATH
// attribute in all UPDATE messages sent to internal peers."
// MUTATION: independently discard senderAS at the ordinary handshake handoff,
// discard connWithID.senderAS at the mapped handoff, always omit LOCAL_PREF in
// each family builder, always include it, write the one-AS AS_PATH on an
// internal session, or normalize raw send-action bytes.
// Each cut must fail the corresponding captured-wire assertion, not compilation.
func TestGeneratedBulkAttributesFollowOPEN(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6"} {
		for _, session := range []string{"ibgp", "ebgp"} {
			for _, rail := range []string{
				"active-ordinary", "passive-ordinary-accepted",
				"passive-connmap-router-id-accepted", "passive-connmap-remote-ip-accepted",
			} {
				t.Run(family+"/"+session+"/"+rail, func(t *testing.T) {
					runGeneratedBulkOPEN(t, family, session, rail)
				})
			}
		}
	}
}

const bulkOPENRemoteAS uint32 = 4200000001
const bulkOPENExternalAS uint32 = 4200000002

// This deliberately carries LOCAL_PREF=201 even on eBGP. Its complete frame is
// literal, independent of buildUpdates, and must survive the raw send arm intact.
const bulkOPENRaw = "ffffffffffffffffffffffffffffffff0036020000001b4001010040020602010000fe12400304c0000201400504000000c9180afe00"
const bulkOPENAck = "ffffffffffffffffffffffffffffffff00170200000000"

func runGeneratedBulkOPEN(t *testing.T, family, session, rail string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	network, host := "tcp4", "127.0.0.1"
	prefix, nextHop := "10.82.0.0/24", "192.0.2.82"
	if family == "ipv6" {
		network, host = "tcp6", "::1"
		prefix, nextHop = "2001:db8:82::/64", "2001:db8::82"
	}
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, network, net.JoinHostPort(host, "0"))
	if err != nil {
		t.Fatalf("owned %s listener: %v (IPv6 loopback is a prerequisite; no skip)", network, err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			if ctx.Err() == nil {
				t.Errorf("close listener: %v", err)
			}
		}
	}()
	stopListener := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stopListener()

	marker := filepath.Join(t.TempDir(), "established")
	var transcript bytes.Buffer
	config := &Config{
		Mode: ModeCheck, Output: &transcript, EstablishedFile: marker,
		SendBulk: []InjectSpec{{
			Prefix: netip.MustParsePrefix(prefix), NextHop: netip.MustParseAddr(nextHop),
			ASN: bulkOPENExternalAS, Count: 5, PerUpdate: 2, MaxMsgLen: 4096, EndOfRIB: true,
		}},
		Expect: []string{
			"action=send:conn=1:seq=1:hex=" + bulkOPENRaw,
			"expect=bgp:conn=1:seq=2:hex=" + bulkOPENAck,
		},
	}
	if session == "ebgp" {
		// A keyed declaration must win over an INTERNAL fallback. Reading a
		// config default instead of the actual OPEN therefore cannot pass.
		config.OpenAS = []OpenASBinding{
			{AS: bulkOPENRemoteAS},
			{Addr: netip.MustParseAddr(host), AS: bulkOPENExternalAS},
		}
	}
	if rail == "active-ordinary" {
		config.Dial = listener.Addr().String()
	}
	if rail == "passive-connmap-router-id-accepted" {
		config.ConnMap = connMapRouterID
	}
	if rail == "passive-connmap-remote-ip-accepted" {
		config.ConnMap = connMapRemoteIP
	}
	p, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan Result, 1)
	go runBulkOPENPeer(ctx, p, listener, result)
	// Every failure closes the listener/socket and joins the peer before the
	// test releases its directory. A result is not presumed from cancellation.
	defer func() {
		cancel()
		select {
		case peerResult := <-result:
			if !peerResult.Success {
				t.Errorf("peer entry failed: %v", peerResult.Error)
			}
			t.Logf("peer transcript:\n%s", transcript.String())
		case <-time.After(2 * time.Second):
			t.Error("peer entry did not join after closing its owned sockets")
		}
	}()

	var conn net.Conn
	if config.Dial != "" {
		conn, err = listener.Accept()
	} else {
		var dialer net.Dialer
		conn, err = dialer.DialContext(ctx, network, listener.Addr().String())
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	stopConn := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopConn()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}

	remoteOpen := bulkOPENMessage(family)
	writeBulkOPENFrame(t, conn, remoteOpen)
	peerOpen := readBulkOPENFrame(t, conn)
	peerAS := bulkOPENReadAS(t, peerOpen)
	remoteAS := bulkOPENReadAS(t, remoteOpen)
	wantAS := bulkOPENRemoteAS
	if session == "ebgp" {
		wantAS = bulkOPENExternalAS
	}
	if peerAS != wantAS {
		t.Errorf("actual sent OPEN AS=%d, want %d", peerAS, wantAS)
	}
	if (peerAS == remoteAS) != (session == "ibgp") {
		t.Errorf("actual OPEN pair %d/%d does not establish %s", peerAS, remoteAS, session)
	}
	keepalive := readBulkOPENFrame(t, conn)
	if !bytes.Equal(keepalive, KeepaliveMsg()) {
		t.Fatalf("OPEN was not followed by KEEPALIVE: %x", keepalive)
	}
	// RFC 4271 Section 8.2.2: finish the OPEN/KEEPALIVE exchange before
	// accepting generated traffic as evidence of an established session.
	writeBulkOPENFrame(t, conn, KeepaliveMsg())
	var generated [3][]byte
	for i := range generated {
		generated[i] = readBulkOPENFrame(t, conn)
	}
	eor := readBulkOPENFrame(t, conn)
	ack, err := hex.DecodeString(bulkOPENAck)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(eor, ack) {
		t.Errorf("generated stream did not terminate in complete EOR: %x", eor)
	}
	raw := readBulkOPENFrame(t, conn)
	if hex.EncodeToString(raw) != bulkOPENRaw {
		t.Errorf("raw hex was changed: got %x want %s", raw, bulkOPENRaw)
	}
	// This peer-received UPDATE must be consumed by the real checker and
	// markEstablished; a successful bulk write alone cannot satisfy it.
	writeBulkOPENFrame(t, conn, ack)
	select {
	case peerResult := <-result:
		result <- peerResult // The deferred sole join also checks the verdict.
		if !peerResult.Success {
			t.Fatalf("peer did not consume post-establishment acknowledgment: %v", peerResult.Error)
		}
	case <-ctx.Done():
		t.Fatal("peer did not finish after the complete generated/raw transmission")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("real UPDATE-receive Established marker: %v", err)
	}
	var prefixes []netip.Prefix
	for _, frame := range generated {
		prefixes = append(prefixes, checkBulkOPENUpdate(t, frame, family, peerAS == remoteAS)...)
	}
	wantPrefixes := []string{"10.82.0.0/24", "10.82.1.0/24", "10.82.2.0/24", "10.82.3.0/24", "10.82.4.0/24"}
	if family == "ipv6" {
		wantPrefixes = []string{"2001:db8:82::/64", "2001:db8:82:1::/64", "2001:db8:82:2::/64", "2001:db8:82:3::/64", "2001:db8:82:4::/64"}
	}
	if len(prefixes) != len(wantPrefixes) {
		t.Fatalf("transmitted prefixes=%v, want %v", prefixes, wantPrefixes)
	}
	for i, want := range wantPrefixes {
		if prefixes[i] != netip.MustParsePrefix(want) {
			t.Errorf("transmitted prefix %d=%s, want %s", i, prefixes[i], want)
		}
	}
}

// runBulkOPENPeer owns one real peer session and MUST publish its result after
// closing its accepted socket. The caller MUST cancel and consume that result.
func runBulkOPENPeer(ctx context.Context, p *Peer, listener net.Listener, result chan<- Result) {
	if p.config.Dial != "" {
		result <- p.Run(ctx)
		return
	}
	if p.config.ConnMap != "" {
		conns, outcome, done := p.acceptConnMapBatch(ctx, listener, 1)
		if done {
			result <- outcome
			return
		}
		sortConnBatch(conns, p.config.ConnMap)
		outcome = p.processConnBatch(ctx, conns)
		closeConnBatch(conns)
		result <- outcome
		return
	}
	conn, err := listener.Accept()
	if err != nil {
		result <- Result{Error: err}
		return
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	outcome := p.handleConnection(ctx, conn)
	stop()
	if err := conn.Close(); err != nil {
		if ctx.Err() == nil {
			outcome = Result{Error: err}
		}
	}
	result <- outcome
}

// bulkOPENMessage supplies an independent RFC 4271 Section 4.2 OPEN, with the
// RFC 6793 Section 3 four-octet ASN and RFC 4760 Section 8 family capabilities.
func bulkOPENMessage(family string) []byte {
	afi := byte(1)
	if family == "ipv6" {
		afi = 2
	}
	frame := make([]byte, 45)
	copy(frame, Marker)
	binary.BigEndian.PutUint16(frame[16:18], uint16(len(frame)))
	frame[18] = MsgOPEN
	copy(frame[19:], []byte{4, 0x5b, 0xa0, 0, 90, 192, 0, 2, 81, 16,
		2, 6, 1, 4, 0, afi, 0, 1, 2, 6, 65, 4, 0, 0, 0, 0})
	binary.BigEndian.PutUint32(frame[41:45], bulkOPENRemoteAS)
	return frame
}

func writeBulkOPENFrame(t *testing.T, conn net.Conn, frame []byte) {
	t.Helper()
	t.Logf("remote sent complete frame: %x", frame)
	n, err := conn.Write(frame)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(frame) {
		t.Fatalf("short wire write: %d/%d", n, len(frame))
	}
}

// readBulkOPENFrame reads framing independently of newMessageReader so a shared
// truncation or length error cannot certify a complete transmission.
func readBulkOPENFrame(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	header := make([]byte, 19)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("capture full header: %v", err)
	}
	if !bytes.Equal(header[:16], bytes.Repeat([]byte{0xff}, 16)) {
		t.Fatalf("invalid captured marker: %x", header)
	}
	length := int(binary.BigEndian.Uint16(header[16:18]))
	if length < 19 {
		t.Fatalf("invalid captured length %d", length)
	}
	frame := make([]byte, length)
	copy(frame, header)
	if _, err := io.ReadFull(conn, frame[19:]); err != nil {
		t.Fatalf("capture complete %d-byte frame: %v", length, err)
	}
	t.Logf("remote received complete frame: %x", frame)
	return frame
}

// bulkOPENReadAS reads the ASN from captured OPEN bytes without calling the
// classifier or identity resolver whose handoff the matrix is intended to test.
func bulkOPENReadAS(t *testing.T, frame []byte) uint32 {
	t.Helper()
	if len(frame) < 29 {
		t.Fatalf("short OPEN: %x", frame)
	}
	if frame[18] != MsgOPEN {
		t.Fatalf("expected actual OPEN, got %x", frame)
	}
	asn := uint32(binary.BigEndian.Uint16(frame[20:22]))
	if int(frame[28])+29 != len(frame) {
		t.Fatalf("OPEN optional parameter framing: %x", frame)
	}
	found := false
	for rest := frame[29:]; len(rest) > 0; {
		if len(rest) < 2 {
			t.Fatal("truncated OPEN parameter")
		}
		n := int(rest[1])
		if n+2 > len(rest) {
			t.Fatal("OPEN parameter exceeds frame")
		}
		if rest[0] == 2 {
			for caps := rest[2 : n+2]; len(caps) > 0; {
				if len(caps) < 2 {
					t.Fatal("truncated capability")
				}
				width := int(caps[1])
				if width+2 > len(caps) {
					t.Fatal("capability exceeds parameter")
				}
				if caps[0] == 65 {
					if width != 4 {
						t.Fatal("invalid ASN4 width")
					}
					if found {
						t.Fatal("duplicate ASN4 carrier")
					}
					asn = binary.BigEndian.Uint32(caps[2:6])
					found = true
				}
				caps = caps[width+2:]
			}
		}
		rest = rest[n+2:]
	}
	if !found {
		t.Fatal("actual OPEN lost the offered ASN4 capability")
	}
	if binary.BigEndian.Uint16(frame[20:22]) != 23456 {
		t.Fatal("four-octet ASN did not use AS_TRANS in My Autonomous System")
	}
	return asn
}

func checkBulkOPENUpdate(t *testing.T, frame []byte, family string, internal bool) []netip.Prefix {
	t.Helper()
	if len(frame) < 23 {
		t.Fatalf("short generated UPDATE: %x", frame)
	}
	if frame[18] != MsgUPDATE {
		t.Fatalf("generated frame is not UPDATE: %x", frame)
	}
	if binary.BigEndian.Uint16(frame[19:21]) != 0 {
		t.Fatal("generated advertisement contains withdrawals")
	}
	end := 23 + int(binary.BigEndian.Uint16(frame[21:23]))
	if end > len(frame) {
		t.Fatal("attribute block exceeds complete generated frame")
	}
	localPrefs := 0
	originCount, pathCount, nextHopCount, reachCount := 0, 0, 0, 0
	nlri := frame[end:]
	for rest := frame[23:end]; len(rest) > 0; {
		if len(rest) < 3 {
			t.Fatal("truncated generated attribute")
		}
		flags, code := rest[0], rest[1]
		header, length := 3, int(rest[2])
		if flags&0x10 != 0 {
			if len(rest) < 4 {
				t.Fatal("truncated extended attribute")
			}
			header, length = 4, int(binary.BigEndian.Uint16(rest[2:4]))
		}
		if header+length > len(rest) {
			t.Fatal("attribute consumes bytes beyond declared attribute block")
		}
		value := rest[header : header+length]
		if code == 1 {
			originCount++
			if !bytes.Equal(rest[:header+length], []byte{0x40, 1, 1, 0}) {
				t.Errorf("generated ORIGIN was damaged: %x", rest[:header+length])
			}
		}
		if code == 2 {
			pathCount++
			// RFC 4271 Section 5.1.2: "the originating speaker includes an
			// empty AS_PATH attribute in all UPDATE messages sent to internal
			// peers."
			want := []byte{0x40, 2, 0}
			if !internal {
				want = []byte{0x40, 2, 6, 2, 1, 0, 0, 0, 0}
				binary.BigEndian.PutUint32(want[5:], bulkOPENExternalAS)
			}
			if !bytes.Equal(rest[:header+length], want) {
				t.Errorf("generated AS_PATH was damaged: %x", rest[:header+length])
			}
		}
		if code == 3 {
			nextHopCount++
			if !bytes.Equal(rest[:header+length], []byte{0x40, 3, 4, 192, 0, 2, 82}) {
				t.Errorf("generated NEXT_HOP was damaged: %x", rest[:header+length])
			}
		}
		if code == 5 {
			localPrefs++
			if !bytes.Equal(rest[:header+length], []byte{0x40, 5, 4, 0, 0, 0, 100}) {
				t.Errorf("LOCAL_PREF must have exact flags/width/value 100: %x", rest[:header+length])
			}
		}
		if code == 14 {
			reachCount++
			if len(value) < 21 {
				t.Fatal("short MP_REACH")
			}
			if !bytes.Equal(value[:4], []byte{0, 2, 1, 16}) {
				t.Fatalf("wrong MP_REACH family/next-hop width: %x", value)
			}
			nextHop := netip.MustParseAddr("2001:db8::82").As16()
			if !bytes.Equal(value[4:20], nextHop[:]) {
				t.Errorf("wrong IPv6 next hop: %x", value[4:20])
			}
			nlri = value[21:]
		}
		rest = rest[header+length:]
	}
	if originCount != 1 {
		t.Errorf("generated ORIGIN count=%d, want 1", originCount)
	}
	if pathCount != 1 {
		t.Errorf("generated AS_PATH count=%d, want 1", pathCount)
	}
	if family == "ipv4" {
		if nextHopCount != 1 {
			t.Errorf("generated IPv4 NEXT_HOP count=%d, want 1", nextHopCount)
		}
		if reachCount != 0 {
			t.Errorf("unexpected MP_REACH on generated IPv4: %d", reachCount)
		}
	} else {
		if reachCount != 1 {
			t.Errorf("generated IPv6 MP_REACH count=%d, want 1", reachCount)
		}
		if nextHopCount != 0 {
			t.Errorf("unexpected NEXT_HOP on generated IPv6: %d", nextHopCount)
		}
		if end != len(frame) {
			t.Error("generated IPv6 UPDATE has trailing IPv4 NLRI")
		}
	}
	wantLocalPrefs := 0
	if internal {
		wantLocalPrefs = 1
	}
	if localPrefs != wantLocalPrefs {
		t.Errorf("actual OPEN-classified internal=%v: LOCAL_PREF count=%d want=%d; complete UPDATE=%x", internal, localPrefs, wantLocalPrefs, frame)
	}
	var prefixes []netip.Prefix
	for len(nlri) > 0 {
		bits := int(nlri[0])
		width := (bits + 7) / 8
		maximum := 32
		if family == "ipv6" {
			maximum = 128
		}
		if bits > maximum {
			t.Fatal("NLRI prefix length exceeds family width")
		}
		if width+1 > len(nlri) {
			t.Fatal("truncated generated NLRI")
		}
		var raw [16]byte
		copy(raw[:], nlri[1:width+1])
		addr := netip.AddrFrom16(raw)
		if family == "ipv4" {
			addr = netip.AddrFrom4([4]byte{raw[0], raw[1], raw[2], raw[3]})
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, bits))
		nlri = nlri[width+1:]
	}
	if len(prefixes) == 0 {
		t.Fatalf("generated %s frame carried no prefixes: %x", family, frame)
	}
	return prefixes
}
