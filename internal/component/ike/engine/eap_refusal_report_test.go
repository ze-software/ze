// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- the responder's EAP account
//
// An EAP method that refuses a peer owes it a last word: a packet the method
// sends before the EAP-Failure, on a round of its own (MethodResult.FinalRequest,
// internal/core/eap). MS-CHAPv2 sends the RFC 2759 Section 6 Failure message
// there and EAP-TLS sends the RFC 5216 Section 2.1.3 fatal TLS alert, and both
// record the operator's cause in that round rather than the next one.
//
// These two tests hold handleResponderEAP to that timing. The peer decides
// whether the EAP-Failure round ever happens -- strongSwan abandons an EAP-TLS
// exchange after ze's alert -- so a refusal reported only there is a refusal an
// operator can be prevented from reading
// (plan/journal/diagnosis-parked-until-a-round-the-peer-may-never-send.md).
//
// MS-CHAPv2 carries them because it reaches the same last-word path over a
// handshake this package can already build end to end. The EAP-TLS half of the
// same path is proven in internal/core/eap, and against strongSwan in interop
// scenario responder-eap-tls13-revoked-client.

package engine

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/eap"
)

const (
	// eaprefPassword is the password the responder authenticates against, and
	// eaprefWrongPassword is the one the initiator answers the challenge with. The
	// NT-Response computed from the second cannot verify against the first, which
	// is the refusal these tests are about.
	eaprefPassword      = "eap-pass"
	eaprefWrongPassword = "eap-pass-typo"

	// eaprefReport is the line handleResponderEAP writes for a refused peer, and
	// eaprefCause is the fragment of the MS-CHAPv2 method's own diagnosis that has
	// to ride with it (mschapv2Method.sendFailure, internal/core/eap).
	eaprefReport = "ike: EAP authentication failed"
	eaprefCause  = "NT-Response does not match the password"

	// eaprefMaxDeliveries bounds an exchange that must not establish. The refusal
	// arrives within six IKE_AUTH deliveries, and the bound stops a wedged
	// handshake from hanging the package.
	eaprefMaxDeliveries = 12
)

// eaprefExchange is a real IKE_AUTH conversation between a ze initiator and a ze
// responder whose EAP-MSCHAPv2 passwords disagree, with every log line the
// responder writes collected.
type eaprefExchange struct {
	ini    *SA
	resp   *SA
	table  *SATable
	log    *slog.Logger
	logged *bytes.Buffer

	// cur is the datagram waiting to be delivered, and toResponder says which end
	// it is addressed to.
	cur         []byte
	toResponder bool
}

// eaprefRefusedExchange runs IKE_SA_INIT and stops with the initiator's IKE_AUTH
// request waiting for delivery.
//
// It follows autEAPHandshake (rfc7296_auth_test.go) and changes one thing: the
// two ends hold different EAP passwords, so the exchange refuses instead of
// establishing. Nothing here stands in for a method or for a message.
func eaprefRefusedExchange(t *testing.T) *eaprefExchange {
	t.Helper()

	logged := &bytes.Buffer{}
	log := slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	autLoadPKI(t)
	iniPeer, respPeer := autPeers(ipsec.AuthConfig{
		Mode:          ipsec.AuthEAPMSCHAPv2,
		PSK:           eaprefPassword,
		Certificate:   autCertName,
		CACertificate: autCAName,
	})
	iniPeer.Auth.PSK = eaprefWrongPassword

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.setSA(resp)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	// The IKE_SA_INIT pair runs on the discard logger: this exchange's own log is
	// the assertion, and the two messages before EAP starts say nothing about it.
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	logged.Reset()

	return &eaprefExchange{
		ini: ini, resp: resp, table: table, log: log, logged: logged,
		cur: ini.LastSentMsg, toResponder: true,
	}
}

// deliver hands the waiting datagram to the end it is addressed to, and reports
// whether that end was the RESPONDER.
func (e *eaprefExchange) deliver(t *testing.T) bool {
	t.Helper()
	if len(e.cur) == 0 {
		t.Fatal("the end that was due to answer produced no datagram")
	}

	toResponder := e.toResponder
	if toResponder {
		handleInbound(e.resp, transport.Packet{Data: e.cur}, e.table, nil, e.log)
		e.cur = e.resp.LastSentMsg
	} else {
		handleInbound(e.ini, transport.Packet{Data: e.cur}, e.table, nil, e.log)
		e.cur = e.ini.LastSentMsg
	}
	e.toResponder = !toResponder
	return toResponder
}

// cause is the diagnosis the responder's EAP session holds, or nil while the
// exchange is still running.
func (e *eaprefExchange) cause(t *testing.T) error {
	t.Helper()
	sess, ok := e.resp.EAPSession.(*eap.Session)
	if !ok || sess == nil {
		t.Fatalf("the responder holds no EAP authenticator session (%T)", e.resp.EAPSession)
	}
	return sess.Err()
}

// reports counts the refusal lines the responder has written so far.
func (e *eaprefExchange) reports() int {
	return strings.Count(e.logged.String(), eaprefReport)
}

// VALIDATES: the responder writes its refusal, with the method's own cause, on
// the round that sends the method's last word -- before the peer has answered it
// and before any EAP-Failure goes out.
//
// PREVENTS: the state measured on 2026-09-05. handleResponderEAP logged
// sess.Err() under next.Code == eap.CodeFailure alone, and RFC 5216 Section 2.1.3
// puts a round the PEER controls in front of that packet. charon abandons an
// EAP-TLS exchange after ze's fatal alert, so ze's whole account of a revoked
// client certificate was "ike: responder handshake timed out, tearing down",
// thirty seconds later. A refused certificate and a dead network read the same.
func TestEAPRefusalIsReportedBeforeTheEAPFailureRound(t *testing.T) {
	ex := eaprefRefusedExchange(t)

	for range eaprefMaxDeliveries {
		if !ex.deliver(t) {
			continue
		}
		if ex.cause(t) != nil {
			break
		}
		if ex.resp.State == StateDead {
			t.Fatalf("the responder tore the SA down without recording a cause; log:\n%s", ex.logged)
		}
	}

	cause := ex.cause(t)
	if cause == nil {
		t.Fatalf("the responder never refused the initiator within %d deliveries; log:\n%s",
			eaprefMaxDeliveries, ex.logged)
	}

	// The refusal is reported while the exchange is still open, which is the whole
	// point: a peer that walks away from the last word never triggers the round
	// the report used to wait for.
	if ex.resp.State != StateEAPInProgress {
		t.Fatalf("the responder was in %v on the round it recorded the cause, want %v: this test "+
			"needs the report to happen BEFORE the EAP-Failure round", ex.resp.State, StateEAPInProgress)
	}
	if got := ex.reports(); got != 1 {
		t.Fatalf("the responder wrote %d %q lines on the round it recorded %v; log:\n%s",
			got, eaprefReport, cause, ex.logged)
	}
	if !strings.Contains(ex.logged.String(), eaprefCause) {
		t.Fatalf("the report does not carry the method's own cause %q; log:\n%s", eaprefCause, ex.logged)
	}
}

// VALIDATES: a peer that DOES answer the last word draws the EAP-Failure and the
// teardown, and draws no second copy of the refusal line.
//
// PREVENTS: the obvious repair of the defect above, which is to log the cause
// where it is recorded and leave the EAP-Failure round logging it as well. Ze's
// own peer answers the last word (PeerSession.handleMSCHAPv2Failure and
// PeerSession.readAndSendTLS, internal/core/eap), so a ze-to-ze refusal would
// then write the same sentence twice and an operator would read two failures.
func TestEAPRefusalIsReportedOnceWhenThePeerAnswersTheLastWord(t *testing.T) {
	ex := eaprefRefusedExchange(t)

	for range eaprefMaxDeliveries {
		ex.deliver(t)
		if ex.resp.State == StateDead {
			break
		}
	}

	if ex.resp.State != StateDead {
		t.Fatalf("the responder was still in %v after %d deliveries, so the peer never reached "+
			"the EAP-Failure round this test is about; log:\n%s",
			ex.resp.State, eaprefMaxDeliveries, ex.logged)
	}
	if got := ex.reports(); got != 1 {
		t.Fatalf("the responder wrote %d %q lines for one refusal; log:\n%s", got, eaprefReport, ex.logged)
	}
	if !strings.Contains(ex.logged.String(), eaprefCause) {
		t.Fatalf("the report does not carry the method's own cause %q; log:\n%s", eaprefCause, ex.logged)
	}
}
