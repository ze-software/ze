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

// ProfileAuth answers what the config applied now makes of profile name for
// the loop it is installed on: the authentication it resolves to, nil for
// none, and whether a client of that loop may name it, which needs the config
// to define it and to permit it for the loop's hop mode. The bfd plugin
// supplies it (SetProfileAuth), so the engine reads the profile table without
// importing the config.
type ProfileAuth func(name string) (auth *api.AuthSettings, defined bool)

// SetProfileAuth installs the profile lookup a refused join reads to tell
// whether the profile its session was built from still holds the session's
// authentication (sourceCurrent). With none installed a refusal never names
// that profile and answers with the release, which applies whatever the
// profile holds now. Safe for concurrent use: it takes l.mu.
func (l *Loop) SetProfileAuth(lookup ProfileAuth) {
	l.mu.Lock()
	l.profileAuth = lookup
	l.mu.Unlock()
}

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
	return joinAuthCheck(entry, req, sessionKey, l.profileAuth)
}

// joinAuthCheck refuses a request that would join the live session entry,
// named by key, when the request's authentication differs from the
// session's. Joins with identical authentication share the session. profiles
// is the loop's lookup (SetProfileAuth), nil when none is installed.
//
// RFC 5882 Section 10.2: "BFD authentication SHOULD be used and is strongly
// encouraged." RFC 5880 Section 6.7: "The same authentication type, and any
// keys or other necessary information, obviously must be in use by the two
// systems." One session carries one authentication configuration, so without
// this check a client whose profile configures authentication would run on an
// unauthenticated session with nothing said, and a client asking for none
// would run on a session its neighbor may not authenticate.
func joinAuthCheck(entry *sessionEntry, req api.SessionRequest, key api.Key, profiles ProfileAuth) error {
	if sameAuth(entry.auth, req.Auth) {
		return nil
	}
	current := sourceCurrent(profiles, entry.auth, entry.authFrom)
	what, fix := authMismatchCase(entry.auth, entry.authFrom, current, req)
	return fmt.Errorf("bfd: session to peer %s%s vrf %s: %w: %s; %s; to fix it, %s",
		key.Peer, refusalInterface(key.Interface), key.VRF, ErrAuthMismatch, what,
		authMismatchWhy, fix)
}

// sourceCurrent reports whether liveFrom, the profile the live session's
// authentication came from, still resolves to that authentication. Only then
// may a refusal send a client to it: a profile edited or removed since would
// give the client an authentication the session does not run, or a name the
// commit refuses, as undefined or as one the session's hop mode may not use
// (ProfileAuth answers not defined for that). The false answer is a guard: no recorded
// profile, or no lookup installed, cannot show the profile is unchanged, so
// the refusal falls to the release, which applies whatever the profile holds.
func sourceCurrent(profiles ProfileAuth, live *api.AuthSettings, liveFrom string) bool {
	if liveFrom == "" {
		return false
	}
	if profiles == nil {
		return false
	}
	auth, defined := profiles(liveFrom)
	if !defined {
		return false
	}
	return sameAuth(auth, live)
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
// session, whichever side differs. It says no more, because what the client
// then does depends on the client: a BGP peer runs without BFD and a
// strict-mode one is held down, and a pinned session fails the commit's apply.
const authMismatchWhy = "one session per neighbor runs one authentication " +
	"(RFC 5882 Section 4.4), so this client gets no BFD session"

// authReleaseFix is the fix when no change to the refused client can match
// the live session: only a release by every client lets the session be built
// again from a profile as it is now. How each kind of client releases is in
// the guide, so the refusal points there.
const authReleaseFix = "release the session from every client to rebuild it " +
	"(docs/guide/bfd.md, \"Editing live\")"

// authMismatchCase names what differs between the live session's
// authentication and the request's, and the one config change that resolves
// it. The refusal stays short: when a change takes effect, and how each kind
// of client releases a session, is in docs/guide/bfd.md "Editing live", which
// the refusal names only where a release is the fix. The caller MUST have
// found the two configurations different (sameAuth).
//
// No fix edits a named profile. An edit to a profile reaches every client
// naming it, on any neighbor, at its next ask, so removing the auth block from
// the profile one refused peer names would drop authentication for every other
// peer naming it. Each fix instead names a profile on the refused client, or
// copies an auth block into a profile only that client names, so it changes
// the refused client alone and takes effect when that client next asks. A live
// session keeps the authentication it was built with (it is set only in
// createSessionLocked and replaceAuth), so a fix that edits another client's
// profile would also wait for every client to release the session.
//
// liveFrom names the profile the live session's authentication came from
// (sessionEntry.authFrom). Authentication reaches a request only through a
// profile (profileConfig.applyTo), so liveFrom is never empty when live is
// not nil. current says whether liveFrom still resolves to live
// (sourceCurrent). A client naming liveFrom with different authentication
// means that profile was edited after the session was built: profiles resolve
// when a client asks (resolveProfile), so the one name resolved to two auth
// blocks at two times. Once liveFrom no longer resolves to live, naming it or
// copying it matches nothing, so a refusal never sends a client to it and only
// a release by every client helps.
//
// An OSPF interface's bfd block names no profile and no auth block (ospf
// bfdRequestForNeighbor), and the engine does not know which protocol a
// client is, so the no-profile case names the OSPF fix beside the profile one.
func authMismatchCase(live *api.AuthSettings, liveFrom string, current bool, req api.SessionRequest) (what, fix string) {
	if req.Profile != "" && req.Profile == liveFrom {
		return clientProfile(req.Profile) + " changed after the session was built from it",
			authReleaseFix
	}
	if live == nil {
		// The fix needs no profile the session was built from: any profile
		// with no auth block matches a session that runs none.
		what = "the session runs none, " + clientProfile(req.Profile) + " has an auth block"
		if current {
			return what, "name " + clientProfile(liveFrom) + " on this client, " +
				"or another bfd profile with no auth block"
		}
		return what, "name a bfd profile with no auth block on this client"
	}
	if !current {
		// The release suits every client, an OSPF interface included, so
		// this case names no second fix.
		return "the session's " + clientProfile(liveFrom) + " changed or was removed since",
			authReleaseFix
	}
	if req.Auth == nil {
		if req.Profile != "" {
			return "the session authenticates, " + clientProfile(req.Profile) + " has no auth block",
				nameOrCopy(liveFrom)
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
		nameOrCopy(liveFrom)
}

// nameOrCopy is the fix for a profiled client that must take the session's
// authentication: name the profile the session was built from, or copy its
// auth block into a profile no other client names, so no shared profile
// changes.
func nameOrCopy(liveFrom string) string {
	return "name " + clientProfile(liveFrom) + " on this client, " +
		"or copy its auth block into a profile only this client names"
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
