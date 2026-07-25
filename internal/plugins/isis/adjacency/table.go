// Design: plan/learned/931-isis-5-adjacency.md -- per-circuit neighbor table + snapshot.
// ISO/IEC 10589 section 8.2: a circuit holds one adjacency per (level, neighbor
// System ID) on a LAN, and one adjacency per level on a point-to-point link.
//
// The table is the single writer of adjacency records on one circuit. The
// circuit goroutine owns it; all mutation happens from that one goroutine, and
// Snapshot takes a read lock so the CLI can read concurrently. Records survive a
// grace period after going Down (absorbing transient flaps) before deletion.

package adjacency

import (
	"sort"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// DefaultGracePeriod is how long a Down adjacency is retained before deletion,
// to absorb transient flaps without re-learning churn (bio-rd uses 120s).
const DefaultGracePeriod = 120 * time.Second

// MaxNeighbors caps the per-circuit adjacency count so a flood of distinct
// source System IDs cannot exhaust memory (security review: resource
// exhaustion). A real LAN segment never carries this many IS neighbors.
const MaxNeighbors = 1024

// adjKey identifies one adjacency within a circuit: the neighbor System ID and
// the level. On P2P there is one neighbor, but an L1L2 circuit can hold an L1
// and an L2 adjacency to it, so the level is part of the key.
type adjKey struct {
	sys   types.SystemID
	level Level
}

// Table is the per-circuit neighbor table.
type Table struct {
	mu   sync.RWMutex
	adjs map[adjKey]*Adjacency
}

// NewTable constructs an empty neighbor table.
func NewTable() *Table {
	return &Table{adjs: make(map[adjKey]*Adjacency)}
}

// Get returns the adjacency for (sys, level), creating a fresh Down record when
// none exists. The boolean reports whether a new record was created (so the
// caller can enforce MaxNeighbors). A nil adjacency with created=false is
// returned when the table is full and the record is new.
func (t *Table) Get(sys types.SystemID, level Level) (adj *Adjacency, created bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := adjKey{sys: sys, level: level}
	if a, ok := t.adjs[k]; ok {
		return a, false
	}
	if len(t.adjs) >= MaxNeighbors {
		return nil, false
	}
	a := &Adjacency{SystemID: sys, Level: level, State: StateDown}
	t.adjs[k] = a
	return a, true
}

// Update runs fn against the adjacency for (sys, level) while holding the table
// write lock, creating a fresh Down record when none exists. It is the
// single-point mutation API: the circuit's RX path and timer sweep both mutate
// through Update (or Each), so all writes to a record are serialized under one
// lock and never race. fn must NOT call back into the table (Get/Snapshot/Each),
// which would deadlock. The boolean passed to fn reports whether the record was
// just created. When the table is full and the record is new, fn is not called
// and ok=false is returned.
func (t *Table) Update(sys types.SystemID, level Level, fn func(adj *Adjacency, created bool)) (ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := adjKey{sys: sys, level: level}
	a, exists := t.adjs[k]
	if !exists {
		if len(t.adjs) >= MaxNeighbors {
			return false
		}
		a = &Adjacency{SystemID: sys, Level: level, State: StateDown}
		t.adjs[k] = a
	}
	fn(a, !exists)
	return true
}

// Lookup returns the adjacency for (sys, level) without creating one. The
// boolean reports presence.
func (t *Table) Lookup(sys types.SystemID, level Level) (*Adjacency, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	a, ok := t.adjs[adjKey{sys: sys, level: level}]
	return a, ok
}

// Each iterates every adjacency under the write lock, invoking fn for each. Used
// by the circuit's hold-timer sweep and circuit-down teardown. fn may mutate the
// adjacency in place (it holds the same lock the single-writer circuit uses).
func (t *Table) Each(fn func(*Adjacency)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, a := range t.adjs {
		fn(a)
	}
}

// Len returns the number of records currently in the table (including Down
// records still inside the grace period).
func (t *Table) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.adjs)
}

// Reap deletes Down records whose grace period has elapsed (deleteAt <= now). It
// returns the number of records removed. Called from the circuit's periodic
// sweep after Expire so the grace period is honored before deletion.
func (t *Table) Reap(now time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	removed := 0
	for k, a := range t.adjs {
		if a.State == StateDown && !a.deleteAt.IsZero() && !now.Before(a.deleteAt) {
			delete(t.adjs, k)
			removed++
		}
	}
	return removed
}

// Clear removes every adjacency record from the table and returns how many were
// dropped. It is the `clear isis adjacency` primitive (spec-isis-13 AC-8): the
// operator forces every neighbor to re-learn from the next Hello, without
// reconfiguring or closing the circuit. Records simply vanish; the circuit
// goroutine recreates them as Down on the next received IIH and walks the FSM
// back to Up. The caller is responsible for any side effects of losing an Up
// adjacency (LSP re-origination, SPF re-run) via the circuit's down hook.
func (t *Table) Clear() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(t.adjs)
	t.adjs = make(map[adjKey]*Adjacency)
	return n
}

// NeighborSnapshot is one row of the `show isis neighbor` view (rendered by
// spec-isis-13). It is a flat value with no pointers so it crosses the CLI
// boundary cleanly. Addresses are rendered as strings ("" when none was learned).
type NeighborSnapshot struct {
	SystemID   string `json:"system-id"`
	SNPA       string `json:"snpa"`
	Level      string `json:"level"`
	State      string `json:"state"`
	IPv4       string `json:"ipv4,omitempty"`
	IPv6       string `json:"ipv6,omitempty"`
	HoldTime   uint16 `json:"hold-time"`
	HoldExpiry int64  `json:"hold-expiry-unix"` // Unix seconds; 0 when Down
}

// Snapshot returns a stable, sorted copy of the current adjacencies for the CLI.
// It takes a read lock so it never blocks the single-writer circuit for long and
// never exposes a live pointer across the boundary (the snapshot is by value).
func (t *Table) Snapshot() []NeighborSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]NeighborSnapshot, 0, len(t.adjs))
	for _, a := range t.adjs {
		out = append(out, snapshotOf(a))
	}
	// Deterministic order: by System ID then level, so the CLI output is stable.
	sort.Slice(out, func(i, j int) bool {
		if out[i].SystemID != out[j].SystemID {
			return out[i].SystemID < out[j].SystemID
		}
		return out[i].Level < out[j].Level
	})
	return out
}

// Snapshot renders this single adjacency as a NeighborSnapshot row WITHOUT
// taking the table lock. It is safe to call from inside Table.Each (which holds
// the table lock) or from the single-writer circuit goroutine that owns the
// record; do NOT call it concurrently with a write to the same record from
// another goroutine. The circuit uses it to build an event payload during a
// transition without re-locking the table (which would deadlock).
func (a *Adjacency) Snapshot() NeighborSnapshot { return snapshotOf(a) }

// snapshotOf renders one adjacency as a NeighborSnapshot row.
func snapshotOf(a *Adjacency) NeighborSnapshot {
	row := NeighborSnapshot{
		SystemID: a.SystemID.String(),
		SNPA:     snpaString(a.SNPA),
		Level:    a.Level.String(),
		State:    a.State.String(),
		HoldTime: a.HoldTime,
	}
	if a.IPv4.IsValid() {
		row.IPv4 = a.IPv4.String()
	}
	if a.IPv6.IsValid() {
		row.IPv6 = a.IPv6.String()
	}
	if a.State != StateDown && !a.HoldExpiry.IsZero() {
		row.HoldExpiry = a.HoldExpiry.Unix()
	}
	return row
}

// snpaString renders a SNPA as colon-separated lowercase hex; the zero SNPA
// (P2P, where there is no LAN MAC) renders as "".
func snpaString(s SNPA) string {
	if s == (SNPA{}) {
		return ""
	}
	const hexDigits = "0123456789abcdef"
	var b [SNPALen*3 - 1]byte
	o := 0
	for i, c := range s {
		if i != 0 {
			b[o] = ':'
			o++
		}
		b[o] = hexDigits[c>>4]
		b[o+1] = hexDigits[c&0x0f]
		o += 2
	}
	return string(b[:o])
}

// UpCount returns the number of adjacencies currently in the Up state, used to
// drive the ze_isis_adjacencies_up gauge.
func (t *Table) UpCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := 0
	for _, a := range t.adjs {
		if a.State == StateUp {
			n++
		}
	}
	return n
}
