// VALIDATES: durable registry history, write coalescing, and online data commands.
// PREVENTS: a restart resurrecting a forgotten device or reporting a dead session.

package server

import (
	"errors"
	"io/fs"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func registryWithHistory(t *testing.T, store storage.Storage) *ManagedServer {
	t.Helper()
	s := registryTestServer(t)
	s.recordIO = newManagedRecordIO(store)
	require.NoError(t, s.loadDeviceRecords())
	return s
}

func recordTestSession(t *testing.T, s *ManagedServer) *rpc.MuxConn {
	t.Helper()
	conn := &rpc.MuxConn{}
	require.NoError(t, s.admitDevice(testClientName, netip.MustParseAddr("192.0.2.10"), conn))
	s.recordAck(testClientName, fleet.ConfigAck{Version: "original"}, ManagedAckApplied)
	s.drainDeviceRecords()
	return conn
}

func TestRegistryLoadMarksEveryDeviceOffline(t *testing.T) {
	store := heldTestStore(t)
	first := registryWithHistory(t, store)
	recordTestSession(t, first)
	before := first.snapshotDevices()[0]
	require.True(t, before.Connected)
	restarted := registryWithHistory(t, store)
	after := restarted.snapshotDevices()[0]
	require.False(t, after.Connected)
	require.True(t, after.Persisted)
	require.Equal(t, before.ManagedDeviceRecord, after.ManagedDeviceRecord)
}

func TestRegistryUndeclaredStoredRecordListedRemoved(t *testing.T) {
	store := heldTestStore(t)
	require.NoError(t, store.WriteKey(deviceRecordKey("retired"), []byte(`{"name":"retired","ack-version":"saved"}`)))
	s := registryWithHistory(t, store)
	rows := s.snapshotDevices()
	require.Len(t, rows, 3)
	require.Equal(t, "retired", rows[2].Name)
	require.False(t, rows[2].Declared)
	require.False(t, rows[2].Connected)
	require.True(t, rows[2].Persisted)
	require.Equal(t, "saved", rows[2].AckVersion)
}

func TestRegistryUnreadableRecordKept(t *testing.T) {
	for _, bad := range []string{`{"name":"edge-01","new-field":2}`, `{"name":"edge-02"}`, `{"name":"edge-01"} {}`,
		`{"name":"edge-01","last-fetch-result":"invalid"}`, `{"name":"edge-01","ack-result":"invalid"}`,
		`{"name":"edge-01","source-address":"invalid"}`, `{"name":"edge-01","last-fetch":"yesterday"}`,
		`{"name":"edge-01","ack-error":"` + "\xff" + `"}`,
		`{"name":"edge-01","ack-error":"` + strings.Repeat("x", managedDetailMax+1) + `"}`} {
		store := heldTestStore(t)
		key := deviceRecordKey(testClientName)
		require.NoError(t, store.WriteKey(key, []byte(bad)))
		s := registryWithHistory(t, store)
		require.True(t, s.snapshotDevices()[0].Unreadable)
		var doctor ManagedDoctorRecord
		s.appendDeviceRecordFailures(&doctor)
		require.Len(t, doctor.Unreadable, 1)
		require.Equal(t, key, doctor.Unreadable[0].Key)
		untouched, err := store.ReadKey(key)
		require.NoError(t, err)
		require.Equal(t, bad, string(untouched))
		recordTestSession(t, s)
		got := s.snapshotDevices()[0]
		require.False(t, got.Unreadable)
		require.True(t, got.Persisted)
		var repaired ManagedDoctorRecord
		s.appendDeviceRecordFailures(&repaired)
		require.Empty(t, repaired.Unreadable)
	}
}

func TestRegistryWriterCoalescesPerDevice(t *testing.T) {
	store := heldTestStore(t)
	s := registryWithHistory(t, store)
	writes := make(map[string]int)
	store.SetWriteObserver(func(key string) {
		writes[key]++
	})
	for range 50 {
		s.recordAck(testClientName, fleet.ConfigAck{Version: "v1"}, ManagedAckApplied)
		s.recordAck(testClient2Name, fleet.ConfigAck{Version: "v2"}, ManagedAckRefused)
	}
	require.Len(t, s.recordIO.pending, 2)
	s.drainDeviceRecords()
	require.Equal(t, map[string]int{deviceRecordKey(testClientName): 1, deviceRecordKey(testClient2Name): 1}, writes)
	for _, row := range s.snapshotDevices() {
		require.True(t, row.Persisted)
		onDisk, err := store.ReadKey(deviceRecordKey(row.Name))
		require.NoError(t, err)
		r, err := decodeDeviceRecord(row.Name, onDisk)
		require.NoError(t, err)
		require.Equal(t, row.ManagedDeviceRecord, r)
	}
}

// failingRecordStore injects a durable release failure after a real guard write.
type failingRecordStore struct {
	storage.Storage
	fail         bool
	afterRelease func()
}

type failingRecordGuard struct {
	storage.WriteGuard
	afterRelease func()
}

func (g failingRecordGuard) Release() error {
	releaseErr := g.WriteGuard.Release()
	if g.afterRelease != nil {
		g.afterRelease()
	}
	return errors.Join(releaseErr, errors.New("disk is full"))
}

func (store *failingRecordStore) AcquireLock(key string) (storage.WriteGuard, error) {
	g, err := store.Storage.AcquireLock(key)
	if err != nil {
		return nil, err
	}
	if store.fail {
		return failingRecordGuard{g, store.afterRelease}, nil
	}
	return g, nil
}

func TestRegistryWriteFailureKeepsLiveFacts(t *testing.T) {
	store := &failingRecordStore{Storage: heldTestStore(t)}
	s := registryWithHistory(t, store)
	store.fail = true
	s.recordAck(testClientName, fleet.ConfigAck{Version: "initial"}, ManagedAckRefused)
	s.drainDeviceRecords()
	before := s.snapshotDevices()[0]
	require.False(t, before.Persisted)
	require.Equal(t, "initial", before.AckVersion)
	var doctor ManagedDoctorRecord
	s.appendDeviceRecordFailures(&doctor)
	require.Len(t, doctor.Unwritable, 1)
	require.Equal(t, testClientName, doctor.Unwritable[0].Name)
	require.Contains(t, doctor.Unwritable[0].Error, "disk is full")
	store.fail = false
	s.recordAck(testClientName, fleet.ConfigAck{Version: "next"}, ManagedAckApplied)
	s.drainDeviceRecords()
	var repaired ManagedDoctorRecord
	s.appendDeviceRecordFailures(&repaired)
	require.Empty(t, repaired.Unwritable)
	require.True(t, s.snapshotDevices()[0].Persisted)
}

func TestRegistryStoppedWriterCannotRetryAfterHandover(t *testing.T) {
	store := &failingRecordStore{Storage: heldTestStore(t)}
	first := registryWithHistory(t, store)
	store.fail = true
	first.recordAck(testClientName, fleet.ConfigAck{Version: "old"}, ManagedAckRefused)
	first.Stop()
	require.Contains(t, first.recordIO.failed, testClientName)

	store.fail = false
	second := registryWithHistory(t, store)
	t.Cleanup(second.Stop)
	second.recordAck(testClientName, fleet.ConfigAck{Version: "new"}, ManagedAckApplied)
	second.drainDeviceRecords()
	first.Stop()
	raw, err := store.ReadKey(deviceRecordKey(testClientName))
	require.NoError(t, err)
	record, err := decodeDeviceRecord(testClientName, raw)
	require.NoError(t, err)
	require.Equal(t, "new", record.AckVersion)
}

func TestRegistryOnlineDeleteForgetsRemovedDevice(t *testing.T) {
	store := heldTestStore(t)
	key := deviceRecordKey("retired")
	require.NoError(t, store.WriteKey(key, []byte(`{"name":"retired","ack-result":"holding"}`)))
	s := registryWithHistory(t, store)
	require.Contains(t, s.devices, "retired")
	require.NoError(t, store.RemoveKey(key))
	s.drainDeviceRecords()
	require.NotContains(t, s.devices, "retired")
	restarted := registryWithHistory(t, store)
	require.NotContains(t, restarted.devices, "retired")
}

func TestRegistryOnlineDeleteClearsDeclaredHistory(t *testing.T) {
	store := heldTestStore(t)
	s := registryWithHistory(t, store)
	conn := recordTestSession(t, s)
	before := s.snapshotDevices()[0]
	key := deviceRecordKey(testClientName)
	require.NoError(t, store.RemoveKey(key))
	s.drainDeviceRecords()
	after := s.snapshotDevices()[0]
	require.True(t, after.Connected)
	require.Equal(t, before.ConnectedSince, after.ConnectedSince)
	require.Equal(t, before.SourceAddress, after.SourceAddress)
	require.Empty(t, after.AckVersion)
	require.True(t, after.FirstConnected.IsZero())
	require.False(t, after.Persisted)
	_, err := store.ReadKey(key)
	require.ErrorIs(t, err, fs.ErrNotExist)
	s.disconnectDevice(testClientName, conn)
	s.drainDeviceRecords()
	restarted := registryWithHistory(t, store).snapshotDevices()[0]
	require.False(t, restarted.Connected)
	require.True(t, restarted.Persisted)
	require.False(t, restarted.LastSeen.IsZero())
	require.Equal(t, before.ConnectedSince, restarted.ConnectedSince)
	require.Equal(t, before.SourceAddress, restarted.SourceAddress)
	require.Empty(t, restarted.AckVersion)
}

func TestRegistryOnlineOverwriteWrittenBack(t *testing.T) {
	store := heldTestStore(t)
	s := registryWithHistory(t, store)
	recordTestSession(t, s)
	key := deviceRecordKey(testClientName)
	before, err := store.ReadKey(key)
	require.NoError(t, err)
	require.NoError(t, store.WriteKey(key, []byte(`{"name":"edge-01","ack-version":"forged"}`)))
	s.drainDeviceRecords()
	restored, err := store.ReadKey(key)
	require.NoError(t, err)
	require.Equal(t, before, restored)
	require.Equal(t, "original", registryWithHistory(t, store).snapshotDevices()[0].AckVersion)
}

func TestRegistryOnlineDeleteFollowedByEventBeforeWriter(t *testing.T) {
	store := heldTestStore(t)
	s := registryWithHistory(t, store)
	recordTestSession(t, s)
	key := deviceRecordKey(testClientName)
	require.NoError(t, store.RemoveKey(key))
	s.recordAck(testClientName, fleet.ConfigAck{Version: "after-delete"}, ManagedAckApplied)
	s.drainDeviceRecords()
	restarted := registryWithHistory(t, store).snapshotDevices()[0]
	require.Equal(t, "after-delete", restarted.AckVersion)
	require.True(t, restarted.FirstConnected.IsZero())
	require.False(t, restarted.ConnectedSince.IsZero())
}

func TestRegistryOnlineDeleteWhileRemovalClosesSession(t *testing.T) {
	store := heldTestStore(t)
	s := registryWithHistory(t, store)
	publishManagedDoctor(s)
	t.Cleanup(func() { withdrawManagedDoctor(s) })
	conn := recordTestSession(t, s)
	device := s.devices[testClientName]
	done := device.sessionDone
	device.declared = false // Reload has withdrawn this name and is closing its connection.
	require.NoError(t, store.RemoveKey(deviceRecordKey(testClientName)))
	require.Same(t, device, s.devices[testClientName])
	rows, err := ManagedRegistrySnapshot()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, testClient2Name, rows[0].Name)
	_, err = ManagedDeviceByName(testClientName)
	require.ErrorIs(t, err, ErrManagedDeviceUnknown)
	require.Equal(t, []string{testClient2Name}, ManagedDeviceNames())
	s.drainDeviceRecords()
	s.disconnectDevice(testClientName, conn)
	select {
	case <-done:
	default:
		t.Fatal("reload is still waiting for the forgotten session")
	}
	s.drainDeviceRecords()
	require.NotContains(t, s.devices, testClientName)
	_, err = store.ReadKey(deviceRecordKey(testClientName))
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRegistryLateDirtyEventAfterForget(t *testing.T) {
	store := heldTestStore(t)
	const name = "retired"
	key := deviceRecordKey(name)
	require.NoError(t, store.WriteKey(key, []byte(`{"name":"retired"}`)))
	s := registryWithHistory(t, store)
	require.NoError(t, store.RemoveKey(key))
	require.NotContains(t, s.devices, name)
	s.mu.Lock()
	s.dirtyDeviceLocked(name)
	s.mu.Unlock()
	require.Empty(t, s.recordIO.pending)
	s.drainDeviceRecords()
	_, err := store.ReadKey(key)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// VALIDATES: failed removal of a foreign key stays visible to doctor even
// though that key has no live or historical device entry.
func TestRegistryOnlineUnknownRecordRepairFailure(t *testing.T) {
	store := &failingRecordStore{Storage: heldTestStore(t)}
	s := registryWithHistory(t, store)
	const name = "invented"
	key := deviceRecordKey(name)
	require.NoError(t, store.WriteKey(key, []byte("injected")))
	store.fail = true
	s.drainDeviceRecords()
	var failed ManagedDoctorRecord
	s.appendDeviceRecordFailures(&failed)
	require.Len(t, failed.Unwritable, 1)
	require.Equal(t, key, failed.Unwritable[0].Key)
	require.True(t, failed.Unwritable[0].UnknownDevice)
	require.Contains(t, failed.Unwritable[0].Error, "disk is full")
	require.NotContains(t, s.devices, name)

	store.fail = false
	require.NoError(t, store.WriteKey(key, []byte("another data write")))
	s.drainDeviceRecords()
	var repaired ManagedDoctorRecord
	s.appendDeviceRecordFailures(&repaired)
	require.Empty(t, repaired.Unwritable)
	_, err := store.ReadKey(key)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// countingRecordStore counts actual storage acquisitions across a complete drain,
// including any echo that the writer inadvertently put back in its own queue.
type countingRecordStore struct {
	storage.Storage
	locks int
}

func (s *countingRecordStore) AcquireLock(key string) (storage.WriteGuard, error) {
	s.locks++
	return s.Storage.AcquireLock(key)
}

func TestRegistryStoreWritesOnceWithoutObserverEcho(t *testing.T) {
	store := &countingRecordStore{Storage: heldTestStore(t)}
	s := registryWithHistory(t, store)
	baseline := store.locks
	recordTestSession(t, s)
	require.Equal(t, baseline+1, store.locks)
	require.True(t, s.snapshotDevices()[0].Persisted)
}

func TestRegistryDeleteBeforeForeignWriteDoesNotRestoreHistory(t *testing.T) {
	for _, declared := range []bool{true, false} {
		store := heldTestStore(t)
		s := registryWithHistory(t, store)
		conn := recordTestSession(t, s)
		s.disconnectDevice(testClientName, conn)
		s.drainDeviceRecords()
		s.devices[testClientName].declared = declared
		key := deviceRecordKey(testClientName)
		require.NoError(t, store.RemoveKey(key))
		if declared {
			require.Empty(t, s.snapshotDevices()[0].AckVersion)
		} else {
			require.NotContains(t, s.devices, testClientName)
		}
		require.NoError(t, store.WriteKey(key, []byte("foreign history")))
		s.drainDeviceRecords()
		_, err := store.ReadKey(key)
		require.ErrorIs(t, err, fs.ErrNotExist)
		restarted := registryWithHistory(t, store)
		require.Empty(t, restarted.snapshotDevices()[0].AckVersion)
	}
}

// An unreadable original has no bytes that the registry can safely copy back.
// Inject a partial result as a failing underlying read is allowed to do.
type unreadableRecordStore struct {
	storage.Storage
}

type unreadableRecordGuard struct{ storage.WriteGuard }

func (s *unreadableRecordStore) AcquireLock(key string) (storage.WriteGuard, error) {
	g, err := s.Storage.AcquireLock(key)
	if err != nil {
		return nil, err
	}
	if key == "meta/fleet/device" {
		return unreadableRecordGuard{g}, nil
	}
	return g, nil
}

func (g unreadableRecordGuard) ReadKey(key string) ([]byte, error) {
	return []byte("partial"), fs.ErrPermission
}

func TestRegistryUnavailableOriginalNeverRestoresPartialBytes(t *testing.T) {
	store := &unreadableRecordStore{heldTestStore(t)}
	key := deviceRecordKey(testClientName)
	require.NoError(t, store.WriteKey(key, []byte("original")))
	s := registryWithHistory(t, store)
	require.True(t, s.snapshotDevices()[0].Unreadable)
	require.NoError(t, store.WriteKey(key, []byte("operator bytes")))
	s.drainDeviceRecords()
	value, err := store.ReadKey(key)
	require.NoError(t, err)
	require.Equal(t, "operator bytes", string(value))
	var doctor ManagedDoctorRecord
	s.appendDeviceRecordFailures(&doctor)
	require.Len(t, doctor.Unwritable, 1)
	s.recordAck(testClientName, fleet.ConfigAck{Version: "new"}, ManagedAckApplied)
	s.drainDeviceRecords()
	require.False(t, s.snapshotDevices()[0].Unreadable)
	value, err = store.ReadKey(key)
	require.NoError(t, err)
	record, err := decodeDeviceRecord(testClientName, value)
	require.NoError(t, err)
	require.Equal(t, "new", record.AckVersion)
}

func TestRegistryShutdownRetriesUnreadableRecordRepair(t *testing.T) {
	store := &failingRecordStore{Storage: heldTestStore(t)}
	key := deviceRecordKey(testClientName)
	original := []byte(`{"name":"edge-01","future-field":true}`)
	require.NoError(t, store.WriteKey(key, original))
	s := registryWithHistory(t, store)
	require.NoError(t, store.WriteKey(key, []byte("foreign")))
	store.fail = true
	s.drainDeviceRecords()
	require.True(t, s.snapshotDevices()[0].Unreadable)
	var failed ManagedDoctorRecord
	s.appendDeviceRecordFailures(&failed)
	require.Len(t, failed.Unwritable, 1)
	store.fail = false
	s.stopDeviceRecords()
	value, err := store.ReadKey(key)
	require.NoError(t, err)
	require.Equal(t, original, value)
	var repaired ManagedDoctorRecord
	s.appendDeviceRecordFailures(&repaired)
	require.Empty(t, repaired.Unwritable)
	require.Len(t, repaired.Unreadable, 1)
}

func TestRegistryDeletionSupersedesCompletingFailedSave(t *testing.T) {
	store := &failingRecordStore{Storage: heldTestStore(t)}
	s := registryWithHistory(t, store)
	recordTestSession(t, s)
	key := deviceRecordKey(testClientName)
	store.fail = true
	store.afterRelease = func() {
		require.NoError(t, store.Storage.RemoveKey(key))
	}
	s.recordAck(testClientName, fleet.ConfigAck{Version: "previous"}, ManagedAckRefused)
	s.drainDeviceRecords()
	var doctor ManagedDoctorRecord
	s.appendDeviceRecordFailures(&doctor)
	require.Empty(t, doctor.Unwritable)
	store.fail = false
	s.stopDeviceRecords()
	_, err := store.ReadKey(key)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRegistrySaveCompletesBeforeUnusedGuardFails(t *testing.T) {
	store := &failingRecordStore{Storage: heldTestStore(t)}
	s := registryWithHistory(t, store)
	recordTestSession(t, s)
	require.Empty(t, s.recordIO.saving)

	// A queued name can have no pending work by the time the writer gets
	// its guard. Releasing this unused guard cannot invalidate a prior save.
	store.fail = true
	s.syncDeviceRecord(testClientName)
	require.Empty(t, s.recordIO.failed)
	require.True(t, s.snapshotDevices()[0].Persisted)

	s.recordAck(testClientName, fleet.ConfigAck{Version: "retry"}, ManagedAckApplied)
	s.drainDeviceRecords()
	require.Empty(t, s.recordIO.saving)
	require.Contains(t, s.recordIO.failed, testClientName)
	store.fail = false
	s.stopDeviceRecords()
	require.Empty(t, s.recordIO.saving)
	require.Empty(t, s.recordIO.failed)
}

// An unused declaration has no history to remove. Durable and unreadable
// records on the same listener survive removal, redeclaration and restart.
func TestManagedApplyRemovesOnlyDeclarationsWithoutHistory(t *testing.T) {
	store := heldTestStore(t)
	const saved = "saved"
	const unreadable = "unreadable"
	corrupt := []byte("cannot decode this history")
	require.NoError(t, store.WriteKey(deviceRecordKey(saved), []byte(`{"name":"saved","ack-version":"v1"}`)))
	require.NoError(t, store.WriteKey(deviceRecordKey(unreadable), corrupt))
	services := []ManagedListener{{
		Addr:    "127.0.0.1:0",
		Clients: map[string]string{testClientName: testClientSecret, saved: "saved-secret", unreadable: "unreadable-secret"},
	}}
	t.Run("reload", func(t *testing.T) {
		s := startApplyTest(t, ManagedServerConfig{Authority: newTestCA(t), Store: store, Listeners: services})
		for range 2 {
			device, err := s.registryDevice(testClientName)
			require.NoError(t, err)
			require.True(t, device.Declared)
			require.True(t, device.RemovedAt.IsZero())
			require.False(t, device.Persisted)
			applyManaged(t, s, nil, "")
			_, err = s.registryDevice(testClientName)
			require.ErrorIs(t, err, ErrManagedDeviceUnknown)
			device, err = s.registryDevice(unreadable)
			require.NoError(t, err)
			require.False(t, device.Declared)
			require.True(t, device.Unreadable)
			applyManaged(t, s, services, "")
		}
		applyManaged(t, s, nil, "")
	}) // Stop flushes every pending change before the store is reopened by the next server.
	restarted := startApplyTest(t, ManagedServerConfig{Store: store})
	rows := restarted.snapshotDevices()
	require.Len(t, rows, 2)
	require.Equal(t, saved, rows[0].Name)
	require.False(t, rows[0].Declared)
	require.False(t, rows[0].RemovedAt.IsZero())
	require.Equal(t, "v1", rows[0].AckVersion)
	require.Equal(t, unreadable, rows[1].Name)
	require.True(t, rows[1].Unreadable)
	data, err := store.ReadKey(deviceRecordKey(unreadable))
	require.NoError(t, err)
	require.Equal(t, corrupt, data)
	_, err = store.ReadKey(deviceRecordKey(testClientName))
	require.ErrorIs(t, err, fs.ErrNotExist)
}
