//go:build integration && linux

// Design: docs/guide/vpp.md -- managed bootstrap with actual physical custody
// Related: startup_cli_test.go -- portable synchronous command transport controls
package vpp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.fd.io/govpp/api"

	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
)

// Hold the native completion at the actual manager activation entry. Both the
// statistics opener and real service-ready event/receipt stay absent until
// the typed reply is released. A pure preflight or asynchronous call fails.
func TestVPPManagedBootstrapBlocksServiceReady(t *testing.T) {
	manager := runtimeStatsTestManager(t)
	stream := &startupCLITestStream{}
	owner, transport := startupCLITestOwner(t, stream)
	manager.connector = physicalFixtureConnector(t, owner)
	manager.settings.Enabled = true
	manager.settings.StartupCLIFile = "/original/bootstrap.cli"
	owned := physicalFixtureManagedConnection(t, owner)
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	var completed atomic.Bool
	stream.receive = func() (api.Message, error) {
		close(entered)
		select {
		case <-release:
			completed.Store(true)
			return &startupCLIReply{}, nil
		case <-stream.ctx.Done():
			return nil, stream.ctx.Err()
		}
	}
	var opened, events atomic.Int32
	startup := make(chan error, 1)
	manager.startup = startup
	setVPPEventBus(runtimeStatsTestBus{emit: func(namespace, event string, _ any) {
		if namespace != vppevents.Namespace {
			t.Error("wrong readiness namespace")
		}
		if event != vppevents.EventConnected {
			t.Error("wrong readiness event")
		}
		if !completed.Load() {
			t.Error("service-ready preceded native completion")
		}
		events.Add(1)
	}})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	statsClient := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	go func() {
		defer close(result)
		lease, err := manager.activateManagedRuntime(ctx, owned, func(string) (runtimeStatsClient, error) {
			opened.Add(1)
			if !completed.Load() {
				t.Error("statistics activated before bootstrap completion")
			}
			return statsClient, nil
		})
		if lease != nil {
			lease.Close()
		}
		result <- err
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-result:
		case <-time.After(2 * time.Second):
			t.Error("bootstrap activation did not join")
		}
	})
	select {
	case <-entered:
	case err := <-result:
		t.Fatal("manager did not reach native completion barrier", err)
	case <-time.After(2 * time.Second):
		t.Fatal("manager never submitted original file")
	}
	if opened.Load() != 0 {
		t.Fatal("statistics publication overtook held native completion")
	}
	if events.Load() != 0 {
		t.Fatal("service-ready overtook held native completion")
	}
	select {
	case <-startup:
		t.Fatal("startup receipt overtook held native completion")
	default:
	}
	// An unbuffered rendezvous releases exactly this held receive; deferred
	// close is the failure-path release and cannot submit another command.
	release <- struct{}{}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("released native reply did not activate runtime")
	}
	if opened.Load() != 1 {
		t.Fatal("statistics activation count", opened.Load())
	}
	if events.Load() != 1 {
		t.Fatal("service-ready count", events.Load())
	}
	if transport.streams.Load() != 1 {
		t.Fatal("managed bootstrap duplicated native execution")
	}
	if stream.sends != 1 {
		t.Fatal("managed bootstrap submitted its original file more than once")
	}
	select {
	case err := <-startup:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("native success never published startup receipt")
	}
}

// A failed native program never opens statistics or publishes readiness.
func TestVPPManagedBootstrapFailureStopsActivation(t *testing.T) {
	manager := runtimeStatsTestManager(t)
	stream := &startupCLITestStream{reply: &startupCLIReply{Retval: -1, Reply: "bad original command"}}
	owner, _ := startupCLITestOwner(t, stream)
	manager.connector = physicalFixtureConnector(t, owner)
	manager.settings.Enabled = true
	manager.settings.StartupCLIFile = "/original/bootstrap.cli"
	owned := physicalFixtureManagedConnection(t, owner)
	calls := 0
	events := 0
	setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) { events++ }})
	lease, err := manager.activateManagedRuntime(t.Context(), owned, func(string) (runtimeStatsClient, error) {
		calls++
		return &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}, nil
	})
	if lease != nil {
		lease.Close()
		t.Error("native failure returned a statistics lease")
	}
	if err == nil {
		t.Fatal("native error became successful startup")
	}
	if calls != 0 {
		t.Fatal("native error still opened statistics")
	}
	if events != 0 {
		t.Fatal("native error published service-ready")
	}
	if manager.ready {
		t.Fatal("native error set ready")
	}
}

// Missing optional leaf and external mode keep their existing activation;
// the supervisor's configured file is neither opened nor sent to native VPP.
func TestVPPManagedBootstrapOptionalAndExternalNoop(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "external"}[external], func(t *testing.T) {
			manager := runtimeStatsTestManager(t)
			stream := &startupCLITestStream{reply: &startupCLIReply{}}
			owner, transport := startupCLITestOwner(t, stream)
			manager.connector = physicalFixtureConnector(t, owner)
			manager.settings.Enabled = true
			manager.settings.External = external
			if external {
				manager.settings.StartupCLIFile = "/unreadable/supervisor.cli"
			}
			calls := 0
			lease, err := manager.activateManagedRuntime(t.Context(), managedConnection{}, func(string) (runtimeStatsClient, error) {
				calls++
				return &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if lease == nil {
				t.Fatal("existing default activation refused")
			}
			lease.Close()
			if calls != 1 {
				t.Fatal("existing runtime activation count", calls)
			}
			if transport.streams.Load() != 0 {
				t.Fatal("absent or externally owned file executed")
			}
		})
	}
}

// A physical binding belongs to exactly its connector and a live nonzero
// epoch. Invalid receipts never submit native commands or open statistics.
func TestVPPManagedBootstrapRequiresBoundOwner(t *testing.T) {
	for _, wrong := range []string{"connector", "zero-generation", "stale-generation"} {
		t.Run(wrong, func(t *testing.T) {
			manager := runtimeStatsTestManager(t)
			stream := &startupCLITestStream{reply: &startupCLIReply{}}
			owner, transport := startupCLITestOwner(t, stream)
			manager.connector = physicalFixtureConnector(t, owner)
			manager.settings.StartupCLIFile = "/original/bootstrap.cli"
			owned := physicalFixtureManagedConnection(t, owner)
			switch wrong {
			case "connector":
				owned.connector++
			case "zero-generation":
				owned.generation = 0
			case "stale-generation":
				owned.generation = 6
			}
			calls := 0
			lease, err := manager.activateManagedRuntime(t.Context(), owned, func(string) (runtimeStatsClient, error) {
				calls++
				return &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}, nil
			})
			if lease != nil {
				lease.Close()
				t.Error("invalid binding returned statistics")
			}
			if err == nil {
				t.Fatal("invalid binding admitted readiness")
			}
			if calls != 0 {
				t.Fatal("invalid binding opened statistics")
			}
			if transport.streams.Load() != 0 {
				t.Fatal("invalid binding selected native transport")
			}
		})
	}
}

// The statistics activation must retain the bootstrap's exact epoch rather
// than snapshot a replacement and publish its service-ready event.
func TestVPPManagedBootstrapKeepsGenerationThroughReadiness(t *testing.T) {
	t.Run("entry-gap", func(t *testing.T) {
		manager := runtimeStatsTestManager(t)
		stream := &startupCLITestStream{reply: &startupCLIReply{}}
		owner, _ := startupCLITestOwner(t, stream)
		manager.connector = physicalFixtureConnector(t, owner)
		owner.generation = 8
		calls := 0
		lease, err := manager.activateRuntimeStatsGeneration(t.Context(), 7, func(string) (runtimeStatsClient, error) {
			calls++
			return &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}, nil
		})
		if lease != nil {
			lease.Close()
			t.Error("stale bootstrap epoch returned statistics")
		}
		if !errors.Is(err, errVppGenerationChanged) {
			t.Fatal("activation replaced bootstrap epoch", err)
		}
		if calls != 0 {
			t.Fatal("stale bootstrap epoch opened statistics")
		}
	})
	t.Run("during-statistics", func(t *testing.T) {
		manager := runtimeStatsTestManager(t)
		stream := &startupCLITestStream{reply: &startupCLIReply{}}
		owner, _ := startupCLITestOwner(t, stream)
		manager.connector = physicalFixtureConnector(t, owner)
		manager.settings.StartupCLIFile = "/original/bootstrap.cli"
		owned := physicalFixtureManagedConnection(t, owner)
		events := 0
		client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
		setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) { events++ }})
		lease, err := manager.activateManagedRuntime(t.Context(), owned, func(string) (runtimeStatsClient, error) {
			owner.mu.Lock()
			owner.generation = 8
			owner.mu.Unlock()
			return client, nil
		})
		if lease != nil {
			lease.Close()
			t.Error("replacement epoch retained statistics")
		}
		if !errors.Is(err, errVppGenerationChanged) {
			t.Fatal("replacement epoch passed bootstrap readiness", err)
		}
		if events != 0 {
			t.Fatal("replacement epoch published service-ready")
		}
		if manager.ready {
			t.Fatal("replacement epoch set ready")
		}
		if GetActiveStatsProvider() != nil {
			t.Fatal("replacement epoch retained publication")
		}
		if client.disconnects != 1 {
			t.Fatal("replacement epoch leaked native statistics")
		}
	})
}

// The manager's entry observation can become stale before the actual stats
// helper is called. Pass that captured epoch into the real helper after the
// controlled replacement, rather than replacing inside the connect callback.
// A stable original epoch remains a positive connected/published control.
func TestVPPManagedBootstrapRetainsEpochAtStatsEntry(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "entry-gap"}[replace], func(t *testing.T) {
			manager := runtimeStatsTestManager(t)
			stream := &startupCLITestStream{reply: &startupCLIReply{}}
			owner, _ := startupCLITestOwner(t, stream)
			manager.connector = physicalFixtureConnector(t, owner)
			observed, err := owner.ConnectionGeneration()
			if err != nil {
				t.Fatal(err)
			}
			if observed != 7 {
				t.Fatal("original entry observation did not name bootstrap epoch")
			}
			if replace {
				owner.mu.Lock()
				owner.generation = 8
				owner.mu.Unlock()
			}
			calls := 0
			client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
			lease, err := manager.connectRuntimeStatsGeneration(t.Context(), observed, func(string) (runtimeStatsClient, error) { calls++; return client, nil })
			if replace {
				if lease != nil {
					lease.Close()
					t.Error("entry gap returned replacement statistics")
				}
				if !errors.Is(err, errVppGenerationChanged) {
					t.Fatal("stats helper replaced retained bootstrap epoch", err)
				}
				if calls != 0 {
					t.Fatal("stats helper selected a connection after original epoch changed", calls)
				}
				if GetActiveStatsProvider() != nil {
					t.Fatal("entry gap published replacement statistics")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if lease == nil {
				t.Fatal("stable original epoch lost statistics")
			}
			defer lease.Close()
			if calls != 1 {
				t.Fatal("stable original stats connection count", calls)
			}
			if lease.generation != observed {
				t.Fatal("stable lease changed captured generation")
			}
			if GetActiveStatsProvider() != lease {
				t.Fatal("stable original lease was not published")
			}
		})
	}
}

// A last-moment replacement, loss or cancellation cannot install even a
// previously valid original lease. This calls the real publication admission;
// a stable live original lease must still install successfully.
func TestVPPManagedBootstrapStatsPublicationRequiresOriginalEpoch(t *testing.T) {
	for _, outcome := range []string{"original", "replacement", "disconnected", "missing-transport", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			manager := runtimeStatsTestManager(t)
			stream := &startupCLITestStream{reply: &startupCLIReply{}}
			owner, _ := startupCLITestOwner(t, stream)
			manager.connector = physicalFixtureConnector(t, owner)
			lease := &runtimeStats{owner: owner, generation: 7, client: &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}}
			defer lease.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch outcome {
			case "original":
			case "replacement":
				owner.generation = 8
			case "disconnected":
				owner.connected = false
			case "missing-transport":
				owner.conn = nil
			case "canceled":
				cancel()
			}
			err := lease.publish(ctx)
			if outcome == "original" {
				if err != nil {
					t.Fatal(err)
				}
				if GetActiveStatsProvider() != lease {
					t.Fatal("stable original stats publication refused")
				}
				return
			}
			if err == nil {
				t.Fatal("invalid late statistics publication admitted", outcome)
			}
			if outcome == "replacement" {
				if !errors.Is(err, errVppGenerationChanged) {
					t.Fatal("late replacement lost epoch cause", err)
				}
			}
			if outcome == "canceled" {
				if !errors.Is(err, context.Canceled) {
					t.Fatal("late cancellation lost original cause", err)
				}
			}
			if GetActiveStatsProvider() != nil {
				t.Fatal("invalid late lease escaped publication admission", outcome)
			}
		})
	}
}
