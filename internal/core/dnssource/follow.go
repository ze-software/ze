// Design: docs/features/dns-resolver.md -- the dns-source list and the learned record
// Related: record.go -- the Snapshot a Follower decodes
// Related: select.go -- the rule a consumer applies to the records

package dnssource

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
)

// Follower is a consumer's view of the system-dns namespace: it
// takes each snapshot (decoded from JSON in a plugin, typed in the daemon), keeps the
// records of the newest one, and drops a snapshot older than the last it
// kept (Snapshot, "a subscriber MUST drop a snapshot whose Sequence is
// below the last one it applied"). The zero Follower holds no snapshot and
// is ready to use. Safe for concurrent use.
type Follower struct {
	mu       sync.Mutex
	held     bool
	sequence uint64
	records  []Record
}

// Accept decodes one (system-dns, snapshot) event and answers whether it is
// newer than the last one kept. A stale or repeated snapshot answers false
// and changes nothing, so the consumer re-evaluates only on a change. A body
// that does not decode is an error and changes nothing either.
func (f *Follower) Accept(event string) (fresh bool, err error) {
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(event), &snapshot); err != nil {
		return false, fmt.Errorf("system-dns snapshot does not decode: %w", err)
	}
	return f.AcceptSnapshot(&snapshot), nil
}

// AcceptSnapshot keeps snapshot when it is newer than the last one kept and
// answers whether it was, like Accept. A consumer in the daemon calls it from
// its SnapshotEvent handler, where the bus hands the typed value. The record
// slice is copied; the producer MUST NOT change a snapshot it emitted.
func (f *Follower) AcceptSnapshot(snapshot *Snapshot) (fresh bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.held && snapshot.Sequence <= f.sequence {
		return false
	}
	f.held = true
	f.sequence = snapshot.Sequence
	f.records = slices.Clone(snapshot.Records)
	return true
}

// Records answers the records of the newest snapshot kept, in received-class
// order; nil before the first. The slice is a copy the caller may keep.
func (f *Follower) Records() []Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.records)
}

// Held reports whether any snapshot was kept. A consumer that evaluates a
// list naming received before the first snapshot sees no record, which is
// the state before any lease, so it MAY evaluate then and again on arrival.
func (f *Follower) Held() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.held
}
