// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS wire format
// Related: packet.go -- verifyResponseMessageAuthenticator
// Related: client.go -- dispatchResponse, the client's only response gate
//
// VALIDATES: that a RADIUS client refuses an Access-Accept whose
// Message-Authenticator does not verify, driven through Client.Exchange rather
// than through the helper alone.
// PREVENTS: the shape found on 2026-08-31 while walking RFC 2869 for
// rfc/extraction/rfc2869.json. dispatchResponse checked only the Response
// Authenticator, so a server-side or on-path party that could produce a valid
// Response Authenticator carried an unverified HMAC past the client, and the
// attribute RFC 2869 Section 5.14 added to catch exactly that was never read.
//
// Both cases build the Message-Authenticator from the RFC's own recipe here,
// never by calling the producer under test, so a formula that drops the
// attribute stream or forgets the Request Authenticator substitution fails.

package radius

import (
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RFC 2869 Section 5.14 mandates HMAC-MD5
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// rfc2869Server answers every datagram with the bytes build returns.
type rfc2869Server struct {
	conn *net.UDPConn
	addr string
	done chan struct{}
}

func newRFC2869Server(t *testing.T, build func(req []byte) []byte) *rfc2869Server {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &rfc2869Server{conn: conn, addr: conn.LocalAddr().String(), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		buf := make([]byte, MaxPacketLen)
		for {
			n, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			if resp := build(buf[:n]); resp != nil {
				conn.WriteToUDP(resp, from) //nolint:errcheck // test mock best-effort
			}
		}
	}()
	t.Cleanup(func() {
		conn.Close() //nolint:errcheck // test cleanup
		<-s.done
	})
	return s
}

// rfc2869AccessAccept builds an Access-Accept carrying a Message-Authenticator
// signed with maSecret and a Response Authenticator signed with respSecret.
//
// RFC 2869 Section 5.14: "For Access-Challenge, Access-Accept, and
// Access-Reject packets, the Message-Authenticator is calculated as follows,
// using the Request-Authenticator from the Access-Request this packet is in
// reply to: Message-Authenticator = HMAC-MD5 (Type, Identifier, Length,
// Request Authenticator, Attributes)".
//
// RFC 2869 Section 5.14: "The is calculated and inserted in the packet before
// the Response Authenticator is calculated." Both signatures are therefore
// applied in that order, so a negative case still presents a Response
// Authenticator the client accepts and the Message-Authenticator is the only
// check that can refuse it.
func rfc2869AccessAccept(req, maSecret, respSecret []byte) []byte {
	if len(req) < MinPacketLen {
		return nil
	}
	var requestAuth [AuthenticatorLen]byte
	copy(requestAuth[:], req[4:4+AuthenticatorLen])

	attrs := []Attr{
		{Type: AttrServiceType, Value: AttrUint32(ServiceTypeFramed)},
		{Type: AttrSessionTimeout, Value: AttrUint32(3600)},
		{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)},
	}
	pkt := &Packet{Code: CodeAccessAccept, Identifier: req[1], Attrs: attrs}
	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		return nil
	}
	wire := buf[:n]

	// Offset of the Message-Authenticator value: it is the last attribute, and
	// the attribute is Type(1) + Length(1) + 16 value octets.
	maOff := n - AuthenticatorLen

	// The Message-Authenticator runs over the packet with the Request
	// Authenticator in the authenticator field and the value zeroed.
	signing := make([]byte, n)
	copy(signing, wire)
	copy(signing[4:4+AuthenticatorLen], requestAuth[:])
	mac := hmac.New(md5.New, maSecret) //nolint:gosec // RFC 2869 Section 5.14 mandates HMAC-MD5
	mac.Write(signing)
	copy(wire[maOff:], mac.Sum(nil))

	// RFC 2865 Section 3: ResponseAuth = MD5(Code+ID+Length+RequestAuth+Attributes+Secret).
	h := md5.New() //nolint:gosec // RFC 2865 Section 3 mandates MD5
	h.Write(wire[:2])
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], uint16(n))
	h.Write(lenBuf[:])
	h.Write(requestAuth[:])
	h.Write(wire[HeaderLen:n])
	h.Write(respSecret)
	copy(wire[4:4+AuthenticatorLen], h.Sum(nil))

	out := make([]byte, n)
	copy(out, wire)
	return out
}

func rfc2869Exchange(t *testing.T, serverAddr string, secret []byte) (*Packet, error) {
	t.Helper()
	client, err := NewClient(ClientConfig{Timeout: 300 * time.Millisecond, Retries: 1})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { closeSilent(client) })

	auth, err := RandomAuthenticator()
	if err != nil {
		t.Fatalf("RandomAuthenticator: %v", err)
	}
	req := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
		Attrs:         []Attr{{Type: AttrUserName, Value: AttrString("carol")}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return client.Exchange(ctx, req, secret, serverAddr)
}

// TestRFC2869AccessAcceptWithValidMessageAuthenticatorIsAccepted drives the
// client's real entry point against a server that signs the reply correctly.
//
// RFC requirement: RFC2869-5.14-1 positive -- an Access-Accept whose
// Message-Authenticator matches the value the client computes is delivered to
// the caller of Client.Exchange (client.go dispatchResponse,
// packet.go verifyResponseMessageAuthenticator).
func TestRFC2869AccessAcceptWithValidMessageAuthenticatorIsAccepted(t *testing.T) {
	secret := []byte("testing123")
	srv := newRFC2869Server(t, func(req []byte) []byte {
		return rfc2869AccessAccept(req, secret, secret)
	})

	resp, err := rfc2869Exchange(t, srv.addr, secret)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if resp.Code != CodeAccessAccept {
		t.Fatalf("Code = %d, want %d", resp.Code, CodeAccessAccept)
	}
	if len(resp.FindAttr(AttrMessageAuthenticator)) != AuthenticatorLen {
		t.Fatal("the accepted reply lost its Message-Authenticator attribute")
	}
}

// TestRFC2869AccessAcceptWithWrongMessageAuthenticatorIsDiscarded presents a
// reply whose Response Authenticator is correct and whose Message-Authenticator
// is signed with a different secret. Only the Message-Authenticator rule can
// refuse it, so a client that does not implement the rule returns the packet.
//
// RFC 2869 Section 5.14: "A RADIUS Client receiving an Access-Accept,
// Access-Reject or Access-Challenge with a Message-Authenticator Attribute
// present MUST calculate the correct value of the Message-Authenticator and
// silently discard the packet if it does not match the value sent."
//
// RFC requirement: RFC2869-5.14-1 negative -- an Access-Accept whose
// Message-Authenticator does not match is discarded, so Client.Exchange
// exhausts its retries and returns an error instead of the packet
// (client.go dispatchResponse).
func TestRFC2869AccessAcceptWithWrongMessageAuthenticatorIsDiscarded(t *testing.T) {
	secret := []byte("testing123")
	srv := newRFC2869Server(t, func(req []byte) []byte {
		return rfc2869AccessAccept(req, []byte("wrong-secret"), secret)
	})

	resp, err := rfc2869Exchange(t, srv.addr, secret)
	if err == nil {
		t.Fatalf("Exchange returned code %d; the reply must be silently discarded", resp.Code)
	}
}
