// Design: docs/research/l2tpv2-ze-integration.md -- PPP credential verification
// Related: l2tpauthlocal.go -- atomic logger

package l2tpauthlocal

import (
	"crypto/md5" //nolint:gosec // RFC 1994 CHAP-MD5 requires MD5 by protocol definition
	"crypto/sha256"
	"crypto/subtle"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
)

// userEntry holds a configured user's credentials.
type userEntry struct {
	Name   string
	secret string // PPP shared secret (PAP cleartext / CHAP key)
}

// localAuth holds the static user table and implements the auth handler.
type localAuth struct {
	mu    sync.RWMutex
	users map[string]userEntry
}

func newLocalAuth() *localAuth {
	return &localAuth{users: make(map[string]userEntry)}
}

func (a *localAuth) setUsers(users map[string]userEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.users = users
}

// handle is the AuthHandler function registered with the l2tp package.
func (a *localAuth) handle(req ppp.EventAuthRequest, _ l2tp.AuthRespondFunc) l2tp.AuthResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.users) == 0 {
		logger().Warn("l2tp-auth-local: no users configured; rejecting",
			"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
		return l2tp.AuthResult{Accept: false, Message: "no users configured"}
	}

	user, ok := a.users[req.Username]
	if !ok {
		logger().Info("l2tp-auth-local: unknown user",
			"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
		return l2tp.AuthResult{Accept: false, Message: "unknown user"}
	}

	switch req.Method {
	case ppp.AuthMethodNone:
		// LCP did not negotiate authentication. Accept: the auth method
		// is a wire-level decision; policy enforcement happens via LCP
		// Auth-Protocol option negotiation, not here.
		return l2tp.AuthResult{Accept: true, Message: "no auth required"}

	case ppp.AuthMethodPAP:
		return a.verifyPAP(req, user)

	case ppp.AuthMethodCHAPMD5:
		return a.verifyCHAPMD5(req, user)

	case ppp.AuthMethodMSCHAPv2:
		logger().Warn("l2tp-auth-local: MS-CHAPv2 not supported by local auth; rejecting",
			"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
		return l2tp.AuthResult{Accept: false, Message: "MS-CHAPv2 not supported by local auth"}

	default:
		return l2tp.AuthResult{Accept: false, Message: "unsupported auth method"}
	}
}

// RFC 1334 Section 2.2.1: PAP carries cleartext password.
func (a *localAuth) verifyPAP(req ppp.EventAuthRequest, user userEntry) l2tp.AuthResult {
	gotHash := sha256.Sum256(req.Response)
	wantHash := sha256.Sum256([]byte(user.secret))
	if subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1 {
		logger().Info("l2tp-auth-local: PAP accepted",
			"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
		return l2tp.AuthResult{Accept: true}
	}
	logger().Info("l2tp-auth-local: PAP rejected",
		"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
	return l2tp.AuthResult{Accept: false, Message: "bad password"}
}

// RFC 1994 Section 4.1: CHAP value = MD5(identifier || secret || challenge).
func (a *localAuth) verifyCHAPMD5(req ppp.EventAuthRequest, user userEntry) l2tp.AuthResult {
	h := md5.New() //nolint:gosec // RFC 1994 Section 4.1: CHAP-MD5 requires MD5
	h.Write([]byte{req.Identifier})
	h.Write([]byte(user.secret))
	h.Write(req.Challenge)
	expected := h.Sum(nil)

	if subtle.ConstantTimeCompare(req.Response, expected) == 1 {
		logger().Info("l2tp-auth-local: CHAP-MD5 accepted",
			"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
		return l2tp.AuthResult{Accept: true}
	}
	logger().Info("l2tp-auth-local: CHAP-MD5 rejected",
		"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
	return l2tp.AuthResult{Accept: false, Message: "bad CHAP response"}
}
