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

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

// managedRecordIO borrows the hub store until Stop. The wake channel carries
// one signal regardless of the number of events. Under the server mutex,
// pending keeps one entry per name; true means its live facts changed.
type managedRecordIO struct {
	store    storage.Storage
	pending  map[string]bool
	observed map[string]managedRecordObservation
	failed   map[string]string // Includes undeclared keys whose removal failed.
	wake     chan struct{}
	closed   bool
}

// A callback cannot read its key. Remember the live facts it observed so an
// authenticated event arriving after a deletion survives delayed reconciliation.
type managedRecordObservation struct {
	at        ManagedDeviceRecord
	connected bool
}

func newManagedRecordIO(store storage.Storage) *managedRecordIO {
	// refuse-by-default: guards NewManagedServer -- an omitted optional store creates a memory-only registry
	if store == nil {
		return nil
	}
	return &managedRecordIO{
		store: store, pending: make(map[string]bool), observed: make(map[string]managedRecordObservation),
		failed: make(map[string]string), wake: make(chan struct{}, 1),
	}
}

func (s *ManagedServer) dirtyDeviceLocked(name string) {
	if s.recordIO == nil {
		return
	}
	device := s.devices[name]
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

// ObserveDeviceRecordKey schedules a read-back after a durable store update.
// The store invokes this on the writing goroutine, including our own writer:
// it MUST never wait here for another store operation.
func (s *ManagedServer) ObserveDeviceRecordKey(key string) {
	if s.recordIO == nil {
		return
	}
	name, ok := deviceRecordName(key)
	if !ok {
		return
	}
	s.mu.Lock()
	if !s.recordIO.closed {
		if d := s.devices[name]; d != nil {
			s.recordIO.observed[name] = managedRecordObservation{at: d.ManagedDeviceRecord, connected: d.conn != nil}
		}
		s.queueDeviceRecordLocked(name, false)
	}
	s.mu.Unlock()
}

func decodeDeviceRecord(name string, data []byte) (ManagedDeviceRecord, error) {
	var record ManagedDeviceRecord
	if !fleet.ValidDeviceKeyName(name) {
		return record, errors.New("record key contains an invalid device name")
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

func (s *ManagedServer) loadDeviceRecords() error {
	// refuse-by-default: guards Start -- a server explicitly built without storage serves memory-only sessions
	if s.recordIO == nil {
		return nil
	}
	store := s.recordIO.store
	keys, err := store.ListKeys(zefs.KeyFleetDeviceRecord.Prefix())
	if err != nil {
		return fmt.Errorf("list managed device records: %w", err)
	}
	for _, key := range keys {
		name, ok := deviceRecordName(key)
		if !ok {
			continue
		}
		data, err := store.ReadKey(key)
		if errors.Is(err, fs.ErrNotExist) {
			continue // Removed between the listing and this read.
		}
		var record ManagedDeviceRecord
		if err == nil {
			record, err = decodeDeviceRecord(name, data)
		}
		s.mu.Lock()
		device := s.devices[name]
		if device == nil {
			device = &managedDevice{ManagedDeviceRecord: ManagedDeviceRecord{Name: name}}
			s.devices[name] = device
		}
		device.durable = data
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
	return nil
}

func (s *ManagedServer) recordWriter() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.recordIO.wake:
			s.drainDeviceRecords()
		}
	}
}

// stopDeviceRecords runs after all event producers and the writer have left.
// Give failed writes a final retry and stop accepting new observer work.
func (s *ManagedServer) stopDeviceRecords() {
	if s.recordIO == nil {
		return
	}
	s.mu.Lock()
	s.recordIO.closed = true
	for name := range s.recordIO.failed {
		s.recordIO.pending[name] = true
	}
	s.mu.Unlock()
	s.drainDeviceRecords()
}

func (s *ManagedServer) drainDeviceRecords() {
	if s.recordIO == nil {
		return
	}
	for {
		s.mu.Lock()
		pending := s.recordIO.pending
		if len(pending) == 0 {
			s.mu.Unlock()
			return
		}
		s.recordIO.pending = make(map[string]bool)
		s.mu.Unlock()
		for name, dirty := range pending {
			s.syncDeviceRecord(name, dirty)
		}
	}
}

// syncDeviceRecord takes the key guard before comparing the durable bytes.
// A data command racing a writer therefore cannot be silently overwritten
// between inspection and write. Release can synchronously call our observer,
// so it MUST run without the registry mutex.
func (s *ManagedServer) syncDeviceRecord(name string, dirty bool) {
	// Rebuild from the single-segment name a storage listing returned. This
	// also accepts a store key created by hand whose name contains "..".
	key := deviceRecordKey(name)
	guard, err := s.recordIO.store.AcquireLock(key)
	if err != nil {
		s.failDeviceRecord(name, key, err)
		return
	}
	disk, readErr := guard.ReadKey(key)
	gone := errors.Is(readErr, fs.ErrNotExist)
	if readErr != nil && !gone {
		err = errors.Join(readErr, guard.Release())
		s.failDeviceRecord(name, key, err)
		return
	}
	s.mu.Lock()
	observed, wasObserved := s.recordIO.observed[name]
	delete(s.recordIO.observed, name)
	device := s.devices[name]
	if device == nil {
		s.mu.Unlock()
		// Memory is authoritative. A newly invented key has no record to
		// restore and must not manufacture a device at the next restart.
		if !gone {
			err = guard.RemoveKey(key)
		}
		if err = errors.Join(err, guard.Release()); err != nil {
			s.failDeviceRecord(name, key, err)
			return
		}
		s.mu.Lock()
		delete(s.recordIO.failed, name)
		s.mu.Unlock()
		if !gone {
			managedLogger().Info("removed a data-command record for a device the registry does not know", "key", key)
		}
		return
	}
	if gone && (device.durable != nil || device.unreadable) {
		if !wasObserved {
			observed = managedRecordObservation{at: device.ManagedDeviceRecord, connected: device.conn != nil}
		}
		s.forgetDeviceRecordLocked(name, device, &observed)
		s.mu.Unlock()
		if err := guard.Release(); err != nil {
			s.failDeviceRecord(name, key, err)
		}
		return
	}
	record := device.ManagedDeviceRecord
	overwritten := !gone && !bytes.Equal(disk, device.durable)
	// An unreadable record is retained byte for byte until an event supplies
	// new facts; keep its original bytes even after an operator overwrite.
	restoreUnreadable := overwritten && device.unreadable && !dirty
	empty := record == (ManagedDeviceRecord{Name: name})
	var encoded []byte
	if restoreUnreadable {
		encoded = slices.Clone(device.durable)
	}
	s.mu.Unlock()
	write := dirty || overwritten
	if write {
		switch {
		case restoreUnreadable:
			err = guard.WriteKey(key, encoded)
		case empty:
			if !gone {
				err = guard.RemoveKey(key)
			}
		default:
			encoded, err = json.Marshal(record)
			if err == nil {
				err = guard.WriteKey(key, encoded)
			}
		}
	}
	if err = errors.Join(err, guard.Release()); err != nil {
		s.failDeviceRecord(name, key, err)
		return
	}
	if !write {
		return
	}
	if overwritten {
		managedLogger().Info("restored managed device history after a data-command overwrite", "key", key)
	}
	s.mu.Lock()
	if encoded != nil {
		s.mRecordWrites.With("ok").Inc()
	}
	device.durable = encoded
	delete(s.recordIO.failed, name)
	if !restoreUnreadable {
		device.readFailure = ""
		device.unreadable = false
		device.persisted = !empty && device.ManagedDeviceRecord == record
	}
	s.mu.Unlock()
}

func (s *ManagedServer) forgetDeviceRecordLocked(name string, device *managedDevice, observed *managedRecordObservation) {
	delete(s.recordIO.failed, name)
	if !device.declared {
		if device.conn != nil {
			// A reload is closing this session and waiting for its done
			// channel. The session handler still needs the entry to finish.
			device.forgetOnDisconnect = true
			device.durable = nil
			return
		}
		delete(s.devices, name)
		return
	}
	current := device.ManagedDeviceRecord
	device.ManagedDeviceRecord = observed.afterDelete(current)
	if current != observed.at {
		s.queueDeviceRecordLocked(name, true)
	} else {
		delete(s.recordIO.pending, name)
	}
	device.durable = nil
	device.readFailure = ""
	device.unreadable = false
	device.persisted = false
}

// afterDelete rebuilds only the history recorded since the operator removed
// the key. A live session at deletion retains its start and source. A later
// session supplies the first connection of the new history.
func (ob *managedRecordObservation) afterDelete(current ManagedDeviceRecord) ManagedDeviceRecord {
	cleared := ManagedDeviceRecord{Name: current.Name}
	if ob.connected {
		cleared.ConnectedSince = ob.at.ConnectedSince
		cleared.SourceAddress = ob.at.SourceAddress
	}
	if current.ConnectedSince != ob.at.ConnectedSince {
		cleared.FirstConnected = current.ConnectedSince
		cleared.ConnectedSince = current.ConnectedSince
		cleared.SourceAddress = current.SourceAddress
	}
	if current.LastSeen != ob.at.LastSeen {
		cleared.LastSeen = current.LastSeen
	}
	if current.LastFetch != ob.at.LastFetch {
		cleared.LastFetch = current.LastFetch
		cleared.DeviceVersion = current.DeviceVersion
		cleared.LastFetchResult = current.LastFetchResult
		cleared.LastFetchError = current.LastFetchError
	}
	if current.LastAck != ob.at.LastAck {
		cleared.LastAck = current.LastAck
		cleared.AckResult = current.AckResult
		cleared.AckVersion = current.AckVersion
		cleared.AckError = current.AckError
	}
	return cleared
}

func (s *ManagedServer) failDeviceRecord(name, key string, err error) {
	s.mRecordWrites.With("failed").Inc()
	s.mu.Lock()
	s.recordIO.failed[name] = err.Error()
	if device := s.devices[name]; device != nil {
		device.persisted = false
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
				Name: name, Key: deviceRecordKey(name), Error: s.recordIO.failed[name], UnknownDevice: s.devices[name] == nil,
			})
		}
	}
}
