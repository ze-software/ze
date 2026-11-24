// Design: docs/architecture/plugin/rib-storage-design.md -- original physical RS actors.
package rs

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Same-address named peers retain independent OPEN capabilities and UP state;
// a delayed old physical DOWN may close only its exact worker and actor.
func TestRSPhysicalActorsKeepSameAddressSiblingAndSuccessor(t *testing.T) {
	server := &routeServer{peers: make(map[workerKey]*PeerState)}
	server.workers = newWorkerPool(func(workerKey, workItem) {}, testPoolConfig())
	t.Cleanup(server.workers.Stop)
	t.Cleanup(server.stopDepartures)
	address := "192.0.2.71"
	first, err := newWorkerKey(address, 7101, 71011)
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := newWorkerKey(address, 7102, 71021)
	if err != nil {
		t.Fatal(err)
	}
	successor, err := newWorkerKey(address, 7101, 71012)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []workerKey{first, sibling, successor} {
		server.handleOpen(&Event{PeerAddr: address, PeerOwner: key.owner, ConnectionID: key.connectionID, Open: &OpenInfo{Capabilities: []CapabilityInfo{{Code: 2, Name: "route-refresh"}}}})
		server.handleState(&Event{PeerAddr: address, PeerOwner: key.owner, ConnectionID: key.connectionID, DestinationSession: key.connectionID + 100, State: "up", Reason: rpc.ReasonAttached, RSClient: true})
	}
	server.handleState(&Event{PeerAddr: address, PeerOwner: first.owner, ConnectionID: first.connectionID, State: "down", SessionGeneration: 77})
	// DOWN queues departure cleanup, which may delete the old actor while
	// the useful actors are inspected. Observe the whole map under its lock.
	server.mu.RLock()
	defer server.mu.RUnlock()
	firstActor := server.peers[first]
	stillUp := firstActor != nil && firstActor.Up
	if stillUp {
		t.Fatal("old physical actor remained up")
	}
	for _, key := range []workerKey{sibling, successor} {
		actor := server.peers[key]
		if actor == nil || !actor.Up {
			t.Fatalf("old DOWN closed useful actor %+v", key)
		}
		if !actor.hasCapability("route-refresh") {
			t.Fatalf("OPEN capabilities crossed actors %+v", key)
		}
	}
}

// Parsed lifecycle fields stay separate: physical identity is never borrowed
// from a destination replay receipt, including a deliberately missing field.
func TestRSTextPhysicalActorFieldsAreIndependent(t *testing.T) {
	for _, kind := range []string{"state up", "received open 1", "received refresh 1 family ipv4/unicast"} {
		line := "peer 192.0.2.72 remote as 65001 " + kind + " peer-owner 7201 connection-id 7202 destination-session 7203"
		var event *Event
		switch kind {
		case "state up":
			event = parseTextState(line)
		case "received open 1":
			event = parseTextOpen(line)
		default:
			event = parseTextRefresh(line)
		}
		if event == nil {
			t.Fatal("valid physical lifecycle was refused")
		}
		if event.PeerOwner != 7201 || event.ConnectionID != 7202 || event.DestinationSession != 7203 {
			t.Fatalf("identity fields conflated: %+v", event)
		}
	}
	event := parseTextState("peer 192.0.2.72 remote as 65001 state up peer-owner 7201 destination-session 7203")
	if event == nil {
		t.Fatal("unrecorded physical field should remain explicit")
	}
	if event.ConnectionID != 0 {
		t.Fatal("destination receipt invented physical identity")
	}
}

// A source-policy capture names the published generation, not the numerically
// unrelated physical connection. A successor and same-address sibling must
// remain useful while the retired generation is refused.
func TestRSSourcePolicyMatchesOriginalPublishedUPGeneration(t *testing.T) {
	server := &routeServer{peers: make(map[workerKey]*PeerState)}
	server.workers = newWorkerPool(func(workerKey, workItem) {}, testPoolConfig())
	t.Cleanup(server.workers.Stop)
	t.Cleanup(server.stopDepartures)
	for _, event := range []*Event{
		{PeerAddr: "192.0.2.74", PeerOwner: 7401, ConnectionID: 7411, SessionGeneration: 7491, DestinationSession: 7441, State: "up", Reason: rpc.ReasonAttached, RSClient: true},
		{PeerAddr: "192.0.2.74", PeerOwner: 7402, ConnectionID: 7421, SessionGeneration: 7491, DestinationSession: 7442, State: "up", Reason: rpc.ReasonAttached, RSClient: true},
		{PeerAddr: "192.0.2.74", PeerOwner: 7401, ConnectionID: 7412, SessionGeneration: 7492, DestinationSession: 7443, State: "up", Reason: rpc.ReasonAttached, RSClient: true},
	} {
		server.handleState(event)
	}
	if server.sourceUp("192.0.2.74", 7401, 7491) {
		t.Fatal("old published generation borrowed the successor connection")
	}
	current, err := server.sourceActor("192.0.2.74", 7401, 7492)
	if err != nil {
		t.Fatal(err)
	}
	if current.connectionID != 7412 {
		t.Fatal("current capture lost original physical identity")
	}
	sibling, err := server.sourceActor("192.0.2.74", 7402, 7491)
	if err != nil {
		t.Fatal(err)
	}
	if sibling.connectionID != 7421 {
		t.Fatal("capture selected another named peer at the same address")
	}
	if server.sourceUp("192.0.2.74", 7401, 0) {
		t.Fatal("missing generation became current")
	}
}
