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
	"strconv"

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
	entry.authFrom = req.Profile
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
	what, fix := authMismatchCase(entry.auth, entry.authFrom, req)
	return fmt.Errorf("bfd: session to peer %s%s vrf %s: %w: %s; %s; to fix it, %s",
		key.Peer, refusalInterface(key.Interface), key.VRF, ErrAuthMismatch, what,
		authMismatchWhy, fix)
}

// refusalInterface names the interface a refusal's key carries, and nothing
// for a key that names none, so the refusal never prints an empty name.
func refusalInterface(name string) string {
	if name == "" {
		return ""
	}
	return " interface " + strconv.Quote(name)
}

// authMismatchWhy says why a client whose authentication differs gets no
// session, whichever side differs.
const authMismatchWhy = "one session per neighbor runs one authentication " +
	"(RFC 5882 Section 4.4), so this client runs without BFD"

// authMismatchCase names what differs between the live session's
// authentication and the request's, and the one config change that resolves
// it. The refusal stays short: when a change takes effect, and how each kind
// of client releases a session, is in docs/guide/bfd.md "Editing live", which
// the refusal names only where a release is the fix. The caller MUST have
// found the two configurations different (sameAuth).
//
// Each fix but the edited-profile one changes only the refused client, so it
// takes effect when that client next asks: a live session keeps the
// authentication it was built with (it is set only in createSessionLocked and
// replaceAuth), so a fix that edits another client's profile would wait for
// every client to release the session.
//
// liveFrom names the profile the live session's authentication came from
// (sessionEntry.authFrom). Authentication reaches a request only through a
// profile (profileConfig.applyTo), so liveFrom is never empty when live is
// not nil. A client naming that same profile with different authentication
// means the profile was edited after the session was built: profiles resolve
// when a client asks (resolveProfile), so the one name resolved to two auth
// blocks at two times. Only a release by every client applies that edit.
//
// An OSPF interface's bfd block names no profile and no auth block (ospf
// bfdRequestForNeighbor), and the engine does not know which protocol a
// client is, so the no-profile case names the OSPF fix beside the profile one.
func authMismatchCase(live *api.AuthSettings, liveFrom string, req api.SessionRequest) (what, fix string) {
	if req.Profile != "" && req.Profile == liveFrom {
		return clientProfile(req.Profile) + " changed after the session was built from it",
			"release the session from every client to apply the edit " +
				"(docs/guide/bfd.md, \"Editing live\")"
	}
	if live == nil {
		return "the session runs none, " + clientProfile(req.Profile) + " has an auth block",
			"remove the auth block from " + clientProfile(req.Profile)
	}
	if req.Auth == nil {
		if req.Profile != "" {
			return "the session authenticates, " + clientProfile(req.Profile) + " has no auth block",
				"give " + clientProfile(req.Profile) + " the auth block of " + clientProfile(liveFrom)
		}
		// No profile: an OSPF interface, or a BGP peer or pinned session
		// that names none.
		return "the session authenticates, this client names no bfd profile",
			"name " + clientProfile(liveFrom) + " on this client, or set " +
				"bfd { enabled false; } if it is an OSPF interface"
	}
	// Both run authentication, so both named a profile and neither is OSPF.
	return "the session was built from " + clientProfile(liveFrom) + ", this client names " +
			strconv.Quote(req.Profile),
		"name " + clientProfile(liveFrom) + " on this client, or copy its auth block into " +
			strconv.Quote(req.Profile)
}

// clientProfile names the profile a request resolved, for a refusal.
func clientProfile(name string) string {
	if name == "" {
		return "bfd profile"
	}
	return "bfd profile " + strconv.Quote(name)
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
