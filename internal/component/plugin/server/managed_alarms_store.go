// Design: docs/architecture/fleet-alarms.md -- durable alarm shard persistence, retention, and flush loop
// Related: managed_alarms.go -- ManagedAlarmStore engine, evaluators, and queries
// Related: managed_alarms_kinds.go -- open alarm kind registry and severity/scope codecs

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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	// CodeDoctorManagedAlarmWriteFailed reports that persisting hub alarm state
	// failed and durability is degraded until a subsequent flush succeeds.
	CodeDoctorManagedAlarmWriteFailed = "doctor-managed-alarm-write-failed"
	// CodeDoctorManagedAlarmCorrupt reports that a persisted hub alarm shard
	// failed size, version, or schema validation on load and was skipped.
	CodeDoctorManagedAlarmCorrupt = "doctor-managed-alarm-corrupt"

	alarmPersistVersion     = 1
	alarmReportSubjectWrite = "fleet-alarms"
	alarmReportWriteFix     = "check daemon state store health and free disk space; ze show doctor"
	alarmReportCorruptFix   = "inspect ze show doctor; the next alarm transition replaces the damaged shard"
)

type persistedAlarmDeviceShard struct {
	Version                int                 `json:"version"`
	Device                 string              `json:"device"`
	Declared               bool                `json:"declared"`
	DeclaredAt             time.Time           `json:"declared-at,omitzero"`
	DeclarationGeneration  uint64              `json:"declaration-generation,omitempty"`
	ConnectedInDeclaration bool                `json:"connected-in-declaration,omitempty"`
	LastDisconnectedAt     time.Time           `json:"last-disconnected-at,omitzero"`
	Open                   []AlarmOccurrence   `json:"open,omitempty"`
	Closed                 []AlarmHistoryEvent `json:"closed,omitempty"`
	EarliestRetained       time.Time           `json:"earliest-retained,omitzero"`
	Truncated              bool                `json:"truncated,omitempty"`
}

type persistedAlarmHubShard struct {
	Version          int                 `json:"version"`
	NextSeq          uint64              `json:"next-seq"`
	Open             []AlarmOccurrence   `json:"open,omitempty"`
	Closed           []AlarmHistoryEvent `json:"closed,omitempty"`
	EarliestRetained time.Time           `json:"earliest-retained,omitzero"`
	Truncated        bool                `json:"truncated,omitempty"`
}

func registerAlarmDoctorCodes() {
	alarmDoctorCodes := []diagnostic.CodeMeta{
		{
			Code:  CodeDoctorManagedAlarmWriteFailed,
			Title: "Managed fleet alarm state write failed",
			Description: "The hub could not persist one or more fleet alarm shards to its state store. " +
				"Live alarms continue to be derived and served from memory while dirty shards remain queued for retry.",
			Examples: []string{exampleShowDoctor, fixShowWarnings, "ze show fleet alarm"},
		},
		{
			Code:  CodeDoctorManagedAlarmCorrupt,
			Title: "Stored managed fleet alarm shard could not be decoded",
			Description: "A persisted fleet alarm shard failed byte-bound, version, or schema validation on startup load and was skipped while healthy shards remained readable. " +
				"The next alarm state change replaces the damaged shard.",
			Examples: []string{exampleShowDoctor, fixShowWarnings, "ze show fleet alarm"},
		},
	}
	for _, meta := range alarmDoctorCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: server: alarm doctor code registration refused: " + err.Error())
		}
	}
}

func alarmDeviceKey(name string) string {
	return zefs.FleetAlarmDevicePrefix + name
}

func parseAlarmDeviceKey(key string) (string, bool) {
	if !strings.HasPrefix(key, zefs.FleetAlarmDevicePrefix) {
		return "", false
	}
	name := strings.TrimPrefix(key, zefs.FleetAlarmDevicePrefix)
	if name == "" {
		return "", false
	}
	if strings.Contains(name, "/") {
		return "", false
	}
	if !fleet.ValidDeviceKeyName(name) {
		return "", false
	}
	return name, true
}

func parseAlarmSeqID(id string) (uint64, bool) {
	if !strings.HasPrefix(id, alarmIDPrefix) {
		return 0, false
	}
	return parseUnsignedDecimal(strings.TrimPrefix(id, alarmIDPrefix))
}

// OnDeviceForgotten removes all in-memory alarm state for name and queues
// durable removal of its meta/fleet/alarm/device/<name> shard for the
// background writer or FlushFinal. It performs no storage I/O directly.
// Safe for concurrent use.
func (s *ManagedAlarmStore) OnDeviceForgotten(name string) {
	if name == "" {
		return
	}
	s.mu.Lock()
	delete(s.devices, name)
	gen := s.deviceGen[name] + 1
	s.deviceGen[name] = gen
	s.pendingDeletes[name] = gen
	s.mu.Unlock()

	s.wakeDeadline()
	s.wakeWriter()
}

// LoadFromStore loads persisted hub and per-device alarm shards from the
// borrowed store. Oversized or corrupt shards are skipped, counted in
// ManagedAlarmHealth, and reported on the operational report bus without
// losing healthy shards. Safe for concurrent use.
func (s *ManagedAlarmStore) LoadFromStore() error {
	// refuse-by-default: guards ManagedServer.Start -- a server built without storage runs memory-only alarms
	if s.store == nil {
		return nil
	}

	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	hubKey := zefs.KeyFleetAlarmHub.Pattern
	rawHub, err := s.store.ReadKey(hubKey)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.recordCorruptShard(hubKey, err)
		}
	} else {
		shard, decErr := decodeAlarmHubShard(rawHub)
		if decErr != nil {
			s.recordCorruptShard(hubKey, decErr)
		} else {
			s.installLoadedHubShard(&shard)
		}
	}

	keys, listErr := s.store.ListKeys(zefs.FleetAlarmDevicePrefix)
	if listErr != nil {
		return fmt.Errorf("list fleet alarm device shards: %w", listErr)
	}
	slices.Sort(keys)

	for _, key := range keys {
		name, ok := parseAlarmDeviceKey(key)
		if !ok {
			s.recordCorruptShard(key, errors.New("invalid alarm device shard key name"))
			continue
		}
		raw, readErr := s.store.ReadKey(key)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			s.recordCorruptShard(key, readErr)
			continue
		}
		shard, decErr := decodeAlarmDeviceShard(name, raw)
		if decErr != nil {
			s.recordCorruptShard(key, decErr)
			continue
		}
		s.installLoadedDeviceShard(&shard)
	}

	now := s.currentTime()
	s.mu.Lock()
	s.pruneAllLocked(now)
	s.mu.Unlock()
	return nil
}

func (s *ManagedAlarmStore) installLoadedHubShard(shard *persistedAlarmHubShard) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if shard.NextSeq > s.nextSeq {
		s.nextSeq = shard.NextSeq
	}
	s.hubOpen = make(map[alarmKey]*AlarmOccurrence, len(shard.Open))
	for i := range shard.Open {
		occ := shard.Open[i]
		occ.Since = occ.Since.UTC()
		k := alarmKey{kind: occ.Kind, device: "", subject: occ.Subject}
		cp := occ
		s.hubOpen[k] = &cp
		if seq, ok := parseAlarmSeqID(occ.ID); ok && seq > s.nextSeq {
			s.nextSeq = seq
		}
	}
	s.hubClosed = make([]AlarmHistoryEvent, len(shard.Closed))
	for i := range shard.Closed {
		ev := shard.Closed[i]
		ev.Opened = ev.Opened.UTC()
		ev.ClearedAt = ev.ClearedAt.UTC()
		s.hubClosed[i] = ev
		if seq, ok := parseAlarmSeqID(ev.ID); ok && seq > s.nextSeq {
			s.nextSeq = seq
		}
	}
	s.hubEarliest = shard.EarliestRetained.UTC()
	s.hubTruncated = shard.Truncated
	s.hubDirty = false
}

func (s *ManagedAlarmStore) installLoadedDeviceShard(shard *persistedAlarmDeviceShard) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dev := s.ensureDeviceLocked(shard.Device)
	dev.declared = shard.Declared
	dev.declaredAt = shard.DeclaredAt.UTC()
	dev.declarationGeneration = shard.DeclarationGeneration
	dev.connectedInDeclaration = shard.ConnectedInDeclaration
	dev.lastDisconnectedAt = shard.LastDisconnectedAt.UTC()
	dev.earliestRetained = shard.EarliestRetained.UTC()
	dev.truncated = shard.Truncated

	dev.open = make(map[alarmKey]*AlarmOccurrence, len(shard.Open))
	for i := range shard.Open {
		occ := shard.Open[i]
		occ.Since = occ.Since.UTC()
		k := alarmKey{kind: occ.Kind, device: shard.Device, subject: occ.Subject}
		cp := occ
		dev.open[k] = &cp
		if seq, ok := parseAlarmSeqID(occ.ID); ok && seq > s.nextSeq {
			s.nextSeq = seq
		}
	}
	dev.closed = make([]AlarmHistoryEvent, len(shard.Closed))
	for i := range shard.Closed {
		ev := shard.Closed[i]
		ev.Opened = ev.Opened.UTC()
		ev.ClearedAt = ev.ClearedAt.UTC()
		dev.closed[i] = ev
		if seq, ok := parseAlarmSeqID(ev.ID); ok && seq > s.nextSeq {
			s.nextSeq = seq
		}
	}
	dev.dirty = false
}

func (s *ManagedAlarmStore) recordCorruptShard(key string, err error) {
	errText := boundedManagedDetail(err.Error())
	managedLogger().Warn("fleet alarm shard could not be decoded and was skipped",
		"code", CodeDoctorManagedAlarmCorrupt, "key", key, "error", err)

	s.mu.Lock()
	s.corruptShards++
	s.lastCorruptErr = errText
	s.mu.Unlock()

	var tb textbuf.Buffer
	msg := tb.Str("fleet alarm shard ").Quoted(key).Str(" could not be decoded and was skipped: ").Str(errText).String()
	report.RaiseWarningFix(managedReportSource, CodeDoctorManagedAlarmCorrupt, fitSubject(key),
		report.FitMessage(msg), alarmReportCorruptFix)
}

func (s *ManagedAlarmStore) recordWriteFailure(err error) {
	now := s.currentTime()
	errText := boundedManagedDetail(err.Error())
	managedLogger().Warn("fleet alarm persistence failed; dirty shards retained for retry",
		"code", CodeDoctorManagedAlarmWriteFailed, "error", err)

	s.mu.Lock()
	s.writeFailed = true
	s.lastWriteFailAt = now
	s.lastWriteErr = errText
	s.mu.Unlock()

	var tb textbuf.Buffer
	msg := tb.Str("fleet alarm state could not be persisted: ").Str(errText).String()
	report.RaiseWarningFix(managedReportSource, CodeDoctorManagedAlarmWriteFailed, alarmReportSubjectWrite,
		report.FitMessage(msg), alarmReportWriteFix)
}

func (s *ManagedAlarmStore) recordWriteSuccess() {
	now := s.currentTime()
	s.mu.Lock()
	wasFailed := s.writeFailed
	s.writeFailed = false
	s.lastWriteOKAt = now
	s.lastWriteErr = ""
	s.mu.Unlock()

	if wasFailed {
		report.ClearWarning(managedReportSource, CodeDoctorManagedAlarmWriteFailed, alarmReportSubjectWrite)
	}
}

// FlushOnce persists all pending device deletions and dirty hub/device alarm
// shards without holding s.mu across storage I/O. Failed writes leave their
// shards dirty for retry, set Degraded, and raise a report-bus warning.
// Safe for concurrent use.
func (s *ManagedAlarmStore) FlushOnce() error {
	// refuse-by-default: guards RunLoop -- memory-only alarm stores perform no storage writes
	if s.store == nil {
		s.mu.Lock()
		clear(s.pendingDeletes)
		s.hubDirty = false
		for _, dev := range s.devices {
			dev.dirty = false
		}
		s.mu.Unlock()
		return nil
	}

	s.ioMu.Lock()
	defer s.ioMu.Unlock()

	var errs []error
	if err := s.drainPendingDeletesLockedIO(); err != nil {
		errs = append(errs, err)
	}
	if err := s.flushHubShardLockedIO(); err != nil {
		errs = append(errs, err)
	}

	s.mu.Lock()
	dirtyNames := make([]string, 0, len(s.devices))
	for name, dev := range s.devices {
		if dev.dirty {
			dirtyNames = append(dirtyNames, name)
		}
	}
	s.mu.Unlock()
	slices.Sort(dirtyNames)

	for _, name := range dirtyNames {
		if err := s.drainPendingDeletesLockedIO(); err != nil {
			errs = append(errs, err)
		}
		if err := s.flushDeviceShardLockedIO(name); err != nil {
			errs = append(errs, err)
		}
	}
	if err := s.drainPendingDeletesLockedIO(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		joined := errors.Join(errs...)
		s.recordWriteFailure(joined)
		return joined
	}
	s.recordWriteSuccess()
	return nil
}

// FlushFinal drains every pending deletion and dirty alarm shard before
// shutdown releases the borrowed daemon store. Safe for concurrent use.
func (s *ManagedAlarmStore) FlushFinal() error {
	return s.FlushOnce()
}

// RunLoop runs the long-lived alarm deadline and persistence worker until ctx
// is canceled. The caller MUST call FlushFinal after RunLoop returns before
// releasing the borrowed store.
func (s *ManagedAlarmStore) RunLoop(ctx context.Context) {
	retryTicker := time.NewTicker(s.flushInterval)
	defer retryTicker.Stop()

	var deadlineTimer *time.Timer
	var deadlineCh <-chan time.Time

	armDeadline := func() {
		if deadlineTimer != nil {
			if !deadlineTimer.Stop() {
				select {
				case <-deadlineTimer.C:
				default:
				}
			}
			deadlineCh = nil
		}
		for {
			deadline, ok := s.NextDeadline()
			if !ok {
				return
			}
			now := s.currentTime()
			wait := deadline.Sub(now)
			if wait <= 0 {
				s.Tick(now)
				continue
			}
			if deadlineTimer == nil {
				deadlineTimer = time.NewTimer(wait)
			} else {
				deadlineTimer.Reset(wait)
			}
			deadlineCh = deadlineTimer.C
			return
		}
	}

	defer func() {
		if deadlineTimer != nil {
			deadlineTimer.Stop()
		}
	}()

	armDeadline()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.writeWake:
			if err := s.FlushOnce(); err != nil && ctx.Err() == nil {
				managedLogger().Warn("fleet alarm background flush failed; will retry",
					"code", CodeDoctorManagedAlarmWriteFailed, "error", err)
			}
		case <-s.deadlineWake:
			armDeadline()
		case <-deadlineCh:
			deadlineCh = nil
			s.Tick(s.currentTime())
			if err := s.FlushOnce(); err != nil && ctx.Err() == nil {
				managedLogger().Warn("fleet alarm deadline flush failed; will retry",
					"code", CodeDoctorManagedAlarmWriteFailed, "error", err)
			}
			armDeadline()
		case <-retryTicker.C:
			s.Tick(s.currentTime())
			if s.needsFlush() {
				if err := s.FlushOnce(); err != nil && ctx.Err() == nil {
					managedLogger().Warn("fleet alarm periodic flush failed; will retry",
						"code", CodeDoctorManagedAlarmWriteFailed, "error", err)
				}
			}
			armDeadline()
		}
	}
}

func (s *ManagedAlarmStore) needsFlush() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hubDirty {
		return true
	}
	if s.writeFailed {
		return true
	}
	if len(s.pendingDeletes) > 0 {
		return true
	}
	for _, dev := range s.devices {
		if dev.dirty {
			return true
		}
	}
	return false
}

func (s *ManagedAlarmStore) drainPendingDeletesLockedIO() error {
	var errs []error
	s.mu.Lock()
	targets := make(map[string]uint64, len(s.pendingDeletes))
	maps.Copy(targets, s.pendingDeletes)
	s.mu.Unlock()

	for _, name := range slices.Sorted(maps.Keys(targets)) {
		gen := targets[name]
		s.mu.Lock()
		stillPending := s.pendingDeletes[name] == gen
		s.mu.Unlock()
		if !stillPending {
			continue
		}
		key := alarmDeviceKey(name)
		if err := s.store.RemoveKey(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove fleet alarm device shard %s: %w", key, err))
			continue
		}
		s.mu.Lock()
		if s.pendingDeletes[name] == gen {
			delete(s.pendingDeletes, name)
		}
		s.mu.Unlock()
	}
	return errors.Join(errs...)
}

func (s *ManagedAlarmStore) flushHubShardLockedIO() error {
	s.mu.Lock()
	if !s.hubDirty {
		s.mu.Unlock()
		return nil
	}
	seq := s.hubMutationSeq
	shard := snapshotHubShardLocked(s)
	s.mu.Unlock()

	encoded, trimmed, err := encodeAlarmHubShard(&shard)
	if err != nil {
		return err
	}
	hubKey := zefs.KeyFleetAlarmHub.Pattern
	if err := s.store.WriteKey(hubKey, encoded); err != nil {
		return fmt.Errorf("write fleet alarm hub shard %s: %w", hubKey, err)
	}

	s.mu.Lock()
	if s.hubMutationSeq == seq {
		s.hubDirty = false
		if trimmed {
			s.hubClosed = shard.Closed
			s.hubEarliest = shard.EarliestRetained
			s.hubTruncated = shard.Truncated
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *ManagedAlarmStore) flushDeviceShardLockedIO(name string) error {
	s.mu.Lock()
	dev := s.devices[name]
	// refuse-by-default: guards FlushOnce -- a device forgotten between dirty-list capture and flush has nothing left to write
	if dev == nil {
		s.mu.Unlock()
		return nil
	}
	if !dev.dirty {
		s.mu.Unlock()
		return nil
	}
	gen := dev.generation
	seq := dev.mutationSeq
	shard := snapshotDeviceShardLocked(dev)
	s.mu.Unlock()

	encoded, trimmed, err := encodeAlarmDeviceShard(&shard)
	if err != nil {
		return err
	}

	// Verify the device was not forgotten while encoding outside the lock.
	s.mu.Lock()
	cur := s.devices[name]
	stillValid := cur != nil && cur.generation == gen
	s.mu.Unlock()
	if !stillValid {
		return nil
	}

	key := alarmDeviceKey(name)
	if err := s.store.WriteKey(key, encoded); err != nil {
		return fmt.Errorf("write fleet alarm device shard %s: %w", key, err)
	}

	s.mu.Lock()
	cur = s.devices[name]
	if cur != nil && cur.generation == gen && cur.mutationSeq == seq {
		cur.dirty = false
		if trimmed {
			cur.closed = shard.Closed
			cur.earliestRetained = shard.EarliestRetained
			cur.truncated = shard.Truncated
		}
	}
	s.mu.Unlock()
	return nil
}

func snapshotHubShardLocked(s *ManagedAlarmStore) persistedAlarmHubShard {
	open := make([]AlarmOccurrence, 0, len(s.hubOpen))
	for _, occ := range s.hubOpen {
		open = append(open, *occ)
	}
	slices.SortFunc(open, compareAlarmOccurrences)

	closed := slices.Clone(s.hubClosed)
	slices.SortFunc(closed, compareClosedForEviction)

	return persistedAlarmHubShard{
		Version:          alarmPersistVersion,
		NextSeq:          s.nextSeq,
		Open:             open,
		Closed:           closed,
		EarliestRetained: s.hubEarliest,
		Truncated:        s.hubTruncated,
	}
}

func snapshotDeviceShardLocked(dev *alarmDevice) persistedAlarmDeviceShard {
	open := make([]AlarmOccurrence, 0, len(dev.open))
	for _, occ := range dev.open {
		open = append(open, *occ)
	}
	slices.SortFunc(open, compareAlarmOccurrences)

	closed := slices.Clone(dev.closed)
	slices.SortFunc(closed, compareClosedForEviction)

	return persistedAlarmDeviceShard{
		Version:                alarmPersistVersion,
		Device:                 dev.name,
		Declared:               dev.declared,
		DeclaredAt:             dev.declaredAt,
		DeclarationGeneration:  dev.declarationGeneration,
		ConnectedInDeclaration: dev.connectedInDeclaration,
		LastDisconnectedAt:     dev.lastDisconnectedAt,
		Open:                   open,
		Closed:                 closed,
		EarliestRetained:       dev.earliestRetained,
		Truncated:              dev.truncated,
	}
}

// encodeAlarmDeviceShard marshals shard into compact JSON bounded by
// zefs.FleetAlarmDeviceValueMax. If the encoded bytes exceed the cap, oldest
// closed episodes are evicted until the shard fits while open occurrences are
// always preserved.
func encodeAlarmDeviceShard(shard *persistedAlarmDeviceShard) ([]byte, bool, error) {
	encoded, err := json.Marshal(shard)
	if err != nil {
		return nil, false, fmt.Errorf("encode fleet alarm device shard %s: %w", shard.Device, err)
	}
	if len(encoded) <= zefs.FleetAlarmDeviceValueMax {
		return encoded, false, nil
	}

	slices.SortFunc(shard.Closed, compareClosedForEviction)
	trimmed := false
	for len(encoded) > zefs.FleetAlarmDeviceValueMax && len(shard.Closed) > 0 {
		// Drop half the excess or at least one oldest episode to converge quickly.
		drop := max(1, len(shard.Closed)/4)
		for i := range drop {
			ev := shard.Closed[i]
			if ev.ClearedAt.After(shard.EarliestRetained) {
				shard.EarliestRetained = ev.ClearedAt
			}
		}
		shard.Closed = slices.Clone(shard.Closed[drop:])
		shard.Truncated = true
		trimmed = true
		encoded, err = json.Marshal(shard)
		if err != nil {
			return nil, trimmed, fmt.Errorf("encode fleet alarm device shard %s: %w", shard.Device, err)
		}
	}
	if len(encoded) > zefs.FleetAlarmDeviceValueMax {
		return nil, trimmed, fmt.Errorf("fleet alarm device shard %s size %d exceeds max %d even after evicting closed history",
			shard.Device, len(encoded), zefs.FleetAlarmDeviceValueMax)
	}
	return encoded, trimmed, nil
}

// encodeAlarmHubShard marshals shard into compact JSON bounded by
// zefs.FleetAlarmHubValueMax, evicting oldest closed hub episodes if needed
// while never dropping active hub occurrences.
func encodeAlarmHubShard(shard *persistedAlarmHubShard) ([]byte, bool, error) {
	encoded, err := json.Marshal(shard)
	if err != nil {
		return nil, false, fmt.Errorf("encode fleet alarm hub shard: %w", err)
	}
	if len(encoded) <= zefs.FleetAlarmHubValueMax {
		return encoded, false, nil
	}

	slices.SortFunc(shard.Closed, compareClosedForEviction)
	trimmed := false
	for len(encoded) > zefs.FleetAlarmHubValueMax && len(shard.Closed) > 0 {
		drop := max(1, len(shard.Closed)/4)
		for i := range drop {
			ev := shard.Closed[i]
			if ev.ClearedAt.After(shard.EarliestRetained) {
				shard.EarliestRetained = ev.ClearedAt
			}
		}
		shard.Closed = slices.Clone(shard.Closed[drop:])
		shard.Truncated = true
		trimmed = true
		encoded, err = json.Marshal(shard)
		if err != nil {
			return nil, trimmed, fmt.Errorf("encode fleet alarm hub shard: %w", err)
		}
	}
	if len(encoded) > zefs.FleetAlarmHubValueMax {
		return nil, trimmed, fmt.Errorf("fleet alarm hub shard size %d exceeds max %d even after evicting closed history",
			len(encoded), zefs.FleetAlarmHubValueMax)
	}
	return encoded, trimmed, nil
}

func decodeAlarmDeviceShard(expectedDevice string, raw []byte) (persistedAlarmDeviceShard, error) {
	if len(raw) == 0 {
		return persistedAlarmDeviceShard{}, errors.New("empty alarm device shard")
	}
	if len(raw) > zefs.FleetAlarmDeviceValueMax {
		return persistedAlarmDeviceShard{}, fmt.Errorf("alarm device shard size %d exceeds max %d", len(raw), zefs.FleetAlarmDeviceValueMax)
	}
	if !utf8.Valid(raw) {
		return persistedAlarmDeviceShard{}, errors.New("alarm device shard contains invalid UTF-8")
	}
	var shard persistedAlarmDeviceShard
	if err := decodeStrictJSON(raw, &shard); err != nil {
		return persistedAlarmDeviceShard{}, err
	}
	if shard.Version != alarmPersistVersion {
		return persistedAlarmDeviceShard{}, fmt.Errorf("unsupported alarm device shard version %d", shard.Version)
	}
	if shard.Device != expectedDevice {
		return persistedAlarmDeviceShard{}, fmt.Errorf("alarm device shard name %q does not match key device %q", shard.Device, expectedDevice)
	}
	for i := range shard.Open {
		if err := validatePersistedOccurrence(&shard.Open[i], AlarmScopeDevice, expectedDevice); err != nil {
			return persistedAlarmDeviceShard{}, err
		}
	}
	for i := range shard.Closed {
		if err := validatePersistedHistoryEvent(&shard.Closed[i], AlarmScopeDevice, expectedDevice); err != nil {
			return persistedAlarmDeviceShard{}, err
		}
	}
	return shard, nil
}

func decodeAlarmHubShard(raw []byte) (persistedAlarmHubShard, error) {
	if len(raw) == 0 {
		return persistedAlarmHubShard{}, errors.New("empty alarm hub shard")
	}
	if len(raw) > zefs.FleetAlarmHubValueMax {
		return persistedAlarmHubShard{}, fmt.Errorf("alarm hub shard size %d exceeds max %d", len(raw), zefs.FleetAlarmHubValueMax)
	}
	if !utf8.Valid(raw) {
		return persistedAlarmHubShard{}, errors.New("alarm hub shard contains invalid UTF-8")
	}
	var shard persistedAlarmHubShard
	if err := decodeStrictJSON(raw, &shard); err != nil {
		return persistedAlarmHubShard{}, err
	}
	if shard.Version != alarmPersistVersion {
		return persistedAlarmHubShard{}, fmt.Errorf("unsupported alarm hub shard version %d", shard.Version)
	}
	for i := range shard.Open {
		if err := validatePersistedOccurrence(&shard.Open[i], AlarmScopeHub, ""); err != nil {
			return persistedAlarmHubShard{}, err
		}
	}
	for i := range shard.Closed {
		if err := validatePersistedHistoryEvent(&shard.Closed[i], AlarmScopeHub, ""); err != nil {
			return persistedAlarmHubShard{}, err
		}
	}
	return shard, nil
}

func decodeStrictJSON(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return fmt.Errorf("unexpected trailing data: %w", err)
	}
	return nil
}

func validatePersistedOccurrence(occ *AlarmOccurrence, expectedScope AlarmScope, expectedDevice string) error {
	if occ.ID == "" {
		return errors.New("persisted alarm occurrence is missing id")
	}
	if occ.Kind == "" {
		return errors.New("persisted alarm occurrence is missing kind")
	}
	if !occ.Severity.Valid() {
		return fmt.Errorf("persisted alarm occurrence %q has invalid severity", occ.ID)
	}
	if occ.Scope != expectedScope {
		return fmt.Errorf("persisted alarm occurrence %q scope %s does not match %s", occ.ID, occ.Scope.String(), expectedScope.String())
	}
	if occ.Device != expectedDevice {
		return fmt.Errorf("persisted alarm occurrence %q device %q does not match %q", occ.ID, occ.Device, expectedDevice)
	}
	if occ.Since.IsZero() {
		return fmt.Errorf("persisted alarm occurrence %q is missing since timestamp", occ.ID)
	}
	return nil
}

func validatePersistedHistoryEvent(ev *AlarmHistoryEvent, expectedScope AlarmScope, expectedDevice string) error {
	if ev.ID == "" {
		return errors.New("persisted alarm history event is missing id")
	}
	if ev.Kind == "" {
		return errors.New("persisted alarm history event is missing kind")
	}
	if !ev.Severity.Valid() {
		return fmt.Errorf("persisted alarm history event %q has invalid severity", ev.ID)
	}
	if ev.Scope != expectedScope {
		return fmt.Errorf("persisted alarm history event %q scope %s does not match %s", ev.ID, ev.Scope.String(), expectedScope.String())
	}
	if ev.Device != expectedDevice {
		return fmt.Errorf("persisted alarm history event %q device %q does not match %q", ev.ID, ev.Device, expectedDevice)
	}
	if ev.Opened.IsZero() {
		return fmt.Errorf("persisted alarm history event %q is missing opened timestamp", ev.ID)
	}
	if !ev.Cleared {
		return fmt.Errorf("persisted closed alarm history event %q must have cleared=true", ev.ID)
	}
	if ev.ClearedAt.IsZero() {
		return fmt.Errorf("persisted alarm history event %q is missing cleared-at timestamp", ev.ID)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Retention horizon and episode-cap eviction (caller MUST hold s.mu)
// -----------------------------------------------------------------------------

func (s *ManagedAlarmStore) pruneAllLocked(now time.Time) {
	s.pruneHubClosedLocked(now)
	for _, name := range slices.Sorted(maps.Keys(s.devices)) {
		s.pruneDeviceLocked(s.devices[name], now)
	}
	s.enforceHubEpisodeCapLocked()
}

func (s *ManagedAlarmStore) pruneDeviceLocked(dev *alarmDevice, now time.Time) {
	if len(dev.closed) == 0 {
		return
	}
	horizon := now.Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
	kept := dev.closed[:0]
	expiredAny := false
	for i := range dev.closed {
		ev := &dev.closed[i]
		if ev.ClearedAt.Before(horizon) {
			expiredAny = true
			continue
		}
		kept = append(kept, *ev)
	}
	if expiredAny {
		dev.closed = slices.Clone(kept)
		s.markDeviceDirtyLocked(dev)
	}

	capPerDevice := max(1, s.cfg.MaxEpisodesPerDevice)
	if len(dev.closed) <= capPerDevice {
		return
	}
	slices.SortFunc(dev.closed, compareClosedForEviction)
	drop := len(dev.closed) - capPerDevice
	for i := range drop {
		if dev.closed[i].ClearedAt.After(dev.earliestRetained) {
			dev.earliestRetained = dev.closed[i].ClearedAt
		}
	}
	dev.closed = slices.Clone(dev.closed[drop:])
	dev.truncated = true
	s.markDeviceDirtyLocked(dev)
}

func (s *ManagedAlarmStore) pruneHubClosedLocked(now time.Time) {
	if len(s.hubClosed) == 0 {
		return
	}
	horizon := now.Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
	kept := s.hubClosed[:0]
	expiredAny := false
	for i := range s.hubClosed {
		ev := &s.hubClosed[i]
		if ev.ClearedAt.Before(horizon) {
			expiredAny = true
			continue
		}
		kept = append(kept, *ev)
	}
	if expiredAny {
		s.hubClosed = slices.Clone(kept)
		s.markHubDirtyLocked()
	}
}

func (s *ManagedAlarmStore) enforceHubEpisodeCapLocked() {
	total := len(s.hubClosed)
	for _, dev := range s.devices {
		total += len(dev.closed)
	}
	limit := max(1, s.cfg.MaxEpisodesHub)
	if total <= limit {
		return
	}

	type episodeRef struct {
		isHub     bool
		device    string
		idx       int
		clearedAt time.Time
		opened    time.Time
		id        string
	}
	refs := make([]episodeRef, 0, total)
	for i := range s.hubClosed {
		ev := &s.hubClosed[i]
		refs = append(refs, episodeRef{
			isHub:     true,
			idx:       i,
			clearedAt: ev.ClearedAt,
			opened:    ev.Opened,
			id:        ev.ID,
		})
	}
	for _, name := range slices.Sorted(maps.Keys(s.devices)) {
		dev := s.devices[name]
		for i := range dev.closed {
			ev := &dev.closed[i]
			refs = append(refs, episodeRef{
				device:    name,
				idx:       i,
				clearedAt: ev.ClearedAt,
				opened:    ev.Opened,
				id:        ev.ID,
			})
		}
	}

	slices.SortFunc(refs, func(a, b episodeRef) int {
		if c := a.clearedAt.Compare(b.clearedAt); c != 0 {
			return c
		}
		if c := a.opened.Compare(b.opened); c != 0 {
			return c
		}
		if c := cmp.Compare(a.device, b.device); c != 0 {
			return c
		}
		return cmp.Compare(a.id, b.id)
	})

	toDrop := total - limit
	hubDrop := make(map[int]struct{})
	devDrops := make(map[string]map[int]struct{})

	for i := range toDrop {
		ref := refs[i]
		if ref.isHub {
			hubDrop[ref.idx] = struct{}{}
			if ref.clearedAt.After(s.hubEarliest) {
				s.hubEarliest = ref.clearedAt
			}
			s.hubTruncated = true
			continue
		}
		set := devDrops[ref.device]
		if set == nil {
			set = make(map[int]struct{})
			devDrops[ref.device] = set
		}
		set[ref.idx] = struct{}{}
		if dev := s.devices[ref.device]; dev != nil {
			if ref.clearedAt.After(dev.earliestRetained) {
				dev.earliestRetained = ref.clearedAt
			}
			dev.truncated = true
		}
		s.hubTruncated = true
	}

	if len(hubDrop) > 0 {
		kept := make([]AlarmHistoryEvent, 0, len(s.hubClosed)-len(hubDrop))
		for i := range s.hubClosed {
			if _, dropped := hubDrop[i]; !dropped {
				kept = append(kept, s.hubClosed[i])
			}
		}
		s.hubClosed = kept
		s.markHubDirtyLocked()
	}

	for _, name := range slices.Sorted(maps.Keys(devDrops)) {
		dev := s.devices[name]
		if dev == nil {
			continue
		}
		dropSet := devDrops[name]
		kept := make([]AlarmHistoryEvent, 0, len(dev.closed)-len(dropSet))
		for i := range dev.closed {
			if _, dropped := dropSet[i]; !dropped {
				kept = append(kept, dev.closed[i])
			}
		}
		dev.closed = kept
		s.markDeviceDirtyLocked(dev)
	}
}
