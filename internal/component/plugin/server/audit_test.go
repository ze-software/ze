// Design: docs/architecture/fleet-config.md -- unit tests for hub fleet audit log store, persistence, receipts, and authorization
// Related: audit.go -- ManagedAuditStore, generation pinning, receipt ingestion, and query filtering

package server

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

type auditFailStore struct {
	storage.Storage
	mu           sync.Mutex
	failWriteKey func(key string) error
	blockWrite   chan struct{}
	onWriteStart func(key string)
}

func newAuditFailStore() *auditFailStore {
	return &auditFailStore{Storage: storage.CreateMemory()}
}

func (f *auditFailStore) AcquireLock(key string) (storage.WriteGuard, error) {
	guard, err := f.Storage.AcquireLock(key)
	if err != nil {
		return nil, err
	}
	return &auditFailGuard{WriteGuard: guard, parent: f}, nil
}

func (f *auditFailStore) setFailWrite(fn func(key string) error) {
	f.mu.Lock()
	f.failWriteKey = fn
	f.mu.Unlock()
}

type auditFailGuard struct {
	storage.WriteGuard
	parent *auditFailStore
}

func (g *auditFailGuard) WriteKey(key string, data []byte) error {
	g.parent.mu.Lock()
	fn := g.parent.failWriteKey
	block := g.parent.blockWrite
	onStart := g.parent.onWriteStart
	g.parent.mu.Unlock()

	if onStart != nil {
		onStart(key)
	}
	if block != nil {
		<-block
	}
	if fn != nil {
		if err := fn(key); err != nil {
			return err
		}
	}
	return g.WriteGuard.WriteKey(key, data)
}

type countingScopeAuth struct {
	mu      sync.Mutex
	allowed map[string]bool
	calls   map[string]int
}

func newCountingScopeAuth(allowed map[string]bool) *countingScopeAuth {
	return &countingScopeAuth{
		allowed: allowed,
		calls:   make(map[string]int),
	}
}

func (c *countingScopeAuth) Authorize(string, string, string, bool) bool {
	return false
}

func (c *countingScopeAuth) AuthorizeLocalScope(_, _ string, args []string, _ bool) bool {
	device := ""
	if len(args) >= 2 {
		device = args[1]
	}
	c.mu.Lock()
	c.calls[device]++
	ok := c.allowed[device]
	c.mu.Unlock()
	return ok
}

func (c *countingScopeAuth) callCount(device string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[device]
}

func allowAllAuditScope() FleetDeviceScope {
	return FleetDeviceScope{mode: fleetScopeAllowAll}
}

func startTestAuditStore(t *testing.T, cfg ManagedAuditStoreConfig) *ManagedAuditStore {
	t.Helper()
	store, err := NewManagedAuditStore(cfg)
	require.NoError(t, err)
	require.NoError(t, store.LoadFromStore())
	store.Start(context.Background())
	t.Cleanup(func() { store.Stop() })
	return store
}

// VALIDATES: Appended entries receive monotonic hub sequences, sanitize and
// clone metadata, accept future producer-agnostic dot-separated kebab-case
// event types, and are returned newest-first by Query.
// PREVENTS: Mutating caller metadata after admission, hardcoded producer lists,
// or wrong chronological ordering.
func TestAuditLogAppend(t *testing.T) {
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store: storage.CreateMemory(),
		Now:   func() time.Time { return t0 },
	})

	meta := map[string]string{"template": "edge-base", "rollout": "r-42"}
	ok := store.Append(FleetAuditEntry{
		EventTime: t0,
		Type:      FleetAuditDeviceConnect,
		Device:    "edge-01",
		Actor:     "system",
		Surface:   "system",
		Outcome:   "success",
		Summary:   "connected\nfrom peer",
		Metadata:  meta,
	})
	require.True(t, ok)
	// Mutating caller map after Append must not alter stored metadata.
	meta["template"] = "mutated"

	// Producer-agnostic future event type (fleet-2 / fleet-5 / fleet-9).
	ok = store.Append(FleetAuditEntry{
		EventTime: t0.Add(time.Second),
		Type:      "template.commit",
		Device:    "edge-01",
		Actor:     "alice",
		Surface:   "web",
		Outcome:   "success",
		Version:   "a1b2c3d4e5f60708",
	})
	require.True(t, ok)

	// Invalid event type is refused.
	assert.False(t, store.Append(FleetAuditEntry{Type: "BadType"}))
	assert.False(t, store.Append(FleetAuditEntry{Type: "nodot"}))

	require.NoError(t, store.Flush(context.Background()))

	rows := store.Query(allowAllAuditScope(), FleetAuditQuery{})
	require.Len(t, rows, 2)
	assert.Equal(t, uint64(2), rows[0].Seq)
	assert.Equal(t, "template.commit", rows[0].Type)
	assert.Equal(t, "alice", rows[0].Actor)
	assert.Equal(t, "a1b2c3d4e5f60708", rows[0].Version)

	assert.Equal(t, uint64(1), rows[1].Seq)
	assert.Equal(t, FleetAuditDeviceConnect, rows[1].Type)
	assert.Equal(t, "connected from peer", rows[1].Summary)
	assert.Equal(t, "edge-base", rows[1].Metadata["template"])

	// Mutating returned metadata must not mutate internal generation.
	rows[1].Metadata["template"] = "tampered"
	again := store.Query(allowAllAuditScope(), FleetAuditQuery{})
	assert.Equal(t, "edge-base", again[1].Metadata["template"])
}

// VALIDATES: Audit entries and monotonic sequence numbers (including equal
// timestamps) survive a hub restart from the live ZeFS store without sequence
// collision.
// PREVENTS: Lost audit history or colliding sequences after daemon restart.
func TestAuditLogPersist(t *testing.T) {
	dir := t.TempDir()
	backed, err := storage.Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backed.Close()) })

	t0 := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)

	first, err := NewManagedAuditStore(ManagedAuditStoreConfig{
		Store: backed,
		Now:   func() time.Time { return t0 },
	})
	require.NoError(t, err)
	require.NoError(t, first.LoadFromStore())
	first.Start(context.Background())

	// Append multiple equal-timestamp events across > 1 page (>128 rows).
	for i := range 150 {
		require.True(t, first.Append(FleetAuditEntry{
			EventTime: t0,
			Type:      FleetAuditConfigFetch,
			Device:    "edge-01",
			Actor:     "system",
			Surface:   "system",
			Outcome:   "success",
			Version:   fmt.Sprintf("%016x", i+1),
		}))
	}
	first.Stop()

	// Reopen a fresh audit store instance against the same live ZeFS store.
	second := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store: backed,
		Now:   func() time.Time { return t0 },
	})

	rows := second.Query(allowAllAuditScope(), FleetAuditQuery{})
	require.Len(t, rows, 150)
	assert.Equal(t, uint64(150), rows[0].Seq)
	assert.Equal(t, uint64(1), rows[149].Seq)

	// Append one more equal-timestamp event after restart: sequence continues at 151.
	require.True(t, second.Append(FleetAuditEntry{
		EventTime: t0,
		Type:      FleetAuditConfigAck,
		Device:    "edge-01",
		Actor:     "system",
		Surface:   "system",
		Outcome:   "success",
	}))
	require.NoError(t, second.Flush(context.Background()))

	after := second.Query(allowAllAuditScope(), FleetAuditQuery{Limit: 3})
	require.Len(t, after, 3)
	assert.Equal(t, uint64(151), after[0].Seq)
	assert.Equal(t, uint64(150), after[1].Seq)
	assert.Equal(t, uint64(149), after[2].Seq)
}

// VALIDATES: Filtering by exact device returns only that device's rows across
// all event types.
// PREVENTS: Cross-device leakage in device-filtered audit queries.
func TestAuditLogFilterDevice(t *testing.T) {
	t0 := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store: storage.CreateMemory(),
		Now:   func() time.Time { return t0 },
	})

	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditDeviceConnect, Device: "edge-01"}))
	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditDeviceConnect, Device: "edge-02"}))
	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "edge-01"}))
	require.NoError(t, store.Flush(context.Background()))

	rows := store.Query(allowAllAuditScope(), FleetAuditQuery{Device: "edge-01"})
	require.Len(t, rows, 2)
	assert.Equal(t, FleetAuditConfigFetch, rows[0].Type)
	assert.Equal(t, "edge-01", rows[0].Device)
	assert.Equal(t, FleetAuditDeviceConnect, rows[1].Type)
	assert.Equal(t, "edge-01", rows[1].Device)
}

// VALIDATES: Filtering by exact event type returns only matching events across
// all authorized devices.
// PREVENTS: Unrelated event types appearing in a type-filtered audit query.
func TestAuditLogFilterType(t *testing.T) {
	t0 := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store: storage.CreateMemory(),
		Now:   func() time.Time { return t0 },
	})

	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditDeviceConnect, Device: "edge-01"}))
	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "edge-01"}))
	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "edge-02"}))
	require.NoError(t, store.Flush(context.Background()))

	rows := store.Query(allowAllAuditScope(), FleetAuditQuery{Type: FleetAuditConfigFetch})
	require.Len(t, rows, 2)
	assert.Equal(t, "edge-02", rows[0].Device)
	assert.Equal(t, "edge-01", rows[1].Device)
}

// VALIDATES: Since and Until bounds are inclusive on EventTime.
// PREVENTS: Off-by-one exclusions on exact boundary timestamps (AC-10).
func TestAuditLogFilterSince(t *testing.T) {
	t1 := time.Date(2026, 5, 14, 23, 59, 59, 0, time.UTC)
	t2 := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 5, 15, 0, 0, 1, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store: storage.CreateMemory(),
		Now:   func() time.Time { return t2 },
	})

	require.True(t, store.Append(FleetAuditEntry{EventTime: t1, Type: FleetAuditDeviceConnect, Device: "edge-01"}))
	require.True(t, store.Append(FleetAuditEntry{EventTime: t2, Type: FleetAuditConfigFetch, Device: "edge-01"}))
	require.True(t, store.Append(FleetAuditEntry{EventTime: t3, Type: FleetAuditConfigAck, Device: "edge-01"}))
	require.NoError(t, store.Flush(context.Background()))

	rows := store.Query(allowAllAuditScope(), FleetAuditQuery{Since: t2})
	require.Len(t, rows, 2)
	assert.Equal(t, FleetAuditConfigAck, rows[0].Type)
	assert.Equal(t, FleetAuditConfigFetch, rows[1].Type)

	bounded := store.Query(allowAllAuditScope(), FleetAuditQuery{Since: t2, Until: t2})
	require.Len(t, bounded, 1)
	assert.Equal(t, FleetAuditConfigFetch, bounded[0].Type)
}

// VALIDATES: Retention pruning trims oldest entries at max capacity and when
// SetMaxEntries lowers the retention ceiling, removing superseded ZeFS pages.
// PREVENTS: Unbounded audit growth or stale pages leaking in ZeFS.
func TestAuditLogPrune(t *testing.T) {
	mem := storage.CreateMemory()
	t0 := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store:      mem,
		MaxEntries: 150,
		Now:        func() time.Time { return t0 },
	})

	for i := range 220 {
		require.True(t, store.Append(FleetAuditEntry{
			EventTime: t0.Add(time.Duration(i) * time.Second),
			Type:      FleetAuditConfigPush,
			Device:    "edge-01",
		}))
	}
	require.NoError(t, store.Flush(context.Background()))

	rows := store.Query(allowAllAuditScope(), FleetAuditQuery{})
	require.Len(t, rows, 150)
	assert.Equal(t, uint64(220), rows[0].Seq)
	assert.Equal(t, uint64(71), rows[149].Seq)

	// Reduce retention ceiling dynamically and verify immediate prune + GC.
	require.NoError(t, store.SetMaxEntries(100))
	reduced := store.Query(allowAllAuditScope(), FleetAuditQuery{})
	require.Len(t, reduced, 100)
	assert.Equal(t, uint64(220), reduced[0].Seq)
	assert.Equal(t, uint64(121), reduced[99].Seq)

	health := store.Health()
	assert.Equal(t, 100, health.RetainedEntries)
	assert.Equal(t, 100, health.MaxEntries)
	assert.Equal(t, uint64(221), health.NextSeq)

	// Only 1 page is needed for 100 entries (<=128); obsolete pages were GC'd.
	keys, err := mem.ListKeys(zefs.FleetAuditPagePrefix)
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	// Out-of-range retention change is refused cleanly.
	assert.Error(t, store.SetMaxEntries(99))
	assert.Error(t, store.SetMaxEntries(100001))
}

// VALIDATES: Append never blocks when the writer is stalled on slow storage and
// reports queue overflow drops accurately in Health.
// PREVENTS: Slow or stuck storage blocking the managed config-fetch path.
func TestAuditLogAsyncNonBlocking(t *testing.T) {
	failStore := newAuditFailStore()
	writeStarted := make(chan struct{}, 1)
	failStore.blockWrite = make(chan struct{})
	failStore.onWriteStart = func(string) {
		select {
		case writeStarted <- struct{}{}:
		default:
		}
	}

	store, err := NewManagedAuditStore(ManagedAuditStoreConfig{Store: failStore})
	require.NoError(t, err)
	store.Start(context.Background())
	t.Cleanup(func() {
		select {
		case <-failStore.blockWrite:
		default:
			close(failStore.blockWrite)
		}
		store.Stop()
	})

	// Trigger one write so the writer goroutine is blocked inside WriteKey.
	require.True(t, store.Append(FleetAuditEntry{Type: FleetAuditConfigFetch, Device: "edge-01"}))
	<-writeStarted

	// Flood more than FleetAuditQueueSize entries; every call must return
	// immediately without blocking, and excess entries must increment QueueDropped.
	done := make(chan int, 1)
	go func() {
		dropped := 0
		for range FleetAuditQueueSize + 50 {
			if !store.Append(FleetAuditEntry{Type: FleetAuditConfigFetch, Device: "edge-01"}) {
				dropped++
			}
		}
		done <- dropped
	}()

	select {
	case dropped := <-done:
		assert.GreaterOrEqual(t, dropped, 50)
		health := store.Health()
		assert.Equal(t, uint64(dropped), health.QueueDropped)
		assert.False(t, health.LastDropAt.IsZero())
	case <-time.After(2 * time.Second):
		t.Fatal("Append blocked while writer was stalled on storage")
	}

	// Unblock writer and verify health retains QueueDropped evidence even after
	// subsequent writes succeed.
	close(failStore.blockWrite)
	require.NoError(t, store.Flush(context.Background()))
	assert.GreaterOrEqual(t, store.Health().QueueDropped, uint64(50))
}

// VALIDATES: Failed ZeFS writes keep previously published rows readable, report
// WriteFailed in Health, and clear WriteFailed (while retaining WriteFailCount)
// once a later write succeeds.
// PREVENTS: Audit persistence errors corrupting visible history or leaving
// stale failure alarms stuck forever after recovery.
func TestAuditLogWriteFailureRecovery(t *testing.T) {
	failStore := newAuditFailStore()
	t0 := time.Date(2026, 5, 15, 1, 0, 0, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store: failStore,
		Now:   func() time.Time { return t0 },
	})

	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditDeviceConnect, Device: "edge-01"}))
	require.NoError(t, store.Flush(context.Background()))
	require.Len(t, store.Query(allowAllAuditScope(), FleetAuditQuery{}), 1)

	// Inject manifest write failure.
	failStore.setFailWrite(func(key string) error {
		if key == zefs.KeyFleetAuditManifest.Pattern {
			return errors.New("injected disk full")
		}
		return nil
	})
	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "edge-01"}))
	require.NoError(t, store.Flush(context.Background()))

	hFail := store.Health()
	assert.True(t, hFail.WriteFailed)
	assert.Equal(t, uint64(1), hFail.WriteFailCount)
	assert.Contains(t, hFail.LastWriteError, "injected disk full")
	assert.False(t, hFail.LastWriteFailAt.IsZero())
	// Previously published row remains readable.
	rows := store.Query(allowAllAuditScope(), FleetAuditQuery{})
	require.Len(t, rows, 1)
	assert.Equal(t, FleetAuditDeviceConnect, rows[0].Type)

	// Clear write failure and verify subsequent publish clears WriteFailed.
	failStore.setFailWrite(nil)
	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigAck, Device: "edge-01"}))
	require.NoError(t, store.Flush(context.Background()))

	hOK := store.Health()
	assert.False(t, hOK.WriteFailed)
	assert.Empty(t, hOK.LastWriteError)
	assert.Equal(t, uint64(1), hOK.WriteFailCount)
	assert.False(t, hOK.LastWriteOKAt.IsZero())
	rows = store.Query(allowAllAuditScope(), FleetAuditQuery{})
	require.Len(t, rows, 2)
	assert.Equal(t, FleetAuditConfigAck, rows[0].Type)
}

// VALIDATES: Startup tolerates corrupt individual pages and corrupt manifests
// without crashing, keeps healthy pages readable, preserves NextSeq/NextPageID
// strictly above readable data, and garbage-collects orphaned page keys.
// PREVENTS: One damaged key rendering all audit history unreadable or causing
// sequence collisions after recovery.
func TestAuditLogCorruptRecoveryAndOrphanGC(t *testing.T) {
	t0 := time.Date(2026, 5, 15, 2, 0, 0, 0, time.UTC)

	t.Run("corrupt page skipped and orphan page GCd", func(t *testing.T) {
		mem := storage.CreateMemory()
		first, err := NewManagedAuditStore(ManagedAuditStoreConfig{
			Store: mem,
			Now:   func() time.Time { return t0 },
		})
		require.NoError(t, err)
		first.Start(context.Background())
		for range 130 {
			require.True(t, first.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "edge-01"}))
		}
		first.Stop()

		// Find the two page keys, corrupt the first, and plant an orphan page.
		keys, err := mem.ListKeys(zefs.FleetAuditPagePrefix)
		require.NoError(t, err)
		require.Len(t, keys, 2)
		require.NoError(t, mem.WriteKey(keys[0], []byte("{corrupt json")))
		orphanKey := zefs.KeyFleetAuditPage.Key("0000000000000099")
		require.NoError(t, mem.WriteKey(orphanKey, []byte(`{"version":1,"page-id":153,"entries":[]}`)))

		recovered := startTestAuditStore(t, ManagedAuditStoreConfig{
			Store: mem,
			Now:   func() time.Time { return t0 },
		})

		health := recovered.Health()
		assert.True(t, health.Unreadable)
		assert.Equal(t, uint64(1), health.UnreadableCount)
		assert.Equal(t, uint64(131), health.NextSeq)

		// Healthy second page (2 entries: seq 129, 130) remains readable.
		rows := recovered.Query(allowAllAuditScope(), FleetAuditQuery{})
		require.Len(t, rows, 2)
		assert.Equal(t, uint64(130), rows[0].Seq)
		assert.Equal(t, uint64(129), rows[1].Seq)

		// Orphan page was removed on load.
		_, err = mem.ReadKey(orphanKey)
		assert.Error(t, err)

		// New append continues monotonically at seq 131 without colliding.
		require.True(t, recovered.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigAck, Device: "edge-01"}))
		require.NoError(t, recovered.Flush(context.Background()))
		after := recovered.Query(allowAllAuditScope(), FleetAuditQuery{Limit: 1})
		require.Len(t, after, 1)
		assert.Equal(t, uint64(131), after[0].Seq)
	})

	t.Run("corrupt manifest falls back to healthy pages", func(t *testing.T) {
		mem := storage.CreateMemory()
		first, err := NewManagedAuditStore(ManagedAuditStoreConfig{
			Store: mem,
			Now:   func() time.Time { return t0 },
		})
		require.NoError(t, err)
		first.Start(context.Background())
		for range 5 {
			require.True(t, first.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "edge-01"}))
		}
		first.Stop()

		require.NoError(t, mem.WriteKey(zefs.KeyFleetAuditManifest.Pattern, []byte("not-json")))

		recovered := startTestAuditStore(t, ManagedAuditStoreConfig{
			Store: mem,
			Now:   func() time.Time { return t0 },
		})
		health := recovered.Health()
		assert.True(t, health.Unreadable)
		assert.GreaterOrEqual(t, health.UnreadableCount, uint64(1))
		assert.Equal(t, uint64(6), health.NextSeq)

		rows := recovered.Query(allowAllAuditScope(), FleetAuditQuery{})
		require.Len(t, rows, 5)
		assert.Equal(t, uint64(5), rows[0].Seq)
	})
}

// VALIDATES: IngestReceipts binds hub actor to device:<name>, preserves peer
// actor/surface only in metadata, deduplicates retries across lost acks and
// even after retention aging, records overflow gaps once, commits only
// contiguous prefixes, refuses non-contiguous starts, and does not advance
// cursor on store write error.
// PREVENTS: Duplicate offline disable/enable rows, spoofed hub actors, or
// premature cursor advancement on persistence failures (AC-14).
func TestAuditLogIngestReceipts(t *testing.T) {
	failStore := newAuditFailStore()
	t0 := time.Date(2026, 5, 15, 3, 0, 0, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store:      failStore,
		MaxEntries: 100,
		Now:        func() time.Time { return t0 },
	})

	// Non-contiguous start without declared loss gap is rejected.
	_, err := store.IngestReceipts("edge-01", fleet.AuditReceiptBatch{
		Epoch: "ep-1",
		Entries: []fleet.AuditReceiptEntry{
			{Seq: 2, Action: fleet.FleetAuditReceiptActionDisable, Actor: "root", Surface: "cli", EventAt: t0, Outcome: "success"},
		},
	}, t0.Add(time.Minute))
	require.Error(t, err)

	// Store write failure must NOT advance cursor or acknowledge.
	failStore.setFailWrite(func(string) error { return errors.New("write refused") })
	batch1 := fleet.AuditReceiptBatch{
		Epoch: "ep-1",
		Entries: []fleet.AuditReceiptEntry{
			{Seq: 1, Action: fleet.FleetAuditReceiptActionDisable, Actor: "local-admin", Surface: "ssh", EventAt: t0, Outcome: "success"},
			{Seq: 2, Action: fleet.FleetAuditReceiptActionEnable, Actor: "local-admin", Surface: "cli", EventAt: t0.Add(10 * time.Second), Outcome: "success"},
		},
	}
	_, err = store.IngestReceipts("edge-01", batch1, t0.Add(time.Minute))
	require.Error(t, err)
	assert.Empty(t, store.Query(allowAllAuditScope(), FleetAuditQuery{}))

	// Retry after store recovers succeeds and binds hub actor to device:edge-01.
	failStore.setFailWrite(nil)
	ack, err := store.IngestReceipts("edge-01", batch1, t0.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, fleet.AuditReceiptAck{Epoch: "ep-1", Committed: 2}, ack)

	rows := store.Query(allowAllAuditScope(), FleetAuditQuery{Device: "edge-01"})
	require.Len(t, rows, 2)
	assert.Equal(t, FleetAuditFleetEnable, rows[0].Type)
	assert.Equal(t, "device:edge-01", rows[0].Actor)
	assert.Equal(t, "local-admin", rows[0].Metadata["device-actor"])
	assert.Equal(t, "cli", rows[0].Metadata["device-surface"])
	assert.Equal(t, FleetAuditFleetDisable, rows[1].Type)
	assert.Equal(t, "device:edge-01", rows[1].Actor)
	assert.Equal(t, "ssh", rows[1].Metadata["device-surface"])

	// Replay after simulated lost ack duplicates nothing.
	ackReplay, err := store.IngestReceipts("edge-01", batch1, t0.Add(2*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, fleet.AuditReceiptAck{Epoch: "ep-1", Committed: 2}, ackReplay)
	assert.Len(t, store.Query(allowAllAuditScope(), FleetAuditQuery{Device: "edge-01"}), 2)

	// Age out the two receipt rows past retention (100 entries) and replay again:
	// cursor in manifest must still deduplicate seq 1..2!
	for range 105 {
		require.True(t, store.Append(FleetAuditEntry{EventTime: t0.Add(3 * time.Minute), Type: FleetAuditConfigFetch, Device: "edge-02"}))
	}
	require.NoError(t, store.Flush(context.Background()))
	assert.Empty(t, store.Query(allowAllAuditScope(), FleetAuditQuery{Device: "edge-01"}))

	ackAgedReplay, err := store.IngestReceipts("edge-01", batch1, t0.Add(4*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, fleet.AuditReceiptAck{Epoch: "ep-1", Committed: 2}, ackAgedReplay)
	assert.Empty(t, store.Query(allowAllAuditScope(), FleetAuditQuery{Device: "edge-01"}))

	// Overflow gap covering seq 3..5 followed by seq 6 and non-contiguous seq 9:
	// records fleet.receipt.gap once, appends contiguous seq 6, ignores seq 9.
	gapBatch := fleet.AuditReceiptBatch{
		Epoch:   "ep-1",
		Lost:    3,
		GapFrom: 3,
		GapTo:   5,
		Entries: []fleet.AuditReceiptEntry{
			{Seq: 6, Action: fleet.FleetAuditReceiptActionDisable, Actor: "ops", Surface: "cli", EventAt: t0.Add(5 * time.Minute), Outcome: "success"},
			{Seq: 9, Action: fleet.FleetAuditReceiptActionEnable, Actor: "ops", Surface: "cli", EventAt: t0.Add(6 * time.Minute), Outcome: "success"},
		},
	}
	ackGap, err := store.IngestReceipts("edge-01", gapBatch, t0.Add(7*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, fleet.AuditReceiptAck{Epoch: "ep-1", Committed: 6}, ackGap)

	edge1Rows := store.Query(allowAllAuditScope(), FleetAuditQuery{Device: "edge-01"})
	require.Len(t, edge1Rows, 2)
	assert.Equal(t, FleetAuditFleetDisable, edge1Rows[0].Type)
	assert.Equal(t, FleetAuditReceiptGap, edge1Rows[1].Type)
	assert.Equal(t, "3", edge1Rows[1].Metadata["lost"])

	health := store.Health()
	assert.Equal(t, uint64(1), health.ReceiptGaps)
	assert.Equal(t, "edge-01", health.LastReceiptGapDevice)

	// Replaying the same gap batch does not duplicate the gap row or seq 6.
	ackGapReplay, err := store.IngestReceipts("edge-01", gapBatch, t0.Add(8*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, fleet.AuditReceiptAck{Epoch: "ep-1", Committed: 6}, ackGapReplay)
	assert.Len(t, store.Query(allowAllAuditScope(), FleetAuditQuery{Device: "edge-01"}), 2)
	assert.Equal(t, uint64(1), store.Health().ReceiptGaps)
}

// VALIDATES: Query checks per-device authorization via a per-query cache before
// applying caller filters, denies device-less rows to restricted callers, and
// fails closed on zero FleetDeviceScope.
// PREVENTS: Unauthorized device rows leaking through filters or repeated RBAC
// evaluations per row (AC-16).
func TestAuditLogPerDeviceAuthorizationAndCaching(t *testing.T) {
	t0 := time.Date(2026, 5, 15, 4, 0, 0, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store: storage.CreateMemory(),
		Now:   func() time.Time { return t0 },
	})

	for range 5 {
		require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "edge-01"}))
		require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "core-01"}))
	}
	require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: "template.commit", Device: ""}))
	require.NoError(t, store.Flush(context.Background()))

	// Zero scope fails closed.
	assert.Empty(t, store.Query(FleetDeviceScope{}, FleetAuditQuery{}))

	// Unrestricted scope sees all 11 rows including device-less hub row.
	assert.Len(t, store.Query(allowAllAuditScope(), FleetAuditQuery{}), 11)

	// Restricted scope sees only edge-01, never core-01 or device-less rows,
	// and evaluates each distinct device at most once per query.
	auth := newCountingScopeAuth(map[string]bool{"edge-01": true, "core-01": false})
	restricted := FleetDeviceScope{
		mode:      fleetScopeEvaluate,
		username:  "west-op",
		scopeAuth: auth,
	}

	rows := store.Query(restricted, FleetAuditQuery{})
	require.Len(t, rows, 5)
	for _, r := range rows {
		assert.Equal(t, "edge-01", r.Device)
	}
	assert.Equal(t, 1, auth.callCount("edge-01"))
	assert.Equal(t, 1, auth.callCount("core-01"))

	// Requesting an unauthorized device explicitly returns empty.
	assert.Empty(t, store.Query(restricted, FleetAuditQuery{Device: "core-01"}))
}

// VALIDATES: Concurrent readers pinning a published generation observe
// consistent snapshots while the writer compacts and prunes pages, and obsolete
// page keys are deleted only after active readers unpin.
// PREVENTS: Data races or premature deletion of pages while a query is reading.
func TestAuditLogConcurrentReadersAndGenerationPinning(t *testing.T) {
	mem := storage.CreateMemory()
	t0 := time.Date(2026, 5, 15, 5, 0, 0, 0, time.UTC)
	store := startTestAuditStore(t, ManagedAuditStoreConfig{
		Store:      mem,
		MaxEntries: 100,
		Now:        func() time.Time { return t0 },
	})

	for range 100 {
		require.True(t, store.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditConfigFetch, Device: "edge-01"}))
	}
	require.NoError(t, store.Flush(context.Background()))

	keysBefore, err := mem.ListKeys(zefs.FleetAuditPagePrefix)
	require.NoError(t, err)
	require.Len(t, keysBefore, 1)
	pinnedPageKey := keysBefore[0]

	// Pin generation directly while writer publishes a replacement page.
	pinned := store.pinGeneration()
	require.NotNil(t, pinned)

	require.True(t, store.Append(FleetAuditEntry{EventTime: t0.Add(time.Second), Type: FleetAuditConfigAck, Device: "edge-01"}))
	require.NoError(t, store.Flush(context.Background()))

	// Superseded page must still exist in ZeFS while generation is pinned.
	_, err = mem.ReadKey(pinnedPageKey)
	require.NoError(t, err, "obsolete page must not be deleted while a reader pins the old generation")

	// Unpin and flush GC: obsolete page is now deleted.
	store.unpinGeneration(pinned)
	require.NoError(t, store.Flush(context.Background()))
	_, err = mem.ReadKey(pinnedPageKey)
	assert.Error(t, err, "obsolete page should be deleted once unpinned")

	// Stress concurrent Query and Append under -race.
	var wg sync.WaitGroup
	var stopFlag atomic.Bool
	for range 4 {
		wg.Go(func() {
			for !stopFlag.Load() {
				got := store.Query(allowAllAuditScope(), FleetAuditQuery{Limit: 20})
				for i := 1; i < len(got); i++ {
					if got[i-1].Seq <= got[i].Seq {
						t.Errorf("non-descending sequence in snapshot: %d <= %d", got[i-1].Seq, got[i].Seq)
						return
					}
				}
			}
		})
	}
	for i := range 60 {
		store.Append(FleetAuditEntry{
			EventTime: t0.Add(time.Duration(i) * time.Millisecond),
			Type:      FleetAuditConfigPush,
			Device:    "edge-01",
			Summary:   strings.Repeat("x", 300),
		})
	}
	require.NoError(t, store.Flush(context.Background()))
	stopFlag.Store(true)
	wg.Wait()
}

// VALIDATES: Sequential async batches with tail compaction and obsolete-page GC
// persist cleanly on both disk tree and ZeFS blob backends, and survive restart.
// PREVENTS: Disk-only persistence regressions missed by in-memory storage tests.
func TestAuditLogDiskTreeAndBlobSequentialBatches(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)

	backends := []struct {
		name string
		open func(t *testing.T) (storage.Storage, func() (storage.Storage, error))
	}{
		{
			name: "tree",
			open: func(t *testing.T) (storage.Storage, func() (storage.Storage, error)) {
				dir := t.TempDir()
				s, err := storage.Create(dir)
				require.NoError(t, err)
				return s, func() (storage.Storage, error) { return storage.Open(dir) }
			},
		},
		{
			name: "blob",
			open: func(t *testing.T) (storage.Storage, func() (storage.Storage, error)) {
				path := filepath.Join(t.TempDir(), "audit.zefs")
				s, err := storage.CreateBlob(path)
				require.NoError(t, err)
				return s, func() (storage.Storage, error) { return storage.OpenBlob(path, true) }
			},
		},
	}

	for _, tc := range backends {
		t.Run(tc.name, func(t *testing.T) {
			disk, reopen := tc.open(t)
			audit, err := NewManagedAuditStore(ManagedAuditStoreConfig{
				Store:      disk,
				MaxEntries: 100,
				Now:        func() time.Time { return t0 },
			})
			require.NoError(t, err)
			require.NoError(t, audit.LoadFromStore())
			audit.Start(context.Background())

			require.True(t, audit.Append(FleetAuditEntry{EventTime: t0, Type: FleetAuditDeviceConnect, Device: "edge-01"}))
			require.NoError(t, audit.Flush(context.Background()))

			require.True(t, audit.Append(FleetAuditEntry{EventTime: t0.Add(time.Millisecond), Type: FleetAuditConfigFetch, Device: "edge-01", Version: "v1"}))
			require.True(t, audit.Append(FleetAuditEntry{EventTime: t0.Add(2 * time.Millisecond), Type: FleetAuditConfigAck, Device: "edge-01", Version: "v1"}))
			require.NoError(t, audit.Flush(context.Background()))

			health := audit.Health()
			assert.False(t, health.WriteFailed, "write error: %s", health.LastWriteError)
			assert.Equal(t, uint64(0), health.WriteFailCount)

			rows := audit.Query(allowAllAuditScope(), FleetAuditQuery{})
			require.Len(t, rows, 3)
			assert.Equal(t, FleetAuditConfigAck, rows[0].Type)
			assert.Equal(t, FleetAuditConfigFetch, rows[1].Type)
			assert.Equal(t, FleetAuditDeviceConnect, rows[2].Type)

			audit.Stop()
			require.NoError(t, disk.Close())

			disk2, err := reopen()
			require.NoError(t, err)
			defer disk2.Close() //nolint:errcheck // test cleanup

			audit2, err := NewManagedAuditStore(ManagedAuditStoreConfig{
				Store:      disk2,
				MaxEntries: 100,
				Now:        func() time.Time { return t0.Add(time.Minute) },
			})
			require.NoError(t, err)
			require.NoError(t, audit2.LoadFromStore())
			rows2 := audit2.Query(allowAllAuditScope(), FleetAuditQuery{})
			require.Len(t, rows2, 3)
			assert.Equal(t, uint64(3), rows2[0].Seq)
		})
	}
}
