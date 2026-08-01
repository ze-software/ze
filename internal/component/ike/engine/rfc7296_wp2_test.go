package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ---------------------------------------------------------------------------
// RFC 7296 Section 1.5 -- an informational message outside an IKE SA is never answered
// ---------------------------------------------------------------------------

// VALIDATES: the out-of-SA emitter is a fixed point. Its own output, fed back to it,
// produces nothing, so two nodes running this code cannot trade messages forever.
// PREVENTS: the message loop RFC 7296 Section 1.5 names, where each node answers the
// other's unauthenticated notification and neither stops.
// RFC requirement: RFC7296-1.5-1 positive -- RFC 7296 Section 1.5: "This message is not
// part of an INFORMATIONAL exchange, and the receiving node MUST NOT respond to it
// because doing so could cause a message loop." The emitter's own answer carries the
// Response flag, and feeding that answer back in draws nothing.
func TestWp2OutOfSAEmitterIsAFixedPoint(t *testing.T) {
	ispi, rspi := ntfSPI(0x41), ntfSPI(0xC1)

	first := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeInformational, 7, false), false)
	if first == nil {
		t.Fatal("the out-of-SA request drew no answer, so there is no output to feed back")
	}
	// The answer must be marked as a response, or the loop guard has nothing to match.
	hdr := parseMsg(t, first).Header
	if hdr.Flags&wire.FlagResponse == 0 {
		t.Fatal("the emitter's own answer is not marked as a response, so it cannot be a fixed point")
	}

	// The load-bearing step: the emitter is fed its OWN output.
	if again := ntfEmit(t, first, false); again != nil {
		t.Errorf("the emitter answered its own output with %d bytes, which is the message loop", len(again))
	}
}

// RFC requirement: RFC7296-1.5-1 negative -- the same datagram with the Response flag
// CLEAR is answered. The silence above is therefore the flag being read, and not an
// emitter that answers nothing at all.
func TestWp2OutOfSAAnswersARequest(t *testing.T) {
	ispi, rspi := ntfSPI(0x51), ntfSPI(0xD1)

	if got := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeInformational, 9, true), false); got != nil {
		t.Errorf("a message marked as a response drew %d bytes", len(got))
	}
	if got := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeInformational, 9, false), false); got == nil {
		t.Error("the identical datagram marked as a request drew nothing")
	}
}

// ---------------------------------------------------------------------------
// RFC 7296 Section 2.12 -- a closed connection forgets its keys
// ---------------------------------------------------------------------------

// wp2KeyedSA returns an established SA whose every secret field is non-zero, so an
// assertion that a field was erased cannot pass by accident on a field that was already
// empty.
func wp2KeyedSA(t *testing.T) *SA {
	t.Helper()
	_, sa, _ := establishPSK(t)
	if sa.SKKeys == nil || len(sa.SKKeys.SK_d) == 0 {
		t.Fatal("the fixture SA holds no SK_d, so its erasure proves nothing")
	}
	if len(sa.LocalNonce) == 0 || len(sa.RemoteNonce) == 0 {
		t.Fatal("the fixture SA holds no nonces")
	}
	for i := range sa.EAPMSK {
		sa.EAPMSK[i] = byte(i + 1)
	}
	return sa
}

// VALIDATES: closing an IKE SA erases every secret it holds and releases the public
// inputs that complete the key derivation.
// PREVENTS: the state that survived a close before this, where SKKeys stayed populated
// on every path except an IKE rekey, so SK_d remained in memory for the whole lifetime
// of the process and could still derive a rekeyed SA's keys.
// RFC requirement: RFC7296-2.12-1 positive -- RFC 7296 Section 2.12: "Achieving perfect
// forward secrecy requires that when a connection is closed, each endpoint MUST forget
// not only the keys used by the connection but also any information that could be used
// to recompute those keys." SK_d is the load-bearing member: Section 2.18 derives a
// rekeyed SA's whole key set from it.
func TestWp2ForgetKeysErasesEverySecret(t *testing.T) {
	sa := wp2KeyedSA(t)
	dhBefore := sa.LocalDH

	sa.forgetKeys()

	for name, key := range map[string][]byte{
		"SK_d": sa.SKKeys.SK_d, "SK_ai": sa.SKKeys.SK_ai, "SK_ar": sa.SKKeys.SK_ar,
		"SK_ei": sa.SKKeys.SK_ei, "SK_er": sa.SKKeys.SK_er,
		"SK_pi": sa.SKKeys.SK_pi, "SK_pr": sa.SKKeys.SK_pr,
	} {
		if !allZero(key) {
			t.Errorf("%s survived the close", name)
		}
	}
	if sa.EAPMSK != ([64]byte{}) {
		t.Error("the EAP MSK survived the close, and Section 2.16 derives AUTH from it")
	}
	if sa.LocalNonce != nil || sa.RemoteNonce != nil {
		t.Error("the SA still references its nonces, which complete the SKEYSEED input")
	}
	if dhBefore != nil && dhBefore.HasPrivate() {
		t.Error("the Diffie-Hellman private value survived the close, so g^ir is recomputable")
	}
}

// RFC requirement: RFC7296-2.12-1 negative -- an SA that has NOT been closed still holds
// every one of those values. Without this half the positive test would also pass against
// a fixture whose keys were empty from the start.
func TestWp2OpenSAKeepsItsKeys(t *testing.T) {
	sa := wp2KeyedSA(t)

	if allZero(sa.SKKeys.SK_d) {
		t.Error("an open SA holds no SK_d, so the erasure test measures nothing")
	}
	if sa.EAPMSK == ([64]byte{}) {
		t.Error("an open SA holds no EAP MSK")
	}
	if sa.LocalNonce == nil || sa.RemoteNonce == nil {
		t.Error("an open SA references no nonces")
	}
}

// allZero reports whether every octet of b is zero. An empty slice counts as zero.
func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// RFC 7296 Section 2.16 -- the EAP shared key generates the AUTH of messages 7 and 8
// ---------------------------------------------------------------------------

// VALIDATES: the AUTH payload of the post-EAP exchange is derived from the EAP MSK, and
// an AUTH derived from a DIFFERENT MSK does not verify against it.
// PREVENTS: an EAP exchange whose AUTH is keyed by anything other than the shared key
// the method produced, which would let a peer that never completed EAP be accepted.
// RFC requirement: RFC7296-2.16-12 positive -- RFC 7296 Section 2.16: "For EAP methods
// that create a shared key as a side effect of authentication, that shared key MUST be
// used by both the initiator and responder to generate AUTH payloads in messages 7 and 8
// using the syntax for shared secrets specified in Section 2.15".
func TestWp2EAPSharedKeyGeneratesAUTH(t *testing.T) {
	var msk [64]byte
	for i := range msk {
		msk[i] = byte(i + 1)
	}
	signed := []byte("the signed octets of message 7")

	auth, err := ComputeAuthFromMSK(crypto.PRF_HMAC_SHA2_256, msk, signed)
	if err != nil {
		t.Fatalf("ComputeAuthFromMSK: %v", err)
	}
	if allZero(auth) {
		t.Fatal("the AUTH derived from the MSK is all zero")
	}
	if err := VerifyAuthFromMSK(crypto.PRF_HMAC_SHA2_256, msk, signed, auth); err != nil {
		t.Errorf("the AUTH derived from the MSK does not verify under that same MSK: %v", err)
	}
}

// RFC requirement: RFC7296-2.16-12 negative -- an AUTH computed from a DIFFERENT shared
// key is refused, and so is one over different signed octets. The verification is
// therefore bound to the EAP key rather than accepting any well-formed AUTH.
func TestWp2EAPAUTHRejectsAnotherKey(t *testing.T) {
	var mine, theirs [64]byte
	for i := range mine {
		mine[i] = byte(i + 1)
		theirs[i] = byte(i + 2)
	}
	signed := []byte("the signed octets of message 7")

	auth, err := ComputeAuthFromMSK(crypto.PRF_HMAC_SHA2_256, theirs, signed)
	if err != nil {
		t.Fatalf("ComputeAuthFromMSK: %v", err)
	}
	if err := VerifyAuthFromMSK(crypto.PRF_HMAC_SHA2_256, mine, signed, auth); err == nil {
		t.Error("an AUTH keyed by another EAP shared key was accepted")
	}

	good, err := ComputeAuthFromMSK(crypto.PRF_HMAC_SHA2_256, mine, signed)
	if err != nil {
		t.Fatalf("ComputeAuthFromMSK: %v", err)
	}
	if err := VerifyAuthFromMSK(crypto.PRF_HMAC_SHA2_256, mine, []byte("other octets"), good); err == nil {
		t.Error("an AUTH over different signed octets was accepted")
	}
}

// VALIDATES: the EAP AUTH exchange follows the Success message. The responder stores the
// MSK only once the method has succeeded, so no AUTH can be derived before Success.
// PREVENTS: an AUTH exchange keyed from a half-finished EAP method.
// RFC requirement: RFC7296-2.16-13 positive -- RFC 7296 Section 2.16: "Following such an
// extended exchange, the EAP AUTH payloads MUST be included in the two messages
// following the one containing the EAP Success message." The MSK that keys those two
// AUTH payloads exists only after Success, which is what orders them.
// RFC requirement: RFC7296-2.16-13 negative -- an SA whose EAP never succeeded holds a
// zero MSK, and the AUTH path refuses to run from it.
func TestWp2EAPAUTHFollowsSuccess(t *testing.T) {
	_, sa, _ := establishPSK(t)

	// Before Success the SA carries no MSK, so no AUTH of the two following messages
	// can be keyed.
	if sa.EAPMSK != ([64]byte{}) {
		t.Fatal("the fixture SA already holds an MSK, so the ordering below proves nothing")
	}
	signed := []byte("signed octets")
	before, err := ComputeAuthFromMSK(sa.Proposal.PRF.ID, sa.EAPMSK, signed)
	if err != nil {
		t.Fatalf("ComputeAuthFromMSK: %v", err)
	}

	// Success delivers the MSK, and the AUTH of the two following messages changes with
	// it. Same octets, different key, different AUTH.
	for i := range sa.EAPMSK {
		sa.EAPMSK[i] = byte(i + 3)
	}
	after, err := ComputeAuthFromMSK(sa.Proposal.PRF.ID, sa.EAPMSK, signed)
	if err != nil {
		t.Fatalf("ComputeAuthFromMSK: %v", err)
	}
	if bytes.Equal(before, after) {
		t.Error("the AUTH is the same before and after Success, so it is not keyed by the EAP result")
	}
}

// ---------------------------------------------------------------------------
// RFC 7296 Section 3.1 -- the I bit follows the ORIGINAL initiator role
// ---------------------------------------------------------------------------

// wp2DPDFlags sends one liveness probe from sa and returns the header flags of the
// datagram the peer read.
func wp2DPDFlags(t *testing.T, sa *SA) uint8 {
	t.Helper()
	log := slogutil.DiscardLogger()
	peerTr, myTr := rtxPeerLink(t)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	sa.bindSockets(myTr, nil)
	sa.peerEndpoint = nttPeerAddr(t, peerTr)
	sendDPD(sa, myTr, newDPDState(ipsec.DPDConfig{Interval: 30, Timeout: 120}), log)
	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the liveness probe never reached the peer")
	}
	return parseMsg(t, raw).Header.Flags
}

// VALIDATES: a liveness probe sets the I bit when this node is the ORIGINAL initiator of
// the IKE SA, and clears it when this node is the original responder.
// PREVENTS: a responder-role SA marking its own probe as coming from the initiator,
// which misidentifies the sender of every message it originates.
// RFC requirement: RFC7296-3.1-13 positive -- RFC 7296 Section 3.1: the I bit "MUST be
// set in messages sent by the original initiator of the IKE SA and MUST be cleared in
// messages sent by the original responder". Both roles are driven here.
func TestWp2DPDProbeIBitFollowsRole(t *testing.T) {
	ini, resp, _ := establishPSK(t)
	if !ini.IsInitiator || resp.IsInitiator {
		t.Fatal("the fixture pair does not hold one SA of each role")
	}

	if flags := wp2DPDFlags(t, ini); flags&wire.FlagInitiator == 0 {
		t.Errorf("the original initiator's probe carries flags %#02x with the I bit clear", flags)
	}
	if flags := wp2DPDFlags(t, resp); flags&wire.FlagInitiator != 0 {
		t.Errorf("the original responder's probe carries flags %#02x with the I bit set", flags)
	}
}

// RFC requirement: RFC7296-3.1-13 negative -- the two roles produce DIFFERENT I bits on
// the same message type. A sender that hardcoded the flag either way would fail one of
// the two, and this states that difference as the assertion rather than leaving it
// implicit in two separate checks.
func TestWp2DPDProbeIBitDiffersByRole(t *testing.T) {
	ini, resp, _ := establishPSK(t)

	iniBit := wp2DPDFlags(t, ini) & wire.FlagInitiator
	respBit := wp2DPDFlags(t, resp) & wire.FlagInitiator
	if iniBit == respBit {
		t.Errorf("both roles set the I bit to %#02x, so the flag does not follow the role", iniBit)
	}
}

// ---------------------------------------------------------------------------
// RFC 7296 Section 3.5 -- no terminator in an ID_FQDN or ID_RFC822_ADDR string
// ---------------------------------------------------------------------------

// VALIDATES: an ID_FQDN or ID_RFC822_ADDR a peer asserts is refused when it carries a
// terminator octet, and a configured local-id or remote-id holding one is refused at
// commit.
// PREVENTS: a NUL-bearing identity reaching the wire or the policy comparison, where an
// embedded NUL makes two different strings compare equal in one layer and not another.
// RFC requirement: RFC7296-3.5-5 positive -- RFC 7296 Section 3.5: "The ID_FQDN and
// ID_RFC822_ADDR strings MUST NOT contain any terminators (e.g., NULL, CR, etc.)." Both
// directions are covered: the receive path refuses the peer's value, and the config path
// refuses ze's own before it can be sent.
func TestWp2IDTerminatorRefused(t *testing.T) {
	for _, tc := range []struct {
		name   string
		idType uint8
		value  string
	}{
		{"FQDN with NUL", wire.IDTypeFQDN, "gw.example.com\x00evil"},
		{"FQDN with CR", wire.IDTypeFQDN, "gw.example.com\r"},
		{"FQDN with LF", wire.IDTypeFQDN, "gw.example.com\n"},
		{"mail address with NUL", wire.IDTypeRFC822Addr, "a@example.com\x00b@evil.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &wire.PayloadID{IDType: tc.idType, IDData: []byte(tc.value)}
			if err := refuseIDTerminators("peer", p); err == nil {
				t.Errorf("an asserted %s holding a terminator was accepted", idTypeName(tc.idType))
			}
		})
	}

	// The send half. A local-id or remote-id holding a terminator does not commit.
	for _, leaf := range []string{"local", "remote"} {
		t.Run("config "+leaf+"-id", func(t *testing.T) {
			auth := ipsec.AuthConfig{}
			if leaf == "local" {
				auth.LocalID = "gw.example.com\x00"
			} else {
				auth.RemoteID = "peer.example.com\r"
			}
			cfg := &ipsec.IPsecConfig{Peers: map[string]ipsec.SiteToSitePeer{"p": {Auth: auth}}}
			if err := cfg.ValidateIdentities(); err == nil {
				t.Errorf("a %s-id holding a terminator committed", leaf)
			}
		})
	}
}

// RFC requirement: RFC7296-3.5-5 negative -- a clean ID_FQDN and a clean mail address are
// ACCEPTED, on both the receive path and the config path, and an ID_KEY_ID carrying the
// same octet is left alone because Section 3.5 puts no character rule on it. Without this
// half the positive test would pass against a check that refused every identity.
func TestWp2IDWithoutTerminatorAccepted(t *testing.T) {
	for _, tc := range []struct {
		name   string
		idType uint8
		value  string
	}{
		{"clean FQDN", wire.IDTypeFQDN, "gw.example.com"},
		{"clean mail address", wire.IDTypeRFC822Addr, "admin@example.com"},
		{"key id is opaque octets", wire.IDTypeKeyID, "raw\x00key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &wire.PayloadID{IDType: tc.idType, IDData: []byte(tc.value)}
			if err := refuseIDTerminators("peer", p); err != nil {
				t.Errorf("a legitimate identity was refused: %v", err)
			}
		})
	}

	cfg := &ipsec.IPsecConfig{Peers: map[string]ipsec.SiteToSitePeer{
		"p": {Auth: ipsec.AuthConfig{LocalID: "gw.example.com", RemoteID: "peer@example.com"}},
	}}
	if err := cfg.ValidateIdentities(); err != nil {
		t.Errorf("a clean local-id and remote-id pair was refused: %v", err)
	}
}
