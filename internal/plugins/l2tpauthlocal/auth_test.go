package l2tpauthlocal

import (
	"crypto/md5"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
)

func TestLocalAuthPAPAccept(t *testing.T) {
	a := newLocalAuth()
	a.setUsers(map[string]userEntry{
		"alice": {Name: "alice", secret: "pass123"},
	})

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 1,
		Method:    ppp.AuthMethodPAP,
		Username:  "alice",
		Response:  []byte("pass123"),
	}, nil)
	if !result.Accept {
		t.Fatalf("expected accept, got reject: %s", result.Message)
	}
}

func TestLocalAuthPAPReject(t *testing.T) {
	a := newLocalAuth()
	a.setUsers(map[string]userEntry{
		"alice": {Name: "alice", secret: "pass123"},
	})

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 1,
		Method:    ppp.AuthMethodPAP,
		Username:  "alice",
		Response:  []byte("wrong"),
	}, nil)
	if result.Accept {
		t.Fatal("expected reject for wrong password")
	}
}

func TestLocalAuthCHAPMD5Accept(t *testing.T) {
	a := newLocalAuth()
	a.setUsers(map[string]userEntry{
		"bob": {Name: "bob", secret: "secret"},
	})

	challenge := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	var identifier uint8 = 42

	// RFC 1994 Section 4.1: MD5(identifier || secret || challenge).
	h := md5.New() //nolint:gosec // test: RFC 1994 requires MD5
	h.Write([]byte{identifier})
	h.Write([]byte("secret"))
	h.Write(challenge)
	response := h.Sum(nil)

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:   1,
		SessionID:  1,
		Method:     ppp.AuthMethodCHAPMD5,
		Identifier: identifier,
		Username:   "bob",
		Challenge:  challenge,
		Response:   response,
	}, nil)
	if !result.Accept {
		t.Fatalf("expected accept, got reject: %s", result.Message)
	}
}

func TestLocalAuthCHAPMD5Reject(t *testing.T) {
	a := newLocalAuth()
	a.setUsers(map[string]userEntry{
		"bob": {Name: "bob", secret: "secret"},
	})

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:   1,
		SessionID:  1,
		Method:     ppp.AuthMethodCHAPMD5,
		Identifier: 1,
		Username:   "bob",
		Challenge:  []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Response:   []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}, nil)
	if result.Accept {
		t.Fatal("expected reject for wrong CHAP response")
	}
}

func TestLocalAuthNoUsersRejectAll(t *testing.T) {
	a := newLocalAuth()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 1,
		Method:    ppp.AuthMethodPAP,
		Username:  "anyone",
		Response:  []byte("anything"),
	}, nil)
	if result.Accept {
		t.Fatal("expected reject when no users configured (fail-closed)")
	}
}

func TestLocalAuthUnknownUser(t *testing.T) {
	a := newLocalAuth()
	a.setUsers(map[string]userEntry{
		"alice": {Name: "alice", secret: "pass"},
	})

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 1,
		Method:    ppp.AuthMethodPAP,
		Username:  "unknown",
		Response:  []byte("pass"),
	}, nil)
	if result.Accept {
		t.Fatal("expected reject for unknown user")
	}
}

func TestLocalAuthMethodNoneAccepted(t *testing.T) {
	a := newLocalAuth()
	a.setUsers(map[string]userEntry{
		"alice": {Name: "alice", secret: "pass"},
	})

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 1,
		Method:    ppp.AuthMethodNone,
		Username:  "alice",
	}, nil)
	if !result.Accept {
		t.Fatal("AuthMethodNone should always accept")
	}
}

func TestLocalAuthHandlerType(t *testing.T) {
	a := newLocalAuth()
	var h l2tp.AuthHandler = a.handle
	result := h(ppp.EventAuthRequest{Method: ppp.AuthMethodNone}, nil)
	if result.Accept {
		t.Fatal("handler should reject when no users configured (fail-closed)")
	}
}
