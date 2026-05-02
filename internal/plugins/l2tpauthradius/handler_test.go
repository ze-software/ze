package l2tpauthradius

import (
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
)

type authCall struct {
	accept  bool
	message string
}

type fakeResponder struct {
	mu     sync.Mutex
	calls  []authCall
	waitCh chan struct{}
}

func newFakeResponder() *fakeResponder {
	return &fakeResponder{waitCh: make(chan struct{}, 10)}
}

func (r *fakeResponder) respond(accept bool, message string, _ []byte) error {
	r.mu.Lock()
	r.calls = append(r.calls, authCall{accept, message})
	r.mu.Unlock()
	r.waitCh <- struct{}{}
	return nil
}

func (r *fakeResponder) waitOne(t *testing.T) authCall {
	t.Helper()
	select {
	case <-r.waitCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for respond callback")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[len(r.calls)-1]
}

func startMockRADIUS(t *testing.T, sharedKey []byte, code uint8) (*net.UDPConn, string) {
	return startMockRADIUSWithAttrs(t, sharedKey, code, nil)
}

func startMockRADIUSWithAttrs(t *testing.T, sharedKey []byte, code uint8, attrs []radius.Attr) (*net.UDPConn, string) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		buf := make([]byte, radius.MaxPacketLen)
		for {
			n, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			if n < radius.MinPacketLen {
				continue
			}
			pkt := &radius.Packet{Code: code, Identifier: buf[1], Attrs: attrs}
			resp := make([]byte, radius.MaxPacketLen)
			nResp, encErr := pkt.EncodeTo(resp, 0)
			if encErr != nil {
				continue
			}
			resp = resp[:nResp]

			var reqAuth [radius.AuthenticatorLen]byte
			copy(reqAuth[:], buf[4:4+radius.AuthenticatorLen])
			auth := radius.ResponseAuthenticator(code, buf[1], uint16(nResp), reqAuth, resp[radius.HeaderLen:], sharedKey)
			copy(resp[4:4+radius.AuthenticatorLen], auth[:])

			conn.WriteToUDP(resp, from) //nolint:errcheck // test mock
		}
	}()

	return conn, conn.LocalAddr().String()
}

func setupAuthWithAttrs(t *testing.T, sharedKey []byte, code uint8, attrs []radius.Attr) (*radiusAuth, *fakeResponder, func()) {
	t.Helper()
	conn, addr := startMockRADIUSWithAttrs(t, sharedKey, code, attrs)

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	a := newRADIUSAuth()
	a.swapClient(client, "test-nas", addr, nil)

	resp := newFakeResponder()

	cleanup := func() {
		conn.Close()   //nolint:errcheck // test cleanup
		client.Close() //nolint:errcheck // test cleanup
	}
	return a, resp, cleanup
}

func setupAuth(t *testing.T, sharedKey []byte, code uint8) (*radiusAuth, *fakeResponder, func()) {
	t.Helper()
	conn, addr := startMockRADIUS(t, sharedKey, code)

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	a := newRADIUSAuth()
	a.swapClient(client, "test-nas", addr, nil)

	resp := newFakeResponder()

	cleanup := func() {
		conn.Close()   //nolint:errcheck // test cleanup
		client.Close() //nolint:errcheck // test cleanup
	}
	return a, resp, cleanup
}

func TestRADIUSAuthPAPAccept(t *testing.T) {
	a, resp, cleanup := setupAuth(t, []byte("testing123"), radius.CodeAccessAccept)
	defer cleanup()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 2,
		Method:    ppp.AuthMethodPAP,
		Username:  "alice",
		Response:  []byte("password123"),
	}, resp.respond)

	if !result.Handled {
		t.Fatal("expected Handled=true")
	}

	call := resp.waitOne(t)
	if !call.accept {
		t.Errorf("expected accept, got reject: %s", call.message)
	}
}

func TestRADIUSAuthPAPReject(t *testing.T) {
	a, resp, cleanup := setupAuth(t, []byte("testing123"), radius.CodeAccessReject)
	defer cleanup()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 3,
		Method:    ppp.AuthMethodPAP,
		Username:  "bob",
		Response:  []byte("wrong"),
	}, resp.respond)

	if !result.Handled {
		t.Fatal("expected Handled=true")
	}

	call := resp.waitOne(t)
	if call.accept {
		t.Error("expected reject")
	}
}

func TestRADIUSAuthCHAPAccept(t *testing.T) {
	a, resp, cleanup := setupAuth(t, []byte("testing123"), radius.CodeAccessAccept)
	defer cleanup()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:   1,
		SessionID:  4,
		Method:     ppp.AuthMethodCHAPMD5,
		Identifier: 42,
		Username:   "charlie",
		Challenge:  make([]byte, 16),
		Response:   make([]byte, 16),
	}, resp.respond)

	if !result.Handled {
		t.Fatal("expected Handled=true")
	}

	call := resp.waitOne(t)
	if !call.accept {
		t.Errorf("expected accept, got reject: %s", call.message)
	}
}

func TestRADIUSAuthMSCHAPv2Accept(t *testing.T) {
	a, resp, cleanup := setupAuth(t, []byte("testing123"), radius.CodeAccessAccept)
	defer cleanup()

	mschapResp := make([]byte, 40)

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:   1,
		SessionID:  5,
		Method:     ppp.AuthMethodMSCHAPv2,
		Identifier: 7,
		Username:   "dave",
		Challenge:  make([]byte, 16),
		Response:   mschapResp,
	}, resp.respond)

	if !result.Handled {
		t.Fatal("expected Handled=true")
	}

	call := resp.waitOne(t)
	if !call.accept {
		t.Errorf("expected accept, got reject: %s", call.message)
	}
}

// TestAccessAcceptUnsupportedAttributes verifies deployment-affecting attributes reject until applied.
// VALIDATES: Access-Accept attributes that would affect address, pool, filter, timeout, or rate policy are explicit rejects.
// PREVENTS: RADIUS policy being silently ignored while the session is accepted.
func TestAccessAcceptUnsupportedAttributes(t *testing.T) {
	tests := []struct {
		name string
		attr radius.Attr
		want string
	}{
		{"framed_ip", radius.Attr{Type: radius.AttrFramedIPAddress, Value: net.IPv4(198, 51, 100, 10).To4()}, "Framed-IP-Address"},
		{"framed_pool", radius.Attr{Type: radius.AttrFramedPool, Value: radius.AttrString("pool-a")}, "Framed-Pool"},
		{"filter_id", radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")}, "Filter-Id"},
		{"session_timeout", radius.Attr{Type: radius.AttrSessionTimeout, Value: radius.AttrUint32(3600)}, "Session-Timeout"},
		{"idle_timeout", radius.Attr{Type: radius.AttrIdleTimeout, Value: radius.AttrUint32(300)}, "Idle-Timeout"},
		{"acct_interim", radius.Attr{Type: radius.AttrAcctInterimInterval, Value: radius.AttrUint32(60)}, "Acct-Interim-Interval"},
		{"framed_netmask", radius.Attr{Type: radius.AttrFramedIPNetmask, Value: net.IPv4(255, 255, 255, 255).To4()}, "Framed-IP-Netmask"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := unsupportedAccessAcceptAttribute(&radius.Packet{Attrs: []radius.Attr{tt.attr}})
			if !ok || got != tt.want {
				t.Fatalf("unsupportedAccessAcceptAttribute = %q, %v; want %q, true", got, ok, tt.want)
			}
		})
	}
}

// TestRADIUSAuthAccessAcceptRejectsUnsupportedAttribute verifies unsupported policy rejects the session.
// VALIDATES: RADIUS Access-Accept with unsupported deployment policy returns reject to PPP.
// PREVENTS: Accepting a subscriber while ignoring assigned address/policy attributes.
func TestRADIUSAuthAccessAcceptRejectsUnsupportedAttribute(t *testing.T) {
	a, resp, cleanup := setupAuthWithAttrs(t, []byte("testing123"), radius.CodeAccessAccept, []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: net.IPv4(198, 51, 100, 10).To4()},
	})
	defer cleanup()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 5,
		Method:    ppp.AuthMethodPAP,
		Username:  "erin",
		Response:  []byte("password123"),
	}, resp.respond)

	if !result.Handled {
		t.Fatal("expected Handled=true")
	}

	call := resp.waitOne(t)
	if call.accept {
		t.Fatal("expected reject for unsupported Access-Accept attribute")
	}
	if !strings.Contains(call.message, "unsupported RADIUS Access-Accept attribute: Framed-IP-Address") {
		t.Fatalf("reject message = %q", call.message)
	}
}

// TestAccessAcceptAllowsMSCHAP2Success verifies authentication-only VSA remains accepted.
// VALIDATES: MS-CHAPv2 success blobs are not treated as unsupported policy.
// PREVENTS: Breaking MS-CHAPv2 Access-Accept while rejecting deployment attributes.
func TestAccessAcceptAllowsMSCHAP2Success(t *testing.T) {
	vsa, err := radius.EncodeVSA(radius.VendorMicrosoft, radius.MSCHAP2Success, []byte("S=ok"))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := unsupportedAccessAcceptAttribute(&radius.Packet{Attrs: []radius.Attr{{Type: radius.AttrVendorSpecific, Value: vsa[2:]}}})
	if ok {
		t.Fatalf("MS-CHAP2-Success marked unsupported as %q", got)
	}
}

func TestRADIUSAuthTimeout(t *testing.T) {
	dead, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer dead.Close() //nolint:errcheck // test cleanup

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: dead.LocalAddr().String(), SharedKey: []byte("key")}},
		Timeout: 100 * time.Millisecond,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	a := newRADIUSAuth()
	a.swapClient(client, "test-nas", dead.LocalAddr().String(), nil)
	resp := newFakeResponder()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 6,
		Method:    ppp.AuthMethodPAP,
		Username:  "timeout-user",
		Response:  []byte("pass"),
	}, resp.respond)

	if !result.Handled {
		t.Fatal("expected Handled=true")
	}

	call := resp.waitOne(t)
	if call.accept {
		t.Error("expected reject on timeout")
	}
}

func TestRADIUSHandledSentinel(t *testing.T) {
	a, resp, cleanup := setupAuth(t, []byte("testing123"), radius.CodeAccessAccept)
	defer cleanup()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 7,
		Method:    ppp.AuthMethodPAP,
		Username:  "alice",
		Response:  []byte("pass"),
	}, resp.respond)

	if !result.Handled {
		t.Fatal("RADIUS handler must return Handled=true")
	}
	if result.Accept {
		t.Error("Accept should be false (decision delivered via goroutine)")
	}

	drainResult := l2tp.AuthResult{Handled: result.Handled, Accept: result.Accept}
	if !drainResult.Handled {
		t.Error("drain should see Handled=true")
	}
}

func TestRADIUSAuthNASIPAddress(t *testing.T) {
	sharedKey := []byte("testing123")
	var capturedReq []byte
	var captured sync.Mutex

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() //nolint:errcheck // test cleanup
	addr := conn.LocalAddr().String()

	go func() {
		buf := make([]byte, radius.MaxPacketLen)
		for {
			n, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			if n < radius.MinPacketLen {
				continue
			}
			captured.Lock()
			capturedReq = make([]byte, n)
			copy(capturedReq, buf[:n])
			captured.Unlock()

			resp := make([]byte, radius.HeaderLen)
			resp[0] = radius.CodeAccessAccept
			resp[1] = buf[1]
			binary.BigEndian.PutUint16(resp[2:4], radius.HeaderLen)
			var reqAuth [radius.AuthenticatorLen]byte
			copy(reqAuth[:], buf[4:4+radius.AuthenticatorLen])
			auth := radius.ResponseAuthenticator(radius.CodeAccessAccept, buf[1], radius.HeaderLen, reqAuth, nil, sharedKey)
			copy(resp[4:4+radius.AuthenticatorLen], auth[:])
			conn.WriteToUDP(resp, from) //nolint:errcheck // test mock
		}
	}()

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	srcIP := net.IPv4(10, 20, 30, 40).To4()
	a := newRADIUSAuth()
	a.swapClient(client, "test-nas", addr, srcIP)
	resp := newFakeResponder()

	a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 9,
		Method:    ppp.AuthMethodPAP,
		Username:  "alice",
		Response:  []byte("pass"),
	}, resp.respond)

	resp.waitOne(t)

	captured.Lock()
	raw := capturedReq
	captured.Unlock()

	if raw == nil {
		t.Fatal("no RADIUS request captured")
	}

	pkt, err := radius.Decode(raw)
	if err != nil {
		t.Fatalf("decode captured request: %v", err)
	}

	nasIP := pkt.FindAttr(radius.AttrNASIPAddress)
	if nasIP == nil {
		t.Fatal("NAS-IP-Address attribute not found in RADIUS request")
	}
	if !net.IP(nasIP).Equal(srcIP) {
		t.Errorf("NAS-IP-Address: got %v, want %v", net.IP(nasIP), srcIP)
	}
}

func TestRADIUSNoClient(t *testing.T) {
	a := newRADIUSAuth()
	resp := newFakeResponder()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 8,
		Method:    ppp.AuthMethodPAP,
		Username:  "nobody",
		Response:  []byte("pass"),
	}, resp.respond)

	if result.Handled {
		t.Error("no-client case should not spawn goroutine")
	}
	if result.Accept {
		t.Error("no-client case should reject")
	}
}
