// RFC: rfc/short/rfc5882.md -- one session per remote system, shared by its clients
// Design: docs/research/bfd-implementation-guide.md -- shared sessions and refcounting
// Related: engine.go -- EnsureSession and ReleaseSession, the callers

package engine

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/session"
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
	// RFC 5882 Section 4.1
	retimeLocked(entry)
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
	// RFC 5882 Section 4.1
	retimeLocked(entry)
	return entry.machine.Release(), true
}

// UpdateSession gives the client holding h the request req in place: a
// profile edit reaching a live session. The session keeps its discriminator,
// its state and what it learned from the remote end; it takes req's
// authentication, and runs the timers of the fastest of its clients
// (retimeLocked), so an Up session changes them through a Poll Sequence
// (session.Machine.ApplyTimers).
//
// req MUST name the session the client asked for when it took h: an edit
// that moves a client to another remote system is a release and a new
// request. A change of authentication is refused with ErrAuthMismatch while
// another client of the session relies on the current one (updateAuthLocked).
// Safe for concurrent use: it takes l.mu.
func (l *Loop) UpdateSession(h api.SessionHandle, req api.SessionRequest) error {
	hh, ok := h.(*handle)
	if !ok {
		return ErrForeignHandle
	}
	if req.Key() != hh.client.key {
		return fmt.Errorf("bfd: edit of the session to peer %s: the new request names peer %s, "+
			"another session than the one this client holds; a client that moves releases its "+
			"session and asks for the new one: %w", hh.client.key.Peer, req.Peer, ErrUnknownSession)
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.sessions[hh.key]
	if !ok {
		return ErrUnknownSession
	}
	// A handle released before the edit holds nothing to edit.
	if !slices.Contains(entry.clients, hh.client) {
		return ErrUnknownSession
	}
	return updateClientLocked(entry, hh.client, req, hh.key)
}

// Reprofile gives every client of the loop that named a bfd profile the
// request resolve answers for it, in place: a reload that edits a profile
// reaching the sessions its clients hold. resolve takes the request a client
// holds and answers it with the profile's current timers and authentication;
// a client naming no profile carries its own and is left alone. A session
// keeps its discriminator and changes its timers through a Poll Sequence
// (retimeLocked).
//
// The clients of one session are edited together (reprofileEntryLocked), so
// an edit giving every profile on a session the same new auth block is taken.
// A session whose clients would no longer agree on one authentication, or
// one of whose clients resolve cannot answer, keeps every request it held,
// and the answer joins the error of each such session. resolve MUST NOT call
// back into the loop. Safe for concurrent use: it takes l.mu.
func (l *Loop) Reprofile(resolve func(api.SessionRequest) (api.SessionRequest, error)) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error
	for sessionKey, entry := range l.sessions {
		// RFC 5882 Section 4.1
		if err := reprofileEntryLocked(entry, sessionKey, resolve); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// reprofileEntryLocked resolves the request of every client of entry that
// named a profile, then installs them all, or none: a session runs one
// authentication for every client (joinAuthCheck), so the edited requests
// MUST agree on it before any is taken. The caller MUST hold l.mu.
//
// RFC 5882 Section 10.2: "BFD authentication SHOULD be used and is strongly
// encouraged." A client may only run on a session whose authentication it
// asked for.
func reprofileEntryLocked(entry *sessionEntry, sessionKey api.Key, resolve func(api.SessionRequest) (api.SessionRequest, error)) error {
	if len(entry.clients) == 0 {
		return nil
	}
	requests := make([]api.SessionRequest, len(entry.clients))
	for i, client := range entry.clients {
		requests[i] = client.req
		if client.req.Profile == "" {
			continue
		}
		req, err := resolve(client.req)
		if err != nil {
			return err
		}
		requests[i] = req
	}
	for _, req := range requests[1:] {
		if sameAuth(requests[0].Auth, req.Auth) {
			continue
		}
		// Only a profile carries an auth block (profileConfig.applyTo), so
		// when one side names none, the other is the profile this commit
		// edited.
		edited, holder := requests[0].Profile, req.Profile
		if edited == "" {
			edited, holder = holder, edited
		}
		return fmt.Errorf("bfd: session to peer %s%s vrf %s: %w: after this commit %s and %s "+
			"ask for different auth blocks; %s; to fix it, %s",
			sessionKey.Peer, refusalInterface(sessionKey.Interface), sessionKey.VRF, ErrAuthMismatch,
			describeClientProfile(edited), describeClientProfile(holder), authEditRefusedWhy,
			authEditFix(edited, holder))
	}
	if !sameAuth(entry.auth, requests[0].Auth) {
		if err := replaceAuth(entry, authRequest(requests), sessionKey); err != nil {
			return err
		}
	}
	for i, client := range entry.clients {
		client.req = requests[i]
	}
	// RFC 5882 Section 4.1
	retimeLocked(entry)
	return nil
}

// authRequest answers the request whose authentication a session takes when
// its clients' edited requests agree on it (reprofileEntryLocked): the first
// that asks for sequence persistence (a pinned session's persist-dir), so an
// edit does not drop it because a protocol client joined first, else the
// first. requests MUST NOT be empty.
func authRequest(requests []api.SessionRequest) api.SessionRequest {
	for _, req := range requests {
		if req.PersistDir != "" {
			return req
		}
	}
	return requests[0]
}

// updateClientLocked gives client of entry the request req: the
// authentication first, refused while another client relies on the current
// one (updateAuthLocked), then the timers of the fastest client
// (retimeLocked). The caller MUST hold l.mu.
func updateClientLocked(entry *sessionEntry, client *sessionClient, req api.SessionRequest, sessionKey api.Key) error {
	if err := updateAuthLocked(entry, client, req, sessionKey); err != nil {
		return err
	}
	client.req = req
	// RFC 5882 Section 4.1
	retimeLocked(entry)
	return nil
}

// updateAuthLocked gives entry the authentication of client's edited request
// req. A session runs one authentication for all its clients (joinAuthCheck),
// so the change is refused while another client relies on the current one: a
// client naming another bfd profile, or none, whose request asks for other
// authentication than req. A client naming the same profile takes the same
// edit from Loop.Reprofile at the same apply. The caller MUST hold l.mu.
//
// RFC 5882 Section 10.2: "BFD authentication SHOULD be used and is strongly
// encouraged." A client may only run on a session whose authentication it
// asked for.
func updateAuthLocked(entry *sessionEntry, client *sessionClient, req api.SessionRequest, key api.Key) error {
	if sameAuth(entry.auth, req.Auth) {
		return nil
	}
	for _, held := range entry.clients {
		if held == client {
			continue
		}
		if req.Profile != "" && held.req.Profile == req.Profile {
			continue
		}
		if sameAuth(held.req.Auth, req.Auth) {
			continue
		}
		return fmt.Errorf("bfd: session to peer %s%s vrf %s: %w: after this commit %s asks for "+
			"another auth block than %s, which shares the session; %s; to fix it, %s",
			key.Peer, refusalInterface(key.Interface), key.VRF, ErrAuthMismatch,
			describeEditedClient(req.Profile), describeClientProfile(held.req.Profile),
			authEditRefusedWhy, authEditFix(req.Profile, held.req.Profile))
	}
	return replaceAuth(entry, req, key)
}

// authEditRefusedWhy says why an edit is refused and what the refusal leaves
// running. Unlike a refused join (authMismatchWhy), every client keeps its
// session: the edit is what is dropped, for the whole session, so its timers
// stay as they were before the commit too (reprofileEntryLocked installs every
// client's edit or none).
const authEditRefusedWhy = "one session per neighbor runs one authentication " +
	"(RFC 5882 Section 4.4), so this session keeps the settings it ran before " +
	"this commit, for every client, and the commit's BFD apply fails"

// authEditFix is the config change that lets a refused edit through. edited
// is the profile whose auth block the commit changed, or empty for a pinned
// session that now names no profile; holder is the profile of the client that
// relies on the current authentication, or empty for a client naming none.
//
// A client naming no profile is an OSPF interface, or a BGP peer, static
// route or pinned session that names none. An OSPF interface's bfd block names
// no profile and no auth block (ospf bfdRequestForNeighbor), and the engine
// does not know which protocol a client is, so the fix that names a profile
// on that client carries the OSPF alternative beside it, as the join refusal
// does (authMismatchCase).
func authEditFix(edited, holder string) string {
	if edited == "" {
		if holder == "" {
			// Two clients naming no profile both run without
			// authentication (sameAuth), so no edit between them is
			// refused; this answer is never printed.
			return authReleaseFix
		}
		return "name " + clientProfile(holder) + " on this pinned session, or remove the " +
			"auth block from " + clientProfile(holder)
	}
	if holder == "" {
		return "restore the previous auth block of " + clientProfile(edited) + ", or name " +
			clientProfile(edited) + " on the client that names no bfd profile (on an OSPF " +
			"interface, which cannot name one, set bfd { enabled false; } instead)"
	}
	return "give " + clientProfile(edited) + " and " + clientProfile(holder) + " the same auth block"
}

// describeEditedClient names the pinned session whose edit UpdateSession
// refused, by the profile it now names.
func describeEditedClient(profile string) string {
	if profile == "" {
		return "this pinned session, which names no bfd profile,"
	}
	return "this pinned session's " + clientProfile(profile)
}

// describeClientProfile names a client by the bfd profile it asked with, for
// a refusal an operator reads.
func describeClientProfile(profile string) string {
	if profile == "" {
		return "a client naming no bfd profile"
	}
	return "a client of bfd profile " + strconv.Quote(profile)
}

// retimeLocked gives entry the timers, the role and the echo interval of the
// fastest of its current clients (fastestRequest), and names that client's
// profile as the session's. It runs
// whenever a client joins, leaves or edits its request, so the session's
// timers are a property of the clients present, never of their order or of
// the session's history. A session with no client left keeps what it runs:
// it is released, and its timers no longer serve anyone. The caller MUST hold
// l.mu.
//
// RFC 5882 Section 4.1: "Note that all protocols sharing a session will
// operate using the same parameters. The mechanism for choosing the
// parameters among those desired by the various protocols is outside the
// scope of this specification. It is generally useful to choose the
// parameters resulting in the shortest Detection Time".
func retimeLocked(entry *sessionEntry) {
	if len(entry.clients) == 0 {
		return
	}
	fastest := fastestRequest(entry.clients)
	entry.profile = fastest.Profile
	// RFC 5880 Section 6.8.3
	entry.machine.ApplyTimers(fastest)
	// RFC 5880 Section 6.1
	entry.machine.ApplyRole(fastest.Passive)
	// RFC 5880 Section 6.8.13
	entry.machine.ApplyEcho(fastest.DesiredMinEchoTxInterval)
}

// fastestRequest answers the request of clients giving the shortest Detection
// Time: the smallest Detect Mult times Required Min RX Interval, which is the
// Detection Time a remote system using the same parameters gives this end
// (RFC 5880 Section 6.8.4). A tie goes to the smaller Detect Mult times
// Desired Min TX Interval, the remote system's Detection Time of this end, and
// then to the profile name, so the answer is the same whatever order the
// clients joined in. A zero field reads as the default the session machine
// gives it (session.RequestTimers). clients MUST NOT be empty.
func fastestRequest(clients []*sessionClient) api.SessionRequest {
	best := clients[0].req
	for _, client := range clients[1:] {
		if fasterRequest(client.req, best) {
			best = client.req
		}
	}
	return best
}

// fasterRequest reports whether candidate ranks before incumbent in the
// fastestRequest order.
func fasterRequest(candidate, incumbent api.SessionRequest) bool {
	candidateTxUs, candidateRxUs, candidateMult := session.RequestTimers(candidate)
	incumbentTxUs, incumbentRxUs, incumbentMult := session.RequestTimers(incumbent)
	candidateRx := uint64(candidateMult) * uint64(candidateRxUs)
	incumbentRx := uint64(incumbentMult) * uint64(incumbentRxUs)
	if candidateRx != incumbentRx {
		return candidateRx < incumbentRx
	}
	candidateTx := uint64(candidateMult) * uint64(candidateTxUs)
	incumbentTx := uint64(incumbentMult) * uint64(incumbentTxUs)
	if candidateTx != incumbentTx {
		return candidateTx < incumbentTx
	}
	return candidate.Profile < incumbent.Profile
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
