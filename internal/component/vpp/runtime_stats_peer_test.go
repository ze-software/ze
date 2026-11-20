//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- matching actual stats peers
package vpp

import (
	"errors"
	"testing"
	"time"

	"go.fd.io/govpp/api"

	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
)

type physicalStatsTestClient struct {
	runtimeStatsTestClient
	peer *processWitness
}

func (c *physicalStatsTestClient) processPeer() *processWitness { return c.peer }

type physicalTestConnection struct {
	recordingConnection
	peer *processWitness
}

func (c *physicalTestConnection) processPeer() *processWitness { return c.peer }

// Give the foreign stats process the same native interface index and same
// logical Connector generation. Only the actual kernel peer distinguishes it.
// MUTATION: removing the physical identity comparison publishes foreign counters.
func TestVPPForeignStatsProcessRefusesReadiness(t *testing.T) {
	manager := runtimeStatsTestManager(t)
	apiPeer, _ := newPeerTestWitness(t, 101)
	statsPeer, _ := newPeerTestWitness(t, 102)
	manager.connector.conn = &physicalTestConnection{peer: apiPeer}
	client := &physicalStatsTestClient{peer: statsPeer}
	events := 0
	setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) { events++ }})
	lease, err := manager.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if !errors.Is(err, errVppStatsPeerMismatch) {
		t.Fatalf("foreign stats failure=%v", err)
	}
	if lease != nil {
		t.Fatal("foreign statistics were retained")
	}
	if manager.ready {
		t.Fatal("foreign statistics granted readiness")
	}
	if events != 0 {
		t.Fatal("foreign statistics emitted readiness")
	}
	if client.reads != 0 {
		t.Fatal("foreign native index was read before validating its process")
	}
	if client.disconnects != 1 {
		t.Fatalf("foreign stats disconnects=%d", client.disconnects)
	}
	if GetActiveStatsProvider() != nil {
		t.Fatal("foreign stats provider was published")
	}
}

// The two sockets own independent descriptors for one actual process. A
// matching pair must publish counters; this prevents a reject-everything guard.
func TestVPPSamePhysicalStatsProcessPublishesCounters(t *testing.T) {
	manager := runtimeStatsTestManager(t)
	apiPeer, _ := newPeerTestWitness(t, 111)
	statsPeer, _ := newPeerTestWitness(t, 111)
	manager.connector.conn = &physicalTestConnection{peer: apiPeer}
	client := &physicalStatsTestClient{peer: statsPeer}
	events := 0
	setVPPEventBus(runtimeStatsTestBus{emit: func(_, event string, _ any) {
		if event != vppevents.EventConnected {
			t.Fatal(event)
		}
		events++
	}})
	lease, err := manager.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("matching stats mapping was not published")
	}
	defer lease.Close()
	row, err := InterfaceCountersForGeneration(manager.connector, 7, 17, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("readiness events=%d", events)
	}
	if row.InterfaceIndex != 17 {
		t.Fatal("counter index changed")
	}
	if row.Rx.Bytes != 2048 {
		t.Fatal("counter payload changed")
	}
}

// Kernel-confirmed death during a read invalidates its populated result even
// when the logical generation has not yet changed in the API health worker.
// MUTATION: removing the post-read check accepts a dead owner's counters.
func TestVPPStatsProcessDeathDuringReadRefusesSnapshot(t *testing.T) {
	manager := runtimeStatsTestManager(t)
	apiPeer, descriptor := newPeerTestWitness(t, 121)
	statsPeer, _ := newPeerTestWitness(t, 121)
	manager.connector.conn = &physicalTestConnection{peer: apiPeer}
	client := &physicalStatsTestClient{peer: statsPeer}
	lease, err := manager.connectRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	client.read = func(out *api.InterfaceStats) error {
		out.Interfaces = []api.InterfaceCounters{{InterfaceIndex: 17}}
		descriptor.mu.Lock()
		descriptor.dead = true
		descriptor.mu.Unlock()
		return nil
	}
	var snapshot api.InterfaceStats
	if err := lease.GetInterfaceStats(&snapshot); err == nil {
		t.Fatal("dead owner's populated snapshot was accepted")
	}
}
