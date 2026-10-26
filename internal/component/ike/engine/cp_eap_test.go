// Design: docs/architecture/ike/ipsec-14-responder.md -- the Configuration payload in IKE_AUTH
// Related: cp.go -- the consumer these tests drive through the EAP rounds
// Related: responder_eap.go -- the first IKE_AUTH keeps the request and the proposal
// VALIDATES: AC-6 and AC-22 over EAP: a CFG_REQUEST in the first IKE_AUTH, or only in the
// final one, is answered once, in the final IKE_AUTH response that carries SAr2, and the
// installed remote selector names the lease.
// PREVENTS: a CFG_REPLY sent before the peer authenticated, a CFG_REPLY in a message with
// no SA payload on the EAP path, and an EAP Child SA installed with the peer's own address.

package engine

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// eapRoundsWithCP drives an EAP-MSCHAPv2 IKE_AUTH between a pool-bound responder and an
// initiator to establishment. It adds cp to the initiator's first IKE_AUTH when inFirst
// is set, and to its final (AUTH-bearing) IKE_AUTH otherwise. It returns every decrypted
// responder IKE_AUTH response in order, the responder SA and its session.
func eapRoundsWithCP(t *testing.T, cp *wire.PayloadCP, inFirst bool) ([][]wire.PayloadEntry, *SA, *PeerSession) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	eapTLSResponderPKI(t, "cp-eap-ca", "cp-eap-cert")
	iniPeer, respPeer := responderTestPeers(ipsec.AuthEAPMSCHAPv2, "s3cr3t-pass")
	iniPeer.Auth.Certificate = "cp-eap-cert"
	iniPeer.Auth.CACertificate = "cp-eap-ca"
	respPeer.Auth.Certificate = "cp-eap-cert"
	respPeer.Auth.CACertificate = "cp-eap-ca"
	respPeer.VirtualIP = &ipsec.VirtualIPBinding{Pool: "rw"}

	table := NewSATable()
	iniSA, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(iniSA)
	saInitReq := buildSAInitRequest(iniSA, ikeGroup)
	iniSA.InitiatorSAInitMsg = saInitReq
	iniSA.State = StateSAInitSent
	respSA, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, iniSA.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup,
		virtualIP: cpVIP(t, cpPool())}
	ps.setSA(respSA)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })
	handleSAInitRequest(respSA, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(iniSA, parseMsg(t, respSA.LastSentMsg), respSA.LastSentMsg, table, nil, nil, log)

	var responses [][]wire.PayloadEntry
	cur := iniSA.LastSentMsg
	injected := false
	for round := range 24 {
		if round%2 == 1 {
			handleInbound(iniSA, transport.Packet{Data: cur}, table, nil, log)
			cur = iniSA.LastSentMsg
		} else {
			cur = withCP(t, iniSA, respSA, cur, cp, round == 0, inFirst, &injected)
			handleInbound(respSA, transport.Packet{Data: cur}, table, nil, log)
			cur = respSA.LastSentMsg
			responses = append(responses, mbDecrypt(t, iniSA, cur))
		}
		if respSA.State == StateDead || iniSA.State == StateDead {
			t.Fatalf("EAP handshake died at round %d (ini=%v resp=%v)", round, iniSA.State, respSA.State)
		}
		if iniSA.State == StateEstablished && respSA.State == StateEstablished {
			if !injected {
				t.Fatal("the CFG_REQUEST was never added to an initiator message")
			}
			return responses, respSA, ps
		}
	}
	t.Fatalf("EAP handshake did not establish (ini=%v resp=%v)", iniSA.State, respSA.State)
	return nil, nil, nil
}

// withCP re-encrypts the initiator's message raw with cp appended when it is the message
// the case targets: the first IKE_AUTH (first and inFirst), or the AUTH-bearing final
// IKE_AUTH (not inFirst). Every other message is returned unchanged.
func withCP(t *testing.T, ini, resp *SA, raw []byte, cp *wire.PayloadCP, first, inFirst bool, injected *bool) []byte {
	t.Helper()
	if inFirst != first {
		return raw
	}
	inner := mbDecrypt(t, resp, raw)
	if !inFirst && !carriesAUTH(inner) {
		return raw
	}
	hdr := parseMsg(t, raw).Header
	out, err := buildEncryptedMessageEx(ini, append(inner, wire.PayloadEntry{Payload: cp}), hdr.MessageID, hdr.ExchangeType, hdr.Flags)
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	*injected = true
	return out
}

// carriesAUTH reports whether a decrypted message holds an AUTH payload.
func carriesAUTH(inner []wire.PayloadEntry) bool {
	for _, e := range inner {
		if _, ok := e.Payload.(*wire.PayloadAUTH); ok {
			return true
		}
	}
	return false
}

// TestCFGReplyInEAPFinalResponse checks AC-6 and AC-22 on the EAP path. For a request in
// the first IKE_AUTH and for one only in the final IKE_AUTH: exactly one responder message
// carries a CP, it is the last one, it is a CFG_REPLY naming a pool address, it precedes
// SAr2 in that message, and the installed remote selector is that address.
func TestCFGReplyInEAPFinalResponse(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inFirst bool
	}{
		{"request in the first IKE_AUTH", true},
		{"request only in the final IKE_AUTH", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			responses, respSA, ps := eapRoundsWithCP(t, cfgRequest(ip4Ask(netip.Addr{})), tc.inFirst)
			final := len(responses) - 1
			for i, entries := range responses {
				cp, cpAt, saAt := replyOf(entries)
				if i != final {
					if cp != nil {
						t.Fatalf("response %d of %d carries a CP before the peer authenticated", i+1, len(responses))
					}
					continue
				}
				if cp == nil {
					t.Fatal("the final IKE_AUTH response carries no CFG_REPLY")
				}
				if cp.CFGType != wire.CFGTypeReply {
					t.Fatalf("CFG type %d, want CFG_REPLY", cp.CFGType)
				}
				if saAt < 0 {
					t.Fatal("the CFG_REPLY is in a message with no SA payload")
				}
				if cpAt > saAt {
					t.Fatalf("CP at %d after SA at %d; RFC 7296 Section 2.19 puts CP before SA", cpAt, saAt)
				}
			}
			cp, _, _ := replyOf(responses[final])
			addrs := attrsOf(cp, wire.CPAttrInternalIP4Address)
			if len(addrs) != 1 || len(addrs[0]) != 4 {
				t.Fatalf("INTERNAL_IP4_ADDRESS values = %v, want one of 4 octets", addrs)
			}
			lease := netip.AddrFrom4([4]byte(addrs[0]))
			if !netip.MustParsePrefix("10.10.0.0/29").Contains(lease) {
				t.Fatalf("leased %v, outside the pool 10.10.0.0/29", lease)
			}
			if got := respSA.VirtualIPs(); len(got) != 1 || got[0] != lease {
				t.Fatalf("the IKE SA holds %v, want the replied %v", got, lease)
			}
			if !respSA.NegotiatedTSi.IP.Equal(net.IP(lease.AsSlice())) {
				t.Fatalf("installed remote selector %v, want the lease %v", respSA.NegotiatedTSi, lease)
			}
			if ps.getChildSA() == nil {
				t.Fatal("the responder installed no Child SA after EAP")
			}
		})
	}
}
