// Design: docs/architecture/fleet-config.md -- durable managed device history
// Related: managed_registry.go -- authenticated events mark a device dirty
// Related: managed_doctor.go -- reads failures from the same device entries

package server

import (
	"bytes"
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

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

// managedRecordIO borrows the hub store until Stop. The wake channel carries
// one signal regardless of the number of events. Under the server mutex,
// pending keeps one entry per name; true means its live facts changed.
type managedRecordIO struct {
	store        storage.Storage
	pending      map[string]bool
	subscription storage.RuntimeSubscription
	failed       map[string]managedRecordFailure // Includes undeclared keys whose removal failed.
	wake         chan struct{}
	loaded       bool
	closed       bool
	saving       string // Active until Release returns or an operator deletes the key.
}

type managedRecordFailure struct {
	message string
	dirty   bool // A failed history save must be retried; a repair preserves the original bytes.
}

func newManagedRecordIO(store storage.Storage) *managedRecordIO {
	// refuse-by-default: guards NewManagedServer -- an omitted optional store creates a memory-only registry
	if store == nil {
		return nil
	}
	return &managedRecordIO{
		store: store, pending: make(map[string]bool),
		failed: make(map[string]managedRecordFailure), wake: make(chan struct{}, 1),
	}
}

func (s *ManagedServer) dirtyDeviceLocked(name string) {
	if s.recordIO == nil {
		return
	}
	device := s.devices[name]
	if device == nil {
		return // A late event has no history to persist after this device was forgotten.
	}
	if device.forgetOnDisconnect && !device.declared {
		return // Reload is already closing this forgotten session.
	}
	device.persisted = false
	s.queueDeviceRecordLocked(name, true)
}

func (s *ManagedServer) queueDeviceRecordLocked(name string, changed bool) {
	work := s.recordIO
	work.pending[name] = changed || work.pending[name]
	select {
	case work.wake <- struct{}{}:
	default:
		// The writer already has a wake for its set.
	}
}

func deviceRecordName(key string) (string, bool) {
	entry := zefs.KeyFleetDeviceRecord
	if !entry.Matches(key) {
		return "", false
	}
	name, _, _ := strings.Cut(strings.TrimPrefix(key, entry.Prefix()), "/")
	return name, true
}

// observeDeviceRecord runs in publication order under the store's runtime
// lock. It only updates memory. A later data write cannot hide an intervening
// deletion, and the next authenticated event starts fresh history.
func (s *ManagedServer) observeDeviceRecord(key string, removed bool) {
	name, ok := deviceRecordName(key)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recordIO.closed {
		return
	}
	if !s.recordIO.loaded {
		return // The initial guarded snapshot will include this change.
	}
	if removed {
		if s.recordIO.saving == name {
			s.recordIO.saving = ""
		}
		delete(s.recordIO.pending, name)
		delete(s.recordIO.failed, name)
		if device := s.devices[name]; device != nil {
			s.forgetDeviceRecordLocked(name, device)
		}
		return
	}
	s.queueDeviceRecordLocked(name, false)
}

func decodeDeviceRecord(name string, data []byte) (ManagedDeviceRecord, error) {
	var record ManagedDeviceRecord
	if !fleet.ValidDeviceKeyName(name) {
		return record, errors.New("record key contains an invalid device name")
	}
	if !utf8.Valid(data) {
		return record, errors.New("device record contains invalid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return record, errors.New("unexpected bytes after device record")
	}
	if record.Name != name {
		return record, errors.New("record name differs from its key")
	}
	if max(len(record.LastFetchError), len(record.AckError)) > managedDetailMax {
		return record, errors.New("device record error detail exceeds its byte limit")
	}
	return record, nil
}

func (s *ManagedServer) loadDeviceRecords() (result error) {
	// refuse-by-default: guards Start -- a server explicitly built without storage serves memory-only sessions
	if s.recordIO == nil {
		return nil
	}
	store := s.recordIO.store
	prefix := zefs.KeyFleetDeviceRecord.Prefix()
	// Subscribe before taking the guard and activate under its runtime lock
	// after the snapshot, so no change can fall between loading and observing.
	s.recordIO.subscription = store.SetRuntimeObserver(prefix, s.observeDeviceRecord)
	guard, err := store.AcquireLock(strings.TrimSuffix(prefix, "/"))
	if err != nil {
		s.recordIO.subscription.Unsubscribe()
		return fmt.Errorf("lock managed device records: %w", err)
	}
	defer func() {
		result = errors.Join(result, guard.Release())
		if result != nil {
			s.recordIO.subscription.Unsubscribe()
		}
	}()
	keys, err := guard.ListKeys(prefix)
	if err != nil {
		return fmt.Errorf("list managed device records: %w", err)
	}
	for _, key := range keys {
		name, ok := deviceRecordName(key)
		if !ok {
			continue
		}
		data, err := guard.ReadKey(key)
		if errors.Is(err, fs.ErrNotExist) {
			continue // A listed key is absent from this encoding.
		}
		var durable []byte
		var record ManagedDeviceRecord
		if err == nil {
			// Keep even an empty original distinct from a failed read. Guard
			// bytes are borrowed until Release, including on the blob backend.
			durable = append([]byte{}, data...)
			record, err = decodeDeviceRecord(name, data)
		}
		s.mu.Lock()
		device := s.devices[name]
		if device == nil {
			device = &managedDevice{ManagedDeviceRecord: ManagedDeviceRecord{Name: name}}
			s.devices[name] = device
		}
		device.durable = durable
		if err != nil {
			device.unreadable = true
			device.readFailure = err.Error()
			managedLogger().Error("cannot read managed device record", "code", CodeDoctorManagedDeviceRecordUnreadable, "key", key, "error", err)
		} else {
			device.ManagedDeviceRecord = record
			device.persisted = true
			if device.declared && !record.RemovedAt.IsZero() {
				device.RemovedAt = time.Time{}
				s.dirtyDeviceLocked(name)
			}
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	s.recordIO.loaded = true
	s.mu.Unlock()
	return nil
}

func (s *ManagedServer) recordWriter() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.recordIO.wake:
			for s.ctx.Err() == nil && s.drainOneDeviceRecord() {
			}
		}
	}
}

// stopDeviceRecords runs after the last session and writer exit. Keep the
// observer available during final saves so concurrent operator deletions win.
func (s *ManagedServer) stopDeviceRecords() {
	if s.recordIO == nil {
		return
	}
	s.mu.Lock()
	for name, failure := range s.recordIO.failed {
		s.recordIO.pending[name] = s.recordIO.pending[name] || failure.dirty
	}
	s.mu.Unlock()
	for {
		s.drainDeviceRecords()
		s.mu.Lock()
		if len(s.recordIO.pending) == 0 {
			s.recordIO.closed = true
			s.mu.Unlock()
			break
		}
		s.mu.Unlock()
	}
	if s.recordIO.subscription != nil {
		s.recordIO.subscription.Unsubscribe()
	}
}

func (s *ManagedServer) drainDeviceRecords() {
	if s.recordIO == nil {
		return
	}
	for s.drainOneDeviceRecord() {
	}
}

func (s *ManagedServer) drainOneDeviceRecord() bool {
	s.mu.Lock()
	var name string
	for queued := range s.recordIO.pending {
		name = queued
		break
	}
	s.mu.Unlock()
	if name == "" {
		return false
	}
	s.syncDeviceRecord(name)
	return true
}

// syncDeviceRecord consumes the pending work under the runtime key lock.
// A deletion cancels even a write that was already waiting for this lock.
// No registry lock is held while accessing storage.
func (s *ManagedServer) syncDeviceRecord(name string) {
	key := deviceRecordKey(name)
	guard, err := s.recordIO.store.AcquireLock(key)
	s.mu.Lock()
	dirty, pending := s.recordIO.pending[name]
	delete(s.recordIO.pending, name)
	if pending {
		s.recordIO.saving = name
	}
	dirty = dirty || s.recordIO.failed[name].dirty
	device := s.devices[name]
	var record ManagedDeviceRecord
	var original []byte
	var readFailure string
	if device != nil {
		record = device.ManagedDeviceRecord
		original = device.durable
		readFailure = device.readFailure
	}
	s.mu.Unlock()
	if err != nil {
		if pending {
			s.failDeviceRecord(name, key, dirty, err)
		}
		return
	}
	if !pending {
		if err := guard.Release(); err != nil {
			managedLogger().Error("cannot release unused managed device record guard", "code", diagnostic.CodeDoctorStoreIntegrity, "key", key, "error", err)
		}
		return
	}

	encoded := original
	var remove bool
	switch {
	case dirty && device != nil:
		encoded, err = json.Marshal(record)
	case device != nil && original == nil && readFailure != "":
		// The original could not be read at startup. Only a real event can
		// replace it; a foreign write must never restore partial or nil bytes.
		err = fmt.Errorf("cannot restore the original device record: %s", readFailure)
	case original == nil:
		remove = true
	}
	if err == nil {
		if remove {
			err = guard.RemoveKey(key)
			if errors.Is(err, fs.ErrNotExist) {
				// A previous attempt already removed this foreign key. No
				// mutation remains for the guard's callback to acknowledge.
				err = nil
				s.mu.Lock()
				delete(s.recordIO.failed, name)
				s.mu.Unlock()
			}
		} else {
			err = guard.WriteKey(key, encoded)
		}
	}
	if err == nil {
		guard.SetRuntimeObserver(s.recordIO.subscription, func(_ string, _ bool) {
			s.mu.Lock()
			defer s.mu.Unlock()
			delete(s.recordIO.failed, name)
			if !remove {
				s.mRecordWrites.With("ok").Inc()
			}
			if device == nil {
				return
			}
			if dirty {
				device.durable = encoded
				device.readFailure = ""
				device.unreadable = false
			}
			device.persisted = !remove && !device.unreadable && device.ManagedDeviceRecord == record
		})
	}
	if err = errors.Join(err, guard.Release()); err != nil {
		s.failDeviceRecord(name, key, dirty, err)
		return
	}
	// A guard can report an error after publishing the mutation to its
	// observer. Keep the save active until Release itself succeeds.
	s.mu.Lock()
	s.recordIO.saving = ""
	s.mu.Unlock()
	if !dirty {
		managedLogger().Info("reconciled a data-command write to managed device history", "key", key, "removed", remove)
	}
}

func (s *ManagedServer) forgetDeviceRecordLocked(name string, device *managedDevice) {
	if !device.declared {
		if device.conn != nil {
			// Reload is closing this session and waiting for its done channel.
			device.forgetOnDisconnect = true
			device.durable = nil
			return
		}
		delete(s.devices, name)
		return
	}
	cleared := ManagedDeviceRecord{Name: name}
	if device.conn != nil {
		cleared.ConnectedSince = device.ConnectedSince
		cleared.SourceAddress = device.SourceAddress
	}
	device.ManagedDeviceRecord = cleared
	device.durable = nil
	device.readFailure = ""
	device.unreadable = false
	device.persisted = false
}

func (s *ManagedServer) failDeviceRecord(name, key string, dirty bool, err error) {
	s.mRecordWrites.With("failed").Inc()
	s.mu.Lock()
	if s.recordIO.saving == name {
		// If an operator already deleted the record after this guard released,
		// shutdown must not retry the obsolete save and recreate the record.
		s.recordIO.saving = ""
		s.recordIO.failed[name] = managedRecordFailure{message: err.Error(), dirty: dirty}
		if device := s.devices[name]; device != nil {
			device.persisted = false
		}
	}
	s.mu.Unlock()
	managedLogger().Error("cannot persist managed device record", "code", CodeDoctorManagedDeviceRecordUnwritable, "key", key, "error", err)
}

// deviceRecordKey preserves even a manually created single segment containing
// ".."; the reader must be able to report or forget every stored record.
func deviceRecordKey(name string) string {
	return strings.Replace(zefs.KeyFleetDeviceRecord.Pattern, "{name}", name, 1)
}

func (s *ManagedServer) appendDeviceRecordFailures(record *ManagedDoctorRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, name := range slices.Sorted(maps.Keys(s.devices)) {
		device := s.devices[name]
		if device.readFailure != "" {
			record.Unreadable = append(record.Unreadable, ManagedDeviceRecordFailure{Name: name, Key: deviceRecordKey(name), Error: device.readFailure})
		}
	}
	if s.recordIO != nil {
		for _, name := range slices.Sorted(maps.Keys(s.recordIO.failed)) {
			record.Unwritable = append(record.Unwritable, ManagedDeviceRecordFailure{
				Name: name, Key: deviceRecordKey(name), Error: s.recordIO.failed[name].message, UnknownDevice: s.devices[name] == nil,
			})
		}
	}
}
