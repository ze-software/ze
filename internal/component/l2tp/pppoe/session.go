// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session table and SID allocation
// Related: server.go -- InterfaceServer uses SessionTable for session lifecycle
// RFC: rfc/short/rfc2516.md -- session ID scope, per-peer session count
//
// RFC 2516 Section 4: session ID 0 is reserved (used in discovery) and
// "A value of 0xffff is reserved for future use and MUST NOT be used";
// valid session IDs are 1-65534, scoped per access interface.

package pppoe

import (
	"bytes"
	"errors"
	"math/bits"
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/callsink"
)

var (
	ErrSIDExhausted    = errors.New("pppoe: no free session IDs")
	ErrSessionExists   = errors.New("pppoe: session already exists")
	ErrMaxSessions     = errors.New("pppoe: max sessions reached")
	ErrSessionNotFound = errors.New("pppoe: session not found")
)

type SessionState uint8

const (
	StateDiscovery SessionState = iota
	StateSession
	StateTeardown
)

const maxSID = 65535

// RFC 2516 Section 4: "A value of 0xffff is reserved for future use and
// MUST NOT be used". The bit stays allocated for the table's lifetime,
// like SID 0, so AllocSID can never hand it out and freeSID never
// reopens it.
const reservedSID = 0xffff

// usableSIDs is how many session IDs one table can hand out: 1 to 0xfffe.
const usableSIDs = maxSID - 1

// bitmapWords is the number of uint64 words needed to cover SIDs 0-65535.
// SID 0 and SID 0xffff are permanently marked as allocated (reserved).
const bitmapWords = (maxSID + 1) / 64

// PADSPublication is the one table-owned monotonic discovery receipt.
// Attempted means PADS may have escaped, including a sender panic/error.
// Submitted means the real kernel send returned nil; it is not a peer ACK.
type PADSPublication uint8

const (
	PADSUnspecified PADSPublication = iota
	PADSNotAttempted
	PADSAttempted
	PADSSubmitted
)

type Session struct {
	padsPublication PADSPublication // table.mu; survives StateTeardown

	// SubscriberID and AccessIfIndex are fixed once from the complete PADR access.
	SubscriberID  string
	AccessIfIndex int
	// Attempt is published with Opened before handoff; Control is bound under table.mu.
	Attempt ppp.SessionAttempt
	Control ppp.SessionControl
	// Transfer intent precedes queueing; only the exact terminal consumer ends it.
	pppTransferred   bool
	pppTerminal      pppTerminalState
	pppRelayReleased bool
	wireEnd          *padtPublication // table.mu; nil until one handler claims this wire end
	// A refused transfer with unresolved cleanup remains owned by this admission.
	pppRefusal  *ppp.EventSessionRejected
	pppNotice   chan struct{} // actual terminal/refusal observation, including failure
	SID         uint16
	MAC         net.HardwareAddr
	IfName      string
	ServiceName string
	HostUniq    []byte
	// Cookie is the AC-Cookie tag value carried by the PADR that admitted
	// this session, copied so the table owns it independently of the
	// packet buffer. RFC 2516 places no per-peer session limit
	// (rfc/short/rfc2516.md), so one MAC's several live sessions can each
	// hold a different cookie from a different PADI/PADO round trip.
	// matchLiveCookie compares an incoming PADR's cookie against this
	// field, which is what tells a retransmission of THIS session's own
	// admitting PADR apart from a distinct, concurrent request from the
	// same MAC.
	Cookie    []byte
	Opened    dataplane.OpenedSession
	State     SessionState
	CreatedAt time.Time
	// unusedLease belongs to the access server until local PPP takes it or
	// a Service-Name relay prepares its bridge. It is protected by the table.
	unusedLease *sessionLease
	// relay belongs to this exact admission; mutable fields use table.mu.
	relay *serviceBridgePreparation
	// closeDone joins this admission, including cleanup after it was claimed.
	// The table owns both fields; only external generation shutdown waits.
	closeDone       chan struct{}
	closeStarted    bool
	closeErr        error         // latest actual producer cleanup failure; owner stays attached
	closeNotice     chan struct{} // bounded retry/failure wake for a waiting drain
	bridgePreparing bool          // a closing local lease can never be handed to PPP
}

// padtPublication is the original handler's completion receipt for one exact
// admission. Its fields are immutable after claimPADT; only that successful
// caller may finish it. A nil receipt grants no publication ownership.
type padtPublication struct {
	session *Session
	done    chan struct{}
}

// sessionLease shares one cached first Close outcome among concurrent borrowers.
// A later claimed admission cleanup may explicitly retry this same unused owner.
// Only setOpened creates this lease; transferred leases never reach retryClose.
type sessionLease struct {
	handle    ppp.SessionHandle
	mu        sync.Mutex
	attempted bool
	closed    bool
	err       error
}

func (lease *sessionLease) Close() error {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.attempted {
		return lease.err
	}
	return lease.closeLocked()
}

// retryClose is a distinct later cleanup attempt, never an ordinary borrower.
// The caller MUST own the table's new closeStarted claim after an earlier
// actual cleanup failure, retain the same owner, and have no PPP transfer.
func (lease *sessionLease) retryClose() error {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.closed {
		return nil
	}
	return lease.closeLocked()
}

func (lease *sessionLease) closeLocked() error {
	lease.attempted = true
	// Preserve the original panic for the existing outer task boundary while
	// concurrent ordinary borrowers receive its same retained error outcome.
	defer func() {
		if failure := recover(); failure != nil {
			cause, ok := failure.(error)
			if !ok {
				cause = errors.New("pppoe: unused PPP lease Close panicked; release is unknown")
			}
			lease.err = cause
			panic(failure)
		}
	}()
	lease.err = lease.handle.Close()
	if lease.err == nil {
		if native, present := lease.handle.(ppp.NativeReleaseHandle); present && native.NativeReleasePending() {
			lease.err = errors.New("pppoe: unused PPP native release remains pending")
		}
	}
	lease.closed = lease.err == nil
	return lease.err
}

// SessionTable manages PPPoE sessions for a single access interface.
// Each interface gets its own table with an independent SID space
// (full 1-65534 range), unlike accel-ppp's global bitmap.
type SessionTable struct {
	mu       sync.Mutex
	bitmap   [bitmapWords]uint64
	hint     int // word index to start scanning from
	sessions map[uint16]*Session
	// relayEnds is bounded by admissions and removed with their ownership.
	relayEnds map[uint16]serviceRelayEnd

	// byMAC indexes every live session by subscriber MAC address. RFC 2516
	// places no per-peer session limit (rfc/short/rfc2516.md), so one MAC can
	// hold several sessions at once. The value is a set of sessions keyed by
	// SID, not a single pointer: a second session for a MAC must join the
	// set, never replace the first.
	byMAC       map[[6]byte]map[uint16]*Session
	maxSessions int
	ifName      string
}

func newSessionTable(ifName string, maxSessions int) *SessionTable {
	if maxSessions <= 0 || maxSessions > usableSIDs {
		maxSessions = usableSIDs
	}
	st := &SessionTable{
		sessions:    make(map[uint16]*Session),
		relayEnds:   make(map[uint16]serviceRelayEnd),
		byMAC:       make(map[[6]byte]map[uint16]*Session),
		maxSessions: maxSessions,
		ifName:      ifName,
	}
	// Mark all bits as free (1 = free, 0 = allocated).
	for i := range st.bitmap {
		st.bitmap[i] = ^uint64(0)
	}
	// SID 0 is reserved: clear bit 0 in word 0. SID 0xffff is reserved by
	// RFC 2516 Section 4 (see reservedSID): clear its bit in the last word.
	st.bitmap[0] &^= 1
	st.bitmap[reservedSID/64] &^= 1 << (reservedSID % 64)
	return st
}

// AllocSID finds and reserves the next free session ID.
// Scans from a rotating hint index for even distribution.
func (st *SessionTable) AllocSID() (uint16, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.sessions) >= st.maxSessions {
		return 0, ErrMaxSessions
	}

	start := st.hint
	for i := range bitmapWords {
		idx := (start + i) % bitmapWords
		word := st.bitmap[idx]
		if word == 0 {
			continue
		}
		bit := bits.TrailingZeros64(word)
		st.bitmap[idx] &^= 1 << uint(bit)
		// Advance hint past this word when it's exhausted.
		if st.bitmap[idx] == 0 {
			st.hint = (idx + 1) % bitmapWords
		} else {
			st.hint = idx
		}
		return uint16(idx*64 + bit), nil
	}
	return 0, ErrSIDExhausted
}

// freeSID returns a session ID to the free pool.
func (st *SessionTable) freeSID(sid uint16) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if sid == 0 || sid == reservedSID {
		return
	}
	word := int(sid) / 64
	bit := uint(sid) % 64
	st.bitmap[word] |= 1 << bit
}

// Add registers a session in the table. The session's SID must
// already be allocated via AllocSID. Returns ErrSessionExists
// if a session with the same SID is already present.
func (st *SessionTable) Add(s *Session) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, exists := st.sessions[s.SID]; exists {
		return ErrSessionExists
	}
	s.padsPublication = PADSNotAttempted
	s.closeDone = make(chan struct{})
	s.pppNotice = make(chan struct{})
	s.closeNotice = make(chan struct{}, 1)
	st.sessions[s.SID] = s
	if len(s.MAC) == 6 {
		var key [6]byte
		copy(key[:], s.MAC)
		set := st.byMAC[key]
		if set == nil {
			set = make(map[uint16]*Session)
			st.byMAC[key] = set
		}
		set[s.SID] = s
	}
	return nil
}

// Remove deletes a session, frees its SID, and transfers its admitted owner
// to the caller. A missing entry returns nil. The caller MUST close the owner;
// the table itself performs no transport or data-plane operation.
func (st *SessionTable) Remove(sid uint16) dataplane.OpenedSession {
	return st.removeAdmitted(sid).opened
}

// removedAdmission carries both ownership roles out of the table in one
// operation. Its zero value means there was no admission to remove.
type removedAdmission struct {
	retryUnused bool // captured under table.mu from a prior actual producer failure
	opened      dataplane.OpenedSession
	unusedLease *sessionLease
	relay       *serviceBridgePreparation
}

// removeAdmitted atomically detaches the transport and any unused local lease.
// Callers MUST finish any outbound PADT before this frees the SID, and MUST
// release the returned ownership after this method releases the table lock.
func (st *SessionTable) removeAdmitted(sid uint16) removedAdmission {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.removeAdmittedLocked(sid)
}

// removeAdmittedLocked releases table ownership after its caller finished cleanup.
// Caller MUST hold mu. Closing admissions use completeAdmissionClose instead.
func (st *SessionTable) removeAdmittedLocked(sid uint16) removedAdmission {

	s, ok := st.sessions[sid]
	// refuse-by-default: guards closeAdmitted -- repeated teardown has no admission to close
	if !ok {
		return removedAdmission{}
	}

	removed := removedAdmission{opened: s.Opened, unusedLease: s.unusedLease, relay: s.relay}
	s.Opened = nil
	s.unusedLease = nil
	s.relay = nil
	delete(st.relayEnds, sid)

	delete(st.sessions, sid)
	if len(s.MAC) == 6 {
		var key [6]byte
		copy(key[:], s.MAC)
		if set, exists := st.byMAC[key]; exists {
			delete(set, sid)
			if len(set) == 0 {
				delete(st.byMAC, key)
			}
		}
	}

	if sid == 0 {
		return removed
	}
	word := int(sid) / 64
	bit := uint(sid) % 64
	st.bitmap[word] |= 1 << bit
	return removed
}

// Lookup returns the session for the given SID, or nil.
func (st *SessionTable) Lookup(sid uint16) *Session {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.sessions[sid]
}

// markTeardown sets the captured admission's state to StateTeardown and returns it,
// which stays in the table -- the caller removes it separately, once its own
// teardown work (such as sending the PADT) is done. A later genuine PPP terminal
// event may complete that same teardown; wire publication and final resource
// cleanup have their own claims. An absent, replaced or closing row returns nil.
//
// The write happens under st.mu, the same lock matchLiveCookie takes to read
// State: handleSessionDown runs on the PPP driver's event-consumer
// goroutine, while a replayed PADR's dedup match runs on the discovery-
// reader goroutine, so without a shared lock the two would race on the field
// (spec-pppoe-padr-replay-allocates-unbounded-sessions R-2). A received PADT
// marks teardown through the same lock. Its wire-end receipt then keeps the
// SID reserved while an outbound PADT is still being written.
func (st *SessionTable) markTeardown(expected *Session) *Session {
	if expected == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[expected.SID]
	if !ok || s != expected || s.closeStarted {
		return nil
	}
	s.State = StateTeardown
	return s
}

// transport returns the admitted owner while the SID stays reserved. The
// owner's idempotent CloseTransport serializes competing teardown requests.
func (st *SessionTable) transport(sid uint16) dataplane.OpenedSession {
	st.mu.Lock()
	defer st.mu.Unlock()
	session := st.sessions[sid]
	// refuse-by-default: guards closeTransport -- an absent session has no transport; repeated teardown must not touch another owner
	if session == nil {
		return nil
	}
	return session.Opened
}

// setOpened publishes an immutable session owner before PPP can emit events.
func (st *SessionTable) setOpened(expected *Session, opened dataplane.OpenedSession, attempt ppp.SessionAttempt) bool {
	if expected == nil {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	session := st.sessions[expected.SID]
	if session != expected {
		return false
	}
	if session.State == StateTeardown {
		return false
	}
	session.Opened = opened
	session.Attempt = attempt
	session.unusedLease = &sessionLease{handle: opened.Dataplane()}
	return true
}

// takePPPLease transfers the server's retained local lease. Local PPP MUST
// take it before queueing StartSession. A nil result means this admission no
// longer owns a transferable lease, so the caller MUST NOT start local PPP.
func (st *SessionTable) takePPPLease(expected *Session) *sessionLease {
	if expected == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	session := st.sessions[expected.SID]
	// refuse-by-default: guards handlePADR -- only this captured admission can transfer its lease
	if session != expected {
		return nil
	}
	if session.State == StateTeardown || session.bridgePreparing {
		return nil
	}
	lease := session.unusedLease
	if lease == nil {
		return nil
	}
	session.pppTransferred = true
	session.unusedLease = nil
	return lease
}

// serviceBridgePreparation names the exact admission whose local lease the
// worker releases. Keeping that owner prevents a delayed worker from acting on
// a new subscriber that reused the SID. PrepareBridge is safe for concurrent use.
type serviceBridgePreparation struct {
	sessions *SessionTable
	sid      uint16
	opened   dataplane.OpenedSession
	wake     chan<- struct{}
	// call is captured once after Relay returns, protected by sessions.mu.
	call callsink.RelayCall
}

func (preparation *serviceBridgePreparation) PrepareBridge() error {
	lease, err := preparation.sessions.bridgeLease(preparation.sid, preparation.opened)
	if err != nil {
		return err
	}
	return lease.Close()
}

// bridgeLease checks the original admission under the table lock, then lends
// its exact shared owner so preparation and teardown close outside the lock.
func (st *SessionTable) bridgeLease(sid uint16, opened dataplane.OpenedSession) (*sessionLease, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	session := st.sessions[sid]
	if session == nil {
		return nil, ErrSessionNotFound
	}
	if session.Opened != opened {
		return nil, ErrSessionNotFound
	}
	if session.State == StateTeardown {
		return nil, ErrSessionNotFound
	}
	if _, err := opened.RelayDescriptor(); err != nil {
		return nil, err
	}
	if session.unusedLease == nil {
		return nil, errors.New("pppoe: local PPP already owns this session; an L2TP bridge cannot take its channel")
	}
	session.bridgePreparing = true
	return session.unusedLease, nil
}

// markSession moves sid from StateDiscovery to StateSession. It leaves every
// other current state alone: a session the event-consumer goroutine has
// already marked StateTeardown stays torn down, because the PADR that
// admitted a session is never the thing that decides the session is alive
// again. There is nothing for the caller to do about either outcome, which is
// why this returns nothing -- the teardown owns the session from the moment
// it is marked, and handlePADR's remaining work (starting PPP, or handing the
// subscriber to the L2TP relay) is bounded by that teardown either way.
//
// The write goes through st.mu because markTeardown's write and
// matchLiveCookie's read of the same field do, and a third accessor outside
// that lock would leave the field unsynchronized however careful the other
// two are: handlePADR runs on the discovery-reader goroutine and
// handleSessionDown on the PPP driver's event-consumer goroutine, and a SID
// freed by Remove is reallocated immediately, so a session-down event still
// queued for the previous holder of that SID reaches the new one while
// handlePADR is still setting it up.
func (st *SessionTable) markSession(sid uint16) {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[sid]
	if !ok {
		return
	}
	if s.State != StateDiscovery {
		return
	}
	s.State = StateSession
	if s.relay != nil {
		if _, pending := st.relayEnds[sid]; pending {
			s.relay.notify()
		}
	}
}

// matchLiveCookie returns the SID of the session held by mac whose stored
// Cookie equals cookie and whose state is not StateTeardown, or (0, false)
// when none matches. The search and the state check both run under st.mu,
// the lock markTeardown and Remove also hold, so a session already marked
// for teardown -- or already removed -- on another goroutine is never
// handed back to a concurrent caller (spec-pppoe-padr-replay-allocates-
// unbounded-sessions R-2).
//
// Matching on the cookie, not on the MAC and state alone, is what lets one
// MAC hold several live sessions at once: RFC 2516 places no per-peer
// session limit (rfc/short/rfc2516.md), so a MAC can be admitting a second,
// distinct session (its own fresh PADI/PADO cookie) at the same moment a
// replay of its first session's PADR arrives, and only the cookie tells
// those two requests apart.
//
// A cookie's HMAC is bucketed to the second (cookie.go, GenerateCookie), so
// two distinct PADI/PADO rounds for the same MAC pair and relay id within
// the same second produce byte-identical cookies, and more than one live
// session can then match here. Go's map iteration order is unspecified, so
// ranging st.byMAC[key] does not itself pick one of them deterministically;
// this function breaks the tie on the lower SID, so the answer never depends
// on iteration order.
func (st *SessionTable) matchLiveCookie(mac net.HardwareAddr, cookie []byte) (uint16, bool) {
	if len(mac) != 6 || len(cookie) == 0 {
		return 0, false
	}
	var key [6]byte
	copy(key[:], mac)

	st.mu.Lock()
	defer st.mu.Unlock()

	var matchSID uint16
	matched := false
	for _, s := range st.byMAC[key] {
		if s.State == StateTeardown {
			continue
		}
		if !bytes.Equal(s.Cookie, cookie) {
			continue
		}
		if !matched || s.SID < matchSID {
			matchSID = s.SID
			matched = true
		}
	}
	return matchSID, matched
}

// sessionsByMAC returns every session held by the given subscriber MAC
// address, live or tearing down, in no particular order, or nil when the MAC
// holds none. RFC 2516 places no per-peer session limit, so one MAC can hold
// several sessions at once; a caller that needs to answer a specific PADR
// uses matchLiveCookie instead, which excludes a session in teardown and
// correlates on the cookie rather than trusting iteration order.
func (st *SessionTable) sessionsByMAC(mac net.HardwareAddr) []*Session {
	if len(mac) != 6 {
		return nil
	}
	var key [6]byte
	copy(key[:], mac)

	st.mu.Lock()
	defer st.mu.Unlock()
	set := st.byMAC[key]
	if len(set) == 0 {
		return nil
	}
	out := make([]*Session, 0, len(set))
	for _, s := range set {
		out = append(out, s)
	}
	return out
}

// Count returns the number of active sessions.
func (st *SessionTable) Count() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.sessions)
}

// setMaxSessions changes the admission cap, with the clamp newSessionTable
// applies. Sessions already above a lowered cap stay up: the cap refuses the
// next AllocSID, it never ends a live session.
func (st *SessionTable) setMaxSessions(maxSessions int) {
	if maxSessions <= 0 || maxSessions > usableSIDs {
		maxSessions = usableSIDs
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.maxSessions = maxSessions
}

// restoreUnsentPPPLease runs only when queueing panics before its send returns.
// No Driver acquired that send, so only this producer regains cleanup ownership.
func (st *SessionTable) restoreUnsentPPPLease(expected *Session, lease *sessionLease) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil || st.sessions[expected.SID] != expected || expected.closeStarted {
		return
	}
	expected.pppTransferred = false
	expected.unusedLease = lease
}

// claimPADT grants the first handler ownership of this exact wire end. A nil
// result refuses an absent, replaced, closing or already claimed admission.
// A successful caller MUST defer finishPADT before running the wire-end work.
func (st *SessionTable) claimPADT(expected *Session) *padtPublication {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil {
		return nil
	}
	if st.sessions[expected.SID] != expected {
		return nil
	}
	if expected.closeStarted {
		return nil
	}
	if expected.wireEnd != nil {
		return nil
	}
	expected.wireEnd = &padtPublication{session: expected, done: make(chan struct{})}
	return expected.wireEnd
}

// localPPPAvailable excludes a lease whose bridge preparation already began
// closing it, even when a failed relay asks the caller to fall back locally.
func (st *SessionTable) localPPPAvailable(expected *Session) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return expected != nil && st.sessions[expected.SID] == expected && expected.State != StateTeardown && !expected.bridgePreparing && expected.unusedLease != nil
}

// PADSPublication returns the original owner's immutable progress snapshot.
func (st *SessionTable) PADSPublication(expected *Session) (PADSPublication, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil || st.sessions[expected.SID] != expected {
		return PADSUnspecified, false
	}
	return expected.padsPublication, true
}

func (st *SessionTable) notePADS(expected *Session, publication PADSPublication) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil || st.sessions[expected.SID] != expected || expected.closeStarted {
		return false
	}
	if publication != PADSAttempted && publication != PADSSubmitted {
		return false
	}
	if expected.padsPublication < PADSNotAttempted || expected.padsPublication > PADSSubmitted {
		return false
	}
	if publication == PADSAttempted && expected.State == StateTeardown {
		return false
	}
	if publication > expected.padsPublication {
		expected.padsPublication = publication
	}
	return true
}

// markSessionFor finishes only the captured admitting PADS, never a current
// occupant found later by its wire SID. A pending relay end wakes only after
// this exact admission leaves discovery, so PADT follows its accepting PADS.
func (st *SessionTable) markSessionFor(expected *Session) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil {
		return
	}
	if st.sessions[expected.SID] != expected {
		return
	}
	if expected.State != StateDiscovery {
		return
	}
	expected.State = StateSession
	if expected.relay != nil {
		if _, pending := st.relayEnds[expected.SID]; pending {
			expected.relay.notify()
		}
	}
}
