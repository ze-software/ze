// Design: docs/guide/config-reload.md — a reload applies a changed global router-id and local AS
// Related: reactor_api.go — reconcilePeersJournaled applies these before the peer diff
// Related: config.go — GlobalsFromTree reads them from the BGP config tree
package reactor

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// applyGlobalsJournaled makes the reactor's global router-id and local AS the
// values the reloaded configuration declares, and records the undo in j.
//
// RFC 4271 Section 4.2: "A given BGP speaker sets the value of its BGP
// Identifier to an IP address that is assigned to that BGP speaker." The
// reactor's global value is what `show bgp`, the ze_info gauge and a peer
// added at run time (AddDynamicPeer) read. Before this ran on reload, the
// value stayed the one read at startup, so the daemon reported an Identifier
// its configuration no longer held. The Identifier on the wire is per peer
// (PeerSettings.RouterID); the peer diff that follows restarts every peer
// whose effective Identifier changed, which sends the new one in a new OPEN.
func (a *reactorAPIAdapter) applyGlobalsJournaled(globals Globals, j configJournal) error {
	r := a.r

	previous := r.globals()
	if previous == globals {
		return nil
	}
	return j.Record(
		func() error {
			r.setGlobals(globals)
			return nil
		},
		func() error {
			r.setGlobals(previous)
			return nil
		},
	)
}

// setGlobals writes the global router-id and local AS, and moves the ze_info
// gauge with them. Safe for concurrent use: it takes r.mu.
func (r *Reactor) setGlobals(globals Globals) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.config.RouterID = globals.RouterID
	r.config.LocalAS = globals.LocalAS
	if r.rmetrics == nil {
		return
	}
	r.rmetrics.setInfo(localRouterIDString(globals.RouterID), textbuf.StringUint(uint64(globals.LocalAS)))
}

// globals returns the reactor's global router-id and local AS. Safe for
// concurrent use: it takes r.mu.
func (r *Reactor) globals() Globals {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return Globals{RouterID: r.config.RouterID, LocalAS: r.config.LocalAS}
}
