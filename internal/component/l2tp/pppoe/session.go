// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session table and SID allocation
// Related: server.go -- InterfaceServer uses SessionTable for session lifecycle
//
// RFC 2516 Section 4: session ID 0 is reserved (used in discovery);
// valid session IDs are 1-65535, scoped per access interface.

package pppoe

import (
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
	PppoxFD     int // -1 when not set or already closed
	UnitNum     int
	State       SessionState
	CreatedAt   time.Time
}

// SessionTable manages PPPoE sessions for a single access interface.
// Each interface gets its own table with an independent SID space
// (full 1-65535 range), unlike accel-ppp's global bitmap.
type SessionTable struct {
	mu          sync.Mutex
	bitmap      [bitmapWords]uint64
	hint        int // word index to start scanning from
	sessions    map[uint16]*Session
	byMAC       map[[6]byte]*Session
	maxSessions int
	ifName      string
}

func newSessionTable(ifName string, maxSessions int) *SessionTable {
	if maxSessions <= 0 || maxSessions > maxSID {
		maxSessions = maxSID
	}
	st := &SessionTable{
		sessions:    make(map[uint16]*Session),
		byMAC:       make(map[[6]byte]*Session),
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
		st.byMAC[key] = s
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
		if cur, exists := st.byMAC[key]; exists && cur.SID == sid {
			delete(st.byMAC, key)
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

// lookupByMAC returns the session for the given subscriber MAC, or nil.
func (st *SessionTable) lookupByMAC(mac net.HardwareAddr) *Session {
	if len(mac) != 6 {
		return nil
	}
	var key [6]byte
	copy(key[:], mac)

	st.mu.Lock()
	defer st.mu.Unlock()
	return st.byMAC[key]
}

// Count returns the number of active sessions.
func (st *SessionTable) Count() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.sessions)
}
