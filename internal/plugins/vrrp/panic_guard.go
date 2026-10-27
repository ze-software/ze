// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- panic containment in the receive and advertisement tasks
// Related: register.go -- runVRRPEngine, which starts rxLoop
// Related: instance.go -- shutdownLocked and startupLocked, the restart it uses

package vrrp

import (
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// taskAdvertisement is the task label of the advertisement dispatcher in
// ze_task_panics_total{component="vrrp"}.
const taskAdvertisement = "advertisement"

// advertisementRestart is what follows a contained advertisement panic, in the
// words the guard's log line gives the operator.
const advertisementRestart = "the virtual router whose advertisement it was restarts " +
	"(Shutdown, then Startup into Backup); every other virtual router is untouched"

// advertGuard is the panic boundary of the plugin's advertisement dispatcher.
// A contained panic is logged, counted in
// ze_task_panics_total{component="vrrp",task="advertisement"} and
// crash-recorded. A test may replace it before starting the dispatcher and
// restore it after: serveAdvertisements reads it each time the task is entered.
var advertGuard = taskguard.New("vrrp", slogutil.Logger("vrrp"))

// rxLoop is the plugin's advertisement dispatcher: one long-lived goroutine for
// the plugin's lifetime (ai/rules/goroutine-lifecycle.md), not one per packet.
// It hands every received advertisement to the instance that owns it until
// items closes. The
// dispatching is the advertisement task (serveAdvertisements), entered again
// after a contained panic, so a Ze defect met in one advertisement restarts one
// virtual router and never stops the others hearing their peers.
func rxLoop(eng *engine, items <-chan transport.RxItem) {
	for serveAdvertisements(eng, items) {
		// The pass ended on a contained panic; the next one dispatches on.
	}
}

// serveAdvertisements is the advertisement task: it dispatches items until the
// channel closes, answering false, or until a panic ends the pass, answering
// true. The guard then restarts the virtual router the advertisement was for
// (engine.restartAfterDefect).
func serveAdvertisements(eng *engine, items <-chan transport.RxItem) (again bool) {
	var current transport.RxItem // the advertisement being dispatched
	describe := func() taskguard.Context {
		return taskguard.Context{Unit: current.Key.String() + ", advertisement from " + current.Meta.Src.String()}
	}
	defer advertGuard.Task(taskAdvertisement, advertisementRestart).Contain(describe, func(*taskguard.Panic) {
		eng.restartAfterDefect(current.Key)
		again = true
	})

	for item := range items {
		current = item
		eng.dispatchRx(item)
	}
	return false
}

// restartAfterDefect restarts the virtual router a contained advertisement
// panic was for. Its state was being changed when the defect hit, so it can no
// longer be trusted; Shutdown then Startup is the restart the instance already
// runs when its parent interface goes away and comes back. An instance torn
// down meanwhile, or not started, is left alone. It runs after the failing
// frames unwound, so the instance's mu, which onPacket holds only under defer,
// is free.
func (e *engine) restartAfterDefect(key transport.InstanceKey) {
	e.mu.Lock()
	target := e.byTransportKey(key)
	e.mu.Unlock()

	if target == nil {
		return
	}
	target.restartAfterDefect()
}

// restartAfterDefect runs the instance through Shutdown and Startup.
func (in *instance) restartAfterDefect() {
	in.mu.Lock()
	defer in.mu.Unlock()

	if !in.started {
		return
	}
	// RFC 9568 Section 6.4.3: "If a Shutdown event is received, then:" ...
	// "Send an ADVERTISEMENT with Priority = 0" ... "Transition to the
	// {Initialize} state". An Active router resigns, so its Backups take over
	// at once rather than after the Active_Down_Interval; a Backup cancels its
	// Active_Down_Timer (Section 6.4.2). Startup then enters the instance anew,
	// as Backup, or as Active when it owns the address.
	in.shutdownLocked()
	in.startupLocked()
}
