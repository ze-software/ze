// RFC: rfc/short/rfc2865.md -- the obligations the extraction walk added
// Related: rfc/extraction/rfc2865.json -- the walk that bounds that checklist
// Related: packet.go, client.go, authenticator.go -- the producers under test

// Tests for the RFC 2865 obligations the extraction sign-off found unmapped.
// They are separate from rfc2865_test.go so the walk's additions read as one
// block against the sentences that produced them.
package radius

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
)

// unofferedServiceRequestAttrs are the RFC 2865 attribute types that belong to a
// service ze does not offer: LAT (34, 35, 36, 63), AppleTalk (37, 38, 39),
// IPX (23), callback (19, 20) and character-mode login (14, 15, 16).
var unofferedServiceRequestAttrs = map[uint8]string{
	14: "Login-IP-Host", 15: "Login-Service", 16: "Login-TCP-Port",
	19: "Callback-Number", 20: "Callback-Id", 23: "Framed-IPX-Network",
	34: "Login-LAT-Service", 35: "Login-LAT-Node", 36: "Login-LAT-Group",
	37: "Framed-AppleTalk-Link", 38: "Framed-AppleTalk-Network",
	39: "Framed-AppleTalk-Zone", 63: "Login-LAT-Port",
}

// VALIDATES: the admin Access-Request carries no attribute for a service ze
// does not offer.
// PREVENTS: a NAS advertising LAT, AppleTalk, IPX or callback support it cannot
// deliver, which invites an Access-Accept authorizing exactly that.
// RFC requirement: RFC2865-1.1-1 positive -- "A NAS that does not implement a
// given service MUST NOT implement the RADIUS attributes for that service"
// (Section 1.1).
func TestAdminAccessRequestCarriesNoUnofferedServiceAttribute(t *testing.T) {
	key := []byte("testing123")
	srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)

	req := srv.captured(t)
	for _, attr := range req.Attrs {
		name, unoffered := unofferedServiceRequestAttrs[attr.Type]
		assert.False(t, unoffered, "Access-Request carries %s (type %d)", name, attr.Type)
	}
}

// VALIDATES: an Access-Accept whose Service-Type ze does not offer is a
// rejection, and one it does offer is not.
// PREVENTS: an operator shell handed to a login the server authorized for
// framed data service instead.
// RFC requirement: RFC2865-1.1-2 positive -- Service-Type Login-User names the
// one service the admin backend offers, so the Access-Accept stands.
func TestAdminAccessAcceptWithOfferedServiceTypeIsAccepted(t *testing.T) {
	key := []byte("testing123")
	reply := []Attr{
		{Type: AttrServiceType, Value: AttrUint32(serviceTypeLogin)},
		{Type: AttrFilterID, Value: []byte("netops")},
	}
	srv := newReplyServer(t, key, CodeAccessAccept, reply)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)
	assert.Equal(t, []string{"netops"}, res.Profiles)
}

// VALIDATES: Service-Type Framed in an admin Access-Accept is a rejection.
// PREVENTS: honoring an Access-Accept that authorizes framed subscriber access
// as though it authorized an operator shell.
// RFC requirement: RFC2865-1.1-2 negative -- "A NAS MUST treat a RADIUS
// access-accept authorizing an unavailable service as an access-reject
// instead." Section 5.6 repeats it for an unsupported Service-Type.
func TestAdminAccessAcceptWithUnofferedServiceTypeIsRejected(t *testing.T) {
	key := []byte("testing123")
	reply := []Attr{
		{Type: AttrServiceType, Value: AttrUint32(ServiceTypeFramed)},
		{Type: AttrFilterID, Value: []byte("netops")},
	}
	srv := newReplyServer(t, key, CodeAccessAccept, reply)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	assert.False(t, res.Authenticated)
}

// VALIDATES: octets after the Length field are padding, not attributes.
// PREVENTS: a trailing octet run being read as an attribute, which lets anyone
// who can append to a datagram add attributes the Response Authenticator never
// covered.
// RFC requirement: RFC2865-3-6 positive -- "Octets outside the range of the
// Length field MUST be treated as padding and ignored on reception"
// (Section 3).
func TestDecodeIgnoresOctetsOutsideTheLengthField(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessAccept, 7, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	padded := append(append([]byte{}, pkt...), 0xAA, 0xBB, 0xCC, 0xDD)

	decoded, err := Decode(padded)
	require.NoError(t, err)
	require.Len(t, decoded.Attrs, 1)
	assert.Equal(t, uint8(AttrFilterID), decoded.Attrs[0].Type)
	assert.Equal(t, []byte("netops"), decoded.Attrs[0].Value)
}

// VALIDATES: a well-formed attribute placed in the padding is not decoded.
// PREVENTS: an appended Filter-Id granting a profile the server never sent.
// RFC requirement: RFC2865-3-6 negative -- the padding here is a syntactically
// valid Filter-Id attribute, and it MUST still be ignored.
func TestDecodeIgnoresAnAttributeHiddenInThePadding(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessAccept, 7, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	smuggled := []byte{AttrFilterID, 2 + 5}
	smuggled = append(smuggled, []byte("admin")...)

	decoded, err := Decode(append(append([]byte{}, pkt...), smuggled...))
	require.NoError(t, err)
	require.Len(t, decoded.Attrs, 1, "the appended Filter-Id is outside Length and is padding")
	assert.Equal(t, []byte("netops"), decoded.Attrs[0].Value)
}

// VALIDATES: a datagram shorter than its own Length field is refused.
// PREVENTS: reading past the end of a received datagram, and accepting a packet
// whose declared extent nobody delivered.
// RFC requirement: RFC2865-3-7 positive -- a packet whose Length matches the
// datagram decodes.
func TestDecodeAcceptsAPacketAsLongAsItsLengthField(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessAccept, 7, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	decoded, err := Decode(pkt)
	require.NoError(t, err)
	assert.Equal(t, uint8(CodeAccessAccept), decoded.Code)
}

// VALIDATES: a Length field larger than the datagram is refused.
// PREVENTS: an out-of-range read, and a truncated reply being treated as whole.
// RFC requirement: RFC2865-3-7 negative -- "If the packet is shorter than the
// Length field indicates, it MUST be silently discarded" (Section 3).
func TestDecodeRefusesAPacketShorterThanItsLengthField(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessAccept, 7, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(pkt)+1))

	_, err := Decode(pkt)
	require.Error(t, err)
}

// VALIDATES: an exchange with a configured shared secret proceeds.
// PREVENTS: the empty-secret guard refusing a legitimate exchange.
// RFC requirement: RFC2865-3-8 positive -- a non-empty secret is the case the
// protocol is defined for.
func TestExchangeAcceptsANonEmptySharedSecret(t *testing.T) {
	key := []byte("testing123")
	srv := newMockServer(t, key, CodeAccessAccept)
	defer srv.close()

	client, err := NewClient(ClientConfig{Timeout: 300 * time.Millisecond, Retries: 2})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	resp, err := client.Exchange(context.Background(), accessRequest(t, client.NextID()), key, srv.addr)
	require.NoError(t, err)
	assert.Equal(t, uint8(CodeAccessAccept), resp.Code)
}

// VALIDATES: an exchange with no shared secret is refused before any datagram
// leaves the host.
// PREVENTS: sending a RADIUS packet whose authenticator anybody can compute,
// and trusting a reply signed with a secret everybody knows.
// RFC requirement: RFC2865-3-8 negative -- "The secret MUST NOT be empty
// (length 0) since this would allow packets to be trivially forged"
// (Section 3).
func TestExchangeRefusesAnEmptySharedSecret(t *testing.T) {
	srv := newMockServer(t, []byte(""), CodeAccessAccept)
	defer srv.close()

	client, err := NewClient(ClientConfig{Timeout: 300 * time.Millisecond, Retries: 2})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exchange(context.Background(), accessRequest(t, client.NextID()), nil, srv.addr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty shared secret")
}

// VALIDATES: the packet an authentication produces carries Code 1.
// PREVENTS: authenticating over a code the server answers differently, or not
// at all.
// RFC requirement: RFC2865-4.1-1 positive -- "An implementation wishing to
// authenticate a user MUST transmit a RADIUS packet with the Code field set to
// 1 (Access-Request)" (Section 4.1).
func TestAdminAuthenticationTransmitsAccessRequest(t *testing.T) {
	key := []byte("testing123")
	srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)

	assert.Equal(t, uint8(CodeAccessRequest), srv.captured(t).Code)
}

// VALIDATES: the admin Access-Request identifies the NAS.
// PREVENTS: a server unable to pick a policy for this device, and a request the
// RFC calls incomplete.
// RFC requirement: RFC2865-4.1-2 positive -- "It MUST contain either a
// NAS-IP-Address attribute or a NAS-Identifier attribute (or both)"
// (Section 4.1).
func TestAdminAccessRequestIdentifiesTheNAS(t *testing.T) {
	key := []byte("testing123")
	srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)

	req := srv.captured(t)
	hasIP := req.FindAttr(AttrNASIPAddress) != nil
	hasID := req.FindAttr(AttrNASIdentifier) != nil
	assert.True(t, hasIP || hasID, "neither NAS-IP-Address nor NAS-Identifier is present")
}

// VALIDATES: the admin Access-Request carries a credential.
// PREVENTS: an Access-Request the server can only reject, because nothing in it
// proves who the user is.
// RFC requirement: RFC2865-4.1-3 positive -- "An Access-Request MUST contain
// either a User-Password or a CHAP-Password or a State."
// RFC requirement: RFC2865-4.1-4 positive -- "An Access-Request MUST NOT
// contain both a User-Password and a CHAP-Password" (Section 4.1).
func TestAdminAccessRequestCarriesExactlyOneCredential(t *testing.T) {
	key := []byte("testing123")
	srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)

	req := srv.captured(t)
	assert.NotNil(t, req.FindAttr(AttrUserPassword), "User-Password is the admin path's credential")
	assert.Nil(t, req.FindAttr(AttrCHAPPassword), "a User-Password request carries no CHAP-Password")
}

// VALIDATES: two successive authentications use different Identifiers.
// PREVENTS: a server reading the second request as a duplicate of the first and
// replaying its answer.
// RFC requirement: RFC2865-4.1-5 positive -- "The Identifier field MUST be
// changed whenever the content of the Attributes field changes, and whenever a
// valid reply has been received for a previous request" (Section 4.1).
func TestSuccessiveRequestsUseDifferentIdentifiers(t *testing.T) {
	key := []byte("testing123")
	srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)
	first := srv.captured(t)

	_, err = a.Authenticate(aaa.AuthRequest{Username: "bob", Password: "pw2"})
	require.NoError(t, err)
	second := srv.captured(t)

	assert.NotEqual(t, first.Identifier, second.Identifier)
}

// VALIDATES: a retransmit to the same server keeps its Identifier.
// PREVENTS: reading "change the Identifier" as "change it on every send", which
// would stop the server detecting a duplicate.
// RFC requirement: RFC2865-4.1-5 negative -- Section 4.1 continues "For
// retransmissions, the Identifier MUST remain unchanged" (Section 4.1).
func TestRetransmitToTheSameServerKeepsItsIdentifier(t *testing.T) {
	key := []byte("testing123")
	srv := newSilentThenReplyServer(t, key)
	defer srv.close()

	client, err := NewClient(ClientConfig{Timeout: 150 * time.Millisecond, Retries: 3})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exchange(context.Background(), accessRequest(t, client.NextID()), key, srv.addr)
	require.NoError(t, err)

	seen := srv.identifiers()
	require.GreaterOrEqual(t, len(seen), 2, "the first request must have gone unanswered and been resent")
	for _, id := range seen[1:] {
		assert.Equal(t, seen[0], id, "a retransmit reuses the Identifier")
	}
}

// VALIDATES: failover to the next server takes a new Identifier and a new
// Request Authenticator together.
// PREVENTS: repeating a Request Authenticator under a secret two servers share,
// which is the replay window Section 3 names: "repetition of a request value in
// conjunction with the same secret would permit an attacker to reply with a
// previously intercepted response".
// RFC requirement: RFC2865-4.1-6 positive -- "The Request Authenticator value
// MUST be changed each time a new Identifier is used" (Section 4.1).
func TestFailoverChangesTheRequestAuthenticatorWithTheIdentifier(t *testing.T) {
	key := []byte("testing123")
	dead, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer func() { _ = dead.Close() }()

	live := newRequestCaptureServer(t, key, nil)
	defer live.close()

	client, err := NewClient(ClientConfig{
		Servers: []Server{
			{Address: dead.LocalAddr().String(), SharedKey: key},
			{Address: live.addr, SharedKey: key},
		},
		Timeout: 100 * time.Millisecond,
		Retries: 1,
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	pkt := accessRequest(t, 0)
	firstAuth := pkt.Authenticator
	firstID := pkt.Identifier

	_, err = client.SendToServers(context.Background(), pkt)
	require.NoError(t, err)

	reached := live.captured(t)
	assert.NotEqual(t, firstID, reached.Identifier, "failover takes a new Identifier")
	assert.NotEqual(t, firstAuth, reached.Authenticator, "and a new Request Authenticator with it")
}

// VALIDATES: a retransmit to the same server keeps its Request Authenticator.
// PREVENTS: a re-randomized authenticator on every send, which would break the
// duplicate detection Section 2.5 relies on.
// RFC requirement: RFC2865-4.1-6 negative -- the obligation is bound to a NEW
// Identifier, so a send that reuses the Identifier reuses the authenticator.
func TestRetransmitToTheSameServerKeepsItsRequestAuthenticator(t *testing.T) {
	key := []byte("testing123")
	srv := newSilentThenReplyServer(t, key)
	defer srv.close()

	client, err := NewClient(ClientConfig{Timeout: 150 * time.Millisecond, Retries: 3})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exchange(context.Background(), accessRequest(t, client.NextID()), key, srv.addr)
	require.NoError(t, err)

	seen := srv.authenticators()
	require.GreaterOrEqual(t, len(seen), 2)
	for _, auth := range seen[1:] {
		assert.Equal(t, seen[0], auth, "a retransmit reuses the Request Authenticator")
	}
}

// VALIDATES: an Access-Challenge stops the login.
// PREVENTS: a server asking for a second factor being answered by ze consulting
// the next backend instead, which is the same shape as the RFC 8907 defect of
// 2026-08-30: an answer that proves nothing was read as permission to look
// elsewhere.
// RFC requirement: RFC2865-4.4-1 positive -- "If the NAS does not support
// challenge/response, it MUST treat an Access-Challenge as though it had
// received an Access-Reject instead" (Section 4.4).
func TestAccessChallengeIsTreatedAsAccessReject(t *testing.T) {
	key := []byte("testing123")
	srv := newReplyServer(t, key, CodeAccessChallenge, []Attr{{Type: 24, Value: []byte("state-cookie")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	assert.False(t, res.Authenticated)
}

// alwaysAcceptBackend stands in for a backend later in the AAA chain.
type alwaysAcceptBackend struct{ reached bool }

func (b *alwaysAcceptBackend) Authenticate(aaa.AuthRequest) (aaa.AuthResult, error) {
	b.reached = true
	return aaa.AuthResult{Authenticated: true, Source: "local", Profiles: []string{"admin"}}, nil
}

// VALIDATES: an Access-Challenge does not hand the login to the next backend.
// PREVENTS: the fall-through this rule exists to stop. Treating the challenge as
// an infra error would let a local password shadow the server's verdict.
// RFC requirement: RFC2865-4.4-1 negative -- an Access-Reject stops the chain
// (ChainAuthenticator, internal/component/aaa/types.go), so a challenge treated
// "as though it had received an Access-Reject" must stop it too.
func TestAccessChallengeDoesNotFallThroughToTheNextBackend(t *testing.T) {
	key := []byte("testing123")
	srv := newReplyServer(t, key, CodeAccessChallenge, nil)
	defer srv.close()

	radiusBackend := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	local := &alwaysAcceptBackend{}
	chain := &aaa.ChainAuthenticator{Backends: []aaa.Authenticator{radiusBackend, local}}

	res, err := chain.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	assert.False(t, res.Authenticated)
	assert.False(t, local.reached, "the chain must not reach the local backend")
}

// VALIDATES: attribute lookup does not depend on the order of different types.
// PREVENTS: a reply read differently because the server emitted its attributes
// in another order, which no RFC lets ze require.
// RFC requirement: RFC2865-5-4 positive -- "A RADIUS server or client MUST NOT
// have any dependencies on the order of attributes of different types"
// (Section 5).
func TestAttributeLookupIgnoresTheOrderOfDifferentTypes(t *testing.T) {
	forward := []Attr{
		{Type: AttrServiceType, Value: AttrUint32(serviceTypeLogin)},
		{Type: AttrFilterID, Value: []byte("netops")},
		{Type: AttrReplyMessage, Value: []byte("welcome")},
	}
	reverse := []Attr{forward[2], forward[1], forward[0]}

	assert.Equal(t, profilesFromReply(t, forward), profilesFromReply(t, reverse))
}

// VALIDATES: attributes of the same type are collected wherever they sit.
// PREVENTS: dropping a Filter-Id because another attribute type separates it
// from its sibling, which would silently shrink a user's profile set.
// RFC requirement: RFC2865-5-5 positive -- "A RADIUS server or client MUST NOT
// require attributes of the same type to be contiguous" (Section 5).
func TestAttributeLookupDoesNotRequireContiguity(t *testing.T) {
	scattered := []Attr{
		{Type: AttrFilterID, Value: []byte("netops")},
		{Type: AttrReplyMessage, Value: []byte("welcome")},
		{Type: AttrFilterID, Value: []byte("read-only")},
		{Type: AttrServiceType, Value: AttrUint32(serviceTypeLogin)},
		{Type: AttrFilterID, Value: []byte("audit")},
	}
	assert.Equal(t, []string{"netops", "read-only", "audit"}, profilesFromReply(t, scattered))
}

// VALIDATES: a response whose attributes all have valid lengths is delivered.
// PREVENTS: the invalid-length guard refusing a well-formed reply.
// RFC requirement: RFC2865-5-6 positive -- the packet is not one Section 5 asks
// ze to discard.
func TestResponseWithValidAttributeLengthsIsDelivered(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessAccept, 9, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	decoded, err := Decode(pkt)
	require.NoError(t, err)
	assert.Len(t, decoded.Attrs, 1)
}

// VALIDATES: a response carrying an attribute with an invalid length is
// discarded rather than partly read.
// PREVENTS: acting on the attributes that happened to parse before the bad one,
// which is a reply the server never sent.
// RFC requirement: RFC2865-5-6 negative -- "If an Attribute is received in an
// Access-Accept, Access-Reject or Access-Challenge packet with an invalid
// length, the packet MUST either be treated as an Access-Reject or else
// silently discarded" (Section 5).
func TestResponseWithAnInvalidAttributeLengthIsDiscarded(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessAccept, 9, []Attr{{Type: AttrFilterID, Value: []byte("netops")}})
	// An attribute Length of 1 is below the two-octet header and cannot be
	// walked past, so the whole packet is refused.
	pkt[HeaderLen+1] = 1

	_, err := Decode(pkt)
	require.Error(t, err)
}

// VALIDATES: an attribute value carrying a null octet round-trips whole.
// PREVENTS: truncating a value at its first null, which loses a byte of a
// password, a Class token or a state cookie.
// RFC requirement: RFC2865-5-7 positive -- "Servers and servers and clients MUST
// be able to deal with embedded nulls" (Section 5).
func TestAttributeValueWithAnEmbeddedNullRoundTrips(t *testing.T) {
	value := []byte("ze\x00radius")
	pkt := encodedPacket(t, CodeAccessAccept, 3, []Attr{{Type: AttrFilterID, Value: value}})

	decoded, err := Decode(pkt)
	require.NoError(t, err)
	require.Len(t, decoded.Attrs, 1)
	assert.Equal(t, value, decoded.Attrs[0].Value)
}

// VALIDATES: an attribute after one containing a null is still decoded.
// PREVENTS: a null ending the attribute walk, which would drop every attribute
// behind it without a word.
// RFC requirement: RFC2865-5-7 negative -- a decoder that stopped at the null
// would return one attribute here instead of two.
func TestAnEmbeddedNullDoesNotEndTheAttributeWalk(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessAccept, 3, []Attr{
		{Type: AttrFilterID, Value: []byte("ze\x00radius")},
		{Type: AttrReplyMessage, Value: []byte("welcome")},
	})

	decoded, err := Decode(pkt)
	require.NoError(t, err)
	require.Len(t, decoded.Attrs, 2)
	assert.Equal(t, []byte("welcome"), decoded.Attrs[1].Value)
}

// VALIDATES: a zero-length attribute value is omitted from the wire.
// PREVENTS: sending an attribute the RFC forbids, which a strict server answers
// with an Access-Reject.
// RFC requirement: RFC2865-5-8 positive -- "Text of length zero (0) MUST NOT be
// sent; omit the entire attribute instead" (Section 5).
func TestZeroLengthAttributeIsOmitted(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessRequest, 4, []Attr{
		{Type: AttrUserName, Value: []byte("alice")},
		{Type: AttrFilterID, Value: []byte{}},
	})

	decoded, err := Decode(pkt)
	require.NoError(t, err)
	require.Len(t, decoded.Attrs, 1)
	assert.Equal(t, uint8(AttrUserName), decoded.Attrs[0].Type)
}

// VALIDATES: a one-octet value is still sent, so the omission is driven by the
// zero length alone.
// PREVENTS: an over-broad guard that drops short attributes generally.
// RFC requirement: RFC2865-5-8 negative -- the rule names length zero, and one
// octet is a legal text or string value.
func TestOneOctetAttributeIsNotOmitted(t *testing.T) {
	pkt := encodedPacket(t, CodeAccessRequest, 4, []Attr{
		{Type: AttrUserName, Value: []byte("alice")},
		{Type: AttrFilterID, Value: []byte("x")},
	})

	decoded, err := Decode(pkt)
	require.NoError(t, err)
	require.Len(t, decoded.Attrs, 2)
	assert.Equal(t, []byte("x"), decoded.Attrs[1].Value)
}

// VALIDATES: human-readable and opaque carrier attributes do not change what
// the protocol does with a packet.
// PREVENTS: a Reply-Message, a Framed-Route or a vendor attribute deciding an
// authentication outcome.
// RFC requirement: RFC2865-5.11-1 positive -- Sections 5.11, 5.18, 5.22, 5.26
// and 5.33 each say the attribute "MUST NOT affect operation of the protocol".
// An Access-Accept carrying all of them is still an accept, with the profiles
// its Filter-Id names.
func TestCarrierAttributesDoNotAffectAnAccessAccept(t *testing.T) {
	reply := []Attr{
		{Type: AttrReplyMessage, Value: []byte("welcome")},
		{Type: AttrFramedRoute, Value: []byte("10.0.0.0/8 0.0.0.0 1")},
		{Type: AttrVendorSpecific, Value: []byte{0, 0, 1, 55, 1, 6, 'a', 'b', 'c', 'd'}},
		{Type: AttrFilterID, Value: []byte("netops")},
	}
	assert.Equal(t, []string{"netops"}, profilesFromReply(t, reply))
}

// VALIDATES: the same carrier attributes do not turn an Access-Reject into an
// accept.
// PREVENTS: a Reply-Message on a rejection being read as a reason to continue.
// RFC requirement: RFC2865-5.11-1 negative -- the Code decides the outcome, and
// no carrier attribute may override it.
func TestCarrierAttributesDoNotAffectAnAccessReject(t *testing.T) {
	key := []byte("testing123")
	reply := []Attr{
		{Type: AttrReplyMessage, Value: []byte("try again")},
		{Type: AttrFilterID, Value: []byte("netops")},
	}
	srv := newReplyServer(t, key, CodeAccessReject, reply)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	assert.False(t, res.Authenticated)
}

// VALIDATES: a State attribute in an Access-Accept names no ze profile.
// PREVENTS: reading an authorization decision out of an attribute the RFC
// reserves as an opaque server cookie.
// RFC requirement: RFC2865-5.25-1 positive -- Section 5.24 says of State, and
// Section 5.25 of Class, that "the client MUST NOT interpret the attribute
// locally". Neither is a profile carrier, so this Accept resolves to nothing and
// the login is rejected.
func TestStateIsNotInterpretedLocally(t *testing.T) {
	key := []byte("testing123")
	srv := newReplyServer(t, key, CodeAccessAccept, []Attr{{Type: 24, Value: []byte("admins")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	assert.Empty(t, res.Profiles)
}

// --- helpers ---

// accessRequest builds a minimal Access-Request with a random authenticator.
func accessRequest(t *testing.T, id uint8) *Packet {
	t.Helper()
	auth, err := RandomAuthenticator()
	require.NoError(t, err)
	return &Packet{
		Code:          CodeAccessRequest,
		Identifier:    id,
		Authenticator: auth,
		Attrs:         []Attr{{Type: AttrUserName, Value: AttrString("alice")}},
	}
}

// encodedPacket encodes a packet and returns the wire bytes.
func encodedPacket(t *testing.T, code, id uint8, attrs []Attr) []byte {
	t.Helper()
	auth, err := RandomAuthenticator()
	require.NoError(t, err)
	pkt := &Packet{Code: code, Identifier: id, Authenticator: auth, Attrs: attrs}
	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	require.NoError(t, err)
	return append([]byte{}, buf[:n]...)
}

// profilesFromReply drives one admin authentication against a server that
// answers with the given Access-Accept attributes, and returns the profiles.
func profilesFromReply(t *testing.T, reply []Attr) []string {
	t.Helper()
	key := []byte("testing123")
	srv := newReplyServer(t, key, CodeAccessAccept, reply)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)
	return res.Profiles
}

// requestCaptureServer answers like newReplyServer and keeps every request it saw.
type requestCaptureServer struct {
	*mockRADIUSServer
	requests chan []byte
}

func newRequestCaptureServer(t *testing.T, sharedKey []byte, reply []Attr) *requestCaptureServer {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	c := &requestCaptureServer{
		mockRADIUSServer: &mockRADIUSServer{conn: conn, addr: conn.LocalAddr().String(), done: make(chan struct{})},
		requests:         make(chan []byte, 16),
	}
	c.handler = func(req []byte) []byte {
		c.requests <- append([]byte{}, req...)
		return buildReplyResponse(CodeAccessAccept, req, sharedKey, reply)
	}
	go c.serve()
	return c
}

// captured returns the next request the server received, decoded.
func (c *requestCaptureServer) captured(t *testing.T) *Packet {
	t.Helper()
	select {
	case raw := <-c.requests:
		pkt, err := Decode(raw)
		require.NoError(t, err)
		return pkt
	case <-time.After(2 * time.Second):
		t.Fatal("no request reached the server")
		return nil
	}
}

// silentThenReplyServer drops the first request and answers every later one, so
// the client is forced to retransmit to the same address.
type silentThenReplyServer struct {
	*mockRADIUSServer
	seen chan []byte
}

func newSilentThenReplyServer(t *testing.T, sharedKey []byte) *silentThenReplyServer {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	s := &silentThenReplyServer{
		mockRADIUSServer: &mockRADIUSServer{conn: conn, addr: conn.LocalAddr().String(), done: make(chan struct{})},
		seen:             make(chan []byte, 16),
	}
	first := true
	s.handler = func(req []byte) []byte {
		s.seen <- append([]byte{}, req...)
		if first {
			first = false
			return nil
		}
		return buildResponse(CodeAccessAccept, req, sharedKey)
	}
	go s.serve()
	return s
}

// requests drains every request the server saw.
func (s *silentThenReplyServer) requests() [][]byte {
	var out [][]byte
	for {
		select {
		case raw := <-s.seen:
			out = append(out, raw)
		default:
			return out
		}
	}
}

// identifiers returns the Identifier octet of every request seen, in order.
func (s *silentThenReplyServer) identifiers() []uint8 {
	var out []uint8
	for _, raw := range s.requests() {
		out = append(out, raw[1])
	}
	return out
}

// authenticators returns the Request Authenticator of every request seen.
func (s *silentThenReplyServer) authenticators() [][AuthenticatorLen]byte {
	var out [][AuthenticatorLen]byte
	for _, raw := range s.requests() {
		var auth [AuthenticatorLen]byte
		copy(auth[:], raw[4:4+AuthenticatorLen])
		out = append(out, auth)
	}
	return out
}
