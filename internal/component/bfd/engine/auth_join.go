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
	return fmt.Errorf("bfd: session to peer %s%s vrf %s: %w: %s; %s; to fix it, %s; %s; %s",
		key.Peer, refusalInterface(key.Interface), key.VRF, ErrAuthMismatch, what,
		authMismatchWhy, fix, authReleaseHow, authRetryHow)
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
const authMismatchWhy = "every client of one remote system shares one BFD session " +
	"(RFC 5882 Section 4.4) that runs one authentication, so this client gets no BFD session"

// authReleaseHow says when a profile edit reaches a live session, and how to
// make it. Authentication is set only when a session is created or revived
// (createSessionLocked, replaceAuth), and a config commit re-requests nothing
// a client already holds: a pinned session is re-requested only for a key it
// does not hold (applyPinned), and a BGP peer, a static route and an OSPF
// interface ask only when they start BFD. So a fix that edits a profile a
// client already holds the session under applies once every client has let
// go, and each client lets go only through the change named here.
const authReleaseHow = "a live session keeps the authentication it was built with until " +
	"every client has released it, so an edit to another client's profile, or to the " +
	"profile the session was built from, reaches it only then: remove each pinned " +
	"single-hop-session or multi-hop-session to this neighbor and commit, run " +
	"'request peer <address> teardown' for each BGP peer (a strict-mode peer releases " +
	"only when it is removed from the config), remove each static route whose next-hop " +
	"uses it, and set bfd { enabled false; } on each OSPF interface to it"

// authRetryHow says when a refused client asks again, which is when a fix to
// its own profile takes effect: each kind of client asks only when it starts
// BFD (applyPinned, startBFDClient, setupBFDLocked, bfdNeighborFull).
const authRetryHow = "a refused client asks again when it next starts BFD: a pinned session " +
	"at the next commit, a strict-mode BGP peer at its next connection attempt, any other " +
	"BGP peer when its session next reaches Established, a static route when it is removed " +
	"and added back, and an OSPF interface when its neighbor next reaches Full"

// ospfDisableHow is the change that takes an OSPF interface out of BFD: its
// bfd block's enabled leaf, which defaults to false, so deleting the block
// does the same.
const ospfDisableHow = "set bfd { enabled false; } or delete its bfd block"

// authMismatchCase names what differs between the live session's
// authentication and the request's, and the config change that resolves it.
// The fix depends on which side runs authentication, because one kind of
// client can never carry any: an OSPF interface's bfd block names no profile
// and no auth block (ospf bfdRequestForNeighbor), so a fix that adds an auth
// block to every client cannot apply when OSPF runs BFD to the neighbor. The
// engine does not know which protocol a client is, so a case OSPF can reach
// names the fix that works when one client is OSPF. The caller MUST have
// found the two configurations different (sameAuth).
//
// liveFrom names the profile the live session's authentication came from
// (sessionEntry.authFrom). A client naming that same profile with different
// authentication means the profile was edited after the session was built:
// profiles resolve when a client asks (resolveProfile), so the one name
// resolved to two auth blocks at two times. No fix to a profile helps then,
// because the session keeps what it was built with, so the refusal names the
// release that applies the edit.
func authMismatchCase(live *api.AuthSettings, liveFrom string, req api.SessionRequest) (what, fix string) {
	if req.Profile != "" && req.Profile == liveFrom {
		return "this client's " + clientProfile(req.Profile) + " is the profile the live " +
				"session's authentication was built from, and its auth block has changed since",
			"apply the edit by releasing the session from every client as below, so the next " +
				"client to ask builds it from the profile as it is now"
	}
	if live == nil {
		// The live session runs none. Its client may be an OSPF interface,
		// which cannot take an auth block, so dropping this client's block
		// is the fix that always works.
		return "the live session runs no authentication but this client's " +
				clientProfile(req.Profile) + " carries an auth block",
			"remove the auth block from this client's " + clientProfile(req.Profile) +
				", or give every other client of this neighbor a bfd profile with the same " +
				"auth { type; key-id; secret; } block and, on every OSPF interface to this neighbor, " +
				ospfDisableHow + ", because an OSPF interface's bfd block carries no authentication"
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
		// No profile: an OSPF interface, or a BGP peer, static next-hop or
		// pinned session that names none. Removing the other clients' auth
		// block works for all four.
		return "the live session runs authentication but this client names no bfd profile, " +
				"so it runs none",
			"remove the auth block from the other clients' bfd profiles for this neighbor; " +
				"a BGP peer, a static next-hop or a pinned single-hop-session or " +
				"multi-hop-session can instead name a bfd profile with the same " +
				"auth { type; key-id; secret; } block, and an OSPF interface, whose bfd block " +
				"carries no authentication, can instead " + ospfDisableHow
	}
	// Both run authentication, so both named a profile and neither is OSPF.
	// The live session's profile may have been edited since it was built;
	// naming it then meets the edited-profile branch above, which names the
	// release.
	return "this client's " + clientProfile(req.Profile) + " carries a different auth block " +
			"from the live session's, which was built from " + clientProfile(liveFrom),
		"name " + clientProfile(liveFrom) + " on this client, or give " +
			clientProfile(req.Profile) + " the auth { type; key-id; secret; } block of " +
			clientProfile(liveFrom) + ", or give every other client of this neighbor the " +
			"auth block of " + clientProfile(req.Profile)
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
