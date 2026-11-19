// Design: docs/architecture/fleet-config.md -- the owner of the daemon's hub client: leave and rejoin with no restart
// Related: main.go -- runYANGConfig builds the owner on a managed start and joins it before it returns the store
// Related: managed.go -- wireManagedCommit, which the owner applies to each client it starts

package hub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ManagedClientBuilder builds the configuration of the daemon's hub client
// from the configuration its store holds when the builder is called: the hub
// to reach, and the callbacks that read the store. It answers
// managed.ErrNoHubClient for a configuration that names no hub client block.
// The daemon calls it each time it starts a hub client, at boot and at every
// `request fleet enable`, so a client never runs on the hub block the daemon
// read at boot. A nil builder is a daemon that did not start managed.
type ManagedClientBuilder func() (*managed.ClientConfig, error)

// errManagedOwnerEnded refuses a start after the daemon began to stop: the
// store the client borrows is about to be returned. The managed flag, not this
// daemon, decides whether the next start runs a hub client.
var errManagedOwnerEnded = opererr.New(
	"the daemon is stopping, so it starts no hub client",
	"the store a hub client reads is being returned, and the managed flag decides whether the next start runs one",
	"ze start")

// noFlagWrite is the flag write of a start or stop that changes no managed
// flag: the boot's own start, which follows the flag it read.
func noFlagWrite() error { return nil }

// managedClientOwner owns the hub client of a daemon that started managed, and
// through it the client's one goroutine. It exists for the whole of a managed
// start, whether or not the configuration at boot named a hub, so a hub block
// added later is followed with no restart.
//
// startWith and stopWith are the operations of `request fleet enable` and
// `request fleet disable`. Each runs the managed flag write it is handed
// FIRST, then starts or stops the client, and holds op across both, so the
// flag stored last and the client's state always agree: two operations never
// interleave their write and their act. A start builds a client configuration,
// wires its commit, and starts the goroutine on a context of the owner's own,
// which no other worker of the daemon shares. A stop cancels that context and
// MUST return only after the goroutine ended, which it does by waiting on the
// run's done channel: a configuration the client is applying is applied, and
// its baseline stored, before the connection closes. The daemon MUST call
// shutdown before it returns the store the client borrows, and no client
// starts after it. Safe for concurrent use.
//
// Locks, taken in the order op, then mu: op serializes the operations and is
// held across a join; mu guards the slots and is never held while anything
// waits, so state answers while a stop waits for its client.
type managedClientOwner struct {
	build ManagedClientBuilder
	// wire sets what a built client needs from the running daemon (OnCommit).
	wire func(*managed.ClientConfig)
	// run is the client: managed.RunManagedClient, or a test's stand-in. It
	// MUST return once its context is canceled.
	run func(context.Context, managed.ClientConfig)

	op sync.Mutex

	mu      sync.Mutex
	running *managedClientRun
	// stopping is the client a stop took out of running and is joining. Only
	// the operation holding op sets and clears it.
	stopping *managedClientRun
	// ended is set by shutdown. Guard: a start refuses on it, because a client
	// started then would read a store the daemon has returned.
	ended bool
}

// managedClientRun is one started client. serve is its goroutine, and it
// closes done when the client returns, so a closed done means no goroutine is
// left. The client ends on its own when the managed flag is false or when the
// configuration it applied names no hub, so a run in the slot may have ended.
type managedClientRun struct {
	cancel    context.CancelFunc
	done      chan struct{}
	connected atomic.Bool
	holding   atomic.Bool
}

// finished reports whether the client's goroutine has returned, read from
// done, so an answer that says stopped states what the goroutine did.
func (r *managedClientRun) finished() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// state answers what this client is doing. A client that returned holds no
// connection, whatever it last reported.
func (r *managedClientRun) state() managed.ClientState {
	if r.finished() {
		return managed.ClientState{}
	}
	return managed.ClientState{Running: true, Connected: r.connected.Load(), Holding: r.holding.Load()}
}

// stoppingState answers what this client is doing while a stop joins it: it
// is stopping until its goroutine returns, with the connection it still holds.
func (r *managedClientRun) stoppingState() managed.ClientState {
	state := r.state()
	state.Stopping = state.Running
	state.Running = false
	return state
}

// newManagedClientOwner answers the owner of a managed daemon's hub client.
// It starts nothing: startAtBoot and startWith do.
func newManagedClientOwner(build ManagedClientBuilder, wire func(*managed.ClientConfig)) *managedClientOwner {
	return &managedClientOwner{build: build, wire: wire, run: managed.RunManagedClient}
}

// target is what the fleet commands reach this owner through
// (managed.InstallFleetTarget).
func (o *managedClientOwner) target() *managed.FleetTarget {
	return &managed.FleetTarget{StartClient: o.startWith, StopClient: o.stopWith, ClientState: o.state}
}

// startAtBoot starts the hub client the configuration at boot asks for. A
// configuration that names no hub starts none, and neither does one whose hub
// block cannot be read: the daemon runs either way, and `request fleet enable`
// starts the client once the configuration names a hub.
func (o *managedClientOwner) startAtBoot() {
	_, err := o.startWith(noFlagWrite)
	if err == nil {
		return
	}
	if errors.Is(err, managed.ErrNoHubClient) {
		slogutil.Logger("hub.managed").Info("the configuration names no plugin hub client block, so this managed daemon starts no hub client")
		return
	}
	slogutil.Logger("hub.managed").Warn("the hub client could not be built from the configuration, so this daemon runs with no hub client; "+
		"correct the plugin hub block, then check it with: ze config validate",
		"code", diagnostic.CodeConfigHubInvalid, "error", err)
}

// startWith runs storeFlag, then builds a hub client from the configuration
// the store holds now and starts it, under op. When storeFlag fails it answers
// that error unchanged and touches no client. It answers false, with no error,
// when a client already runs: nothing was built and nothing was started.
func (o *managedClientOwner) startWith(storeFlag func() error) (bool, error) {
	o.op.Lock()
	defer o.op.Unlock()

	if err := storeFlag(); err != nil {
		return false, err
	}
	if !o.claimSlot() {
		return false, nil
	}
	o.mu.Lock()
	ended := o.ended
	o.mu.Unlock()
	if ended {
		return false, errManagedOwnerEnded
	}

	// The build reads the store, so it runs outside mu: op keeps every other
	// start, stop and shutdown out until the client is in the slot.
	client, err := o.build()
	if err != nil {
		return false, err
	}
	o.wire(client)
	run := &managedClientRun{done: make(chan struct{})}
	client.OnConnection = run.connected.Store
	client.OnHold = run.holding.Store
	ctx, cancel := context.WithCancel(context.Background())
	run.cancel = cancel
	go o.serve(ctx, run, *client)

	o.mu.Lock()
	o.running = run
	o.mu.Unlock()
	return true, nil
}

// claimSlot reports whether the running slot is free for a start: a client
// that returned on its own is cleared from it, one that runs keeps it. The
// caller MUST hold op.
func (o *managedClientOwner) claimSlot() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	previous := o.running
	if previous == nil {
		return true
	}
	if !previous.finished() {
		return false
	}
	// The client returned on its own. Its goroutine is gone, so the slot is
	// free once its context is released.
	previous.cancel()
	o.running = nil
	return true
}

// serve runs one client until its context is canceled or it ends on its own,
// then closes done. It MUST be the only goroutine started for run (startWith
// does).
func (o *managedClientOwner) serve(ctx context.Context, run *managedClientRun, client managed.ClientConfig) {
	defer close(run.done)
	o.run(ctx, client)
}

// stopWith runs storeFlag, then ends the running client and returns once its
// goroutine has ended, under op. It answers the client's state before the
// stop; with no client it answers the zero state and stops nothing. When
// storeFlag fails it answers that error unchanged and touches no client.
func (o *managedClientOwner) stopWith(storeFlag func() error) (managed.ClientState, error) {
	o.op.Lock()
	defer o.op.Unlock()

	if err := storeFlag(); err != nil {
		return managed.ClientState{}, err
	}
	return o.join(), nil
}

// shutdown is a stop for a daemon that is stopping: the client is ended and
// joined, and every later start is refused. The daemon MUST call it before it
// returns the store, and MUST withdraw the fleet target first.
func (o *managedClientOwner) shutdown() {
	o.op.Lock()
	defer o.op.Unlock()

	o.mu.Lock()
	o.ended = true
	o.mu.Unlock()
	o.join()
}

// join takes the running client out of its slot under mu and marks it
// stopping, then cancels it and waits for its goroutine with mu released, so
// state answers during the wait. It answers the client's state before the
// cancel. The caller MUST hold op.
func (o *managedClientOwner) join() managed.ClientState {
	o.mu.Lock()
	run := o.running
	if run == nil {
		o.mu.Unlock()
		return managed.ClientState{}
	}
	before := run.state()
	o.running = nil
	o.stopping = run
	o.mu.Unlock()

	run.cancel()
	<-run.done

	o.mu.Lock()
	o.stopping = nil
	o.mu.Unlock()
	return before
}

// state answers what the hub client is doing now. It never waits on a client:
// a client being stopped answers Stopping, with the connection it still holds.
func (o *managedClientOwner) state() managed.ClientState {
	o.mu.Lock()
	defer o.mu.Unlock()

	if run := o.stopping; run != nil {
		return run.stoppingState()
	}
	run := o.running
	// refuse-by-default: guards managedClientOwner -- a nil running slot is the no-client state a stop and a boot with no hub block leave, and the answer names it as not running
	if run == nil {
		return managed.ClientState{}
	}
	return run.state()
}
