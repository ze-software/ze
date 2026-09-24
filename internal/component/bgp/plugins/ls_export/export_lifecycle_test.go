// Design: docs/architecture/wire/nlri-bgpls.md -- native exporter lifecycle

package ls_export

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/linkstateevents"
)

type exportDelivery struct {
	refused  string
	omitted  bool
	commands map[string][]string
}

func (d *exportDelivery) UpdateRouteWithMeta(_ context.Context, peer, command string, _ map[string]any) (uint32, uint32, error) {
	if peer == d.refused {
		return 0, 0, errors.New("collector unavailable")
	}
	if d.omitted {
		d.omitted = false
		return 0, 0, nil
	}
	d.commands[peer] = append(d.commands[peer], command)
	if strings.Contains(command, " del ") {
		return 0, 1, nil
	}
	return 1, 0, nil
}

// TestNativeTopologyUnacceptedAnnouncementRemainsPending verifies that a
// warning-only dispatch cannot suppress the next attempt to advertise a route.
func TestNativeTopologyUnacceptedAnnouncementRemainsPending(t *testing.T) {
	delivery := &exportDelivery{omitted: true, commands: make(map[string][]string)}
	exporter := newTopologyExporter(delivery)
	exporter.configure(exportConfig{enabled: true})
	exporter.peerState("192.0.2.9", true)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Nodes: []linkstateevents.Node{{ID: nativeTestNode()}}}
	require.NoError(t, exporter.replace("isis", snapshot))
	require.Error(t, exporter.reconcile(context.Background()))
	require.Empty(t, delivery.commands)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, delivery.commands["192.0.2.9"], 1)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, delivery.commands["192.0.2.9"], 1)
}

// TestNativeTopologyCollectorFailureIsIsolated checks that a failed first peer
// cannot prevent the remaining collector from receiving its native topology.
func TestNativeTopologyCollectorFailureIsIsolated(t *testing.T) {
	delivery := &exportDelivery{refused: "192.0.2.8", commands: make(map[string][]string)}
	exporter := newTopologyExporter(delivery)
	exporter.configure(exportConfig{enabled: true})
	exporter.peerState("192.0.2.8", true)
	exporter.peerState("192.0.2.9", true)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Nodes: []linkstateevents.Node{{ID: nativeTestNode()}}}
	require.NoError(t, exporter.replace("isis", snapshot))
	require.Error(t, exporter.reconcile(context.Background()))
	require.Len(t, delivery.commands["192.0.2.9"], 1)
	delivery.refused = ""
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, delivery.commands["192.0.2.8"], 1)
	require.Len(t, delivery.commands["192.0.2.9"], 1)
}

// TestNativeTopologyRefreshRetainsWithdrawals changes the source immediately
// before refresh; the removed prefix must still be withdrawn before replay.
func TestNativeTopologyRefreshRetainsWithdrawals(t *testing.T) {
	exporter, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Generation: 1, Nodes: []linkstateevents.Node{{ID: nativeTestNode()}},
		Prefixes: []linkstateevents.Prefix{{Node: nativeTestNode(), Prefix: netip.MustParsePrefix("192.0.2.0/24")}}}
	require.NoError(t, exporter.replace("isis", snapshot))
	require.NoError(t, exporter.reconcile(context.Background()))
	prefix := exportCommandBytes(t, capture.commands[1], "nlri")
	snapshot.Generation++
	snapshot.Prefixes = nil
	require.NoError(t, exporter.replace("isis", snapshot))
	exporter.peerRefresh("192.0.2.9")
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.Contains(t, capture.commands[2], " del ")
	require.Equal(t, prefix, exportCommandBytes(t, capture.commands[2], "nlri"))
	require.Equal(t, capture.commands[0], capture.commands[3])
	exporter.configure(exportConfig{})
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 5)
	require.Contains(t, capture.commands[4], " del ")
	require.Equal(t, exportCommandBytes(t, capture.commands[0], "nlri"), exportCommandBytes(t, capture.commands[4], "nlri"))
}

// TestNativeTopologyConflictRetainsAcceptedDatabase checks the replacement
// boundary rather than allowing a conflicting source to stall every collector.
func TestNativeTopologyConflictRetainsAcceptedDatabase(t *testing.T) {
	exporter, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Nodes: []linkstateevents.Node{{ID: nativeTestNode(), Attributes: []linkstateevents.TLV{{Type: 1026, Value: []byte("core1")}}}}}
	require.NoError(t, exporter.replace("isis", snapshot))
	require.NoError(t, exporter.reconcile(context.Background()))
	snapshot.Nodes[0].Attributes[0].Value = []byte("conflict")
	require.Error(t, exporter.replace("other-source", snapshot))
	exporter.peerRefresh("192.0.2.9")
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Equal(t, []string{capture.commands[0], capture.commands[0]}, capture.commands)
}

type blockedExportDelivery struct {
	blocked  string
	commands map[string][]string
}

func (d *blockedExportDelivery) UpdateRouteWithMeta(ctx context.Context, peer, command string, _ map[string]any) (uint32, uint32, error) {
	if peer == d.blocked {
		<-ctx.Done()
		return 0, 0, ctx.Err()
	}
	d.commands[peer] = append(d.commands[peer], command)
	if strings.Contains(command, " del ") {
		return 0, 1, nil
	}
	return 1, 0, nil
}

func TestNativeTopologyBlockedCollectorDeadlineIsIsolated(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		delivery := &blockedExportDelivery{blocked: "192.0.2.8", commands: make(map[string][]string)}
		exporter := newTopologyExporter(delivery)
		exporter.configure(exportConfig{enabled: true})
		exporter.peerState("192.0.2.8", true)
		exporter.peerState("192.0.2.9", true)
		snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
			Nodes: []linkstateevents.Node{{ID: nativeTestNode()}}}
		require.NoError(t, exporter.replace("isis", snapshot))
		require.ErrorIs(t, exporter.reconcile(context.Background()), context.DeadlineExceeded)
		require.Len(t, delivery.commands["192.0.2.9"], 1)
		require.Contains(t, delivery.commands["192.0.2.9"][0], " add ")
	})
}

func TestNativeTopologyShutdownRetriesAndResumes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		delivery := &blockedExportDelivery{commands: make(map[string][]string)}
		exporter := newTopologyExporter(delivery)
		defer exporter.stopWorker()
		snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
			Nodes: []linkstateevents.Node{{ID: nativeTestNode()}}}
		request := func() error { return exporter.replace("isis", snapshot) }
		exporter.configure(exportConfig{enabled: true})
		exporter.peerState("192.0.2.8", true)
		exporter.peerState("192.0.2.9", true)
		require.NoError(t, exporter.start(request))
		require.NoError(t, exporter.reconcile(context.Background()))
		delivery.blocked = "192.0.2.8"
		exporter.peerRefresh("192.0.2.8")
		exporter.resume(context.Background(), exportConfig{enabled: true}, request)
		synctest.Wait()

		cleanup, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		require.ErrorIs(t, exporter.shutdown(cleanup), context.DeadlineExceeded)
		cancel()
		require.Len(t, delivery.commands["192.0.2.9"], 2)
		require.Contains(t, delivery.commands["192.0.2.9"][1], " del ")
		require.Len(t, delivery.commands["192.0.2.8"], 1)

		delivery.blocked = ""
		require.NoError(t, exporter.shutdown(context.Background()))
		require.Len(t, delivery.commands["192.0.2.8"], 2)
		require.Contains(t, delivery.commands["192.0.2.8"][1], " del ")
		exporter.resume(context.Background(), exportConfig{enabled: true}, request)
		synctest.Wait()
		for _, peer := range []string{"192.0.2.8", "192.0.2.9"} {
			require.Len(t, delivery.commands[peer], 3)
			require.Contains(t, delivery.commands[peer][2], " add ")
			require.Equal(t, exportCommandBytes(t, delivery.commands[peer][0], "nlri"), exportCommandBytes(t, delivery.commands[peer][2], "nlri"))
		}
	})
}
