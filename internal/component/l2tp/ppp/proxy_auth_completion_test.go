package ppp

import (
	"bytes"
	"crypto/md5" //nolint:gosec // CHAP-MD5 computes the RFC 1994 response from Identifier, secret and Challenge.
	"encoding/binary"
	"testing"
	"time"
)

// TestProxyLCPDisallowedPAPAuthenticatesWithCHAP proves AC-11 through the real
// Driver: refused proxy PAP starts normal LCP, whose fresh CHAP exchange must
// validate the subscriber's secret before CHAP Success and SessionUp appear.
func TestProxyLCPDisallowedPAPAuthenticatesWithCHAP(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 19301)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	const (
		tunnelID = 19
		sessionID = 4
		proxiedPeerMRU = 1400
		actualPeerMRU = 1380
		secret = "proxy-policy-chap-secret"
	)
	proxiedPeer := optStream(mruOpt(proxiedPeerMRU), magicOpt(0x11223344))
	proxiedLocal := optStream(mruOpt(1492), magicOpt(0x01020304), authProtoOpt(authProtoPAP))
	d.SessionsIn() <- StartSession{
		TunnelID: tunnelID,
		SessionID: sessionID,
		ChanFD: 19301,
		UnitFD: 19302,
		UnitNum: 8,
		MaxMRU: 1492,
		AuthMethod: AuthMethodCHAPMD5,
		AuthFallbackOrder: []AuthMethod{AuthMethodCHAPMD5},
		AuthRequired: true,
		DisableIPCP: true,
		DisableIPv6CP: true,
		ProxyLCPInitialRecv: proxiedPeer,
		ProxyLCPLastRecv: proxiedPeer,
		ProxyLCPLastSent: proxiedLocal,
	}

	request := readPolicyLCP(t, pair.peerEnd)
	if request.Code != LCPConfigureRequest {
		t.Fatalf("refused proxy response code=%d, want ConfigureRequest", request.Code)
	}
	opts, err := ParseLCPOptions(request.Data)
	if err != nil {
		t.Fatal(err)
	}
	auth, present := lookupOption(opts, LCPOptAuthProto)
	if !present || len(auth) != 3 {
		t.Fatalf("normal request Auth-Protocol=%x, want CHAP-MD5", auth)
	}
	if method := authMethodFromAuthProto(binary.BigEndian.Uint16(auth), auth[2:]); method != AuthMethodCHAPMD5 {
		t.Fatalf("normal request method=%v, want CHAP-MD5", method)
	}
	if mru, present := lookupMRUOption(opts); !present || mru != 1492 {
		t.Fatalf("normal request MRU=%d, want1492", mru)
	}
	select {
	case ev := <-d.EventsOut():
		t.Fatalf("refused proxy published lifecycle event before negotiation: %+v", ev)
	default:
	}
	select {
	case ev := <-d.AuthEventsOut():
		t.Fatalf("refused proxy published authentication callback: %+v", ev)
	default:
	}

	peerRequest := optStream(mruOpt(actualPeerMRU), magicOpt(0x55667788))
	writePolicyPeerPacket(t, pair.peerEnd, LCPConfigureRequest, 31, peerRequest)
	ack := readPolicyLCP(t, pair.peerEnd)
	if ack.Code != LCPConfigureAck || ack.Identifier != 31 || !bytes.Equal(ack.Data, peerRequest) {
		t.Fatalf("peer's fresh LCP request not acknowledged exactly: %+v", ack)
	}
	writePolicyPeerPacket(t, pair.peerEnd, LCPConfigureAck, request.Identifier, request.Data)
	select {
	case ev := <-d.EventsOut():
		up, ok := ev.(EventLCPUp)
		if !ok {
			t.Fatalf("event=%T, want LCPUp", ev)
		}
		if up.TunnelID != tunnelID || up.SessionID != sessionID || up.NegotiatedMRU != actualPeerMRU || up.NegotiatedAuthMethod != AuthMethodCHAPMD5 {
			t.Fatalf("fresh negotiation published proxy/configured values: %+v", up)
		}
	case <-time.After(time.Second):
		t.Fatal("fresh LCP did not reach Opened")
	}

	proto, challengePacket := readPeerFrame(t, pair.peerEnd)
	if proto != ProtoCHAP || len(challengePacket) < chapHeaderLen+1+chapChallengeValueLen {
		t.Fatalf("expected full CHAP Challenge, proto=%x packet=%x", proto, challengePacket)
	}
	if challengePacket[0] != CHAPCodeChallenge || challengePacket[4] != chapChallengeValueLen {
		t.Fatalf("expected CHAP-MD5 Challenge, got %x", challengePacket)
	}
	identifier := challengePacket[1]
	challenge := challengePacket[chapHeaderLen+1 : chapHeaderLen+1+chapChallengeValueLen]
	var peerInput [1+len(secret)+chapChallengeValueLen]byte
	peerInput[0] = identifier
	copy(peerInput[1:], secret)
	copy(peerInput[1+len(secret):], challenge)
	digest := md5.Sum(peerInput[:])
	if err := pair.peerEnd.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	writeCHAPResponseFrame(t, pair.peerEnd, identifier, digest[:])

	select {
	case ev := <-d.AuthEventsOut():
		req, ok := ev.(EventAuthRequest)
		if !ok {
			t.Fatalf("auth event=%T, want CHAP request", ev)
		}
		if req.Method != AuthMethodCHAPMD5 || req.TunnelID != tunnelID || req.SessionID != sessionID {
			t.Fatalf("proxied PAP or wrong session reached authentication backend: %+v", req)
		}
		if req.Identifier != identifier || req.Username != chapReauthPeerName || !bytes.Equal(req.Challenge, challenge) {
			t.Fatalf("backend received different CHAP identity/challenge: %+v", req)
		}
		// The test backend independently verifies the callback's response using
		// the known subscriber secret, rather than unconditionally accepting it.
		var backendInput [1+len(secret)+chapChallengeValueLen]byte
		backendInput[0] = req.Identifier
		copy(backendInput[1:], secret)
		copy(backendInput[1+len(secret):], req.Challenge)
		expected := md5.Sum(backendInput[:])
		if !bytes.Equal(req.Response, expected[:]) {
			t.Fatalf("backend rejected CHAP digest: got%x, want%x", req.Response, expected)
		}
	case <-time.After(time.Second):
		t.Fatal("CHAP response did not reach authentication backend")
	}
	select {
	case ev := <-d.EventsOut():
		t.Fatalf("network phase published before credential acceptance: %+v", ev)
	default:
	}
	if err := d.AuthResponse(tunnelID, sessionID, true, "verified CHAP", nil); err != nil {
		t.Fatal(err)
	}
	proto, success := readPeerFrame(t, pair.peerEnd)
	if proto != ProtoCHAP || len(success) < chapHeaderLen || success[0] != CHAPCodeSuccess || success[1] != identifier {
		t.Fatalf("expected correlated CHAP Success, proto=%x packet=%x", proto, success)
	}
	select {
	case ev := <-d.AuthEventsOut():
		if _, ok := ev.(eventAuthSuccess); !ok {
			t.Fatalf("auth event=%T, want CHAP success without PAP callback", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted CHAP did not publish authentication success")
	}
	select {
	case ev := <-d.EventsOut():
		up, ok := ev.(EventSessionUp)
		if !ok || up.TunnelID != tunnelID || up.SessionID != sessionID {
			t.Fatalf("event=%+v, want authenticated SessionUp", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted CHAP did not bring session up")
	}
	select {
	case ev := <-d.AuthEventsOut():
		t.Fatalf("unexpected extra authentication callback after CHAP success: %+v", ev)
	default:
	}
}
