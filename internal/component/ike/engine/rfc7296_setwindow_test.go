// RFC 7296 Section 2.3 SET_WINDOW_SIZE handling on the engine side. The wire codec
// half lives in internal/component/ike/wire/rfc7296_notify_test.go.
//
// Helpers here start with `swz`, so they cannot collide with the sibling RFC files in
// this package.

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// swzWindowNotify builds a SET_WINDOW_SIZE notify whose data is the given octets.
func swzWindowNotify(data []byte) *wire.PayloadNotify {
	return &wire.PayloadNotify{
		NotifyMsgType:    wire.NotifySetWindowSize,
		NotificationData: data,
	}
}

// swzAddNotify re-encrypts an IKE_AUTH message with one extra notify in its payload
// chain. It decrypts with the receiver's keys, appends, and re-encrypts with the
// sender's, so the result is a genuine message the real handler accepts. RFC 7296
// Section 2.15 computes AUTH over the IKE_SA_INIT message and the nonce, never over
// the IKE_AUTH body, so the added payload leaves AUTH verification intact.
func swzAddNotify(t *testing.T, sender, receiver *SA, raw []byte, notify *wire.PayloadNotify, flags uint8) []byte {
	t.Helper()
	msg := parseMsg(t, raw)
	inner, err := decryptAndParse(receiver, msg, raw)
	if err != nil {
		t.Fatalf("decrypt the IKE_AUTH message to rebuild it: %v", err)
	}
	inner = append(inner, wire.PayloadEntry{Payload: notify})
	out, err := buildEncryptedMessageEx(sender, inner, msg.Header.MessageID,
		wire.ExchangeIKEAuth, flags)
	if err != nil {
		t.Fatalf("rebuild the IKE_AUTH message: %v", err)
	}
	return out
}

// swzHandshake runs a PSK handshake and adds notify to the initiator's IKE_AUTH
// request, to the responder's IKE_AUTH response, or to neither. It returns both SAs
// after the exchange each side actually processed.
func swzHandshake(t *testing.T, reqNotify, respNotify *wire.PayloadNotify) (ini, resp *SA) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup, espGroup := testIKEGroup(), testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "setwindow-psk")

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)

	authReq := ini.LastSentMsg
	if reqNotify != nil {
		authReq = swzAddNotify(t, ini, resp, authReq, reqNotify, initiatorFlag(ini))
	}
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, authReq), authReq, nil, nil, log)
	if resp.State == StateDead {
		return ini, resp
	}

	authResp := resp.LastSentMsg
	if respNotify != nil {
		authResp = swzAddNotify(t, resp, ini, authResp, respNotify,
			initiatorFlag(resp)|wire.FlagResponse)
	}
	handleAuthResponse(ini, parseMsg(t, authResp), authResp, table, nil, log)
	return ini, resp
}

// VALIDATES: the window a peer promises is read from its IKE_AUTH and stored, on both
// the responder path and the initiator path.
// PREVENTS: a notification the RFC gives a fixed length being read at any length, and
// a decoder wired into only one of the two IKE_AUTH walks.
//
// RFC requirement: RFC7296-2.3-7 positive -- RFC 7296 Section 2.3 states "The data associated
// with a SET_WINDOW_SIZE notification MUST be 4 octets long and contain the big endian
// representation of the number of messages the sender promises to keep"
// (rfc/full/rfc7296.txt:1447-1449). A 4-octet body carried in a real IKE_AUTH sets
// SA.PeerWindowSize to its big-endian value. handleAuthRequest (responder.go) and
// handleAuthResponse (fsm.go) both call recordPeerWindowSize (msgid.go).
//
// RFC requirement: RFC7296-2.3-7 negative -- a peer that sends no SET_WINDOW_SIZE leaves the
// field at zero. The value above therefore comes from the notification and not from a
// default this side writes on every handshake.
func TestSwzPeerWindowSizeIsRead(t *testing.T) {
	// The responder reads the initiator's promise.
	_, resp := swzHandshake(t, swzWindowNotify([]byte{0x00, 0x00, 0x00, 0x03}), nil)
	if resp.State != StateEstablished {
		t.Fatalf("responder state = %v after a well-formed SET_WINDOW_SIZE, want established", resp.State)
	}
	if resp.PeerWindowSize != 3 {
		t.Errorf("responder PeerWindowSize = %d, want 3", resp.PeerWindowSize)
	}

	// The initiator reads the responder's promise.
	ini, _ := swzHandshake(t, nil, swzWindowNotify([]byte{0x00, 0x00, 0x00, 0x07}))
	if ini.State != StateEstablished {
		t.Fatalf("initiator state = %v after a well-formed SET_WINDOW_SIZE, want established", ini.State)
	}
	if ini.PeerWindowSize != 7 {
		t.Errorf("initiator PeerWindowSize = %d, want 7", ini.PeerWindowSize)
	}

	// A peer that sends none leaves the field at zero, which Section 2.3 reads as one.
	noneIni, noneResp := swzHandshake(t, nil, nil)
	if noneIni.PeerWindowSize != 0 || noneResp.PeerWindowSize != 0 {
		t.Errorf("PeerWindowSize = ini:%d resp:%d with no notify sent, want 0 and 0",
			noneIni.PeerWindowSize, noneResp.PeerWindowSize)
	}
}

// VALIDATES: a SET_WINDOW_SIZE body of any length other than 4 octets ends the IKE_AUTH
// on both paths, and the peer window is not recorded from it.
// PREVENTS: a length check written as a minimum, which accepts a 5-octet body and reads
// a window the peer never stated.
//
// RFC requirement: RFC7296-2.3-7 negative -- the 4-octet rule is a boundary, so 3 octets and 5
// octets are each refused, and so is an empty body. Ze does not silently accept a
// notification whose length RFC 7296 Section 2.3 fixes (ai/rules/protocol.md).
// The IKE_AUTH ends and PeerWindowSize stays at zero on both paths.
func TestSwzMalformedWindowSizeIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"one octet short", []byte{0x00, 0x00, 0x01}},
		{"one octet long", []byte{0x00, 0x00, 0x00, 0x01, 0x00}},
		{"empty", []byte{}},
	} {
		t.Run(tc.name+" on the responder path", func(t *testing.T) {
			_, resp := swzHandshake(t, swzWindowNotify(tc.data), nil)
			if resp.State != StateDead {
				t.Errorf("responder state = %v after a %d-octet SET_WINDOW_SIZE, want dead",
					resp.State, len(tc.data))
			}
			if resp.PeerWindowSize != 0 {
				t.Errorf("responder PeerWindowSize = %d after a refused notify, want 0",
					resp.PeerWindowSize)
			}
		})
		t.Run(tc.name+" on the initiator path", func(t *testing.T) {
			ini, _ := swzHandshake(t, nil, swzWindowNotify(tc.data))
			if ini.State != StateDead {
				t.Errorf("initiator state = %v after a %d-octet SET_WINDOW_SIZE, want dead",
					ini.State, len(tc.data))
			}
			if ini.PeerWindowSize != 0 {
				t.Errorf("initiator PeerWindowSize = %d after a refused notify, want 0",
					ini.PeerWindowSize)
			}
		})
	}
}

// TestSwzRecordPeerWindowSize pins the producer both IKE_AUTH walks call. A nil notify
// is the ordinary case and must leave the field alone.
func TestSwzRecordPeerWindowSize(t *testing.T) {
	sa := testSA()
	if err := recordPeerWindowSize(sa, nil); err != nil {
		t.Fatalf("recordPeerWindowSize(nil) = %v, want nil", err)
	}
	if sa.PeerWindowSize != 0 {
		t.Errorf("PeerWindowSize = %d with no notify, want 0", sa.PeerWindowSize)
	}

	if err := recordPeerWindowSize(sa, swzWindowNotify([]byte{0x00, 0x00, 0x02, 0x00})); err != nil {
		t.Fatalf("recordPeerWindowSize(4 octets) = %v, want nil", err)
	}
	if sa.PeerWindowSize != 512 {
		t.Errorf("PeerWindowSize = %d, want 512", sa.PeerWindowSize)
	}

	err := recordPeerWindowSize(sa, swzWindowNotify([]byte{0x00, 0x00, 0x02}))
	if !errors.Is(err, wire.ErrSetWindowSizeLength) {
		t.Errorf("recordPeerWindowSize(3 octets) = %v, want ErrSetWindowSizeLength", err)
	}
	if sa.PeerWindowSize != 512 {
		t.Errorf("a refused notify changed PeerWindowSize to %d, want the earlier 512",
			sa.PeerWindowSize)
	}
}
