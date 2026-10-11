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
	what, fix := authMismatchCase(entry.auth, req)
	return fmt.Errorf("bfd: session to peer %s interface %q vrf %s: %w: %s; %s; to fix it, %s",
		key.Peer, key.Interface, key.VRF, ErrAuthMismatch, what, authMismatchWhy, fix)
}

// authMismatchWhy says why a client whose authentication differs gets no
// session, whichever side differs.
const authMismatchWhy = "every client of one remote system shares one BFD session " +
	"(RFC 5882 Section 4.4) that runs one authentication, so this client gets no BFD session"

// authMismatchCase names what differs between the live session's
// authentication and the request's, and the config change that resolves it.
// The fix depends on which side runs authentication, because one kind of
// client can never carry any: an OSPF interface's bfd block names no profile
// and no auth block (ospf bfdRequestForNeighbor), so a fix that adds an auth
// block to every client cannot apply when OSPF runs BFD to the neighbor. The
// engine does not know which protocol a client is, so a case OSPF can reach
// names the fix that works when one client is OSPF. The caller MUST have
// found the two configurations different (sameAuth).
func authMismatchCase(live *api.AuthSettings, req api.SessionRequest) (what, fix string) {
	if live == nil {
		// The live session runs none. Its client may be an OSPF interface,
		// which cannot take an auth block, so dropping this client's block
		// is the fix that always works.
		return "the live session runs no authentication but this client's " +
				clientProfile(req.Profile) + " carries an auth block",
			"remove the auth block from this client's " + clientProfile(req.Profile) +
				", or give every other client of this neighbor a bfd profile with the same " +
				"auth { type; key-id; secret; } block and disable bfd on any OSPF interface " +
				"to this neighbor, because an OSPF interface's bfd block carries no authentication"
	}
	if req.Auth == nil {
		if req.Profile != "" {
			// A client that named a profile is not OSPF, so it can take
			// the auth block.
			return "the live session runs authentication but this client's " +
					clientProfile(req.Profile) + " carries no auth block",
				"give " + clientProfile(req.Profile) + " the same auth { type; key-id; secret; } " +
					"block as the other clients of this neighbor, or remove the auth block from their profiles"
		}
		// No profile: an OSPF interface, or a BGP peer or static next-hop
		// that names none. Removing the other clients' auth block works for
		// all three.
		return "the live session runs authentication but this client names no bfd profile, " +
				"so it runs none",
			"remove the auth block from the other clients' bfd profiles for this neighbor; " +
				"a BGP peer or static next-hop can instead name a bfd profile with the same " +
				"auth { type; key-id; secret; } block, and an OSPF interface, whose bfd block " +
				"carries no authentication, can instead disable bfd on that interface"
	}
	// Both run authentication, so both named a profile and neither is OSPF.
	return "this client's " + clientProfile(req.Profile) + " carries a different auth block " +
			"from the live session's",
		"name the same bfd profile on every client of this neighbor, " +
			"or give their profiles the same auth { type; key-id; secret; } block"
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
