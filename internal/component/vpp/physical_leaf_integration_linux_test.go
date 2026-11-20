//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- physical startup ownership
// Related: process_peer_integration_linux_test.go -- genuine child-owned Unix peers
// These tests use genuine kernel peer descriptors with the original fake
// counter payload. They prove custody and admission, not VPP packet forwarding.
package vpp

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/memclnt"
)

// Give a foreign physical process the same connector epoch and native counter
// index. Its genuine peer must refuse before counters, publication or readiness.
// The matching-peer case preserves the original positive 2048-byte payload.
func TestVPPPhysicalLeafStatsRequiresOriginalKernelPeer(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign", true: "same"}[same], func(t *testing.T) {
			t.Chdir(t.TempDir())
			firstPath, secondPath := "a.sock", "b.sock"
			startPeerProcess(t, firstPath)
			startPeerProcess(t, secondPath)
			apiPeer := capturePeerSocket(t, firstPath)
			statsPath := secondPath
			if same {
				statsPath = firstPath
			}
			statsPeer := capturePeerSocket(t, statsPath)
			manager := runtimeStatsTestManager(t)
			manager.connector.conn = &physicalTestConnection{peer: apiPeer}
			client := &physicalStatsTestClient{peer: statsPeer}
			events := 0
			setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) { events++ }})
			lease, err := manager.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
			if !same {
				if !errors.Is(err, errVppStatsPeerMismatch) {
					t.Fatal("foreign physical statistics did not refuse", err)
				}
				if lease != nil {
					lease.Close()
					t.Error("foreign physical peer retained a lease")
				}
				if client.reads != 0 {
					t.Fatal("foreign physical peer supplied native counters")
				}
				if client.disconnects != 1 {
					t.Fatal("foreign original stats client was not closed")
				}
				if events != 0 {
					t.Fatal("foreign physical peer published readiness")
				}
				if GetActiveStatsProvider() != nil {
					t.Fatal("foreign physical statistics were published")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if lease == nil {
				t.Fatal("same physical peer lost original counter lease")
			}
			defer lease.Close()
			row, err := InterfaceCountersForGeneration(manager.connector, 7, 17, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if publishedCounterDumpRows(t) != 1 {
				t.Fatal("original counter row population changed")
			}
			if row.InterfaceIndex != 17 {
				t.Fatal("original counter identity changed")
			}
			if row.Rx.Bytes != 2048 {
				t.Fatal("original counter payload changed")
			}
			if events != 1 {
				t.Fatal("same physical peer readiness count", events)
			}
		})
	}
}

// Final provider admission must compare physical peers while the original API
// generation is still locked. A same-number replacement is not the bootstrap
// peer even when both retained descriptors and both processes remain live.
func TestVPPPhysicalLeafPublicationRejectsSameEpochForeignPeer(t *testing.T) {
	t.Chdir(t.TempDir())
	firstPath, secondPath := "a.sock", "b.sock"
	startPeerProcess(t, firstPath)
	startPeerProcess(t, secondPath)
	firstAPI, firstStats := capturePeerSocket(t, firstPath), capturePeerSocket(t, firstPath)
	foreignAPI := capturePeerSocket(t, secondPath)
	manager := runtimeStatsTestManager(t)
	manager.connector.conn = &physicalTestConnection{peer: firstAPI}
	client := &physicalStatsTestClient{peer: firstStats}
	lease := &runtimeStats{owner: manager.connector, generation: 7, client: client}
	defer lease.Close()
	if err := lease.GetInterfaceStats(new(api.InterfaceStats)); err != nil {
		t.Fatal("original peer positive read refused", err)
	}
	manager.connector.mu.Lock()
	manager.connector.conn = &physicalTestConnection{peer: foreignAPI}
	manager.connector.mu.Unlock()
	if err := lease.publish(t.Context()); !errors.Is(err, errVppStatsPeerMismatch) {
		t.Fatal("same-epoch foreign peer passed final physical admission", err)
	}
	if GetActiveStatsProvider() != nil {
		t.Fatal("late foreign peer published original counters")
	}
	manager.connector.mu.Lock()
	manager.connector.conn = &physicalTestConnection{peer: firstAPI}
	manager.connector.mu.Unlock()
	if err := lease.publish(t.Context()); err != nil {
		t.Fatal("original peer positive publication refused", err)
	}
	if GetActiveStatsProvider() != lease {
		t.Fatal("original physical peer publication missing")
	}
}

// A claimed matching child PID cannot repair an actual foreign Unix peer.
// The positive uses the atomic clone witness and a separately captured socket
// of that exact child; binding must ask neither API for an identity claim.
func TestVPPPhysicalLeafManagedBindingIgnoresClaimedPID(t *testing.T) {
	t.Chdir(t.TempDir())
	childPath, foreignPath := "a.sock", "b.sock"
	child := startManagedPeerFixture(t, childPath, false)
	foreign := startManagedPeerFixture(t, foreignPath, false)
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign", true: "same"}[same], func(t *testing.T) {
			path := foreignPath
			if same {
				path = childPath
			}
			peer := capturePeerSocket(t, path)
			connector := NewConnector("model-original")
			wire := &physicalTestConnection{peer: peer}
			wire.open = func(ctx context.Context) (api.Stream, error) {
				return &startupCLITestStream{ctx: ctx, reply: &memclnt.ControlPingReply{VpePID: uint32(child.reportedPID)}}, nil
			}
			connector.conn, connector.connected, connector.generation = wire, true, 7
			owned, err := connector.bindManagedProcess(t.Context(), child.witness)
			if !same {
				if err == nil {
					t.Fatal("claimed child PID admitted a genuinely foreign Unix peer")
				}
				if owned != (managedConnection{}) {
					t.Fatal("foreign physical refusal retained managed authority")
				}
			}
			if same {
				if err != nil {
					t.Fatal(err)
				}
				if owned.physical != child.witness.identity {
					t.Fatal("managed binding lost actual clone identity")
				}
			}
			if wire.streams.Load() != 0 {
				t.Fatal("self-reported PID was consulted as physical authority")
			}
		})
	}
	if err := foreign.stop(); err != nil {
		t.Fatal(err)
	}
}
