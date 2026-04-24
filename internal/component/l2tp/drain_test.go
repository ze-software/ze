// Design: docs/research/l2tpv2-ze-integration.md -- auth/pool drain goroutines
// Related: handler.go -- AuthHandler, PoolHandler, AuthResult types

package l2tp

import (
	"log/slog"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
)

func TestAuthDrainCallsHandler(t *testing.T) {
	authCh := make(chan ppp.AuthEvent, 1)
	responded := make(chan authDrainResponse, 1)

	called := false
	handler := AuthHandler(func(req ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		called = true
		if req.Username != "alice" {
			t.Errorf("want username alice, got %s", req.Username)
		}
		return AuthResult{Accept: true, Message: "ok"}
	})

	go drainAuth(slog.Default(), authCh, handler, responded)

	authCh <- ppp.EventAuthRequest{TunnelID: 1, SessionID: 2, Username: "alice"}
	close(authCh)

	resp := <-responded
	if !called {
		t.Fatal("handler was not called")
	}
	if !resp.result.Accept {
		t.Fatal("expected accept")
	}
	if resp.tunnelID != 1 || resp.sessionID != 2 {
		t.Fatalf("wrong IDs: %d/%d", resp.tunnelID, resp.sessionID)
	}
}

func TestAuthDrainNilHandlerRejects(t *testing.T) {
	authCh := make(chan ppp.AuthEvent, 1)
	responded := make(chan authDrainResponse, 1)

	go drainAuth(slog.Default(), authCh, nil, responded)

	authCh <- ppp.EventAuthRequest{TunnelID: 1, SessionID: 1}
	close(authCh)

	resp := <-responded
	if resp.result.Accept {
		t.Fatal("nil handler should reject (fail closed)")
	}
}

func TestAuthDrainPanicRecovery(t *testing.T) {
	authCh := make(chan ppp.AuthEvent, 2)
	responded := make(chan authDrainResponse, 2)

	callCount := 0
	handler := AuthHandler(func(req ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		callCount++
		if callCount == 1 {
			panic("boom")
		}
		return AuthResult{Accept: true}
	})

	go drainAuth(slog.Default(), authCh, handler, responded)

	authCh <- ppp.EventAuthRequest{TunnelID: 1, SessionID: 1}
	authCh <- ppp.EventAuthRequest{TunnelID: 1, SessionID: 2}
	close(authCh)

	resp1 := <-responded
	if resp1.result.Accept {
		t.Fatal("panicked request should be rejected")
	}

	resp2 := <-responded
	if !resp2.result.Accept {
		t.Fatal("second request should succeed after panic recovery")
	}
}

func TestAuthDrainHandledSentinel(t *testing.T) {
	authCh := make(chan ppp.AuthEvent, 1)
	responded := make(chan authDrainResponse, 1)

	handler := AuthHandler(func(req ppp.EventAuthRequest, respond AuthRespondFunc) AuthResult {
		go func() {
			if err := respond(true, "async accept", nil); err != nil {
				t.Errorf("respond failed: %v", err)
			}
		}()
		return AuthResult{Handled: true}
	})

	go drainAuth(slog.Default(), authCh, handler, responded)

	authCh <- ppp.EventAuthRequest{TunnelID: 1, SessionID: 2, Username: "bob"}
	close(authCh)

	resp := <-responded
	if !resp.result.Handled {
		t.Fatal("expected Handled=true in drain response")
	}
	if !resp.result.Accept {
		t.Fatal("expected accept via respond callback")
	}
}

func TestPoolDrainCallsHandler(t *testing.T) {
	ipCh := make(chan ppp.IPEvent, 1)
	responded := make(chan poolDrainResponse, 1)

	called := false
	handler := PoolHandler(func(req ppp.EventIPRequest) ppp.IPResponseArgs {
		called = true
		return ppp.IPResponseArgs{Accept: true, Family: req.Family}
	})

	go drainPool(slog.Default(), ipCh, handler, responded)

	ipCh <- ppp.EventIPRequest{TunnelID: 1, SessionID: 2, Family: ppp.AddressFamilyIPv4}
	close(ipCh)

	resp := <-responded
	if !called {
		t.Fatal("handler was not called")
	}
	if !resp.result.Accept {
		t.Fatal("expected accept")
	}
	if resp.tunnelID != 1 || resp.sessionID != 2 {
		t.Fatalf("wrong IDs: %d/%d", resp.tunnelID, resp.sessionID)
	}
}

func TestPoolDrainDefaultReject(t *testing.T) {
	ipCh := make(chan ppp.IPEvent, 1)
	responded := make(chan poolDrainResponse, 1)

	go drainPool(slog.Default(), ipCh, nil, responded)

	ipCh <- ppp.EventIPRequest{TunnelID: 1, SessionID: 1}
	close(ipCh)

	select {
	case resp := <-responded:
		if resp.result.Accept {
			t.Fatal("nil handler should reject")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestPoolDrainPanicRecovery(t *testing.T) {
	ipCh := make(chan ppp.IPEvent, 2)
	responded := make(chan poolDrainResponse, 2)

	callCount := 0
	handler := PoolHandler(func(req ppp.EventIPRequest) ppp.IPResponseArgs {
		callCount++
		if callCount == 1 {
			panic("boom")
		}
		return ppp.IPResponseArgs{Accept: true}
	})

	go drainPool(slog.Default(), ipCh, handler, responded)

	ipCh <- ppp.EventIPRequest{TunnelID: 1, SessionID: 1}
	ipCh <- ppp.EventIPRequest{TunnelID: 1, SessionID: 2}
	close(ipCh)

	resp1 := <-responded
	if resp1.result.Accept {
		t.Fatal("panicked request should be rejected")
	}

	resp2 := <-responded
	if !resp2.result.Accept {
		t.Fatal("second request should succeed after panic recovery")
	}
}
