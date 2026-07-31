package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Fixtures for rfc7296_errornotify_test.go. They live apart from the tagged tests so a
// fixture edit is never mistaken for a change to an RFC obligation.

// errPair holds an established initiator and responder plus the loopback sockets that
// stand in for the peer.
type errPair struct {
	ini    *SA
	resp   *SA
	ps     *PeerSession
	peerTr *transport.UDPTransport
	myTr   *transport.UDPTransport
	remote *net.UDPAddr
}

// errLink prepares an established pair plus the loopback sockets, with both SAs
// pointed at the peer transport so sendRaw reaches it.
func errLink(t *testing.T) errPair {
	t.Helper()
	ini, resp, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	resp.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := resp.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the responder has no resolvable peer address")
	}
	return errPair{ini: ini, resp: resp, ps: ps, peerTr: peerTr, myTr: myTr, remote: remote}
}

// errNotifyIn returns the notify types carried by a datagram's decrypted inner chain.
func errNotifyIn(t *testing.T, reader *SA, raw []byte) []uint16 {
	t.Helper()
	inner, err := decryptAndParse(reader, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("the peer could not decrypt the answer: %v", err)
	}
	var out []uint16
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok {
			out = append(out, n.NotifyMsgType)
		}
	}
	return out
}

// rkyChildRekeyRequestUnmatched builds a Child SA rekey request whose ESP proposal
// names transforms the responder's esp-group does not offer, so matchOfferedESPProposal
// finds nothing. The refusal is then a real NO_PROPOSAL_CHOSEN rather than the
// malformed-request case, which draws INVALID_SYNTAX instead.
func rkyChildRekeyRequestUnmatched(oldSPI, peerESPSPI uint32, ni []byte) []wire.PayloadEntry {
	inner := rkyChildRekeyRequest(oldSPI, peerESPSPI, ni)
	for i := range inner {
		sa, ok := inner[i].Payload.(*wire.PayloadSA)
		if !ok {
			continue
		}
		for p := range sa.Proposals {
			for tf := range sa.Proposals[p].Transforms {
				// 9999 names no transform any RFC assigns, in any transform type.
				sa.Proposals[p].Transforms[tf].ID = 9999
			}
		}
	}
	return inner
}

// errTruncatedInnerRequest builds an encrypted INFORMATIONAL request whose SK payload
// decrypts and passes its integrity check, and whose PLAINTEXT payload chain is
// malformed. The single payload's generic header names a NEXT payload that is not
// there, so ParsePayloadChain runs out inside the following generic header.
//
// The plaintext is built by hand. buildEncryptedMessageEx always writes a terminating
// NextPayload of zero, so a malformed chain cannot be reached through it.
func errTruncatedInnerRequest(t *testing.T, ini *SA, msgID uint32) []byte {
	t.Helper()
	body := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	total := uint16(wire.GenericHeaderLen + len(body))
	inner := make([]byte, 0, int(total))
	// NextPayload names a second Nonce that no octet follows.
	inner = append(inner, wire.PayloadTypeNonce, 0, byte(total>>8), byte(total))
	inner = append(inner, body...)

	var raw []byte
	var err error
	if ini.Proposal.Encryption.IsAEAD {
		raw, err = buildSKMessageAEADWithMsgID(ini, inner, wire.PayloadTypeNonce, msgID,
			wire.ExchangeInformational, initiatorFlag(ini))
	} else {
		raw, err = buildSKMessageCBCWithMsgID(ini, inner, wire.PayloadTypeNonce, msgID,
			wire.ExchangeInformational, initiatorFlag(ini))
	}
	if err != nil {
		t.Fatalf("build the encrypted request: %v", err)
	}
	return raw
}

// errDriveAuthResponseWithNotify runs handleAuthResponse over an IKE_AUTH response
// carrying IDr, AUTH and one notify of the named type, with no SAr2. It returns the
// initiator SA state the response left behind.
func errDriveAuthResponseWithNotify(t *testing.T, notifyType uint16) SAState {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp := podPair(t)

	idPayload := buildIDPayload(resp, false)
	authPayload, err := computeLocalAuth(resp)
	if err != nil {
		t.Fatalf("compute the responder AUTH: %v", err)
	}
	inner := []wire.PayloadEntry{
		{Payload: idPayload},
		{Payload: authPayload},
		{Payload: &wire.PayloadNotify{NotifyMsgType: notifyType}},
	}
	raw, err := buildEncryptedMessageEx(resp, inner, 1, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the IKE_AUTH response: %v", err)
	}
	handleAuthResponse(ini, parseMsg(t, raw), raw, nil, nil, log)
	return ini.State
}
