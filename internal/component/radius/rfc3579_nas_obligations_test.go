// Related: authenticator_eap.go -- authenticateEAP and processEAPMessage, the
//   loop these obligations bind
// Related: authenticator.go -- exchange, which names the NAS on every request
// RFC: rfc/short/rfc3579.md -- Sections 1.2, 2.1, 2.2, 2.6.4, 3, 4.3.6
//
// VALIDATES: the RFC 3579 obligations ze meets by RUNNING the conversation, as
// opposed to by framing one attribute. Each was implemented by
// plan/spec-radius-admin-eap.md and left unproven: the ledger recorded them as
// gaps whose stated reason had become false.
// PREVENTS: a ledger that understates ze's conformance, and the four regressions
// each pair pins -- an identity dropped after the first round, an EAP header
// forwarded without validation, a reply code read before its attributes, and a
// login that answers a rejection by trying a weaker credential.

package radius

import (
	"bytes"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/eap"
)

// eapAuthenticatorWithLog is eapAuthenticator with the authenticator's log
// captured, for the two obligations whose only observable is what ze recorded
// about a packet it processed and then did not act on.
func eapAuthenticatorWithLog(t *testing.T, addr string, secret []byte, method AuthMethod) (*radiusAuthenticator, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	client, err := NewClient(ClientConfig{
		Servers: []Server{{Address: addr, SharedKey: secret}},
		Timeout: 300 * time.Millisecond,
		Retries: 2,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	cfg := ExtractedConfig{
		ProfileAttr: AttrFilterID,
		AuthMethod:  method,
		Servers:     []Server{{Address: addr, SharedKey: secret}},
	}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return newRadiusAuthenticator(client, cfg, "ze-test", logger), buf
}

// eapPacketOf decodes the EAP packet one captured Access-Request encapsulates.
func eapPacketOf(t *testing.T, pkt *Packet) *eap.Packet {
	t.Helper()
	encoded, err := eapPacketFrom(pkt)
	require.NoError(t, err)
	require.NotNil(t, encoded, "an EAP Access-Request always carries an EAP-Message")
	decoded, err := eap.DecodePacket(encoded)
	require.NoError(t, err)
	return decoded
}

// TestRadiusAdminEapUserNameIsThePeerIdentity reads the first Access-Request as
// a non-EAP-aware RADIUS proxy would: it parses nothing inside the EAP-Message
// and takes the identity from User-Name alone.
//
// VALIDATES: the User-Name ze sends is the Type-Data the PEER put in its own
// EAP-Response/Identity, read back out of the packet on the wire rather than
// out of the login form.
// PREVENTS: a User-Name and an encapsulated identity that disagree, which is
// invisible to ze and fatal to a proxy that can only read the attribute.
//
// RFC requirement: RFC3579-2.1-3 positive -- "the NAS MUST copy the contents of
// the Type-Data field of the EAP-Response/Identity received from the peer into
// the User-Name attribute" (authenticator_eap.go authenticateEAP, which assigns
// username from identity.Response.TypeData).
func TestRadiusAdminEapUserNameIsThePeerIdentity(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	captured := srv.captured(t)
	require.NotEmpty(t, captured)

	first := eapPacketOf(t, captured[0])
	require.Equal(t, eap.CodeResponse, first.Code, "the first EAP packet is a Response")
	require.Equal(t, eap.TypeIdentity, first.Type, "and its Type is Identity")
	require.NotEmpty(t, first.TypeData, "the Identity Response carries the identity")

	assert.Equal(t, string(first.TypeData), string(captured[0].FindAttr(AttrUserName)),
		"User-Name is the Type-Data of the EAP-Response/Identity, byte for byte")
}

// TestRadiusAdminEapUserNameRidesEverySubsequentRequest is the half of the rule
// that a first-request-only implementation passes and then breaks.
//
// VALIDATES: every Access-Request after the identity round carries the same
// User-Name, although the EAP packet it encapsulates is a method Response with
// no identity in it for the attribute to be derived from.
// PREVENTS: a loop that sets User-Name from whatever the current EAP packet
// happens to hold, which leaves rounds two and later with no identity at all.
//
// RFC requirement: RFC3579-2.1-3 negative -- "MUST include the Type-Data field
// of the EAP-Response/Identity in the User-Name attribute in every subsequent
// Access-Request"; the subsequent requests carry no EAP-Response/Identity and
// still carry that identity (authenticator_eap.go authenticateEAP, which passes
// the same username to every a.exchange call).
func TestRadiusAdminEapUserNameRidesEverySubsequentRequest(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	captured := srv.captured(t)
	require.Greater(t, len(captured), 1,
		"the rule is about SUBSEQUENT requests, so the exchange must have more than one")

	identity := string(captured[0].FindAttr(AttrUserName))
	require.NotEmpty(t, identity)

	for index, pkt := range captured[1:] {
		encapsulated := eapPacketOf(t, pkt)
		assert.NotEqualf(t, eap.TypeIdentity, encapsulated.Type,
			"request %d encapsulates a method Response, not another identity", index+1)
		assert.Equalf(t, identity, string(pkt.FindAttr(AttrUserName)),
			"request %d carries the identity from the first round", index+1)
	}
}

// TestRadiusAdminEapReadsTheServerEAPHeaderFields is the conforming side of the
// header validation: the three fields are read, and what ze answers depends on
// each of them.
//
// VALIDATES: the Identifier of ze's EAP-Response is the Identifier of the
// server's EAP-Request, and the MD5-Challenge hash ze returns is computed over
// the Type-Data that the Request's Length field delimited -- which the server
// proves by accepting it.
// PREVENTS: a carrier that reads the encapsulated octets without honoring the
// header, which produces a hash over the wrong bytes and an Identifier the
// server cannot match to its outstanding Request.
//
// RFC requirement: RFC3579-2.2-1 positive -- "the NAS MUST validate the EAP
// header fields (Code, Identifier, Length) prior to forwarding an EAP packet to
// or from the RADIUS server"; eap.DecodePacket performs it and the fields it
// returns are what the peer answers (authenticator_eap.go processEAPMessage).
func TestRadiusAdminEapReadsTheServerEAPHeaderFields(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated,
		"the server accepted the MD5 hash, so the Type-Data its Length delimited is what ze hashed")

	captured := srv.captured(t)
	require.Len(t, captured, 2, "identity, then the MD5-Challenge response")

	answer := eapPacketOf(t, captured[1])
	assert.Equal(t, eap.CodeResponse, answer.Code, "ze answers a Request with a Response")
	assert.Equal(t, eap.TypeMD5Challenge, answer.Type)
	// The server's Request carried Identifier 0 for the identity round and drew
	// its own for the challenge; the peer echoes whichever it received. A carrier
	// that invented one would not match the server's outstanding Request.
	identity := eapPacketOf(t, captured[0])
	assert.NotEqual(t, identity.Identifier, answer.Identifier,
		"the challenge carried its own Identifier and the Response echoes that one")
}

// TestRadiusAdminEapRefusesAMalformedServerEAPHeader is the fail-closed case.
//
// VALIDATES: an Access-Challenge whose encapsulated EAP header declares a Length
// past the octets the attributes carried is refused, and the peer never sees it.
// The login ends as an infrastructure error, so the chain tries the next
// backend.
// PREVENTS: forwarding an unvalidated EAP packet into the peer, where a Length
// past the buffer is the first thing an attacker reaches for.
//
// RFC requirement: RFC3579-2.2-1 negative -- the same sentence; a header that
// fails the validation is not forwarded (authenticator_eap.go processEAPMessage,
// which returns the eap.DecodePacket error before session.Process runs).
func TestRadiusAdminEapRefusesAMalformedServerEAPHeader(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello", nil)
	srv.mu.Lock()
	srv.corruptEAPFrom = 1 // the identity round answers honestly; the challenge is malformed
	srv.mu.Unlock()

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

	require.Error(t, err)
	assert.ErrorContains(t, err, "EAP header from the server",
		"the refusal names the header validation, not a later failure")
	assert.NotErrorIs(t, err, aaa.ErrAuthRejected,
		"a malformed header is an infrastructure failure, so the chain continues")
	assert.False(t, res.Authenticated)
}

// TestRadiusAdminEapProcessesTheEAPMessageBeforeTheCode drives the ordering with
// the one packet a concluding EAP-Message can carry that the peer always reports
// having read.
//
// A well-formed EAP-Failure is NOT that packet, although it is the obvious
// choice. MD5-Challenge has one round and no mutual authentication, so
// handleMD5Challenge (internal/core/eap/peer.go) reaches peerStateMethodDone the
// moment the peer answers, and RFC 3748 Section 4.2 then makes
// (*PeerSession).Process DISCARD the concluding Failure silently. A correct
// implementation and one that never looked at the attribute are
// indistinguishable under it. A Notification Request is reported whatever the
// method and whatever the peer state.
//
// VALIDATES: an Access-Accept carrying an EAP Notification is handed to the peer
// FIRST -- its message reaches the operator's log -- and the RADIUS code then
// decides the login.
// PREVENTS: a loop that switches on resp.Code first and returns, leaving the
// EAP-Message of a concluding packet read by nothing.
//
// RFC requirement: RFC3579-2.6.4-1 positive -- "the NAS MUST first process the
// attributes, including the EAP-Message attribute(s), prior to processing the
// Accept/Reject indication" (authenticator_eap.go authenticateEAP, which calls
// processEAPMessage before it reads resp.Code).
func TestRadiusAdminEapProcessesTheEAPMessageBeforeTheCode(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	srv.mu.Lock()
	srv.concludeWithNotification = true
	srv.mu.Unlock()

	a, logged := eapAuthenticatorWithLog(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

	require.NoError(t, err)
	assert.True(t, res.Authenticated, "the RADIUS code decides")
	assert.Contains(t, logged.String(), "your session ends at 18:00",
		"the concluding EAP-Message was processed before the reply code was read")
}

// TestRadiusAdminEapProcessesAnUnparseableEAPMessageBeforeTheCode is the same
// ordering under an attribute that cannot be parsed at all.
//
// VALIDATES: an Access-Accept whose EAP-Message carries a malformed EAP header
// is still processed before the code is read -- the decode error is recorded --
// and the login still authorizes on the RADIUS code.
// PREVENTS: an implementation that skips the attribute processing whenever it
// would fail, which passes the test above and still reads the code first.
//
// RFC requirement: RFC3579-2.6.4-1 negative -- the same sentence; the attribute
// is processed even when processing it produces nothing usable, and the failure
// does not become the access control decision (authenticator_eap.go
// authenticateEAP, whose non-challenge branch logs eapErr and drops it).
func TestRadiusAdminEapProcessesAnUnparseableEAPMessageBeforeTheCode(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	srv.mu.Lock()
	// Round 1 is the concluding Access-Accept for MD5-Challenge: identity, then
	// the challenge response, then the verdict.
	srv.corruptEAPFrom = 1
	srv.mu.Unlock()

	a, logged := eapAuthenticatorWithLog(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

	require.NoError(t, err)
	assert.True(t, res.Authenticated, "an unparseable EAP-Message does not deny an Access-Accept")
	assert.Contains(t, logged.String(), "EAP header from the server",
		"the attribute was processed first: its decode error reached the log")
}

// TestRadiusAdminEapAccessRequestNamesTheNAS covers the attribute set every
// EAP-bearing Access-Request owes, with a source address configured so both
// candidates are present.
//
// VALIDATES: every Access-Request of the conversation carries NAS-Identifier,
// and carries NAS-IP-Address when a source address is configured.
// PREVENTS: an EAP path that builds its own attribute list and forgets what the
// PAP path appends.
//
// RFC requirement: RFC3579-3-1 positive -- "either NAS-Identifier,
// NAS-IP-Address or NAS-IPv6-Address attributes MUST be included" in an
// Access-Request (authenticator.go exchange, which appends both).
func TestRadiusAdminEapAccessRequestNamesTheNAS(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := testAuthenticator(t, srv.addr, secret, ExtractedConfig{
		ProfileAttr:   AttrFilterID,
		AuthMethod:    AuthMethodEAPMSCHAPv2,
		SourceAddress: net.IPv4(10, 1, 2, 3),
	})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	captured := srv.captured(t)
	require.Greater(t, len(captured), 1)
	for index, pkt := range captured {
		assert.Equalf(t, "ze-test", string(pkt.FindAttr(AttrNASIdentifier)),
			"request %d names the NAS", index)
		assert.Equalf(t, net.IPv4(10, 1, 2, 3).To4(), net.IP(pkt.FindAttr(AttrNASIPAddress)),
			"request %d carries the configured source address", index)
	}
}

// TestRadiusAdminEapNamesTheNASWithoutASourceAddress removes the attribute that
// would otherwise satisfy the rule.
//
// VALIDATES: with no source address configured, no NAS-IP-Address is sent, and
// NAS-Identifier alone still satisfies the requirement on every request.
// PREVENTS: a deployment whose only NAS identification came from an optional
// leaf, which would leave the requirement unmet as soon as that leaf is unset.
//
// RFC requirement: RFC3579-3-1 negative -- the same sentence read as a
// disjunction: one of the three is enough, and the absent NAS-IP-Address does
// not leave the request unidentified (authenticator.go exchange, which appends
// NAS-IP-Address only when sourceIP.To4() resolves).
func TestRadiusAdminEapNamesTheNASWithoutASourceAddress(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	captured := srv.captured(t)
	require.Greater(t, len(captured), 1)
	for index, pkt := range captured {
		assert.Nilf(t, pkt.FindAttr(AttrNASIPAddress),
			"request %d carries no NAS-IP-Address, because none is configured", index)
		assert.Equalf(t, "ze-test", string(pkt.FindAttr(AttrNASIdentifier)),
			"request %d is still identified, by NAS-Identifier alone", index)
	}
}

// TestRadiusAdminEapNotificationIsAnsweredAndLogged is what a displayable
// message IS allowed to do.
//
// VALIDATES: an EAP Notification Request arriving mid-conversation draws a
// Notification Response on the wire and its text reaches the operator's log.
// PREVENTS: a carrier that drops a Notification, which leaves the peer owing a
// Response the RFC requires and the operator without the message.
//
// RFC requirement: RFC3579-1.2-1 positive -- the displayable message is
// delivered rather than acted on; RFC 3748 Section 5.2 owes the Notification
// Response, and ze logs the text as a value (authenticator_eap.go
// processEAPMessage, whose result.Notified branch logs and does not branch).
func TestRadiusAdminEapNotificationIsAnsweredAndLogged(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	srv.mu.Lock()
	srv.notifyBeforeMethodReply = true
	srv.mu.Unlock()

	a, logged := eapAuthenticatorWithLog(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	assert.Contains(t, logged.String(), "your password expires in 3 days",
		"the displayable message reached the operator's log")

	answered := false
	for _, pkt := range srv.captured(t) {
		if eapPacketOf(t, pkt).Type == eap.TypeNotification {
			answered = true
		}
	}
	assert.True(t, answered, "the peer answered the Notification Request with a Notification Response")
}

// TestRadiusAdminEapNotificationDoesNotSteerTheLogin is the prohibition itself,
// measured against a control run of the same exchange without the message.
//
// VALIDATES: the verdict, the source tag and the profile set are identical with
// and without a Notification carrying text an implementation might act on.
// PREVENTS: any branch on the message -- the attack the sentence exists to stop
// is a server or a man in the middle steering the login with prose.
//
// RFC requirement: RFC3579-1.2-1 negative -- a displayable message "MUST NOT
// affect operation of the protocol" (authenticator_eap.go processEAPMessage,
// where result.Notification is passed to the logger and read by nothing else).
func TestRadiusAdminEapNotificationDoesNotSteerTheLogin(t *testing.T) {
	secret := []byte("testing123")
	reply := []Attr{{Type: AttrFilterID, Value: []byte("admin")}}

	control := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello", reply)
	quiet := eapAuthenticator(t, control.addr, secret, AuthMethodEAPMSCHAPv2)
	want, err := quiet.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, want.Authenticated)

	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello", reply)
	srv.mu.Lock()
	srv.notifyBeforeMethodReply = true
	srv.mu.Unlock()

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	got, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

	require.NoError(t, err, "the message changed no error")
	assert.Equal(t, want.Authenticated, got.Authenticated, "the message changed no verdict")
	assert.Equal(t, want.Profiles, got.Profiles, "the message changed no authorization")
	assert.Equal(t, want.Source, got.Source, "the message changed no source tag")
}

// TestRadiusAdminEapAccessRejectDeniesAccess is the plain denial.
//
// VALIDATES: an Access-Reject concluding an EAP conversation denies the login
// with aaa.ErrAuthRejected, which stops the chain, and grants no profile.
// PREVENTS: a rejected EAP login falling through to the local password
// database, which would let a wrong password reach a different backend.
//
// RFC requirement: RFC3579-2.1-1 positive -- "Reception of a RADIUS
// Access-Reject packet MUST result in the NAS denying access to the
// authenticating peer" (authenticator.go result, whose CodeAccessReject branch
// returns aaa.ErrAuthRejected).
func TestRadiusAdminEapAccessRejectDeniesAccess(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello", nil)

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "wrong"})

	require.ErrorIs(t, err, aaa.ErrAuthRejected, "the chain stops on a rejection")
	assert.False(t, res.Authenticated)
	assert.Empty(t, res.Profiles)
	assert.Greater(t, srv.requestCount(), 1, "the rejection concluded a real EAP conversation")
}

// TestRadiusAdminEapAccessRejectDeniesEvenCarryingEAPSuccess removes the only
// thing that could talk ze out of the denial.
//
// VALIDATES: an Access-Reject whose EAP-Message carries an EAP-Success still
// denies the login, and still stops the chain.
// PREVENTS: an access control decision taken from the encapsulated packet, which
// a server or anything on the path can set independently of the RADIUS code.
//
// RFC requirement: RFC3579-2.1-1 negative -- the same sentence; the denial
// follows the RADIUS packet type and nothing in the EAP-Message overrides it
// (authenticator_eap.go authenticateEAP, which drops eapErr on a non-challenge
// code, and authenticator.go result, which switches on resp.Code alone).
func TestRadiusAdminEapAccessRejectDeniesEvenCarryingEAPSuccess(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello", nil)
	srv.mu.Lock()
	srv.rejectCarriesEAPSuccess = true
	srv.mu.Unlock()

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "wrong"})

	require.ErrorIs(t, err, aaa.ErrAuthRejected,
		"an EAP-Success inside an Access-Reject does not authorize the login")
	assert.False(t, res.Authenticated)
	assert.Empty(t, res.Profiles)
}

// TestRadiusAdminEapNeverSendsAPasswordCredential is the conforming run: the
// operator asked for EAP and every request on the wire is EAP.
//
// VALIDATES: a successful eap-mschapv2 login puts an EAP-Message on every
// Access-Request and a User-Password or CHAP-Password on none of them.
// PREVENTS: a loop that appends the operator's password beside the EAP packet,
// which offers a server the weaker credential it was told not to use.
//
// RFC requirement: RFC3579-4.3.6-2 positive -- an authenticating peer expecting
// EAP negotiates EAP and nothing weaker; every request carries the EAP
// credential alone (authenticator_eap.go eapCredential, which builds the
// attribute list from the EAP packet and never from request.Password).
func TestRadiusAdminEapNeverSendsAPasswordCredential(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	captured := srv.captured(t)
	require.Greater(t, len(captured), 1)
	for index, pkt := range captured {
		assert.NotNilf(t, pkt.FindAttr(AttrEAPMessage), "request %d carries the EAP credential", index)
		assert.Nilf(t, pkt.FindAttr(AttrUserPassword), "request %d carries no User-Password", index)
		assert.Nilf(t, pkt.FindAttr(AttrCHAPPassword), "request %d carries no CHAP-Password", index)
	}
}

// TestRadiusAdminEapDoesNotDowngradeAfterARejection applies the pressure that
// makes a downgrade tempting.
//
// VALIDATES: when the EAP conversation ends in an Access-Reject, ze does not
// retry the login with PAP or CHAP: no request in the whole exchange carries a
// password credential, and the chain stops rather than trying a weaker one.
// PREVENTS: a fallback that answers a failed EAP exchange by offering the
// password, which hands an attacker the downgrade the section is written about.
//
// RFC requirement: RFC3579-4.3.6-2 negative -- "An authenticating peer expecting
// EAP to be negotiated for a session MUST NOT negotiate a weaker method, such as
// CHAP or PAP"; the rejection produces no weaker attempt (authenticator.go
// Authenticate, whose EAP branch returns authenticateEAP's result with no
// fallback path, and credential, which is never reached for an EAP method).
func TestRadiusAdminEapDoesNotDowngradeAfterARejection(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello", nil)

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "wrong"})

	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	assert.False(t, res.Authenticated)

	captured := srv.captured(t)
	require.NotEmpty(t, captured)
	for index, pkt := range captured {
		assert.Nilf(t, pkt.FindAttr(AttrUserPassword),
			"request %d never offers the password after the EAP exchange failed", index)
		assert.Nilf(t, pkt.FindAttr(AttrCHAPPassword),
			"request %d never offers a CHAP-Password after the EAP exchange failed", index)
		assert.NotNilf(t, pkt.FindAttr(AttrEAPMessage),
			"request %d stayed on the configured EAP method", index)
	}
}
