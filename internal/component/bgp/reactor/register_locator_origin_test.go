package reactor

import (
	"context"
	"encoding/json"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/locatororigin"
)

// TestReactorLocatorOriginSubscriptionAndCleanup verifies the daemon EventBus
// wiring between IGP locator-originator snapshots and the BGP reactor, across
// typed in-process emits, JSON string emits matching forked plugin transport,
// StartPeers replay requests, process-generation disconnect cleanup, and
// reactor stop unsubscription.
//
// VALIDATES: StartWithContext subscribes locator-origin:snapshot into the
// in-daemon index for both typed and JSON-encoded emits, StartPeers requests a
// replay before peers open, disconnect cleanup clears the stopped source and
// allows a replacement generation 1, and reactor stop drops the subscription.
// PREVENTS: BGP receiving MUP updates with a stale or empty locator index after
// a forked IGP plugin emits JSON, starts before BGP, or exits abruptly.
func TestReactorLocatorOriginSubscriptionAndCleanup(t *testing.T) {
	t.Cleanup(locatororigin.ResetForTest)
	locatororigin.ResetForTest()

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	r := New(&Config{})
	r.SetPluginServer(srv)
	r.SetEventBus(srv)

	require.NoError(t, r.StartWithContext(context.Background()))
	stopped := false
	stopReactor := func() {
		if stopped {
			return
		}
		stopped = true
		r.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = r.Wait(waitCtx)
	}
	t.Cleanup(stopReactor)

	loc1 := netip.MustParsePrefix("fc00:1::/48")
	sid1 := netip.MustParseAddr("fc00:1::1")
	addr1 := netip.MustParseAddr("2001:db8::1")

	_, err = locatororigin.SnapshotEvent.Emit(srv, &locatororigin.Snapshot{
		Source:     "isis",
		Generation: 1,
		Active:     true,
		Entries: []locatororigin.LocatorEntry{
			{
				Prefix:     loc1,
				Known:      true,
				Originator: "0100.0000.0001",
				Addresses:  []netip.Addr{addr1},
			},
		},
	})
	require.NoError(t, err)

	res := locatororigin.Resolve(sid1)
	require.True(t, res.Known)
	assert.Equal(t, "0100.0000.0001", res.Originator)
	assert.Equal(t, []netip.Addr{addr1}, res.Addresses)

	// Forked and external plugins emit compact JSON strings through
	// pluginserver.deliverEvent, which decodes registered typed events into
	// *locatororigin.Snapshot before invoking engine subscribers.
	loc2 := netip.MustParsePrefix("fc00:2::/48")
	sid2 := netip.MustParseAddr("fc00:2::1")
	addr2 := netip.MustParseAddr("2001:db8::2")
	rawSnap, err := json.Marshal(&locatororigin.Snapshot{
		Source:     "isis",
		Generation: 2,
		Active:     true,
		Entries: []locatororigin.LocatorEntry{
			{
				Prefix:     loc2,
				Known:      true,
				Originator: "0100.0000.0002",
				Addresses:  []netip.Addr{addr2},
			},
		},
	})
	require.NoError(t, err)
	_, err = srv.EmitEngineEvent(locatororigin.Namespace, locatororigin.EventSnapshot, string(rawSnap))
	require.NoError(t, err)

	res1 := locatororigin.Resolve(sid1)
	assert.False(t, res1.Known)
	assert.Equal(t, locatororigin.ReasonNoCoveringLocator, res1.ReasonClass)

	res2 := locatororigin.Resolve(sid2)
	require.True(t, res2.Known)
	assert.Equal(t, "0100.0000.0002", res2.Originator)
	assert.Equal(t, []netip.Addr{addr2}, res2.Addresses)

	// StartPeers emits locator-origin:snapshot-request before opening peers.
	var replayRequests atomic.Int32
	unsubReplay := locatororigin.SnapshotRequest.Subscribe(srv, func() {
		replayRequests.Add(1)
	})
	t.Cleanup(unsubReplay)
	require.NoError(t, r.StartPeers())
	assert.Equal(t, int32(1), replayRequests.Load())

	// Disconnect cleanup clears the ended source and resets its generation
	// floor so a replacement incarnation starting at generation 1 applies.
	proc := process.NewProcess(plugin.PluginConfig{Name: "isis"})
	locatorOriginGenerationCleanup(srv, proc)
	cleared := locatororigin.Resolve(sid2)
	assert.False(t, cleared.Known)
	assert.Equal(t, locatororigin.ReasonNoActiveSource, cleared.ReasonClass)

	_, err = locatororigin.SnapshotEvent.Emit(srv, &locatororigin.Snapshot{
		Source:     "isis",
		Generation: 1,
		Active:     true,
		Entries: []locatororigin.LocatorEntry{
			{
				Prefix:     loc1,
				Known:      true,
				Originator: "0100.0000.0009",
				Addresses:  []netip.Addr{addr1},
			},
		},
	})
	require.NoError(t, err)
	reborn := locatororigin.Resolve(sid1)
	require.True(t, reborn.Known)
	assert.Equal(t, "0100.0000.0009", reborn.Originator)

	// Stopping the reactor releases its EventBus subscription so later emits do
	// not mutate the index through a stopped reactor.
	stopReactor()
	locatororigin.ResetForTest()
	_, err = locatororigin.SnapshotEvent.Emit(srv, &locatororigin.Snapshot{
		Source:     "isis",
		Generation: 3,
		Active:     true,
		Entries: []locatororigin.LocatorEntry{
			{
				Prefix:     loc1,
				Known:      true,
				Originator: "0100.0000.0003",
				Addresses:  []netip.Addr{addr1},
			},
		},
	})
	require.NoError(t, err)
	afterStop := locatororigin.Resolve(sid1)
	assert.False(t, afterStop.Known)
	assert.Equal(t, locatororigin.ReasonNoActiveSource, afterStop.ReasonClass)
}
