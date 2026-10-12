// RFC: rfc/short/rfc5882.md -- one session per remote system, shared by its clients
// Design: docs/research/bfd-implementation-guide.md -- shared sessions and refcounting
// Related: engine.go -- EnsureSession and ReleaseSession, the callers

package engine

import (
	"github.com/ze-software/ze/internal/component/bfd/api"
)

// sessionClient is one client's hold on a session: the key it asked with and
// the request it made. EnsureSession creates one per successful call and puts
// it in the handle it returns; ReleaseSession removes exactly that one, so a
// session always knows which clients it is serving.
type sessionClient struct {
	key api.Key
	req api.SessionRequest
}

// addClientLocked records a hold on entry, which the caller has already
// counted on the machine (session.Machine.Init counts the creator), and
// returns the handle that carries it. The caller MUST hold l.mu.
func (l *Loop) addClientLocked(entry *sessionEntry, sessionKey api.Key, req api.SessionRequest) *handle {
	client := &sessionClient{key: req.Key(), req: req}
	entry.clients = append(entry.clients, client)
	// RFC 5882 Section 4.4
	entry.joined = joinedKey(sessionKey, entry.clients)
	return &handle{loop: l, key: sessionKey, client: client}
}

// attachClientLocked adds one more client to a live entry: it counts the hold
// on the machine and records it (addClientLocked). The caller MUST hold l.mu,
// and MUST have revived a released entry first (reviveLocked).
func (l *Loop) attachClientLocked(entry *sessionEntry, sessionKey api.Key, req api.SessionRequest) *handle {
	entry.machine.Acquire()
	return l.addClientLocked(entry, sessionKey, req)
}

// detachClientLocked removes client's hold from entry and answers the number
// of holds left. It answers false when entry does not hold client: a handle
// released twice, or one that never held anything, releases nothing the
// second time. The caller MUST hold l.mu.
func detachClientLocked(entry *sessionEntry, sessionKey api.Key, client *sessionClient) (int, bool) {
	at := -1
	for i, held := range entry.clients {
		if held == client {
			at = i
			break
		}
	}
	if at < 0 {
		return 0, false
	}
	entry.clients = append(entry.clients[:at], entry.clients[at+1:]...)
	// RFC 5882 Section 4.4
	entry.joined = joinedKey(sessionKey, entry.clients)
	return entry.machine.Release(), true
}

// reviveLocked brings a released entry (ReleaseSession) back for a client that
// asks for it inside its Detection Time: it leaves AdminDown and keeps its
// discriminator. An entry that is not released is left as it is. The caller
// MUST hold l.mu.
func (l *Loop) reviveLocked(entry *sessionEntry) {
	if !entry.released {
		return
	}
	entry.released = false
	entry.machine.AdminEnable()
}

// joinedKey answers the identity a session answers for while clients hold it:
// sessionKey, with every field it left unset taken from a client that named
// it. sharesSession refused any client naming a field differently from the
// session or from another client, so the order of clients changes nothing.
//
// RFC 5882 Section 4.4: "If multiple control protocols wish to establish BFD
// sessions with the same remote system for the same data protocol, all MUST
// share a single BFD session."
//
// The narrowing is a property of the clients present, never of the session's
// history. A client pinned to a local address narrows the session to that
// address pair while it holds it (RFC 5883 Section 4.1 then keeps a client
// pinned elsewhere apart); once it leaves, the session never ran on that
// pinned address for anyone still attached, so keeping the narrowing would
// open a second session to the same remote system for the next client pinned
// elsewhere. The loop is bounded by the session's clients, one per protocol
// instance that asked for this remote system.
func joinedKey(sessionKey api.Key, clients []*sessionClient) api.Key {
	joined := sessionKey
	for _, client := range clients {
		if !joined.Local.IsValid() {
			joined.Local = client.key.Local
		}
		if joined.Interface == "" {
			joined.Interface = client.key.Interface
		}
	}
	return joined
}

// sharedEntryLocked answers the one live session a request shares, when its
// key is not one the engine already holds exactly. The caller MUST hold l.mu.
//
// RFC 5882 Section 4.4: "If multiple control protocols wish to establish BFD
// sessions with the same remote system for the same data protocol, all MUST
// share a single BFD session."
//
// The clients do not name a session with the same fields.
// api.SessionRequest.Canonical completes what it can from the link table, and
// what it cannot complete stays unset: the local address of a multi-hop BGP
// peer with no `connection local ip`, which Canonical never derives, and the
// link and address of a single-hop peer on two links, on no connected prefix,
// or on a daemon with no interface backend. An unset field says the client
// does not care, so it matches any value (sharesSession). ok is false when no
// session matches, and also when more than one does: a request that could
// mean either of two sessions is not merged onto one of them by a guess, and
// gets its own.
//
// A released session (ReleaseSession) has no client left to share with, so it
// is matched only when no live session is: the live sessions are counted
// first, and the released ones decide only when none of them matches. Counted
// together, a released session kept for its Detection Time would make a
// request that matches one live session ambiguous, and open a second session
// to the same remote system.
//
// The scan is linear in the loop's sessions. EnsureSession runs once per
// client per session lifetime, never per packet.
func (l *Loop) sharedEntryLocked(key api.Key) (api.Key, *sessionEntry, bool) {
	var live, released sharedMatch
	for k, entry := range l.sessions {
		if !sharesSession(key, entry.joined) {
			continue
		}
		if entry.released {
			released.add(k, entry)
			continue
		}
		live.add(k, entry)
	}
	if live.count > 0 {
		return live.only()
	}
	return released.only()
}

// sharedMatch counts the sessions sharedEntryLocked matched in one class, live
// or released, and remembers the last one.
type sharedMatch struct {
	key   api.Key
	entry *sessionEntry
	count int
}

// add records one more matching session.
func (s *sharedMatch) add(key api.Key, entry *sessionEntry) {
	s.key, s.entry = key, entry
	s.count++
}

// only answers the matched session when exactly one matched; none, or more
// than one, is no answer.
func (s *sharedMatch) only() (api.Key, *sessionEntry, bool) {
	if s.count != 1 {
		return api.Key{}, nil, false
	}
	return s.key, s.entry, true
}

// sharesSession reports whether two keys can name one session: the same peer,
// VRF and hop mode, and no field that both name with different values.
//
// RFC 5883 Section 4.1: "Multiple sessions between the same pair of systems
// must have at least one endpoint address distinct from one another." Two
// local addresses both named are two address pairs, so they never share; an
// address one side left unset is no endpoint distinct from anything.
//
// RFC 5881 Section 2 puts a single-hop session on "a single IP hop that is
// associated with an incoming interface", so two named links never share
// either.
//
// A link-local peer is the exception to the unset link. RFC 4007 Section 6:
// "the same non-global address may be in use in more than one zone of the
// same scope (e.g., the use of link-local address fe80::1 in two separate
// physical links)". fe80::1 with no link does not name a remote system, so it
// matches only a session that names no link either.
func sharesSession(a, b api.Key) bool {
	if a.Peer != b.Peer {
		return false
	}
	if a.VRF != b.VRF {
		return false
	}
	if a.Mode != b.Mode {
		return false
	}
	if a.Local.IsValid() && b.Local.IsValid() && a.Local != b.Local {
		return false
	}
	if a.Peer.IsLinkLocalUnicast() {
		return a.Interface == b.Interface
	}
	if a.Interface != "" && b.Interface != "" && a.Interface != b.Interface {
		return false
	}
	return true
}
