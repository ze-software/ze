// Design: plan/learned/760-subscriber-session-model.md -- handler registry tests

package subscriber

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func TestAuthHandlerRegistration(t *testing.T) {
	defer UnregisterAuthHandler()

	if GetAuthHandler() != nil {
		t.Fatal("expected nil auth handler before registration")
	}

	called := false
	RegisterAuthHandler(func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
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

	UnregisterAuthHandler()
	if GetAuthHandler() != nil {
		t.Fatal("expected nil after unregister")
	}
}

func TestAuthHandlerNilIgnored(t *testing.T) {
	defer UnregisterAuthHandler()

	called := false
	RegisterAuthHandler(func(_ ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
		called = true
		return AuthResult{}
	})
	RegisterAuthHandler(nil)

	h := GetAuthHandler()
	if h == nil {
		t.Fatal("nil registration should not clear existing handler")
	}
	h(ppp.EventAuthRequest{}, nil)
	if !called {
		t.Fatal("original handler should still be active")
	}
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
	defer UnregisterShaperHandler()

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
