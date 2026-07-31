package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// kegIKEGroup offers two Diffie-Hellman groups. newInitiatorSA guesses the first, so a
// responder that prefers the second drives the INVALID_KE_PAYLOAD retry.
func kegIKEGroup() ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "test-ike",
		Proposals: []ipsec.IKEProposal{
			{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 14},
			{Number: 2, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 19},
		},
	}
}

// kegResponderGroup accepts only the group the initiator guesses second.
func kegResponderGroup() ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "test-ike",
		Proposals: []ipsec.IKEProposal{
			{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 19},
		},
	}
}

// kegSuiteGroup offers three distinct cryptographic suites across two groups, so a
// narrowed re-offer is visible.
func kegSuiteGroup() ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "test-ike",
		Proposals: []ipsec.IKEProposal{
			{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 14},
			{Number: 2, Encryption: ipsec.EncryptionAES128, Hash: ipsec.HashSHA256, DHGroup: 19},
			{Number: 3, Encryption: ipsec.EncryptionAES256GCM, Hash: ipsec.HashSHA256, DHGroup: 19},
		},
	}
}

// kegNotifyResponse builds the IKE_SA_INIT response a responder sends when it answers
// with a single notify and creates no IKE SA.
func kegNotifyResponse(spiI, spiR [8]byte, notifyType uint16, data []byte) []byte {
	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: spiI,
			ResponderSPI: spiR,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagResponse,
		},
		Payloads: []wire.PayloadEntry{{Payload: &wire.PayloadNotify{
			NotifyMsgType:    notifyType,
			NotificationData: data,
		}}},
	}
	buf := make([]byte, msg.Len())
	return buf[:msg.WriteTo(buf, 0)]
}

// kegInitiator returns an initiator SA that has sent its first IKE_SA_INIT, plus the
// table holding it.
func kegInitiator(t *testing.T, group ipsec.IKEGroup) (*SA, *SATable) {
	t.Helper()
	iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
	sa, err := newInitiatorSA("ze", iniPeer, group, testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table := NewSATable()
	table.Insert(sa)
	req := buildSAInitRequest(sa, group)
	sa.InitiatorSAInitMsg = req
	sa.LastSentMsg = req
	sa.State = StateSAInitSent
	return sa, table
}

// kegPayload returns the first payload of the given type in a raw message, or nil.
func kegPayload[T wire.Payload](t *testing.T, raw []byte) T {
	t.Helper()
	var zero T
	for _, pe := range parseMsg(t, raw).Payloads {
		if p, ok := pe.Payload.(T); ok {
			return p
		}
	}
	return zero
}

// VALIDATES: an INVALID_KE_PAYLOAD naming a group this node proposed makes the
// initiator retry the IKE_SA_INIT with that group, rather than killing the SA.
// PREVENTS: the defect this row closes -- the response fell to the completeness gate and
// set StateDead, runInitiator re-sent the SAME wrong-group request until its budget was
// spent, and newInitiatorSA rebuilt the same group on the next cycle. Ze could never
// establish with a peer preferring another group.
// RFC requirement: RFC7296-1.2-5 positive -- handleSAInitResponse (fsm.go) records the
// notify and retrySAInit (sa_init_retry.go) rebuilds the request under the named group,
// re-anchoring sa.InitiatorSAInitMsg so the later AUTH is computed over what was sent.
func TestKegInitiatorRetriesOnInvalidKEPayload(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa, table := kegInitiator(t, kegIKEGroup())
	first := append([]byte(nil), sa.LastSentMsg...)

	resp := kegNotifyResponse(sa.InitiatorSPI, [8]byte{}, wire.NotifyInvalidKEPayload, []byte{0x00, 19})
	handleSAInitResponse(sa, parseMsg(t, resp), resp, table, nil, nil, log)

	if sa.State != StateSAInitSent {
		t.Fatalf("state = %v, want sa-init-sent; the SA was killed instead of retried", sa.State)
	}
	if sa.LocalDH == nil || sa.LocalDH.GroupID != 19 {
		t.Fatalf("the retry did not rebuild the Diffie-Hellman key in the named group")
	}
	if bytes.Equal(sa.LastSentMsg, first) {
		t.Fatal("no new IKE_SA_INIT was built; the same wrong-group request would be re-sent")
	}
	ke := kegPayload[*wire.PayloadKE](t, sa.LastSentMsg)
	if ke == nil || ke.DHGroup != 19 {
		t.Errorf("the retry's KE payload names group %v, want 19", ke)
	}
	if !bytes.Equal(ke.KeyExchangeData, sa.LocalDH.PublicKey) {
		t.Error("the KE payload does not carry the key the retry actually built")
	}
	if !bytes.Equal(sa.InitiatorSAInitMsg, sa.LastSentMsg) {
		t.Error("sa.InitiatorSAInitMsg was not re-anchored to the retried request; every later AUTH would be computed over bytes the peer never saw")
	}
	if sa.RetransmitCount != 0 {
		t.Errorf("RetransmitCount = %d, want 0; the retry must restart the retransmit schedule", sa.RetransmitCount)
	}
}

// VALIDATES: a group this node never proposed, and every malformed body, are refused and
// the SA dies without a retry.
// PREVENTS: an off-path attacker who can forge one unauthenticated notify choosing this
// node's Diffie-Hellman group, and a truncated body being read as group 0.
// RFC requirement: RFC7296-1.2-5 negative -- the retry is a GUARDED decision, not
// obedience to an unauthenticated packet. groupIsProposed and parseInvalidKEGroup
// (sa_init_retry.go) both deny before any key is rebuilt.
func TestKegInitiatorRefusesUnofferedGroup(t *testing.T) {
	log := slogutil.DiscardLogger()
	cases := []struct {
		name string
		data []byte
	}{
		// rfc-test-change-approved: 2026-07-31 owner standing approval for
		// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only.
		//
		// Group 20 is the DISCRIMINATING case, and it is the reason this row is proven
		// at all. crypto.NewDHExchange can build 14, 19 and 20, and kegIKEGroup proposes
		// only 14 and 19. A case naming a group crypto cannot build (5, say) is refused
		// by the NewDHExchange error branch whether or not groupIsProposed exists, so it
		// cannot tell the security guard from a capability limit. Mutation-verified:
		// deleting groupIsProposed leaves every other case here green.
		{"a buildable group we never proposed", []byte{0x00, 20}},
		{"a group nothing can build", []byte{0x00, 0x05}},
		{"a one-octet body", []byte{0x13}},
		{"a three-octet body", []byte{0x00, 0x13, 0x00}},
		{"group zero", []byte{0x00, 0x00}},
		{"a value above the octet-wide group number", []byte{0x01, 0x13}},
		{"an empty body", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sa, table := kegInitiator(t, kegIKEGroup())
			first := append([]byte(nil), sa.LastSentMsg...)
			group := sa.LocalDH.GroupID

			resp := kegNotifyResponse(sa.InitiatorSPI, [8]byte{}, wire.NotifyInvalidKEPayload, tc.data)
			handleSAInitResponse(sa, parseMsg(t, resp), resp, table, nil, nil, log)

			if sa.State != StateDead {
				t.Errorf("state = %v, want dead; %s was acted on", sa.State, tc.name)
			}
			if !bytes.Equal(sa.LastSentMsg, first) {
				t.Errorf("a new request was built from %s", tc.name)
			}
			if sa.LocalDH.GroupID != group {
				t.Errorf("the Diffie-Hellman group moved to %d on %s", sa.LocalDH.GroupID, tc.name)
			}
		})
	}
}

// VALIDATES: the retry re-offers every configured suite, with the same Proposal Nums.
// PREVENTS: a retry that offers only the suite carrying the responder's group, which
// would let an attacker who forges one notify pick the cipher as well as the group.
// RFC requirement: RFC7296-1.2-6 positive -- buildSAInitRequest (initiator.go) calls
// buildWireIKEProposals over the whole configured group on every attempt, so the offer is
// never narrowed by the rejection.
func TestKegRetryReproposesEveryConfiguredSuite(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa, table := kegInitiator(t, kegSuiteGroup())

	firstSA := kegPayload[*wire.PayloadSA](t, sa.LastSentMsg)
	if got := len(firstSA.Proposals); got != 3 {
		t.Fatalf("the first attempt offered %d proposals, want 3", got)
	}

	resp := kegNotifyResponse(sa.InitiatorSPI, [8]byte{}, wire.NotifyInvalidKEPayload, []byte{0x00, 19})
	handleSAInitResponse(sa, parseMsg(t, resp), resp, table, nil, nil, log)

	retrySA := kegPayload[*wire.PayloadSA](t, sa.LastSentMsg)
	if retrySA == nil || len(retrySA.Proposals) != 3 {
		t.Fatalf("the retry offered %v proposals, want all 3", len(retrySA.Proposals))
	}
	for i := range retrySA.Proposals {
		if retrySA.Proposals[i].Number != firstSA.Proposals[i].Number {
			t.Errorf("proposal %d is numbered %d on the retry and %d on the first attempt",
				i, retrySA.Proposals[i].Number, firstSA.Proposals[i].Number)
		}
		if len(retrySA.Proposals[i].Transforms) != len(firstSA.Proposals[i].Transforms) {
			t.Errorf("proposal %d changed shape between the attempts", i)
		}
	}
}

// VALIDATES: the retry's SA payload is byte-identical to the first attempt's while its
// KE payload changed.
// PREVENTS: a narrowed offer. Neither assertion says this alone: identical proposals
// could mean nothing was retried, and a changed KE could sit beside a narrowed offer.
// Together they say the group moved and nothing else did.
// RFC requirement: RFC7296-1.2-6 negative -- the rejection is unauthenticated, so it
// steers exactly one field. Narrowing the offer to the responder's named group would be
// the downgrade this MUST exists to stop.
func TestKegRetryOfferIsNotNarrowedByTheNotify(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa, table := kegInitiator(t, kegSuiteGroup())

	firstSABytes := kegRawPayload(t, sa.LastSentMsg, wire.PayloadTypeSA)
	firstKEBytes := kegRawPayload(t, sa.LastSentMsg, wire.PayloadTypeKE)

	resp := kegNotifyResponse(sa.InitiatorSPI, [8]byte{}, wire.NotifyInvalidKEPayload, []byte{0x00, 19})
	handleSAInitResponse(sa, parseMsg(t, resp), resp, table, nil, nil, log)

	retrySABytes := kegRawPayload(t, sa.LastSentMsg, wire.PayloadTypeSA)
	retryKEBytes := kegRawPayload(t, sa.LastSentMsg, wire.PayloadTypeKE)

	if !bytes.Equal(firstSABytes, retrySABytes) {
		t.Error("the retry's SA payload differs from the first attempt's; the offer was narrowed by an unauthenticated notify")
	}
	if bytes.Equal(firstKEBytes, retryKEBytes) {
		t.Error("the retry's KE payload is unchanged, so the corrected group was never applied")
	}
}

// kegRawPayload returns the raw body octets of the first payload of the given type.
func kegRawPayload(t *testing.T, raw []byte, want uint8) []byte {
	t.Helper()
	next := raw[16]
	off := wire.HeaderLen
	end := len(raw)
	for next != 0 && off+wire.GenericHeaderLen <= end {
		length := int(raw[off+2])<<8 | int(raw[off+3])
		if length < wire.GenericHeaderLen || off+length > end {
			t.Fatalf("malformed payload chain at offset %d", off)
		}
		if next == want {
			return raw[off+wire.GenericHeaderLen : off+length]
		}
		next = raw[off]
		off += length
	}
	t.Fatalf("no payload of type %d in the message", want)
	return nil
}

// VALIDATES: an IKE SA whose IKE_SA_INIT was retried still authenticates.
// PREVENTS: the highest-probability defect of this work. RFC 7296 Section 2.15 computes
// AUTH over the first IKE_SA_INIT message, and auth.go reads sa.InitiatorSAInitMsg for
// it. A retry that left the pre-retry bytes there passes EVERY payload-shape assertion
// above and then fails two messages later as an opaque AUTH mismatch. This is the only
// test in the package that can see that.
func TestKegRetriedSAInitStillAuthenticates(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "retry-psk")
	iniGroup := kegIKEGroup()
	respGroup := kegResponderGroup()
	espGroup := testESPGroup()

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, iniGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	req := buildSAInitRequest(ini, iniGroup)
	ini.InitiatorSAInitMsg = req
	ini.LastSentMsg = req
	ini.State = StateSAInitSent
	if ini.LocalDH.GroupID != 14 {
		t.Fatalf("setup: the first guess is group %d, want 14", ini.LocalDH.GroupID)
	}

	// The responder accepts only group 19, so it answers with INVALID_KE_PAYLOAD.
	// handleSAInitRequest is the producer of that notify; it is rebuilt here rather than
	// captured because sendSAInitNotify needs a real socket.
	notify := kegNotifyResponse(ini.InitiatorSPI, [8]byte{}, wire.NotifyInvalidKEPayload, []byte{0x00, 19})
	handleSAInitResponse(ini, parseMsg(t, notify), notify, table, nil, nil, log)
	if ini.State != StateSAInitSent || ini.LocalDH.GroupID != 19 {
		t.Fatalf("the retry did not move to group 19: state=%v group=%d", ini.State, ini.LocalDH.GroupID)
	}
	retried := ini.LastSentMsg

	// A fresh responder now processes the RETRIED request and the handshake runs on.
	resp, err := newResponderSA("ze", respPeer, respGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, retried), retried, nil, nil, log)
	if resp.State != StateSAInitReceived {
		t.Fatalf("the responder refused the retried IKE_SA_INIT: state=%v", resp.State)
	}
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	if ini.State != StateAuthSent {
		t.Fatalf("the initiator did not reach IKE_AUTH after the retried exchange: state=%v", ini.State)
	}

	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: respGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	handleAuthResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, log)

	if ini.State != StateEstablished || resp.State != StateEstablished {
		t.Fatalf("a retried IKE_SA_INIT did not authenticate: ini=%v resp=%v", ini.State, resp.State)
	}
}
