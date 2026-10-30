package engine

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// The round-2 credentials of the Ze-to-Ze tests. Each direction has its own key and
// identity, and neither matches round 1 ("round-one-secret", the peer name "ze"), so
// a reader that takes the wrong round's credential fails its verification.
const (
	maIniRoundTwoID   = "ini-user.example.com"
	maRespRoundTwoID  = "resp-user.example.com"
	maIniRoundTwoPSK  = "initiator-round-two"
	maRespRoundTwoPSK = "responder-round-two"
	maRoundOnePSK     = "round-one-secret"
	maVerifiedMessage = "ike: second authentication round verified"
)

// maAuth is a pre-shared-secret round 1 with the given second rounds. A nil round is
// not configured, and with both nil the peer has no second-round container at all.
func maAuth(local, remote *ipsec.AuthConfig) ipsec.AuthConfig {
	auth := ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: maRoundOnePSK}
	if local == nil && remote == nil {
		return auth
	}
	auth.Additions = &ipsec.AuthAdditions{SecondRound: &ipsec.SecondRoundAuth{Local: local, Remote: remote}}
	return auth
}

// The four round-2 directions of a mirrored Ze-to-Ze pair: what the initiator proves
// is what the responder requires, and the other way round.
func maIniLocal() *ipsec.AuthConfig {
	return &ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: maIniRoundTwoPSK, LocalID: maIniRoundTwoID}
}

func maIniRemote() *ipsec.AuthConfig {
	return &ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: maRespRoundTwoPSK, RemoteID: maRespRoundTwoID}
}

func maRespLocal() *ipsec.AuthConfig {
	return &ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: maRespRoundTwoPSK, LocalID: maRespRoundTwoID}
}

func maRespRemote() *ipsec.AuthConfig {
	return &ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: maIniRoundTwoPSK, RemoteID: maIniRoundTwoID}
}

// maPair is one Ze initiator and one Ze responder after IKE_SA_INIT, driven message
// by message through the production handlers. peerTr receives what Ze sends through
// myTr, so a refusal that never reaches LastSentMsg can still be read off the wire.
type maPair struct {
	t         *testing.T
	ini, resp *SA
	ps        *PeerSession
	table     *SATable
	peerTr    *transport.UDPTransport
	myTr      *transport.UDPTransport
	peerAddr  *net.UDPAddr
	log       *slog.Logger
	logs      *bytes.Buffer
}

// maSAInitPair runs IKE_SA_INIT between an initiator configured with iniAuth and a
// responder configured with respAuth. Every log line of both, the package logger
// included, lands in p.logs.
func maSAInitPair(t *testing.T, iniAuth, respAuth ipsec.AuthConfig) *maPair {
	t.Helper()
	p := &maPair{t: t, logs: &bytes.Buffer{}, table: NewSATable()}
	p.log = slog.New(slog.NewTextHandler(p.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	previous := getLogger()
	setLogger(p.log)
	t.Cleanup(func() { setLogger(previous) })
	p.peerTr, p.myTr = rtxPeerLink(t)
	addr, ok := p.peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	p.peerAddr = addr

	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, _ := autPeers(iniAuth)
	_, respPeer := autPeers(respAuth)
	// One literal both ends, as tdnAbandonedAuth uses: rtxPeerLink points the
	// initiator's remote address at peerTr, and single-address selectors stay valid.
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"

	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	p.table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, p.log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, p.table, nil, nil, p.log)
	if resp.State != StateSAInitReceived || len(ini.LastSentMsg) == 0 {
		t.Fatalf("IKE_SA_INIT did not complete: resp=%v, initiator IKE_AUTH bytes=%d", resp.State, len(ini.LastSentMsg))
	}
	p.ini, p.resp = ini, resp
	p.ps = &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	return p
}

// respondWith hands one request to the responder through its dispatch entry point,
// so the state routing of handleResponderInbound is exercised too. With wired set,
// the responder answers through myTr to peerTr.
func (p *maPair) respondWith(raw []byte, wired bool) {
	p.t.Helper()
	var tr *transport.UDPTransport
	var remote *net.UDPAddr
	if wired {
		tr, remote = p.myTr, p.peerAddr
	}
	p.ps.handleResponderInbound(p.resp, parseMsg(p.t, raw), transport.Packet{Data: raw, RemoteAddr: remote}, tr, p.log)
}

// answerWith hands one response to the initiator. With wired set, a teardown it sends
// goes through myTr to peerTr.
func (p *maPair) answerWith(raw []byte, wired bool) {
	p.t.Helper()
	var tr *transport.UDPTransport
	if wired {
		tr = p.myTr
	}
	handleAuthResponse(p.ini, parseMsg(p.t, raw), raw, p.table, tr, p.log)
}

// sentAuthFailed reads the next datagram Ze sent, decrypts it with the other side's
// SA, and fails unless it carries one AUTHENTICATION_FAILED notify.
func (p *maPair) sentAuthFailed(other *SA) {
	msgType := wire.NotifyAuthenticationFailed
	p.t.Helper()
	raw := rtxRecv(p.t, p.peerTr)
	if raw == nil {
		p.t.Fatalf("Ze sent nothing; it owed the peer a notify of type %d", msgType)
	}
	inner := lcyDecrypt(p.t, other, raw)
	if got := rfc4739Notifies(inner, msgType); len(got) != 1 {
		p.t.Fatalf("Ze's datagram carries %d notifies of type %s, want 1", len(got), wire.NotifyTypeName(msgType))
	}
}

// maDecrypt decrypts a message the given side sent, read with the other side's keys.
func maDecrypt(t *testing.T, reader *SA, raw []byte) []wire.PayloadEntry {
	t.Helper()
	return lcyDecrypt(t, reader, raw)
}

// maCount counts the payloads of one Go type in a chain, and the ID payloads of one
// direction (idType 0 counts no ID).
func maCount(inner []wire.PayloadEntry) (ids map[uint8]string, auths, sas, tss int) {
	ids = map[uint8]string{}
	for i := range inner {
		switch p := inner[i].Payload.(type) {
		case *wire.PayloadID:
			ids[p.IDPayloadType] = string(p.IDData)
		case *wire.PayloadAUTH:
			auths++
		case *wire.PayloadSA:
			sas++
		case *wire.PayloadTS:
			tss++
		}
	}
	return ids, auths, sas, tss
}

// TestRFC4739ZeToZeTwoRoundsEachWay proves AC-4, AC-5 and AC-6 end to end. Goal: a Ze
// initiator and a Ze responder, each with a second round each way, run four IKE_AUTH
// messages and both establish once every round verified, each round signed over its
// own identity with its own key. Method: the production handlers carry each message
// to the other side, and every message is decrypted and its payloads checked.
func TestRFC4739ZeToZeTwoRoundsEachWay(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		iniAuth, respAuth     ipsec.AuthConfig
		iniRoundID, respRound string
		pki                   bool
	}{
		{
			name:       "pre-shared-secret second rounds",
			iniAuth:    maAuth(maIniLocal(), maIniRemote()),
			respAuth:   maAuth(maRespLocal(), maRespRemote()),
			iniRoundID: maIniRoundTwoID, respRound: maRespRoundTwoID,
		},
		{
			name: "x509 second rounds",
			iniAuth: maAuth(
				&ipsec.AuthConfig{Mode: ipsec.AuthX509, Certificate: autCertName, LocalID: autCertName},
				&ipsec.AuthConfig{Mode: ipsec.AuthX509, CACertificate: autCAName, RemoteID: autOtherName}),
			respAuth: maAuth(
				&ipsec.AuthConfig{Mode: ipsec.AuthX509, Certificate: autOtherName, LocalID: autOtherName},
				&ipsec.AuthConfig{Mode: ipsec.AuthX509, CACertificate: autCAName, RemoteID: autCertName}),
			iniRoundID: autCertName, respRound: autOtherName,
			pki: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pki {
				autLoadPKI(t)
			}
			p := maSAInitPair(t, tc.iniAuth, tc.respAuth)

			// Message 3: round 1 of the initiator, the Child SA offer, both notifies.
			req1 := p.ini.LastSentMsg
			p.respondWith(req1, false)
			if p.resp.State != StateAuthReceived {
				t.Fatalf("after the first request the responder is %v, want %v", p.resp.State, StateAuthReceived)
			}

			// Message 4: round 1 of the responder and its ANOTHER_AUTH_FOLLOWS, no Child SA.
			resp1 := p.resp.LastSentMsg
			inner := maDecrypt(t, p.ini, resp1)
			ids, auths, sas, tss := maCount(inner)
			if ids[wire.PayloadTypeIDr] != "ze" || auths != 1 {
				t.Errorf("first response: IDr %q and %d AUTH, want IDr \"ze\" and one AUTH", ids[wire.PayloadTypeIDr], auths)
			}
			if sas != 0 || tss != 0 {
				t.Errorf("first response carries %d SA and %d TS payloads; RFC 4739 answers the Child SA last", sas, tss)
			}
			if got := rfc4739Notifies(inner, wire.NotifyAnotherAuthFollows); len(got) != 1 {
				t.Errorf("first response carries %d ANOTHER_AUTH_FOLLOWS, want 1", len(got))
			}
			p.answerWith(resp1, false)
			if p.ini.State != StateAuthSent || !p.ini.secondAuthExchange {
				t.Fatalf("after the first response the initiator is %v (second exchange %v), want auth-sent in the second exchange",
					p.ini.State, p.ini.secondAuthExchange)
			}

			// Message 5: round 2 of the initiator alone.
			req2 := p.ini.LastSentMsg
			if parseMsg(t, req2).Header.MessageID != 2 {
				t.Errorf("second request message id = %d, want 2", parseMsg(t, req2).Header.MessageID)
			}
			inner = maDecrypt(t, p.resp, req2)
			ids, auths, sas, tss = maCount(inner)
			if ids[wire.PayloadTypeIDi] != tc.iniRoundID || auths != 1 {
				t.Errorf("second request: IDi %q and %d AUTH, want IDi %q and one AUTH", ids[wire.PayloadTypeIDi], auths, tc.iniRoundID)
			}
			if sas != 0 || tss != 0 {
				t.Errorf("second request carries %d SA and %d TS payloads, want none", sas, tss)
			}
			for _, msgType := range []uint16{wire.NotifyMultipleAuthSupported, wire.NotifyAnotherAuthFollows} {
				if got := rfc4739Notifies(inner, msgType); len(got) != 0 {
					t.Errorf("second request carries %d notifies of type %d, want none", len(got), msgType)
				}
			}
			p.respondWith(req2, false)
			if p.resp.State != StateEstablished {
				t.Fatalf("after the second request the responder is %v, want established", p.resp.State)
			}

			// Message 6: round 2 of the responder and the Child SA answer.
			resp2 := p.resp.LastSentMsg
			inner = maDecrypt(t, p.ini, resp2)
			ids, auths, sas, tss = maCount(inner)
			if ids[wire.PayloadTypeIDr] != tc.respRound || auths != 1 {
				t.Errorf("final response: IDr %q and %d AUTH, want IDr %q and one AUTH", ids[wire.PayloadTypeIDr], auths, tc.respRound)
			}
			if sas != 1 || tss != 2 {
				t.Errorf("final response carries %d SA and %d TS payloads, want 1 and 2", sas, tss)
			}
			p.answerWith(resp2, false)
			if p.ini.State != StateEstablished {
				t.Fatalf("after the final response the initiator is %v, want established", p.ini.State)
			}
			if p.ini.ChildOutboundSPI == 0 || p.resp.ChildOutboundSPI == 0 {
				t.Errorf("Child SA SPIs: initiator outbound %#x, responder outbound %#x, want both set",
					p.ini.ChildOutboundSPI, p.resp.ChildOutboundSPI)
			}

			logs := p.logs.String()
			for _, id := range []string{tc.iniRoundID, tc.respRound} {
				if !maLogLine(logs, maVerifiedMessage, "peer=ze", "id="+id) {
					t.Errorf("no %q line names id %s:\n%s", maVerifiedMessage, id, logs)
				}
			}
		})
	}
}

// maLogLine reports whether one log line holds the message and every part.
func maLogLine(logs, message string, parts ...string) bool {
	for line := range strings.SplitSeq(logs, "\n") {
		if !strings.Contains(line, message) {
			continue
		}
		all := true
		for _, part := range parts {
			if !strings.Contains(line, part) {
				all = false
			}
		}
		if all {
			return true
		}
	}
	return false
}

// TestRFC4739ResponderAnotherAuthFollows proves AC-5's notify. Goal: the responder's
// first IKE_AUTH response, which carries its AUTH, carries one ANOTHER_AUTH_FOLLOWS
// with every field the RFC fixes at zero. Method: the real first exchange of a
// responder with a second local round, the response decrypted with the initiator's
// keys and its notify octets read off the decrypted chain and through the parser.
func TestRFC4739ResponderAnotherAuthFollows(t *testing.T) {
	p := maSAInitPair(t, maAuth(nil, maIniRemote()), maAuth(maRespLocal(), nil))
	p.respondWith(p.ini.LastSentMsg, false)
	inner := maDecrypt(t, p.ini, p.resp.LastSentMsg)
	if !rfc4555HasAuth(inner) {
		t.Fatal("the first IKE_AUTH response carries no AUTH payload")
	}

	// RFC requirement: RFC4739-3.2-1 positive -- the first IKE_AUTH response, built by
	// answerFirstAuthExchange for a responder with a second local round, carries an AUTH
	// payload and exactly one ANOTHER_AUTH_FOLLOWS notify (16405) whose parsed Protocol ID
	// and SPI Size are 0 and which holds no SPI and no Notification Data.
	follows := rfc4739Notifies(inner, wire.NotifyAnotherAuthFollows)
	if len(follows) != 1 {
		t.Fatalf("first IKE_AUTH response carries %d ANOTHER_AUTH_FOLLOWS notifies, want 1", len(follows))
	}
	rfc4739CheckEmptyNotify(t, "ANOTHER_AUTH_FOLLOWS", follows[0])

	// The same notify read as octets off the decrypted chain, not through the parser.
	raw := p.resp.LastSentMsg
	var sk *wire.PayloadSK
	for _, pe := range parseMsg(t, raw).Payloads {
		if s, ok := pe.Payload.(*wire.PayloadSK); ok {
			sk = s
		}
	}
	if sk == nil {
		t.Fatal("the first IKE_AUTH response carries no SK payload")
	}
	plain, err := decryptSKPayload(p.ini, raw, sk)
	if err != nil {
		t.Fatalf("decrypt the first IKE_AUTH response: %v", err)
	}
	octets := rfc4739ChainNotifies(t, plain, sk.InnerNextPayload, 0, wire.NotifyAnotherAuthFollows)
	if len(octets) != 1 {
		t.Fatalf("decrypted chain carries %d ANOTHER_AUTH_FOLLOWS notifies, want 1", len(octets))
	}
	if len(octets[0]) != 8 {
		t.Errorf("ANOTHER_AUTH_FOLLOWS is %d octets, want 8 (no SPI, no data)", len(octets[0]))
	}
	if octets[0][4] != 0 {
		t.Errorf("ANOTHER_AUTH_FOLLOWS Protocol ID octet = %d, want 0", octets[0][4])
	}
	if octets[0][5] != 0 {
		t.Errorf("ANOTHER_AUTH_FOLLOWS SPI Size octet = %d, want 0", octets[0][5])
	}
}

// TestRFC4739NonFinalResponseDoesNotEstablish proves R-4. Goal: an initiator whose
// first IKE_AUTH response announced another round neither establishes nor adopts a
// Child SA from it. Method: the first exchange runs, and the initiator is inspected
// before the second request is answered.
func TestRFC4739NonFinalResponseDoesNotEstablish(t *testing.T) {
	p := maSAInitPair(t, maAuth(nil, maIniRemote()), maAuth(maRespLocal(), nil))
	p.respondWith(p.ini.LastSentMsg, false)
	p.answerWith(p.resp.LastSentMsg, false)
	if p.ini.State == StateEstablished {
		t.Fatal("the initiator established on a response that announced another round")
	}
	if p.ini.ChildOutboundSPI != 0 {
		t.Errorf("the initiator adopted Child SA SPI %#x from a non-final response", p.ini.ChildOutboundSPI)
	}
	if p.ini.IKEAuthChildless {
		t.Error("the initiator judged a non-final response as a childless final one")
	}
	// Example 4 of RFC 4739: the initiator, with no round of its own left, sends an
	// IKE_AUTH request with no payload.
	inner := maDecrypt(t, p.resp, p.ini.LastSentMsg)
	if len(inner) != 0 {
		t.Errorf("the initiator's second request carries %d payloads, want none (HDR, SK { })", len(inner))
	}
	p.respondWith(p.ini.LastSentMsg, false)
	p.answerWith(p.resp.LastSentMsg, false)
	if p.ini.State != StateEstablished || p.resp.State != StateEstablished {
		t.Fatalf("after the final exchange ini=%v resp=%v, want both established", p.ini.State, p.resp.State)
	}
}

// TestRFC4739RoundTwoAuthUsesLatestID proves AC-6's latest-ID rule. Goal: a round-2
// AUTH is accepted only when computed over the round-2 IDi it travels with; one
// computed over the round-1 identity, with the round-2 key, is refused. Method: after
// the first exchange the initiator's round-2 AUTH is built once over its round-2
// identity (accepted) and once over its round-1 identity (refused), each sent beside
// the round-2 IDi.
func TestRFC4739RoundTwoAuthUsesLatestID(t *testing.T) {
	for _, tc := range []struct {
		name     string
		signedID string
		accepted bool
	}{
		{"over the round-2 identity", maIniRoundTwoID, true},
		{"over the round-1 identity", "ze", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(nil, maRespRemote()))
			p.respondWith(p.ini.LastSentMsg, false)
			p.answerWith(p.resp.LastSentMsg, false)
			if p.ini.localRound != authRoundSecond {
				t.Fatal("fixture: the initiator did not move to its second round")
			}
			local := p.ini.PeerCfg.Auth.SecondRound().Local
			local.LocalID = tc.signedID
			auth, err := computeLocalAuth(p.ini)
			local.LocalID = maIniRoundTwoID
			if err != nil {
				t.Fatalf("computeLocalAuth: %v", err)
			}
			inner := []wire.PayloadEntry{{Payload: buildIDPayload(p.ini, true)}, {Payload: auth}}
			raw, err := buildEncryptedMessage(p.ini, inner, p.ini.NextMsgID)
			if err != nil {
				t.Fatalf("build the second request: %v", err)
			}
			p.respondWith(raw, true)
			if tc.accepted {
				if p.resp.State != StateEstablished {
					t.Fatalf("an AUTH over the round-2 identity was refused (state %v)", p.resp.State)
				}
				return
			}
			if p.resp.State != StateDead {
				t.Fatalf("an AUTH over the round-1 identity was accepted (state %v)", p.resp.State)
			}
			p.sentAuthFailed(p.ini)
		})
	}
}

// TestRFC4739RoundTwoCredentialNotCrossed proves AC-6's credential rule in both roles.
// Goal: a round-2 AUTH that only the round-1 key produces is refused, by the
// responder with AUTHENTICATION_FAILED in its response and by the initiator with
// AUTHENTICATION_FAILED in an INFORMATIONAL. Method: the signer's round-2 key is set
// to the round-1 key, so its AUTH is valid under round 1 only.
func TestRFC4739RoundTwoCredentialNotCrossed(t *testing.T) {
	t.Run("responder verifies the initiator", func(t *testing.T) {
		crossed := maIniLocal()
		crossed.PSK = maRoundOnePSK
		p := maSAInitPair(t, maAuth(crossed, nil), maAuth(nil, maRespRemote()))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, false)
		p.respondWith(p.ini.LastSentMsg, true)
		if p.resp.State != StateDead {
			t.Fatalf("a round-2 AUTH under the round-1 key was accepted (state %v)", p.resp.State)
		}
		p.sentAuthFailed(p.ini)
	})
	t.Run("initiator verifies the responder", func(t *testing.T) {
		crossed := maRespLocal()
		crossed.PSK = maRoundOnePSK
		p := maSAInitPair(t, maAuth(nil, maIniRemote()), maAuth(crossed, nil))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, false)
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, true)
		if p.ini.State != StateDead {
			t.Fatalf("a round-2 AUTH under the round-1 key was accepted (state %v)", p.ini.State)
		}
		p.sentAuthFailed(p.resp)
	})
}

// TestRFC4739UnconfiguredRemoteRoundRefused proves AC-7 in both roles. Goal: a peer
// that announces ANOTHER_AUTH_FOLLOWS to a Ze side with no second-round remote is
// refused with AUTHENTICATION_FAILED and nothing establishes. Method: one side gets a
// local second round, the other a configuration that announces support but requires
// no round from it.
func TestRFC4739UnconfiguredRemoteRoundRefused(t *testing.T) {
	t.Run("responder", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(maRespLocal(), nil))
		p.respondWith(p.ini.LastSentMsg, true)
		if p.resp.State != StateDead {
			t.Fatalf("the responder accepted an unconfigured round (state %v)", p.resp.State)
		}
		p.sentAuthFailed(p.ini)
		if !maLogLine(p.logs.String(), "IKE_AUTH request refused", "authentication second-round remote") {
			t.Errorf("the refusal does not name the leaf:\n%s", p.logs.String())
		}
	})
	t.Run("initiator", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(maRespLocal(), maRespRemote()))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, true)
		if p.ini.State != StateDead {
			t.Fatalf("the initiator accepted an unconfigured round (state %v)", p.ini.State)
		}
		p.sentAuthFailed(p.resp)
	})
}

// TestRFC4739MissingRemoteRoundRefused proves AC-8 in both roles. Goal: a Ze side
// with a second-round remote refuses a peer that finishes without that round, and the
// log names the leaf. Method: the peer has no second round at all; then, for the
// responder, a peer that announces the round and sends an empty second request.
func TestRFC4739MissingRemoteRoundRefused(t *testing.T) {
	t.Run("responder, round never announced", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(nil, nil), maAuth(nil, maRespRemote()))
		p.respondWith(p.ini.LastSentMsg, true)
		if p.resp.State != StateDead {
			t.Fatalf("the responder established without the required round (state %v)", p.resp.State)
		}
		p.sentAuthFailed(p.ini)
		if !maLogLine(p.logs.String(), "IKE_AUTH request refused", "without the second round",
			"authentication second-round remote") {
			t.Errorf("the refusal does not name the missing round and its leaf:\n%s", p.logs.String())
		}
	})
	t.Run("responder, announced round never sent", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(nil, maRespRemote()))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, false)
		empty, err := buildEncryptedMessage(p.ini, nil, p.ini.NextMsgID)
		if err != nil {
			t.Fatalf("build an empty second request: %v", err)
		}
		p.respondWith(empty, true)
		if p.resp.State != StateDead {
			t.Fatalf("the responder established without the announced round (state %v)", p.resp.State)
		}
		p.sentAuthFailed(p.ini)
	})
	t.Run("initiator", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(nil, maIniRemote()), maAuth(nil, nil))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, true)
		if p.ini.State != StateDead {
			t.Fatalf("the initiator established without the required round (state %v)", p.ini.State)
		}
		p.sentAuthFailed(p.resp)
		if !maLogLine(p.logs.String(), "IKE_AUTH response refused", "authentication second-round remote") {
			t.Errorf("the refusal does not name the leaf:\n%s", p.logs.String())
		}
	})
}

// TestRFC4739PeerWithoutSupportSkipsLocalRound proves AC-9 in both roles. Goal: a Ze
// side with a second local round, facing a peer that never announced support,
// completes in one exchange, sends no ANOTHER_AUTH_FOLLOWS and warns once. Method: the
// whole handshake runs against a peer with no second round.
func TestRFC4739PeerWithoutSupportSkipsLocalRound(t *testing.T) {
	for _, tc := range []struct {
		name              string
		iniAuth, respAuth ipsec.AuthConfig
	}{
		{"initiator", maAuth(maIniLocal(), nil), maAuth(nil, nil)},
		{"responder", maAuth(nil, nil), maAuth(maRespLocal(), nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := maSAInitPair(t, tc.iniAuth, tc.respAuth)
			p.respondWith(p.ini.LastSentMsg, false)
			for _, raw := range [][]byte{p.ini.LastSentMsg, p.resp.LastSentMsg} {
				reader := p.ini
				if parseMsg(t, raw).Header.Flags&wire.FlagResponse == 0 {
					reader = p.resp
				}
				if got := rfc4739Notifies(maDecrypt(t, reader, raw), wire.NotifyAnotherAuthFollows); len(got) != 0 {
					t.Errorf("a message carries ANOTHER_AUTH_FOLLOWS to a peer without support")
				}
			}
			p.answerWith(p.resp.LastSentMsg, false)
			if p.ini.State != StateEstablished || p.resp.State != StateEstablished {
				t.Fatalf("ini=%v resp=%v, want both established after one exchange", p.ini.State, p.resp.State)
			}
			const warning = "the peer does not support RFC 4739 multiple authentication"
			if n := strings.Count(p.logs.String(), warning); n != 1 {
				t.Errorf("the skipped-round warning appears %d times, want 1:\n%s", n, p.logs.String())
			}
		})
	}
}

// TestRFC4739ThirdRoundRefused proves AC-10 in both roles. Goal: a second-round
// message that announces ANOTHER_AUTH_FOLLOWS again is refused, because no third
// round exists. Method: the real round-2 message of the peer is rebuilt with the
// notification added.
func TestRFC4739ThirdRoundRefused(t *testing.T) {
	t.Run("responder", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(nil, maRespRemote()))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, false)
		inner := append(maDecrypt(t, p.resp, p.ini.LastSentMsg),
			wire.PayloadEntry{Payload: anotherAuthFollowsNotify()})
		raw, err := buildEncryptedMessage(p.ini, inner, p.ini.NextMsgID)
		if err != nil {
			t.Fatalf("build the second request: %v", err)
		}
		p.respondWith(raw, true)
		if p.resp.State != StateDead {
			t.Fatalf("the responder accepted a third round (state %v)", p.resp.State)
		}
		p.sentAuthFailed(p.ini)
		if !maLogLine(p.logs.String(), "second IKE_AUTH request refused", "third authentication round") {
			t.Errorf("the refusal does not name the third round:\n%s", p.logs.String())
		}
	})
	t.Run("initiator", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(nil, maIniRemote()), maAuth(maRespLocal(), nil))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, false)
		p.respondWith(p.ini.LastSentMsg, false)
		inner := append(maDecrypt(t, p.ini, p.resp.LastSentMsg),
			wire.PayloadEntry{Payload: anotherAuthFollowsNotify()})
		raw, err := buildEncryptedMessageEx(p.resp, inner, p.ini.NextMsgID, wire.ExchangeIKEAuth, wire.FlagResponse)
		if err != nil {
			t.Fatalf("build the final response: %v", err)
		}
		p.answerWith(raw, true)
		if p.ini.State != StateDead {
			t.Fatalf("the initiator accepted a third round (state %v)", p.ini.State)
		}
		p.sentAuthFailed(p.resp)
	})
}

// TestRFC4739RoundTwoRetransmitReplayed proves AC-12. Goal: a retransmitted IKE_AUTH
// request is answered from the cached response and never processed twice, both the
// first request while the second exchange is pending and the second once the SA
// established. Method: each request is delivered twice through the dispatch entry
// point, over the wire, and the replay is compared byte for byte with the cache.
func TestRFC4739RoundTwoRetransmitReplayed(t *testing.T) {
	p := maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(nil, maRespRemote()))
	req1 := p.ini.LastSentMsg
	p.respondWith(req1, true)
	first := rtxRecv(t, p.peerTr)
	if first == nil || !bytes.Equal(first, p.resp.LastSentMsg) {
		t.Fatal("the first response did not go out")
	}

	p.respondWith(req1, true)
	if p.resp.State != StateAuthReceived {
		t.Fatalf("a retransmitted first request moved the responder to %v", p.resp.State)
	}
	if replay := rtxRecv(t, p.peerTr); !bytes.Equal(replay, first) {
		t.Fatal("a retransmitted first request was not answered with the cached response")
	}

	p.answerWith(first, false)
	req2 := p.ini.LastSentMsg
	p.respondWith(req2, true)
	final := rtxRecv(t, p.peerTr)
	if p.resp.State != StateEstablished || final == nil {
		t.Fatalf("the second request did not establish the responder (state %v)", p.resp.State)
	}
	p.respondWith(req2, true)
	if replay := rtxRecv(t, p.peerTr); !bytes.Equal(replay, final) {
		t.Fatal("a retransmitted second request was not answered with the cached final response")
	}
	if p.resp.State != StateEstablished {
		t.Fatalf("a retransmitted second request moved the responder to %v", p.resp.State)
	}
}

// maTampered returns a copy of raw with its last octet flipped, which breaks the
// integrity check of the SK payload while every header field stays valid.
func maTampered(raw []byte) []byte {
	out := append([]byte(nil), raw...)
	out[len(out)-1] ^= 0xff
	return out
}

// TestRFC4739SecondExchangeGuards proves the three guards of the second IKE_AUTH
// exchange that keep an unauthenticated or stale message from changing the SA. Goal:
// a second-exchange message that fails its integrity check, a responder request at a
// message id other than the expected one, and a late copy of the first response at
// the initiator are each dropped: the SA keeps its state, sends nothing, and the
// genuine message still completes the exchange. Method: each forged or stale
// datagram goes through the production handler over the wire, then a sentinel shows
// nothing was written, then the genuine message is delivered.
func TestRFC4739SecondExchangeGuards(t *testing.T) {
	t.Run("responder drops a request that fails its integrity check", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(nil, maRespRemote()))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, false)
		req2 := p.ini.LastSentMsg
		p.respondWith(maTampered(req2), true)
		if p.resp.State != StateAuthReceived {
			t.Fatalf("a tampered second request moved the responder to %v", p.resp.State)
		}
		rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "tampered second request")
		p.respondWith(req2, false)
		if p.resp.State != StateEstablished {
			t.Fatalf("the genuine second request after a tampered one left the responder %v", p.resp.State)
		}
	})
	t.Run("responder drops a request at an unexpected message id", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(nil, maRespRemote()))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, false)
		inner := maDecrypt(t, p.resp, p.ini.LastSentMsg)
		stray, err := buildEncryptedMessage(p.ini, inner, p.ini.NextMsgID+5)
		if err != nil {
			t.Fatalf("build the stray request: %v", err)
		}
		p.respondWith(stray, true)
		if p.resp.State != StateAuthReceived {
			t.Fatalf("a request at message id %d moved the responder to %v", p.ini.NextMsgID+5, p.resp.State)
		}
		rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "request at an unexpected message id")
		p.respondWith(p.ini.LastSentMsg, false)
		if p.resp.State != StateEstablished {
			t.Fatalf("the genuine second request after a stray one left the responder %v", p.resp.State)
		}
	})
	t.Run("initiator drops a late copy of the first response", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(nil, maIniRemote()), maAuth(maRespLocal(), nil))
		p.respondWith(p.ini.LastSentMsg, false)
		resp1 := p.resp.LastSentMsg
		p.answerWith(resp1, false)
		req2 := p.ini.LastSentMsg
		p.answerWith(resp1, true)
		if p.ini.State != StateAuthSent || !bytes.Equal(p.ini.LastSentMsg, req2) {
			t.Fatalf("a late first response moved the initiator to %v or replaced its second request", p.ini.State)
		}
		rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "late first response")
		p.respondWith(req2, false)
		p.answerWith(p.resp.LastSentMsg, false)
		if p.ini.State != StateEstablished {
			t.Fatalf("the genuine final response after a late first one left the initiator %v", p.ini.State)
		}
	})
	t.Run("initiator drops a response that fails its integrity check", func(t *testing.T) {
		p := maSAInitPair(t, maAuth(nil, maIniRemote()), maAuth(maRespLocal(), nil))
		p.respondWith(p.ini.LastSentMsg, false)
		p.answerWith(p.resp.LastSentMsg, false)
		p.respondWith(p.ini.LastSentMsg, false)
		resp2 := p.resp.LastSentMsg
		p.answerWith(maTampered(resp2), true)
		if p.ini.State != StateAuthSent {
			t.Fatalf("a tampered final response moved the initiator to %v", p.ini.State)
		}
		rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "tampered final response")
		p.answerWith(resp2, false)
		if p.ini.State != StateEstablished {
			t.Fatalf("the genuine final response after a tampered one left the initiator %v", p.ini.State)
		}
	})
}

// TestRFC4739RoundTwoRefusalNamesTheRoundLeaf proves that a refusal of the peer's
// second round names the second-round leaf, never the first round's. Goal: the
// operator is pointed at `authentication second-round remote id`. Method: the
// initiator's round-2 identity differs from the one the responder's second round
// requires, and the logged refusal is read.
func TestRFC4739RoundTwoRefusalNamesTheRoundLeaf(t *testing.T) {
	wrong := maIniLocal()
	wrong.LocalID = "someone-else.example.com"
	p := maSAInitPair(t, maAuth(wrong, nil), maAuth(nil, maRespRemote()))
	p.respondWith(p.ini.LastSentMsg, false)
	p.answerWith(p.resp.LastSentMsg, false)
	p.respondWith(p.ini.LastSentMsg, true)
	if p.resp.State != StateDead {
		t.Fatalf("a round-2 identity the second round does not require was accepted (state %v)", p.resp.State)
	}
	p.sentAuthFailed(p.ini)
	logs := p.logs.String()
	if !maLogLine(logs, "second IKE_AUTH request refused", "authentication second-round remote id expects") {
		t.Fatalf("the refusal does not name the second-round leaf:\n%s", logs)
	}
	if maLogLine(logs, "second IKE_AUTH request refused", "and remote-id expects") {
		t.Fatalf("the refusal names the first round's remote-id:\n%s", logs)
	}
}
