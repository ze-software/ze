package l2tp_test

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
)

func TestRegisterAuthHandler(t *testing.T) {
	t.Cleanup(l2tp.UnregisterAuthHandler)

	called := false
	h := func(req ppp.EventAuthRequest, _ l2tp.AuthRespondFunc) l2tp.AuthResult {
		called = true
		return l2tp.AuthResult{Accept: true}
	}
	l2tp.RegisterAuthHandler(h)

	got := l2tp.GetAuthHandler()
	if got == nil {
		t.Fatal("expected non-nil auth handler")
	}
	got(ppp.EventAuthRequest{}, nil)
	if !called {
		t.Fatal("registered handler was not called")
	}
}

func TestRegisterAuthHandlerNil(t *testing.T) {
	t.Cleanup(l2tp.UnregisterAuthHandler)

	l2tp.RegisterAuthHandler(nil)
	if l2tp.GetAuthHandler() != nil {
		t.Fatal("nil handler should be ignored")
	}
}

func TestRegisterPoolHandler(t *testing.T) {
	t.Cleanup(l2tp.UnregisterPoolHandler)

	called := false
	h := func(req ppp.EventIPRequest) ppp.IPResponseArgs {
		called = true
		return ppp.IPResponseArgs{Accept: true}
	}
	l2tp.RegisterPoolHandler(h)

	got := l2tp.GetPoolHandler()
	if got == nil {
		t.Fatal("expected non-nil pool handler")
	}
	got(ppp.EventIPRequest{})
	if !called {
		t.Fatal("registered handler was not called")
	}
}

func TestRegisterPoolHandlerNil(t *testing.T) {
	t.Cleanup(l2tp.UnregisterPoolHandler)

	l2tp.RegisterPoolHandler(nil)
	if l2tp.GetPoolHandler() != nil {
		t.Fatal("nil handler should be ignored")
	}
}
