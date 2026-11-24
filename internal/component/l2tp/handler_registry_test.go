package l2tp_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func TestRegisterAuthHandler(t *testing.T) {
	t.Cleanup(func() {
		l2tp.UnregisterAuthHandler(l2tp.AuthPriorityRemote)
		l2tp.UnregisterAuthHandler(l2tp.AuthPriorityLocal)
	})

	called := false
	h := func(req ppp.EventAuthRequest, _ l2tp.AuthRespondFunc) l2tp.AuthResult {
		called = true
		return l2tp.AuthResult{Accept: true}
	}
	l2tp.RegisterAuthHandler(l2tp.AuthPriorityLocal, h)

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
	t.Cleanup(func() {
		l2tp.UnregisterAuthHandler(l2tp.AuthPriorityRemote)
		l2tp.UnregisterAuthHandler(l2tp.AuthPriorityLocal)
	})

	l2tp.RegisterAuthHandler(l2tp.AuthPriorityLocal, nil)
	if l2tp.GetAuthHandler() != nil {
		t.Fatal("nil handler should be ignored")
	}
}

// VALIDATES: l2tp.RegisterAuthHandler preserves AuthPriorityRemote precedence
// over AuthPriorityLocal regardless of registration order, and falls back to
// local when remote is unregistered.
func TestRegisterAuthHandlerPrecedence(t *testing.T) {
	t.Cleanup(func() {
		l2tp.UnregisterAuthHandler(l2tp.AuthPriorityRemote)
		l2tp.UnregisterAuthHandler(l2tp.AuthPriorityLocal)
	})

	local := func(_ ppp.EventAuthRequest, _ l2tp.AuthRespondFunc) l2tp.AuthResult {
		return l2tp.AuthResult{Accept: true, Message: "local"}
	}
	remote := func(_ ppp.EventAuthRequest, _ l2tp.AuthRespondFunc) l2tp.AuthResult {
		return l2tp.AuthResult{Accept: true, Message: "radius"}
	}

	l2tp.RegisterAuthHandler(l2tp.AuthPriorityRemote, remote)
	l2tp.RegisterAuthHandler(l2tp.AuthPriorityLocal, local)
	if got := l2tp.GetAuthHandler()(ppp.EventAuthRequest{}, nil); got.Message != "radius" {
		t.Fatalf("remote-then-local answered %q, want radius", got.Message)
	}

	l2tp.UnregisterAuthHandler(l2tp.AuthPriorityRemote)
	if got := l2tp.GetAuthHandler()(ppp.EventAuthRequest{}, nil); got.Message != "local" {
		t.Fatalf("after removing remote answered %q, want local", got.Message)
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
