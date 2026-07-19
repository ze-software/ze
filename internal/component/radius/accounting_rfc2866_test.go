// RFC 2866 (RADIUS Accounting) client-side wire requirements.
//
// VALIDATES: RFC 2866 Section 3 accounting authenticator formula
// (MD5 over Code+ID+Length+16 zero octets+Attributes+Secret) and the RFC 2865
// Section 2.5 retransmit rule that a retransmitted Accounting-Request reuses its Identifier.
// PREVENTS: an accounting authenticator that ignores a covered field, and a retransmit
// that changes the Identifier (which a server would treat as a new request).
//
// ze is a RADIUS accounting client (NAS): it sends Accounting-Request packets
// and never receives them as a server. The producers exercised here are:
//   - AccountingRequestAuth (packet.go) -- the computed accounting authenticator (RFC 2866 Section 3)
//   - Client.Exchange retry loop (client.go) -- retransmit reuses the Identifier (RFC 2865 Section 2.5)

package radius

import (
	"context"
	"crypto/md5" //nolint:gosec // RFC 2866 Section 3 mandates MD5 for the accounting authenticator
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

// startRecordingAcctServer starts a mock RADIUS accounting server that records
// the Identifier of every datagram it receives. When dropFirst is set it silently
// drops the first datagram to force the client to retransmit; every subsequent
// datagram is answered with a valid Accounting-Response.
func startRecordingAcctServer(t *testing.T, secret []byte, dropFirst bool) (addr string, ids func() []uint8) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var seen []uint8
	done := make(chan struct{})

	go func() {
		defer close(done)
		buf := make([]byte, MaxPacketLen)
		for {
			_, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			mu.Lock()
			seen = append(seen, buf[1])
			count := len(seen)
			mu.Unlock()

			if dropFirst && count == 1 {
				continue // drop the first datagram to force a retransmit
			}

			var reqAuth [AuthenticatorLen]byte
			copy(reqAuth[:], buf[4:4+AuthenticatorLen])
			resp := make([]byte, HeaderLen)
			resp[0] = CodeAccountingResp
			resp[1] = buf[1]
			binary.BigEndian.PutUint16(resp[2:4], HeaderLen)
			auth := ResponseAuthenticator(CodeAccountingResp, buf[1], HeaderLen, reqAuth, nil, secret)
			copy(resp[4:4+AuthenticatorLen], auth[:])
			conn.WriteToUDP(resp, from) //nolint:errcheck // test mock best-effort
		}
	}()

	t.Cleanup(func() {
		conn.Close() //nolint:errcheck // test cleanup
		<-done
	})

	return conn.LocalAddr().String(), func() []uint8 {
		mu.Lock()
		defer mu.Unlock()
		out := make([]uint8, len(seen))
		copy(out, seen)
		return out
	}
}

// RFC requirement: RFC2866-3-2 positive -- the Accounting-Request authenticator equals
// MD5(Code + Identifier + Length + 16 zero octets + Attributes + Secret), and the 16 octets
// that carry the authenticator on the wire are treated as zero during the computation.
func TestRFC2866AccountingRequestAuthFormula(t *testing.T) {
	secret := []byte("acct-secret")
	pkt := &Packet{
		Code:       CodeAccountingReq,
		Identifier: 77,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("alice")},
			{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
			{Type: AttrAcctSessionID, Value: AttrString("1-2-3")},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Put non-zero garbage where the authenticator lives on the wire to prove the
	// formula substitutes 16 zero octets rather than hashing the wire bytes.
	for i := 4; i < 4+AuthenticatorLen; i++ {
		buf[i] = 0xAB
	}

	got := AccountingRequestAuth(buf, n, secret)

	// Independent reference implementation of the RFC 2866 Section 3 formula.
	h := md5.New()                          //nolint:gosec // RFC 2866 Section 3 mandates MD5
	h.Write(buf[:4])                        // Code + Identifier + Length
	h.Write(make([]byte, AuthenticatorLen)) // 16 zero octets
	h.Write(buf[HeaderLen:n])               // Attributes
	h.Write(secret)                         // Shared secret
	var want [AuthenticatorLen]byte
	copy(want[:], h.Sum(nil))

	if got != want {
		t.Fatalf("accounting authenticator: got %x, want %x", got, want)
	}
}

// RFC requirement: RFC2866-3-2 negative -- the authenticator genuinely covers every field
// the formula names: changing the secret, an attribute byte, or the Code each yields a
// different authenticator, so a conformant server verifying the packet rejects a mismatch.
func TestRFC2866AccountingRequestAuthRejectsTampering(t *testing.T) {
	secret := []byte("acct-secret")
	pkt := &Packet{
		Code:       CodeAccountingReq,
		Identifier: 77,
		Attrs: []Attr{
			{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStop)},
			{Type: AttrAcctSessionID, Value: AttrString("1-2-3")},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	base := AccountingRequestAuth(buf, n, secret)

	if AccountingRequestAuth(buf, n, []byte("other-secret")) == base {
		t.Error("authenticator did not depend on the shared secret")
	}

	tampered := make([]byte, n)
	copy(tampered, buf[:n])
	tampered[HeaderLen+2]++ // first byte of the first attribute value
	if AccountingRequestAuth(tampered, n, secret) == base {
		t.Error("authenticator did not depend on the attributes")
	}

	code := make([]byte, n)
	copy(code, buf[:n])
	code[0] = CodeAccessRequest
	if AccountingRequestAuth(code, n, secret) == base {
		t.Error("authenticator did not depend on the Code field")
	}
}

// RFC requirement: RFC2866-3-3 positive -- a retransmitted Accounting-Request carries the
// same Identifier as the original transmission (RFC 2865 Section 2.5 retransmit rules,
// applied to Accounting-Request by RFC 2866 Section 3).
func TestRFC2866AccountingRetransmitSameIdentifier(t *testing.T) {
	secret := []byte("acct-secret")
	addr, ids := startRecordingAcctServer(t, secret, true)

	client, err := NewClient(ClientConfig{
		Timeout: 150 * time.Millisecond,
		Retries: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	pkt := &Packet{
		Code:       CodeAccountingReq,
		Identifier: client.NextID(),
		Attrs: []Attr{
			{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
		},
	}

	resp, err := client.Exchange(context.Background(), pkt, secret, addr)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeAccountingResp {
		t.Fatalf("response code: got %d, want %d", resp.Code, CodeAccountingResp)
	}

	got := ids()
	if len(got) < 2 {
		t.Fatalf("expected at least 2 datagrams (original + retransmit), got %d", len(got))
	}
	for i, id := range got {
		if id != got[0] {
			t.Fatalf("datagram %d used Identifier %d, want %d: retransmit must reuse the Identifier", i, id, got[0])
		}
	}
}

// RFC requirement: RFC2866-3-3 negative -- two distinct Accounting-Requests are assigned
// different Identifiers (NextID advances), so the retransmit invariant is a genuine property
// and not the trivial case of every packet sharing a single Identifier.
func TestRFC2866AccountingDistinctRequestsDifferIdentifier(t *testing.T) {
	secret := []byte("acct-secret")
	addr, ids := startRecordingAcctServer(t, secret, false)

	client, err := NewClient(ClientConfig{
		Servers: []Server{{Address: addr, SharedKey: secret}},
		Timeout: 500 * time.Millisecond,
		Retries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	for i := range 2 {
		pkt := &Packet{
			Code: CodeAccountingReq,
			Attrs: []Attr{
				{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
			},
		}
		if _, err := client.SendToServers(context.Background(), pkt); err != nil {
			t.Fatalf("accounting exchange %d: %v", i, err)
		}
	}

	got := ids()
	if len(got) < 2 {
		t.Fatalf("expected 2 datagrams, got %d", len(got))
	}
	if got[0] == got[1] {
		t.Fatalf("two distinct Accounting-Requests reused Identifier %d", got[0])
	}
}
