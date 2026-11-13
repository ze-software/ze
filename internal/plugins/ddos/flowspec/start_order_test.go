// Design: docs/guide/ddos-mitigation.md -- the live-announcement record, withdrawn at start
// Related: register.go (runEngine OnConfigure, OnStarted), exit_test.go (the engine-pair harness)

package flowspec

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDetectionDuringTheHandshakeIsNotActedOn proves that a detection the bus
// delivers between configure and started reaches no responder: it sends no
// route command and no state call, and the start-time read of the record is
// the plugin's first state call. Once started, a detection is acted on.
//
// VALIDATES: the start-time withdrawal (withdrawRecordedAnnouncement, from
// OnStarted) runs before any responder can announce. The daemon refuses a
// plugin's state calls until its handshake ends, so an announcement inside
// the handshake reads its record as unreadable and goes ahead: the update is
// then either refused, and the mitigation lost, or accepted with its record
// write refused, and OnStarted then withdraws the previous run's record, which
// takes the live rule away when it names the same rule while show ddos flowspec
// still reports it.
// METHOD: runEngine over a real sdk.Plugin pair. The test drives the startup
// itself, emits one critical remote AttackDetected after the configure answer
// (the blackhole fallback announces it at once on a subscribed responder), and
// reads the engine stub's record before and after the rest of the startup.
// PREVENTS: a responder subscribed at configure, a startup stage before
// OnStarted.
//
// MUTATION: with subscribe(bus, resp) moved back into the startup OnConfigure,
// the route commands inside the handshake hold one "/flow add ".
func TestDetectionDuringTheHandshakeIsNotActedOn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e, bus := launchEngineStub(ctx, t, newStubStore(nil), "")
	e.configure(ctx)

	if _, err := ddosevent.Detected.Emit(bus, &ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17},
		Family:    ddosevent.FamilyUDPFlood,
		Severity:  ddosevent.SeverityCritical,
		Direction: ddosevent.DirectionRemote,
	}); err != nil {
		t.Fatalf("emit attack-detected: %v", err)
	}
	if cmds := e.routeCommands(); len(cmds) != 0 {
		t.Fatalf("route commands for a detection inside the handshake = %v, want none", cmds)
	}
	if calls := e.stateCalls(); len(calls) != 0 {
		t.Fatalf("state calls for a detection inside the handshake = %v, want none", calls)
	}

	e.finishStartup(ctx)

	calls := e.stateCalls()
	if len(calls) == 0 {
		t.Fatalf("the plugin made no state call at start, want the read of %s", announcementRecordKey)
	}
	if calls[0].method != rpc.MethodStateGet || calls[0].key != announcementRecordKey {
		t.Fatalf("first state call = %s %s, want %s %s: the start-time read comes first (state calls: %v)",
			calls[0].method, calls[0].key, rpc.MethodStateGet, announcementRecordKey, calls)
	}
	if cmds := e.routeCommands(); len(cmds) != 0 {
		t.Fatalf("route commands after the startup = %v, want none: the handshake detection was not seen", cmds)
	}

	// The survivor: a started plugin is subscribed, so the same detection now
	// announces.
	announceVictim(t, e, bus)
}
