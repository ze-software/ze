// VALIDATES: RFC 1994 Sections 4.1 and 4.2 across the whole authenticator path
// -- the Response Value a peer sends reaches the registered local verifier
// (l2tp-auth-local, verifyCHAPMD5), and the verdict of its comparison with the
// expected value decides whether the Success (Code 3) or the Failure (Code 4)
// packet goes on the wire.
// PREVENTS: a verifier and a writer that each pass their own unit test while
// the Response Value never reaches the comparison, or the verdict never reaches
// the reply.
//
// The test lives in package ppp_test because l2tp-auth-local imports ppp: an
// internal test importing it would be an import cycle.
//
// RFC: rfc/short/rfc1994.md
package ppp_test

import (
	"crypto/md5" //nolint:gosec // RFC 1994 Section 4.1 defines the Response Value as an MD5 digest
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	authlocal "github.com/ze-software/ze/internal/component/l2tp/plugins/authlocal"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

const (
	chapTestUser   = "bob"
	chapTestSecret = "secret"
)

// pumpLocalAuth answers every auth request the driver emits through handler,
// the way the l2tp subsystem's auth drain does, until the channel closes at
// Driver.Stop.
func pumpLocalAuth(d *ppp.Driver, handler l2tp.AuthHandler) {
	for ev := range d.AuthEventsOut() {
		req, ok := ev.(ppp.EventAuthRequest)
		if !ok {
			continue
		}
		respond := func(accept bool, msg string, blob []byte) error {
			return d.AuthResponse(req.TunnelID, req.SessionID, accept, msg, blob)
		}
		r := handler(req, respond)
		if r.Handled {
			continue
		}
		_ = d.AuthResponse(req.TunnelID, req.SessionID, r.Accept, r.Message, r.AuthResponseBlob) //nolint:errcheck // the wire reply is the assertion
	}
}

// drainSessionEvents empties the driver's lifecycle channel until stop closes,
// so a session never blocks on an event nobody reads.
func drainSessionEvents(d *ppp.Driver, stop <-chan struct{}) {
	for {
		select {
		case <-d.EventsOut():
		case <-stop:
			return
		}
	}
}

// writePeerPacket writes one control packet from the peer end.
func writePeerPacket(t *testing.T, conn net.Conn, proto uint16, code, id uint8, data []byte) {
	t.Helper()
	buf := make([]byte, ppp.MaxFrameLen)
	off := ppp.WriteFrame(buf, 0, proto, nil)
	off += ppp.WriteLCPPacket(buf, off, code, id, data)
	if _, err := conn.Write(buf[:off]); err != nil {
		t.Fatalf("peer write: %v", err)
	}
}

// chapMD5Response is RFC 1994 Section 4.1's Response Value:
// MD5(Identifier || secret || Challenge Value).
func chapMD5Response(id uint8, secret string, challenge []byte) []byte {
	h := md5.New() //nolint:gosec // RFC 1994 Section 4.1 defines the Response Value as an MD5 digest
	h.Write([]byte{id})
	h.Write([]byte(secret))
	h.Write(challenge)
	return h.Sum(nil)
}

// chapReplyThroughLocalVerifier starts an LNS session that authenticates the
// peer with CHAP-MD5 against the local user bob/secret, plays the peer through
// LCP, answers ze's Challenge with a Response computed from peerSecret, and
// returns the Code of the CHAP packet ze sends back.
func chapReplyThroughLocalVerifier(t *testing.T, chanFD int, session uint16, peerSecret string) uint8 {
	t.Helper()
	authlocal.SetUsersForTest(map[string]string{chapTestUser: chapTestSecret})
	t.Cleanup(authlocal.ResetForTest)
	handler := l2tp.GetAuthHandler()
	if handler == nil {
		t.Fatal("no auth handler registered; l2tp-auth-local registers one in init")
	}

	d, peer := ppp.NewPipeDriverForTest(t, chanFD)
	go pumpLocalAuth(d, handler)
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go drainSessionEvents(d, stop)

	d.SessionsIn() <- ppp.StartSession{
		TunnelID:          1,
		SessionID:         session,
		ChanFD:            chanFD,
		UnitFD:            chanFD + 1,
		UnitNum:           int(session),
		LNSMode:           true,
		MaxMRU:            1500,
		AuthMethod:        ppp.AuthMethodCHAPMD5,
		AuthFallbackOrder: []ppp.AuthMethod{ppp.AuthMethodCHAPMD5},
		AuthTimeout:       2 * time.Second,
		DisableIPCP:       true,
		DisableIPv6CP:     true,
	}

	if err := peer.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("peer SetReadDeadline: %v", err)
	}
	buf := make([]byte, ppp.MaxFrameLen)
	sentOwnRequest := false
	for {
		n, err := peer.Read(buf)
		if err != nil {
			t.Fatalf("peer read before ze's CHAP verdict: %v", err)
		}
		proto, payload, _, err := ppp.ParseFrame(buf[:n])
		if err != nil {
			t.Fatalf("peer ParseFrame: %v", err)
		}
		pkt, err := ppp.ParseLCPPacket(payload)
		if err != nil {
			t.Fatalf("peer ParseLCPPacket: %v", err)
		}
		switch proto {
		case ppp.ProtoLCP:
			if pkt.Code != ppp.LCPConfigureRequest {
				continue
			}
			writePeerPacket(t, peer, ppp.ProtoLCP, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data)
			if !sentOwnRequest {
				writePeerPacket(t, peer, ppp.ProtoLCP, ppp.LCPConfigureRequest, 0x70, nil)
				sentOwnRequest = true
			}
		case ppp.ProtoCHAP:
			switch pkt.Code {
			case ppp.CHAPCodeChallenge:
				if len(pkt.Data) < 1 || len(pkt.Data) < 1+int(pkt.Data[0]) {
					t.Fatalf("Challenge Value-Size exceeds the packet: %x", pkt.Data)
				}
				size := int(pkt.Data[0])
				value := chapMD5Response(pkt.Identifier, peerSecret, pkt.Data[1:1+size])
				body := append([]byte{byte(len(value))}, value...)
				body = append(body, chapTestUser...)
				writePeerPacket(t, peer, ppp.ProtoCHAP, ppp.CHAPCodeResponse, pkt.Identifier, body)
			case ppp.CHAPCodeSuccess, ppp.CHAPCodeFailure:
				return pkt.Code
			}
		}
	}
}

// VALIDATES: a Response Value equal to the expected MD5 digest draws Success.
// METHOD: the peer computes its Response Value from bob's configured secret.
//
// RFC requirement: RFC1994-4.2-1 positive -- a CHAP Response whose Value equals MD5(Identifier, the configured secret, the Challenge), judged by the registered l2tp-auth-local verifier, draws a CHAP packet with Code 3 (Success) on the wire.
// RFC requirement: RFC1994-4.2-2 negative -- that equal Response Value does not draw Code 4 (Failure).
// RFC requirement: RFC1994-4.1-3 positive -- the reply ze sends after comparing an equal Response Value is Success, a Success or Failure packet chosen by the comparison.
func TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier(t *testing.T) {
	code := chapReplyThroughLocalVerifier(t, 17001, 71, chapTestSecret)
	if code == ppp.CHAPCodeFailure {
		t.Fatal("Response Value equal to the expected digest drew Failure (Code 4)")
	}
	if code != ppp.CHAPCodeSuccess {
		t.Fatalf("CHAP reply Code = %d, want 3 (Success)", code)
	}
}

// VALIDATES: a Response Value that differs from the expected digest draws
// Failure.
// METHOD: the peer computes its Response Value from a secret other than bob's.
//
// RFC requirement: RFC1994-4.2-2 positive -- a CHAP Response whose Value differs from MD5(Identifier, the configured secret, the Challenge), judged by the registered l2tp-auth-local verifier, draws a CHAP packet with Code 4 (Failure) on the wire.
// RFC requirement: RFC1994-4.2-1 negative -- that differing Response Value does not draw Code 3 (Success).
// RFC requirement: RFC1994-4.1-3 negative -- the comparison, not a fixed answer, chooses the reply: a differing Response Value draws Failure where an equal one draws Success.
func TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier(t *testing.T) {
	code := chapReplyThroughLocalVerifier(t, 17011, 72, "not-the-secret")
	if code == ppp.CHAPCodeSuccess {
		t.Fatal("Response Value different from the expected digest drew Success (Code 3)")
	}
	if code != ppp.CHAPCodeFailure {
		t.Fatalf("CHAP reply Code = %d, want 4 (Failure)", code)
	}
}
