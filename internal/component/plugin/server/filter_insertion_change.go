// Design: docs/architecture/api/process-protocol.md -- Filter Insertion
// Related: reload_tx.go -- restartPlugin, the respawn that can change a declaration
// Related: startup_autoload.go -- stopCollectedProcesses, the stop that withdraws one
// Related: reload.go -- txLock.release, where a deferred change is announced
// Related: server.go -- thirdPartyFilterInsertions, the set config load reads

package server

import (
	"reflect"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// declaredInsertions answers the filter insertions proc declared at Stage 1,
// or nil for no process or no declaration.
func declaredInsertions(proc *process.Process) []rpc.FilterInsertion {
	if proc == nil {
		return nil
	}
	reg := proc.Registration()
	if reg == nil {
		return nil
	}
	return reg.FilterInsertions
}

// announceInsertionChange tells the subsystem that places filter insertions
// that a running process's declaration changed across a respawn, so the
// peers' lists follow it without a reload (spec-filter-wire-6-stage-filters,
// re-insertion on a runtime declaration change). A respawn whose declaration
// is unchanged says nothing, because a rebuild would then replace no peer.
func (s *Server) announceInsertionChange(before, after []rpc.FilterInsertion) {
	if reflect.DeepEqual(before, after) {
		return
	}
	s.announceInsertionsChanged()
}

// announceInsertionsChanged runs the rebuild a change of the running
// declared insertions owes, serialized with config transactions.
//
// A change during startup says nothing: the BGP plugin rebuilds the peers
// once after Stage 1, before they start. Otherwise the rebuild MUST NOT
// interleave with a transaction, because it re-parses the running tree and a
// reload swapping that tree under it would have one apply undo the other. So
// a change met while a transaction holds txLock (a broken plugin restarted
// inside a reload, a plugin a reload stops) is deferred to that holder, which
// announces it at release from the tree it leaves running; with no holder,
// this takes the lock for the rebuild itself.
//
// Lock order: the caller holds no Server mutex and not txLock.mu. The
// listener takes the reactor's lock and stores the delivery graph through
// UpdateDeliveryGraph, which takes no Server mutex either.
func (s *Server) announceInsertionsChanged() {
	if !s.startupComplete() {
		return
	}
	for {
		if s.txLock.deferInsertionChange() {
			return
		}
		if s.txLock.tryAcquire() {
			break
		}
	}
	// release announces it: the change is recorded as the holder's own.
	s.txLock.deferInsertionChange()
	s.txLock.release()
}

// notifyInsertionsChanged runs the installed listener, saying so when this
// daemon holds none.
func notifyInsertionsChanged() {
	if !registry.NotifyFilterInsertionsChanged() {
		logger().Info("filter insertions changed and no subsystem in this daemon places them, so nothing is rebuilt")
	}
}
