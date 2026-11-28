// Design: docs/architecture/fleet-config.md -- centralized hub-side fleet audit log store, writer, and authorized query
// Related: managed_serve.go -- observes authenticated sessions, config offers, acks, and device receipt batches
// Related: managed_doctor.go -- exposes audit queue drop, write failure, unreadable history, and receipt gap health

package server

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

// Built-in fleet audit event type identifiers (dot-separated kebab-case).
// ManagedAuditStore is producer-agnostic and accepts any event type matching
// ValidFleetAuditEventType.
const (
	FleetAuditDeviceConnect    = "device.connect"
	FleetAuditDeviceDisconnect = "device.disconnect"
	FleetAuditConfigFetch      = "config.fetch"
	FleetAuditConfigPush       = "config.push"
	FleetAuditConfigAck        = "config.ack"
	FleetAuditConfigReject     = "config.reject"
	FleetAuditConfigHeld       = "config.held"
	FleetAuditDivergenceAdopt  = "divergence.adopt"
	FleetAuditDivergenceRevert = "divergence.revert"
	FleetAuditFleetDisable     = "fleet.disable"
	FleetAuditFleetEnable      = "fleet.enable"
	FleetAuditReceiptGap       = "fleet.receipt.gap"
)

// Capacity, layout, and field-length bounds for the hub fleet audit store.
const (
	// FleetAuditQueueSize is the bounded depth of the non-blocking async
	// admission channel drained by the single hub writer goroutine.
	FleetAuditQueueSize = 1024
	// FleetAuditPageMaxEntries is the maximum number of audit rows stored in
	// one immutable ZeFS page.
	FleetAuditPageMaxEntries = 128
	// FleetAuditLockKey is the runtime-state lock key serializing manifest and
	// page publication against ZeFS.
	FleetAuditLockKey = "meta/fleet/audit"

	// FleetAuditEventTypeMax bounds dot-separated kebab-case event type names.
	FleetAuditEventTypeMax = 64
	// FleetAuditFieldMax bounds Actor, Surface, Outcome, Device, Version,
	// PreviousVersion, and DiagnosticCode on admission.
	FleetAuditFieldMax = 128
	// FleetAuditSummaryMax bounds Summary on admission (UTF-8 safe).
	FleetAuditSummaryMax = 256
	// FleetAuditMetadataMaxEntries bounds the number of metadata pairs per row.
	FleetAuditMetadataMaxEntries = 16
	// FleetAuditMetadataKeyMax bounds each metadata key in bytes.
	FleetAuditMetadataKeyMax = 64
	// FleetAuditMetadataValueMax bounds each metadata value in bytes.
	FleetAuditMetadataValueMax = 128

	fleetAuditManifestVersion = 1
	fleetAuditDrainBatchMax   = 128
	fleetAuditTruncatedSuffix = "..."

	fleetAuditDefaultActor   = "system"
	fleetAuditDefaultSurface = "system"
	fleetAuditDefaultOutcome = "success"
	fleetAuditOutcomeError   = "error"
	fleetAuditDevicePrefix   = "device:"
	fleetAuditSurfaceDevice  = "device"
)

// ErrAuditStoreStopped is returned by synchronous store operations called after
// Stop has closed admission or after the writer context exited.
var ErrAuditStoreStopped = errors.New("fleet audit store is stopped")

// ErrAuditStoreNotStarted is returned by synchronous store operations called
// before Start has launched the single writer goroutine.
var ErrAuditStoreNotStarted = errors.New("fleet audit store is not started")

// FleetAuditEntry is one immutable structured event recorded in the centralized
// hub fleet audit log.
type FleetAuditEntry struct {
	Seq             uint64            `json:"seq"`
	EventTime       time.Time         `json:"event-time"`
	ReceiptTime     time.Time         `json:"receipt-time"`
	Type            string            `json:"type"`
	Device          string            `json:"device,omitempty"`
	Actor           string            `json:"actor"`
	Surface         string            `json:"surface"`
	Outcome         string            `json:"outcome"`
	Version         string            `json:"version,omitempty"`
	PreviousVersion string            `json:"previous-version,omitempty"`
	DiagnosticCode  string            `json:"diagnostic-code,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Clone returns a deep copy of e with its Metadata map cloned so callers never
// alias memory owned by a published generation.
func (e FleetAuditEntry) Clone() FleetAuditEntry {
	if len(e.Metadata) > 0 {
		e.Metadata = maps.Clone(e.Metadata)
	}
	return e
}

// FleetAuditQuery selects a filtered, newest-first slice of authorized audit
// rows. Since and Until are inclusive bounds evaluated against EventTime.
type FleetAuditQuery struct {
	Device string
	Type   string
	Since  time.Time
	Until  time.Time
	Limit  int
}

// ManagedAuditHealth is a value-type snapshot of hub audit queue drops, write
// failures, unreadable persisted history, device receipt gaps, and retention
// state reported to runtime doctor checks.
type ManagedAuditHealth struct {
	QueueDropped         uint64
	LastDropAt           time.Time
	WriteFailed          bool
	WriteFailCount       uint64
	LastWriteFailAt      time.Time
	LastWriteOKAt        time.Time
	LastWriteError       string
	Unreadable           bool
	UnreadableCount      uint64
	LastUnreadableAt     time.Time
	LastUnreadableError  string
	ReceiptGaps          uint64
	LastReceiptGapAt     time.Time
	LastReceiptGapDevice string
	RetainedEntries      int
	MaxEntries           int
	NextSeq              uint64
}

// ManagedAuditStoreConfig configures a ManagedAuditStore instance.
type ManagedAuditStoreConfig struct {
	// Store is the borrowed daemon live store. Nil creates a memory-only store
	// suitable for unit tests.
	Store storage.Storage
	// MaxEntries bounds retained audit rows (100..100000). Zero selects
	// plugin.DefaultAuditMaxEntries (10000).
	MaxEntries int
	// Now supplies UTC timestamps. Nil defaults to time.Now.
	Now func() time.Time
}

type auditPageRef struct {
	PageID uint64 `json:"page-id"`
	MinSeq uint64 `json:"min-seq"`
	MaxSeq uint64 `json:"max-seq"`
	Count  int    `json:"count"`
}

type auditDeviceCursor struct {
	Epoch       string `json:"epoch"`
	Committed   uint64 `json:"committed"`
	LastGapFrom uint64 `json:"last-gap-from,omitempty"`
	LastGapTo   uint64 `json:"last-gap-to,omitempty"`
}

type auditManifest struct {
	Version       int                          `json:"version"`
	NextSeq       uint64                       `json:"next-seq"`
	NextPageID    uint64                       `json:"next-page-id"`
	MinSeq        uint64                       `json:"min-seq,omitempty"`
	MaxSeq        uint64                       `json:"max-seq,omitempty"`
	MaxEntries    int                          `json:"max-entries"`
	Pages         []auditPageRef               `json:"pages,omitempty"`
	DeviceCursors map[string]auditDeviceCursor `json:"device-cursors,omitempty"`
}

type auditPageData struct {
	Version int               `json:"version"`
	PageID  uint64            `json:"page-id"`
	Entries []FleetAuditEntry `json:"entries"`
}

type auditLoadedPage struct {
	ref     auditPageRef
	entries []FleetAuditEntry
}

type auditGeneration struct {
	id         uint64
	pages      []auditLoadedPage
	totalCount int
	refs       int
	retired    bool
	obsolete   []uint64
}

type auditCtrlKind uint8

const (
	auditCtrlFlush auditCtrlKind = iota + 1
	auditCtrlSetMaxEntries
	auditCtrlIngestReceipts
)

type auditControlReq struct {
	kind       auditCtrlKind
	maxEntries int
	device     string
	batch      fleet.AuditReceiptBatch
	now        time.Time
	reply      chan auditControlReply
}

type auditControlReply struct {
	ack fleet.AuditReceiptAck
	err error
}

// ManagedAuditStore owns the hub's bounded, append-only fleet audit log.
// A single long-lived writer goroutine (started by Start and joined by Stop)
// drains the bounded async queue and synchronous control requests, writes
// immutable pages and atomic manifests to ZeFS, and garbage-collects superseded
// pages once no concurrent reader pins them.
//
// Safe for concurrent use. Callers MUST call Stop after Start before closing
// the borrowed storage.Storage.
type ManagedAuditStore struct {
	store storage.Storage
	now   func() time.Time

	admMu   sync.RWMutex
	closing bool

	queue  chan FleetAuditEntry
	ctrl   chan auditControlReq
	gcWake chan struct{}
	stopCh chan struct{}
	doneCh chan struct{}

	startOnce sync.Once
	stopOnce  sync.Once

	// Writer-owned mutable state (accessed only before Start, or exclusively
	// by the single writer goroutine once Start has been called).
	writerPages      []auditLoadedPage
	writerNextSeq    uint64
	writerNextPageID uint64
	writerMaxEntries int
	writerCursors    map[string]auditDeviceCursor
	writerGenID      uint64

	// Reader/health shared state protected by mu.
	mu                 sync.Mutex
	started            bool
	current            *auditGeneration
	retiredGens        []*auditGeneration
	pendingDeletePages []uint64
	health             ManagedAuditHealth
}

// NewManagedAuditStore builds a ManagedAuditStore with the given configuration.
// Call LoadFromStore before Start to recover persisted pages and device receipt
// cursors from cfg.Store.
func NewManagedAuditStore(cfg ManagedAuditStoreConfig) (*ManagedAuditStore, error) {
	maxEntries := cfg.MaxEntries
	if maxEntries == 0 {
		maxEntries = plugin.DefaultAuditMaxEntries
	}
	if err := plugin.ValidateHubAuditConfig(plugin.HubAuditConfig{MaxEntries: maxEntries}); err != nil {
		return nil, err
	}
	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	initialGen := &auditGeneration{id: 1}
	return &ManagedAuditStore{
		store:            cfg.Store,
		now:              nowFn,
		queue:            make(chan FleetAuditEntry, FleetAuditQueueSize),
		ctrl:             make(chan auditControlReq, 16),
		gcWake:           make(chan struct{}, 1),
		stopCh:           make(chan struct{}),
		doneCh:           make(chan struct{}),
		writerNextSeq:    1,
		writerNextPageID: 1,
		writerMaxEntries: maxEntries,
		writerCursors:    make(map[string]auditDeviceCursor),
		writerGenID:      1,
		current:          initialGen,
		health: ManagedAuditHealth{
			MaxEntries: maxEntries,
			NextSeq:    1,
		},
	}, nil
}

// ValidFleetAuditEventType reports whether eventType is a non-empty, bounded,
// dot-separated kebab-case event type name (at least two segments, each
// starting with a lowercase ASCII letter and containing only lowercase ASCII
// letters, digits, or internal hyphens).
func ValidFleetAuditEventType(eventType string) bool {
	if eventType == "" {
		return false
	}
	if len(eventType) > FleetAuditEventTypeMax {
		return false
	}
	if !strings.Contains(eventType, ".") {
		return false
	}
	rest := eventType
	for {
		seg, after, hasDot := strings.Cut(rest, ".")
		if !validAuditEventSegment(seg) {
			return false
		}
		if hasDot {
			rest = after
			continue
		}
		break
	}
	return true
}

func validAuditEventSegment(seg string) bool {
	if seg == "" {
		return false
	}
	first := seg[0]
	if first < 'a' {
		return false
	}
	if first > 'z' {
		return false
	}
	last := seg[len(seg)-1]
	if last == '-' {
		return false
	}
	prevHyphen := false
	for i := range len(seg) {
		ch := seg[i]
		isLower := ch >= 'a' && ch <= 'z'
		isDigit := ch >= '0' && ch <= '9'
		if ch == '-' {
			if prevHyphen {
				return false
			}
			prevHyphen = true
			continue
		}
		if !isLower {
			if !isDigit {
				return false
			}
		}
		prevHyphen = false
	}
	return true
}

func (s *ManagedAuditStore) currentTime() time.Time {
	return s.now().UTC()
}

// LoadFromStore reads the persisted fleet audit manifest and immutable pages
// from the borrowed ZeFS store. Corrupt manifests or individual corrupt pages
// are counted in ManagedAuditHealth without crashing startup; healthy pages
// remain readable, sequence and page IDs stay strictly above all readable data,
// and unreferenced orphan page keys are garbage-collected.
//
// MUST be called before Start.
func (s *ManagedAuditStore) LoadFromStore() (result error) {
	// refuse-by-default: guards NewManagedAuditStore -- nil storage runs an in-memory audit store for unit tests.
	if s.store == nil {
		return nil
	}
	guard, err := s.store.AcquireLock(FleetAuditLockKey)
	if err != nil {
		return fmt.Errorf("lock fleet audit store: %w", err)
	}
	defer func() {
		result = errors.Join(result, guard.Release())
	}()

	now := s.currentTime()
	existingPageKeys, err := guard.ListKeys(zefs.FleetAuditPagePrefix)
	if err != nil {
		return fmt.Errorf("list fleet audit pages: %w", err)
	}

	rawManifest, err := guard.ReadKey(zefs.KeyFleetAuditManifest.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		s.gcOrphanKeysLocked(guard, existingPageKeys, nil)
		s.publishLoadedState(nil, 1, 1, make(map[string]auditDeviceCursor))
		return nil
	}
	// refuse-by-default: guards ManagedAuditStore.Health -- unreadable manifest is recorded in ManagedAuditHealth.Unreadable and surfaced by doctor while healthy pages remain readable.
	if err != nil {
		s.recordUnreadable(now, fmt.Errorf("read fleet audit manifest: %w", err))
		s.recoverPagesWithoutManifest(guard, existingPageKeys, now)
		return nil
	}

	manifest, err := decodeAuditManifest(rawManifest)
	// refuse-by-default: guards ManagedAuditStore.Health -- corrupt manifest is recorded in ManagedAuditHealth.Unreadable and surfaced by doctor while healthy pages remain readable.
	if err != nil {
		s.recordUnreadable(now, fmt.Errorf("decode fleet audit manifest: %w", err))
		s.recoverPagesWithoutManifest(guard, existingPageKeys, now)
		return nil
	}

	nextSeq := max(manifest.NextSeq, 1)
	nextPageID := max(manifest.NextPageID, 1)
	referenced := make(map[string]struct{}, len(manifest.Pages))
	loaded := make([]auditLoadedPage, 0, len(manifest.Pages))

	for _, ref := range manifest.Pages {
		key := auditPageKey(ref.PageID)
		referenced[key] = struct{}{}
		nextSeq = max(nextSeq, ref.MaxSeq+1)
		nextPageID = max(nextPageID, ref.PageID+1)

		rawPage, readErr := guard.ReadKey(key)
		if readErr != nil {
			s.recordUnreadable(now, fmt.Errorf("read fleet audit page %s: %w", key, readErr))
			continue
		}
		page, decErr := decodeAuditPage(rawPage, ref.PageID)
		if decErr != nil {
			s.recordUnreadable(now, fmt.Errorf("decode fleet audit page %s: %w", key, decErr))
			continue
		}
		if len(page.Entries) == 0 {
			continue
		}
		lastSeq := page.Entries[len(page.Entries)-1].Seq
		nextSeq = max(nextSeq, lastSeq+1)
		loaded = append(loaded, makeLoadedPage(ref.PageID, page.Entries))
	}

	for _, key := range existingPageKeys {
		if id, ok := parseAuditPageID(key); ok {
			nextPageID = max(nextPageID, id+1)
		}
	}
	s.gcOrphanKeysLocked(guard, existingPageKeys, referenced)

	cursors := make(map[string]auditDeviceCursor, len(manifest.DeviceCursors))
	for dev, cur := range manifest.DeviceCursors {
		if fleet.ValidDeviceKeyName(dev) {
			cursors[dev] = cur
		}
	}

	s.publishLoadedState(loaded, nextSeq, nextPageID, cursors)
	return nil
}

func (s *ManagedAuditStore) recoverPagesWithoutManifest(guard storage.WriteGuard, keys []string, now time.Time) {
	var loaded []auditLoadedPage
	var nextSeq uint64 = 1
	var nextPageID uint64 = 1

	for _, key := range keys {
		id, ok := parseAuditPageID(key)
		if !ok {
			continue
		}
		nextPageID = max(nextPageID, id+1)

		rawPage, err := guard.ReadKey(key)
		if err != nil {
			s.recordUnreadable(now, fmt.Errorf("read fleet audit page %s: %w", key, err))
			continue
		}
		page, err := decodeAuditPage(rawPage, id)
		if err != nil {
			s.recordUnreadable(now, fmt.Errorf("decode fleet audit page %s: %w", key, err))
			continue
		}
		if len(page.Entries) == 0 {
			continue
		}
		loaded = append(loaded, makeLoadedPage(id, page.Entries))
		lastSeq := page.Entries[len(page.Entries)-1].Seq
		nextSeq = max(nextSeq, lastSeq+1)
	}

	slices.SortFunc(loaded, func(a, b auditLoadedPage) int {
		if c := cmp.Compare(a.ref.MinSeq, b.ref.MinSeq); c != 0 {
			return c
		}
		return cmp.Compare(a.ref.PageID, b.ref.PageID)
	})

	// Drop overlapping/duplicate sequences if an older un-GC'd page survived
	// beside its compacted replacement before manifest corruption.
	var deduped []auditLoadedPage
	var highestSeq uint64
	for _, p := range loaded {
		var kept []FleetAuditEntry
		for i := range p.entries {
			if p.entries[i].Seq > highestSeq {
				kept = append(kept, p.entries[i])
				highestSeq = p.entries[i].Seq
			}
		}
		if len(kept) > 0 {
			deduped = append(deduped, makeLoadedPage(p.ref.PageID, kept))
		}
	}

	s.publishLoadedState(deduped, nextSeq, nextPageID, make(map[string]auditDeviceCursor))
}

func (s *ManagedAuditStore) gcOrphanKeysLocked(guard storage.WriteGuard, existing []string, referenced map[string]struct{}) {
	for _, key := range existing {
		if referenced != nil {
			if _, ok := referenced[key]; ok {
				continue
			}
		}
		if err := guard.RemoveKey(key); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				managedLogger().Warn("failed to remove orphaned fleet audit page", "code", CodeDoctorManagedAuditWriteFailed, "key", key, "error", err)
			}
		}
	}
}

func (s *ManagedAuditStore) publishLoadedState(pages []auditLoadedPage, nextSeq, nextPageID uint64, cursors map[string]auditDeviceCursor) {
	s.writerPages = pages
	s.writerNextSeq = max(nextSeq, 1)
	s.writerNextPageID = max(nextPageID, 1)
	s.writerCursors = cursors
	s.writerGenID++

	total := countLoadedEntries(pages)
	gen := &auditGeneration{
		id:         s.writerGenID,
		pages:      slices.Clone(pages),
		totalCount: total,
	}

	s.mu.Lock()
	s.current = gen
	s.health.RetainedEntries = total
	s.health.MaxEntries = s.writerMaxEntries
	s.health.NextSeq = s.writerNextSeq
	s.mu.Unlock()
}

// Start launches the single long-lived hub audit writer goroutine. Idempotent.
// Callers MUST call Stop before closing the borrowed storage.Storage.
func (s *ManagedAuditStore) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.startOnce.Do(func() {
		s.admMu.RLock()
		closing := s.closing
		s.admMu.RUnlock()
		if closing {
			return
		}
		s.mu.Lock()
		s.started = true
		s.mu.Unlock()
		go s.runWriter(ctx)
	})
}

// Stop closes admission of new async entries and synchronous control requests,
// drains every already-queued event and control request, publishes the final
// batch to ZeFS, and joins the writer goroutine. Idempotent.
func (s *ManagedAuditStore) Stop() {
	s.stopOnce.Do(func() {
		s.admMu.Lock()
		s.closing = true
		close(s.queue)
		s.admMu.Unlock()

		s.mu.Lock()
		started := s.started
		s.mu.Unlock()
		if !started {
			return
		}
		close(s.stopCh)
		<-s.doneCh
	})
}

// Append enqueues one audit entry for asynchronous publication by the single
// hub writer. Strictly non-blocking: returns false and increments QueueDropped
// if the bounded queue is full, the store is closing/stopped, or entry.Type
// fails ValidFleetAuditEventType.
//
// Safe for concurrent use.
func (s *ManagedAuditStore) Append(entry FleetAuditEntry) bool {
	if !ValidFleetAuditEventType(entry.Type) {
		managedLogger().Warn("refused fleet audit entry with invalid event type", "code", CodeDoctorManagedAuditDropped, "type", entry.Type)
		return false
	}
	sanitized := sanitizeAuditEntry(entry, s.currentTime())

	s.admMu.RLock()
	if s.closing {
		s.admMu.RUnlock()
		s.recordQueueDrop()
		return false
	}
	select {
	case s.queue <- sanitized:
		s.admMu.RUnlock()
		return true
	default:
		s.admMu.RUnlock()
		s.recordQueueDrop()
		return false
	}
}

// Flush blocks until the single writer goroutine has drained and attempted to
// publish all async entries queued prior to this call.
// Safe for concurrent use.
func (s *ManagedAuditStore) Flush(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := s.sendControl(ctx, auditControlReq{kind: auditCtrlFlush})
	return err
}

// SetMaxEntries updates the retention ceiling (100..100000) and prunes oldest
// retained entries on publication.
// Safe for concurrent use.
func (s *ManagedAuditStore) SetMaxEntries(maxEntries int) error {
	if err := plugin.ValidateHubAuditConfig(plugin.HubAuditConfig{MaxEntries: maxEntries}); err != nil {
		return err
	}
	s.mu.Lock()
	started := s.started
	if !started {
		s.writerMaxEntries = maxEntries
		s.health.MaxEntries = maxEntries
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	_, err := s.sendControl(context.Background(), auditControlReq{
		kind:       auditCtrlSetMaxEntries,
		maxEntries: maxEntries,
	})
	return err
}

// IngestReceipts validates and durably commits a bounded batch of device-side
// offline mode-change audit receipts through the single hub writer goroutine.
// It binds the hub actor to "device:<device>" (storing peer-reported actor and
// surface only in metadata), deduplicates sequences already committed for the
// epoch even after retention aging, records a fleet.receipt.gap entry once per
// new overflow gap, appends only the contiguous prefix, and commits entries and
// cursor in one atomic manifest write before acknowledging.
//
// Safe for concurrent use.
func (s *ManagedAuditStore) IngestReceipts(device string, batch fleet.AuditReceiptBatch, now time.Time) (fleet.AuditReceiptAck, error) {
	if !fleet.ValidDeviceKeyName(device) {
		return fleet.AuditReceiptAck{}, fmt.Errorf("invalid managed device name %q", device)
	}
	if err := batch.Validate(); err != nil {
		return fleet.AuditReceiptAck{}, err
	}
	if now.IsZero() {
		now = s.currentTime()
	} else {
		now = now.UTC()
	}
	return s.sendControl(context.Background(), auditControlReq{
		kind:   auditCtrlIngestReceipts,
		device: device,
		batch:  cloneReceiptBatch(batch),
		now:    now,
	})
}

func (s *ManagedAuditStore) sendControl(ctx context.Context, req auditControlReq) (fleet.AuditReceiptAck, error) {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return fleet.AuditReceiptAck{}, ErrAuditStoreNotStarted
	}

	req.reply = make(chan auditControlReply, 1)

	s.admMu.RLock()
	if s.closing {
		s.admMu.RUnlock()
		return fleet.AuditReceiptAck{}, ErrAuditStoreStopped
	}
	select {
	case s.ctrl <- req:
		s.admMu.RUnlock()
	case <-s.doneCh:
		s.admMu.RUnlock()
		return fleet.AuditReceiptAck{}, ErrAuditStoreStopped
	case <-ctx.Done():
		s.admMu.RUnlock()
		return fleet.AuditReceiptAck{}, ctx.Err()
	}

	select {
	case rep := <-req.reply:
		return rep.ack, rep.err
	case <-s.doneCh:
		select {
		case rep := <-req.reply:
			return rep.ack, rep.err
		default:
			return fleet.AuditReceiptAck{}, ErrAuditStoreStopped
		}
	case <-ctx.Done():
		return fleet.AuditReceiptAck{}, ctx.Err()
	}
}

// Query scans one pinned published generation newest-first (descending Seq),
// enforcing per-device authorization via a per-query cache before applying
// caller selectors (Device, Type, inclusive Since/Until, Limit).
//
// Safe for concurrent use.
func (s *ManagedAuditStore) Query(scope FleetDeviceScope, q FleetAuditQuery) []FleetAuditEntry {
	gen := s.pinGeneration()
	defer s.unpinGeneration(gen)

	if gen == nil {
		return nil
	}
	if gen.totalCount == 0 {
		return nil
	}
	if scope.mode == fleetScopeDenyAll {
		return nil
	}

	deviceAllowed := make(map[string]bool)
	var out []FleetAuditEntry

	for _, page := range slices.Backward(gen.pages) {
		entries := page.entries
		for ei := len(entries) - 1; ei >= 0; ei-- {
			entry := &entries[ei]
			if !auditEntryAuthorized(scope, entry.Device, deviceAllowed) {
				continue
			}
			if !auditEntryMatchesQuery(entry, q) {
				continue
			}
			out = append(out, entry.Clone())
			if q.Limit > 0 {
				if len(out) >= q.Limit {
					return out
				}
			}
		}
	}
	return out
}

func auditEntryAuthorized(scope FleetDeviceScope, device string, cache map[string]bool) bool {
	if device == "" {
		return scope.mode == fleetScopeAllowAll
	}
	allowed, cached := cache[device]
	if !cached {
		allowed = scope.AllowsDevice(device)
		cache[device] = allowed
	}
	return allowed
}

func auditEntryMatchesQuery(entry *FleetAuditEntry, q FleetAuditQuery) bool {
	if q.Device != "" {
		if entry.Device != q.Device {
			return false
		}
	}
	if q.Type != "" {
		if entry.Type != q.Type {
			return false
		}
	}
	if !q.Since.IsZero() {
		if entry.EventTime.Before(q.Since) {
			return false
		}
	}
	if !q.Until.IsZero() {
		if entry.EventTime.After(q.Until) {
			return false
		}
	}
	return true
}

// Health returns a value-type snapshot of audit store durability, queue drops,
// unreadable history, and device receipt gap counters.
// Safe for concurrent use.
func (s *ManagedAuditStore) Health() ManagedAuditHealth {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.health
}

func (s *ManagedAuditStore) pinGeneration() *auditGeneration {
	s.mu.Lock()
	gen := s.current
	if gen != nil {
		gen.refs++
	}
	s.mu.Unlock()
	return gen
}

func (s *ManagedAuditStore) unpinGeneration(gen *auditGeneration) {
	if gen == nil {
		return
	}
	shouldWakeGC := false
	s.mu.Lock()
	gen.refs--
	if gen.retired {
		if gen.refs == 0 {
			s.pendingDeletePages = append(s.pendingDeletePages, gen.obsolete...)
			gen.obsolete = nil
			for i, r := range s.retiredGens {
				if r == gen {
					s.retiredGens = slices.Delete(s.retiredGens, i, i+1)
					break
				}
			}
			shouldWakeGC = len(s.pendingDeletePages) > 0
		}
	}
	s.mu.Unlock()

	if shouldWakeGC {
		select {
		case s.gcWake <- struct{}{}:
		default:
		}
	}
}

func (s *ManagedAuditStore) recordQueueDrop() {
	now := s.currentTime()
	s.mu.Lock()
	s.health.QueueDropped++
	s.health.LastDropAt = now
	s.mu.Unlock()
}

func (s *ManagedAuditStore) recordUnreadable(now time.Time, err error) {
	s.mu.Lock()
	s.health.Unreadable = true
	s.health.UnreadableCount++
	s.health.LastUnreadableAt = now
	s.health.LastUnreadableError = err.Error()
	s.mu.Unlock()
	managedLogger().Error("unreadable fleet audit history", "code", CodeDoctorManagedAuditCorrupt, "error", err)
}

func (s *ManagedAuditStore) recordWriteFailure(now time.Time, err error) {
	s.mu.Lock()
	s.health.WriteFailed = true
	s.health.WriteFailCount++
	s.health.LastWriteFailAt = now
	s.health.LastWriteError = err.Error()
	s.mu.Unlock()
	managedLogger().Error("fleet audit store write failed", "code", CodeDoctorManagedAuditWriteFailed, "error", err)
}

func (s *ManagedAuditStore) runWriter(ctx context.Context) {
	defer close(s.doneCh)

	for {
		select {
		case entry, ok := <-s.queue:
			if !ok {
				s.drainAndFinalize()
				return
			}
			batch := s.collectAsyncBatch(entry)
			s.publishAsyncBatch(batch)
		case req := <-s.ctrl:
			s.handleControl(req)
		case <-s.gcWake:
			s.runPendingPageGC()
		case <-s.stopCh:
			s.drainAndFinalize()
			return
		case <-ctx.Done():
			s.drainAndFinalize()
			return
		}
	}
}

func (s *ManagedAuditStore) collectAsyncBatch(first FleetAuditEntry) []FleetAuditEntry {
	batch := make([]FleetAuditEntry, 0, min(fleetAuditDrainBatchMax, len(s.queue)+1))
	batch = append(batch, first)
	for len(batch) < fleetAuditDrainBatchMax {
		select {
		case next, ok := <-s.queue:
			if !ok {
				return batch
			}
			batch = append(batch, next)
		default:
			return batch
		}
	}
	return batch
}

func (s *ManagedAuditStore) drainQueuedAsync() {
	for {
		var batch []FleetAuditEntry
		for len(batch) < fleetAuditDrainBatchMax {
			select {
			case next, ok := <-s.queue:
				if !ok {
					if len(batch) > 0 {
						s.publishAsyncBatch(batch)
					}
					return
				}
				batch = append(batch, next)
			default:
				if len(batch) > 0 {
					s.publishAsyncBatch(batch)
				}
				return
			}
		}
		s.publishAsyncBatch(batch)
	}
}

func (s *ManagedAuditStore) drainAndFinalize() {
	s.drainQueuedAsync()
	for {
		select {
		case req := <-s.ctrl:
			s.handleControl(req)
		default:
			s.drainQueuedAsync()
			s.runPendingPageGC()
			return
		}
	}
}

func (s *ManagedAuditStore) handleControl(req auditControlReq) {
	var rep auditControlReply
	switch req.kind {
	case auditCtrlFlush:
		s.drainQueuedAsync()
		s.runPendingPageGC()
	case auditCtrlSetMaxEntries:
		s.drainQueuedAsync()
		rep.err = s.applyMaxEntries(req.maxEntries)
	case auditCtrlIngestReceipts:
		rep.ack, rep.err = s.applyIngestReceipts(req.device, req.batch, req.now)
	default:
		panic("BUG: server: unknown auditCtrlKind")
	}
	req.reply <- rep
}

func (s *ManagedAuditStore) publishAsyncBatch(batch []FleetAuditEntry) {
	if len(batch) == 0 {
		return
	}
	now := s.currentTime()
	if err := s.commitMutation(batch, s.writerCursors, s.writerMaxEntries, now); err != nil {
		s.recordWriteFailure(now, err)
	}
}

func (s *ManagedAuditStore) applyMaxEntries(maxEntries int) error {
	now := s.currentTime()
	if maxEntries == s.writerMaxEntries {
		if countLoadedEntries(s.writerPages) <= maxEntries {
			return nil
		}
	}
	err := s.commitMutation(nil, s.writerCursors, maxEntries, now)
	if err != nil {
		s.recordWriteFailure(now, err)
		return err
	}
	return nil
}

func (s *ManagedAuditStore) applyIngestReceipts(device string, batch fleet.AuditReceiptBatch, now time.Time) (fleet.AuditReceiptAck, error) {
	prevCursor := s.writerCursors[device]
	cursor := prevCursor
	if cursor.Epoch != batch.Epoch {
		cursor = auditDeviceCursor{Epoch: batch.Epoch}
	}

	var newEntries []FleetAuditEntry
	gapRecorded := false

	if batch.Lost > 0 {
		if batch.GapFrom > cursor.Committed+1 {
			return fleet.AuditReceiptAck{}, fmt.Errorf("non-contiguous audit receipt gap: expected sequence %d, got gap starting at %d", cursor.Committed+1, batch.GapFrom)
		}
		if batch.GapTo > cursor.Committed {
			if batch.GapTo > cursor.LastGapTo {
				gapMeta := map[string]string{
					"epoch":    batch.Epoch,
					"gap-from": formatAuditUint(batch.GapFrom),
					"gap-to":   formatAuditUint(batch.GapTo),
					"lost":     formatAuditUint(batch.Lost),
				}
				newEntries = append(newEntries, sanitizeAuditEntry(FleetAuditEntry{
					EventTime:   now,
					ReceiptTime: now,
					Type:        FleetAuditReceiptGap,
					Device:      device,
					Actor:       fleetAuditDevicePrefix + device,
					Surface:     fleetAuditDefaultSurface,
					Outcome:     fleetAuditOutcomeError,
					Summary:     "device receipt outbox overflow",
					Metadata:    gapMeta,
				}, now))
				gapRecorded = true
				cursor.LastGapFrom = batch.GapFrom
				cursor.LastGapTo = batch.GapTo
			}
			cursor.Committed = batch.GapTo
		}
	}

	deduped := 0
	accepted := 0
	for i := range batch.Entries {
		e := batch.Entries[i]
		if e.Seq <= cursor.Committed {
			deduped++
			continue
		}
		if e.Seq == cursor.Committed+1 {
			meta := map[string]string{
				"epoch":          batch.Epoch,
				"device-seq":     formatAuditUint(e.Seq),
				"device-actor":   e.Actor,
				"device-surface": e.Surface,
			}
			newEntries = append(newEntries, sanitizeAuditEntry(FleetAuditEntry{
				EventTime:   e.EventAt.UTC(),
				ReceiptTime: now,
				Type:        e.Action,
				Device:      device,
				Actor:       fleetAuditDevicePrefix + device,
				Surface:     fleetAuditSurfaceDevice,
				Outcome:     e.Outcome,
				Metadata:    meta,
			}, now))
			cursor.Committed = e.Seq
			accepted++
			continue
		}
		if accepted == 0 {
			if deduped == 0 {
				if !gapRecorded {
					return fleet.AuditReceiptAck{}, fmt.Errorf("non-contiguous audit receipt sequence: expected %d, got %d", cursor.Committed+1, e.Seq)
				}
			}
		}
		break
	}

	if len(newEntries) == 0 {
		if cursor == prevCursor {
			return fleet.AuditReceiptAck{Epoch: batch.Epoch, Committed: cursor.Committed}, nil
		}
	}

	nextCursors := maps.Clone(s.writerCursors)
	if nextCursors == nil {
		nextCursors = make(map[string]auditDeviceCursor)
	}
	nextCursors[device] = cursor

	if err := s.commitMutation(newEntries, nextCursors, s.writerMaxEntries, now); err != nil {
		s.recordWriteFailure(now, err)
		return fleet.AuditReceiptAck{}, err
	}

	if gapRecorded {
		s.mu.Lock()
		s.health.ReceiptGaps++
		s.health.LastReceiptGapAt = now
		s.health.LastReceiptGapDevice = device
		s.mu.Unlock()
	}

	return fleet.AuditReceiptAck{Epoch: batch.Epoch, Committed: cursor.Committed}, nil
}

// commitMutation assigns monotonic sequence numbers to added entries, builds
// new immutable pages via copy-on-write tail compaction, prunes to maxEntries,
// persists pages and atomic manifest under FleetAuditLockKey, and publishes the
// new generation only after durable success.
func (s *ManagedAuditStore) commitMutation(added []FleetAuditEntry, nextCursors map[string]auditDeviceCursor, maxEntries int, now time.Time) error {
	nextSeq := s.writerNextSeq
	sequenced := make([]FleetAuditEntry, len(added))
	for i := range added {
		e := added[i].Clone()
		e.Seq = nextSeq
		nextSeq++
		sequenced[i] = e
	}

	nextPageID := s.writerNextPageID
	nextPages, newPages, obsoleteIDs := buildNextAuditPages(s.writerPages, sequenced, maxEntries, &nextPageID)

	if s.store != nil {
		if err := s.persistAuditGeneration(nextPages, newPages, nextSeq, nextPageID, maxEntries, nextCursors); err != nil {
			// Advance writerNextPageID past any page ID touched by a partial
			// write so a future retry never overwrites a previously written key.
			s.writerNextPageID = nextPageID
			return err
		}
	}

	s.writerPages = nextPages
	s.writerNextSeq = nextSeq
	s.writerNextPageID = nextPageID
	s.writerMaxEntries = maxEntries
	s.writerCursors = nextCursors
	s.writerGenID++

	total := countLoadedEntries(nextPages)
	newGen := &auditGeneration{
		id:         s.writerGenID,
		pages:      slices.Clone(nextPages),
		totalCount: total,
	}

	s.mu.Lock()
	oldGen := s.current
	s.current = newGen
	if oldGen != nil {
		if oldGen.refs > 0 {
			oldGen.retired = true
			oldGen.obsolete = append(oldGen.obsolete, obsoleteIDs...)
			s.retiredGens = append(s.retiredGens, oldGen)
		} else {
			s.pendingDeletePages = append(s.pendingDeletePages, obsoleteIDs...)
		}
	} else {
		s.pendingDeletePages = append(s.pendingDeletePages, obsoleteIDs...)
	}
	s.health.WriteFailed = false
	s.health.LastWriteError = ""
	s.health.LastWriteOKAt = now
	s.health.RetainedEntries = total
	s.health.MaxEntries = maxEntries
	s.health.NextSeq = nextSeq
	s.mu.Unlock()

	s.runPendingPageGC()
	return nil
}

func (s *ManagedAuditStore) persistAuditGeneration(
	allPages, newPages []auditLoadedPage,
	nextSeq, nextPageID uint64,
	maxEntries int,
	cursors map[string]auditDeviceCursor,
) error {
	encodedPages := make([][]byte, len(newPages))
	for i := range newPages {
		payload := auditPageData{
			Version: fleetAuditManifestVersion,
			PageID:  newPages[i].ref.PageID,
			Entries: newPages[i].entries,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal fleet audit page %d: %w", newPages[i].ref.PageID, err)
		}
		if len(raw) > zefs.FleetAuditPageValueMax {
			return fmt.Errorf("fleet audit page %d exceeds %d bytes (%d)", newPages[i].ref.PageID, zefs.FleetAuditPageValueMax, len(raw))
		}
		encodedPages[i] = raw
	}

	manifest := auditManifest{
		Version:       fleetAuditManifestVersion,
		NextSeq:       nextSeq,
		NextPageID:    nextPageID,
		MaxEntries:    maxEntries,
		Pages:         make([]auditPageRef, len(allPages)),
		DeviceCursors: cursors,
	}
	for i := range allPages {
		manifest.Pages[i] = allPages[i].ref
	}
	if len(allPages) > 0 {
		manifest.MinSeq = allPages[0].ref.MinSeq
		manifest.MaxSeq = allPages[len(allPages)-1].ref.MaxSeq
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal fleet audit manifest: %w", err)
	}
	if len(manifestBytes) > zefs.FleetAuditManifestValueMax {
		return fmt.Errorf("fleet audit manifest exceeds %d bytes (%d)", zefs.FleetAuditManifestValueMax, len(manifestBytes))
	}

	guard, err := s.store.AcquireLock(FleetAuditLockKey)
	if err != nil {
		return fmt.Errorf("lock fleet audit store: %w", err)
	}

	var writtenKeys []string
	var writeErr error
	for i := range newPages {
		key := auditPageKey(newPages[i].ref.PageID)
		if err := guard.WriteKey(key, encodedPages[i]); err != nil {
			writeErr = fmt.Errorf("write fleet audit page %s: %w", key, err)
			break
		}
		writtenKeys = append(writtenKeys, key)
	}
	if writeErr == nil {
		if err := guard.WriteKey(zefs.KeyFleetAuditManifest.Pattern, manifestBytes); err != nil {
			writeErr = fmt.Errorf("write fleet audit manifest: %w", err)
		}
	}
	if writeErr != nil {
		for _, key := range writtenKeys {
			if rmErr := guard.RemoveKey(key); rmErr != nil {
				if !errors.Is(rmErr, fs.ErrNotExist) {
					managedLogger().Warn("failed to clean up uncommitted fleet audit page", "code", CodeDoctorManagedAuditWriteFailed, "key", key, "error", rmErr)
				}
			}
		}
	}
	return errors.Join(writeErr, guard.Release())
}

func (s *ManagedAuditStore) runPendingPageGC() {
	if s.store == nil {
		s.mu.Lock()
		s.pendingDeletePages = nil
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	if len(s.pendingDeletePages) == 0 {
		s.mu.Unlock()
		return
	}
	toDelete := s.pendingDeletePages
	s.pendingDeletePages = nil
	s.mu.Unlock()

	guard, err := s.store.AcquireLock(FleetAuditLockKey)
	if err != nil {
		s.mu.Lock()
		s.pendingDeletePages = append(toDelete, s.pendingDeletePages...)
		s.mu.Unlock()
		return
	}
	var remaining []uint64
	for _, id := range toDelete {
		key := auditPageKey(id)
		if rmErr := guard.RemoveKey(key); rmErr != nil {
			if !errors.Is(rmErr, fs.ErrNotExist) {
				remaining = append(remaining, id)
				managedLogger().Warn("failed to garbage-collect obsolete fleet audit page", "code", CodeDoctorManagedAuditWriteFailed, "key", key, "error", rmErr)
			}
		}
	}
	if relErr := guard.Release(); relErr != nil {
		managedLogger().Warn("failed to release fleet audit GC lock", "code", CodeDoctorManagedAuditWriteFailed, "error", relErr)
	}
	if len(remaining) > 0 {
		s.mu.Lock()
		s.pendingDeletePages = append(remaining, s.pendingDeletePages...)
		s.mu.Unlock()
	}
}

// buildNextAuditPages produces the next slice of immutable loaded pages from
// prev and added entries, compacting a non-full tail page onto a brand-new page
// ID and trimming oldest entries past maxEntries. Existing page IDs are never
// overwritten in place.
func buildNextAuditPages(
	prev []auditLoadedPage,
	added []FleetAuditEntry,
	maxEntries int,
	nextPageID *uint64,
) (working, newPages []auditLoadedPage, obsoleteIDs []uint64) {
	working = make([]auditLoadedPage, len(prev))
	copy(working, prev)
	isNew := make(map[uint64]bool)

	if len(added) > 0 {
		pending := added
		if len(working) > 0 {
			tail := working[len(working)-1]
			if len(tail.entries) < FleetAuditPageMaxEntries {
				working = working[:len(working)-1]
				obsoleteIDs = append(obsoleteIDs, tail.ref.PageID)
				combined := make([]FleetAuditEntry, 0, len(tail.entries)+len(added))
				combined = append(combined, tail.entries...)
				combined = append(combined, added...)
				pending = combined
			}
		}
		for len(pending) > 0 {
			chunkLen := min(len(pending), FleetAuditPageMaxEntries)
			chunk := slices.Clone(pending[:chunkLen])
			pending = pending[chunkLen:]
			id := *nextPageID
			*nextPageID++
			page := makeLoadedPage(id, chunk)
			working = append(working, page)
			newPages = append(newPages, page)
			isNew[id] = true
		}
	}

	total := countLoadedEntries(working)
	if total <= maxEntries {
		return working, newPages, obsoleteIDs
	}

	excess := total - maxEntries
	for len(working) > 0 {
		if len(working[0].entries) > excess {
			break
		}
		dropped := working[0]
		working = working[1:]
		excess -= len(dropped.entries)
		if isNew[dropped.ref.PageID] {
			delete(isNew, dropped.ref.PageID)
			newPages = removeLoadedPageByID(newPages, dropped.ref.PageID)
		} else {
			obsoleteIDs = append(obsoleteIDs, dropped.ref.PageID)
		}
	}

	if excess > 0 {
		if len(working) > 0 {
			head := working[0]
			trimmed := slices.Clone(head.entries[excess:])
			if isNew[head.ref.PageID] {
				updated := makeLoadedPage(head.ref.PageID, trimmed)
				working[0] = updated
				replaceLoadedPageByID(newPages, updated)
			} else {
				obsoleteIDs = append(obsoleteIDs, head.ref.PageID)
				id := *nextPageID
				*nextPageID++
				updated := makeLoadedPage(id, trimmed)
				working[0] = updated
				newPages = append(newPages, updated)
				isNew[id] = true
			}
		}
	}

	if len(working) >= 2 {
		if len(working[0].entries)+len(working[1].entries) <= FleetAuditPageMaxEntries {
			head := working[0]
			second := working[1]
			combined := make([]FleetAuditEntry, 0, len(head.entries)+len(second.entries))
			combined = append(combined, head.entries...)
			combined = append(combined, second.entries...)
			for _, p := range []auditLoadedPage{head, second} {
				if isNew[p.ref.PageID] {
					delete(isNew, p.ref.PageID)
					newPages = removeLoadedPageByID(newPages, p.ref.PageID)
				} else {
					obsoleteIDs = append(obsoleteIDs, p.ref.PageID)
				}
			}
			id := *nextPageID
			*nextPageID++
			merged := makeLoadedPage(id, combined)
			working = append([]auditLoadedPage{merged}, working[2:]...)
			newPages = append(newPages, merged)
		}
	}

	return working, newPages, obsoleteIDs
}

func removeLoadedPageByID(pages []auditLoadedPage, id uint64) []auditLoadedPage {
	for i := range pages {
		if pages[i].ref.PageID == id {
			return slices.Delete(pages, i, i+1)
		}
	}
	return pages
}

func replaceLoadedPageByID(pages []auditLoadedPage, updated auditLoadedPage) {
	for i := range pages {
		if pages[i].ref.PageID == updated.ref.PageID {
			pages[i] = updated
			return
		}
	}
}

func makeLoadedPage(pageID uint64, entries []FleetAuditEntry) auditLoadedPage {
	ref := auditPageRef{
		PageID: pageID,
		Count:  len(entries),
	}
	if len(entries) > 0 {
		ref.MinSeq = entries[0].Seq
		ref.MaxSeq = entries[len(entries)-1].Seq
	}
	return auditLoadedPage{
		ref:     ref,
		entries: entries,
	}
}

func countLoadedEntries(pages []auditLoadedPage) int {
	total := 0
	for i := range pages {
		total += len(pages[i].entries)
	}
	return total
}

func formatAuditUint(v uint64) string {
	var tb textbuf.Buffer
	tb.Uint(v)
	return tb.String()
}

func auditPageKey(pageID uint64) string {
	const hexDigits = "0123456789abcdef"
	var hexBuf [16]byte
	v := pageID
	for i := 15; i >= 0; i-- {
		hexBuf[i] = hexDigits[v&0xf]
		v >>= 4
	}
	return zefs.KeyFleetAuditPage.Key(string(hexBuf[:]))
}

func parseAuditPageID(key string) (uint64, bool) {
	if !strings.HasPrefix(key, zefs.FleetAuditPagePrefix) {
		return 0, false
	}
	raw := strings.TrimPrefix(key, zefs.FleetAuditPagePrefix)
	if raw == "" {
		return 0, false
	}
	if strings.Contains(raw, "/") {
		return 0, false
	}
	id, err := strconv.ParseUint(raw, 16, 64)
	if err != nil {
		return 0, false
	}
	if id == 0 {
		return 0, false
	}
	return id, true
}

func decodeAuditManifest(raw []byte) (auditManifest, error) {
	var manifest auditManifest
	if len(raw) > zefs.FleetAuditManifestValueMax {
		return manifest, fmt.Errorf("manifest exceeds %d bytes (%d)", zefs.FleetAuditManifestValueMax, len(raw))
	}
	if !utf8.Valid(raw) {
		return manifest, errors.New("manifest contains invalid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return manifest, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return manifest, errors.New("unexpected trailing bytes after manifest")
	}
	if manifest.Version != fleetAuditManifestVersion {
		return manifest, fmt.Errorf("unsupported manifest version %d", manifest.Version)
	}
	if manifest.NextSeq == 0 {
		return manifest, errors.New("manifest next-seq must be positive")
	}
	if manifest.NextPageID == 0 {
		return manifest, errors.New("manifest next-page-id must be positive")
	}
	for i := range manifest.Pages {
		ref := manifest.Pages[i]
		if ref.PageID == 0 {
			return manifest, fmt.Errorf("manifest page %d has zero page-id", i)
		}
		if ref.Count <= 0 {
			return manifest, fmt.Errorf("manifest page %d has non-positive count %d", i, ref.Count)
		}
		if ref.Count > FleetAuditPageMaxEntries {
			return manifest, fmt.Errorf("manifest page %d count %d exceeds %d", i, ref.Count, FleetAuditPageMaxEntries)
		}
	}
	return manifest, nil
}

func decodeAuditPage(raw []byte, expectedPageID uint64) (auditPageData, error) {
	var page auditPageData
	if len(raw) > zefs.FleetAuditPageValueMax {
		return page, fmt.Errorf("page exceeds %d bytes (%d)", zefs.FleetAuditPageValueMax, len(raw))
	}
	if !utf8.Valid(raw) {
		return page, errors.New("page contains invalid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&page); err != nil {
		return page, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return page, errors.New("unexpected trailing bytes after page")
	}
	if page.Version != fleetAuditManifestVersion {
		return page, fmt.Errorf("unsupported page version %d", page.Version)
	}
	if page.PageID != expectedPageID {
		return page, fmt.Errorf("page id %d does not match key page id %d", page.PageID, expectedPageID)
	}
	if len(page.Entries) > FleetAuditPageMaxEntries {
		return page, fmt.Errorf("page entry count %d exceeds %d", len(page.Entries), FleetAuditPageMaxEntries)
	}
	var prevSeq uint64
	for i := range page.Entries {
		entry := &page.Entries[i]
		if entry.Seq == 0 {
			return page, fmt.Errorf("entry %d has zero sequence", i)
		}
		if entry.Seq <= prevSeq {
			return page, fmt.Errorf("entry %d sequence %d is not strictly greater than %d", i, entry.Seq, prevSeq)
		}
		prevSeq = entry.Seq
		entry.EventTime = entry.EventTime.UTC()
		entry.ReceiptTime = entry.ReceiptTime.UTC()
	}
	return page, nil
}

func sanitizeAuditEntry(entry FleetAuditEntry, now time.Time) FleetAuditEntry {
	if entry.EventTime.IsZero() {
		entry.EventTime = now
	} else {
		entry.EventTime = entry.EventTime.UTC()
	}
	if entry.ReceiptTime.IsZero() {
		entry.ReceiptTime = now
	} else {
		entry.ReceiptTime = entry.ReceiptTime.UTC()
	}
	entry.Device = sanitizeAuditText(entry.Device, FleetAuditFieldMax, false)
	entry.Actor = sanitizeAuditText(entry.Actor, FleetAuditFieldMax, false)
	if entry.Actor == "" {
		entry.Actor = fleetAuditDefaultActor
	}
	entry.Surface = sanitizeAuditText(entry.Surface, FleetAuditFieldMax, false)
	if entry.Surface == "" {
		entry.Surface = fleetAuditDefaultSurface
	}
	entry.Outcome = sanitizeAuditText(entry.Outcome, FleetAuditFieldMax, false)
	if entry.Outcome == "" {
		entry.Outcome = fleetAuditDefaultOutcome
	}
	entry.Version = sanitizeAuditText(entry.Version, FleetAuditFieldMax, false)
	entry.PreviousVersion = sanitizeAuditText(entry.PreviousVersion, FleetAuditFieldMax, false)
	entry.DiagnosticCode = sanitizeAuditText(entry.DiagnosticCode, FleetAuditFieldMax, false)
	entry.Summary = sanitizeAuditText(entry.Summary, FleetAuditSummaryMax, true)
	entry.Metadata = sanitizeAuditMetadata(entry.Metadata)
	return entry
}

func sanitizeAuditMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	keys := slices.Sorted(maps.Keys(in))
	out := make(map[string]string, min(len(keys), FleetAuditMetadataMaxEntries))
	for _, k := range keys {
		if len(out) >= FleetAuditMetadataMaxEntries {
			break
		}
		cleanKey := sanitizeAuditText(k, FleetAuditMetadataKeyMax, false)
		if cleanKey == "" {
			continue
		}
		cleanVal := sanitizeAuditText(in[k], FleetAuditMetadataValueMax, false)
		out[cleanKey] = cleanVal
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sanitizeAuditText(raw string, maxBytes int, appendEllipsis bool) string {
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(min(len(raw), maxBytes))
	for _, r := range raw {
		if r == utf8.RuneError {
			continue
		}
		if !unicode.IsPrint(r) {
			switch r {
			case '\n', '\r', '\t':
				b.WriteByte(' ')
			}
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.TrimSpace(b.String())
	if len(cleaned) <= maxBytes {
		return cleaned
	}
	if appendEllipsis {
		if maxBytes > len(fleetAuditTruncatedSuffix) {
			limit := maxBytes - len(fleetAuditTruncatedSuffix)
			for limit > 0 {
				if utf8.RuneStart(cleaned[limit]) {
					break
				}
				limit--
			}
			return cleaned[:limit] + fleetAuditTruncatedSuffix
		}
	}
	limit := maxBytes
	for limit > 0 {
		if utf8.RuneStart(cleaned[limit]) {
			break
		}
		limit--
	}
	return cleaned[:limit]
}

func cloneReceiptBatch(b fleet.AuditReceiptBatch) fleet.AuditReceiptBatch {
	if len(b.Entries) > 0 {
		b.Entries = slices.Clone(b.Entries)
	}
	return b
}
