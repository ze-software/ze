// Design: docs/architecture/l2tp/subscriber-session-model.md -- handler registry tests

package subscriber

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func clearAuthHandlersForTest(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		UnregisterAuthHandler(AuthPriorityRemote)
		UnregisterAuthHandler(AuthPriorityLocal)
	})
}

func TestAuthHandlerRegistration(t *testing.T) {
	clearAuthHandlersForTest(t)

	if GetAuthHandler() != nil {
		t.Fatal("expected nil auth handler before registration")
	}

	called := false
	RegisterAuthHandler(AuthPriorityLocal, func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		called = true
		return AuthResult{Accept: true}
	})

	h := GetAuthHandler()
	if h == nil {
		t.Fatal("expected non-nil auth handler after registration")
	}

	h(ppp.EventAuthRequest{}, nil)
	if !called {
		t.Fatal("auth handler was not called")
	}

	UnregisterAuthHandler(AuthPriorityLocal)
	if GetAuthHandler() != nil {
		t.Fatal("expected nil after unregister")
	}
}

func TestAuthHandlerNilIgnored(t *testing.T) {
	clearAuthHandlersForTest(t)

	called := false
	RegisterAuthHandler(AuthPriorityLocal, func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		called = true
		return AuthResult{}
	})
	RegisterAuthHandler(AuthPriorityLocal, nil)

	h := GetAuthHandler()
	if h == nil {
		t.Fatal("nil registration should not clear existing handler")
	}
	h(ppp.EventAuthRequest{}, nil)
	if !called {
		t.Fatal("original handler should still be active")
	}
}

// VALIDATES: AuthPriorityRemote wins over AuthPriorityLocal when local
// registers first, and unregistering remote immediately falls back to local.
// PREVENTS: a single-slot auth registry where removing RADIUS leaves no
// handler or requires snapshotting a prior closure.
func TestAuthHandlerPrecedenceLocalThenRemote(t *testing.T) {
	clearAuthHandlersForTest(t)

	RegisterAuthHandler(AuthPriorityLocal, func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		return AuthResult{Accept: true, Message: "local"}
	})
	RegisterAuthHandler(AuthPriorityRemote, func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		return AuthResult{Accept: true, Message: "radius"}
	})

	got := GetAuthHandler()
	if got == nil {
		t.Fatal("expected active auth handler")
	}
	if res := got(ppp.EventAuthRequest{}, nil); res.Message != "radius" {
		t.Fatalf("got handler %q, want radius", res.Message)
	}

	UnregisterAuthHandler(AuthPriorityRemote)
	got = GetAuthHandler()
	if got == nil {
		t.Fatal("expected local fallback after unregistering remote")
	}
	if res := got(ppp.EventAuthRequest{}, nil); res.Message != "local" {
		t.Fatalf("after removing remote got handler %q, want local", res.Message)
	}

	UnregisterAuthHandler(AuthPriorityLocal)
	if GetAuthHandler() != nil {
		t.Fatal("expected nil when no auth handlers are registered")
	}
}

// VALIDATES: AuthPriorityRemote still wins when AuthPriorityLocal registers
// AFTER AuthPriorityRemote, proving precedence does not depend on init or
// configure timing.
// PREVENTS: last-write-wins where a late local registration silently steals
// subscriber authentication from configured RADIUS.
func TestAuthHandlerPrecedenceRemoteThenLocal(t *testing.T) {
	clearAuthHandlersForTest(t)

	RegisterAuthHandler(AuthPriorityRemote, func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		return AuthResult{Accept: true, Message: "radius"}
	})
	RegisterAuthHandler(AuthPriorityLocal, func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		return AuthResult{Accept: true, Message: "local"}
	})

	got := GetAuthHandler()
	if got == nil {
		t.Fatal("expected active auth handler")
	}
	if res := got(ppp.EventAuthRequest{}, nil); res.Message != "radius" {
		t.Fatalf("remote-then-local got handler %q, want radius", res.Message)
	}

	UnregisterAuthHandler(AuthPriorityRemote)
	got = GetAuthHandler()
	if got == nil {
		t.Fatal("expected local fallback after unregistering remote")
	}
	if res := got(ppp.EventAuthRequest{}, nil); res.Message != "local" {
		t.Fatalf("after removing remote got handler %q, want local", res.Message)
	}
}

// VALIDATES: AuthPriorityUnspecified (zero) is rejected with a BUG panic so
// an unset priority can never claim the lowest numeric value.
func TestAuthHandlerUnspecifiedPriorityPanics(t *testing.T) {
	clearAuthHandlersForTest(t)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for AuthPriorityUnspecified")
		}
	}()
	RegisterAuthHandler(AuthPriorityUnspecified, func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		return AuthResult{}
	})
}

// VALIDATES: concurrent registration, unregistration and lookup of prioritized
// auth handlers are race-free under -race.
func TestAuthHandlerConcurrentAccess(t *testing.T) {
	clearAuthHandlersForTest(t)

	RegisterAuthHandler(AuthPriorityLocal, func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		return AuthResult{Accept: true, Message: "local"}
	})
	remote := func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		return AuthResult{Accept: true, Message: "radius"}
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 200 {
			RegisterAuthHandler(AuthPriorityRemote, remote)
			UnregisterAuthHandler(AuthPriorityRemote)
		}
	})
	wg.Go(func() {
		for range 200 {
			h := GetAuthHandler()
			if h == nil {
				t.Error("local handler should always remain active while remote toggles")
				return
			}
			msg := h(ppp.EventAuthRequest{}, nil).Message
			if msg != "local" && msg != "radius" {
				t.Errorf("unexpected handler message %q", msg)
				return
			}
		}
	})
	wg.Wait()
}

func TestPoolHandlerRegistration(t *testing.T) {
	defer UnregisterPoolHandler()

	if GetPoolHandler() != nil {
		t.Fatal("expected nil pool handler before registration")
	}

	called := false
	RegisterPoolHandler(func(_ ppp.EventIPRequest) ppp.IPResponseArgs {
		called = true
		return ppp.IPResponseArgs{Accept: true}
	})

	h := GetPoolHandler()
	if h == nil {
		t.Fatal("expected non-nil pool handler after registration")
	}
	h(ppp.EventIPRequest{})
	if !called {
		t.Fatal("pool handler was not called")
	}
}

func TestShaperHandlerRegistration(t *testing.T) {
	defer unregisterShaperHandler()

	if GetShaperHandler() != nil {
		t.Fatal("expected nil shaper handler before registration")
	}

	called := false
	RegisterShaperHandler(func(_ string, _, _ uint64) {
		called = true
	})

	h := GetShaperHandler()
	if h == nil {
		t.Fatal("expected non-nil shaper handler after registration")
	}
	h("ppp0", 1000, 500)
	if !called {
		t.Fatal("shaper handler was not called")
	}
}
