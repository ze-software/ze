// Design: docs/architecture/fleet-alarms.md -- hub alarm engine, evaluators, deadlines, and authorized queries
// Related: managed_alarms_kinds.go -- open alarm kind registry and built-in kinds
// Related: managed_alarms_version.go -- software version ordering for software-behind
// Related: managed_alarms_store.go -- durable shard persistence, retention, and background flush loop

package server

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
)

const (
	// AlarmPageMax is the maximum number of open alarm occurrences returned in
	// one page by QueryOpen.
	AlarmPageMax = 1024
	// AlarmHistoryPageMax is the maximum number of alarm history events returned
	// in one page by QueryHistory.
	AlarmHistoryPageMax = 512

	alarmSnapshotCacheMax  = 64
	alarmSnapshotTTL       = 5 * time.Minute
	alarmCursorBytesMax    = 512
	defaultAlarmFlushEvery = 5 * time.Second

	alarmIDPrefix       = "alm-"
	alarmSnapshotPrefix = "snap-"

	alarmCursorTypeOpen    = "open"
	alarmCursorTypeHistory = "history"

	alarmResourceSubjectCPU     = "cpu"
	alarmResourceSubjectMemory  = "memory"
	alarmResourceSubjectStorage = "storage"

	alarmShowCommand        = "show fleet alarm"
	alarmHubScopeProbeAlpha = "z0-ze-hub-scope-probe-a"
	alarmHubScopeProbeDigit = "0z-ze-hub-scope-probe-b"

	alarmRestartFix = "restart the query without a cursor to start a fresh snapshot"
)

// AlarmOccurrence is one currently open alarm condition derived by the hub.
type AlarmOccurrence struct {
	ID           string        `json:"id"`
	Kind         string        `json:"kind"`
	Severity     AlarmSeverity `json:"severity"`
	Scope        AlarmScope    `json:"scope"`
	Device       string        `json:"device,omitempty"`
	Subject      string        `json:"subject,omitempty"`
	Summary      string        `json:"summary"`
	Detail       string        `json:"detail,omitempty"`
	Since        time.Time     `json:"since"`
	HiddenBehind string        `json:"hidden-behind,omitempty"`
}

// AlarmCounts summarizes an entire authorized open-alarm result across all
// pages.
type AlarmCounts struct {
	Total         int `json:"total"`
	Critical      int `json:"critical"`
	Warning       int `json:"warning"`
	Informational int `json:"informational"`
	Visible       int `json:"visible"`
	Hidden        int `json:"hidden"`
}

// AlarmListQuery filters and paginates an open-alarm query.
type AlarmListQuery struct {
	Severity string
	Kind     string
	Device   string
	Limit    int
	Cursor   string
}

// AlarmListPage is one bounded page of an authorized open-alarm snapshot.
type AlarmListPage struct {
	Alarms         []AlarmOccurrence `json:"alarms"`
	Total          int               `json:"total"`
	Counts         AlarmCounts       `json:"counts"`
	Cursor         string            `json:"cursor,omitempty"`
	HasMore        bool              `json:"has-more"`
	SnapshotID     string            `json:"snapshot-id"`
	Degraded       bool              `json:"degraded,omitempty"`
	DegradedReason string            `json:"degraded-reason,omitempty"`
}

// AlarmHistoryEvent is one past or currently open alarm occurrence within the
// retention period.
type AlarmHistoryEvent struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind"`
	Severity  AlarmSeverity `json:"severity"`
	Scope     AlarmScope    `json:"scope"`
	Device    string        `json:"device,omitempty"`
	Subject   string        `json:"subject,omitempty"`
	Summary   string        `json:"summary"`
	Detail    string        `json:"detail,omitempty"`
	Opened    time.Time     `json:"opened"`
	Cleared   bool          `json:"cleared"`
	ClearedAt time.Time     `json:"cleared-at,omitzero"`
}

// AlarmHistoryQuery filters and paginates a device's alarm history query.
type AlarmHistoryQuery struct {
	Device string
	Limit  int
	Cursor string
}

// AlarmHistoryPage is one bounded page of an authorized device alarm history
// snapshot.
type AlarmHistoryPage struct {
	Device         string              `json:"device"`
	Events         []AlarmHistoryEvent `json:"events"`
	Total          int                 `json:"total"`
	Cursor         string              `json:"cursor,omitempty"`
	HasMore        bool                `json:"has-more"`
	SnapshotID     string              `json:"snapshot-id"`
	RetainedSince  time.Time           `json:"retained-since,omitzero"`
	Truncated      bool                `json:"truncated,omitempty"`
	Degraded       bool                `json:"degraded,omitempty"`
	DegradedReason string              `json:"degraded-reason,omitempty"`
}

// ManagedAlarmHealth summarizes hub alarm store durability, shard health, and
// retention state for runtime doctor checks.
type ManagedAlarmHealth struct {
	Degraded         bool
	DegradedReason   string
	WriteFailed      bool
	LastWriteError   string
	LastWriteOKAt    time.Time
	LastWriteFailAt  time.Time
	CorruptShards    int
	LastCorruptError string
	OpenCount        int
	RetainedClosed   int
	Truncated        bool
}

// ManagedAlarmStoreConfig configures a ManagedAlarmStore instance.
type ManagedAlarmStoreConfig struct {
	Config        plugin.HubAlarmsConfig
	Store         storage.Storage
	Clock         clock.Clock
	Now           func() time.Time
	Registry      *AlarmKindRegistry
	FlushInterval time.Duration
}

type alarmKey struct {
	kind    string
	device  string
	subject string
}

type alarmDevice struct {
	name                   string
	generation             uint64
	declared               bool
	declaredAt             time.Time
	declarationGeneration  uint64
	connectedInDeclaration bool
	connected              bool
	connectedSince         time.Time
	lastDisconnectedAt     time.Time

	softwareVersion      string
	configDiverged       bool
	configDivergedDetail string
	configRejected       bool
	configRejectedError  string
	health               *ManagedHealthSnapshot
	inventory            *ManagedInventorySnapshot
	latestSamples        map[string]float64
	latestSampleAt       time.Time

	resourceQualifySince map[string]time.Time

	open             map[alarmKey]*AlarmOccurrence
	closed           []AlarmHistoryEvent
	earliestRetained time.Time
	truncated        bool

	dirty       bool
	mutationSeq uint64
}

type alarmCursorPayload struct {
	Type       string `json:"t"`
	SnapshotID string `json:"s"`
	Offset     int    `json:"o"`
}

type alarmOpenSnapshot struct {
	id                string
	createdAt         time.Time
	expiresAt         time.Time
	lastUsed          uint64
	username          string
	scopeMode         fleetScopeMode
	includedHub       bool
	authorizedDevices []string
	filterSeverity    AlarmSeverity
	filterKind        string
	filterDevice      string
	rows              []AlarmOccurrence
	counts            AlarmCounts
}

type alarmHistorySnapshot struct {
	id            string
	createdAt     time.Time
	expiresAt     time.Time
	lastUsed      uint64
	username      string
	scopeMode     fleetScopeMode
	device        string
	events        []AlarmHistoryEvent
	retainedSince time.Time
	truncated     bool
}

// ManagedAlarmStore derives hub-side fleet alarms from device declarations,
// session connectivity, component health, config status, metric samples,
// inventory versions, history capacity health, and registered external kinds.
// It persists active occurrences, declaration timing, and bounded closed
// episode history to the borrowed hub store and answers authorized paginated
// queries with immutable continuation snapshots.
//
// Safe for concurrent use. Lock ordering: never acquire ioMu while holding mu,
// and always acquire ManagedServer.mu before ManagedAlarmStore.mu when both
// are needed.
type ManagedAlarmStore struct {
	store         storage.Storage
	now           func() time.Time
	registry      *AlarmKindRegistry
	flushInterval time.Duration

	// ioMu serializes all storage I/O. Never acquired while holding mu.
	ioMu sync.Mutex

	mu              sync.Mutex
	cfg             plugin.HubAlarmsConfig
	nextSeq         uint64
	devices         map[string]*alarmDevice
	deviceGen       map[string]uint64
	pendingDeletes  map[string]uint64
	hubOpen         map[alarmKey]*AlarmOccurrence
	hubClosed       []AlarmHistoryEvent
	hubEarliest     time.Time
	hubTruncated    bool
	hubDirty        bool
	hubMutationSeq  uint64
	historyHealth   *ManagedHistoryHealth
	writeFailed     bool
	lastWriteErr    string
	lastWriteOKAt   time.Time
	lastWriteFailAt time.Time
	corruptShards   int
	lastCorruptErr  string

	snapshotSeq      uint64
	snapshotLRUSeq   uint64
	openSnapshots    map[string]*alarmOpenSnapshot
	historySnapshots map[string]*alarmHistorySnapshot

	writeWake    chan struct{}
	deadlineWake chan struct{}
}

func normalizeAlarmsConfig(cfg plugin.HubAlarmsConfig) plugin.HubAlarmsConfig {
	if cfg == (plugin.HubAlarmsConfig{}) {
		return plugin.DefaultHubAlarmsConfig()
	}
	return cfg
}

// NewManagedAlarmStore constructs a hub alarm store using cfg, falling back to
// DefaultHubAlarmsConfig, RealClock, and the package-level alarm kind registry
// when optional fields are omitted.
func NewManagedAlarmStore(cfg ManagedAlarmStoreConfig) *ManagedAlarmStore {
	acfg := normalizeAlarmsConfig(cfg.Config)
	nowFn := cfg.Now
	if nowFn == nil {
		if cfg.Clock != nil {
			nowFn = cfg.Clock.Now
		} else {
			nowFn = clock.RealClock{}.Now
		}
	}
	reg := cfg.Registry
	if reg == nil {
		reg = defaultAlarmKindRegistry
	}
	flushEvery := cfg.FlushInterval
	if flushEvery <= 0 {
		flushEvery = defaultAlarmFlushEvery
	}
	return &ManagedAlarmStore{
		store:            cfg.Store,
		now:              nowFn,
		registry:         reg,
		flushInterval:    flushEvery,
		cfg:              acfg,
		devices:          make(map[string]*alarmDevice),
		deviceGen:        make(map[string]uint64),
		pendingDeletes:   make(map[string]uint64),
		hubOpen:          make(map[alarmKey]*AlarmOccurrence),
		openSnapshots:    make(map[string]*alarmOpenSnapshot),
		historySnapshots: make(map[string]*alarmHistorySnapshot),
		writeWake:        make(chan struct{}, 1),
		deadlineWake:     make(chan struct{}, 1),
	}
}

func (s *ManagedAlarmStore) currentTime() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

// Config returns the active hub alarm configuration. Safe for concurrent use.
func (s *ManagedAlarmStore) Config() plugin.HubAlarmsConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// UpdateConfig installs validated hub alarm configuration after a reload,
// re-evaluates software-behind thresholds, and enforces tightened episode caps.
// Safe for concurrent use.
func (s *ManagedAlarmStore) UpdateConfig(next plugin.HubAlarmsConfig) {
	now := s.currentTime()
	s.mu.Lock()
	s.cfg = normalizeAlarmsConfig(next)
	s.evaluateSoftwareBehindLocked(now)
	s.pruneAllLocked(now)
	s.mu.Unlock()
	s.evaluateExternalKindsAt(now)
	s.wakeDeadline()
	s.wakeWriter()
}

// SyncDeclarations reconciles declared device membership across startup and
// configuration reloads. Unchanged declarations preserve their original
// DeclaredAt, DeclarationGeneration, and ConnectedInDeclaration; removed
// declarations stop offline tracking and close session/telemetry alarms while
// leaving config-diverged governed by held-record existence; re-added names
// begin a new declaration generation and grace interval. Safe for concurrent
// use.
func (s *ManagedAlarmStore) SyncDeclarations(declared map[string]bool) {
	now := s.currentTime()
	s.mu.Lock()
	changed := false

	// Process newly declared and re-added devices in deterministic name order.
	names := make([]string, 0, len(declared))
	for name, ok := range declared {
		if ok && name != "" {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	for _, name := range names {
		dev := s.devices[name]
		if dev == nil {
			dev = s.ensureDeviceLocked(name)
			dev.declared = true
			dev.declarationGeneration = 1
			dev.declaredAt = now
			dev.connectedInDeclaration = dev.connected
			dev.lastDisconnectedAt = time.Time{}
			s.markDeviceDirtyLocked(dev)
			changed = true
			continue
		}
		if !dev.declared {
			dev.declared = true
			dev.declarationGeneration++
			dev.declaredAt = now
			dev.connectedInDeclaration = dev.connected
			dev.lastDisconnectedAt = time.Time{}
			s.closeDeviceOccurrenceLocked(dev, alarmKey{kind: AlarmKindDeviceOffline, device: name}, now)
			s.markDeviceDirtyLocked(dev)
			changed = true
		}
	}

	// Process removed declarations in deterministic name order.
	existingNames := slices.Sorted(maps.Keys(s.devices))
	for _, name := range existingNames {
		dev := s.devices[name]
		if !dev.declared {
			continue
		}
		if declared[name] {
			continue
		}
		dev.declared = false
		dev.connectedInDeclaration = false
		dev.configRejected = false
		dev.configRejectedError = ""
		s.closeUndeclaredDeviceAlarmsLocked(dev, now)
		s.markDeviceDirtyLocked(dev)
		changed = true
	}

	s.evaluateSoftwareBehindLocked(now)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(now)
	if changed {
		s.wakeDeadline()
		s.wakeWriter()
	}
}

// ReconcileStartup aligns persisted alarm state loaded from storage with live
// startup device records, held-config divergence details, and metric history
// health. Persisted declaration start times, active occurrence IDs, and opening
// timestamps survive unchanged for continuing declarations and conditions still
// true; re-added declarations start a fresh generation and grace interval;
// unclean crashes while connected seed a fresh offline grace window from
// restart time; ended conditions record an observed clear at now without
// inventing downtime timestamps. Safe for concurrent use.
func (s *ManagedAlarmStore) ReconcileStartup(devices []ManagedDevice, held map[string]string, histHealth *ManagedHistoryHealth) {
	now := s.currentTime()
	s.mu.Lock()

	seen := make(map[string]struct{}, len(devices))
	ordered := slices.Clone(devices)
	slices.SortFunc(ordered, func(a, b ManagedDevice) int {
		return cmp.Compare(a.Name, b.Name)
	})

	for i := range ordered {
		d := &ordered[i]
		if d.Name == "" {
			continue
		}
		seen[d.Name] = struct{}{}
		heldDetail, isHeld := held[d.Name]
		diverged := d.Diverged || isHeld

		dev := s.devices[d.Name]
		existed := dev != nil
		if !existed {
			if !d.Declared && !diverged {
				continue
			}
			dev = s.ensureDeviceLocked(d.Name)
		}

		if d.Declared {
			if !dev.declared {
				// Freshly declared (first observation or previously removed
				// declaration re-added across restart): begin a new
				// declaration generation and grace interval from now.
				dev.declared = true
				dev.declaredAt = now
				dev.declarationGeneration++
				dev.connectedInDeclaration = false
				dev.lastDisconnectedAt = time.Time{}
				s.closeDeviceOccurrenceLocked(dev, alarmKey{kind: AlarmKindDeviceOffline, device: d.Name}, now)
				s.markDeviceDirtyLocked(dev)
			} else if dev.declaredAt.IsZero() {
				dev.declaredAt = now
				s.markDeviceDirtyLocked(dev)
			}
		} else {
			if dev.declared {
				dev.declared = false
				dev.connectedInDeclaration = false
				s.markDeviceDirtyLocked(dev)
			}
			dev.configRejected = false
			dev.configRejectedError = ""
			s.closeUndeclaredDeviceAlarmsLocked(dev, now)
		}

		dev.connected = d.Connected
		if d.Connected {
			if d.ConnectedSince.IsZero() {
				dev.connectedSince = now
			} else {
				dev.connectedSince = d.ConnectedSince.UTC()
			}
			dirty := false
			if dev.declared && !dev.connectedInDeclaration {
				dev.connectedInDeclaration = true
				dirty = true
			}
			if !dev.lastDisconnectedAt.IsZero() {
				dev.lastDisconnectedAt = time.Time{}
				dirty = true
			}
			if dirty {
				s.markDeviceDirtyLocked(dev)
			}
			s.closeDeviceOccurrenceLocked(dev, alarmKey{kind: AlarmKindDeviceOffline, device: d.Name}, now)
			s.closeSettlingOccurrencesLocked(dev, now)
			clear(dev.resourceQualifySince)
		} else if dev.declared && dev.connectedInDeclaration && dev.lastDisconnectedAt.IsZero() {
			// The hub restarted without recording a clean disconnect for a
			// device that was connected during this declaration. Seed the
			// offline grace window from restart time rather than immediately
			// flapping device-offline from an older declaredAt timestamp.
			dev.lastDisconnectedAt = now
			s.markDeviceDirtyLocked(dev)
		}

		if d.Declared && d.Health != nil {
			s.applyHealthReportLocked(dev, *d.Health, now)
		}
		if d.Inventory != nil {
			dev.inventory = d.Inventory.Clone()
			dev.softwareVersion = d.Inventory.Report.SoftwareVersion
		}
		if diverged && heldDetail == "" {
			if existing := dev.open[alarmKey{kind: AlarmKindConfigDiverged, device: d.Name}]; existing != nil {
				heldDetail = existing.Detail
			}
		}
		rejected := d.Declared && d.AckResult == ManagedAckRefused
		rejectErr := ""
		if rejected {
			rejectErr = d.AckError
		}
		s.applyConfigStateLocked(dev, diverged, heldDetail, rejected, rejectErr, now)
	}

	// Mark any loaded device absent from the startup registry as undeclared and
	// disconnected, closing session/telemetry alarms while leaving
	// config-diverged governed by held-record existence.
	for _, name := range slices.Sorted(maps.Keys(s.devices)) {
		if _, ok := seen[name]; ok {
			continue
		}
		dev := s.devices[name]
		dev.connected = false
		if dev.declared {
			dev.declared = false
			dev.connectedInDeclaration = false
			s.markDeviceDirtyLocked(dev)
		}
		dev.configRejected = false
		dev.configRejectedError = ""
		s.closeUndeclaredDeviceAlarmsLocked(dev, now)
		heldDetail, isHeld := held[name]
		if isHeld && heldDetail == "" {
			if existing := dev.open[alarmKey{kind: AlarmKindConfigDiverged, device: name}]; existing != nil {
				heldDetail = existing.Detail
			}
		}
		s.applyConfigStateLocked(dev, isHeld, heldDetail, false, "", now)
	}

	s.evaluateSoftwareBehindLocked(now)
	if histHealth != nil {
		s.applyHistoryHealthLocked(histHealth, now)
	}
	s.pruneAllLocked(now)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(now)
	s.wakeDeadline()
	s.wakeWriter()
}

// OnDeviceConnected records an authenticated session admission for name at
// connectedAt. It cancels any pending offline grace interval, clears
// lastDisconnectedAt and device-offline, immediately closes any open
// resource-high (or other settling) occurrences at connectedAt, and resets
// resource qualification timers while leaving non-settling occurrences (such as
// component-down) untouched. Safe for concurrent use.
func (s *ManagedAlarmStore) OnDeviceConnected(name string, connectedAt time.Time) {
	if name == "" {
		return
	}
	at := connectedAt.UTC()
	if at.IsZero() {
		at = s.currentTime()
	}

	s.mu.Lock()
	dev := s.ensureDeviceLocked(name)
	dev.connected = true
	dev.connectedSince = at
	dirty := false
	if dev.declared && !dev.connectedInDeclaration {
		dev.connectedInDeclaration = true
		dirty = true
	}
	if !dev.lastDisconnectedAt.IsZero() {
		dev.lastDisconnectedAt = time.Time{}
		dirty = true
	}
	if dirty {
		s.markDeviceDirtyLocked(dev)
	}
	s.closeDeviceOccurrenceLocked(dev, alarmKey{kind: AlarmKindDeviceOffline, device: name}, at)
	s.closeSettlingOccurrencesLocked(dev, at)
	clear(dev.resourceQualifySince)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(at)
	s.wakeDeadline()
	s.wakeWriter()
}

// OnDeviceDisconnected records session termination for name at disconnectedAt.
// It arms the offline deadline from disconnectedAt, clears any pending
// (unopened) resource qualification timer so offline gaps cannot advance
// qualification, and preserves all already-open occurrences with their original
// IDs and opening times. Safe for concurrent use.
func (s *ManagedAlarmStore) OnDeviceDisconnected(name string, disconnectedAt time.Time) {
	if name == "" {
		return
	}
	at := disconnectedAt.UTC()
	if at.IsZero() {
		at = s.currentTime()
	}

	s.mu.Lock()
	dev := s.devices[name]
	if dev == nil {
		s.mu.Unlock()
		return
	}
	dev.connected = false
	dev.lastDisconnectedAt = at
	clear(dev.resourceQualifySince)
	s.markDeviceDirtyLocked(dev)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(at)
	s.wakeDeadline()
	s.wakeWriter()
}

// OnHealthReport evaluates component-down and component-degraded conditions
// from snap for name. Identical repeated reports are strictly idempotent.
// Safe for concurrent use.
func (s *ManagedAlarmStore) OnHealthReport(name string, snap ManagedHealthSnapshot) {
	if name == "" {
		return
	}
	at := snap.ReceivedAt.UTC()
	if at.IsZero() {
		at = s.currentTime()
	}

	s.mu.Lock()
	dev := s.ensureDeviceLocked(name)
	s.applyHealthReportLocked(dev, snap, at)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(at)
}

func (s *ManagedAlarmStore) applyHealthReportLocked(dev *alarmDevice, snap ManagedHealthSnapshot, at time.Time) {
	dev.health = snap.Clone()

	wantDown := make(map[string]fleet.HealthComponent)
	wantDegraded := make(map[string]fleet.HealthComponent)
	for _, c := range snap.Report.Components {
		switch c.Status {
		case fleet.HealthStatusDown:
			wantDown[c.Name] = c
		case fleet.HealthStatusDegraded:
			wantDegraded[c.Name] = c
		case fleet.HealthStatusHealthy, fleet.HealthStatusUnspecified:
			// Healthy and unspecified components open no component alarm.
		default:
			// Open enum: unknown future status opens no component alarm.
		}
	}

	// Close resolved component-down and component-degraded occurrences first so
	// degraded <-> down transitions record the clear before opening the new kind.
	var toClose []alarmKey
	for k := range dev.open {
		if k.kind == AlarmKindComponentDown {
			if _, ok := wantDown[k.subject]; !ok {
				toClose = append(toClose, k)
			}
		}
		if k.kind == AlarmKindComponentDegraded {
			if _, ok := wantDegraded[k.subject]; !ok {
				toClose = append(toClose, k)
			}
		}
	}
	slices.SortFunc(toClose, compareAlarmKeys)
	for _, k := range toClose {
		s.closeDeviceOccurrenceLocked(dev, k, at)
	}

	var tb textbuf.Buffer
	for _, compName := range slices.Sorted(maps.Keys(wantDown)) {
		comp := wantDown[compName]
		summary := tb.Reset().Str("Component ").Str(compName).Str(" is down").String()
		s.openDeviceOccurrenceLocked(
			dev,
			alarmKey{kind: AlarmKindComponentDown, device: dev.name, subject: compName},
			AlarmSeverityCritical,
			summary,
			boundedManagedDetail(comp.Reason),
			at,
		)
	}
	for _, compName := range slices.Sorted(maps.Keys(wantDegraded)) {
		comp := wantDegraded[compName]
		summary := tb.Reset().Str("Component ").Str(compName).Str(" is degraded").String()
		s.openDeviceOccurrenceLocked(
			dev,
			alarmKey{kind: AlarmKindComponentDegraded, device: dev.name, subject: compName},
			AlarmSeverityWarning,
			summary,
			boundedManagedDetail(comp.Reason),
			at,
		)
	}
}

// OnConfigState evaluates config-diverged and config-rejected independently for
// name. Safe for concurrent use.
func (s *ManagedAlarmStore) OnConfigState(name string, diverged bool, heldDetail string, rejected bool, rejectErr string) {
	if name == "" {
		return
	}
	now := s.currentTime()
	s.mu.Lock()
	dev := s.ensureDeviceLocked(name)
	s.applyConfigStateLocked(dev, diverged, heldDetail, rejected, rejectErr, now)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(now)
}

func (s *ManagedAlarmStore) applyConfigStateLocked(dev *alarmDevice, diverged bool, heldDetail string, rejected bool, rejectErr string, at time.Time) {
	dev.configDiverged = diverged
	dev.configDivergedDetail = boundedManagedDetail(heldDetail)
	dev.configRejected = rejected
	dev.configRejectedError = boundedManagedDetail(rejectErr)

	divKey := alarmKey{kind: AlarmKindConfigDiverged, device: dev.name}
	if diverged {
		s.openDeviceOccurrenceLocked(
			dev,
			divKey,
			AlarmSeverityWarning,
			"Device holds a locally edited configuration",
			dev.configDivergedDetail,
			at,
		)
	} else {
		s.closeDeviceOccurrenceLocked(dev, divKey, at)
	}

	rejKey := alarmKey{kind: AlarmKindConfigRejected, device: dev.name}
	if rejected {
		s.openDeviceOccurrenceLocked(
			dev,
			rejKey,
			AlarmSeverityWarning,
			"Device rejected the hub configuration",
			dev.configRejectedError,
			at,
		)
	} else {
		s.closeDeviceOccurrenceLocked(dev, rejKey, at)
	}
}

// OnSampleReport evaluates resource-high across CPU, memory, and storage
// figures for name. Only samples received while connected (and at or after the
// current session's connectedAt) advance qualification. Missing figures or
// non-positive totals reset pending qualification timers so gaps never advance
// qualification while preserving any already-open occurrence. Safe for
// concurrent use.
func (s *ManagedAlarmStore) OnSampleReport(name string, report fleet.SampleReport, receivedAt time.Time) {
	if name == "" {
		return
	}
	at := receivedAt.UTC()
	if at.IsZero() {
		at = s.currentTime()
	}

	vals := make(map[string]float64, len(report.Samples))
	for _, reading := range report.Samples {
		vals[reading.Figure] = reading.Value
	}

	s.mu.Lock()
	dev := s.devices[name]
	if dev == nil {
		s.mu.Unlock()
		return
	}
	if !dev.connected {
		s.mu.Unlock()
		return
	}
	if !dev.connectedSince.IsZero() && at.Before(dev.connectedSince) {
		s.mu.Unlock()
		return
	}

	dev.latestSamples = vals
	dev.latestSampleAt = at

	duration := time.Duration(s.cfg.ResourceDurationSeconds) * time.Second

	if cpuPct, ok := vals[fleet.FigureCPUUsed]; ok && isFiniteAlarmFloat(cpuPct) {
		s.evalResourceSubjectLocked(dev, alarmResourceSubjectCPU, "CPU", cpuPct, float64(s.cfg.ResourceCPUPercent), duration, at)
	} else {
		delete(dev.resourceQualifySince, alarmResourceSubjectCPU)
	}
	if memPct, ok := ratioPercentFromSamples(vals, fleet.FigureMemoryUsed, fleet.FigureMemoryTotal); ok {
		s.evalResourceSubjectLocked(dev, alarmResourceSubjectMemory, "Memory", memPct, float64(s.cfg.ResourceMemoryPercent), duration, at)
	} else {
		delete(dev.resourceQualifySince, alarmResourceSubjectMemory)
	}
	if storPct, ok := ratioPercentFromSamples(vals, fleet.FigureStorageUsed, fleet.FigureStorageTotal); ok {
		s.evalResourceSubjectLocked(dev, alarmResourceSubjectStorage, "Storage", storPct, float64(s.cfg.ResourceStoragePercent), duration, at)
	} else {
		delete(dev.resourceQualifySince, alarmResourceSubjectStorage)
	}
	s.mu.Unlock()

	s.evaluateExternalKindsAt(at)
}

func ratioPercentFromSamples(vals map[string]float64, usedKey, totalKey string) (float64, bool) {
	used, okUsed := vals[usedKey]
	if !okUsed {
		return 0, false
	}
	total, okTotal := vals[totalKey]
	if !okTotal {
		return 0, false
	}
	ratio, ok := fleet.SampleRatioValue(used, total)
	if !ok {
		return 0, false
	}
	return ratio * 100.0, true
}

func isFiniteAlarmFloat(v float64) bool {
	if math.IsNaN(v) {
		return false
	}
	return !math.IsInf(v, 0)
}

func (s *ManagedAlarmStore) evalResourceSubjectLocked(dev *alarmDevice, subject, label string, pct, threshold float64, duration time.Duration, at time.Time) {
	key := alarmKey{kind: AlarmKindResourceHigh, device: dev.name, subject: subject}
	if pct < threshold {
		delete(dev.resourceQualifySince, subject)
		s.closeDeviceOccurrenceLocked(dev, key, at)
		return
	}

	var tb textbuf.Buffer
	summary := tb.Reset().Str(label).Str(" utilization at or above ").Int(int64(threshold)).Str("% threshold").String()
	detail := tb.Reset().Str(subject).Byte('=').Float(pct, 1).Str("% threshold=").Int(int64(threshold)).Str("% duration=").Int(int64(s.cfg.ResourceDurationSeconds)).Byte('s').String()

	if _, alreadyOpen := dev.open[key]; alreadyOpen {
		s.openDeviceOccurrenceLocked(dev, key, AlarmSeverityWarning, summary, detail, at)
		return
	}

	since, qualifying := dev.resourceQualifySince[subject]
	if !qualifying {
		dev.resourceQualifySince[subject] = at
		since = at
	}
	if at.Sub(since) >= duration {
		// Opens at qualification time (at), never backdated to since.
		s.openDeviceOccurrenceLocked(dev, key, AlarmSeverityWarning, summary, detail, at)
	}
}

// OnInventoryReport records name's latest software version from snap and
// re-evaluates software-behind across all declared devices. Safe for concurrent
// use.
func (s *ManagedAlarmStore) OnInventoryReport(name string, snap ManagedInventorySnapshot) {
	if name == "" {
		return
	}
	at := snap.ReceivedAt.UTC()
	if at.IsZero() {
		at = s.currentTime()
	}

	s.mu.Lock()
	dev := s.ensureDeviceLocked(name)
	dev.inventory = snap.Clone()
	dev.softwareVersion = snap.Report.SoftwareVersion
	s.evaluateSoftwareBehindLocked(at)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(at)
}

func (s *ManagedAlarmStore) evaluateSoftwareBehindLocked(at time.Time) {
	parsedByDevice := make(map[string]parsedSoftwareVersion, len(s.devices))
	parsedList := make([]parsedSoftwareVersion, 0, len(s.devices))
	for name, dev := range s.devices {
		if !dev.declared {
			continue
		}
		if pv, ok := parseSoftwareVersion(dev.softwareVersion); ok {
			parsedByDevice[name] = pv
			parsedList = append(parsedList, pv)
		}
	}

	unique := buildUniqueOrderedVersions(parsedList)
	names := slices.Sorted(maps.Keys(s.devices))
	if len(unique) <= 1 {
		for _, name := range names {
			s.closeDeviceOccurrenceLocked(s.devices[name], alarmKey{kind: AlarmKindSoftwareBehind, device: name}, at)
		}
		return
	}

	newestIdx := len(unique) - 1
	newestDisplay := unique[newestIdx].display
	threshold := max(1, s.cfg.SoftwareBehindVersions)

	var tb textbuf.Buffer
	for _, name := range names {
		dev := s.devices[name]
		key := alarmKey{kind: AlarmKindSoftwareBehind, device: name}
		if !dev.declared {
			s.closeDeviceOccurrenceLocked(dev, key, at)
			continue
		}
		pv, ok := parsedByDevice[name]
		if !ok {
			s.closeDeviceOccurrenceLocked(dev, key, at)
			continue
		}
		rank := softwareVersionRank(unique, pv)
		if rank < 0 {
			s.closeDeviceOccurrenceLocked(dev, key, at)
			continue
		}
		behind := newestIdx - rank
		if behind < threshold {
			s.closeDeviceOccurrenceLocked(dev, key, at)
			continue
		}
		unit := " versions"
		if behind == 1 {
			unit = " version"
		}
		summary := tb.Reset().Str("Software ").Str(dev.softwareVersion).Str(" is ").Int(int64(behind)).Str(unit).Str(" behind ").Str(newestDisplay).String()
		detail := tb.Reset().Str("device=").Str(dev.softwareVersion).Str(" newest=").Str(newestDisplay).Str(" behind=").Int(int64(behind)).Str(" threshold=").Int(int64(threshold)).String()
		s.openDeviceOccurrenceLocked(dev, key, AlarmSeverityInformational, summary, detail, at)
	}
}

// OnHistoryHealth evaluates the hub-scoped history-full alarm from health,
// exposing actual used bytes, budget, and effective retention in the alarm
// summary and detail. Safe for concurrent use.
func (s *ManagedAlarmStore) OnHistoryHealth(health ManagedHistoryHealth) {
	now := s.currentTime()
	s.mu.Lock()
	s.applyHistoryHealthLocked(&health, now)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(now)
}

func (s *ManagedAlarmStore) applyHistoryHealthLocked(health *ManagedHistoryHealth, at time.Time) {
	cp := *health
	cp.CorruptRecords = slices.Clone(health.CorruptRecords)
	s.historyHealth = &cp

	key := alarmKey{kind: AlarmKindHistoryFull}
	if !health.CapacityPressure {
		s.closeHubOccurrenceLocked(key, at)
		return
	}

	var tb textbuf.Buffer
	summary := tb.Reset().
		Str("Hub metric history reached capacity cap (used ").Int(health.UsedBytes).
		Str(" / ").Int(health.HubBudgetBytes).Str(" bytes, effective retention ").
		Int(health.EffectiveRetentionSeconds).Str("s)").String()
	detail := tb.Reset().
		Str("used-bytes=").Int(health.UsedBytes).
		Str(" hub-budget-bytes=").Int(health.HubBudgetBytes).
		Str(" device-budget-bytes=").Int(health.DeviceBudgetBytes).
		Str(" effective-retention-seconds=").Int(health.EffectiveRetentionSeconds).
		Str(" configured-retention-days=").Int(int64(health.ConfiguredRetentionDays)).String()
	s.openHubOccurrenceLocked(key, AlarmSeverityWarning, summary, detail, at)
}

// EvaluateExternalKinds invokes every registered AlarmKindSpec carrying a
// non-nil Evaluate callback against a read-only snapshot of hub facts and
// reconciles their active occurrences. Callbacks run outside s.mu.
// Safe for concurrent use.
func (s *ManagedAlarmStore) EvaluateExternalKinds() {
	s.evaluateExternalKindsAt(s.currentTime())
}

func (s *ManagedAlarmStore) evaluateExternalKindsAt(now time.Time) {
	if !s.registry.HasExternalEvaluators() {
		return
	}
	specs := s.registry.List()
	evalSpecs := make([]AlarmKindSpec, 0, len(specs))
	for _, spec := range specs {
		if spec.Evaluate != nil {
			evalSpecs = append(evalSpecs, spec)
		}
	}
	if len(evalSpecs) == 0 {
		return
	}

	at := now.UTC()
	if at.IsZero() {
		at = s.currentTime()
	}
	ctx := s.buildEvalContextAt(at)

	type kindEvalResult struct {
		spec AlarmKindSpec
		obs  []AlarmObservation
	}
	results := make([]kindEvalResult, 0, len(evalSpecs))
	for _, spec := range evalSpecs {
		results = append(results, kindEvalResult{
			spec: spec,
			obs:  spec.Evaluate(ctx),
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, res := range results {
		s.reconcileExternalKindLocked(&res.spec, res.obs, at)
	}
}

func (s *ManagedAlarmStore) buildEvalContextAt(now time.Time) AlarmEvalContext {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := slices.Sorted(maps.Keys(s.devices))
	devs := make([]AlarmEvalDevice, 0, len(names))
	for _, name := range names {
		dev := s.devices[name]
		devs = append(devs, AlarmEvalDevice{
			Name:                   dev.name,
			Declared:               dev.declared,
			Connected:              dev.connected,
			DeclaredAt:             dev.declaredAt,
			DeclarationGeneration:  dev.declarationGeneration,
			ConnectedInDeclaration: dev.connectedInDeclaration,
			ConnectedSince:         dev.connectedSince,
			LastDisconnectedAt:     dev.lastDisconnectedAt,
			SoftwareVersion:        dev.softwareVersion,
			ConfigDiverged:         dev.configDiverged,
			ConfigDivergedDetail:   dev.configDivergedDetail,
			ConfigRejected:         dev.configRejected,
			ConfigRejectedError:    dev.configRejectedError,
			Health:                 dev.health.Clone(),
			Inventory:              dev.inventory.Clone(),
			LatestSamples:          maps.Clone(dev.latestSamples),
			LatestSampleAt:         dev.latestSampleAt,
		})
	}
	var histCopy *ManagedHistoryHealth
	if s.historyHealth != nil {
		cp := *s.historyHealth
		cp.CorruptRecords = slices.Clone(s.historyHealth.CorruptRecords)
		histCopy = &cp
	}
	return AlarmEvalContext{
		Now:           now,
		Config:        s.cfg,
		Devices:       devs,
		HistoryHealth: histCopy,
	}
}

func (s *ManagedAlarmStore) reconcileExternalKindLocked(spec *AlarmKindSpec, observations []AlarmObservation, now time.Time) {
	want := make(map[alarmKey]AlarmObservation, len(observations))
	for _, obs := range observations {
		switch spec.Scope {
		case AlarmScopeHub:
			if obs.Device != "" {
				continue
			}
		case AlarmScopeDevice:
			if obs.Device == "" {
				continue
			}
		case AlarmScopeUnspecified:
			continue
		default:
			// Open enum: unsupported scope produces no observations.
			continue
		}
		k := alarmKey{kind: spec.Name, device: obs.Device, subject: obs.Subject}
		if _, dup := want[k]; !dup {
			want[k] = obs
		}
	}

	switch spec.Scope {
	case AlarmScopeHub:
		var toClose []alarmKey
		for k := range s.hubOpen {
			if k.kind == spec.Name {
				if _, ok := want[k]; !ok {
					toClose = append(toClose, k)
				}
			}
		}
		slices.SortFunc(toClose, compareAlarmKeys)
		for _, k := range toClose {
			s.closeHubOccurrenceLocked(k, now)
		}
		wantKeys := slices.SortedFunc(maps.Keys(want), compareAlarmKeys)
		for _, k := range wantKeys {
			obs := want[k]
			at := obs.ObservedAt.UTC()
			if at.IsZero() {
				at = now
			}
			summary := obs.Summary
			if summary == "" {
				summary = spec.Summary
			}
			s.openHubOccurrenceLocked(k, spec.Severity, summary, boundedManagedDetail(obs.Detail), at)
		}

	case AlarmScopeDevice:
		for _, name := range slices.Sorted(maps.Keys(s.devices)) {
			dev := s.devices[name]
			var toClose []alarmKey
			for k := range dev.open {
				if k.kind == spec.Name {
					if _, ok := want[k]; !ok {
						toClose = append(toClose, k)
					}
				}
			}
			slices.SortFunc(toClose, compareAlarmKeys)
			for _, k := range toClose {
				s.closeDeviceOccurrenceLocked(dev, k, now)
			}
		}
		wantKeys := slices.SortedFunc(maps.Keys(want), compareAlarmKeys)
		for _, k := range wantKeys {
			obs := want[k]
			dev := s.ensureDeviceLocked(k.device)
			at := obs.ObservedAt.UTC()
			if at.IsZero() {
				at = now
			}
			summary := obs.Summary
			if summary == "" {
				summary = spec.Summary
			}
			s.openDeviceOccurrenceLocked(dev, k, spec.Severity, summary, boundedManagedDetail(obs.Detail), at)
		}

	case AlarmScopeUnspecified:
		// Invalid scope cannot reconcile.
	default:
		// Open enum: ignore unknown scope.
	}
}

// NextDeadline returns the earliest pending offline grace deadline across all
// declared disconnected devices that do not yet have device-offline open.
// Safe for concurrent use.
func (s *ManagedAlarmStore) NextDeadline() (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var earliest time.Time
	hasAny := false
	for _, dev := range s.devices {
		deadline, ok := s.offlineDeadlineLocked(dev)
		if !ok {
			continue
		}
		if !hasAny || deadline.Before(earliest) {
			earliest = deadline
			hasAny = true
		}
	}
	return earliest, hasAny
}

func (s *ManagedAlarmStore) offlineDeadlineLocked(dev *alarmDevice) (time.Time, bool) {
	if !dev.declared {
		return time.Time{}, false
	}
	if dev.connected {
		return time.Time{}, false
	}
	if _, open := dev.open[alarmKey{kind: AlarmKindDeviceOffline, device: dev.name}]; open {
		return time.Time{}, false
	}
	var start time.Time
	switch {
	case dev.connectedInDeclaration && !dev.lastDisconnectedAt.IsZero():
		start = dev.lastDisconnectedAt
	case !dev.declaredAt.IsZero():
		start = dev.declaredAt
	default:
		return time.Time{}, false
	}
	delay := time.Duration(s.cfg.OfflineDelaySeconds) * time.Second
	return start.Add(delay), true
}

// Tick advances time-driven alarm evaluation to now: opens device-offline for
// any declared disconnected device whose grace delay has elapsed, prunes
// expired closed episodes, evicts expired query continuation snapshots, and
// evaluates any registered external alarm kinds. Safe for concurrent use.
func (s *ManagedAlarmStore) Tick(now time.Time) {
	nowUTC := now.UTC()
	if nowUTC.IsZero() {
		nowUTC = s.currentTime()
	}

	s.mu.Lock()
	names := slices.Sorted(maps.Keys(s.devices))
	var tb textbuf.Buffer
	for _, name := range names {
		dev := s.devices[name]
		deadline, ok := s.offlineDeadlineLocked(dev)
		if !ok {
			continue
		}
		if deadline.After(nowUTC) {
			continue
		}
		var detail string
		if dev.connectedInDeclaration && !dev.lastDisconnectedAt.IsZero() {
			detail = tb.Reset().Str("disconnected-at=").Str(dev.lastDisconnectedAt.Format(time.RFC3339)).Str(" offline-delay=").Int(int64(s.cfg.OfflineDelaySeconds)).Byte('s').String()
		} else {
			detail = tb.Reset().Str("declared-at=").Str(dev.declaredAt.Format(time.RFC3339)).Str(" offline-delay=").Int(int64(s.cfg.OfflineDelaySeconds)).Byte('s').String()
		}
		s.openDeviceOccurrenceLocked(
			dev,
			alarmKey{kind: AlarmKindDeviceOffline, device: name},
			AlarmSeverityCritical,
			"Device has no managed connection",
			detail,
			nowUTC,
		)
	}

	s.pruneAllLocked(nowUTC)
	s.evictExpiredSnapshotsLocked(nowUTC)
	s.mu.Unlock()

	s.evaluateExternalKindsAt(nowUTC)
}

// Health returns a point-in-time health snapshot of the hub alarm store for
// doctor reporting. Safe for concurrent use.
func (s *ManagedAlarmStore) Health() ManagedAlarmHealth {
	s.mu.Lock()
	defer s.mu.Unlock()

	degraded, reason := s.degradedStatusLocked()
	openCount := len(s.hubOpen)
	retainedClosed := len(s.hubClosed)
	truncated := s.hubTruncated
	for _, dev := range s.devices {
		openCount += len(dev.open)
		retainedClosed += len(dev.closed)
		if dev.truncated {
			truncated = true
		}
	}
	return ManagedAlarmHealth{
		Degraded:         degraded,
		DegradedReason:   reason,
		WriteFailed:      s.writeFailed,
		LastWriteError:   s.lastWriteErr,
		LastWriteOKAt:    s.lastWriteOKAt,
		LastWriteFailAt:  s.lastWriteFailAt,
		CorruptShards:    s.corruptShards,
		LastCorruptError: s.lastCorruptErr,
		OpenCount:        openCount,
		RetainedClosed:   retainedClosed,
		Truncated:        truncated,
	}
}

func (s *ManagedAlarmStore) degradedStatusLocked() (bool, string) {
	if !s.writeFailed && s.corruptShards == 0 {
		return false, ""
	}
	var tb textbuf.Buffer
	if s.writeFailed {
		tb.Str("alarm persistence failed: ").Str(s.lastWriteErr)
	}
	if s.corruptShards > 0 {
		if tb.Len() > 0 {
			tb.Str("; ")
		}
		tb.Str("skipped ").Int(int64(s.corruptShards)).Str(" corrupt alarm shard(s): ").Str(s.lastCorruptErr)
	}
	return true, tb.String()
}

// -----------------------------------------------------------------------------
// Occurrence lifecycle helpers (caller MUST hold s.mu)
// -----------------------------------------------------------------------------

func (s *ManagedAlarmStore) ensureDeviceLocked(name string) *alarmDevice {
	if existing := s.devices[name]; existing != nil {
		return existing
	}
	gen := s.deviceGen[name] + 1
	s.deviceGen[name] = gen
	delete(s.pendingDeletes, name)
	dev := &alarmDevice{
		name:                 name,
		generation:           gen,
		resourceQualifySince: make(map[string]time.Time),
		open:                 make(map[alarmKey]*AlarmOccurrence),
	}
	s.devices[name] = dev
	return dev
}

func (s *ManagedAlarmStore) markDeviceDirtyLocked(dev *alarmDevice) {
	dev.mutationSeq++
	dev.dirty = true
	s.wakeWriter()
}

func (s *ManagedAlarmStore) markHubDirtyLocked() {
	s.hubMutationSeq++
	s.hubDirty = true
	s.wakeWriter()
}

func (s *ManagedAlarmStore) nextOccurrenceIDLocked() string {
	s.nextSeq++
	s.markHubDirtyLocked()
	return formatSeqID(alarmIDPrefix, s.nextSeq)
}

func (s *ManagedAlarmStore) openDeviceOccurrenceLocked(dev *alarmDevice, key alarmKey, severity AlarmSeverity, summary, detail string, at time.Time) {
	if existing := dev.open[key]; existing != nil {
		if existing.Summary == summary && existing.Detail == detail && existing.Severity == severity {
			return
		}
		existing.Summary = summary
		existing.Detail = detail
		existing.Severity = severity
		s.markDeviceDirtyLocked(dev)
		return
	}
	id := s.nextOccurrenceIDLocked()
	dev.open[key] = &AlarmOccurrence{
		ID:       id,
		Kind:     key.kind,
		Severity: severity,
		Scope:    AlarmScopeDevice,
		Device:   dev.name,
		Subject:  key.subject,
		Summary:  summary,
		Detail:   detail,
		Since:    at.UTC(),
	}
	s.markDeviceDirtyLocked(dev)
}

func (s *ManagedAlarmStore) closeDeviceOccurrenceLocked(dev *alarmDevice, key alarmKey, at time.Time) {
	existing := dev.open[key]
	if existing == nil {
		return
	}
	delete(dev.open, key)
	clearedAt := at.UTC()
	if clearedAt.Before(existing.Since) {
		clearedAt = existing.Since
	}
	dev.closed = append(dev.closed, AlarmHistoryEvent{
		ID:        existing.ID,
		Kind:      existing.Kind,
		Severity:  existing.Severity,
		Scope:     existing.Scope,
		Device:    existing.Device,
		Subject:   existing.Subject,
		Summary:   existing.Summary,
		Detail:    existing.Detail,
		Opened:    existing.Since,
		Cleared:   true,
		ClearedAt: clearedAt,
	})
	s.markDeviceDirtyLocked(dev)
	s.pruneDeviceLocked(dev, clearedAt)
	s.enforceHubEpisodeCapLocked()
}

func (s *ManagedAlarmStore) closeUndeclaredDeviceAlarmsLocked(dev *alarmDevice, at time.Time) {
	var toClose []alarmKey
	for k := range dev.open {
		switch k.kind {
		case AlarmKindDeviceOffline,
			AlarmKindSoftwareBehind,
			AlarmKindComponentDown,
			AlarmKindComponentDegraded,
			AlarmKindResourceHigh,
			AlarmKindConfigRejected:
			toClose = append(toClose, k)
		}
	}
	slices.SortFunc(toClose, compareAlarmKeys)
	for _, k := range toClose {
		s.closeDeviceOccurrenceLocked(dev, k, at)
	}
	clear(dev.resourceQualifySince)
}

func (s *ManagedAlarmStore) closeSettlingOccurrencesLocked(dev *alarmDevice, at time.Time) {
	var toClose []alarmKey
	for k := range dev.open {
		if k.kind == AlarmKindResourceHigh {
			toClose = append(toClose, k)
			continue
		}
		if spec, ok := s.registry.Lookup(k.kind); ok && spec.Settles {
			toClose = append(toClose, k)
		}
	}
	slices.SortFunc(toClose, compareAlarmKeys)
	for _, k := range toClose {
		s.closeDeviceOccurrenceLocked(dev, k, at)
	}
}

func (s *ManagedAlarmStore) openHubOccurrenceLocked(key alarmKey, severity AlarmSeverity, summary, detail string, at time.Time) {
	if existing := s.hubOpen[key]; existing != nil {
		if existing.Summary == summary && existing.Detail == detail && existing.Severity == severity {
			return
		}
		existing.Summary = summary
		existing.Detail = detail
		existing.Severity = severity
		s.markHubDirtyLocked()
		return
	}
	id := s.nextOccurrenceIDLocked()
	s.hubOpen[key] = &AlarmOccurrence{
		ID:       id,
		Kind:     key.kind,
		Severity: severity,
		Scope:    AlarmScopeHub,
		Subject:  key.subject,
		Summary:  summary,
		Detail:   detail,
		Since:    at.UTC(),
	}
	s.markHubDirtyLocked()
}

func (s *ManagedAlarmStore) closeHubOccurrenceLocked(key alarmKey, at time.Time) {
	existing := s.hubOpen[key]
	if existing == nil {
		return
	}
	delete(s.hubOpen, key)
	clearedAt := at.UTC()
	if clearedAt.Before(existing.Since) {
		clearedAt = existing.Since
	}
	s.hubClosed = append(s.hubClosed, AlarmHistoryEvent{
		ID:        existing.ID,
		Kind:      existing.Kind,
		Severity:  existing.Severity,
		Scope:     existing.Scope,
		Subject:   existing.Subject,
		Summary:   existing.Summary,
		Detail:    existing.Detail,
		Opened:    existing.Since,
		Cleared:   true,
		ClearedAt: clearedAt,
	})
	s.markHubDirtyLocked()
	s.pruneHubClosedLocked(clearedAt)
	s.enforceHubEpisodeCapLocked()
}

func (s *ManagedAlarmStore) wakeWriter() {
	select {
	case s.writeWake <- struct{}{}:
	default:
	}
}

func (s *ManagedAlarmStore) wakeDeadline() {
	select {
	case s.deadlineWake <- struct{}{}:
	default:
	}
}

// -----------------------------------------------------------------------------
// Authorized queries and bounded snapshot continuation (AC-13)
// -----------------------------------------------------------------------------

// QueryOpen returns one bounded, deterministically sorted page of open alarms
// visible to scope. Scope authorization runs before filtering, sorting, and
// counting. Counts and Total always cover the complete authorized snapshot.
// Continuation cursors are bound to the snapshot, filters, and caller scope;
// narrowed permissions, changed filters, or expired snapshots return an
// *opererr.Error instructing an explicit restart. Safe for concurrent use.
func (s *ManagedAlarmStore) QueryOpen(scope FleetDeviceScope, query AlarmListQuery) (AlarmListPage, error) {
	var severityFilter AlarmSeverity
	if query.Severity != "" {
		parsed, err := ParseAlarmSeverity(query.Severity)
		if err != nil {
			var tb textbuf.Buffer
			what := tb.Reset().Str("unknown alarm severity ").Quoted(query.Severity).String()
			return AlarmListPage{}, opererr.New(what,
				"alarm severity must be critical, warning, or informational",
				"use severity critical, warning, or informational")
		}
		severityFilter = parsed
	}
	if query.Kind != "" {
		if _, ok := s.registry.Lookup(query.Kind); !ok {
			var tb textbuf.Buffer
			what := tb.Reset().Str("unknown alarm kind ").Quoted(query.Kind).String()
			return AlarmListPage{}, opererr.New(what,
				"the alarm kind is not registered on this hub",
				"omit kind or choose a registered alarm kind")
		}
	}
	if query.Limit < 0 {
		return AlarmListPage{}, opererr.New("alarm query limit cannot be negative",
			"page limit must be between 1 and 1024",
			"omit limit or set a positive page limit up to 1024")
	}
	limit := query.Limit
	if limit == 0 || limit > AlarmPageMax {
		limit = AlarmPageMax
	}

	now := s.currentTime()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.evictExpiredSnapshotsLocked(now)
	degraded, degradedReason := s.degradedStatusLocked()

	if query.Cursor != "" {
		return s.queryOpenContinuationLocked(scope, severityFilter, query, limit, now, degraded, degradedReason)
	}

	hubAllowed := s.allowsHubScopeLocked(scope)
	rows := make([]AlarmOccurrence, 0)

	if hubAllowed && query.Device == "" {
		for _, occ := range s.hubOpen {
			if matchesOpenFilters(occ, severityFilter, query.Kind, query.Device) {
				rows = append(rows, *occ)
			}
		}
	}

	authorizedDevices := make([]string, 0, len(s.devices))
	for _, name := range slices.Sorted(maps.Keys(s.devices)) {
		if !scope.AllowsDevice(name) {
			continue
		}
		authorizedDevices = append(authorizedDevices, name)
		if query.Device != "" && name != query.Device {
			continue
		}
		dev := s.devices[name]
		for _, occ := range dev.open {
			if matchesOpenFilters(occ, severityFilter, query.Kind, query.Device) {
				rows = append(rows, *occ)
			}
		}
	}

	slices.SortFunc(rows, compareAlarmOccurrences)
	counts := computeAlarmCounts(rows)

	s.snapshotSeq++
	snapID := formatSeqID(alarmSnapshotPrefix, s.snapshotSeq)

	end := min(limit, len(rows))
	pageRows := make([]AlarmOccurrence, end)
	copy(pageRows, rows[:end])

	var nextCursor string
	hasMore := end < len(rows)
	if hasMore {
		s.snapshotLRUSeq++
		s.ensureSnapshotCapacityLocked(now)
		s.openSnapshots[snapID] = &alarmOpenSnapshot{
			id:                snapID,
			createdAt:         now,
			expiresAt:         now.Add(alarmSnapshotTTL),
			lastUsed:          s.snapshotLRUSeq,
			username:          scope.username,
			scopeMode:         scope.mode,
			includedHub:       hubAllowed,
			authorizedDevices: authorizedDevices,
			filterSeverity:    severityFilter,
			filterKind:        query.Kind,
			filterDevice:      query.Device,
			rows:              rows,
			counts:            counts,
		}
		nextCursor = encodeAlarmCursor(alarmCursorPayload{
			Type:       alarmCursorTypeOpen,
			SnapshotID: snapID,
			Offset:     end,
		})
	}

	return AlarmListPage{
		Alarms:         pageRows,
		Total:          len(rows),
		Counts:         counts,
		Cursor:         nextCursor,
		HasMore:        hasMore,
		SnapshotID:     snapID,
		Degraded:       degraded,
		DegradedReason: degradedReason,
	}, nil
}

func (s *ManagedAlarmStore) queryOpenContinuationLocked(scope FleetDeviceScope, severityFilter AlarmSeverity, query AlarmListQuery, limit int, now time.Time, degraded bool, degradedReason string) (AlarmListPage, error) {
	token, err := decodeAlarmCursor(query.Cursor, alarmCursorTypeOpen)
	if err != nil {
		return AlarmListPage{}, err
	}
	snap := s.openSnapshots[token.SnapshotID]
	if snap == nil {
		return AlarmListPage{}, opererr.New(
			"alarm continuation snapshot has expired or is no longer cached",
			"continuation snapshots expire after 5 minutes or when the bounded snapshot cache rotates",
			alarmRestartFix)
	}
	if !now.Before(snap.expiresAt) {
		delete(s.openSnapshots, token.SnapshotID)
		return AlarmListPage{}, opererr.New(
			"alarm continuation snapshot has expired or is no longer cached",
			"continuation snapshots expire after 5 minutes or when the bounded snapshot cache rotates",
			alarmRestartFix)
	}
	if snap.filterSeverity != severityFilter {
		return AlarmListPage{}, opererr.New(
			"alarm query filters changed between continuation pages",
			"a continuation cursor is bound to the severity, kind, and device filters of the first page",
			alarmRestartFix)
	}
	if snap.filterKind != query.Kind {
		return AlarmListPage{}, opererr.New(
			"alarm query filters changed between continuation pages",
			"a continuation cursor is bound to the severity, kind, and device filters of the first page",
			alarmRestartFix)
	}
	if snap.filterDevice != query.Device {
		return AlarmListPage{}, opererr.New(
			"alarm query filters changed between continuation pages",
			"a continuation cursor is bound to the severity, kind, and device filters of the first page",
			alarmRestartFix)
	}
	if !s.scopePermitsOpenSnapshotLocked(scope, snap) {
		return AlarmListPage{}, opererr.New(
			"caller device visibility scope changed between continuation pages",
			"the caller's current permissions no longer authorize the complete snapshot bound to this cursor",
			alarmRestartFix)
	}
	if token.Offset > len(snap.rows) {
		return AlarmListPage{}, opererr.New(
			"invalid alarm continuation cursor offset",
			"the cursor offset lies past the end of the bound snapshot",
			alarmRestartFix)
	}

	s.snapshotLRUSeq++
	snap.lastUsed = s.snapshotLRUSeq

	end := min(token.Offset+limit, len(snap.rows))
	pageRows := make([]AlarmOccurrence, end-token.Offset)
	copy(pageRows, snap.rows[token.Offset:end])

	var nextCursor string
	hasMore := end < len(snap.rows)
	if hasMore {
		nextCursor = encodeAlarmCursor(alarmCursorPayload{
			Type:       alarmCursorTypeOpen,
			SnapshotID: snap.id,
			Offset:     end,
		})
	}

	return AlarmListPage{
		Alarms:         pageRows,
		Total:          len(snap.rows),
		Counts:         snap.counts,
		Cursor:         nextCursor,
		HasMore:        hasMore,
		SnapshotID:     snap.id,
		Degraded:       degraded,
		DegradedReason: degradedReason,
	}, nil
}

// QueryHistory returns one bounded, newest-first page of open and closed alarm
// occurrences for query.Device within the retention period. Safe for concurrent
// use.
func (s *ManagedAlarmStore) QueryHistory(scope FleetDeviceScope, query AlarmHistoryQuery) (AlarmHistoryPage, error) {
	if query.Device == "" {
		return AlarmHistoryPage{}, opererr.New(
			"device name is required for alarm history",
			"alarm history queries target one managed device",
			"specify a device name: show fleet alarm history device <name>")
	}
	if !scope.AllowsDevice(query.Device) {
		var tb textbuf.Buffer
		what := tb.Reset().Str("access denied for alarm history of device ").Quoted(query.Device).String()
		return AlarmHistoryPage{}, opererr.New(
			what,
			"caller device visibility scope does not permit reading this device",
			"query an authorized device or request access to this device")
	}
	if query.Limit < 0 {
		return AlarmHistoryPage{}, opererr.New(
			"alarm history limit cannot be negative",
			"page limit must be between 1 and 512",
			"omit limit or set a positive page limit up to 512")
	}
	limit := query.Limit
	if limit == 0 || limit > AlarmHistoryPageMax {
		limit = AlarmHistoryPageMax
	}

	now := s.currentTime()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.evictExpiredSnapshotsLocked(now)
	degraded, degradedReason := s.degradedStatusLocked()

	if query.Cursor != "" {
		return s.queryHistoryContinuationLocked(scope, query, limit, now, degraded, degradedReason)
	}

	horizon := now.Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
	retainedSince := horizon
	truncated := false

	var events []AlarmHistoryEvent
	if dev := s.devices[query.Device]; dev != nil {
		s.pruneDeviceLocked(dev, now)
		events = make([]AlarmHistoryEvent, 0, len(dev.open)+len(dev.closed))
		for _, occ := range dev.open {
			events = append(events, AlarmHistoryEvent{
				ID:       occ.ID,
				Kind:     occ.Kind,
				Severity: occ.Severity,
				Scope:    occ.Scope,
				Device:   occ.Device,
				Subject:  occ.Subject,
				Summary:  occ.Summary,
				Detail:   occ.Detail,
				Opened:   occ.Since,
				Cleared:  false,
			})
		}
		events = append(events, dev.closed...)
		if dev.truncated && !dev.earliestRetained.Before(horizon) {
			truncated = true
			if dev.earliestRetained.After(retainedSince) {
				retainedSince = dev.earliestRetained
			}
		}
	}

	slices.SortFunc(events, compareAlarmHistoryEvents)

	s.snapshotSeq++
	snapID := formatSeqID(alarmSnapshotPrefix, s.snapshotSeq)

	end := min(limit, len(events))
	pageEvents := make([]AlarmHistoryEvent, end)
	copy(pageEvents, events[:end])

	var nextCursor string
	hasMore := end < len(events)
	if hasMore {
		s.snapshotLRUSeq++
		s.ensureSnapshotCapacityLocked(now)
		s.historySnapshots[snapID] = &alarmHistorySnapshot{
			id:            snapID,
			createdAt:     now,
			expiresAt:     now.Add(alarmSnapshotTTL),
			lastUsed:      s.snapshotLRUSeq,
			username:      scope.username,
			scopeMode:     scope.mode,
			device:        query.Device,
			events:        events,
			retainedSince: retainedSince,
			truncated:     truncated,
		}
		nextCursor = encodeAlarmCursor(alarmCursorPayload{
			Type:       alarmCursorTypeHistory,
			SnapshotID: snapID,
			Offset:     end,
		})
	}

	return AlarmHistoryPage{
		Device:         query.Device,
		Events:         pageEvents,
		Total:          len(events),
		Cursor:         nextCursor,
		HasMore:        hasMore,
		SnapshotID:     snapID,
		RetainedSince:  retainedSince,
		Truncated:      truncated,
		Degraded:       degraded,
		DegradedReason: degradedReason,
	}, nil
}

func (s *ManagedAlarmStore) queryHistoryContinuationLocked(scope FleetDeviceScope, query AlarmHistoryQuery, limit int, now time.Time, degraded bool, degradedReason string) (AlarmHistoryPage, error) {
	token, err := decodeAlarmCursor(query.Cursor, alarmCursorTypeHistory)
	if err != nil {
		return AlarmHistoryPage{}, err
	}
	snap := s.historySnapshots[token.SnapshotID]
	if snap == nil {
		return AlarmHistoryPage{}, opererr.New(
			"alarm history continuation snapshot has expired or is no longer cached",
			"continuation snapshots expire after 5 minutes or when the bounded snapshot cache rotates",
			alarmRestartFix)
	}
	if !now.Before(snap.expiresAt) {
		delete(s.historySnapshots, token.SnapshotID)
		return AlarmHistoryPage{}, opererr.New(
			"alarm history continuation snapshot has expired or is no longer cached",
			"continuation snapshots expire after 5 minutes or when the bounded snapshot cache rotates",
			alarmRestartFix)
	}
	if snap.device != query.Device {
		return AlarmHistoryPage{}, opererr.New(
			"alarm history device changed between continuation pages",
			"a continuation cursor is bound to the device named on the first page",
			alarmRestartFix)
	}
	if snap.username != scope.username {
		return AlarmHistoryPage{}, opererr.New(
			"caller device visibility scope changed between continuation pages",
			"the caller's current permissions no longer match the snapshot bound to this cursor",
			alarmRestartFix)
	}
	if snap.scopeMode != scope.mode {
		return AlarmHistoryPage{}, opererr.New(
			"caller device visibility scope changed between continuation pages",
			"the caller's current permissions no longer match the snapshot bound to this cursor",
			alarmRestartFix)
	}
	if !scope.AllowsDevice(snap.device) {
		return AlarmHistoryPage{}, opererr.New(
			"caller device visibility scope changed between continuation pages",
			"the caller's current permissions no longer match the snapshot bound to this cursor",
			alarmRestartFix)
	}
	if token.Offset > len(snap.events) {
		return AlarmHistoryPage{}, opererr.New(
			"invalid alarm history continuation cursor offset",
			"the cursor offset lies past the end of the bound snapshot",
			alarmRestartFix)
	}

	s.snapshotLRUSeq++
	snap.lastUsed = s.snapshotLRUSeq

	end := min(token.Offset+limit, len(snap.events))
	pageEvents := make([]AlarmHistoryEvent, end-token.Offset)
	copy(pageEvents, snap.events[token.Offset:end])

	var nextCursor string
	hasMore := end < len(snap.events)
	if hasMore {
		nextCursor = encodeAlarmCursor(alarmCursorPayload{
			Type:       alarmCursorTypeHistory,
			SnapshotID: snap.id,
			Offset:     end,
		})
	}

	return AlarmHistoryPage{
		Device:         snap.device,
		Events:         pageEvents,
		Total:          len(snap.events),
		Cursor:         nextCursor,
		HasMore:        hasMore,
		SnapshotID:     snap.id,
		RetainedSince:  snap.retainedSince,
		Truncated:      snap.truncated,
		Degraded:       degraded,
		DegradedReason: degradedReason,
	}, nil
}

// allowsHubScopeLocked reports whether scope is permitted to read hub-scoped
// alarms (such as history-full). Unrestricted (no-RBAC) scopes are allowed;
// zero/deny-all scopes fail closed; RBAC-evaluated scopes are allowed only when
// the caller is authorized for "show fleet alarm" and holds unrestricted device
// read visibility (verified against disjoint synthetic device probes so an
// empty fleet never fails open, as well as every known device).
func (s *ManagedAlarmStore) allowsHubScopeLocked(scope FleetDeviceScope) bool {
	switch scope.mode {
	case fleetScopeAllowAll:
		return true
	case fleetScopeDenyAll:
		return false
	case fleetScopeEvaluate:
		if scope.scopeAuth == nil {
			return false
		}
		if !scope.scopeAuth.AuthorizeLocalScope(scope.username, alarmShowCommand, nil, true) {
			return false
		}
		if !scope.AllowsDevice(alarmHubScopeProbeAlpha) {
			return false
		}
		if !scope.AllowsDevice(alarmHubScopeProbeDigit) {
			return false
		}
		for name := range s.devices {
			if !scope.AllowsDevice(name) {
				return false
			}
		}
		return true
	default:
		panic("BUG: server: unknown fleetScopeMode")
	}
}

func (s *ManagedAlarmStore) scopePermitsOpenSnapshotLocked(scope FleetDeviceScope, snap *alarmOpenSnapshot) bool {
	if snap.username != scope.username {
		return false
	}
	if snap.scopeMode != scope.mode {
		return false
	}
	if snap.includedHub && !s.allowsHubScopeLocked(scope) {
		return false
	}
	if snap.filterDevice != "" && !scope.AllowsDevice(snap.filterDevice) {
		return false
	}
	for _, devName := range snap.authorizedDevices {
		if !scope.AllowsDevice(devName) {
			return false
		}
	}
	for i := range snap.rows {
		devName := snap.rows[i].Device
		if devName == "" {
			if !s.allowsHubScopeLocked(scope) {
				return false
			}
			continue
		}
		if !scope.AllowsDevice(devName) {
			return false
		}
	}
	return true
}

func (s *ManagedAlarmStore) evictExpiredSnapshotsLocked(now time.Time) {
	for id, snap := range s.openSnapshots {
		if !now.Before(snap.expiresAt) {
			delete(s.openSnapshots, id)
		}
	}
	for id, snap := range s.historySnapshots {
		if !now.Before(snap.expiresAt) {
			delete(s.historySnapshots, id)
		}
	}
}

func (s *ManagedAlarmStore) ensureSnapshotCapacityLocked(now time.Time) {
	s.evictExpiredSnapshotsLocked(now)
	for len(s.openSnapshots)+len(s.historySnapshots) >= alarmSnapshotCacheMax {
		var victimID string
		var victimIsOpen bool
		var oldestUse uint64
		hasVictim := false
		for id, snap := range s.openSnapshots {
			if !hasVictim || snap.lastUsed < oldestUse {
				victimID = id
				victimIsOpen = true
				oldestUse = snap.lastUsed
				hasVictim = true
			}
		}
		for id, snap := range s.historySnapshots {
			if !hasVictim || snap.lastUsed < oldestUse {
				victimID = id
				victimIsOpen = false
				oldestUse = snap.lastUsed
				hasVictim = true
			}
		}
		if !hasVictim {
			break
		}
		if victimIsOpen {
			delete(s.openSnapshots, victimID)
		} else {
			delete(s.historySnapshots, victimID)
		}
	}
}

func matchesOpenFilters(occ *AlarmOccurrence, severity AlarmSeverity, kind, device string) bool {
	if severity != AlarmSeverityUnspecified && occ.Severity != severity {
		return false
	}
	if kind != "" && occ.Kind != kind {
		return false
	}
	if device != "" && occ.Device != device {
		return false
	}
	return true
}

func computeAlarmCounts(rows []AlarmOccurrence) AlarmCounts {
	counts := AlarmCounts{Total: len(rows)}
	for i := range rows {
		switch rows[i].Severity {
		case AlarmSeverityCritical:
			counts.Critical++
		case AlarmSeverityWarning:
			counts.Warning++
		case AlarmSeverityInformational:
			counts.Informational++
		case AlarmSeverityUnspecified:
			// Unspecified cannot be produced for valid occurrences.
		default:
			// Open enum: ignore unknown severity in per-severity tallies.
		}
		if rows[i].HiddenBehind == "" {
			counts.Visible++
		} else {
			counts.Hidden++
		}
	}
	return counts
}

func compareAlarmKeys(a, b alarmKey) int {
	if c := cmp.Compare(a.kind, b.kind); c != 0 {
		return c
	}
	if c := cmp.Compare(a.device, b.device); c != 0 {
		return c
	}
	return cmp.Compare(a.subject, b.subject)
}

func compareAlarmOccurrences(a, b AlarmOccurrence) int {
	if c := cmp.Compare(a.Severity.Rank(), b.Severity.Rank()); c != 0 {
		return c
	}
	if c := a.Since.Compare(b.Since); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Device, b.Device); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Kind, b.Kind); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Subject, b.Subject); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

func compareAlarmHistoryEvents(a, b AlarmHistoryEvent) int {
	// Newest Opened first, tie-break by ID descending.
	if c := b.Opened.Compare(a.Opened); c != 0 {
		return c
	}
	return cmp.Compare(b.ID, a.ID)
}

func compareClosedForEviction(a, b AlarmHistoryEvent) int {
	// Oldest ClearedAt first, tie-break Opened ascending, Device ascending, ID ascending.
	if c := a.ClearedAt.Compare(b.ClearedAt); c != 0 {
		return c
	}
	if c := a.Opened.Compare(b.Opened); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Device, b.Device); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

func encodeAlarmCursor(payload alarmCursorPayload) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic("BUG: server: encode alarm cursor: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeAlarmCursor(rawCursor, expectedType string) (alarmCursorPayload, error) {
	if len(rawCursor) > alarmCursorBytesMax {
		return alarmCursorPayload{}, opererr.New(
			"invalid alarm continuation cursor",
			"the cursor exceeds the maximum length",
			alarmRestartFix)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(rawCursor)
	if err != nil {
		return alarmCursorPayload{}, opererr.New(
			"invalid alarm continuation cursor",
			"the cursor is not valid base64url",
			alarmRestartFix)
	}
	var payload alarmCursorPayload
	dec := json.NewDecoder(bytes.NewReader(decoded))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return alarmCursorPayload{}, opererr.New(
			"invalid alarm continuation cursor",
			"the cursor payload could not be decoded",
			alarmRestartFix)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return alarmCursorPayload{}, opererr.New(
			"invalid alarm continuation cursor",
			"the cursor payload contains unexpected trailing data",
			alarmRestartFix)
	}
	if payload.Type != expectedType {
		return alarmCursorPayload{}, opererr.New(
			"invalid alarm continuation cursor",
			"the cursor was issued for a different alarm command",
			alarmRestartFix)
	}
	if payload.SnapshotID == "" {
		return alarmCursorPayload{}, opererr.New(
			"invalid alarm continuation cursor",
			"the cursor is missing its snapshot identifier",
			alarmRestartFix)
	}
	if payload.Offset < 0 {
		return alarmCursorPayload{}, opererr.New(
			"invalid alarm continuation cursor",
			"the cursor offset is negative",
			alarmRestartFix)
	}
	return payload, nil
}

func formatSeqID(prefix string, seq uint64) string {
	var numBuf textbuf.Buffer
	num := numBuf.Uint(seq).String()
	var tb textbuf.Buffer
	tb.Str(prefix)
	for i := len(num); i < 16; i++ {
		tb.Byte('0')
	}
	return tb.Str(num).String()
}
