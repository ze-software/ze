// Design: docs/architecture/isis/isis-4-component-config.md -- locator-originator EventBus subscription and disconnect cleanup
// Related: reactor_signal_events.go -- EventBus subscription lifecycle in r.eventBusUnsubs
// Related: register_source_lease.go -- RegisterProcessGenerationCleanup registration pattern

package reactor

import (
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/internal/core/report"
)

func init() {
	if err := pluginserver.RegisterProcessGenerationCleanup(locatorOriginGenerationCleanup); err != nil {
		panic("BUG: bgp reactor: locator origin cleanup registration failed: " + err.Error())
	}
}

// locatorOriginGenerationCleanup clears any locator-originator entries owned by
// an ended plugin process after admitted requests drain and before any
// replacement incarnation starts, so abrupt external process exits leave no
// stale locators in the in-daemon index.
func locatorOriginGenerationCleanup(_ *pluginserver.Server, proc *process.Process) {
	if proc == nil {
		return
	}
	locatororigin.ClearSource(proc.Name())
}

// subscribeLocatorOrigins subscribes the reactor to locator-origin:snapshot
// events on the daemon EventBus, updating the read-locked in-daemon index with
// zero RPC on the BGP receive hot path. Must be called before StartWithContext
// returns; Stop unsubscribes via r.eventBusUnsubs.
func (r *Reactor) subscribeLocatorOrigins() {
	if r.eventBus == nil {
		return
	}
	unsubscribe := locatororigin.SnapshotEvent.Subscribe(r.eventBus, func(snap *locatororigin.Snapshot) {
		locatororigin.ApplySnapshot(snap)
	})
	r.eventBusUnsubs = append(r.eventBusUnsubs, unsubscribe)
}

// requestLocatorOriginReplay emits locator-origin:snapshot-request before BGP
// peers open so any already-running internal or forked IGP plugin replays its
// current SRv6 locator-originator snapshot.
func (r *Reactor) requestLocatorOriginReplay() {
	if r.eventBus == nil {
		return
	}
	if _, err := locatororigin.SnapshotRequest.Emit(r.eventBus); err != nil {
		reactorLogger().Warn("bgp locator-origin snapshot-request not delivered on event bus",
			"code", bgpserver.CodeDoctorBGPPluginEventsUndelivered,
			"event", locatororigin.EventSnapshotRequest,
			"why", "IGP locator-originator plugins may not replay their current SRv6 locators until their next LSDB change",
			"fix", bgpserver.PluginEventsUndeliveredFix,
			"error", err)
		report.RaiseErrorFixOnce(reportSourceBGP, bgpserver.CodeDoctorBGPPluginEventsUndelivered, "locator-origin-replay",
			report.FitMessage("BGP locator-origin snapshot-request was not delivered on the event bus: "+err.Error()),
			bgpserver.PluginEventsUndeliveredFix, wellKnownScanLogInterval)
	}
}
