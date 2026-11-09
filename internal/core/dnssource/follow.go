// Design: docs/features/dns-resolver.md -- the dns-source list and the learned record
// Related: record.go -- the Snapshot a Follower decodes
// Related: select.go -- the rule a consumer applies to the records

package dnssource

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// RejectedMetricName is the counter of the system-dns snapshots a consumer
// received and could not apply, labelled by consumer (the name it publishes its selections under). A
// consumer that rejects a snapshot logs it at WARN, counts it here, and
// answers its event handler with no error: a handler error ends the SDK's
// batched delivery (pkg/plugin/sdk activateBridge, OnEvent), so a newer
// snapshot queued behind the rejected one would never be applied.
const RejectedMetricName = "ze_dns_source_snapshots_rejected_total"

// RejectedCounter answers consumer's series of RejectedMetricName in reg.
// Every consumer registers the same vector, which reg hands back to each.
func RejectedCounter(reg metrics.Registry, consumer string) metrics.Counter {
	return reg.CounterVec(RejectedMetricName,
		"System-dns snapshots a consumer received and could not apply, by consumer.",
		[]string{"consumer"}).With(consumer)
}

// Follower is a consumer's view of the system-dns namespace: it
// takes each snapshot (decoded from JSON in a plugin, typed in the daemon), keeps the
// records of the newest one, and drops a snapshot older than the last it
// kept (Snapshot, "a subscriber MUST drop a snapshot whose Sequence is
// below the last one it applied"). The zero Follower holds no snapshot and
// is ready to use. Safe for concurrent use.
//
// A consumer that answers clients from the records also holds them until the
// replay arrives (ExpectReplay, Replayed, AwaitReplay), so a plugin that
// starts or restarts while leases exist never answers from the empty list it
// starts with (spec-home-cpe-23b AC-13).
type Follower struct {
	mu       sync.Mutex
	held     bool
	sequence uint64
	records  []Record
	// replay is closed once the consumer applied the replay the engine
	// answers its snapshot-request with, or the replay can no longer arrive.
	// nil when no replay is expected (the zero Follower, a test's): then
	// AwaitReplay never waits. replayDone records the close, so a second
	// Replayed does not close it twice.
	replay     chan struct{}
	replayDone bool
}

// ReplayWaitMax bounds how long AwaitReplay holds a caller. The engine
// answers a snapshot-request in milliseconds; past the bound the consumer
// answers from what it holds rather than leave a client without an answer
// until its own timeout, and every later caller passes at once.
const ReplayWaitMax = 2 * time.Second

// ExpectReplay makes AwaitReplay hold its callers until Replayed runs. It
// MUST be called before the consumer answers any client (before its plugin
// runs), and Replayed MUST then be called once the consumer applied the
// first snapshot it accepted, or once the replay can no longer arrive (the
// snapshot-request was not sent).
func (f *Follower) ExpectReplay() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replay == nil {
		f.replay = make(chan struct{})
	}
}

// Replayed releases every caller AwaitReplay holds, and every later one. It
// MUST be called after ExpectReplay, and only once the snapshot is applied:
// a released caller reads what the consumer then holds. The consumer calls
// it rather than Accept, because the records stored here are not always
// what the consumer answers from (the resolver builds a policy from them).
// Safe for concurrent use, and for repeated calls.
func (f *Follower) Replayed() {
	f.mu.Lock()
	defer f.mu.Unlock()
	// No channel: no replay is expected, so no caller waits to be released.
	if f.replay == nil {
		return
	}
	if f.replayDone {
		return
	}
	f.replayDone = true
	close(f.replay)
}

// AwaitReplay holds the caller until Replayed ran or ReplayWaitMax passed,
// and at the bound releases every caller. A Follower that expects no replay
// never holds. Once released it costs one lock and one non-blocking channel
// receive. Safe for concurrent use.
func (f *Follower) AwaitReplay() {
	f.mu.Lock()
	replay := f.replay
	f.mu.Unlock()
	if replay == nil {
		return
	}
	select {
	case <-replay:
		return
	default:
	}

	timer := time.NewTimer(ReplayWaitMax)
	defer timer.Stop()

	select {
	case <-replay:
	case <-timer.C:
		f.Replayed()
	}
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
