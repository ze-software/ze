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

type Session struct {
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
	closeDone    chan struct{}
	closeStarted bool
}

// sessionLease closes an unused local lease once, whether bridge preparation
// or access teardown wins. Only setOpened creates it with a validated handle.
// Close is safe for concurrent use; a nil *sessionLease is invalid.
type sessionLease struct {
	handle ppp.SessionHandle
	once   sync.Once
	err    error
}

func (lease *sessionLease) Close() error {
	lease.once.Do(func() { lease.err = lease.handle.Close() })
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
	s.closeDone = make(chan struct{})
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

// markTeardown sets sid's state to StateTeardown and returns the session,
// which stays in the table -- the caller removes it separately, once its own
// teardown work (such as sending the PADT) is done. Returns nil when sid is
// absent or another caller already owns its teardown.
//
// The write happens under st.mu, the same lock matchLiveCookie takes to read
// State: handleSessionDown runs on the PPP driver's event-consumer
// goroutine, while a replayed PADR's dedup match runs on the discovery-
// reader goroutine, so without a shared lock the two would race on the field
// (spec-pppoe-padr-replay-allocates-unbounded-sessions R-2). A received PADT
// claims teardown through the same lock, so it cannot free a SID whose
// outbound PADT is still being written.
func (st *SessionTable) markTeardown(sid uint16) *Session {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[sid]
	if !ok || s.State == StateTeardown {
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
func (st *SessionTable) setOpened(sid uint16, opened dataplane.OpenedSession) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	session := st.sessions[sid]
	if session == nil {
		return false
	}
	if session.State == StateTeardown {
		return false
	}
	session.Opened = opened
	session.unusedLease = &sessionLease{handle: opened.Dataplane()}
	return true
}

// takePPPLease transfers the server's retained local lease. Local PPP MUST
// take it before queueing StartSession. A nil result means this admission no
// longer owns a transferable lease, so the caller MUST NOT start local PPP.
func (st *SessionTable) takePPPLease(sid uint16) *sessionLease {
	st.mu.Lock()
	defer st.mu.Unlock()
	session := st.sessions[sid]
	// refuse-by-default: guards handlePADR -- a removed admission cannot transfer a lease into a new PPP session
	if session == nil {
		return nil
	}
	if session.State == StateTeardown {
		return nil
	}
	lease := session.unusedLease
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
// its once-owner so bridge preparation and teardown can close outside the lock.
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

// sids returns the session id of every session in the table, in no order.
// Reload uses it to end each session on an access interface it removes.
func (st *SessionTable) sids() []uint16 {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]uint16, 0, len(st.sessions))
	for sid := range st.sessions {
		out = append(out, sid)
	}
	return out
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
