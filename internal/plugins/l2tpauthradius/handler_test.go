package l2tpauthradius

import (
	"encoding/binary"
	"net"
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
			resp := make([]byte, radius.HeaderLen)
			resp[0] = code
			resp[1] = buf[1]
			binary.BigEndian.PutUint16(resp[2:4], radius.HeaderLen)

			var reqAuth [radius.AuthenticatorLen]byte
			copy(reqAuth[:], buf[4:4+radius.AuthenticatorLen])
			auth := radius.ResponseAuthenticator(code, buf[1], radius.HeaderLen, reqAuth, nil, sharedKey)
			copy(resp[4:4+radius.AuthenticatorLen], auth[:])

			conn.WriteToUDP(resp, from) //nolint:errcheck // test mock
		}
	}()

	return conn, conn.LocalAddr().String()
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
	a.swapClient(client, "test-nas", addr)

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
	a.swapClient(client, "test-nas", dead.LocalAddr().String())
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
