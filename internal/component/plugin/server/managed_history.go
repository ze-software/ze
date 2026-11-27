// Design: docs/architecture/fleet-config.md -- hub-side fleet metric history store, rings, persistence and queries
// Related: managed_serve.go -- ManagedServer owns the history store lifecycle and sessions
// Related: fleet_report.go -- sample-report receiver dispatches validated payloads here
// Related: managed_record_store.go -- device record deletion fences and erases metric history

package server

import (
	"bytes"
	"cmp"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"math"
	"math/bits"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	// HistoryFineHorizonMinutes is the 1-day fine resolution horizon in 1-minute UTC buckets.
	HistoryFineHorizonMinutes = 24 * 60
	// HistoryCoarseSlotMinutes is the width of one coarse UTC bucket in minutes.
	HistoryCoarseSlotMinutes = 15
	// HistoryCoarseSlotsPerDay is the number of 15-minute UTC slots in one UTC day.
	HistoryCoarseSlotsPerDay = 24 * 60 / HistoryCoarseSlotMinutes
	// HistoryCoarseMaxSlots is the maximum number of 15-minute UTC slots at max retention (7 days).
	HistoryCoarseMaxSlots = fleet.MaxHistoryRetentionDays * HistoryCoarseSlotsPerDay
	// HistorySeriesMaxPoints is the maximum point count accepted by the series points operator.
	HistorySeriesMaxPoints = HistoryFineHorizonMinutes + HistoryCoarseMaxSlots
	// HistoryQueryMaxRows is the maximum number of result rows allowed to leave a history query.
	HistoryQueryMaxRows = 4096
	// HistoryQueryMaxBytes is the maximum serialized JSON byte size allowed to leave a history query.
	HistoryQueryMaxBytes = 4 << 20

	defaultHistoryCheckpointInterval = 5 * time.Minute
	historyPersistSchemaVersion      = 1
	historyShardMagic                = "ZEH1"
	historyShardKindFine             = 1
	historyShardKindCoarse           = 2
	historyShardFlagZlib             = 1
	historyShardNameFine             = "fine"
	historyShardNameCoarse           = "coarse"
	historyShardUncompressedMax      = 4 << 20
	historyWriteHeadroomBytes        = 512

	historyStatusOK          = "ok"
	historyStatusNoReports   = "no-reports"
	historyStatusUnsupported = "unsupported"

	historyResolutionFine     = "fine"
	historyResolutionCoarse   = "coarse"
	historyResolutionCombined = "combined"

	historyRefineFix = "narrow the query with | device, | label, | figure, | from / | until, | reduce, or | points"
)

// HistoryDeviceMeta captures live registry metadata for one managed device at
// query time so latest, series, and storage answers carry current labels and
// connection state.
type HistoryDeviceMeta struct {
	Name      string
	Declared  bool
	Connected bool
	Labels    map[string]string
}

// ManagedHistoryStoreConfig configures a ManagedHistoryStore instance.
type ManagedHistoryStoreConfig struct {
	Config             plugin.HubHistoryConfig
	Store              storage.Storage
	Clock              clock.Clock
	Now                func() time.Time
	CheckpointInterval time.Duration
	DeclaredClients    int
	// OnHealthChanged is optionally invoked outside h.mu after CheckpointOnce
	// updates persisted byte accounting or checkpoint status.
	OnHealthChanged func(ManagedHistoryHealth)
}

// ManagedHistoryHealth summarizes hub-side metric history durability and
// capacity status for runtime doctor checks.
type ManagedHistoryHealth struct {
	CheckpointFailed          bool
	LastCheckpointError       string
	LastCheckpointOKAt        time.Time
	LastCheckpointFailAt      time.Time
	LastCheckpointAgeSeconds  int64
	CapacityPressure          bool
	UsedBytes                 int64
	HubBudgetBytes            int64
	DeviceBudgetBytes         int64
	ConfiguredRetentionDays   int
	EffectiveRetentionSeconds int64
	CorruptRecords            []ManagedDeviceRecordFailure
}

// HistoryLatestFilter narrows a latest-samples query.
type HistoryLatestFilter struct {
	Device string
	Labels map[string]string
	Figure string
}

// HistoryLatestRow is one authorized device's latest metric observation.
type HistoryLatestRow struct {
	Device      string             `json:"device"`
	Declared    bool               `json:"declared"`
	Connected   bool               `json:"connected"`
	Labels      map[string]string  `json:"labels,omitempty"`
	HasReport   bool               `json:"has-report"`
	Unsupported bool               `json:"unsupported,omitempty"`
	Status      string             `json:"status"`
	ReceivedAt  time.Time          `json:"received-at,omitzero"`
	AgeSeconds  int64              `json:"age-seconds,omitempty"`
	DeviceTime  string             `json:"device-time,omitempty"`
	Figures     map[string]float64 `json:"figures,omitempty"`
}

// HistoryLatestReducedRow is one figure's folded latest reading across a set of
// authorized devices.
type HistoryLatestReducedRow struct {
	Figure       string   `json:"figure"`
	Unit         string   `json:"unit"`
	Aggregation  string   `json:"aggregation"`
	Value        *float64 `json:"value,omitempty"`
	Ratio        *float64 `json:"ratio,omitempty"`
	Contributors int      `json:"contributors"`
	Missing      int      `json:"missing"`
}

// HistorySeriesQuery configures a series query across one or more devices.
type HistorySeriesQuery struct {
	Device string
	Labels map[string]string
	Figure string
	From   time.Time
	Until  time.Time
	Reduce bool
	Points int
}

// HistorySeriesRow is one timestamped bucket in a series query response.
type HistorySeriesRow struct {
	Device          string            `json:"device,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Figure          string            `json:"figure"`
	Unit            string            `json:"unit"`
	Resolution      string            `json:"resolution"`
	Start           time.Time         `json:"start"`
	End             time.Time         `json:"end"`
	Value           *float64          `json:"value,omitempty"`
	Sum             float64           `json:"sum,omitempty"`
	Max             *float64          `json:"max,omitempty"`
	Ratio           *float64          `json:"ratio,omitempty"`
	Count           uint32            `json:"count"`
	ObservedMinutes uint32            `json:"observed-minutes"`
	TotalMinutes    uint32            `json:"total-minutes"`
	Coverage        float64           `json:"coverage"`
	Gap             bool              `json:"gap"`
	Contributors    int               `json:"contributors"`
	Missing         int               `json:"missing"`
}

// HistoryStorageDeviceUsage reports persisted byte accounting for one
// authorized device.
type HistoryStorageDeviceUsage struct {
	Device                    string    `json:"device"`
	UsedBytes                 int64     `json:"used-bytes"`
	LatestBytes               int64     `json:"latest-bytes"`
	ShardBytes                int64     `json:"shard-bytes"`
	OldestSample              time.Time `json:"oldest-sample,omitzero"`
	NewestSample              time.Time `json:"newest-sample,omitzero"`
	CapacityPressure          bool      `json:"capacity-pressure"`
	EffectiveRetentionSeconds int64     `json:"effective-retention-seconds"`
}

// HistoryStorageStatus reports hub-wide metric history storage capacity,
// retention, checkpoint health, and caller-scoped per-device usage.
type HistoryStorageStatus struct {
	IntervalSeconds           int                         `json:"interval-seconds"`
	ConfiguredRetentionDays   int                         `json:"configured-retention-days"`
	EffectiveRetentionSeconds int64                       `json:"effective-retention-seconds"`
	UsedBytes                 int64                       `json:"used-bytes"`
	HubBudgetBytes            int64                       `json:"hub-budget-bytes"`
	DeviceBudgetBytes         int64                       `json:"device-budget-bytes"`
	CapacityPressure          bool                        `json:"capacity-pressure"`
	OldestSample              time.Time                   `json:"oldest-sample,omitzero"`
	DeviceCount               int                         `json:"device-count"`
	LastCheckpointAt          time.Time                   `json:"last-checkpoint-at,omitzero"`
	LastCheckpointAgeSeconds  int64                       `json:"last-checkpoint-age-seconds,omitempty"`
	LastCheckpointError       string                      `json:"last-checkpoint-error,omitempty"`
	CheckpointFailed          bool                        `json:"checkpoint-failed"`
	CorruptShards             int                         `json:"corrupt-shards,omitempty"`
	RejectedReports           uint64                      `json:"rejected-reports"`
	RateLimitedReports        uint64                      `json:"rate-limited-reports"`
	Devices                   []HistoryStorageDeviceUsage `json:"devices"`
}

// fineBucket holds one occupied 1-minute UTC slot for a single device figure.
type fineBucket struct {
	Minute int64
	Sum    float64
	Max    float64
	Count  uint16
}

// fineRing is a lazily grown circular buffer of strictly ascending 1-minute UTC
// buckets, bounded by HistoryFineHorizonMinutes.
type fineRing struct {
	buf   []fineBucket
	head  int
	count int
}

// coarseBucket holds one occupied 15-minute UTC slot for a single device
// figure. Its packed per-minute sums live in coarseRing.minuteVals.
type coarseBucket struct {
	Slot15     int64
	Sum        float64
	Max        float64
	Count      uint32
	MinuteMask uint16
}

// coarseRing is a lazily grown circular buffer of strictly ascending 15-minute
// UTC buckets paired with a flat packed circular buffer of per-minute sums (and
// lazily allocated per-minute counts only when sub-minute intervals observe
// multiple samples in one minute).
type coarseRing struct {
	buckets      []coarseBucket
	head         int
	count        int
	minuteVals   []float64
	minuteCounts []uint16 // Nil when every contributing minute has count == 1.
	valHead      int
	valCount     int
}

type historySeries struct {
	fine   fineRing
	coarse coarseRing
}

type historyLatest struct {
	hasReport        bool
	unsupported      bool
	receivedAt       time.Time
	deviceTime       string
	lastAcceptedUnix int64
	lastAcceptedSlot int64
	lastIntervalSec  int
	values           map[string]float64
}

type historyDevice struct {
	name                string
	generation          uint64
	mutationSeq         uint64
	latest              historyLatest
	series              []*historySeries // Indexed by canonical vocabulary index; nil until reported.
	dirty               bool
	latestBytes         int64
	persistedShardBytes int64
	unflushedRawDelta   int64
	hasFineShardOnDisk  bool
	hasCoarseOnDisk     bool
	evictedByCap        bool
	evictedCutoff       time.Time
}

type persistedHistoryLatest struct {
	SchemaVersion    int                `json:"schema-version"`
	Device           string             `json:"device"`
	HasReport        bool               `json:"has-report"`
	Unsupported      bool               `json:"unsupported,omitempty"`
	ReceivedAt       time.Time          `json:"received-at,omitzero"`
	DeviceTime       string             `json:"device-time,omitempty"`
	LastAcceptedUnix int64              `json:"last-accepted-unix,omitempty"`
	LastAcceptedSlot int64              `json:"last-accepted-slot,omitempty"`
	LastIntervalSec  int                `json:"last-interval-sec,omitempty"`
	Values           map[string]float64 `json:"values,omitempty"`
}

type persistedHistoryDeleteIntent struct {
	SchemaVersion int       `json:"schema-version"`
	Device        string    `json:"device"`
	Generation    uint64    `json:"generation"`
	DeletedAt     time.Time `json:"deleted-at"`
}

type historyDeviceFlushPayload struct {
	name        string
	generation  uint64
	mutationSeq uint64
	latestJSON  []byte
	fineBlob    []byte
	coarseBlob  []byte
	hadFine     bool
	hadCoarse   bool
}

type historyDeviceRawClone struct {
	name        string
	generation  uint64
	mutationSeq uint64
	latest      historyLatest
	series      []*historySeries
	hadFine     bool
	hadCoarse   bool
}

type historySeriesBucketAccum struct {
	start        time.Time
	end          time.Time
	resolution   string
	sum          float64
	maxVal       float64
	hasVal       bool
	value        float64
	count        uint32
	obsMinutes   uint32
	totalMinutes uint32
	contributors int
	missing      int
	pairedNumSum float64
	pairedDenSum float64
	hasPaired    bool
	contribMask  map[int]struct{}
}

// ManagedHistoryStore holds bounded dual-resolution metric history for managed
// devices, handles UTC-slot admission and backwards-clock protection, manages
// periodic and shutdown checkpointing, fences deletions against in-flight
// writes, and answers authorized latest, series, and storage queries.
// Safe for concurrent use.
type ManagedHistoryStore struct {
	store              storage.Storage
	now                func() time.Time
	checkpointInterval time.Duration

	// ioMu serializes all storage reads/writes/deletes performed by checkpoint
	// flushes and deletion fencing. Never acquired while holding mu.
	ioMu sync.Mutex

	// mu guards all in-memory history state. Lock ordering with ManagedServer:
	// ALWAYS acquire ManagedServer.mu before ManagedHistoryStore.mu.
	mu                   sync.Mutex
	cfg                  plugin.HubHistoryConfig
	declaredClients      int
	devices              map[string]*historyDevice
	deviceGen            map[string]uint64
	pendingDeletes       map[string]uint64
	totalAccountedBytes  int64
	hubEvictedByCap      bool
	hubEvictedCutoff     time.Time
	lastCheckpointOKAt   time.Time
	lastCheckpointFailAt time.Time
	lastCheckpointErr    string
	checkpointFailed     bool
	corruptRecords       map[string]ManagedDeviceRecordFailure
	rejectedReports      uint64
	rateLimitedReports   uint64

	deleteWake      chan struct{}
	onHealthChanged func(ManagedHistoryHealth)

	// Optional test hooks (nil in production).
	beforeShardWrite func(device, shard string)
}

var (
	historyFigureSpecs = fleet.AllSampleFigures()
	historyFigureIdx   = buildHistoryFigureIndex(historyFigureSpecs)
)

func buildHistoryFigureIndex(specs []fleet.SampleFigureSpec) map[string]int {
	idx := make(map[string]int, len(specs))
	for i := range specs {
		idx[specs[i].Name] = i
	}
	return idx
}

func normalizeHistoryConfig(cfg plugin.HubHistoryConfig) plugin.HubHistoryConfig {
	if cfg == (plugin.HubHistoryConfig{}) {
		return plugin.DefaultHubHistoryConfig()
	}
	return cfg
}

// NewManagedHistoryStore constructs a bounded hub metric history store. Devices
// are allocated lazily on first accepted report or storage load; offline
// declared devices allocate no rings.
func NewManagedHistoryStore(cfg ManagedHistoryStoreConfig) *ManagedHistoryStore {
	hcfg := normalizeHistoryConfig(cfg.Config)
	nowFn := cfg.Now
	if nowFn == nil {
		if cfg.Clock != nil {
			nowFn = cfg.Clock.Now
		} else {
			nowFn = clock.RealClock{}.Now
		}
	}
	cpInterval := cfg.CheckpointInterval
	if cpInterval <= 0 {
		cpInterval = defaultHistoryCheckpointInterval
	}
	return &ManagedHistoryStore{
		store:              cfg.Store,
		now:                nowFn,
		checkpointInterval: cpInterval,
		cfg:                hcfg,
		declaredClients:    max(0, cfg.DeclaredClients),
		devices:            make(map[string]*historyDevice),
		deviceGen:          make(map[string]uint64),
		pendingDeletes:     make(map[string]uint64),
		corruptRecords:     make(map[string]ManagedDeviceRecordFailure),
		deleteWake:         make(chan struct{}, 1),
		onHealthChanged:    cfg.OnHealthChanged,
	}
}

func (h *ManagedHistoryStore) currentTime() time.Time {
	if h.now != nil {
		return h.now().UTC()
	}
	return time.Now().UTC()
}

// IntervalSeconds returns the currently configured metric sample interval in
// seconds (0 disables sampling). Safe for concurrent use.
func (h *ManagedHistoryStore) IntervalSeconds() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cfg.IntervalSeconds
}

// Config returns the active hub history configuration. Safe for concurrent use.
func (h *ManagedHistoryStore) Config() plugin.HubHistoryConfig {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cfg
}

// HasDevice reports whether the store holds any retained state (latest or
// series) for the named device. Safe for concurrent use.
func (h *ManagedHistoryStore) HasDevice(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.devices[name]
	return ok
}

// RecordRejected increments the rejected sample-report counter without mutating
// any device history. Safe for concurrent use.
func (h *ManagedHistoryStore) RecordRejected() {
	h.mu.Lock()
	h.rejectedReports++
	h.mu.Unlock()
}

// SetDeclaredCount updates the declared managed device count used for
// latest-observation reserve accounting. Safe for concurrent use.
func (h *ManagedHistoryStore) SetDeclaredCount(declared int) {
	now := h.currentTime()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.declaredClients = max(0, declared)
	h.enforceBudgetsLocked(now)
}

// UpdateConfig installs validated history settings after a reload and
// immediately enforces any tightened retention or byte caps. Safe for
// concurrent use.
func (h *ManagedHistoryStore) UpdateConfig(next plugin.HubHistoryConfig, declared int) {
	now := h.currentTime()
	h.mu.Lock()
	defer h.mu.Unlock()
	prev := h.cfg
	h.cfg = normalizeHistoryConfig(next)
	h.declaredClients = max(0, declared)

	// Clear stale capacity-pressure marks when budgets increased or retention
	// shrank past previous eviction cutoffs; enforceBudgetsLocked will re-raise
	// pressure immediately if the new budget still requires eviction.
	horizonStart := now.Add(-h.configuredRetentionLocked())
	if h.cfg.HubBudgetBytes > prev.HubBudgetBytes || !h.hubEvictedCutoff.After(horizonStart) {
		h.hubEvictedByCap = false
		h.hubEvictedCutoff = time.Time{}
	}
	for _, dev := range h.devices {
		if h.cfg.DeviceBudgetBytes > prev.DeviceBudgetBytes || !dev.evictedCutoff.After(horizonStart) {
			dev.evictedByCap = false
			dev.evictedCutoff = time.Time{}
		}
	}
	h.pruneAllExpiredLocked(now)
	h.enforceBudgetsLocked(now)
}

// MarkUnsupported records an explicit device declaration that metric sample
// reporting is unsupported, without creating synthetic series points. Safe for
// concurrent use.
func (h *ManagedHistoryStore) MarkUnsupported(device string, now time.Time) {
	nowUTC := now.UTC()
	h.mu.Lock()
	defer h.mu.Unlock()
	var beforeBytes int64
	if existing := h.devices[device]; existing != nil {
		if existing.latest.hasReport {
			return
		}
		beforeBytes = h.deviceAccountedBytesLocked(existing)
	}
	dev := h.ensureDeviceLocked(device)
	dev.latest.unsupported = true
	dev.latest.receivedAt = nowUTC
	dev.mutationSeq++
	dev.dirty = true
	h.refreshLatestChargeLocked(dev)
	h.totalAccountedBytes += h.deviceAccountedBytesLocked(dev) - beforeBytes
	h.enforceDeviceAndHubBudgetLocked(dev, nowUTC)
}

// Ingest validates admission rate limits and wall-clock monotonicity for a
// pre-validated SampleReport and inserts its readings into the device's latest
// record and fine/coarse UTC rings. A rejected report leaves latest, timestamp,
// next eligible slot, and all rings completely unchanged. Safe for concurrent
// use.
func (h *ManagedHistoryStore) Ingest(device string, report *fleet.SampleReport, now time.Time) error {
	if device == "" {
		h.RecordRejected()
		return errors.New("fleet history: device name is required")
	}
	if report == nil {
		h.RecordRejected()
		return errors.New("fleet history: sample report is nil")
	}
	if err := report.Validate(); err != nil {
		h.RecordRejected()
		return err
	}

	nowUTC := now.UTC()
	nowUnix := nowUTC.Unix()
	if nowUnix < 0 {
		h.RecordRejected()
		return errors.New("fleet history: receipt time precedes Unix epoch")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cfg.IntervalSeconds <= 0 {
		h.rejectedReports++
		return errors.New("fleet sample reporting is disabled on this hub")
	}
	interval := int64(h.cfg.IntervalSeconds)
	slot := nowUnix / interval

	dev := h.devices[device]
	if dev != nil && dev.latest.hasReport {
		if nowUnix < dev.latest.lastAcceptedUnix {
			h.rejectedReports++
			h.rateLimitedReports++
			return errors.New("sample report rejected: hub wall clock stepped backwards behind last accepted sample")
		}
		if dev.latest.lastIntervalSec == h.cfg.IntervalSeconds {
			if slot <= dev.latest.lastAcceptedSlot {
				h.rejectedReports++
				h.rateLimitedReports++
				return errors.New("sample report rate limited: already accepted a report in this UTC interval")
			}
		} else {
			prevSlotInNew := dev.latest.lastAcceptedUnix / interval
			if slot <= prevSlotInNew {
				h.rejectedReports++
				h.rateLimitedReports++
				return errors.New("sample report rate limited: already accepted a report in this UTC interval")
			}
			if nowUnix == dev.latest.lastAcceptedUnix {
				h.rejectedReports++
				h.rateLimitedReports++
				return errors.New("sample report rate limited: already accepted a report in this UTC interval")
			}
		}
	}

	var beforeBytes int64
	if dev != nil {
		beforeBytes = h.deviceAccountedBytesLocked(dev)
	} else {
		dev = h.ensureDeviceLocked(device)
	}

	// Update latest independently of bucket eviction. Absent figures stay absent;
	// measured zero values are retained explicitly.
	values := make(map[string]float64, len(report.Samples))
	for _, reading := range report.Samples {
		values[reading.Figure] = reading.Value
	}
	dev.latest = historyLatest{
		hasReport:        true,
		unsupported:      false,
		receivedAt:       nowUTC,
		deviceTime:       report.DeviceTime,
		lastAcceptedUnix: nowUnix,
		lastAcceptedSlot: slot,
		lastIntervalSec:  h.cfg.IntervalSeconds,
		values:           values,
	}

	minute := nowUnix / 60
	slot15 := nowUnix / (HistoryCoarseSlotMinutes * 60)
	minuteOffset := int(minute % HistoryCoarseSlotMinutes)
	maxCoarse := h.cfg.RetentionDays * HistoryCoarseSlotsPerDay

	var addedRaw int64
	for _, reading := range report.Samples {
		figIdx, ok := historyFigureIdx[reading.Figure]
		if !ok {
			continue
		}
		ser := dev.series[figIdx]
		if ser == nil {
			ser = &historySeries{}
			dev.series[figIdx] = ser
			addedRaw += 32 // per-series header overhead estimate
		}
		if ser.fine.push(minute, reading.Value, HistoryFineHorizonMinutes) {
			addedRaw += 26
		}
		addedCoarseBytes := ser.coarse.push(slot15, minuteOffset, reading.Value, maxCoarse)
		addedRaw += int64(addedCoarseBytes)
	}

	dev.mutationSeq++
	dev.dirty = true
	dev.unflushedRawDelta += addedRaw
	h.refreshLatestChargeLocked(dev)
	h.totalAccountedBytes += h.deviceAccountedBytesLocked(dev) - beforeBytes
	h.enforceDeviceAndHubBudgetLocked(dev, nowUTC)
	return nil
}

func (h *ManagedHistoryStore) ensureDeviceLocked(name string) *historyDevice {
	if existing := h.devices[name]; existing != nil {
		return existing
	}
	gen := h.deviceGen[name] + 1
	h.deviceGen[name] = gen
	dev := &historyDevice{
		name:       name,
		generation: gen,
		series:     make([]*historySeries, len(historyFigureSpecs)),
	}
	h.devices[name] = dev
	return dev
}

// FenceForgetDevice immediately fences in-flight checkpoint flushes for name,
// removes its in-memory history, and queues durable deletion intent and shard
// erasure on the history worker. It performs no storage I/O and never acquires
// ioMu, so it is safe to call while holding ManagedServer.mu or inside a
// storage RuntimeObserver callback. Safe for concurrent use.
func (h *ManagedHistoryStore) FenceForgetDevice(name string) {
	if name == "" {
		return
	}
	h.mu.Lock()
	gen := h.deviceGen[name] + 1
	h.deviceGen[name] = gen
	if existing := h.devices[name]; existing != nil {
		h.totalAccountedBytes -= h.deviceAccountedBytesLocked(existing)
		if h.totalAccountedBytes < 0 {
			h.totalAccountedBytes = 0
		}
		delete(h.devices, name)
	}
	for key, fail := range h.corruptRecords {
		if fail.Name == name {
			delete(h.corruptRecords, key)
		}
	}
	h.pendingDeletes[name] = gen
	h.mu.Unlock()

	select {
	case h.deleteWake <- struct{}{}:
	default:
	}
}

// ForgetDevice fences concurrent checkpoint writes for name and synchronously
// persists deletion intent, erases all stored shards and latest record, and
// removes the deletion intent. Safe for concurrent use.
func (h *ManagedHistoryStore) ForgetDevice(name string) error {
	h.FenceForgetDevice(name)
	return h.DrainPendingDeletes()
}

// DrainPendingDeletes executes all queued deletion intents under ioMu. Safe for
// concurrent use.
func (h *ManagedHistoryStore) DrainPendingDeletes() error {
	h.ioMu.Lock()
	defer h.ioMu.Unlock()
	return h.drainPendingDeletesLockedIO()
}

func (h *ManagedHistoryStore) drainPendingDeletesLockedIO() error {
	for {
		h.mu.Lock()
		var target string
		var targetGen uint64
		for name, gen := range h.pendingDeletes {
			target = name
			targetGen = gen
			break
		}
		h.mu.Unlock()
		if target == "" {
			return nil
		}
		if err := h.eraseDeviceDurableLockedIO(target, targetGen); err != nil {
			h.recordCheckpointFailure(err)
			return err
		}
		h.mu.Lock()
		if h.pendingDeletes[target] == targetGen {
			delete(h.pendingDeletes, target)
		}
		h.mu.Unlock()
	}
}

func (h *ManagedHistoryStore) eraseDeviceDurableLockedIO(name string, gen uint64) error {
	if h.store == nil {
		return nil
	}
	intentKey := historyDeleteIntentKey(name)
	intent := persistedHistoryDeleteIntent{
		SchemaVersion: historyPersistSchemaVersion,
		Device:        name,
		Generation:    gen,
		DeletedAt:     h.currentTime(),
	}
	intentBytes, err := json.Marshal(intent)
	if err != nil {
		return fmt.Errorf("encode fleet history delete intent for %s: %w", name, err)
	}
	if len(intentBytes) > zefs.FleetHistoryDeleteIntentValueMax {
		return fmt.Errorf("fleet history delete intent for %s exceeds max %d bytes", name, zefs.FleetHistoryDeleteIntentValueMax)
	}
	if err := h.store.WriteKey(intentKey, intentBytes); err != nil {
		return fmt.Errorf("persist fleet history delete intent %s: %w", intentKey, err)
	}
	if err := h.removeDeviceKeysLockedIO(name); err != nil {
		return err
	}
	if err := h.store.RemoveKey(intentKey); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove fleet history delete intent %s: %w", intentKey, err)
	}
	return nil
}

func (h *ManagedHistoryStore) removeDeviceKeysLockedIO(name string) error {
	shardPrefix := historyDeviceShardPrefix(name)
	keys, err := h.store.ListKeys(shardPrefix)
	if err != nil {
		return fmt.Errorf("list fleet history shards %s: %w", shardPrefix, err)
	}
	for _, key := range keys {
		if rmErr := h.store.RemoveKey(key); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			return fmt.Errorf("remove fleet history shard %s: %w", key, rmErr)
		}
	}
	latestKey := historyLatestKey(name)
	if rmErr := h.store.RemoveKey(latestKey); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
		return fmt.Errorf("remove fleet history latest %s: %w", latestKey, rmErr)
	}
	return nil
}

// LoadFromStore replays any interrupted deletion intents first, then loads
// persisted latest observations and compressed fine/coarse shards from the
// borrowed store. Corrupt or unsupported records are recorded for doctor and
// skipped without losing healthy shards. Safe for concurrent use.
func (h *ManagedHistoryStore) LoadFromStore() error {
	// refuse-by-default: guards ManagedServer.Start -- a server built without storage runs memory-only history
	if h.store == nil {
		return nil
	}
	h.ioMu.Lock()
	defer h.ioMu.Unlock()

	// Step 1: Replay any interrupted deletion intents before loading any data.
	intentKeys, err := h.store.ListKeys(zefs.FleetHistoryDeleteIntentPrefix)
	if err != nil {
		return fmt.Errorf("list fleet history deletion intents: %w", err)
	}
	for _, key := range intentKeys {
		name, ok := parseHistorySingleKey(zefs.FleetHistoryDeleteIntentPrefix, key)
		if !ok {
			continue
		}
		raw, readErr := h.store.ReadKey(key)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return fmt.Errorf("read fleet history deletion intent %s: %w", key, readErr)
		}
		if len(raw) > zefs.FleetHistoryDeleteIntentValueMax {
			h.recordCorruptLocked(name, key, fmt.Errorf("deletion intent size %d exceeds max %d", len(raw), zefs.FleetHistoryDeleteIntentValueMax))
		}
		if rmErr := h.removeDeviceKeysLockedIO(name); rmErr != nil {
			return rmErr
		}
		if rmErr := h.store.RemoveKey(key); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			return fmt.Errorf("remove replayed fleet history deletion intent %s: %w", key, rmErr)
		}
	}

	// Step 2: Load latest observations.
	latestKeys, err := h.store.ListKeys(zefs.FleetHistoryLatestPrefix)
	if err != nil {
		return fmt.Errorf("list fleet history latest keys: %w", err)
	}
	for _, key := range latestKeys {
		name, ok := parseHistorySingleKey(zefs.FleetHistoryLatestPrefix, key)
		if !ok {
			continue
		}
		raw, readErr := h.store.ReadKey(key)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			h.recordCorruptLocked(name, key, readErr)
			continue
		}
		parsed, decErr := decodeHistoryLatest(name, raw)
		if decErr != nil {
			h.recordCorruptLocked(name, key, decErr)
			continue
		}
		h.mu.Lock()
		dev := h.ensureDeviceLocked(name)
		dev.latest = parsed
		dev.latestBytes = int64(len(raw))
		h.mu.Unlock()
	}

	// Step 3: Load compressed fine and coarse shards.
	shardKeys, err := h.store.ListKeys(zefs.FleetHistoryShardPrefix)
	if err != nil {
		return fmt.Errorf("list fleet history shard keys: %w", err)
	}
	for _, key := range shardKeys {
		name, shardKind, ok := parseHistoryShardKey(key)
		if !ok {
			continue
		}
		raw, readErr := h.store.ReadKey(key)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			h.recordCorruptLocked(name, key, readErr)
			continue
		}
		switch shardKind {
		case historyShardNameFine:
			loaded, decErr := decodeFineShard(name, raw)
			if decErr != nil {
				h.recordCorruptLocked(name, key, decErr)
				continue
			}
			h.mu.Lock()
			dev := h.ensureDeviceLocked(name)
			for figIdx, ring := range loaded {
				if dev.series[figIdx] == nil {
					dev.series[figIdx] = &historySeries{}
				}
				dev.series[figIdx].fine = ring
			}
			dev.persistedShardBytes += int64(len(raw))
			dev.hasFineShardOnDisk = true
			h.mu.Unlock()

		case historyShardNameCoarse:
			loaded, decErr := decodeCoarseShard(name, raw)
			if decErr != nil {
				h.recordCorruptLocked(name, key, decErr)
				continue
			}
			h.mu.Lock()
			dev := h.ensureDeviceLocked(name)
			for figIdx, ring := range loaded {
				if dev.series[figIdx] == nil {
					dev.series[figIdx] = &historySeries{}
				}
				dev.series[figIdx].coarse = ring
			}
			dev.persistedShardBytes += int64(len(raw))
			dev.hasCoarseOnDisk = true
			h.mu.Unlock()
		}
	}

	now := h.currentTime()
	h.mu.Lock()
	h.pruneAllExpiredLocked(now)
	h.recomputeTotalAccountedLocked()
	h.enforceBudgetsLocked(now)
	h.mu.Unlock()
	return nil
}

func (h *ManagedHistoryStore) recordCorruptLocked(device, key string, err error) {
	managedLogger().Warn("fleet metric history record could not be decoded and was skipped",
		"code", CodeDoctorManagedHistoryCorrupt, "device", device, "key", key, "error", err)
	h.mu.Lock()
	h.corruptRecords[key] = ManagedDeviceRecordFailure{
		Name:  device,
		Key:   key,
		Error: boundedManagedDetail(err.Error()),
	}
	h.mu.Unlock()
}

// runCheckpointLoop runs as a long-lived worker owned by ManagedServer until
// ctx is canceled. It handles periodic checkpoints and immediate delete-intent
// wakes.
func (h *ManagedHistoryStore) runCheckpointLoop(ctx context.Context) {
	ticker := time.NewTicker(h.checkpointInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.deleteWake:
			if err := h.DrainPendingDeletes(); err != nil && ctx.Err() == nil {
				managedLogger().Warn("fleet metric history deletion flush failed; will retry",
					"code", CodeDoctorManagedHistoryCheckpointFailed, "error", err)
			}
		case <-ticker.C:
			if err := h.CheckpointOnce(); err != nil && ctx.Err() == nil {
				managedLogger().Warn("fleet metric history checkpoint failed; dirty data retained for retry",
					"code", CodeDoctorManagedHistoryCheckpointFailed, "error", err)
			}
		}
	}
}

// FlushFinal drains all pending device deletions and flushes every dirty device
// before shutdown releases the borrowed store. Safe for concurrent use.
func (h *ManagedHistoryStore) FlushFinal() error {
	return h.CheckpointOnce()
}

// CheckpointOnce drains queued deletion intents first, then flushes every dirty
// device independently of the ingest lock with per-device generation checks.
// On write failure, failed devices remain dirty for retry while healthy writes
// stay committed. Safe for concurrent use.
func (h *ManagedHistoryStore) CheckpointOnce() error {
	err := h.checkpointOnceIO()
	h.notifyHealthChanged(h.currentTime())
	return err
}

func (h *ManagedHistoryStore) notifyHealthChanged(now time.Time) {
	if h.onHealthChanged == nil {
		return
	}
	h.onHealthChanged(h.HealthSnapshot(now))
}

func (h *ManagedHistoryStore) checkpointOnceIO() error {
	// refuse-by-default: guards runCheckpointLoop -- memory-only stores perform no disk checkpointing
	if h.store == nil {
		h.mu.Lock()
		clear(h.pendingDeletes)
		for _, dev := range h.devices {
			dev.dirty = false
		}
		h.mu.Unlock()
		return nil
	}

	h.ioMu.Lock()
	defer h.ioMu.Unlock()

	var errs []error
	if delErr := h.drainPendingDeletesLockedIO(); delErr != nil {
		errs = append(errs, delErr)
	}

	// Capture list of dirty devices (sorted for deterministic checkpoint order).
	h.mu.Lock()
	dirtyNames := make([]string, 0, len(h.devices))
	for name, dev := range h.devices {
		if dev.dirty {
			dirtyNames = append(dirtyNames, name)
		}
	}
	h.mu.Unlock()
	slices.Sort(dirtyNames)

	for _, name := range dirtyNames {
		// Always honor newly arriving deletions before flushing more shards.
		if delErr := h.drainPendingDeletesLockedIO(); delErr != nil {
			errs = append(errs, delErr)
		}
		if err := h.flushOneDeviceLockedIO(name); err != nil {
			errs = append(errs, err)
		}
	}
	// Drain any deletions that raced with the final dirty device flush.
	if delErr := h.drainPendingDeletesLockedIO(); delErr != nil {
		errs = append(errs, delErr)
	}

	if len(errs) > 0 {
		joined := errors.Join(errs...)
		h.recordCheckpointFailure(joined)
		return joined
	}

	now := h.currentTime()
	h.mu.Lock()
	h.lastCheckpointOKAt = now
	h.lastCheckpointErr = ""
	h.checkpointFailed = false
	h.mu.Unlock()
	return nil
}

func (h *ManagedHistoryStore) recordCheckpointFailure(err error) {
	now := h.currentTime()
	h.mu.Lock()
	h.checkpointFailed = true
	h.lastCheckpointFailAt = now
	h.lastCheckpointErr = boundedManagedDetail(err.Error())
	h.mu.Unlock()
}

func (h *ManagedHistoryStore) flushOneDeviceLockedIO(name string) error {
	// Clone minimal state under mu, then compress and write outside mu so
	// ingest is never blocked by zlib compression or disk I/O.
	h.mu.Lock()
	dev := h.devices[name]
	// refuse-by-default: guards CheckpointOnce -- a device forgotten between dirty-list capture and flush has nothing left to write
	if dev == nil {
		h.mu.Unlock()
		return nil
	}
	if !dev.dirty {
		h.mu.Unlock()
		return nil
	}
	cloned := cloneDeviceForFlushLocked(dev)
	h.mu.Unlock()

	snap, err := encodeDeviceFlushPayload(&cloned)
	if err != nil {
		return err
	}

	// Re-check generation before every write so a concurrent deletion fences
	// stale flushes immediately.
	if !h.deviceGenerationMatches(name, snap.generation) {
		return nil
	}

	latestKey := historyLatestKey(name)
	if len(snap.latestJSON) > zefs.FleetHistoryLatestValueMax {
		return fmt.Errorf("fleet history latest for %s size %d exceeds max %d", name, len(snap.latestJSON), zefs.FleetHistoryLatestValueMax)
	}
	if h.beforeShardWrite != nil {
		h.beforeShardWrite(name, "latest")
	}
	if !h.deviceGenerationMatches(name, snap.generation) {
		return nil
	}
	if err := h.store.WriteKey(latestKey, snap.latestJSON); err != nil {
		return fmt.Errorf("write fleet history latest %s: %w", latestKey, err)
	}

	fineKey := historyShardKey(name, historyShardNameFine)
	var wroteFine bool
	if len(snap.fineBlob) > 0 {
		if len(snap.fineBlob) > zefs.FleetHistoryShardValueMax {
			return fmt.Errorf("fleet history fine shard for %s size %d exceeds max %d", name, len(snap.fineBlob), zefs.FleetHistoryShardValueMax)
		}
		if h.beforeShardWrite != nil {
			h.beforeShardWrite(name, historyShardNameFine)
		}
		if !h.deviceGenerationMatches(name, snap.generation) {
			return nil
		}
		if err := h.store.WriteKey(fineKey, snap.fineBlob); err != nil {
			return fmt.Errorf("write fleet history fine shard %s: %w", fineKey, err)
		}
		wroteFine = true
	} else if snap.hadFine {
		if rmErr := h.store.RemoveKey(fineKey); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			return fmt.Errorf("remove empty fleet history fine shard %s: %w", fineKey, rmErr)
		}
	}

	coarseKey := historyShardKey(name, historyShardNameCoarse)
	var wroteCoarse bool
	if len(snap.coarseBlob) > 0 {
		if len(snap.coarseBlob) > zefs.FleetHistoryShardValueMax {
			return fmt.Errorf("fleet history coarse shard for %s size %d exceeds max %d", name, len(snap.coarseBlob), zefs.FleetHistoryShardValueMax)
		}
		if h.beforeShardWrite != nil {
			h.beforeShardWrite(name, historyShardNameCoarse)
		}
		if !h.deviceGenerationMatches(name, snap.generation) {
			return nil
		}
		if err := h.store.WriteKey(coarseKey, snap.coarseBlob); err != nil {
			return fmt.Errorf("write fleet history coarse shard %s: %w", coarseKey, err)
		}
		wroteCoarse = true
	} else if snap.hadCoarse {
		if rmErr := h.store.RemoveKey(coarseKey); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			return fmt.Errorf("remove empty fleet history coarse shard %s: %w", coarseKey, rmErr)
		}
	}

	// Verify generation and mutation sequence before clearing dirty flag and
	// updating persisted byte charge so concurrent eviction/pruning/ingest is
	// never masked or overwritten by a stale pre-mutation snapshot.
	h.mu.Lock()
	cur := h.devices[name]
	if cur != nil && cur.generation == snap.generation {
		cur.hasFineShardOnDisk = wroteFine
		cur.hasCoarseOnDisk = wroteCoarse
		if cur.mutationSeq == snap.mutationSeq {
			beforeBytes := h.deviceAccountedBytesLocked(cur)
			cur.latestBytes = int64(len(snap.latestJSON))
			cur.persistedShardBytes = int64(len(snap.fineBlob) + len(snap.coarseBlob))
			cur.dirty = false
			cur.unflushedRawDelta = 0
			h.totalAccountedBytes += h.deviceAccountedBytesLocked(cur) - beforeBytes
		}
		delete(h.corruptRecords, latestKey)
		delete(h.corruptRecords, fineKey)
		delete(h.corruptRecords, coarseKey)
		delete(h.corruptRecords, historyDeleteIntentKey(name))
	}
	h.mu.Unlock()
	return nil
}

func cloneDeviceForFlushLocked(dev *historyDevice) historyDeviceRawClone {
	latestCopy := dev.latest
	latestCopy.values = maps.Clone(dev.latest.values)
	seriesCopy := make([]*historySeries, len(dev.series))
	for i, ser := range dev.series {
		if ser == nil {
			continue
		}
		cp := &historySeries{}
		if ser.fine.count > 0 {
			cp.fine = fineRing{
				buf:   slices.Clone(ser.fine.buf),
				head:  ser.fine.head,
				count: ser.fine.count,
			}
		}
		if ser.coarse.count > 0 {
			cp.coarse = coarseRing{
				buckets:      slices.Clone(ser.coarse.buckets),
				head:         ser.coarse.head,
				count:        ser.coarse.count,
				minuteVals:   slices.Clone(ser.coarse.minuteVals),
				minuteCounts: slices.Clone(ser.coarse.minuteCounts),
				valHead:      ser.coarse.valHead,
				valCount:     ser.coarse.valCount,
			}
		}
		seriesCopy[i] = cp
	}
	return historyDeviceRawClone{
		name:        dev.name,
		generation:  dev.generation,
		mutationSeq: dev.mutationSeq,
		latest:      latestCopy,
		series:      seriesCopy,
		hadFine:     dev.hasFineShardOnDisk,
		hadCoarse:   dev.hasCoarseOnDisk,
	}
}

func encodeDeviceFlushPayload(cloned *historyDeviceRawClone) (historyDeviceFlushPayload, error) {
	latestJSON, err := encodeHistoryLatest(cloned.name, &cloned.latest)
	if err != nil {
		return historyDeviceFlushPayload{}, err
	}
	fineBlob, err := encodeFineShard(cloned.name, cloned.series)
	if err != nil {
		return historyDeviceFlushPayload{}, err
	}
	coarseBlob, err := encodeCoarseShard(cloned.name, cloned.series)
	if err != nil {
		return historyDeviceFlushPayload{}, err
	}
	return historyDeviceFlushPayload{
		name:        cloned.name,
		generation:  cloned.generation,
		mutationSeq: cloned.mutationSeq,
		latestJSON:  latestJSON,
		fineBlob:    fineBlob,
		coarseBlob:  coarseBlob,
		hadFine:     cloned.hadFine,
		hadCoarse:   cloned.hadCoarse,
	}, nil
}

func (h *ManagedHistoryStore) deviceGenerationMatches(name string, gen uint64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, deleting := h.pendingDeletes[name]; deleting {
		return false
	}
	dev := h.devices[name]
	return dev != nil && dev.generation == gen
}

// HealthSnapshot returns a point-in-time doctor health view of checkpoint
// durability, capacity pressure, and corrupt shards. Safe for concurrent use.
func (h *ManagedHistoryStore) HealthSnapshot(now time.Time) ManagedHistoryHealth {
	nowUTC := now.UTC()
	h.mu.Lock()
	defer h.mu.Unlock()

	health := ManagedHistoryHealth{
		CheckpointFailed:          h.checkpointFailed,
		LastCheckpointError:       h.lastCheckpointErr,
		LastCheckpointOKAt:        h.lastCheckpointOKAt,
		LastCheckpointFailAt:      h.lastCheckpointFailAt,
		CapacityPressure:          h.hasCapacityPressureLocked(nowUTC),
		UsedBytes:                 h.totalAccountedBytes,
		HubBudgetBytes:            h.cfg.HubBudgetBytes,
		DeviceBudgetBytes:         h.cfg.DeviceBudgetBytes,
		ConfiguredRetentionDays:   h.cfg.RetentionDays,
		EffectiveRetentionSeconds: int64(h.effectiveRetentionLocked(nowUTC).Seconds()),
	}
	if !h.lastCheckpointOKAt.IsZero() {
		age := nowUTC.Sub(h.lastCheckpointOKAt)
		if age > 0 {
			health.LastCheckpointAgeSeconds = int64(age.Seconds())
		}
	}
	if len(h.corruptRecords) > 0 {
		keys := slices.Sorted(maps.Keys(h.corruptRecords))
		health.CorruptRecords = make([]ManagedDeviceRecordFailure, 0, len(keys))
		for _, k := range keys {
			health.CorruptRecords = append(health.CorruptRecords, h.corruptRecords[k])
		}
	}
	return health
}

// -----------------------------------------------------------------------------
// Budget accounting, retention pruning, and oldest-first cap eviction
// -----------------------------------------------------------------------------

func (h *ManagedHistoryStore) configuredRetentionLocked() time.Duration {
	return time.Duration(h.cfg.RetentionDays) * 24 * time.Hour
}

func (h *ManagedHistoryStore) deviceKeyOverhead(name string) int64 {
	return int64(len(historyLatestKey(name)) + len(historyShardKey(name, historyShardNameFine)) + len(historyShardKey(name, historyShardNameCoarse)))
}

func (h *ManagedHistoryStore) refreshLatestChargeLocked(dev *historyDevice) {
	encoded, err := encodeHistoryLatest(dev.name, &dev.latest)
	if err == nil {
		dev.latestBytes = int64(len(encoded))
		return
	}
	dev.latestBytes = fleet.MinHistoryLatestReservePerDeviceBytes
}

func (h *ManagedHistoryStore) deviceAccountedBytesLocked(dev *historyDevice) int64 {
	return dev.latestBytes + h.deviceKeyOverhead(dev.name) + historyWriteHeadroomBytes + dev.persistedShardBytes + dev.unflushedRawDelta
}

func (h *ManagedHistoryStore) deviceExactShardBytesLocked(dev *historyDevice) error {
	fineBlob, err := encodeFineShard(dev.name, dev.series)
	if err != nil {
		return err
	}
	coarseBlob, err := encodeCoarseShard(dev.name, dev.series)
	if err != nil {
		return err
	}
	dev.persistedShardBytes = int64(len(fineBlob) + len(coarseBlob))
	dev.unflushedRawDelta = 0
	return nil
}

func (h *ManagedHistoryStore) recomputeTotalAccountedLocked() {
	var total int64
	for _, dev := range h.devices {
		total += h.deviceAccountedBytesLocked(dev)
	}
	h.totalAccountedBytes = total
}

func (h *ManagedHistoryStore) reserveForUnreportedDeclaredLocked() int64 {
	unreported := h.declaredClients - len(h.devices)
	if unreported <= 0 {
		return 0
	}
	return int64(unreported) * fleet.MinHistoryLatestReservePerDeviceBytes
}

func (h *ManagedHistoryStore) pruneAllExpiredLocked(now time.Time) {
	nowUnix := now.Unix()
	minFineMinute := (nowUnix / 60) - int64(HistoryFineHorizonMinutes) + 1
	maxCoarse := h.cfg.RetentionDays * HistoryCoarseSlotsPerDay
	minCoarseSlot := (nowUnix / (HistoryCoarseSlotMinutes * 60)) - int64(maxCoarse) + 1
	for _, dev := range h.devices {
		var pruned bool
		for _, ser := range dev.series {
			if ser == nil {
				continue
			}
			if ser.fine.evictBefore(minFineMinute) {
				pruned = true
			}
			if ser.coarse.evictBefore(minCoarseSlot) {
				pruned = true
			}
		}
		if pruned {
			dev.mutationSeq++
			dev.dirty = true
			_ = h.deviceExactShardBytesLocked(dev)
		}
	}
	h.recomputeTotalAccountedLocked()
}

func (h *ManagedHistoryStore) enforceDeviceAndHubBudgetLocked(dev *historyDevice, now time.Time) {
	h.enforceSingleDeviceBudgetLocked(dev, now)
	availableForDevices := h.availableHubDeviceBytesLocked()
	if h.totalAccountedBytes > availableForDevices {
		h.enforceHubBudgetLocked(now)
	}
}

func (h *ManagedHistoryStore) enforceSingleDeviceBudgetLocked(dev *historyDevice, now time.Time) {
	before := h.deviceAccountedBytesLocked(dev)
	if before <= h.cfg.DeviceBudgetBytes {
		return
	}
	// Measure exact compressed shard size first before evicting any data.
	_ = h.deviceExactShardBytesLocked(dev)
	afterMeasure := h.deviceAccountedBytesLocked(dev)
	h.totalAccountedBytes += afterMeasure - before
	if afterMeasure <= h.cfg.DeviceBudgetBytes {
		return
	}
	h.evictDeviceToBudgetLocked(dev, h.cfg.DeviceBudgetBytes, now)
	afterEvict := h.deviceAccountedBytesLocked(dev)
	h.totalAccountedBytes += afterEvict - afterMeasure
}

func (h *ManagedHistoryStore) availableHubDeviceBytesLocked() int64 {
	reserve := h.reserveForUnreportedDeclaredLocked()
	available := h.cfg.HubBudgetBytes - reserve
	if available < 0 {
		return 0
	}
	return available
}

func (h *ManagedHistoryStore) enforceBudgetsLocked(now time.Time) {
	// 1. Per-device budget enforcement (never evicts latest).
	for _, dev := range h.devices {
		h.enforceSingleDeviceBudgetLocked(dev, now)
	}

	// 2. Hub-wide budget enforcement (preserving latest reserve for all declared devices).
	h.recomputeTotalAccountedLocked()
	h.enforceHubBudgetLocked(now)
}

func (h *ManagedHistoryStore) enforceHubBudgetLocked(now time.Time) {
	availableForDevices := h.availableHubDeviceBytesLocked()
	if h.totalAccountedBytes <= availableForDevices {
		return
	}

	// Re-measure devices with unflushed raw deltas first so compression savings
	// are credited before evicting any history.
	for _, dev := range h.devices {
		if dev.unflushedRawDelta > 0 {
			before := h.deviceAccountedBytesLocked(dev)
			_ = h.deviceExactShardBytesLocked(dev)
			h.totalAccountedBytes += h.deviceAccountedBytesLocked(dev) - before
		}
	}
	if h.totalAccountedBytes <= availableForDevices {
		return
	}

	// Evict oldest buckets across the hub down to a low-watermark target so
	// subsequent samples do not re-trigger eviction on every single tick.
	hubHeadroom := min(availableForDevices/16, int64(64<<10))
	hubTarget := max(int64(0), availableForDevices-hubHeadroom)
	_ = now

	for h.totalAccountedBytes > hubTarget {
		victim, oldestTime, ok := h.oldestSeriesDeviceLocked()
		if !ok {
			break
		}
		before := h.deviceAccountedBytesLocked(victim)
		evictedCutoff, evicted := h.evictOldestStepLocked(victim)
		if !evicted {
			break
		}
		h.hubEvictedByCap = true
		if evictedCutoff.After(h.hubEvictedCutoff) {
			h.hubEvictedCutoff = evictedCutoff
		}
		if oldestTime.After(h.hubEvictedCutoff) {
			h.hubEvictedCutoff = oldestTime
		}
		victim.dirty = true
		_ = h.deviceExactShardBytesLocked(victim)
		h.totalAccountedBytes += h.deviceAccountedBytesLocked(victim) - before
	}
}

func (h *ManagedHistoryStore) evictDeviceToBudgetLocked(dev *historyDevice, limit int64, _ time.Time) {
	// Evict down to a low-watermark target (leaving headroom below limit) and
	// batch oldest-slot drops proportionally to the measured shard bytes per
	// coarse slot so dense devices converge in 1-2 compression passes while
	// strictly honoring the hard byte ceiling and preserving latest and the
	// newest live slot whenever it fits within limit.
	floorBytes := dev.latestBytes + h.deviceKeyOverhead(dev.name) + historyWriteHeadroomBytes
	headroom := min(limit/8, int64(128<<10))
	evictTarget := max(floorBytes, limit-headroom)

	for cur := h.deviceAccountedBytesLocked(dev); cur > evictTarget; cur = h.deviceAccountedBytesLocked(dev) {
		estBytesPerStep, maxCoarseCount := estimateDeviceEvictStepBytes(dev)
		var steps int
		if maxCoarseCount <= 1 {
			// Never drop the final remaining coarse slot merely to create
			// low-watermark headroom; only evict it if the hard cap is exceeded.
			if cur <= limit {
				break
			}
			steps = 1
		} else {
			need := cur - evictTarget
			steps = min(int(max(int64(1), (need+estBytesPerStep-1)/estBytesPerStep)), 128)
			// Never blind-batch past the newest coarse slot before re-measuring.
			steps = min(steps, maxCoarseCount-1)
		}
		var lastCutoff time.Time
		var evictedAny bool
		for range steps {
			cutoff, ok := h.evictOldestStepLocked(dev)
			if !ok {
				break
			}
			evictedAny = true
			if cutoff.After(lastCutoff) {
				lastCutoff = cutoff
			}
		}
		if !evictedAny {
			break
		}
		dev.evictedByCap = true
		if lastCutoff.After(dev.evictedCutoff) {
			dev.evictedCutoff = lastCutoff
		}
		dev.dirty = true
		_ = h.deviceExactShardBytesLocked(dev)
	}
}

func estimateDeviceEvictStepBytes(dev *historyDevice) (estBytes int64, maxCoarseCount int) {
	activeSeries := 0
	for _, ser := range dev.series {
		if ser == nil {
			continue
		}
		if ser.coarse.count > 0 || ser.fine.count > 0 {
			activeSeries++
		}
		if ser.coarse.count > maxCoarseCount {
			maxCoarseCount = ser.coarse.count
		}
	}
	if maxCoarseCount > 0 && dev.persistedShardBytes > 0 {
		perSlot := dev.persistedShardBytes / int64(maxCoarseCount)
		if perSlot >= 128 {
			return perSlot, maxCoarseCount
		}
	}
	return max(int64(128), int64(activeSeries)*150), maxCoarseCount
}

// evictOldestStepLocked removes the single oldest timestamp slot across dev's
// series (dropping coarse slot first together with any fine buckets at or
// before that slot, or dropping the oldest fine minute when only fine buckets
// remain). Never touches dev.latest.
func (h *ManagedHistoryStore) evictOldestStepLocked(dev *historyDevice) (time.Time, bool) {
	var minSlot15 int64
	hasCoarse := false
	for _, ser := range dev.series {
		if ser == nil {
			continue
		}
		if ser.coarse.count == 0 {
			continue
		}
		s := ser.coarse.oldest().Slot15
		if !hasCoarse || s < minSlot15 {
			minSlot15 = s
			hasCoarse = true
		}
	}
	if hasCoarse {
		cutoffMinute := (minSlot15 + 1) * HistoryCoarseSlotMinutes
		for _, ser := range dev.series {
			if ser == nil {
				continue
			}
			if ser.coarse.count > 0 && ser.coarse.oldest().Slot15 == minSlot15 {
				ser.coarse.popOldest()
			}
			ser.fine.evictBefore(cutoffMinute)
		}
		dev.mutationSeq++
		return time.Unix(cutoffMinute*60, 0).UTC(), true
	}

	var minMinute int64
	hasFine := false
	for _, ser := range dev.series {
		if ser == nil {
			continue
		}
		if ser.fine.count == 0 {
			continue
		}
		m := ser.fine.oldest().Minute
		if !hasFine || m < minMinute {
			minMinute = m
			hasFine = true
		}
	}
	if !hasFine {
		return time.Time{}, false
	}
	for _, ser := range dev.series {
		if ser == nil {
			continue
		}
		if ser.fine.count > 0 && ser.fine.oldest().Minute == minMinute {
			ser.fine.popOldest()
		}
	}
	dev.mutationSeq++
	return time.Unix((minMinute+1)*60, 0).UTC(), true
}

func (h *ManagedHistoryStore) oldestSeriesDeviceLocked() (*historyDevice, time.Time, bool) {
	var victim *historyDevice
	var oldest time.Time
	for _, dev := range h.devices {
		t, ok := dev.oldestSeriesTime()
		if !ok {
			continue
		}
		if victim == nil || t.Before(oldest) || (t.Equal(oldest) && dev.name < victim.name) {
			victim = dev
			oldest = t
		}
	}
	return victim, oldest, victim != nil
}

func (d *historyDevice) oldestSeriesTime() (time.Time, bool) {
	var oldestUnix int64
	found := false
	for _, ser := range d.series {
		if ser == nil {
			continue
		}
		if ser.coarse.count > 0 {
			u := ser.coarse.oldest().Slot15 * HistoryCoarseSlotMinutes * 60
			if !found || u < oldestUnix {
				oldestUnix = u
				found = true
			}
		}
		if ser.fine.count > 0 {
			u := ser.fine.oldest().Minute * 60
			if !found || u < oldestUnix {
				oldestUnix = u
				found = true
			}
		}
	}
	if !found {
		return time.Time{}, false
	}
	return time.Unix(oldestUnix, 0).UTC(), true
}

func (d *historyDevice) newestSeriesTime() (time.Time, bool) {
	var newestUnix int64
	found := false
	for _, ser := range d.series {
		if ser == nil {
			continue
		}
		if ser.fine.count > 0 {
			u := ser.fine.newest().Minute * 60
			if !found || u > newestUnix {
				newestUnix = u
				found = true
			}
		}
		if ser.coarse.count > 0 {
			u := ser.coarse.newest().Slot15 * HistoryCoarseSlotMinutes * 60
			if !found || u > newestUnix {
				newestUnix = u
				found = true
			}
		}
	}
	if !found {
		if d.latest.hasReport && !d.latest.receivedAt.IsZero() {
			return d.latest.receivedAt, true
		}
		return time.Time{}, false
	}
	return time.Unix(newestUnix, 0).UTC(), true
}

func (h *ManagedHistoryStore) hasCapacityPressureLocked(now time.Time) bool {
	horizonStart := now.Add(-h.configuredRetentionLocked())
	if h.hubEvictedByCap && h.hubEvictedCutoff.After(horizonStart) {
		return true
	}
	for _, dev := range h.devices {
		if dev.evictedByCap && dev.evictedCutoff.After(horizonStart) {
			return true
		}
	}
	return false
}

func (h *ManagedHistoryStore) deviceEffectiveRetentionLocked(dev *historyDevice, now time.Time) (time.Duration, bool) {
	configured := h.configuredRetentionLocked()
	horizonStart := now.Add(-configured)
	var cutoff time.Time
	pressured := false
	if h.hubEvictedByCap && h.hubEvictedCutoff.After(horizonStart) {
		cutoff = h.hubEvictedCutoff
		pressured = true
	}
	if dev.evictedByCap && dev.evictedCutoff.After(horizonStart) {
		if dev.evictedCutoff.After(cutoff) {
			cutoff = dev.evictedCutoff
		}
		pressured = true
	}
	if !pressured {
		return configured, false
	}
	eff := max(now.Sub(cutoff), 0)
	eff = min(eff, configured)
	return eff, true
}

func (h *ManagedHistoryStore) effectiveRetentionLocked(now time.Time) time.Duration {
	configured := h.configuredRetentionLocked()
	minEff := configured
	horizonStart := now.Add(-configured)
	if h.hubEvictedByCap && h.hubEvictedCutoff.After(horizonStart) {
		eff := max(time.Duration(0), now.Sub(h.hubEvictedCutoff))
		if eff < minEff {
			minEff = eff
		}
	}
	for _, dev := range h.devices {
		eff, pressured := h.deviceEffectiveRetentionLocked(dev, now)
		if pressured && eff < minEff {
			minEff = eff
		}
	}
	return minEff
}

// -----------------------------------------------------------------------------
// Query Engine: Latest, Reduce, Rank, Series, Storage
// -----------------------------------------------------------------------------

// QueryLatest answers the latest accepted sample for each authorized matching
// device in meta. Devices are filtered through scope BEFORE any row is
// constructed, so absent and denied devices yield the same empty answer.
// Safe for concurrent use.
func (h *ManagedHistoryStore) QueryLatest(scope FleetDeviceScope, meta map[string]HistoryDeviceMeta, filter HistoryLatestFilter, now time.Time) ([]HistoryLatestRow, error) {
	if filter.Figure != "" {
		if _, ok := fleet.LookupSampleFigure(filter.Figure); !ok {
			return nil, unknownSampleFigureError(filter.Figure)
		}
	}
	nowUTC := now.UTC()

	h.mu.Lock()
	defer h.mu.Unlock()

	names := h.matchingAuthorizedDevicesLocked(scope, meta, filter.Device, filter.Labels)
	if len(names) > HistoryQueryMaxRows {
		return nil, historyRowBoundError(len(names))
	}
	rows := make([]HistoryLatestRow, 0, len(names))
	for _, name := range names {
		m := meta[name]
		row := HistoryLatestRow{
			Device:    name,
			Declared:  m.Declared,
			Connected: m.Connected,
			Labels:    maps.Clone(m.Labels),
			Status:    historyStatusNoReports,
		}
		if dev := h.devices[name]; dev != nil {
			switch {
			case dev.latest.hasReport:
				row.HasReport = true
				row.Status = historyStatusOK
				row.ReceivedAt = dev.latest.receivedAt
				row.DeviceTime = dev.latest.deviceTime
				if !dev.latest.receivedAt.IsZero() {
					age := nowUTC.Sub(dev.latest.receivedAt)
					if age > 0 {
						row.AgeSeconds = int64(age.Seconds())
					}
				}
				if filter.Figure != "" {
					if v, ok := dev.latest.values[filter.Figure]; ok {
						row.Figures = map[string]float64{filter.Figure: v}
					}
				} else if len(dev.latest.values) > 0 {
					row.Figures = maps.Clone(dev.latest.values)
				}
			case dev.latest.unsupported:
				row.Unsupported = true
				row.Status = historyStatusUnsupported
				row.ReceivedAt = dev.latest.receivedAt
			}
		}
		rows = append(rows, row)
	}
	if err := checkSerializedQuerySize(rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ReduceLatestRows folds authorized latest observation rows using each figure's
// registered aggregation rule (sum, mean, and paired capacity ratios).
func ReduceLatestRows(rows []HistoryLatestRow, figure string) ([]HistoryLatestReducedRow, error) {
	specs, err := selectedFigureSpecs(figure)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryLatestReducedRow, 0, len(specs))
	for _, spec := range specs {
		var sum float64
		var contributors int
		var pairedNum float64
		var pairedDen float64
		var pairedCount int
		for i := range rows {
			if !rows[i].HasReport {
				continue
			}
			val, ok := rows[i].Figures[spec.Name]
			if !ok {
				continue
			}
			sum += val
			contributors++
			if spec.Denominator != "" {
				if den, hasDen := rows[i].Figures[spec.Denominator]; hasDen {
					pairedNum += val
					pairedDen += den
					pairedCount++
				}
			}
		}
		row := HistoryLatestReducedRow{
			Figure:       spec.Name,
			Unit:         spec.Unit,
			Aggregation:  spec.Agg.String(),
			Contributors: contributors,
			Missing:      len(rows) - contributors,
		}
		if contributors > 0 {
			var folded float64
			switch spec.Agg {
			case fleet.SampleAggSum:
				folded = sum
			case fleet.SampleAggMean:
				folded = sum / float64(contributors)
			case fleet.SampleAggRatio:
				if r, ok := fleet.SampleRatioValue(pairedNum, pairedDen); ok {
					folded = r
				}
			case fleet.SampleAggUnspecified:
				folded = sum
			default:
				// Open enum from pkg/fleet: default to sum.
				folded = sum
			}
			row.Value = &folded
		}
		if spec.Denominator != "" && pairedCount > 0 {
			if r, ok := fleet.SampleRatioValue(pairedNum, pairedDen); ok {
				row.Ratio = &r
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// RankLatestRows drops rows lacking a measurement for figure and orders the
// remaining rows descending by figure value, breaking ties ascending by device
// name.
func RankLatestRows(rows []HistoryLatestRow, figure string) ([]HistoryLatestRow, error) {
	if _, ok := fleet.LookupSampleFigure(figure); !ok {
		return nil, unknownSampleFigureError(figure)
	}
	measured := make([]HistoryLatestRow, 0, len(rows))
	for i := range rows {
		if !rows[i].HasReport {
			continue
		}
		if _, ok := rows[i].Figures[figure]; !ok {
			continue
		}
		measured = append(measured, rows[i])
	}
	slices.SortFunc(measured, func(a, b HistoryLatestRow) int {
		va := a.Figures[figure]
		vb := b.Figures[figure]
		if c := cmp.Compare(vb, va); c != 0 {
			return c
		}
		return cmp.Compare(a.Device, b.Device)
	})
	return measured, nil
}

// QuerySeries returns dual-resolution time series rows for one device or a
// reduced set across authorized devices, counting every period once across the
// 1-day fine/coarse boundary, computing exact aligned set maxima from packed
// per-minute contributions, preserving offline gaps, and enforcing row and
// serialized byte output bounds. Safe for concurrent use.
func (h *ManagedHistoryStore) QuerySeries(scope FleetDeviceScope, meta map[string]HistoryDeviceMeta, query HistorySeriesQuery, now time.Time) ([]HistorySeriesRow, error) {
	if query.Figure != "" {
		if _, ok := fleet.LookupSampleFigure(query.Figure); !ok {
			return nil, unknownSampleFigureError(query.Figure)
		}
	}
	if query.Points < 0 {
		return nil, opererr.New("invalid points count", "points must be between 1 and 2112", "pass | points with a value from 1 to 2112")
	}
	if query.Points > HistorySeriesMaxPoints {
		return nil, opererr.New("points count exceeds maximum 2112", "points cannot exceed the 2,112 native buckets in one week", "pass | points 2112 or smaller")
	}

	nowUTC := now.UTC()
	until := query.Until.UTC()
	if query.Until.IsZero() {
		until = nowUTC
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	from := query.From.UTC()
	if query.From.IsZero() {
		from = nowUTC.Add(-h.configuredRetentionLocked())
	}
	if !from.Before(until) {
		return nil, opererr.New("empty or reversed time range", "the series start time (from) must be strictly before the end time (until)", "set | from earlier than | until in RFC 3339 UTC format")
	}

	names := h.matchingAuthorizedDevicesLocked(scope, meta, query.Device, query.Labels)
	if len(names) == 0 {
		return []HistorySeriesRow{}, nil
	}

	specs, err := h.seriesTargetFiguresLocked(names, query.Figure)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return []HistorySeriesRow{}, nil
	}

	// Clip [from, until) to the supported retained horizon before allocating
	// any UTC slot grid so an unbounded caller range (e.g. year 0001..9999)
	// cannot exhaust memory. Allow up to the end of the current UTC minute so a
	// query including the just-ingested minute bucket stays intact.
	horizonStart := nowUTC.Add(-h.configuredRetentionLocked())
	horizonEnd := time.Unix(((nowUTC.Unix()/60)+1)*60, 0).UTC()
	clippedFrom := from
	if clippedFrom.Before(horizonStart) {
		clippedFrom = horizonStart
	}
	clippedUntil := until
	if clippedUntil.After(horizonEnd) {
		clippedUntil = horizonEnd
	}
	if !clippedFrom.Before(clippedUntil) {
		return []HistorySeriesRow{}, nil
	}

	// Partition [clippedFrom, clippedUntil) at a 15-minute UTC boundary >= (now - 24h)
	// so coarse slots [slot*900, (slot+1)*900) and fine slots [min*60, (min+1)*60)
	// never overlap and never leave a hole at the 1-day seam.
	boundaryUnix := ceilDivInt64(nowUTC.Unix()-int64(HistoryFineHorizonMinutes)*60, HistoryCoarseSlotMinutes*60) * HistoryCoarseSlotMinutes * 60
	boundary := time.Unix(boundaryUnix, 0).UTC()

	coarseSlots, fineMinutes := buildSeriesTimeGrid(clippedFrom, clippedUntil, boundary)
	nativeBucketCount := len(coarseSlots) + len(fineMinutes)
	if nativeBucketCount == 0 {
		return []HistorySeriesRow{}, nil
	}
	effectivePoints := nativeBucketCount
	if query.Points > 0 && query.Points < effectivePoints {
		effectivePoints = query.Points
	}

	// Pre-check row bound before allocating full output.
	seriesCount := len(specs)
	if !query.Reduce {
		seriesCount *= len(names)
	}
	if seriesCount > 0 && effectivePoints > HistoryQueryMaxRows/seriesCount {
		return nil, historyRowBoundError(seriesCount * effectivePoints)
	}

	if query.Reduce {
		rows := make([]HistorySeriesRow, 0, seriesCount*effectivePoints)
		for _, spec := range specs {
			buckets := h.buildReducedBucketsLocked(names, &spec, coarseSlots, fineMinutes, clippedFrom, clippedUntil)
			if query.Points > 0 && len(buckets) > query.Points {
				buckets = rebucketSeriesAccum(buckets, query.Points, &spec, true)
			}
			for i := range buckets {
				rows = append(rows, accumToSeriesRow("", nil, &spec, &buckets[i], true))
			}
		}
		if len(rows) > HistoryQueryMaxRows {
			return nil, historyRowBoundError(len(rows))
		}
		if err := checkSerializedQuerySize(rows); err != nil {
			return nil, err
		}
		return rows, nil
	}

	rows := make([]HistorySeriesRow, 0, seriesCount*effectivePoints)
	for _, name := range names {
		labels := maps.Clone(meta[name].Labels)
		dev := h.devices[name]
		for _, spec := range specs {
			buckets := h.buildDeviceBucketsLocked(dev, &spec, coarseSlots, fineMinutes, clippedFrom, clippedUntil)
			if query.Points > 0 && len(buckets) > query.Points {
				buckets = rebucketSeriesAccum(buckets, query.Points, &spec, false)
			}
			for i := range buckets {
				rows = append(rows, accumToSeriesRow(name, labels, &spec, &buckets[i], false))
			}
		}
	}
	if len(rows) > HistoryQueryMaxRows {
		return nil, historyRowBoundError(len(rows))
	}
	if err := checkSerializedQuerySize(rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// QueryStorage returns hub-wide history capacity and checkpoint status plus
// per-device usage rows filtered strictly through scope. Safe for concurrent
// use.
func (h *ManagedHistoryStore) QueryStorage(scope FleetDeviceScope, meta map[string]HistoryDeviceMeta, now time.Time) (HistoryStorageStatus, error) {
	nowUTC := now.UTC()
	h.mu.Lock()
	defer h.mu.Unlock()

	status := HistoryStorageStatus{
		IntervalSeconds:           h.cfg.IntervalSeconds,
		ConfiguredRetentionDays:   h.cfg.RetentionDays,
		EffectiveRetentionSeconds: int64(h.effectiveRetentionLocked(nowUTC).Seconds()),
		UsedBytes:                 h.totalAccountedBytes,
		HubBudgetBytes:            h.cfg.HubBudgetBytes,
		DeviceBudgetBytes:         h.cfg.DeviceBudgetBytes,
		CapacityPressure:          h.hasCapacityPressureLocked(nowUTC),
		DeviceCount:               len(h.devices),
		LastCheckpointAt:          h.lastCheckpointOKAt,
		LastCheckpointError:       h.lastCheckpointErr,
		CheckpointFailed:          h.checkpointFailed,
		CorruptShards:             len(h.corruptRecords),
		RejectedReports:           h.rejectedReports,
		RateLimitedReports:        h.rateLimitedReports,
		Devices:                   []HistoryStorageDeviceUsage{},
	}
	if !h.lastCheckpointOKAt.IsZero() {
		age := nowUTC.Sub(h.lastCheckpointOKAt)
		if age > 0 {
			status.LastCheckpointAgeSeconds = int64(age.Seconds())
		}
	}

	for name, dev := range h.devices {
		oldest, hasOldest := dev.oldestSeriesTime()
		if !hasOldest && dev.latest.hasReport {
			oldest = dev.latest.receivedAt
			hasOldest = !oldest.IsZero()
		}
		if hasOldest && (status.OldestSample.IsZero() || oldest.Before(status.OldestSample)) {
			status.OldestSample = oldest
		}
		if !scope.AllowsDevice(name) {
			continue
		}
		newest, _ := dev.newestSeriesTime()
		eff, pressured := h.deviceEffectiveRetentionLocked(dev, nowUTC)
		status.Devices = append(status.Devices, HistoryStorageDeviceUsage{
			Device:                    name,
			UsedBytes:                 h.deviceAccountedBytesLocked(dev),
			LatestBytes:               dev.latestBytes,
			ShardBytes:                dev.persistedShardBytes + dev.unflushedRawDelta,
			OldestSample:              oldest,
			NewestSample:              newest,
			CapacityPressure:          pressured,
			EffectiveRetentionSeconds: int64(eff.Seconds()),
		})
	}
	// Also include authorized declared devices with 0 usage only if they have history;
	// keep Devices sorted descending by UsedBytes then ascending by Device name.
	_ = meta
	slices.SortFunc(status.Devices, func(a, b HistoryStorageDeviceUsage) int {
		if c := cmp.Compare(b.UsedBytes, a.UsedBytes); c != 0 {
			return c
		}
		return cmp.Compare(a.Device, b.Device)
	})
	return status, nil
}

func (h *ManagedHistoryStore) matchingAuthorizedDevicesLocked(scope FleetDeviceScope, meta map[string]HistoryDeviceMeta, device string, labels map[string]string) []string {
	candidates := make(map[string]struct{}, len(meta)+len(h.devices))
	for name := range meta {
		candidates[name] = struct{}{}
	}
	for name := range h.devices {
		candidates[name] = struct{}{}
	}
	out := make([]string, 0, len(candidates))
	for name := range candidates {
		if !scope.AllowsDevice(name) {
			continue
		}
		if device != "" && name != device {
			continue
		}
		if len(labels) > 0 {
			devLabels := meta[name].Labels
			matched := true
			for k, v := range labels {
				if devLabels[k] != v {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func (h *ManagedHistoryStore) seriesTargetFiguresLocked(names []string, figure string) ([]fleet.SampleFigureSpec, error) {
	if figure != "" {
		return selectedFigureSpecs(figure)
	}
	present := make([]bool, len(historyFigureSpecs))
	for _, name := range names {
		dev := h.devices[name]
		if dev == nil {
			continue
		}
		for i, ser := range dev.series {
			if ser != nil && (ser.fine.count > 0 || ser.coarse.count > 0) {
				present[i] = true
			}
		}
	}
	out := make([]fleet.SampleFigureSpec, 0, len(historyFigureSpecs))
	for i := range historyFigureSpecs {
		if present[i] {
			out = append(out, historyFigureSpecs[i])
		}
	}
	return out, nil
}

func selectedFigureSpecs(figure string) ([]fleet.SampleFigureSpec, error) {
	if figure == "" {
		return fleet.AllSampleFigures(), nil
	}
	spec, ok := fleet.LookupSampleFigure(figure)
	if !ok {
		return nil, unknownSampleFigureError(figure)
	}
	return []fleet.SampleFigureSpec{spec}, nil
}

func unknownSampleFigureError(figure string) *opererr.Error {
	var tb textbuf.Buffer
	what := tb.Reset().Str("unknown fleet sample figure ").Quoted(figure).String()
	return opererr.New(what,
		"the figure name is not in the fixed fleet sample vocabulary",
		"use a registered figure name such as cpu-used, memory-used, traffic-in, bgp-peers-established, or routes-held")
}

func historyRowBoundError(rows int) *opererr.Error {
	var tb textbuf.Buffer
	what := tb.Reset().Str("fleet sample query would return ").Int(int64(rows)).Str(" rows, exceeding the ").Int(HistoryQueryMaxRows).Str("-row output bound").String()
	return opererr.New(what,
		"queries are bounded to 4,096 rows and 4 MiB of serialized output",
		historyRefineFix)
}

func checkSerializedQuerySize(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("fleet history: serialize query answer: %w", err)
	}
	if len(raw) > HistoryQueryMaxBytes {
		var tb textbuf.Buffer
		what := tb.Reset().Str("fleet sample query output is ").Int(int64(len(raw))).Str(" bytes, exceeding the 4 MiB output bound").String()
		return opererr.New(what,
			"queries are bounded to 4,096 rows and 4 MiB of serialized output",
			historyRefineFix)
	}
	return nil
}

func buildSeriesTimeGrid(from, until, boundary time.Time) (coarseSlots, fineMinutes []int64) {
	coarseEnd := until
	if boundary.Before(coarseEnd) {
		coarseEnd = boundary
	}
	if from.Before(coarseEnd) {
		startSlot := floorDivInt64(from.Unix(), HistoryCoarseSlotMinutes*60)
		endSlot := ceilDivInt64(coarseEnd.Unix(), HistoryCoarseSlotMinutes*60)
		if endSlot > startSlot {
			coarseSlots = make([]int64, 0, endSlot-startSlot)
			for s := startSlot; s < endSlot; s++ {
				coarseSlots = append(coarseSlots, s)
			}
		}
	}

	fineStart := from
	if boundary.After(fineStart) {
		fineStart = boundary
	}
	if fineStart.Before(until) {
		startMin := floorDivInt64(fineStart.Unix(), 60)
		endMin := ceilDivInt64(until.Unix(), 60)
		if endMin > startMin {
			fineMinutes = make([]int64, 0, endMin-startMin)
			for m := startMin; m < endMin; m++ {
				fineMinutes = append(fineMinutes, m)
			}
		}
	}
	return coarseSlots, fineMinutes
}

func clipTimeRange(slotStart, slotEnd, from, until time.Time) (clipStart, clipEnd time.Time) {
	clipStart = slotStart
	if from.After(clipStart) {
		clipStart = from
	}
	clipEnd = slotEnd
	if until.Before(clipEnd) {
		clipEnd = until
	}
	return clipStart, clipEnd
}

func clipMinuteRange(slotStart, slotEnd, from, until time.Time) (startMin, endMin int64, clipStart, clipEnd time.Time) {
	clipStart, clipEnd = clipTimeRange(slotStart, slotEnd, from, until)
	startMin = floorDivInt64(clipStart.Unix(), 60)
	endMin = ceilDivInt64(clipEnd.Unix(), 60)
	return startMin, endMin, clipStart, clipEnd
}

func (h *ManagedHistoryStore) buildDeviceBucketsLocked(dev *historyDevice, spec *fleet.SampleFigureSpec, coarseSlots, fineMinutes []int64, from, until time.Time) []historySeriesBucketAccum {
	out := make([]historySeriesBucketAccum, 0, len(coarseSlots)+len(fineMinutes))
	figIdx := historyFigureIdx[spec.Name]
	denIdx := -1
	if spec.Denominator != "" {
		if idx, ok := historyFigureIdx[spec.Denominator]; ok {
			denIdx = idx
		}
	}
	var ser *historySeries
	var denSer *historySeries
	if dev != nil {
		ser = dev.series[figIdx]
		if denIdx >= 0 {
			denSer = dev.series[denIdx]
		}
	}

	for _, slot15 := range coarseSlots {
		slotStart := time.Unix(slot15*HistoryCoarseSlotMinutes*60, 0).UTC()
		slotEnd := slotStart.Add(HistoryCoarseSlotMinutes * time.Minute)
		minStart, minEnd, clipStart, clipEnd := clipMinuteRange(slotStart, slotEnd, from, until)
		totalMins := uint32(max(int64(1), minEnd-minStart))
		acc := historySeriesBucketAccum{
			start:        clipStart,
			end:          clipEnd,
			resolution:   historyResolutionCoarse,
			totalMinutes: totalMins,
			missing:      1,
		}
		if ser != nil {
			if sum, maxVal, cnt, obsMask, ok := ser.coarse.lookupClipped(slot15, minStart, minEnd); ok {
				acc.sum = sum
				acc.maxVal = maxVal
				acc.hasVal = true
				acc.value = sum / float64(cnt)
				acc.count = cnt
				acc.obsMinutes = uint32(bits.OnesCount16(obsMask))
				acc.contributors = 1
				acc.missing = 0
				if denSer != nil {
					if dSum, _, dCnt, _, hasDen := denSer.coarse.lookupClipped(slot15, minStart, minEnd); hasDen {
						acc.pairedNumSum = acc.value
						acc.pairedDenSum = dSum / float64(dCnt)
						acc.hasPaired = true
					}
				}
			}
		}
		out = append(out, acc)
	}

	for _, minute := range fineMinutes {
		slotStart := time.Unix(minute*60, 0).UTC()
		slotEnd := slotStart.Add(time.Minute)
		clipStart, clipEnd := clipTimeRange(slotStart, slotEnd, from, until)
		acc := historySeriesBucketAccum{
			start:        clipStart,
			end:          clipEnd,
			resolution:   historyResolutionFine,
			totalMinutes: 1,
			missing:      1,
		}
		if ser != nil {
			if b, ok := ser.fine.lookup(minute); ok {
				acc.sum = b.Sum
				acc.maxVal = b.Max
				acc.hasVal = true
				acc.value = b.Sum / float64(b.Count)
				acc.count = uint32(b.Count)
				acc.obsMinutes = 1
				acc.contributors = 1
				acc.missing = 0
				if denSer != nil {
					if db, hasDen := denSer.fine.lookup(minute); hasDen {
						acc.pairedNumSum = acc.value
						acc.pairedDenSum = db.Sum / float64(db.Count)
						acc.hasPaired = true
					}
				}
			}
		}
		out = append(out, acc)
	}
	return out
}

func (h *ManagedHistoryStore) buildReducedBucketsLocked(names []string, spec *fleet.SampleFigureSpec, coarseSlots, fineMinutes []int64, from, until time.Time) []historySeriesBucketAccum {
	out := make([]historySeriesBucketAccum, 0, len(coarseSlots)+len(fineMinutes))
	figIdx := historyFigureIdx[spec.Name]
	denIdx := -1
	if spec.Denominator != "" {
		if idx, ok := historyFigureIdx[spec.Denominator]; ok {
			denIdx = idx
		}
	}

	// Pre-resolve device series pointers once per query.
	seriesList := make([]*historySeries, len(names))
	denList := make([]*historySeries, len(names))
	for i, name := range names {
		dev := h.devices[name]
		if dev == nil {
			continue
		}
		seriesList[i] = dev.series[figIdx]
		if denIdx >= 0 {
			denList[i] = dev.series[denIdx]
		}
	}

	for _, slot15 := range coarseSlots {
		slotStart := time.Unix(slot15*HistoryCoarseSlotMinutes*60, 0).UTC()
		slotEnd := slotStart.Add(HistoryCoarseSlotMinutes * time.Minute)
		minStart, minEnd, clipStart, clipEnd := clipMinuteRange(slotStart, slotEnd, from, until)
		totalMins := uint32(max(int64(1), minEnd-minStart))

		var minSums [HistoryCoarseSlotMinutes]float64
		var minCounts [HistoryCoarseSlotMinutes]uint32
		var minMeanSums [HistoryCoarseSlotMinutes]float64
		var unionMask uint16

		acc := historySeriesBucketAccum{
			start:        clipStart,
			end:          clipEnd,
			resolution:   historyResolutionCoarse,
			totalMinutes: totalMins,
			missing:      len(names),
		}
		var singleMax float64
		for devIdx, ser := range seriesList {
			if ser == nil {
				continue
			}
			b, valStart, ok := ser.coarse.lookupRaw(slot15)
			if !ok {
				continue
			}
			var devSum float64
			var devCount uint32
			var devMask uint16
			baseMin := slot15 * HistoryCoarseSlotMinutes
			packedIdx := 0
			for off := range HistoryCoarseSlotMinutes {
				if b.MinuteMask&(1<<off) == 0 {
					continue
				}
				mSum, mCnt := ser.coarse.minuteAt(valStart, packedIdx)
				packedIdx++
				absMin := baseMin + int64(off)
				if absMin < minStart {
					continue
				}
				if absMin >= minEnd {
					continue
				}
				devMask |= 1 << off
				devSum += mSum
				devCount += uint32(mCnt)
				minSums[off] += mSum
				minCounts[off] += uint32(mCnt)
				minMeanSums[off] += mSum / float64(mCnt)
			}
			if devCount == 0 {
				continue
			}
			if acc.contribMask == nil {
				acc.contribMask = make(map[int]struct{})
			}
			acc.contribMask[devIdx] = struct{}{}
			acc.contributors++
			acc.sum += devSum
			acc.count += devCount
			unionMask |= devMask
			if acc.contributors == 1 {
				if devMask == b.MinuteMask {
					singleMax = b.Max
				} else {
					singleMax = devSum / float64(devCount)
				}
			}
			if denList[devIdx] != nil {
				if dSum, _, dCnt, _, hasDen := denList[devIdx].coarse.lookupClipped(slot15, minStart, minEnd); hasDen {
					acc.pairedNumSum += devSum / float64(devCount)
					acc.pairedDenSum += dSum / float64(dCnt)
					acc.hasPaired = true
				}
			}
		}

		acc.missing = len(names) - acc.contributors
		if acc.contributors > 0 {
			acc.hasVal = true
			acc.obsMinutes = uint32(bits.OnesCount16(unionMask))

			var alignedPeak float64
			hasPeak := false
			var estateSumOverMins float64
			for off := range HistoryCoarseSlotMinutes {
				if unionMask&(1<<off) == 0 {
					continue
				}
				var estateAtMin float64
				switch spec.Agg {
				case fleet.SampleAggSum:
					estateAtMin = minMeanSums[off]
				case fleet.SampleAggMean:
					estateAtMin = minSums[off] / float64(minCounts[off])
				case fleet.SampleAggRatio, fleet.SampleAggUnspecified:
					estateAtMin = minMeanSums[off]
				default:
					// Open enum from pkg/fleet.
					estateAtMin = minMeanSums[off]
				}
				estateSumOverMins += estateAtMin
				if !hasPeak || estateAtMin > alignedPeak {
					alignedPeak = estateAtMin
					hasPeak = true
				}
			}

			switch spec.Agg {
			case fleet.SampleAggSum:
				acc.value = estateSumOverMins / float64(acc.obsMinutes)
			case fleet.SampleAggMean:
				acc.value = acc.sum / float64(acc.count)
			case fleet.SampleAggRatio:
				if r, ok := fleet.SampleRatioValue(acc.pairedNumSum, acc.pairedDenSum); ok {
					acc.value = r
				}
			case fleet.SampleAggUnspecified:
				acc.value = estateSumOverMins / float64(acc.obsMinutes)
			default:
				// Open enum from pkg/fleet.
				acc.value = estateSumOverMins / float64(acc.obsMinutes)
			}

			if acc.contributors == 1 && singleMax > alignedPeak {
				acc.maxVal = singleMax
			} else {
				acc.maxVal = alignedPeak
			}
		}
		out = append(out, acc)
	}

	for _, minute := range fineMinutes {
		slotStart := time.Unix(minute*60, 0).UTC()
		slotEnd := slotStart.Add(time.Minute)
		clipStart, clipEnd := clipTimeRange(slotStart, slotEnd, from, until)
		acc := historySeriesBucketAccum{
			start:        clipStart,
			end:          clipEnd,
			resolution:   historyResolutionFine,
			totalMinutes: 1,
			missing:      len(names),
		}
		var sumOfDeviceMeans float64
		var singleMax float64
		for devIdx, ser := range seriesList {
			if ser == nil {
				continue
			}
			b, ok := ser.fine.lookup(minute)
			if !ok {
				continue
			}
			if acc.contribMask == nil {
				acc.contribMask = make(map[int]struct{})
			}
			acc.contribMask[devIdx] = struct{}{}
			acc.contributors++
			acc.sum += b.Sum
			acc.count += uint32(b.Count)
			devMean := b.Sum / float64(b.Count)
			sumOfDeviceMeans += devMean
			if acc.contributors == 1 {
				singleMax = b.Max
			}
			if denList[devIdx] != nil {
				if db, hasDen := denList[devIdx].fine.lookup(minute); hasDen {
					acc.pairedNumSum += devMean
					acc.pairedDenSum += db.Sum / float64(db.Count)
					acc.hasPaired = true
				}
			}
		}
		acc.missing = len(names) - acc.contributors
		if acc.contributors > 0 {
			acc.hasVal = true
			acc.obsMinutes = 1
			switch spec.Agg {
			case fleet.SampleAggSum:
				acc.value = sumOfDeviceMeans
			case fleet.SampleAggMean:
				acc.value = acc.sum / float64(acc.count)
			case fleet.SampleAggRatio:
				if r, ok := fleet.SampleRatioValue(acc.pairedNumSum, acc.pairedDenSum); ok {
					acc.value = r
				}
			case fleet.SampleAggUnspecified:
				acc.value = sumOfDeviceMeans
			default:
				// Open enum from pkg/fleet.
				acc.value = sumOfDeviceMeans
			}
			if acc.contributors == 1 {
				acc.maxVal = singleMax
			} else {
				acc.maxVal = acc.value
			}
		}
		out = append(out, acc)
	}

	return out
}

func rebucketSeriesAccum(in []historySeriesBucketAccum, points int, spec *fleet.SampleFigureSpec, reduced bool) []historySeriesBucketAccum {
	n := len(in)
	if points <= 0 {
		return in
	}
	if n <= points {
		return in
	}
	out := make([]historySeriesBucketAccum, 0, points)
	for g := range points {
		lo := (g * n) / points
		hi := ((g + 1) * n) / points
		if lo >= hi {
			continue
		}
		first := in[lo]
		last := in[hi-1]
		merged := historySeriesBucketAccum{
			start:      first.start,
			end:        last.end,
			resolution: first.resolution,
		}
		var weightedSum float64
		var totalSelected int
		var unionContrib map[int]struct{}
		for i := lo; i < hi; i++ {
			b := &in[i]
			if b.resolution != merged.resolution {
				merged.resolution = historyResolutionCombined
			}
			merged.totalMinutes += b.totalMinutes
			if b.contributors+b.missing > totalSelected {
				totalSelected = b.contributors + b.missing
			}
			if !b.hasVal {
				continue
			}
			merged.obsMinutes += b.obsMinutes
			merged.sum += b.sum
			merged.count += b.count
			if !merged.hasVal || b.maxVal > merged.maxVal {
				merged.maxVal = b.maxVal
			}
			merged.hasVal = true
			weightedSum += b.value * float64(b.obsMinutes)
			if b.hasPaired {
				merged.pairedNumSum += b.pairedNumSum * float64(b.obsMinutes)
				merged.pairedDenSum += b.pairedDenSum * float64(b.obsMinutes)
				merged.hasPaired = true
			}
			if len(b.contribMask) > 0 {
				if unionContrib == nil {
					unionContrib = make(map[int]struct{}, len(b.contribMask))
				}
				for idx := range b.contribMask {
					unionContrib[idx] = struct{}{}
				}
			} else if b.contributors > merged.contributors {
				merged.contributors = b.contributors
			}
		}
		if len(unionContrib) > 0 {
			merged.contributors = len(unionContrib)
		}
		merged.missing = max(0, totalSelected-merged.contributors)
		if merged.hasVal {
			if reduced {
				switch spec.Agg {
				case fleet.SampleAggMean:
					merged.value = merged.sum / float64(merged.count)
				case fleet.SampleAggRatio:
					if r, ok := fleet.SampleRatioValue(merged.pairedNumSum, merged.pairedDenSum); ok {
						merged.value = r
					}
				case fleet.SampleAggSum, fleet.SampleAggUnspecified:
					if merged.obsMinutes > 0 {
						merged.value = weightedSum / float64(merged.obsMinutes)
					}
				default:
					// Open enum from pkg/fleet.
					if merged.obsMinutes > 0 {
						merged.value = weightedSum / float64(merged.obsMinutes)
					}
				}
			} else {
				merged.value = merged.sum / float64(merged.count)
			}
		}
		out = append(out, merged)
	}
	return out
}

func accumToSeriesRow(device string, labels map[string]string, spec *fleet.SampleFigureSpec, acc *historySeriesBucketAccum, reduced bool) HistorySeriesRow {
	var coverage float64
	if acc.totalMinutes > 0 {
		coverage = float64(acc.obsMinutes) / float64(acc.totalMinutes)
	}
	row := HistorySeriesRow{
		Device:          device,
		Labels:          labels,
		Figure:          spec.Name,
		Unit:            spec.Unit,
		Resolution:      acc.resolution,
		Start:           acc.start,
		End:             acc.end,
		Sum:             acc.sum,
		Count:           acc.count,
		ObservedMinutes: acc.obsMinutes,
		TotalMinutes:    acc.totalMinutes,
		Coverage:        coverage,
		Gap:             !acc.hasVal,
		Contributors:    acc.contributors,
		Missing:         acc.missing,
	}
	_ = reduced
	if acc.hasVal {
		v := acc.value
		m := acc.maxVal
		row.Value = &v
		row.Max = &m
		if acc.hasPaired {
			if r, ok := fleet.SampleRatioValue(acc.pairedNumSum, acc.pairedDenSum); ok {
				row.Ratio = &r
			}
		}
	}
	return row
}

// -----------------------------------------------------------------------------
// Fine and Coarse Ring Operations
// -----------------------------------------------------------------------------

func (r *fineRing) oldest() fineBucket {
	return r.buf[r.head]
}

func (r *fineRing) newest() fineBucket {
	return r.buf[(r.head+r.count-1)%len(r.buf)]
}

func (r *fineRing) popOldest() {
	if r.count == 0 {
		return
	}
	r.head = (r.head + 1) % len(r.buf)
	r.count--
}

func (r *fineRing) evictBefore(minMinute int64) bool {
	evicted := false
	for r.count > 0 && r.buf[r.head].Minute < minMinute {
		r.popOldest()
		evicted = true
	}
	return evicted
}

// push appends or folds (minute, val) in strictly non-decreasing minute order
// and reports whether net bucket count increased (false when folding into the
// tail minute or replacing an evicted bucket at capacity).
func (r *fineRing) push(minute int64, val float64, capMax int) bool {
	if r.count > 0 {
		tailIdx := (r.head + r.count - 1) % len(r.buf)
		tail := &r.buf[tailIdx]
		if tail.Minute == minute {
			tail.Sum += val
			if val > tail.Max {
				tail.Max = val
			}
			if tail.Count < math.MaxUint16 {
				tail.Count++
			}
			return false
		}
	}
	prevCount := r.count
	// Trim buckets older than the rolling horizon.
	r.evictBefore(minute - int64(capMax) + 1)
	if r.count == len(r.buf) {
		if len(r.buf) < capMax {
			r.grow(capMax)
		} else {
			r.popOldest()
		}
	}
	insIdx := (r.head + r.count) % len(r.buf)
	r.buf[insIdx] = fineBucket{
		Minute: minute,
		Sum:    val,
		Max:    val,
		Count:  1,
	}
	r.count++
	return r.count > prevCount
}

func (r *fineRing) grow(capMax int) {
	newCap := 4
	if len(r.buf) > 0 {
		newCap = min(capMax, len(r.buf)*2)
	}
	next := make([]fineBucket, newCap)
	for i := range r.count {
		next[i] = r.buf[(r.head+i)%len(r.buf)]
	}
	r.buf = next
	r.head = 0
}

func (r *fineRing) lookup(minute int64) (fineBucket, bool) {
	if r.count == 0 {
		return fineBucket{}, false
	}
	lo, hi := 0, r.count-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		b := r.buf[(r.head+mid)%len(r.buf)]
		switch {
		case b.Minute == minute:
			return b, true
		case b.Minute < minute:
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}
	return fineBucket{}, false
}

func (r *coarseRing) oldest() coarseBucket {
	return r.buckets[r.head]
}

func (r *coarseRing) newest() coarseBucket {
	return r.buckets[(r.head+r.count-1)%len(r.buckets)]
}

func (r *coarseRing) popOldest() {
	if r.count == 0 {
		return
	}
	old := r.buckets[r.head]
	r.head = (r.head + 1) % len(r.buckets)
	r.count--
	dropVals := min(bits.OnesCount16(old.MinuteMask), r.valCount)
	if len(r.minuteVals) > 0 && dropVals > 0 {
		r.valHead = (r.valHead + dropVals) % len(r.minuteVals)
		r.valCount -= dropVals
	}
}

func (r *coarseRing) evictBefore(minSlot15 int64) bool {
	evicted := false
	for r.count > 0 && r.buckets[r.head].Slot15 < minSlot15 {
		r.popOldest()
		evicted = true
	}
	return evicted
}

// push appends or folds (slot15, minuteOffset, val) in strictly non-decreasing
// time order and returns the approximate raw serialized byte delta added.
func (r *coarseRing) push(slot15 int64, minuteOffset int, val float64, capMax int) int {
	bit := uint16(1) << minuteOffset
	if r.count > 0 {
		tailIdx := (r.head + r.count - 1) % len(r.buckets)
		tail := &r.buckets[tailIdx]
		if tail.Slot15 == slot15 {
			tail.Sum += val
			if val > tail.Max {
				tail.Max = val
			}
			tail.Count++
			if tail.MinuteMask&bit != 0 {
				// Second or later sample in the same minute (interval < 60s).
				if r.minuteCounts == nil {
					r.materializeMinuteCounts()
				}
				lastValIdx := (r.valHead + r.valCount - 1) % len(r.minuteVals)
				r.minuteVals[lastValIdx] += val
				if r.minuteCounts[lastValIdx] < math.MaxUint16 {
					r.minuteCounts[lastValIdx]++
				}
				return 0
			}
			tail.MinuteMask |= bit
			r.pushMinuteVal(val, 1, capMax*HistoryCoarseSlotMinutes)
			return 8
		}
	}

	prevCount := r.count
	prevValCount := r.valCount
	r.evictBefore(slot15 - int64(capMax) + 1)
	if r.count == len(r.buckets) {
		if len(r.buckets) < capMax {
			r.growBuckets(capMax)
		} else {
			r.popOldest()
		}
	}
	insIdx := (r.head + r.count) % len(r.buckets)
	r.buckets[insIdx] = coarseBucket{
		Slot15:     slot15,
		Sum:        val,
		Max:        val,
		Count:      1,
		MinuteMask: bit,
	}
	r.count++
	r.pushMinuteVal(val, 1, capMax*HistoryCoarseSlotMinutes)
	netBytes := 0
	if r.count > prevCount {
		netBytes += (r.count - prevCount) * 31
	}
	if r.valCount > prevValCount {
		netBytes += (r.valCount - prevValCount) * 8
	}
	return netBytes
}

func (r *coarseRing) materializeMinuteCounts() {
	r.minuteCounts = make([]uint16, len(r.minuteVals))
	for i := range r.valCount {
		r.minuteCounts[(r.valHead+i)%len(r.minuteVals)] = 1
	}
}

func (r *coarseRing) pushMinuteVal(val float64, cnt uint16, maxVals int) {
	if r.valCount == len(r.minuteVals) {
		newCap := 8
		if len(r.minuteVals) > 0 {
			newCap = min(max(maxVals, r.valCount+1), len(r.minuteVals)*2)
		}
		nextVals := make([]float64, newCap)
		for i := range r.valCount {
			nextVals[i] = r.minuteVals[(r.valHead+i)%len(r.minuteVals)]
		}
		r.minuteVals = nextVals
		if r.minuteCounts != nil {
			nextCounts := make([]uint16, newCap)
			for i := range r.valCount {
				nextCounts[i] = r.minuteCounts[(r.valHead+i)%len(r.minuteCounts)]
			}
			r.minuteCounts = nextCounts
		}
		r.valHead = 0
	}
	idx := (r.valHead + r.valCount) % len(r.minuteVals)
	r.minuteVals[idx] = val
	if r.minuteCounts != nil {
		r.minuteCounts[idx] = cnt
	}
	r.valCount++
}

func (r *coarseRing) growBuckets(capMax int) {
	newCap := 4
	if len(r.buckets) > 0 {
		newCap = min(capMax, len(r.buckets)*2)
	}
	next := make([]coarseBucket, newCap)
	for i := range r.count {
		next[i] = r.buckets[(r.head+i)%len(r.buckets)]
	}
	r.buckets = next
	r.head = 0
}

func (r *coarseRing) minuteAt(valStart, packedOffset int) (float64, uint16) {
	idx := (valStart + packedOffset) % len(r.minuteVals)
	if r.minuteCounts != nil {
		return r.minuteVals[idx], r.minuteCounts[idx]
	}
	return r.minuteVals[idx], 1
}

// lookupRaw finds slot15 and returns its coarseBucket along with the starting
// ring index into r.minuteVals for its packed per-minute contributions.
func (r *coarseRing) lookupRaw(slot15 int64) (coarseBucket, int, bool) {
	if r.count == 0 {
		return coarseBucket{}, 0, false
	}
	// Binary search for slot15 first, then compute packed minute offset only on hit.
	lo, hi := 0, r.count-1
	foundPos := -1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		b := r.buckets[(r.head+mid)%len(r.buckets)]
		switch {
		case b.Slot15 == slot15:
			foundPos = mid
			lo = hi + 1
		case b.Slot15 < slot15:
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}
	if foundPos < 0 {
		return coarseBucket{}, 0, false
	}
	skip := 0
	for i := range foundPos {
		skip += bits.OnesCount16(r.buckets[(r.head+i)%len(r.buckets)].MinuteMask)
	}
	return r.buckets[(r.head+foundPos)%len(r.buckets)], (r.valHead + skip) % len(r.minuteVals), true
}

func (r *coarseRing) lookupClipped(slot15, minStart, minEnd int64) (sum, maxVal float64, cnt uint32, mask uint16, ok bool) {
	b, valStart, found := r.lookupRaw(slot15)
	if !found {
		return 0, 0, 0, 0, false
	}
	baseMin := slot15 * HistoryCoarseSlotMinutes
	if minStart <= baseMin && minEnd >= baseMin+HistoryCoarseSlotMinutes {
		return b.Sum, b.Max, b.Count, b.MinuteMask, true
	}
	packedIdx := 0
	hasAny := false
	for off := range HistoryCoarseSlotMinutes {
		if b.MinuteMask&(1<<off) == 0 {
			continue
		}
		mSum, mCnt := r.minuteAt(valStart, packedIdx)
		packedIdx++
		absMin := baseMin + int64(off)
		if absMin < minStart {
			continue
		}
		if absMin >= minEnd {
			continue
		}
		mask |= 1 << off
		sum += mSum
		cnt += uint32(mCnt)
		mMean := mSum / float64(mCnt)
		if !hasAny || mMean > maxVal {
			maxVal = mMean
			hasAny = true
		}
	}
	if !hasAny {
		return 0, 0, 0, 0, false
	}
	return sum, maxVal, cnt, mask, true
}

// -----------------------------------------------------------------------------
// Storage Key Helpers and Binary/JSON Codecs
// -----------------------------------------------------------------------------

func historyLatestKey(name string) string {
	return zefs.FleetHistoryLatestPrefix + name
}

func historyDeviceShardPrefix(name string) string {
	return zefs.FleetHistoryShardPrefix + name + "/"
}

func historyShardKey(name, shard string) string {
	return zefs.FleetHistoryShardPrefix + name + "/" + shard
}

func historyDeleteIntentKey(name string) string {
	return zefs.FleetHistoryDeleteIntentPrefix + name
}

func isFiniteHistoryFloat(v float64) bool {
	if math.IsNaN(v) {
		return false
	}
	return !math.IsInf(v, 0)
}

func validateHistoryFigureValue(spec *fleet.SampleFigureSpec, val float64) error {
	if !isFiniteHistoryFloat(val) {
		return fmt.Errorf("figure %q value is non-finite", spec.Name)
	}
	if val < spec.Min {
		return fmt.Errorf("figure %q value %g is below minimum %g", spec.Name, val, spec.Min)
	}
	if spec.HasMax && val > spec.Max {
		return fmt.Errorf("figure %q value %g exceeds maximum %g", spec.Name, val, spec.Max)
	}
	return nil
}

func parseHistorySingleKey(prefix, key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, prefix)
	if !ok {
		return "", false
	}
	if rest == "" {
		return "", false
	}
	if strings.ContainsRune(rest, '/') {
		return "", false
	}
	return rest, true
}

func parseHistoryShardKey(key string) (device, shard string, ok bool) {
	rest, trimmed := strings.CutPrefix(key, zefs.FleetHistoryShardPrefix)
	if !trimmed {
		return "", "", false
	}
	device, shard, found := strings.Cut(rest, "/")
	if !found {
		return "", "", false
	}
	if device == "" {
		return "", "", false
	}
	if shard == "" {
		return "", "", false
	}
	if strings.ContainsRune(shard, '/') {
		return "", "", false
	}
	return device, shard, true
}

func encodeHistoryLatest(name string, latest *historyLatest) ([]byte, error) {
	wire := persistedHistoryLatest{
		SchemaVersion:    historyPersistSchemaVersion,
		Device:           name,
		HasReport:        latest.hasReport,
		Unsupported:      latest.unsupported,
		ReceivedAt:       latest.receivedAt.UTC(),
		DeviceTime:       latest.deviceTime,
		LastAcceptedUnix: latest.lastAcceptedUnix,
		LastAcceptedSlot: latest.lastAcceptedSlot,
		LastIntervalSec:  latest.lastIntervalSec,
		Values:           latest.values,
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode fleet history latest for %s: %w", name, err)
	}
	if len(raw) > zefs.FleetHistoryLatestValueMax {
		return nil, fmt.Errorf("encoded fleet history latest for %s size %d exceeds max %d", name, len(raw), zefs.FleetHistoryLatestValueMax)
	}
	return raw, nil
}

func decodeHistoryLatest(expectedDevice string, raw []byte) (historyLatest, error) {
	if len(raw) == 0 {
		return historyLatest{}, errors.New("empty latest history record")
	}
	if len(raw) > zefs.FleetHistoryLatestValueMax {
		return historyLatest{}, fmt.Errorf("latest history record size %d exceeds max %d", len(raw), zefs.FleetHistoryLatestValueMax)
	}
	if !utf8.Valid(raw) {
		return historyLatest{}, errors.New("latest history record is not valid UTF-8")
	}
	var wire persistedHistoryLatest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return historyLatest{}, fmt.Errorf("decode latest history record: %w", err)
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return historyLatest{}, errors.New("trailing bytes after latest history record")
	}
	if wire.SchemaVersion != historyPersistSchemaVersion {
		return historyLatest{}, fmt.Errorf("unsupported latest history schema-version %d", wire.SchemaVersion)
	}
	if wire.Device != expectedDevice {
		return historyLatest{}, fmt.Errorf("latest history record device %q does not match key device %q", wire.Device, expectedDevice)
	}
	if len(wire.Values) > fleet.SampleFiguresMax {
		return historyLatest{}, fmt.Errorf("latest history figure count %d exceeds max %d", len(wire.Values), fleet.SampleFiguresMax)
	}
	for fig, val := range wire.Values {
		spec, ok := fleet.LookupSampleFigure(fig)
		if !ok {
			return historyLatest{}, fmt.Errorf("latest history contains unknown figure %q", fig)
		}
		if err := validateHistoryFigureValue(&spec, val); err != nil {
			return historyLatest{}, fmt.Errorf("latest history %w", err)
		}
	}
	return historyLatest{
		hasReport:        wire.HasReport,
		unsupported:      wire.Unsupported,
		receivedAt:       wire.ReceivedAt.UTC(),
		deviceTime:       wire.DeviceTime,
		lastAcceptedUnix: wire.LastAcceptedUnix,
		lastAcceptedSlot: wire.LastAcceptedSlot,
		lastIntervalSec:  wire.LastIntervalSec,
		values:           maps.Clone(wire.Values),
	}, nil
}

func wrapCompressedShard(kind byte, raw []byte) ([]byte, error) {
	if len(raw) > historyShardUncompressedMax {
		return nil, fmt.Errorf("history shard uncompressed size %d exceeds max %d", len(raw), historyShardUncompressedMax)
	}
	var buf bytes.Buffer
	buf.Grow(12 + len(raw)/2)
	buf.WriteString(historyShardMagic)
	buf.WriteByte(kind)
	buf.WriteByte(historyShardFlagZlib)
	buf.WriteByte(0)
	buf.WriteByte(0)
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(raw)))
	buf.Write(lenBuf[:])
	zw, err := zlib.NewWriterLevel(&buf, zlib.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	if len(out) > zefs.FleetHistoryShardValueMax {
		return nil, fmt.Errorf("compressed history shard size %d exceeds max %d", len(out), zefs.FleetHistoryShardValueMax)
	}
	return out, nil
}

func unwrapCompressedShard(expectedKind byte, blob []byte) ([]byte, error) {
	if len(blob) < 12 {
		return nil, errors.New("history shard truncated header")
	}
	if len(blob) > zefs.FleetHistoryShardValueMax {
		return nil, fmt.Errorf("history shard size %d exceeds max %d", len(blob), zefs.FleetHistoryShardValueMax)
	}
	if string(blob[0:4]) != historyShardMagic {
		return nil, errors.New("history shard invalid magic header")
	}
	if blob[4] != expectedKind {
		return nil, fmt.Errorf("history shard kind %d does not match expected %d", blob[4], expectedKind)
	}
	flags := blob[5]
	uncompLen := binary.LittleEndian.Uint32(blob[8:12])
	if uncompLen > historyShardUncompressedMax {
		return nil, fmt.Errorf("history shard declared uncompressed size %d exceeds max %d", uncompLen, historyShardUncompressedMax)
	}
	if flags == 0 {
		payload := blob[12:]
		if uint32(len(payload)) != uncompLen {
			return nil, errors.New("history shard raw payload length mismatch")
		}
		return payload, nil
	}
	if flags != historyShardFlagZlib {
		return nil, fmt.Errorf("history shard unsupported flags %d", flags)
	}
	zr, err := zlib.NewReader(bytes.NewReader(blob[12:]))
	if err != nil {
		return nil, fmt.Errorf("open history shard zlib stream: %w", err)
	}
	defer zr.Close() //nolint:errcheck // read-only stream cleanup
	limited := io.LimitReader(zr, int64(uncompLen)+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("decompress history shard: %w", err)
	}
	if uint32(len(raw)) != uncompLen {
		return nil, fmt.Errorf("history shard decompressed length %d does not match header %d", len(raw), uncompLen)
	}
	return raw, nil
}

func encodeFineShard(device string, series []*historySeries) ([]byte, error) {
	nonEmpty := 0
	totalBuckets := 0
	for _, ser := range series {
		if ser != nil && ser.fine.count > 0 {
			nonEmpty++
			totalBuckets += ser.fine.count
		}
	}
	if nonEmpty == 0 {
		return nil, nil
	}
	raw := make([]byte, 0, 4+len(device)+nonEmpty*32+totalBuckets*26)
	raw = binary.LittleEndian.AppendUint16(raw, uint16(len(device)))
	raw = append(raw, device...)
	raw = binary.LittleEndian.AppendUint16(raw, uint16(nonEmpty))
	for i, ser := range series {
		if ser == nil {
			continue
		}
		if ser.fine.count == 0 {
			continue
		}
		figName := historyFigureSpecs[i].Name
		raw = binary.LittleEndian.AppendUint16(raw, uint16(len(figName)))
		raw = append(raw, figName...)
		raw = binary.LittleEndian.AppendUint32(raw, uint32(ser.fine.count))
		for j := range ser.fine.count {
			b := ser.fine.buf[(ser.fine.head+j)%len(ser.fine.buf)]
			raw = binary.LittleEndian.AppendUint64(raw, uint64(b.Minute))
			raw = binary.LittleEndian.AppendUint16(raw, b.Count)
			raw = binary.LittleEndian.AppendUint64(raw, math.Float64bits(b.Sum))
			raw = binary.LittleEndian.AppendUint64(raw, math.Float64bits(b.Max))
		}
	}
	return wrapCompressedShard(historyShardKindFine, raw)
}

func decodeFineShard(expectedDevice string, blob []byte) (map[int]fineRing, error) {
	raw, err := unwrapCompressedShard(historyShardKindFine, blob)
	if err != nil {
		return nil, err
	}
	off := 0
	devName, nextOff, err := readBinString(raw, off)
	if err != nil {
		return nil, err
	}
	off = nextOff
	if devName != expectedDevice {
		return nil, fmt.Errorf("fine shard device %q does not match key %q", devName, expectedDevice)
	}
	seriesCount, nextOff, err := readBinUint16(raw, off)
	if err != nil {
		return nil, err
	}
	off = nextOff
	if int(seriesCount) > fleet.SampleFiguresMax {
		return nil, fmt.Errorf("fine shard series count %d exceeds max %d", seriesCount, fleet.SampleFiguresMax)
	}
	out := make(map[int]fineRing, seriesCount)
	for range seriesCount {
		figName, nextOff, err := readBinString(raw, off)
		if err != nil {
			return nil, err
		}
		off = nextOff
		figIdx, ok := historyFigureIdx[figName]
		if !ok {
			return nil, fmt.Errorf("fine shard unknown figure %q", figName)
		}
		if _, dup := out[figIdx]; dup {
			return nil, fmt.Errorf("fine shard duplicate figure %q", figName)
		}
		bucketCount, nextOff, err := readBinUint32(raw, off)
		if err != nil {
			return nil, err
		}
		off = nextOff
		if bucketCount > HistoryFineHorizonMinutes {
			return nil, fmt.Errorf("fine shard figure %q bucket count %d exceeds max %d", figName, bucketCount, HistoryFineHorizonMinutes)
		}
		if len(raw)-off < int(bucketCount)*26 {
			return nil, errors.New("fine shard truncated buckets")
		}
		buf := make([]fineBucket, bucketCount)
		var prevMin int64
		for j := range bucketCount {
			minBits := binary.LittleEndian.Uint64(raw[off : off+8])
			off += 8
			cnt := binary.LittleEndian.Uint16(raw[off : off+2])
			off += 2
			sumVal := math.Float64frombits(binary.LittleEndian.Uint64(raw[off : off+8]))
			off += 8
			maxVal := math.Float64frombits(binary.LittleEndian.Uint64(raw[off : off+8]))
			off += 8
			minute := int64(minBits)
			if j > 0 && minute <= prevMin {
				return nil, errors.New("fine shard buckets are not strictly increasing")
			}
			if cnt == 0 {
				return nil, errors.New("fine shard bucket has zero count")
			}
			if !isFiniteHistoryFloat(sumVal) {
				return nil, errors.New("fine shard bucket contains non-finite float")
			}
			spec := &historyFigureSpecs[figIdx]
			if err := validateHistoryFigureValue(spec, maxVal); err != nil {
				return nil, fmt.Errorf("fine shard bucket max %w", err)
			}
			if err := validateHistoryFigureValue(spec, sumVal/float64(cnt)); err != nil {
				return nil, fmt.Errorf("fine shard bucket mean %w", err)
			}
			prevMin = minute
			buf[j] = fineBucket{Minute: minute, Count: cnt, Sum: sumVal, Max: maxVal}
		}
		out[figIdx] = fineRing{buf: buf, head: 0, count: int(bucketCount)}
	}
	if off != len(raw) {
		return nil, errors.New("fine shard has trailing bytes")
	}
	return out, nil
}

func encodeCoarseShard(device string, series []*historySeries) ([]byte, error) {
	nonEmpty := 0
	totalBytes := 4 + len(device)
	for _, ser := range series {
		if ser != nil && ser.coarse.count > 0 {
			nonEmpty++
			totalBytes += 32 + ser.coarse.count*31 + ser.coarse.valCount*10
		}
	}
	if nonEmpty == 0 {
		return nil, nil
	}
	raw := make([]byte, 0, totalBytes)
	raw = binary.LittleEndian.AppendUint16(raw, uint16(len(device)))
	raw = append(raw, device...)
	raw = binary.LittleEndian.AppendUint16(raw, uint16(nonEmpty))
	for i, ser := range series {
		if ser == nil {
			continue
		}
		if ser.coarse.count == 0 {
			continue
		}
		figName := historyFigureSpecs[i].Name
		raw = binary.LittleEndian.AppendUint16(raw, uint16(len(figName)))
		raw = append(raw, figName...)
		raw = binary.LittleEndian.AppendUint32(raw, uint32(ser.coarse.count))
		valCursor := ser.coarse.valHead
		for j := range ser.coarse.count {
			b := ser.coarse.buckets[(ser.coarse.head+j)%len(ser.coarse.buckets)]
			m := bits.OnesCount16(b.MinuteMask)
			hasCounts := byte(0)
			if ser.coarse.minuteCounts != nil && b.Count != uint32(m) {
				hasCounts = 1
			}
			raw = binary.LittleEndian.AppendUint64(raw, uint64(b.Slot15))
			raw = binary.LittleEndian.AppendUint16(raw, b.MinuteMask)
			raw = binary.LittleEndian.AppendUint32(raw, b.Count)
			raw = binary.LittleEndian.AppendUint64(raw, math.Float64bits(b.Sum))
			raw = binary.LittleEndian.AppendUint64(raw, math.Float64bits(b.Max))
			raw = append(raw, hasCounts)
			for k := range m {
				vIdx := (valCursor + k) % len(ser.coarse.minuteVals)
				raw = binary.LittleEndian.AppendUint64(raw, math.Float64bits(ser.coarse.minuteVals[vIdx]))
			}
			if hasCounts == 1 {
				for k := range m {
					vIdx := (valCursor + k) % len(ser.coarse.minuteCounts)
					raw = binary.LittleEndian.AppendUint16(raw, ser.coarse.minuteCounts[vIdx])
				}
			}
			valCursor = (valCursor + m) % len(ser.coarse.minuteVals)
		}
	}
	return wrapCompressedShard(historyShardKindCoarse, raw)
}

func decodeCoarseShard(expectedDevice string, blob []byte) (map[int]coarseRing, error) {
	raw, err := unwrapCompressedShard(historyShardKindCoarse, blob)
	if err != nil {
		return nil, err
	}
	off := 0
	devName, nextOff, err := readBinString(raw, off)
	if err != nil {
		return nil, err
	}
	off = nextOff
	if devName != expectedDevice {
		return nil, fmt.Errorf("coarse shard device %q does not match key %q", devName, expectedDevice)
	}
	seriesCount, nextOff, err := readBinUint16(raw, off)
	if err != nil {
		return nil, err
	}
	off = nextOff
	if int(seriesCount) > fleet.SampleFiguresMax {
		return nil, fmt.Errorf("coarse shard series count %d exceeds max %d", seriesCount, fleet.SampleFiguresMax)
	}
	out := make(map[int]coarseRing, seriesCount)
	for range seriesCount {
		figName, nextOff, err := readBinString(raw, off)
		if err != nil {
			return nil, err
		}
		off = nextOff
		figIdx, ok := historyFigureIdx[figName]
		if !ok {
			return nil, fmt.Errorf("coarse shard unknown figure %q", figName)
		}
		if _, dup := out[figIdx]; dup {
			return nil, fmt.Errorf("coarse shard duplicate figure %q", figName)
		}
		bucketCount, nextOff, err := readBinUint32(raw, off)
		if err != nil {
			return nil, err
		}
		off = nextOff
		if bucketCount > HistoryCoarseMaxSlots {
			return nil, fmt.Errorf("coarse shard figure %q bucket count %d exceeds max %d", figName, bucketCount, HistoryCoarseMaxSlots)
		}
		ring := coarseRing{
			buckets:    make([]coarseBucket, bucketCount),
			minuteVals: make([]float64, 0, int(bucketCount)*4),
		}
		var prevSlot int64
		for j := range bucketCount {
			if len(raw)-off < 31 {
				return nil, errors.New("coarse shard truncated bucket header")
			}
			slot15 := int64(binary.LittleEndian.Uint64(raw[off : off+8]))
			off += 8
			mask := binary.LittleEndian.Uint16(raw[off : off+2])
			off += 2
			cnt := binary.LittleEndian.Uint32(raw[off : off+4])
			off += 4
			sumVal := math.Float64frombits(binary.LittleEndian.Uint64(raw[off : off+8]))
			off += 8
			maxVal := math.Float64frombits(binary.LittleEndian.Uint64(raw[off : off+8]))
			off += 8
			hasCounts := raw[off]
			off++

			if j > 0 && slot15 <= prevSlot {
				return nil, errors.New("coarse shard buckets are not strictly increasing")
			}
			if mask == 0 {
				return nil, errors.New("coarse shard invalid minute mask")
			}
			if mask&0x8000 != 0 {
				return nil, errors.New("coarse shard invalid minute mask")
			}
			m := bits.OnesCount16(mask)
			if cnt < uint32(m) {
				return nil, errors.New("coarse shard sample count is less than observed minutes")
			}
			if !isFiniteHistoryFloat(sumVal) {
				return nil, errors.New("coarse shard bucket contains non-finite float")
			}
			spec := &historyFigureSpecs[figIdx]
			if err := validateHistoryFigureValue(spec, maxVal); err != nil {
				return nil, fmt.Errorf("coarse shard bucket max %w", err)
			}
			if err := validateHistoryFigureValue(spec, sumVal/float64(cnt)); err != nil {
				return nil, fmt.Errorf("coarse shard bucket mean %w", err)
			}
			if hasCounts != 0 && hasCounts != 1 {
				return nil, errors.New("coarse shard invalid hasCounts flag")
			}
			if hasCounts == 0 && cnt != uint32(m) {
				return nil, errors.New("coarse shard bucket count disagrees with observed minute count")
			}
			needed := m * 8
			if hasCounts == 1 {
				needed += m * 2
			}
			if len(raw)-off < needed {
				return nil, errors.New("coarse shard truncated minute values")
			}
			if hasCounts == 1 && ring.minuteCounts == nil {
				ring.minuteCounts = make([]uint16, len(ring.minuteVals))
				for k := range ring.minuteCounts {
					ring.minuteCounts[k] = 1
				}
			}
			minuteValStart := len(ring.minuteVals)
			for range m {
				v := math.Float64frombits(binary.LittleEndian.Uint64(raw[off : off+8]))
				off += 8
				if !isFiniteHistoryFloat(v) {
					return nil, errors.New("coarse shard minute value is non-finite")
				}
				if hasCounts == 0 {
					if err := validateHistoryFigureValue(spec, v); err != nil {
						return nil, fmt.Errorf("coarse shard minute %w", err)
					}
				}
				ring.minuteVals = append(ring.minuteVals, v)
				if hasCounts == 0 && ring.minuteCounts != nil {
					ring.minuteCounts = append(ring.minuteCounts, 1)
				}
			}
			if hasCounts == 1 {
				var countSum uint32
				for k := range m {
					mc := binary.LittleEndian.Uint16(raw[off : off+2])
					off += 2
					if mc == 0 {
						return nil, errors.New("coarse shard minute count is zero")
					}
					countSum += uint32(mc)
					if err := validateHistoryFigureValue(spec, ring.minuteVals[minuteValStart+k]/float64(mc)); err != nil {
						return nil, fmt.Errorf("coarse shard minute mean %w", err)
					}
					ring.minuteCounts = append(ring.minuteCounts, mc)
				}
				if countSum != cnt {
					return nil, errors.New("coarse shard minute counts do not sum to bucket count")
				}
			}
			prevSlot = slot15
			ring.buckets[j] = coarseBucket{
				Slot15:     slot15,
				MinuteMask: mask,
				Count:      cnt,
				Sum:        sumVal,
				Max:        maxVal,
			}
		}
		ring.count = int(bucketCount)
		ring.valCount = len(ring.minuteVals)
		out[figIdx] = ring
	}
	if off != len(raw) {
		return nil, errors.New("coarse shard has trailing bytes")
	}
	return out, nil
}

func readBinUint16(b []byte, off int) (uint16, int, error) {
	if len(b)-off < 2 {
		return 0, off, errors.New("history shard unexpected EOF reading uint16")
	}
	return binary.LittleEndian.Uint16(b[off : off+2]), off + 2, nil
}

func readBinUint32(b []byte, off int) (uint32, int, error) {
	if len(b)-off < 4 {
		return 0, off, errors.New("history shard unexpected EOF reading uint32")
	}
	return binary.LittleEndian.Uint32(b[off : off+4]), off + 4, nil
}

func readBinString(b []byte, off int) (string, int, error) {
	n, next, err := readBinUint16(b, off)
	if err != nil {
		return "", off, err
	}
	if len(b)-next < int(n) {
		return "", off, errors.New("history shard unexpected EOF reading string")
	}
	return string(b[next : next+int(n)]), next + int(n), nil
}

func floorDivInt64(a, b int64) int64 {
	q := a / b
	r := a % b
	if r < 0 {
		q--
	}
	return q
}

func ceilDivInt64(a, b int64) int64 {
	q := a / b
	r := a % b
	if r > 0 {
		q++
	}
	return q
}

// -----------------------------------------------------------------------------
// Published Hub Accessors (consumed by internal/component/managed/cmd_history.go)
// -----------------------------------------------------------------------------

// ManagedHistory returns the running hub's metric history store, or
// ErrManagedNotServing when no managed server is published. Safe for concurrent
// use.
func ManagedHistory() (*ManagedHistoryStore, error) {
	managedRegistryReaders.RLock()
	defer managedRegistryReaders.RUnlock()
	s := managedDoctorSource.Load()
	if s == nil {
		return nil, ErrManagedNotServing
	}
	if s.history == nil {
		return nil, ErrManagedNotServing
	}
	return s.history, nil
}

// ManagedHistoryLatest answers a caller-scoped latest-samples query from the
// running hub using a pinned registry and history snapshot. Safe for concurrent
// use.
func ManagedHistoryLatest(scope FleetDeviceScope, filter HistoryLatestFilter) ([]HistoryLatestRow, error) {
	managedRegistryReaders.RLock()
	defer managedRegistryReaders.RUnlock()
	s := managedDoctorSource.Load()
	if s == nil {
		return nil, ErrManagedNotServing
	}
	if s.history == nil {
		return nil, ErrManagedNotServing
	}
	return s.HistoryLatest(scope, filter)
}

// ManagedHistorySeries answers a caller-scoped series query from the running
// hub using a pinned registry and history snapshot. Safe for concurrent use.
func ManagedHistorySeries(scope FleetDeviceScope, query HistorySeriesQuery) ([]HistorySeriesRow, error) {
	managedRegistryReaders.RLock()
	defer managedRegistryReaders.RUnlock()
	s := managedDoctorSource.Load()
	if s == nil {
		return nil, ErrManagedNotServing
	}
	if s.history == nil {
		return nil, ErrManagedNotServing
	}
	return s.HistorySeries(scope, query)
}

// ManagedHistoryStorage answers a caller-scoped history storage status query
// from the running hub. Safe for concurrent use.
func ManagedHistoryStorage(scope FleetDeviceScope) (HistoryStorageStatus, error) {
	managedRegistryReaders.RLock()
	defer managedRegistryReaders.RUnlock()
	s := managedDoctorSource.Load()
	if s == nil {
		return HistoryStorageStatus{}, ErrManagedNotServing
	}
	if s.history == nil {
		return HistoryStorageStatus{}, ErrManagedNotServing
	}
	return s.HistoryStorage(scope)
}
