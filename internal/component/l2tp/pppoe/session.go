// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session table and SID allocation
// Related: server.go -- InterfaceServer uses SessionTable for session lifecycle
// RFC: rfc/short/rfc2516.md -- session ID scope, per-peer session count
//
// RFC 2516 Section 4: session ID 0 is reserved (used in discovery);
// valid session IDs are 1-65535, scoped per access interface.

package pppoe

import (
	"bytes"
	"errors"
	"math/bits"
	"net"
	"sync"
	"time"
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

// bitmapWords is the number of uint64 words needed to cover SIDs 0-65535.
// SID 0 is permanently marked as allocated (reserved).
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
	PppoxFD   int // -1 when not set or already closed
	UnitNum   int
	State     SessionState
	CreatedAt time.Time
}

// SessionTable manages PPPoE sessions for a single access interface.
// Each interface gets its own table with an independent SID space
// (full 1-65535 range), unlike accel-ppp's global bitmap.
type SessionTable struct {
	mu       sync.Mutex
	bitmap   [bitmapWords]uint64
	hint     int // word index to start scanning from
	sessions map[uint16]*Session

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
	if maxSessions <= 0 || maxSessions > maxSID {
		maxSessions = maxSID
	}
	st := &SessionTable{
		sessions:    make(map[uint16]*Session),
		byMAC:       make(map[[6]byte]map[uint16]*Session),
		maxSessions: maxSessions,
		ifName:      ifName,
	}
	// Mark all bits as free (1 = free, 0 = allocated).
	for i := range st.bitmap {
		st.bitmap[i] = ^uint64(0)
	}
	// SID 0 is reserved: clear bit 0 in word 0.
	st.bitmap[0] &^= 1
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

	if sid == 0 {
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

// Remove deletes a session from the table, frees its SID, and returns
// the PppoxFD that was associated with the session (-1 if none or
// already removed). The caller is responsible for closing the fd via
// closePPPoxFD. Returning the fd instead of closing it here keeps
// the session table free of kernel dependencies.
func (st *SessionTable) Remove(sid uint16) int {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[sid]
	if !ok {
		return -1
	}

	pppoxFD := s.PppoxFD
	s.PppoxFD = -1

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
		return pppoxFD
	}
	word := int(sid) / 64
	bit := uint(sid) % 64
	st.bitmap[word] |= 1 << bit
	return pppoxFD
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
// not in the table.
//
// The write happens under st.mu, the same lock matchLiveCookie takes to read
// State: handleSessionDown runs on the PPP driver's event-consumer
// goroutine, while a replayed PADR's dedup match runs on the discovery-
// reader goroutine, so without a shared lock the two would race on the field
// (spec-pppoe-padr-replay-allocates-unbounded-sessions R-2). handlePADT
// needs no such call: it runs on the discovery-reader goroutine itself, the
// same one that runs every dedup match, so the two can never interleave.
func (st *SessionTable) markTeardown(sid uint16) *Session {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[sid]
	if !ok {
		return nil
	}
	s.State = StateTeardown
	return s
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
