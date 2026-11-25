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
	store.SetWriteObserver(s.ObserveDeviceRecordKey)
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
		s.ObserveDeviceRecordKey(key)
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
	fail bool
}

type failingRecordGuard struct{ storage.WriteGuard }

func (g failingRecordGuard) Release() error {
	return errors.Join(g.WriteGuard.Release(), errors.New("disk is full"))
}

func (store *failingRecordStore) AcquireLock(key string) (storage.WriteGuard, error) {
	g, err := store.Storage.AcquireLock(key)
	if err != nil {
		return nil, err
	}
	if store.fail {
		return failingRecordGuard{g}, nil
	}
	return g, nil
}

func TestRegistryWriteFailureKeepsLiveFacts(t *testing.T) {
	store := &failingRecordStore{Storage: heldTestStore(t), fail: true}
	s := registryWithHistory(t, store)
	s.recordAck(testClientName, fleet.ConfigAck{Version: "initial"}, ManagedAckRefused)
	// The injected failure follows the underlying guard release. Suppress the
	// underlying store's success callback, as a real failed release would.
	store.SetWriteObserver(nil)
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
	conn := recordTestSession(t, s)
	device := s.devices[testClientName]
	done := device.sessionDone
	device.declared = false // Reload has withdrawn this name and is closing its connection.
	require.NoError(t, store.RemoveKey(deviceRecordKey(testClientName)))
	s.drainDeviceRecords()
	s.disconnectDevice(testClientName, conn)
	select {
	case <-done:
	default:
		t.Fatal("reload is still waiting for the forgotten session")
	}
	s.drainDeviceRecords()
	require.NotContains(t, s.devices, testClientName)
	_, err := store.ReadKey(deviceRecordKey(testClientName))
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
	// The underlying test guard succeeds before the injected release error;
	// a truly failed release would suppress its callback.
	store.SetWriteObserver(nil)
	s.drainDeviceRecords()
	var failed ManagedDoctorRecord
	s.appendDeviceRecordFailures(&failed)
	require.Len(t, failed.Unwritable, 1)
	require.Equal(t, key, failed.Unwritable[0].Key)
	require.True(t, failed.Unwritable[0].UnknownDevice)
	require.Contains(t, failed.Unwritable[0].Error, "disk is full")
	require.NotContains(t, s.devices, name)

	store.fail = false
	store.SetWriteObserver(s.ObserveDeviceRecordKey)
	require.NoError(t, store.WriteKey(key, []byte("another data write")))
	s.drainDeviceRecords()
	var repaired ManagedDoctorRecord
	s.appendDeviceRecordFailures(&repaired)
	require.Empty(t, repaired.Unwritable)
	_, err := store.ReadKey(key)
	require.ErrorIs(t, err, fs.ErrNotExist)
}
