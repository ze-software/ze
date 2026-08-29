// RFC 2865 (RADIUS) NAS/client behavioral requirements not already pinned by the
// packet/attr/client round-trip tests.
//
// VALIDATES: RFC 2865 client obligations -- the Request Authenticator is 16 random
// octets (Section 3), an attribute Length never exceeds 255 octets (Section 5), a
// response is matched only to the server address it was sent to (Section 3), the
// User-Password ciphertext depends on the shared secret (Section 5.2), an over-long
// User-Password clamps to 128 octets (Section 5.2), and every Access-Request the
// admin authenticator builds carries a User-Name attribute (Section 5).
// PREVENTS: a constant/short Request Authenticator, an over-long attribute escaping the
// 255-octet limit, a spoofed-source response being accepted, a User-Password encoded
// independently of the secret, an unclamped password, and an Access-Request missing
// User-Name.

package radius

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
)

// TestRFC2865RequestAuthenticatorRandom pins the width and the source of the Request
// Authenticator.
//
// No unit test can prove that 16 octets are random; a counter passes every statistical
// check a single sample allows. What a test CAN prove is structural, and it is the
// property RFC 2865 Section 3 actually depends on: the octets come from the operating
// system's cryptographic generator rather than from any other source. So the second
// half substitutes crypto/rand.Reader and requires the authenticator to be exactly what
// that reader produced. Any other source, a counter included, fails it.
func TestRFC2865RequestAuthenticatorRandom(t *testing.T) {
	// RFC requirement: RFC2865-3-2 positive -- the Request Authenticator is 16 octets drawn
	// from crypto/rand, and two successive values differ (not a fixed constant).
	a1, err := RandomAuthenticator()
	require.NoError(t, err)
	if len(a1) != AuthenticatorLen {
		t.Fatalf("authenticator width: got %d, want %d", len(a1), AuthenticatorLen)
	}
	a2, err := RandomAuthenticator()
	require.NoError(t, err)
	if a1 == a2 {
		t.Fatal("two Request Authenticators must differ (crypto/rand, not a constant)")
	}

	// RFC requirement: RFC2865-3-2 positive -- the 16 octets are read from the crypto/rand
	// reader itself: with that reader substituted, the authenticator is byte for byte what
	// it produced, so no other generator can be supplying the value (RandomAuthenticator,
	// packet.go, rand.Read over crypto/rand.Reader).
	saved := rand.Reader
	t.Cleanup(func() { rand.Reader = saved })

	var marker [AuthenticatorLen]byte
	for i := range marker {
		marker[i] = byte(0xa0 + i)
	}
	rand.Reader = bytes.NewReader(marker[:])

	a3, err := RandomAuthenticator()
	require.NoError(t, err)
	if a3 != marker {
		t.Fatalf("Request Authenticator = % x, want % x (the octets crypto/rand.Reader supplied)", a3, marker)
	}
}

func TestRFC2865AttributeLengthBound(t *testing.T) {
	auth, err := RandomAuthenticator()
	require.NoError(t, err)
	buf := make([]byte, MaxPacketLen)

	// RFC requirement: RFC2865-5-2 positive -- a 253-octet value (Type+Length+Value == 255,
	// the maximum) encodes and round-trips.
	okPkt := &Packet{
		Code: CodeAccessRequest, Identifier: 1, Authenticator: auth,
		Attrs: []Attr{{Type: AttrReplyMessage, Value: make([]byte, 253)}},
	}
	n, err := okPkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatalf("a 253-octet value must encode (attribute length 255): %v", err)
	}
	decoded, err := Decode(buf[:n])
	require.NoError(t, err)
	if got := len(decoded.FindAttr(AttrReplyMessage)); got != 253 {
		t.Fatalf("round-tripped value length: got %d, want 253", got)
	}

	// RFC requirement: RFC2865-5-2 negative -- a 254-octet value (Type+Length+Value == 256)
	// exceeds the 255-octet attribute limit and is rejected by the encoder.
	badPkt := &Packet{
		Code: CodeAccessRequest, Identifier: 1, Authenticator: auth,
		Attrs: []Attr{{Type: AttrReplyMessage, Value: make([]byte, 254)}},
	}
	if _, err := badPkt.EncodeTo(buf, 0); err == nil {
		t.Fatal("a 254-octet value must be rejected (attribute length would exceed 255)")
	}
}

func TestRFC2865ResponseSourceAddress(t *testing.T) {
	client, err := NewClient(ClientConfig{})
	require.NoError(t, err)
	defer closeSilent(client)

	secret := []byte("s3cr3t")
	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], "0123456789abcdef")

	const id uint8 = 7
	key := responseKey{server: "192.0.2.10:1812", id: id}
	waiter, err := client.registerWaiter(key, reqAuth, secret)
	require.NoError(t, err)
	defer client.unregisterWaiter(key, waiter)

	// A header-only Access-Accept signed for this request authenticator and secret, so the
	// authenticator check inside dispatchResponse passes and only the source-address keying
	// decides whether the reply is matched.
	resp := make([]byte, HeaderLen)
	resp[0] = CodeAccessAccept
	resp[1] = id
	binary.BigEndian.PutUint16(resp[2:4], HeaderLen)
	respAuth := ResponseAuthenticator(CodeAccessAccept, id, HeaderLen, reqAuth, nil, secret)
	copy(resp[4:4+AuthenticatorLen], respAuth[:])

	// RFC requirement: RFC2865-3-5 negative -- a response arriving from a source address other
	// than the one the request was sent to has no matching waiter and is not delivered.
	client.dispatchResponse(responseKey{server: "192.0.2.99:1812", id: id}, resp)
	select {
	case <-waiter.ch:
		t.Fatal("a response from a non-matching source address must not be accepted")
	default:
	}

	// RFC requirement: RFC2865-3-5 positive -- a response arriving from the server address the
	// request was sent to matches its waiter and is delivered.
	client.dispatchResponse(key, resp)
	select {
	case <-waiter.ch:
	default:
		t.Fatal("a response from the correct source address must be delivered")
	}
}

func TestRFC2865UserPasswordDependsOnSecret(t *testing.T) {
	var auth [AuthenticatorLen]byte
	copy(auth[:], "0123456789abcdef")
	password := []byte("hunter2!")

	c1 := EncodeUserPassword(password, []byte("secret-A"), auth)
	c2 := EncodeUserPassword(password, []byte("secret-B"), auth)
	// RFC requirement: RFC2865-5.2-1 negative -- the ciphertext is a function of the shared
	// secret through MD5(S+RA); the same password under a different secret yields a different
	// ciphertext, so the encoding is not a fixed pad independent of the secret.
	if bytes.Equal(c1, c2) {
		t.Fatal("same password under different secrets must produce different ciphertext")
	}

	// RFC requirement: RFC2865-5.2-1 negative -- MD5(S+RA) also folds in the Request
	// Authenticator; a different authenticator yields a different ciphertext.
	c3 := EncodeUserPassword(password, []byte("secret-A"), [AuthenticatorLen]byte{})
	if bytes.Equal(c1, c3) {
		t.Fatal("same password under different authenticators must produce different ciphertext")
	}
}

func TestRFC2865UserPasswordClamp(t *testing.T) {
	var auth [AuthenticatorLen]byte
	copy(auth[:], "0123456789abcdef")

	// RFC requirement: RFC2865-5.2-2 positive -- a password longer than 128 octets is clamped
	// to the 128-octet maximum, which is still a multiple of 16.
	encoded := EncodeUserPassword(make([]byte, 130), []byte("secret"), auth)
	if len(encoded) != 128 {
		t.Fatalf("a 130-octet password must clamp to 128 octets, got %d", len(encoded))
	}
	if len(encoded)%16 != 0 {
		t.Fatalf("clamped length %d must remain a multiple of 16", len(encoded))
	}
}

func TestRFC2865AccessRequestUserName(t *testing.T) {
	key := []byte("testing123")
	reqCh := make(chan []byte, 1)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	addr := conn.LocalAddr().String()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, MaxPacketLen)
		n, from, readErr := conn.ReadFromUDP(buf)
		if readErr != nil {
			return
		}
		cp := make([]byte, n)
		copy(cp, buf[:n])
		select {
		case reqCh <- cp:
		default:
		}
		resp := buildReplyResponse(CodeAccessAccept, buf[:n], key,
			[]Attr{{Type: AttrFilterID, Value: []byte("operator")}})
		conn.WriteToUDP(resp, from) //nolint:errcheck // test mock best-effort
	}()
	defer func() {
		closeSilent(conn)
		<-done
	}()

	a := testAuthenticator(t, addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	_, err = a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)

	// RFC requirement: RFC2865-5-1 positive -- the Access-Request the admin authenticator
	// builds and sends carries a User-Name attribute (Section 5, required in Access-Request).
	select {
	case raw := <-reqCh:
		pkt, decErr := Decode(raw)
		require.NoError(t, decErr)
		assert.Equal(t, uint8(CodeAccessRequest), pkt.Code)
		assert.Equal(t, "alice", string(pkt.FindAttr(AttrUserName)),
			"an Access-Request MUST carry a User-Name attribute")
	default:
		t.Fatal("no Access-Request was captured by the server")
	}
}
