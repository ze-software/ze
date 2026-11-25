// VALIDATES: fleet-1 AC-13 and AC-25, the hub reads held presence live and hashes
// only the client configurations that changed, including never-connected devices.
// PREVENTS: treating an unreadable held record as absence or serving stale hashes.

package server

import (
	"bufio"
	"errors"
	"io/fs"
	"net"
	"net/netip"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRegistryDivergedFromHeldRecord(t *testing.T) {
	store := heldTestStore(t)
	s := registryTestServer(t)
	s.svc.records = newHeldRecords(store)
	require.NoError(t, store.WriteKey(HeldRecordKey(testClientName), []byte("unreadable")))
	rows, err := s.registrySnapshot()
	require.NoError(t, err)
	require.True(t, rows[0].Diverged)
	require.False(t, rows[1].Diverged)
	require.NoError(t, store.RemoveKey(HeldRecordKey(testClientName)))
	rows, err = s.registrySnapshot()
	require.NoError(t, err)
	require.False(t, rows[0].Diverged)
}

type heldLookupStore struct {
	storage.Storage
	readKeys []string
}

func (s *heldLookupStore) ListKeys(string) ([]string, error) {
	return nil, errors.New("a named detail must not enumerate the fleet's held records")
}

func (s *heldLookupStore) ReadKey(key string) ([]byte, error) {
	s.readKeys = append(s.readKeys, key)
	return s.Storage.ReadKey(key)
}

func TestRegistryDetailLooksUpOneHeldKey(t *testing.T) {
	store := &heldLookupStore{Storage: heldTestStore(t)}
	s := registryTestServer(t)
	s.svc.records = newHeldRecords(store)
	key := HeldRecordKey(testClientName)
	require.NoError(t, store.WriteKey(key, []byte("malformed")))
	got, err := s.registryDevice(testClientName)
	require.NoError(t, err)
	require.True(t, got.Diverged, "the existence of a held copy is enough")
	require.NoError(t, store.RemoveKey(key))
	got, err = s.registryDevice(testClientName)
	require.NoError(t, err)
	require.False(t, got.Diverged)
	_, err = s.registryDevice("missing")
	require.ErrorIs(t, err, ErrManagedDeviceUnknown)
	require.Equal(t, []string{key, key}, store.readKeys)
}

func TestRegistryHubVersionStaleOnConfigWrite(t *testing.T) {
	s := registryTestServer(t)
	reads := map[string]int{}
	content := map[string]string{testClientName: "first"}
	s.svc.readConfig = func(name string) ([]byte, error) {
		reads[name]++
		value, ok := content[name]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return []byte(value), nil
	}
	rows, err := s.registrySnapshot()
	require.NoError(t, err)
	require.Equal(t, fleet.VersionHash([]byte("first")), rows[0].HubVersion)
	require.Empty(t, rows[1].HubVersion)
	require.True(t, rows[0].FirstConnected.IsZero())
	require.Equal(t, map[string]int{testClientName: 1, testClient2Name: 1}, reads)

	content[testClient2Name] = "new"
	s.MarkManagedConfigChanged(testClient2Name)
	rows, err = s.registrySnapshot()
	require.NoError(t, err)
	require.Equal(t, fleet.VersionHash([]byte("new")), rows[1].HubVersion)
	require.Equal(t, map[string]int{testClientName: 1, testClient2Name: 2}, reads)
	s.MarkManagedConfigChanged(testClientName)
	failure := errors.New("configuration store unavailable")
	s.svc.readConfig = func(string) ([]byte, error) { return nil, failure }
	_, err = s.registrySnapshot()
	require.ErrorIs(t, err, failure)
}

func TestRegistryConfigWriteDuringRead(t *testing.T) {
	s := registryTestServer(t)
	reads := 0
	s.svc.readConfig = func(name string) ([]byte, error) {
		reads++
		if reads == 1 {
			// The observer may run before a read of the previous bytes returns.
			// Calling back here also proves the read never holds the registry lock.
			s.MarkManagedConfigChanged(name)
			return []byte("old"), nil
		}
		return []byte("current"), nil
	}
	current, err := s.currentConfigVersion(testClientName, s.devices[testClientName])
	require.NoError(t, err)
	require.Equal(t, fleet.VersionHash([]byte("current")), current)
	again, err := s.currentConfigVersion(testClientName, s.devices[testClientName])
	require.NoError(t, err)
	require.Equal(t, current, again)
	require.Equal(t, 2, reads)
}

func TestRegistryOnlyActiveClientConfigKeysInvalidateVersions(t *testing.T) {
	for _, key := range []string{"client-edge.conf", "file/active/client-edge.conf"} {
		name, ok := ClientNameFromConfigKey(key)
		require.True(t, ok, key)
		require.Equal(t, "edge", name)
	}
	for _, key := range []string{"file/draft/client-edge.conf", "file/active/nested/client-edge.conf",
		"meta/managed-device/client-edge.conf", "meta/config/client-edge.conf/held", "file/active/hub.conf"} {
		_, ok := ClientNameFromConfigKey(key)
		require.False(t, ok, key)
	}
}

func TestRegistryMalformedHistoryNameDoesNotHideOtherDevices(t *testing.T) {
	s := registryTestServer(t)
	const badName = "x..y"
	s.devices[badName] = &managedDevice{ManagedDeviceRecord: ManagedDeviceRecord{Name: badName}, unreadable: true}
	s.svc.readConfig = func(name string) ([]byte, error) {
		require.NotEqual(t, badName, name, "a malformed name has no config file")
		return []byte(name), nil
	}
	rows, err := s.registrySnapshot()
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.Equal(t, badName, rows[2].Name)
	require.True(t, rows[2].Unreadable)
	require.False(t, rows[2].Declared)
	require.Empty(t, rows[2].HubVersion)
}

func TestRegistryConfigUnderContinuousWritesKeepsNextReadFresh(t *testing.T) {
	s := registryTestServer(t)
	reads := 0
	s.svc.readConfig = func(name string) ([]byte, error) {
		reads++
		s.MarkManagedConfigChanged(name)
		return []byte("sample"), nil
	}
	version, err := s.currentConfigVersion(testClientName, s.devices[testClientName])
	require.NoError(t, err)
	require.Equal(t, fleet.VersionHash([]byte("sample")), version)
	require.Equal(t, 2, reads)
	s.svc.readConfig = func(string) ([]byte, error) { return []byte("final"), nil }
	version, err = s.currentConfigVersion(testClientName, s.devices[testClientName])
	require.NoError(t, err)
	require.Equal(t, fleet.VersionHash([]byte("final")), version)
}

// Forgetting a record while its file is being read omits that row. A new
// declaration using the same name appears with its own facts on the next read.
func TestRegistryForgetDuringConfigRead(t *testing.T) {
	for _, replace := range []bool{false, true} {
		store := heldTestStore(t)
		s := registryWithHistory(t, store)
		conn := recordTestSession(t, s)
		s.disconnectDevice(testClientName, conn)
		s.drainDeviceRecords()
		s.devices[testClientName].declared = false
		s.svc.readConfig = func(name string) ([]byte, error) {
			if name == testClientName {
				require.NoError(t, store.RemoveKey(deviceRecordKey(name)))
				s.drainDeviceRecords()
				if replace {
					s.mu.Lock()
					s.devices[name] = &managedDevice{ManagedDeviceRecord: ManagedDeviceRecord{Name: name}, declared: true}
					s.mu.Unlock()
				}
				return nil, errors.New("read from the forgotten record")
			}
			return []byte(name), nil
		}
		rows, err := s.registrySnapshot()
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, testClient2Name, rows[0].Name)
		s.svc.readConfig = func(name string) ([]byte, error) { return []byte(name), nil }
		rows, err = s.registrySnapshot()
		require.NoError(t, err)
		if replace {
			require.Len(t, rows, 2)
			require.Equal(t, testClientName, rows[0].Name)
			require.True(t, rows[0].Declared)
			require.Equal(t, fleet.VersionHash([]byte(testClientName)), rows[0].HubVersion)
		} else {
			require.Len(t, rows, 1)
			require.Equal(t, testClient2Name, rows[0].Name)
		}
	}
}

// VALIDATES: deleting a client config file clears the cached hub-version on the
// next registry read even though config deletion does not trigger write observers.
// PREVENTS: show fleet devices displaying a deleted config's stale hash as in-sync.
func TestRegistryHubVersionClearsWhenConfigRemoved(t *testing.T) {
	store := heldTestStore(t)
	s := registryWithHistory(t, store)
	s.svc.readConfig = func(name string) ([]byte, error) {
		return store.ReadFile(ClientConfigKey(name))
	}
	require.NoError(t, store.WriteFile(ClientConfigKey(testClientName), []byte("v1"), 0o600))
	got, err := s.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, fleet.VersionHash([]byte("v1")), got.HubVersion)

	require.NoError(t, store.Remove(ClientConfigKey(testClientName)))
	got, err = s.registryDevice(testClientName)
	require.NoError(t, err)
	require.Empty(t, got.HubVersion)
}

// VALIDATES: malformed config-ack JSON returns an RPC error and leaves both
// persisted ack history and doctor rejection state untouched.
// PREVENTS: corrupt ack frames overwriting recorded applied state with a blank refusal.
func TestMalformedConfigAckDoesNotCorruptHistory(t *testing.T) {
	s := registryTestServer(t)
	s.ctx = t.Context()
	conn := &rpc.MuxConn{}
	require.NoError(t, s.admitDevice(testClientName, netip.MustParseAddr("192.0.2.10"), conn))
	s.recordAck(testClientName, fleet.ConfigAck{Version: "v1", OK: true}, ManagedAckApplied)

	clientPipe, serverPipe := net.Pipe()
	defer clientPipe.Close()
	defer serverPipe.Close()
	serverEnd := rpc.NewMuxConn(rpc.NewConn(serverPipe, serverPipe))
	defer serverEnd.Close()
	var session managedSession
	go s.handleRequest(testClientName, serverEnd, &rpc.Request{ID: 7, Method: fleet.VerbConfigAck, Params: []byte("{bad")}, &session)
	raw, err := bufio.NewReader(clientPipe).ReadString('\n')
	require.NoError(t, err)
	require.Contains(t, raw, "bad config-ack payload")

	got := s.snapshotDevices()[0]
	require.Equal(t, ManagedAckApplied, got.AckResult)
	require.Equal(t, "v1", got.AckVersion)
	require.Empty(t, s.doctor.snapshot().Rejected)
}

// VALIDATES: doctor rejection detail truncation cuts on a UTF-8 rune boundary.
// PREVENTS: invalid UTF-8 sequences in doctor output for long multi-byte details.
func TestDoctorDetailTruncationPreservesUTF8(t *testing.T) {
	var d managedDoctorState
	detail := strings.Repeat("a", managedDetailMax-1) + "é"
	d.recordUnknown("alien", detail)
	d.recordRejected("edge-01", ManagedRejectConfigRejected, detail)
	for _, r := range d.snapshot().Rejected {
		require.LessOrEqual(t, len(r.Detail), managedDetailMax)
		require.True(t, utf8.ValidString(r.Detail))
	}
}
