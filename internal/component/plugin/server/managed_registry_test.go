// VALIDATES: the hub records authenticated session, fetch and ack facts and rejects
// competing sessions before they can replace the source or connection time.
// PREVENTS: the device list reporting a stale connection or a previous fetch/ack.

package server

import (
	"errors"
	"net/netip"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (s *ManagedServer) admitDevice(name string, addr netip.Addr, conn *rpc.MuxConn) error {
	return s.admitOnListener(name, addr, conn, nil, "")
}

// snapshotDevices reads just the session facts, without consulting storage.
func (s *ManagedServer) snapshotDevices() []ManagedDevice {
	s.mu.Lock()
	devices := make([]ManagedDevice, 0, len(s.devices))
	for _, entry := range s.devices {
		devices = append(devices, entry.snapshotLocked())
	}
	s.mu.Unlock()
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices
}

func registryTestServer(t *testing.T) *ManagedServer {
	t.Helper()
	s, err := NewManagedServer(ManagedServerConfig{
		ReadConfig: func(string) ([]byte, error) { return []byte(testClientConfig), nil },
		Authority:  newTestCA(t),
		Listeners: []ManagedListener{{Addr: "127.0.0.1:0", Clients: map[string]string{
			testClientName: testClientSecret, testClient2Name: testClient2Secret,
		}}},
	})
	require.NoError(t, err)
	return s
}

func TestRegistryFirstConnectRecordsConnected(t *testing.T) {
	s, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	resp := fetch(t, mc, "")
	got, err := s.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, testClientName, got.Name)
	require.True(t, got.Connected)
	require.Equal(t, netip.MustParseAddr("127.0.0.1"), got.SourceAddress)
	require.False(t, got.FirstConnected.IsZero())
	require.Equal(t, got.FirstConnected, got.ConnectedSince)
	require.True(t, got.LastSeen.IsZero())
	require.Equal(t, resp.Version, got.HubVersion)
	require.Equal(t, ManagedFetchServed, got.LastFetchResult)
}

func TestRegistryReconnectKeepsFirstConnected(t *testing.T) {
	s := registryTestServer(t)
	first, second := &rpc.MuxConn{}, &rpc.MuxConn{}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start
	s.now = func() time.Time { return now }
	firstAddr := netip.MustParseAddr("192.0.2.1")
	secondAddr := netip.MustParseAddr("192.0.2.2")
	require.NoError(t, s.admitDevice(testClientName, firstAddr, first))
	now = start.Add(time.Hour)
	require.ErrorIs(t, s.admitDevice(testClientName, secondAddr, second), ErrDuplicateClient)
	require.Equal(t, firstAddr, s.snapshotDevices()[0].SourceAddress)
	s.disconnectDevice(testClientName, first)
	offline := s.snapshotDevices()[0]
	require.False(t, offline.Connected)
	require.Equal(t, now, offline.LastSeen)
	require.Equal(t, start, offline.ConnectedSince)
	now = start.Add(2 * time.Hour)
	require.NoError(t, s.admitDevice(testClientName, secondAddr, second))
	online := s.snapshotDevices()[0]
	require.True(t, online.Connected)
	require.Equal(t, secondAddr, online.SourceAddress)
	require.Equal(t, start, online.FirstConnected)
	require.Equal(t, now, online.ConnectedSince)
	require.Equal(t, offline.LastSeen, online.LastSeen)
}

func TestRegistryListsDeclaredNeverConnected(t *testing.T) {
	s := registryTestServer(t)
	devices := s.snapshotDevices()
	require.Len(t, devices, 2)
	for _, device := range devices {
		require.True(t, device.Declared)
		require.False(t, device.Connected)
		require.True(t, device.FirstConnected.IsZero())
		require.True(t, device.LastFetch.IsZero())
	}
}

func TestRegistryDisconnectRecordsLastSeen(t *testing.T) {
	s := registryTestServer(t)
	conn := &rpc.MuxConn{}
	require.NoError(t, s.admitDevice(testClientName, netip.MustParseAddr("192.0.2.1"), conn))
	end := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return end }
	s.disconnectDevice(testClientName, conn)
	got := s.snapshotDevices()[0]
	require.False(t, got.Connected)
	require.Equal(t, end, got.LastSeen)
}

func TestRegistryDuplicateRefused(t *testing.T) {
	s := registryTestServer(t)
	first, duplicate := &rpc.MuxConn{}, &rpc.MuxConn{}
	require.NoError(t, s.admitDevice(testClientName, netip.MustParseAddr("192.0.2.1"), first))
	before := s.snapshotDevices()[0]
	require.ErrorIs(t, s.admitDevice(testClientName, netip.MustParseAddr("192.0.2.2"), duplicate), ErrDuplicateClient)
	s.disconnectDevice(testClientName, duplicate)
	require.Equal(t, before, s.snapshotDevices()[0])
}

func TestRegistryFetchRecordsVersionAndResult(t *testing.T) {
	s, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	resp := fetch(t, mc, "")
	current := fetch(t, mc, resp.Version)
	require.Equal(t, "current", current.Status)
	got, err := s.registryDevice(testClientName)
	require.NoError(t, err)
	require.False(t, got.LastFetch.IsZero())
	require.Equal(t, ManagedFetchCurrent, got.LastFetchResult)
	require.Equal(t, resp.Version, got.DeviceVersion)
	require.Equal(t, resp.Version, got.HubVersion)

	failure := strings.Repeat("x", managedDetailMax+1)
	s.recordFetch(testClientName, "other", fleet.ConfigFetchResponse{}, errors.New(failure))
	got, err = s.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, ManagedFetchError, got.LastFetchResult)
	require.Equal(t, "other", got.DeviceVersion)
	require.Equal(t, failure[:managedDetailMax], got.LastFetchError)
	require.Equal(t, resp.Version, got.HubVersion, "a fetch error does not remove the hub's configuration")
	s.recordFetch(testClientName, "other", resp, nil)
	require.Empty(t, s.snapshotDevices()[0].LastFetchError)
}

func TestRegistryLateEventsLeaveForgottenDeviceAbsent(t *testing.T) {
	s := registryTestServer(t)
	delete(s.devices, testClientName)
	s.recordFetch(testClientName, "previous", fleet.ConfigFetchResponse{}, nil)
	s.recordAck(testClientName, fleet.ConfigAck{Version: "previous"}, ManagedAckApplied)
	require.NotContains(t, s.devices, testClientName)
}

func TestRegistryAckRefusedBoundsError(t *testing.T) {
	s := registryTestServer(t)
	message := strings.Repeat("z", managedDetailMax+2)
	s.handleConfigAck(testClientName, fleet.ConfigAck{Version: "new", Error: message}, "")
	got := s.snapshotDevices()[0]
	require.False(t, got.LastAck.IsZero())
	require.Equal(t, ManagedAckRefused, got.AckResult)
	require.Equal(t, "new", got.AckVersion)
	require.Equal(t, message[:managedDetailMax], got.AckError)
}

func TestRegistryAckAppliedRecorded(t *testing.T) {
	s := registryTestServer(t)
	s.handleConfigAck(testClientName, fleet.ConfigAck{Error: "prior refusal"}, "")
	s.handleConfigAck(testClientName, fleet.ConfigAck{OK: true, Version: "applied", Error: "ignored on success"}, "")
	got := s.snapshotDevices()[0]
	require.Equal(t, ManagedAckApplied, got.AckResult)
	require.Equal(t, "applied", got.AckVersion)
	require.Empty(t, got.AckError)
}

func TestRegistryAckHoldingRecorded(t *testing.T) {
	s, mc := heldTestServer(t, heldTestStore(t))
	sendHeld(t, s, mc, heldClientText)
	hubVersion := fleet.VersionHash([]byte(heldHubText))
	s.handleConfigAck(testClientName, fleet.ConfigAck{Version: hubVersion, Holding: true, Error: "holding local edits"}, fleet.VersionHash([]byte(heldClientText)))
	got := s.snapshotDevices()[0]
	require.Equal(t, ManagedAckHolding, got.AckResult)
	require.Equal(t, hubVersion, got.AckVersion)
	require.Equal(t, "holding local edits", got.AckError)
}
