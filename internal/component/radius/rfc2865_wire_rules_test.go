// RFC: rfc/short/rfc2865.md -- the packet and attribute wire rules of Sections 2.5, 3 and 5
// Related: packet.go -- EncodeTo and Decode, the producers of the wire rules
// Related: client.go -- Exchange, the retransmit loop and the response path

// Tests that read the octets ze puts on the wire, or that feed Decode and
// Client.Exchange a datagram built octet by octet. Each test states the clause
// of the RFC sentence it proves, and each negative input breaks that clause
// alone, so a refusal for another reason cannot turn it green.
package radius

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// datagramCaptureServer records every datagram it reads, with the address it
// came from, and answers through reply. The reply function sees the index of
// the datagram, so a test can leave the first one unanswered to force a
// retransmission. A nil reply sends nothing. Safe for concurrent use.
type datagramCaptureServer struct {
	conn      *net.UDPConn
	addr      string
	reply     func(index int, req []byte) []byte
	done      chan struct{}
	mu        sync.Mutex
	datagrams [][]byte
	sources   []string
}

// newDatagramCaptureServer starts the server on a loopback port. The test
// cleanup closes the socket and waits for the reader to stop.
func newDatagramCaptureServer(t *testing.T, reply func(index int, req []byte) []byte) *datagramCaptureServer {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	s := &datagramCaptureServer{conn: conn, addr: conn.LocalAddr().String(), reply: reply, done: make(chan struct{})}
	go s.serve()
	t.Cleanup(func() {
		conn.Close() //nolint:errcheck // test cleanup
		<-s.done
	})
	return s
}

// serve reads until the socket closes.
func (s *datagramCaptureServer) serve() {
	defer close(s.done)
	buf := make([]byte, MaxPacketLen+64)
	for {
		n, from, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		req := append([]byte{}, buf[:n]...)
		s.mu.Lock()
		index := len(s.datagrams)
		s.datagrams = append(s.datagrams, req)
		s.sources = append(s.sources, from.String())
		s.mu.Unlock()
		if resp := s.reply(index, req); resp != nil {
			s.conn.WriteToUDP(resp, from) //nolint:errcheck // test mock best-effort
		}
	}
}

// snapshot returns copies of the datagrams and their source addresses so far.
func (s *datagramCaptureServer) snapshot() (datagrams [][]byte, sources []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte{}, s.datagrams...), append([]string{}, s.sources...)
}

// resignResponse recomputes the Response Authenticator of a reply after a test
// changed its octets, so the only defect left in the reply is the one the test
// put there. RFC 2865 Section 3: MD5(Code+ID+Length+RequestAuth+Attributes+Secret).
func resignResponse(resp, req, secret []byte) {
	var requestAuth [AuthenticatorLen]byte
	copy(requestAuth[:], req[4:4+AuthenticatorLen])
	length := binary.BigEndian.Uint16(resp[2:4])
	auth := ResponseAuthenticator(resp[0], resp[1], length, requestAuth, resp[HeaderLen:length], secret)
	copy(resp[4:4+AuthenticatorLen], auth[:])
}

// wireRulesExchange sends one Access-Request to addr with a short timeout and a
// single attempt, so a discarded reply costs one timeout.
func wireRulesExchange(t *testing.T, addr string, secret []byte) (*Packet, error) {
	t.Helper()
	client, err := NewClient(ClientConfig{Timeout: 200 * time.Millisecond, Retries: 1})
	require.NoError(t, err)
	defer closeSilent(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return client.Exchange(ctx, accessRequest(t, client.NextID()), secret, addr)
}

// VALIDATES: a retransmission to the same server repeats the Identifier, the
// Request Authenticator and the source port of the first transmission.
// PREVENTS: a retransmit that the server cannot recognize as a duplicate.
// The server leaves the first datagram unanswered and answers the second, so
// the client retransmits over its real Exchange loop. The attributes are
// asserted unchanged first, because the sentence binds only that case.
//
// RFC requirement: RFC2865-2.5-1 positive -- a retransmitted Access-Request whose
// attributes are unchanged carries the same Identifier octet, the same 16
// Request Authenticator octets and the same source address and port as the first
// datagram (client.go Exchange).
func TestRFC2865RetransmitRepeatsIdentifierAuthenticatorAndSourcePort(t *testing.T) {
	key := []byte("testing123")
	srv := newDatagramCaptureServer(t, func(index int, req []byte) []byte {
		if index == 0 {
			return nil
		}
		return buildResponse(CodeAccessAccept, req, key)
	})

	client, err := NewClient(ClientConfig{Timeout: 150 * time.Millisecond, Retries: 3})
	require.NoError(t, err)
	defer closeSilent(client)

	resp, err := client.Exchange(context.Background(), accessRequest(t, client.NextID()), key, srv.addr)
	require.NoError(t, err)
	require.Equal(t, uint8(CodeAccessAccept), resp.Code)

	datagrams, sources := srv.snapshot()
	require.GreaterOrEqual(t, len(datagrams), 2, "the first request must have gone unanswered and been resent")
	first := datagrams[0]
	for index, again := range datagrams[1:] {
		require.Equal(t, first[HeaderLen:], again[HeaderLen:], "retransmit %d: the attributes are unchanged, so the rule applies", index+1)
		assert.Equal(t, first[1], again[1], "retransmit %d: the Identifier", index+1)
		assert.Equal(t, first[4:4+AuthenticatorLen], again[4:4+AuthenticatorLen], "retransmit %d: the Request Authenticator", index+1)
		assert.Equal(t, sources[0], sources[index+1], "retransmit %d: the source address and port", index+1)
	}
}

// packetOfLength builds a well-formed Access-Accept of exactly total octets:
// the header, then Filter-Id attributes of at most 255 octets each. total MUST
// be at least 20 and MUST NOT leave a remainder of one octet.
func packetOfLength(t *testing.T, total int) []byte {
	t.Helper()
	wire := make([]byte, total)
	wire[0] = CodeAccessAccept
	wire[1] = 7
	binary.BigEndian.PutUint16(wire[2:4], uint16(total))
	off := HeaderLen
	for off < total {
		attrLen := min(MaxAttrLen, total-off)
		require.GreaterOrEqual(t, attrLen, 2, "the length leaves a one-octet remainder")
		wire[off] = AttrFilterID
		wire[off+1] = uint8(attrLen)
		for index := off + 2; index < off+attrLen; index++ {
			wire[index] = 'a'
		}
		off += attrLen
	}
	return wire
}

// VALIDATES: the two packet sizes the bounds name are accepted: 20 octets, the
// header alone, and 4096 octets filled with valid attributes. EncodeTo builds a
// packet of exactly 4096 octets.
// PREVENTS: an off-by-one at either bound that refuses a legal packet.
//
// RFC requirement: RFC2865-3-1 positive -- Decode accepts a packet whose Length
// and datagram are both 20 octets, and one whose Length and datagram are both
// 4096 octets, keeping all 16 attributes; EncodeTo writes a 4096-octet packet
// and puts 4096 in its Length field (packet.go Decode, EncodeTo).
func TestRFC2865PacketOfTheMinimumAndMaximumLengthIsAccepted(t *testing.T) {
	shortest, err := Decode(packetOfLength(t, MinPacketLen))
	require.NoError(t, err)
	assert.Empty(t, shortest.Attrs)

	longest, err := Decode(packetOfLength(t, MaxPacketLen))
	require.NoError(t, err)
	assert.Len(t, longest.Attrs, 16, "15 attributes of 255 octets and one of 251")

	attrs := make([]Attr, 0, 16)
	for range 15 {
		attrs = append(attrs, Attr{Type: AttrFilterID, Value: bytes.Repeat([]byte{'a'}, MaxAttrLen-2)})
	}
	attrs = append(attrs, Attr{Type: AttrFilterID, Value: bytes.Repeat([]byte{'a'}, 249)})
	buf := make([]byte, MaxPacketLen)
	n, err := (&Packet{Code: CodeAccessRequest, Identifier: 1, Attrs: attrs}).EncodeTo(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, MaxPacketLen, n)
	assert.Equal(t, uint16(MaxPacketLen), binary.BigEndian.Uint16(buf[2:4]))
}

// VALIDATES: a packet one octet under the minimum or one octet over the
// maximum is refused, on reception and on transmission.
// PREVENTS: a decoder that bounds the datagram only from below, or an encoder
// that emits a packet no conformant server can accept.
// The 4097-octet input is well formed in every other way: its Length field
// matches the datagram and every attribute Length is valid, so only the
// maximum can refuse it.
//
// RFC requirement: RFC2865-3-1 negative -- Decode refuses a 19-octet datagram
// and a 4097-octet packet whose Length field says 4097, and EncodeTo refuses
// attributes that would make the packet 4097 octets (packet.go Decode,
// EncodeTo).
func TestRFC2865PacketOutsideTheLengthBoundsIsRefused(t *testing.T) {
	_, err := Decode(packetOfLength(t, MinPacketLen)[:MinPacketLen-1])
	require.Error(t, err, "19 octets is under the minimum")

	_, err = Decode(packetOfLength(t, MaxPacketLen+1))
	require.Error(t, err, "4097 octets is over the maximum")

	attrs := make([]Attr, 0, 16)
	for range 15 {
		attrs = append(attrs, Attr{Type: AttrFilterID, Value: bytes.Repeat([]byte{'a'}, MaxAttrLen-2)})
	}
	attrs = append(attrs, Attr{Type: AttrFilterID, Value: bytes.Repeat([]byte{'a'}, 250)})
	buf := make([]byte, MaxPacketLen+MaxAttrLen)
	_, err = (&Packet{Code: CodeAccessRequest, Identifier: 1, Attrs: attrs}).EncodeTo(buf, 0)
	require.Error(t, err, "the attributes make a 4097-octet packet")
}

// VALIDATES: the attribute Length octet ze writes counts the Type, the Length
// and the Value octets, and the decoder reads it the same way.
// PREVENTS: an encoder and a decoder that both count only the Value: they
// round-trip each other and fail against every other implementation.
//
// RFC requirement: RFC2865-5-2 positive -- EncodeTo writes a Length octet of 3,
// 8 and 255 for values of 1, 6 and 253 octets, and Decode reads a Filter-Id
// whose Length octet is 8 as the six-octet value "netops" (packet.go EncodeTo,
// Decode).
func TestRFC2865AttributeLengthOctetCountsTypeLengthAndValue(t *testing.T) {
	values := [][]byte{{'x'}, []byte("netops"), bytes.Repeat([]byte{'v'}, MaxAttrLen-2)}
	attrs := make([]Attr, 0, len(values))
	for _, value := range values {
		attrs = append(attrs, Attr{Type: AttrFilterID, Value: value})
	}
	buf := make([]byte, MaxPacketLen)
	n, err := (&Packet{Code: CodeAccessRequest, Identifier: 1, Attrs: attrs}).EncodeTo(buf, 0)
	require.NoError(t, err)

	off := HeaderLen
	for _, want := range []uint8{3, 8, 255} {
		require.Less(t, off+1, n)
		assert.Equal(t, uint8(AttrFilterID), buf[off], "Type at offset %d", off)
		assert.Equal(t, want, buf[off+1], "Length octet at offset %d", off)
		off += int(buf[off+1])
	}
	assert.Equal(t, n, off, "the Length octets walk exactly to the end of the packet")

	wire := append(packetOfLength(t, MinPacketLen), AttrFilterID, 8, 'n', 'e', 't', 'o', 'p', 's')
	binary.BigEndian.PutUint16(wire[2:4], uint16(len(wire)))
	decoded, err := Decode(wire)
	require.NoError(t, err)
	require.Len(t, decoded.Attrs, 1)
	assert.Equal(t, []byte("netops"), decoded.Attrs[0].Value)
}

// VALIDATES: an attribute whose Length octet counts only its Value is refused.
// PREVENTS: a decoder that reads the Length octet as the Value length, which
// accepts this packet and reads "netops".
// The packet Length is right for eight attribute octets, so under the RFC
// reading the attribute is "neto" followed by a two-octet attribute header
// that claims 115 octets, past the end of the packet.
//
// RFC requirement: RFC2865-5-2 negative -- Decode refuses a Filter-Id "netops"
// whose Length octet is 6, the Value length alone (packet.go Decode).
func TestRFC2865AttributeLengthOctetThatOmitsTheHeaderIsRefused(t *testing.T) {
	wire := append(packetOfLength(t, MinPacketLen), AttrFilterID, 6, 'n', 'e', 't', 'o', 'p', 's')
	binary.BigEndian.PutUint16(wire[2:4], uint16(len(wire)))

	decoded, err := Decode(wire)
	require.Error(t, err, "the attribute was decoded as %v", decoded)
}

// VALIDATES: an Access-Reject and an Access-Challenge whose attribute lengths
// are all valid reach the caller with their code.
// PREVENTS: the invalid-length guard discarding a well-formed reply of either
// code, which would read as a timeout.
//
// RFC requirement: RFC2865-5-6 positive -- Client.Exchange returns an
// Access-Reject and an Access-Challenge that each carry a valid Reply-Message
// attribute, with the code the server sent (client.go Exchange).
func TestRFC2865RejectAndChallengeWithValidAttributeLengthsAreDelivered(t *testing.T) {
	key := []byte("testing123")
	for _, code := range []uint8{CodeAccessReject, CodeAccessChallenge} {
		srv := newDatagramCaptureServer(t, func(_ int, req []byte) []byte {
			return buildReplyResponse(code, req, key, []Attr{{Type: AttrReplyMessage, Value: []byte("denied")}})
		})
		resp, err := wireRulesExchange(t, srv.addr, key)
		require.NoError(t, err, "code %d", code)
		assert.Equal(t, code, resp.Code)
		assert.Equal(t, []byte("denied"), resp.FindAttr(AttrReplyMessage))
	}
}

// VALIDATES: an Access-Accept, an Access-Reject or an Access-Challenge that
// carries an attribute with an invalid length is not delivered to the caller.
// PREVENTS: acting on a reply the server never sent whole.
// Two invalid lengths are driven for each code: 1, under the two-octet header,
// and a Length that runs five octets past the end of the packet. The Response
// Authenticator is recomputed over the damaged octets, so the reply passes the
// authenticator check and only the length rule can refuse it.
//
// RFC requirement: RFC2865-5-6 negative -- for each of the three codes and each
// of the two invalid lengths, the damaged reply still passes VerifyResponseAuth,
// Decode refuses it, and Client.Exchange returns an error with no packet, the
// silent discard (client.go Exchange, packet.go Decode).
func TestRFC2865ReplyWithAnInvalidAttributeLengthIsDiscarded(t *testing.T) {
	key := []byte("testing123")
	damages := map[string]func(attrLen uint8) uint8{
		"under the header":    func(uint8) uint8 { return 1 },
		"past the packet end": func(attrLen uint8) uint8 { return attrLen + 5 },
	}
	for _, code := range []uint8{CodeAccessAccept, CodeAccessReject, CodeAccessChallenge} {
		for name, damage := range damages {
			var mu sync.Mutex
			var sent, request []byte
			srv := newDatagramCaptureServer(t, func(_ int, req []byte) []byte {
				resp := buildReplyResponse(code, req, key, []Attr{{Type: AttrReplyMessage, Value: []byte("hello")}})
				resp[HeaderLen+1] = damage(resp[HeaderLen+1])
				resignResponse(resp, req, key)
				mu.Lock()
				sent, request = append([]byte{}, resp...), append([]byte{}, req...)
				mu.Unlock()
				return resp
			})

			resp, err := wireRulesExchange(t, srv.addr, key)
			require.Error(t, err, "code %d, %s: the reply was delivered as %v", code, name, resp)

			mu.Lock()
			require.NotNil(t, sent, "code %d, %s: the server never answered", code, name)
			var requestAuth [AuthenticatorLen]byte
			copy(requestAuth[:], request[4:4+AuthenticatorLen])
			assert.True(t, VerifyResponseAuth(sent, requestAuth, key), "code %d, %s: only the length is wrong", code, name)
			_, decodeErr := Decode(sent)
			assert.Error(t, decodeErr, "code %d, %s", code, name)
			mu.Unlock()
		}
	}
}
