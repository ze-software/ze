// Design: docs/architecture/bfd.md -- session registry and authentication
// Related: engine.go -- EnsureSession, the two join arms that call these
// RFC: rfc/short/rfc5882.md
//
// One BFD session carries one authentication configuration. The functions
// here decide what a request that reaches an existing session does with its
// authentication: a released session takes it, a live session refuses a
// request whose authentication differs.
package engine

import (
	"bytes"
	"fmt"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/session"
)

// replaceAuth gives entry the authentication req configures, none when req
// carries no Auth. The pair is built before anything changes, so a request
// whose pair cannot be built leaves entry as it was.
func replaceAuth(entry *sessionEntry, req api.SessionRequest, key api.Key) error {
	var pair *session.AuthPair
	if req.Auth != nil {
		built, err := buildAuthPair(req, key)
		if err != nil {
			return err
		}
		pair = built
	}
	if closeErr := entry.machine.CloseAuth(); closeErr != nil {
		engineLog().Debug("bfd auth persister close failed", "key", key, "err", closeErr)
	}
	entry.machine.SetAuth(pair)
	entry.auth = cloneAuth(req.Auth)
	return nil
}

// sharedAuthLocked settles the authentication of a shared join
// (sharedEntryLocked). A released session has no client left relying on its
// authentication, so it takes the request's, as reviveReleasedLocked does for
// the exact key; a live session refuses a request whose authentication
// differs (joinAuthCheck). The caller MUST hold l.mu.
func (l *Loop) sharedAuthLocked(entry *sessionEntry, req api.SessionRequest, sessionKey api.Key) error {
	if entry.released {
		return replaceAuth(entry, req, sessionKey)
	}
	return joinAuthCheck(entry, req, sessionKey)
}

// joinAuthCheck refuses a request that would join the live session entry,
// named by key, when the request's authentication differs from the
// session's. Joins with identical authentication share the session.
//
// RFC 5882 Section 10.2: "BFD authentication SHOULD be used and is strongly
// encouraged." RFC 5880 Section 6.7: "The same authentication type, and any
// keys or other necessary information, obviously must be in use by the two
// systems." One session carries one authentication configuration, so without
// this check a client whose profile configures authentication would run on an
// unauthenticated session with nothing said, and a client asking for none
// would run on a session its neighbor may not authenticate.
func joinAuthCheck(entry *sessionEntry, req api.SessionRequest, key api.Key) error {
	if sameAuth(entry.auth, req.Auth) {
		return nil
	}
	return fmt.Errorf("bfd: session to peer %s interface %q vrf %s: %w",
		key.Peer, key.Interface, key.VRF, ErrAuthMismatch)
}

// sameAuth reports whether two authentication configurations are the one
// configuration: both absent, or the same Auth Type, Key ID, Meticulous flag
// and secret.
func sameAuth(a, b *api.AuthSettings) bool {
	if a == nil {
		return b == nil
	}
	if b == nil {
		return false
	}
	if a.Type != b.Type {
		return false
	}
	if a.KeyID != b.KeyID {
		return false
	}
	if a.Meticulous != b.Meticulous {
		return false
	}
	return bytes.Equal(a.Secret, b.Secret)
}

// cloneAuth copies a, secret included, so the session's record of its
// authentication does not alias the caller's request. nil stays nil.
func cloneAuth(a *api.AuthSettings) *api.AuthSettings {
	if a == nil {
		return nil
	}
	c := *a
	c.Secret = bytes.Clone(a.Secret)
	return &c
}
