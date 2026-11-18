// Design: docs/architecture/api/process-protocol.md -- runtime peer declarations.
// Related: reload_tx.go -- exact replacement lifetime and restart ownership.
// Related: reload.go -- transaction exclusion and deferred reconciliation.
package server

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// peerDeclarations borrows immutable Stage 1 metadata from one Process.
// It carries no configured authority; the listener resolves the actual graph.
type peerDeclarations struct {
	insertions    []rpc.FilterInsertion
	claims        []string
	dissemination []rpc.DisseminationDecl
}

func declaredPeerMetadata(proc *process.Process) peerDeclarations {
	if proc == nil {
		return peerDeclarations{}
	}
	reg := proc.Registration()
	if reg == nil {
		return peerDeclarations{}
	}
	return peerDeclarations{insertions: reg.FilterInsertions, claims: reg.Claims, dissemination: reg.Dissemination}
}

func (d peerDeclarations) empty() bool {
	return len(d.insertions) == 0 && len(d.claims) == 0 && len(d.dissemination) == 0
}

// peerDeclarationReplacement owns refusal of an exact runtime lifetime. The
// transaction keeps at most one current replacement per configured name.
type peerDeclarationReplacement struct {
	server *Server
	proc   *process.Process
}

func (replacement peerDeclarationReplacement) refuse(err error) {
	s, proc := replacement.server, replacement.proc
	// A deferred transaction no longer owns the restart controller that created
	// this process. Reserve the name before checking or releasing registrations.
	controller, acquireErr := s.restartControllers.wait(s.Context(), proc.Name())
	if acquireErr != nil {
		return
	}
	defer controller.release()
	// An unchanged successor may have refreshed this entry while the listener
	// ran or while this callback waited for the preceding restart controller.
	s.txLock.mu.Lock()
	current, pending := s.txLock.peerDeclarationReplacements[proc.Name()]
	s.txLock.mu.Unlock()
	if !pending {
		return
	}
	proc = current.proc
	pm := s.procManager.Load()
	if pm == nil || pm.GetProcess(proc.Name()) != proc {
		return
	}
	// The exact lifetime retains the refusal after rollback removes its slot.
	proc.SetStartupError(err)
	s.rollbackReplacement(proc)
	pm.RecordStopped(proc.Name(), process.PluginStop{Cause: process.StopRestartFailed, Detail: err.Error()})
	s.txLock.mu.Lock()
	if s.txLock.peerDeclarationReplacements[proc.Name()].proc == proc {
		delete(s.txLock.peerDeclarationReplacements, proc.Name())
	}
	s.txLock.mu.Unlock()
}

// admit reserves the same per-name controller as a restart before releasing
// its captured exact process. No transaction mutex is held while waiting.
func (replacement peerDeclarationReplacement) admit() {
	s, proc := replacement.server, replacement.proc
	controller, err := s.restartControllers.wait(s.Context(), proc.Name())
	if err != nil {
		return
	}
	defer controller.release()
	// The restart that deferred this work has released its handshake context.
	// The transaction owner MUST join restoration under its own bounded scope.
	ctx, cancel := context.WithTimeout(s.Context(), restartStartupHandshakeBound(proc, previousStartupBudget(proc)))
	defer cancel()
	if err := s.admitPeerDeclarationReplacement(ctx, proc); err != nil {
		if pm := s.procManager.Load(); pm != nil {
			pm.RecordStopped(proc.Name(), process.PluginStop{Cause: process.StopRestartFailed, Detail: err.Error()})
		}
	}
}

// announcePeerDeclarationChange leaves identical declarations alone, while
// refreshing an already-deferred refusal to the latest exact replacement.
func (s *Server) announcePeerDeclarationChange(before, after peerDeclarations, replacement *process.Process) error {
	if reflect.DeepEqual(before, after) {
		s.txLock.refreshPeerDeclarationReplacement(s, replacement)
		return nil
	}
	return s.announcePeerDeclarationsChanged(replacement)
}

// announcePeerDeclarationsChanged serializes the cold rebuild with reloads.
// A change during a transaction is reconciled against the running tree that
// transaction leaves. The listener takes reactor locks, so callers hold no
// Server mutex, process-manager lock, peer lock or txLock.mu.
func (s *Server) announcePeerDeclarationsChanged(replacement *process.Process) error {
	if !s.startupComplete() {
		return nil
	}
	for {
		if s.txLock.deferPeerDeclarationChange(s, replacement) {
			return nil
		}
		if s.txLock.tryAcquire() {
			break
		}
	}
	defer s.txLock.release()
	return notifyPeerDeclarationsChanged()
}

func notifyPeerDeclarationsChanged() error {
	installed, err := registry.NotifyPeerDeclarationsChanged()
	if err != nil {
		logger().Error("runtime peer declarations could not be reconciled; fix the cause and reload the configuration",
			"error", err, "code", diagnostic.CodeConfigPluginVerify)
		return fmt.Errorf("reconcile runtime peer declarations: %w", err)
	}
	if !installed {
		logger().Info("peer declarations changed and no subsystem in this daemon places them, so nothing is rebuilt")
	}
	return nil
}
