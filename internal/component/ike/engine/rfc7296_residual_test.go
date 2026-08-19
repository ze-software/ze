package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// resHasInitialContact reports whether a decrypted payload chain carries the
// INITIAL_CONTACT notify.
func resHasInitialContact(inner []wire.PayloadEntry) bool {
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok &&
			n.NotifyMsgType == wire.NotifyInitialContact {
			return true
		}
	}
	return false
}

// resAuthRequestPayloads builds a first IKE_AUTH request under the given auth mode
// and returns its decrypted inner payload chain. The established PSK pair supplies
// the key hierarchy; only the configured auth MODE varies, which is the single input
// mayBeReplicated reads.
func resAuthRequestPayloads(t *testing.T, mode ipsec.AuthMode) []wire.PayloadEntry {
	t.Helper()
	ini, resp, _ := establishPSK(t)
	ini.PeerCfg.Auth.Mode = mode
	raw, err := buildAuthRequest(ini)
	if err != nil {
		t.Fatalf("buildAuthRequest(%v): %v", mode, err)
	}
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt IKE_AUTH request: %v", err)
	}
	return inner
}

// VALIDATES: a device identity still asserts INITIAL_CONTACT on its first IKE_AUTH.
// A pre-shared secret and an X.509 certificate both name one configured peer, which
// ze runs as one SA, so the assertion that this is the only IKE SA between the two
// identities is truthful and is sent.
// PREVENTS: closing the Section 2.4 MUST NOT by simply deleting the notification.
// That would silence it for every peer, and a responder would then keep a stale SA
// to us until it timed out instead of replacing it at once.
// RFC requirement: RFC7296-2.4-14 positive -- mayBeReplicated (auth.go) reports false
// for a device identity, so buildAuthRequest still emits INITIAL_CONTACT.
func TestResInitialContactSentByNonReplicableIdentity(t *testing.T) {
	for _, mode := range []ipsec.AuthMode{ipsec.AuthPreSharedSecret, ipsec.AuthX509} {
		t.Run(mode.String(), func(t *testing.T) {
			if mayBeReplicated(ipsec.SiteToSitePeer{Auth: ipsec.AuthConfig{Mode: mode}}) {
				t.Fatalf("auth mode %v names one configured device, so it is not replicable", mode)
			}
		})
	}
	if !resHasInitialContact(resAuthRequestPayloads(t, ipsec.AuthPreSharedSecret)) {
		t.Fatal("a pre-shared-secret peer is not replicable, but its first IKE_AUTH carried no INITIAL_CONTACT")
	}
}

// VALIDATES: an identity that may be authenticated from more than one system at a
// time does NOT send INITIAL_CONTACT. RFC 7296 Section 2.4: "This notification MUST
// NOT be sent by an entity that may be replicated (e.g., a roaming user's credentials
// where the user is allowed to connect to the corporate firewall from two remote
// systems at the same time)." An EAP credential names a user, not a device.
// PREVENTS: the roaming-user harm the MUST NOT exists to stop. The notification tells
// the gateway that this is the only SA to the authenticated identity, and the gateway
// acts by deleting the others, so a user's second device would tear down the session
// held by the first. The whole IKE_AUTH request is decrypted and walked, so a notify
// moved to another position in the chain still counts.
// RFC requirement: RFC7296-2.4-14 negative -- mayBeReplicated (auth.go) reports true
// for an EAP identity, so buildAuthRequest omits INITIAL_CONTACT.
func TestResInitialContactNotSentByReplicableIdentity(t *testing.T) {
	for _, mode := range []ipsec.AuthMode{ipsec.AuthEAPTLS, ipsec.AuthEAPMSCHAPv2} {
		t.Run(mode.String(), func(t *testing.T) {
			if !mayBeReplicated(ipsec.SiteToSitePeer{Auth: ipsec.AuthConfig{Mode: mode}}) {
				t.Fatalf("auth mode %v names a user credential, which may be replicated", mode)
			}
			if resHasInitialContact(resAuthRequestPayloads(t, mode)) {
				t.Fatalf("auth mode %v may be replicated, but its first IKE_AUTH carried INITIAL_CONTACT", mode)
			}
		})
	}
}

// VALIDATES: an IKE SA inside its negotiated lifetime is used normally, so the
// expiry refusal is reached only by an expired SA.
// PREVENTS: a refusal that fires for every SA. Without this half, a check that always
// returned "expired" would satisfy the negative test below while breaking every
// exchange, and the negative would still look green.
// RFC requirement: RFC7296-2.8-8 positive -- SA.lifetimeExpired (sa.go) reports false
// before the hard time, and buildEncryptedMessageEx protects the message.
func TestResUnexpiredSAIsUsed(t *testing.T) {
	ini, _, _ := establishPSK(t)
	payloads := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyInitialContact}}}

	for _, c := range []struct {
		name string
		set  func()
	}{
		{"no lifetime configured", func() { ini.setHardExpiry(time.Time{}) }},
		{"an hour of lifetime left", func() { ini.setHardExpiry(time.Now().Add(time.Hour)) }},
		{"one second of lifetime left", func() { ini.setHardExpiry(time.Now().Add(time.Second)) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.set()
			if ini.lifetimeExpired(time.Now()) {
				t.Fatal("an SA inside its lifetime reported itself expired")
			}
			if _, err := buildEncryptedMessageEx(ini, payloads, 7, wire.ExchangeInformational, 0); err != nil {
				t.Fatalf("an SA inside its lifetime refused to protect a message: %v", err)
			}
		})
	}
}

// VALIDATES: once the negotiated lifetime has passed, the IKE SA is not used. Every
// message this node protects with the SA's keys is built by buildEncryptedMessageEx,
// so refusing there is what makes the SA unusable rather than merely scheduled for
// teardown.
// PREVENTS: the gap the owner loop cannot close on its own. It notices expiry on a
// one-second tick and only after any in-flight rekey, so without this refusal an SA
// past its hard lifetime keeps protecting DPD probes, rekeys and Delete payloads --
// exactly the "MUST NOT be used" Section 2.8 forbids. The boundary instant is
// included because expiry is inclusive: at the hard time the lifetime has expired.
// RFC requirement: RFC7296-2.8-8 negative -- SA.lifetimeExpired (sa.go) reports true
// at and after the hard time, and buildEncryptedMessageEx refuses with errSAExpired.
func TestResExpiredSAIsNotUsed(t *testing.T) {
	ini, _, _ := establishPSK(t)
	payloads := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyInitialContact}}}

	for _, c := range []struct {
		name string
		at   time.Duration
	}{
		{"expired an hour ago", -time.Hour},
		{"expired a second ago", -time.Second},
		{"expiring exactly now", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			ini.setHardExpiry(time.Now().Add(c.at))
			if !ini.lifetimeExpired(time.Now()) {
				t.Fatal("an SA past its hard lifetime did not report itself expired")
			}
			_, err := buildEncryptedMessageEx(ini, payloads, 7, wire.ExchangeInformational, 0)
			if err == nil {
				t.Fatal("an expired SA protected a message; RFC 7296 Section 2.8 forbids using it")
			}
			if !errors.Is(err, errSAExpired) {
				t.Fatalf("an expired SA was refused for the wrong reason: %v", err)
			}
		})
	}
}

// VALIDATES: the rekey trigger is placed a whole retransmit budget before the hard
// time, so a rekey that starts on time has room to finish and retransmit before the
// SA becomes unusable.
// PREVENTS: the regression that unconditional hard expiry would otherwise cause.
// lifetimeJitter alone can return zero, which would put the soft trigger exactly ON
// the hard time; the rekey would then be cut off the tick it started and the tunnel
// would bounce every lifetime instead of rekeying.
// RFC requirement: RFC7296-2.8-8 positive -- rekeyLead (rekey.go) keeps the soft
// trigger a full retransmit budget before the hard time, so an unexpired SA stays
// usable for the whole exchange that replaces it.
func TestResRekeyLeadLeavesRoomBeforeHardExpiry(t *testing.T) {
	budget := time.Duration(maxRetransmissions) * rekeyRetransmitTimeout

	for _, c := range []struct {
		name     string
		lifetime time.Duration
		wantMin  time.Duration
	}{
		{"default ESP lifetime", 3600 * time.Second, budget},
		{"default IKE lifetime", 28800 * time.Second, budget},
		{"a lifetime just above twice the budget", 2*budget + time.Second, budget},
		{"a short lifetime funds half of itself", 10 * time.Second, 5 * time.Second},
		{"a very short lifetime still leads", 2 * time.Second, time.Second},
	} {
		t.Run(c.name, func(t *testing.T) {
			for range 32 {
				lead := rekeyLead(c.lifetime)
				if lead < c.wantMin {
					t.Fatalf("rekeyLead(%v) = %v, want at least %v", c.lifetime, lead, c.wantMin)
				}
				if lead >= c.lifetime {
					t.Fatalf("rekeyLead(%v) = %v, which is not before the hard time", c.lifetime, lead)
				}
			}
		})
	}

	// The state built from a lifetime therefore always rekeys strictly before it dies.
	ls := newLifetimeState(3600)
	if ls == nil {
		t.Fatal("a configured lifetime produced no lifetime state")
	}
	if !ls.softTime.Before(ls.hardTime) {
		t.Fatal("the soft (rekey) time is not before the hard (unusable) time")
	}
	if got := ls.hardTime.Sub(ls.softTime); got < budget {
		t.Fatalf("the rekey trigger leaves %v before the hard time, want at least %v", got, budget)
	}
}

// resNoAdditionalSAs returns the payload chain of a CREATE_CHILD_SA response that
// carries nothing but a NO_ADDITIONAL_SAS notify. RFC 7296 Section 4 describes the
// peer that sends it: a minimal implementation that recognizes the request only to
// reject it.
func resNoAdditionalSAs() []wire.PayloadEntry {
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNoAdditionalSAs}},
	}
}

// resRekeyRefused drives one Child SA rekey to the point of an answer, feeds it the
// given response chain, and returns the outcome the owner loop would act on.
func resRekeyRefused(t *testing.T, inner []wire.PayloadEntry) ownedOutcome {
	t.Helper()
	log := slogutil.DiscardLogger()
	_, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"

	dp := &rkyDP{}
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)

	ps.startChildRekey(sa, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the rekey request never reached the peer")
	}
	if ps.pendingRekey == nil {
		t.Fatal("the rekey left no outstanding exchange")
	}
	respMsg := &wire.Message{Header: wire.Header{MessageID: ps.pendingRekey.messageID}}
	return ps.handleCreateChildSAOwned(sa, respMsg, inner, true, myTr, dp, log)
}

// VALIDATES: a rekey the peer refuses with NO_ADDITIONAL_SAS makes ze fall back to
// deleting the old SA and creating a new one. RFC 7296 Section 4: "If the responder
// rejects the CREATE_CHILD_SA request with a NO_ADDITIONAL_SAS notification, the
// implementation MUST be capable of instead deleting the old SA and creating a new
// one." The owner loop reads the reestablish signal, drops the Child SA and returns,
// and the reconnect path then runs the initial exchanges, which build a fresh one.
// PREVENTS: the 1 Hz rekey loop this answer used to cause. NO_ADDITIONAL_SAS is a
// RECOGNIZED error notify, so it slips past failIfUnrecognizedErrorNotify and used to
// surface as a generic "missing Nr" warning; the soft lifetime is a level trigger, so
// the next tick retried against a peer that had said never, until the hard lifetime
// dropped the tunnel altogether.
// RFC requirement: RFC7296-4-1 positive -- applyChildRekeyResponse (rekey.go) reports
// errNoAdditionalSAs and handleCreateChildSAOwned (inbound.go) asks for re-establishment.
func TestResNoAdditionalSAsTriggersReestablish(t *testing.T) {
	out := resRekeyRefused(t, resNoAdditionalSAs())
	if !out.reestablish {
		t.Fatal("a NO_ADDITIONAL_SAS answer did not ask for the delete-and-create fallback RFC 7296 Section 4 requires")
	}
	if out.newChild != nil {
		t.Fatal("a refused rekey installed a replacement Child SA")
	}
}

// VALIDATES: the fallback is specific to NO_ADDITIONAL_SAS. A TEMPORARY_FAILURE
// answer, and a response that simply lacks the keys, both leave the session up.
// PREVENTS: a fallback that fires on any rekey failure. Tearing the tunnel down on a
// TEMPORARY_FAILURE would contradict Section 2.25, which says to wait and retry, and
// tearing it down on a malformed response would hand any peer a way to force a
// re-handshake. This is what makes the positive above evidence of the notify being
// read, rather than of the rekey merely having failed.
// RFC requirement: RFC7296-4-1 negative -- handleCreateChildSAOwned (inbound.go) asks
// for no re-establishment when the refusal is not NO_ADDITIONAL_SAS.
func TestResOtherRekeyFailuresDoNotReestablish(t *testing.T) {
	for _, c := range []struct {
		name  string
		inner []wire.PayloadEntry
	}{
		{"TEMPORARY_FAILURE", midTemporaryFailure()},
		{"a response carrying no keys", []wire.PayloadEntry{}},
		{"an unrelated status notify", []wire.PayloadEntry{
			{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyAdditionalTSPossible}},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if resRekeyRefused(t, c.inner).reestablish {
				t.Fatalf("a %s answer asked for the NO_ADDITIONAL_SAS fallback", c.name)
			}
		})
	}
}

// resRewriteKEGroup rewrites the Diffie-Hellman Group Num of the KE payload in an
// unencrypted IKE_SA_INIT message, leaving every other field alone.
func resRewriteKEGroup(t *testing.T, raw []byte, group uint16) []byte {
	t.Helper()
	msg := parseMsg(t, raw)
	found := false
	for i := range msg.Payloads {
		if p, ok := msg.Payloads[i].Payload.(*wire.PayloadKE); ok {
			p.DHGroup = group
			found = true
		}
	}
	if !found {
		t.Fatal("the IKE_SA_INIT response carries no KE payload")
	}
	buf := make([]byte, 4096)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		t.Fatalf("rewrite the IKE_SA_INIT response: %v", err)
	}
	return buf[:n]
}

// VALIDATES: the initiator applies the Section 3.4 rule to the responder's
// IKE_SA_INIT response. An untouched response is accepted; the same response with
// only the KEr group changed to one the accepted proposal does not name is refused,
// and the SA does not reach the established path.
// PREVENTS: the validator existing but never being called. handleSAInitResponse read
// only KeyExchangeData and never looked at DHGroup, so a responder could answer in a
// group it never named and the shared secret would be derived under it. This is the
// wiring half: the unit test proves ValidateKEGroup decides correctly, and this
// proves the initiator asks it.
// RFC requirement: RFC7296-3.4-2 negative -- handleSAInitResponse (fsm.go) calls
// PayloadSA.ValidateKEGroup and kills the SA when the KEr group is not in SAr1.
func TestResInitiatorRejectsKEGroupOutsideTheAcceptedOffer(t *testing.T) {
	log := slogutil.DiscardLogger()

	// Baseline: the untouched response is accepted, so a later refusal is the check
	// and not a broken fixture.
	ini, table, respRaw := negSAInitPair(t)
	handleSAInitResponse(ini, parseMsg(t, respRaw), respRaw, table, nil, nil, log)
	if ini.State == StateDead {
		t.Fatalf("the unmodified IKE_SA_INIT response was refused; state %v", ini.State)
	}

	// The same response, with only the KE group changed to one SAr1 does not name.
	for _, group := range []uint16{19, 20, wire.DHGroupNone} {
		ini, table, respRaw := negSAInitPair(t)
		mutated := resRewriteKEGroup(t, respRaw, group)
		handleSAInitResponse(ini, parseMsg(t, mutated), mutated, table, nil, nil, log)
		if ini.State != StateDead {
			t.Fatalf("a KEr in group %d, which the accepted proposal does not name, was accepted; state %v", group, ini.State)
		}
	}
}

// VALIDATES: ze never selects the Diffie-Hellman group NONE for an IKE SA, so the
// case Section 3.3.6 governs -- a responder that SELECTED NONE and must then ignore
// the initiator's KE payload and omit its own -- is never entered. The refusal is the
// property that makes this true: acceptDHGroup rejects NONE outright for an IKE SA,
// and a proposal offering only NONE is incomplete for the same reason.
// PREVENTS: reading conformance out of the absence of a code path. The obligation is
// met because a real guard refuses NONE, not because nobody wrote the branch; delete
// that guard and this test fails while the KE-omission branch is still missing.
// RFC requirement: RFC7296-3.3.6-8 negative -- acceptDHGroup (crypto/proposal.go)
// refuses group NONE for an IKE SA, so a responder never selects it.
func TestResIKESANeverSelectsDHGroupNone(t *testing.T) {
	local := ipsec.IKEGroup{
		Name: "test-ike",
		Proposals: []ipsec.IKEProposal{
			{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 14},
		},
	}
	offer := []wire.Proposal{{
		Number: 1, ProtocolID: wire.ProtocolIKE,
		Transforms: []wire.Transform{
			{Type: wire.TransformTypeENCR, ID: 12, Attrs: []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: 256}}},
			{Type: wire.TransformTypePRF, ID: 5},
			{Type: wire.TransformTypeINTG, ID: 12},
			{Type: wire.TransformTypeDH, ID: wire.DHGroupNone},
		},
	}}
	if _, err := crypto.NegotiateIKE(wireProposalsToIKE(offer), buildIKEProposals(local)); err == nil {
		t.Fatal("an IKE proposal offering only Diffie-Hellman group NONE was accepted; RFC 7296 Section 3.3.6's omit-KE case would then be reachable and is unimplemented")
	}
}

// The RFC7296-3.3.6-8 positive asserts the rule's consequent, KE ignored on input
// and omitted from the response, on the Child SA path, which is the one exchange
// where ze really does select NONE. Asserting only which group NegotiateIKE picks
// gated nothing: the pick is defended by three independent refusals plus
// `chosen := *local`, so no single mutation could kill it.

// VALIDATES: the consequent of the rule, on the one exchange where ze does select
// the Diffie-Hellman group NONE. A Child SA rekey without PFS IS a selected NONE, and
// RFC 7296 Section 3.3.6 then requires the responder to "ignore the initiator's KE
// payload and omit the KE payload from the response". A CREATE_CHILD_SA request that
// offers no DH group and carries a KE payload anyway is answered with a response that
// carries none, and the rekey still succeeds -- which is the payload being ignored
// rather than the request being refused.
// PREVENTS: reading the KE payload of a no-PFS Child SA rekey, or echoing one back.
// Either would key the replacement Child SA off a Diffie-Hellman exchange neither
// side selected, and a peer that sent a stray KE payload would change the keys ze
// installs. The request is byte-identical to the baseline except for that payload, so
// a response that differs can only be the KE payload being honored.
// RFC requirement: RFC7296-3.3.6-8 positive -- respondChildRekey (rekey.go) builds
// SA, Nr and TS only, so a selected NONE draws a response with no KE payload.
func TestResSelectedDHGroupNoneOmitsKEFromTheResponse(t *testing.T) {
	build := func(t *testing.T, withKE bool) []wire.PayloadEntry {
		t.Helper()
		sa := testSAWithGCMKeys(t)
		sa.ESPGroup = testESPGroup()
		dp := &mockDP{}
		log := slogutil.DiscardLogger()
		old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		// TSi and TSr are part of the rekey request (RFC 7296 Section 1.3.3), and both
		// cases carry the same pair. The KE payload is the only difference between them,
		// which is what makes a difference in the response readable.
		inner := []wire.PayloadEntry{
			{Payload: espSAPayload(0x01020304)},
			{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
			{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.0.0.2/32")},
			{Payload: tsPayload(t, wire.PayloadTypeTSr, "10.0.0.1/32")},
		}
		if withKE {
			inner = append(inner, wire.PayloadEntry{
				Payload: &wire.PayloadKE{DHGroup: 14, KeyExchangeData: testNonce(9)},
			})
		}
		raw, child, err := respondChildRekey(sa, inner, old, 5, dp, log)
		if err != nil {
			t.Fatalf("respondChildRekey(withKE=%v): %v", withKE, err)
		}
		if child == nil {
			t.Fatalf("respondChildRekey(withKE=%v) installed no replacement Child SA", withKE)
		}
		reader := *sa
		reader.IsInitiator = !sa.IsInitiator
		out, err := decryptAndParse(&reader, parseMsg(t, raw), raw)
		if err != nil {
			t.Fatalf("decrypt the child rekey response (withKE=%v): %v", withKE, err)
		}
		return out
	}

	for _, withKE := range []bool{false, true} {
		for _, pe := range build(t, withKE) {
			if _, ok := pe.Payload.(*wire.PayloadKE); ok {
				t.Fatalf("the child rekey response carried a KE payload (request carried one: %v); "+
					"a selected Diffie-Hellman group of NONE must omit it", withKE)
			}
		}
	}
}
