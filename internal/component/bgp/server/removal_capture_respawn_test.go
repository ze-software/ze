// Design: docs/architecture/api/process-protocol.md -- OPEN observer removal lifetime.
// Related: event_dispatcher.go -- CapturePeerStateChange, the capture under the reactor lock
// Related: events.go -- addOpenRemovalObservers, the derivation under test

package server

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The four plugins of a removal-capture server, one per combination of
// "validates OPEN" and "subscribes to BGP state".
const (
	removalCaptureStuck        = "removal-capture-stuck"        // validator, subscribed, handler waits on the gate
	removalCaptureObserver     = "removal-capture-observer"     // validator, subscribed
	removalCaptureUnsubscribed = "removal-capture-unsubscribed" // validator, no subscription
	removalCapturePlain        = "removal-capture-plain"        // subscribed, validates nothing
)

// removalCaptureServer is a started plugin server running the four plugins
// above in the daemon, through real SDK startup.
type removalCaptureServer struct {
	srv *pluginserver.Server
	ctx context.Context
	// observerEvents counts the events the observer's handler received.
	observerEvents *atomic.Int64
}

// startRemovalCaptureServer registers and starts the four plugins. The stuck
// plugin's event handler signals inFlight (without blocking) on each call, then
// waits to read-lock gate, which stands in for the reactor's lock: a test that
// holds gate for writing parks the handler on the process's delivery goroutine,
// as an in-daemon plugin that calls back into the engine parks on r.mu.
func startRemovalCaptureServer(t *testing.T, gate *sync.RWMutex, inFlight chan<- struct{}) removalCaptureServer {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	observerEvents := &atomic.Int64{}
	specs := []struct {
		name      string
		validates bool
		subscribe bool
		handler   func(string) error
	}{
		{removalCaptureStuck, true, true, func(string) error {
			select {
			case inFlight <- struct{}{}:
			default:
			}
			gate.RLock()
			defer gate.RUnlock()
			return nil
		}},
		{removalCaptureObserver, true, true, func(string) error {
			observerEvents.Add(1)
			return nil
		}},
		{removalCaptureUnsubscribed, true, false, func(string) error { return nil }},
		{removalCapturePlain, false, true, func(string) error { return nil }},
	}
	var plugins []plugin.PluginConfig
	for _, spec := range specs {
		err := registry.Register(registry.Registration{
			Name: spec.name, Description: "removal capture test",
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(spec.name, conn)
				defer p.Close() //nolint:errcheck // Test connection teardown.
				if spec.validates {
					p.OnValidateOpen(func(*sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
						return &sdk.ValidateOpenOutput{Accept: true}
					})
				}
				if spec.subscribe {
					p.SetStartupSubscriptionsIn(bgpevents.Namespace, []string{"state"}, nil, "full")
				}
				p.OnEvent(spec.handler)
				if err := p.Run(ctx, &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
			CLIHandler: func([]string) int { return 0 },
		})
		if err != nil {
			t.Fatal(err)
		}
		plugins = append(plugins, plugin.PluginConfig{Name: spec.name, Internal: true, Run: "ze." + spec.name, Encoder: plugin.EncodingJSON})
	}
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: plugins,
		ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{})}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.SetProcessSpawner(&openObserverSpawner{ctx: ctx})
	if err := srv.StartWithContext(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Stop)
	if err := srv.WaitForStartupComplete(ctx); err != nil {
		t.Fatal(err)
	}
	return removalCaptureServer{srv: srv, ctx: ctx, observerEvents: observerEvents}
}

// removalCapturePeer is a peer no plugin is attached to, so only the removal
// observers can receive its removal.
func removalCapturePeer() *plugin.PeerInfo {
	return &plugin.PeerInfo{Name: "removed-peer", Address: netip.MustParseAddr("192.0.2.1"),
		PeerAS: rpc.RemoteASOf(65001), SessionGeneration: 9}
}

// removalCaptureProcess answers the named process of the server's manager.
func removalCaptureProcess(t *testing.T, srv *pluginserver.Server, name string) *process.Process {
	t.Helper()
	proc := srv.ProcessManager().GetProcess(name)
	if proc == nil {
		t.Fatalf("the process manager holds no process %s", name)
	}
	return proc
}

// restartOverlapRaised answers the doctor-plugin-restart-overlap error events
// on the report bus, which Respawn raises when its join of the replaced
// process expires.
func restartOverlapRaised() []report.Issue {
	var raised []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorPluginRestartOverlap {
			raised = append(raised, issue)
		}
	}
	return raised
}

// TestRemovalCaptureDoesNotWaitOnRespawn captures a peer removal's recipients
// while a Respawn holds the process manager's lock, joining a process whose
// handler waits on the lock the capture's caller holds.
//
// VALIDATES: CapturePeerStateChange for a removal takes no process-manager
// lock, so it returns while Respawn is still parked in its join; once the
// stand-in reactor lock is released the join succeeds and no restart overlap
// is raised; and the captured recipients still carry the OPEN validator that
// subscribed to state, which receives the removal when the delivery runs.
// PREVENTS: addOpenRemovalObservers listing the manager's processes
// (pm.AllProcesses takes pm.mu) under the reactor's write lock. The removal
// then waited for Respawn, Respawn waited for the old handler, and the handler
// waited for the reactor lock, until the join bound expired: every reactor
// user stalled for the bound and a false doctor-plugin-restart-overlap was
// raised, with the replacement started beside the live old handler.
// METHOD: no sleep. gate stands in for r.mu. The stuck plugin's handler is put
// in flight and parked on gate, which the test holds for writing. Respawn then
// stops the process (its Done channel closes under pm.mu, so Respawn holds the
// lock from then until it returns) and waits in its join. The capture runs
// with gate still held. With the lock taken, the capture could only return
// after Respawn released pm.mu, which follows the expired join and its overlap
// record, so an overlap seen when the capture returns proves it waited.
func TestRemovalCaptureDoesNotWaitOnRespawn(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	var gate sync.RWMutex
	inFlight := make(chan struct{}, 1)
	server := startRemovalCaptureServer(t, &gate, inFlight)
	pm := server.srv.ProcessManager()
	previous := removalCaptureProcess(t, server.srv, removalCaptureStuck)
	observer := removalCaptureProcess(t, server.srv, removalCaptureObserver)
	stopped := previous.Done()

	gate.Lock()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(gate.Unlock) }
	var workers sync.WaitGroup
	// Cleanups run last-in first-out: the gate is released before the workers
	// are joined, and both before the server stops.
	t.Cleanup(workers.Wait)
	t.Cleanup(release)

	results := make(chan process.EventResult, 1)
	if !previous.Deliver(process.EventDelivery{Output: "{}", Result: results}) {
		t.Fatal("the stuck plugin refused the event that puts its handler in flight")
	}
	select {
	case <-inFlight:
	case <-server.ctx.Done():
		t.Fatal("the stuck plugin's handler never ran")
	}

	respawned := make(chan error, 1)
	workers.Go(func() {
		_, err := pm.Respawn(removalCaptureStuck)
		respawned <- err
	})
	select {
	case <-stopped:
	case <-server.ctx.Done():
		t.Fatal("Respawn never stopped the process it replaces")
	}

	captured := make(chan func(), 1)
	dispatcher := NewEventDispatcher(server.srv)
	workers.Go(func() {
		captured <- dispatcher.CapturePeerStateChange(removalCapturePeer(), rpc.SessionStateDown, rpc.ReasonPeerRemoved)
	})
	var deliver func()
	select {
	case deliver = <-captured:
	case <-server.ctx.Done():
		t.Fatal("the removal capture never returned")
	}
	if raised := restartOverlapRaised(); len(raised) != 0 {
		t.Fatalf("the removal capture returned only after Respawn's join expired (%q): it waited on the process manager's lock", raised[0].Message)
	}
	select {
	case err := <-respawned:
		t.Fatalf("Respawn returned (err %v) while the replaced handler was still parked on the stand-in lock", err)
	default:
	}

	release()
	select {
	case err := <-respawned:
		if err != nil {
			t.Fatalf("Respawn: %v", err)
		}
	case <-server.ctx.Done():
		t.Fatal("Respawn never returned after the stand-in lock was released")
	}
	if raised := restartOverlapRaised(); len(raised) != 0 {
		t.Fatalf("a restart overlap was raised although the replaced handler returned inside the join: %q", raised[0].Message)
	}

	if deliver == nil {
		t.Fatal("the removal capture answered no delivery although the server has a process manager and subscriptions")
	}
	before := server.observerEvents.Load()
	deliver()
	if got := server.observerEvents.Load() - before; got != 1 {
		t.Fatalf("the subscribed OPEN validator %s received %d removals from the capture, want 1", observer.Name(), got)
	}
}

// TestOpenRemovalObserversAreTheSubscribedValidators reads the removal
// observers of a peer no plugin is attached to.
//
// VALIDATES: addOpenRemovalObservers adds exactly the processes that both
// declared WantsValidateOpen and subscribed to BGP state for the peer, by
// process identity: a validator with no state subscription is not added, and
// a subscriber that validates nothing is not added. A process already in the
// fed set is not added twice, and the fed slice's spare capacity is not
// written, so an observer cannot replace a peer-fed claimant.
// PREVENTS: the derivation that replaced the process manager's list narrowing
// or widening the set it had: the same four combinations through the name
// list gave the same answer.
func TestOpenRemovalObserversAreTheSubscribedValidators(t *testing.T) {
	var gate sync.RWMutex
	server := startRemovalCaptureServer(t, &gate, make(chan struct{}, 1))
	stuck := removalCaptureProcess(t, server.srv, removalCaptureStuck)
	observer := removalCaptureProcess(t, server.srv, removalCaptureObserver)
	unsubscribed := removalCaptureProcess(t, server.srv, removalCaptureUnsubscribed)
	plain := removalCaptureProcess(t, server.srv, removalCapturePlain)
	peer := removalCapturePeer()

	got := addOpenRemovalObservers(server.srv, peer, nil)
	if len(got) != 2 {
		t.Fatalf("removal observers = %d processes, want the 2 subscribed validators", len(got))
	}
	for _, want := range []*process.Process{stuck, observer} {
		if !slices.Contains(got, want) {
			t.Errorf("subscribed validator %s is not a removal observer", want.Name())
		}
	}
	for _, unwanted := range []*process.Process{unsubscribed, plain} {
		if slices.Contains(got, unwanted) {
			t.Errorf("process %s became a removal observer", unwanted.Name())
		}
	}

	fed := make([]*process.Process, 1, 4)
	fed[0] = observer
	got = addOpenRemovalObservers(server.srv, peer, fed)
	if len(got) != 2 {
		t.Fatalf("fed plus observers = %d processes, want the fed validator once and the other validator", len(got))
	}
	if got[0] != observer {
		t.Errorf("the fed process is no longer first: %s", got[0].Name())
	}
	if got[1] != stuck {
		t.Errorf("the added observer is %s, want %s", got[1].Name(), stuck.Name())
	}
	if spare := fed[:2][1]; spare != nil {
		t.Errorf("the fed slice's spare capacity was written with %s", spare.Name())
	}

	if procs, _ := peerStateRecipients(server.srv, peer, rpc.SessionStateDown, rpc.ReasonTCPFailure); len(procs) != 0 {
		t.Errorf("an ordinary down of an unattached peer reached %d processes, want none", len(procs))
	}
}
