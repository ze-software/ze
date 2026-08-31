// RFC 2866 (RADIUS Accounting) packet-format obligations ze meets as a client.
//
// VALIDATES: the Length field rules of Section 3 (octets outside the Length are
// padding, a datagram shorter than its Length is discarded), the Identifier
// rules of Section 4.1 (a new request takes a new Identifier), the
// Accounting-Response check of Section 4.2 (the Response Authenticator must be
// the correct response for the pending request), and the attribute value rule
// of Section 5 (an embedded null is data, never a terminator).
// PREVENTS: a truncated Accounting-Response read past its data, an accounting
// record accepted on a forged authenticator, a second request reusing the
// Identifier of the first, and an attribute value cut at its first null.
//
// ze sends Accounting-Request packets and receives Accounting-Response packets,
// so every requirement here is exercised through the client that speaks them.
// The producers are Decode and ResponseAuthenticator (packet.go), and
// Client.readLoop, Client.dispatchResponse and Client.SendToServers (client.go).

package radius

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

// startScriptedAcctServer answers every Accounting-Request with a valid
// Accounting-Response, then hands the encoded reply to mutate so one test can
// damage exactly one field. A nil mutate sends the reply unchanged. It records
// the Identifier of every request it reads.
func startScriptedAcctServer(t *testing.T, secret []byte, mutate func([]byte) []byte) (addr string, ids func() []uint8) {
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
			mu.Unlock()

			var reqAuth [AuthenticatorLen]byte
			copy(reqAuth[:], buf[4:4+AuthenticatorLen])
			resp := make([]byte, HeaderLen)
			resp[0] = CodeAccountingResp
			resp[1] = buf[1]
			binary.BigEndian.PutUint16(resp[2:4], HeaderLen)
			auth := ResponseAuthenticator(CodeAccountingResp, buf[1], HeaderLen, reqAuth, nil, secret)
			copy(resp[4:4+AuthenticatorLen], auth[:])
			if mutate != nil {
				resp = mutate(resp)
			}
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

// acctExchange sends one Accounting-Request to addr and reports whether the
// client accepted a reply. Retries are kept at one and the timeout short, so a
// discarded reply costs the test one timeout rather than three.
func acctExchange(t *testing.T, addr string, secret []byte) bool {
	t.Helper()
	client, err := NewClient(ClientConfig{
		Servers: []Server{{Address: addr, SharedKey: secret}},
		Timeout: 200 * time.Millisecond,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	pkt := &Packet{
		Code: CodeAccountingReq,
		Attrs: []Attr{
			{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
			{Type: AttrAcctSessionID, Value: AttrString("1-2-3")},
		},
	}
	_, err = client.SendToServers(context.Background(), pkt)
	return err == nil
}

// RFC requirement: RFC2866-3-4 positive -- an Accounting-Response that arrives
// with octets after the range its Length field covers is treated as padded: the
// padding is ignored, and the reply is accepted. The authenticator the client
// verifies is computed over the Length, so a decoder that read the padding as
// data would reject a conformant server.
func TestRFC2866LengthPaddingIgnoredOnReception(t *testing.T) {
	secret := []byte("acct-secret")
	padded := func(resp []byte) []byte {
		return append(resp, 0xDE, 0xAD, 0xBE, 0xEF)
	}
	addr, _ := startScriptedAcctServer(t, secret, padded)

	if !acctExchange(t, addr, secret) {
		t.Fatal("octets outside the Length field MUST be ignored as padding, not rejected")
	}
}

// RFC requirement: RFC2866-3-4 negative -- the boundary is the Length field and
// not the end of the datagram. An attribute that sits inside the Length decodes;
// the same attribute placed after it does not, so the padding is genuinely
// ignored rather than the decoder dropping whatever comes last.
func TestRFC2866LengthPaddingBoundaryIsTheLengthField(t *testing.T) {
	inside := []byte{AttrAcctSessionID, 5, 'a', 'b', 'c'}
	outside := []byte{AttrUserName, 7, 'i', 'n', 'v', 'a', 'd'}

	packet := make([]byte, HeaderLen)
	packet[0] = CodeAccountingResp
	packet[1] = 9
	packet = append(packet, inside...)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	packet = append(packet, outside...)

	decoded, err := Decode(packet)
	if err != nil {
		t.Fatalf("a padded packet MUST decode: %v", err)
	}
	if got := string(decoded.FindAttr(AttrAcctSessionID)); got != "abc" {
		t.Fatalf("attribute inside the Length: got %q, want %q", got, "abc")
	}
	if decoded.FindAttr(AttrUserName) != nil {
		t.Fatal("an attribute outside the Length field is padding and MUST NOT be read")
	}
}

// RFC requirement: RFC2866-3-5 positive -- a datagram shorter than its Length
// field claims is silently discarded at the entry point: the client's read loop
// drops it, so the exchange never completes and the record is retransmitted
// rather than acknowledged on a truncated reply.
func TestRFC2866ShortPacketSilentlyDiscarded(t *testing.T) {
	secret := []byte("acct-secret")
	overstate := func(resp []byte) []byte {
		binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)+8))
		return resp
	}
	addr, _ := startScriptedAcctServer(t, secret, overstate)

	if acctExchange(t, addr, secret) {
		t.Fatal("a packet shorter than its Length field MUST be silently discarded")
	}
}

// RFC requirement: RFC2866-3-5 negative -- the discard is caused by the length
// claim alone. The same server, answering with a Length that matches what it
// sent, completes the exchange.
func TestRFC2866HonestLengthIsAccepted(t *testing.T) {
	secret := []byte("acct-secret")
	addr, _ := startScriptedAcctServer(t, secret, nil)

	if !acctExchange(t, addr, secret) {
		t.Fatal("an Accounting-Response whose Length matches its datagram MUST be accepted")
	}
}

// RFC requirement: RFC2866-4.1-4 positive -- the Identifier changes when the
// content of the Attributes field changes and when a valid reply has been
// received for the previous request. Both triggers hold here: the second record
// is sent after the first is acknowledged, and it carries a different
// Acct-Status-Type.
func TestRFC2866IdentifierChangesForANewRequest(t *testing.T) {
	secret := []byte("acct-secret")
	addr, ids := startScriptedAcctServer(t, secret, nil)

	client, err := NewClient(ClientConfig{
		Servers: []Server{{Address: addr, SharedKey: secret}},
		Timeout: 500 * time.Millisecond,
		Retries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	for _, status := range []uint8{AcctStatusStart, AcctStatusStop} {
		pkt := &Packet{
			Code: CodeAccountingReq,
			Attrs: []Attr{
				{Type: AttrAcctStatusType, Value: AttrUint32(uint32(status))},
				{Type: AttrAcctSessionID, Value: AttrString("1-2-3")},
			},
		}
		if _, err := client.SendToServers(context.Background(), pkt); err != nil {
			t.Fatalf("accounting record %d: %v", status, err)
		}
	}

	got := ids()
	if len(got) != 2 {
		t.Fatalf("datagrams the server read: got %d, want 2", len(got))
	}
	if got[0] == got[1] {
		t.Fatalf("a new Accounting-Request reused Identifier %d", got[0])
	}
}

// RFC requirement: RFC2866-4.1-4 negative -- the change is a property of the
// counter and not of the pair of records above. NextID answers 256 distinct
// values in a row, so no second request can silently take the Identifier of the
// one before it.
func TestRFC2866IdentifierCounterCoversTheWholeSpace(t *testing.T) {
	client, err := NewClient(ClientConfig{Timeout: time.Second, Retries: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	seen := map[uint8]bool{}
	previous := client.NextID()
	for range 256 {
		id := client.NextID()
		if id == previous {
			t.Fatalf("NextID answered %d twice in a row", id)
		}
		seen[id] = true
		previous = id
	}
	if len(seen) != 256 {
		t.Fatalf("distinct Identifiers over 256 requests: got %d, want 256", len(seen))
	}
}

// RFC requirement: RFC2866-4.2-1 positive -- an Accounting-Response whose
// Response Authenticator is the correct response for the pending
// Accounting-Request is matched to it and accepted.
func TestRFC2866AccountingResponseAuthenticatorAccepted(t *testing.T) {
	secret := []byte("acct-secret")
	addr, _ := startScriptedAcctServer(t, secret, nil)

	if !acctExchange(t, addr, secret) {
		t.Fatal("a correct Response Authenticator MUST be accepted")
	}
}

// RFC requirement: RFC2866-4.2-1 negative -- an Accounting-Response whose
// Response Authenticator is not the correct response for the pending request is
// discarded, so a forged acknowledgement cannot retire an accounting record.
func TestRFC2866AccountingResponseAuthenticatorForgeryDiscarded(t *testing.T) {
	secret := []byte("acct-secret")
	forge := func(resp []byte) []byte {
		resp[4]++ // one bit of the Response Authenticator
		return resp
	}
	addr, _ := startScriptedAcctServer(t, secret, forge)

	if acctExchange(t, addr, secret) {
		t.Fatal("an Accounting-Response with a wrong Response Authenticator MUST be discarded")
	}
}

// RFC requirement: RFC2866-5-2 positive -- an attribute value carrying embedded
// nulls survives the wire: every octet the sender wrote is the octet the
// receiver reads, because the Length field bounds the value and no null ends it.
func TestRFC2866EmbeddedNullsSurviveTheWire(t *testing.T) {
	value := []byte{'a', 0x00, 'b', 0x00, 0x00, 'c'}
	pkt := &Packet{
		Code:       CodeAccountingReq,
		Identifier: 3,
		Attrs:      []Attr{{Type: AttrAcctSessionID, Value: value}},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.FindAttr(AttrAcctSessionID); !bytes.Equal(got, value) {
		t.Fatalf("attribute value: got %v, want %v", got, value)
	}
}

// RFC requirement: RFC2866-5-2 negative -- nothing reads a null as a terminator:
// a value that is nothing but nulls keeps its whole length, which a decoder that
// cut at the first null would report as empty.
func TestRFC2866AllNullValueKeepsItsLength(t *testing.T) {
	value := []byte{0x00, 0x00, 0x00, 0x00}
	pkt := &Packet{
		Code:       CodeAccountingReq,
		Identifier: 4,
		Attrs:      []Attr{{Type: AttrAcctSessionID, Value: value}},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.FindAttr(AttrAcctSessionID); len(got) != len(value) {
		t.Fatalf("attribute value length: got %d, want %d", len(got), len(value))
	}
}
