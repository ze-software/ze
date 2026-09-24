// Design: docs/architecture/api/process-protocol.md -- acknowledged live removal
package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const pluginRemovalGrace = 500 * time.Millisecond

type removalRecoveryContextKey struct{}

// Failed replacement startup belongs to the recovery attempt. Only these
// named startup generations bypass the ordinary fatal-startup policy.
func (s *Server) runPluginRecoveryPhase(plugins []plugin.PluginConfig, tree map[string]any) error {
	if len(plugins) == 0 {
		return nil
	}
	s.removalMu.Lock()
	if s.recoveryConfigs == nil {
		s.recoveryConfigs = make(map[string]map[string]any, len(plugins))
	}
	for _, cfg := range plugins {
		s.recoveryConfigs[cfg.Name] = tree
	}
	s.removalMu.Unlock()
	defer func() {
		s.removalMu.Lock()
		for _, cfg := range plugins {
			delete(s.recoveryConfigs, cfg.Name)
		}
		s.removalMu.Unlock()
	}()
	return s.runPluginPhase(plugins)
}

func (s *Server) pluginRecoveryConfig(name string) (map[string]any, bool) {
	s.removalMu.Lock()
	defer s.removalMu.Unlock()
	tree, recovering := s.recoveryConfigs[name]
	return tree, recovering
}

// done publishes the actual callback result, not the caller's waiting deadline.
// scope keeps compensation behind the outer reload's publication or rollback.
// stopped, recovering and recoveryErr are protected by Server.removalMu.
type pluginRemoval struct {
	done        chan struct{}
	err         error
	scope       *reloadAcceptance
	stop        sync.Once
	stopped     bool
	recovering  bool
	recoveryErr error
}

// rollbackStartupProcess keeps dependencies and registrations available until a
// running plugin acknowledges cleanup. A timeout rejects removal; it does not
// cancel the pending acknowledgement or pretend the plugin released its state.
func (s *Server) rollbackStartupProcess(proc *process.Process) error {
	if proc.Stage() < plugin.StageRunning {
		s.stopRemovedProcess(proc)
		return nil
	}
	return s.notifyPluginRemoval(proc)
}

func (s *Server) stopRemovedProcess(proc *process.Process) {
	if pm := s.procManager.Load(); pm != nil && pm.GetProcess(proc.Name()) == proc {
		pm.RemoveProcess(proc.Name())
	}
	proc.Stop()
	if err := proc.Wait(s.Context()); err != nil {
		logger().Debug("wait for removed plugin", "plugin", proc.Name(), "error", err)
	}
	s.releasePluginRegistrations(proc)
	s.unmarkPluginLoaded(proc.Name())
}

func (s *Server) pluginRemovalPending(proc *process.Process) bool {
	s.removalMu.Lock()
	defer s.removalMu.Unlock()
	return s.removals[proc] != nil
}

func (s *Server) notifyPluginRemoval(proc *process.Process) error {
	parent := s.Context()
	if err := parent.Err(); err != nil {
		return err
	}
	s.txLock.mu.Lock()
	scope := s.txLock.acceptance
	s.txLock.mu.Unlock()
	s.removalMu.Lock()
	attempt := s.removals[proc]
	if attempt != nil && (attempt.recovering || attempt.stopped) {
		s.removalMu.Unlock()
		return fmt.Errorf("plugin %q removal is awaiting committed-configuration recovery", proc.Name())
	}
	if attempt == nil {
		conn := proc.Conn()
		if conn == nil {
			s.removalMu.Unlock()
			return fmt.Errorf("plugin %q removal has no live callback connection", proc.Name())
		}
		attempt = &pluginRemoval{done: make(chan struct{}), scope: scope}
		if s.removals == nil {
			s.removals = make(map[*process.Process]*pluginRemoval)
		}
		s.removals[proc] = attempt
		// The callback already runs on the SDK's callback loop. This owned waiter
		// retains its acknowledgement after a config caller's grace expires, and
		// server cancellation ends it on daemon shutdown.
		s.wg.Go(func() {
			attempt.err = conn.SendBye(parent, "removed")
			close(attempt.done)
		})
	}
	s.removalMu.Unlock()

	ctx, cancel := context.WithTimeout(parent, pluginRemovalGrace)
	defer cancel()
	select {
	case <-attempt.done:
		if attempt.err != nil {
			s.recoverPluginRemoval(proc, attempt)
			return fmt.Errorf("plugin %q removal callback: %w", proc.Name(), attempt.err)
		}
		s.finishPluginRemoval(proc, attempt)
		if scope == nil {
			s.removalMu.Lock()
			delete(s.removals, proc)
			s.removalMu.Unlock()
		}
		return nil
	case <-ctx.Done():
		s.recoverPluginRemoval(proc, attempt)
		return fmt.Errorf("plugin %q removal callback: %w", proc.Name(), ctx.Err())
	}
}

func (s *Server) finishPluginRemoval(proc *process.Process, attempt *pluginRemoval) {
	attempt.stop.Do(func() {
		s.stopRemovedProcess(proc)
		s.removalMu.Lock()
		attempt.stopped = true
		s.removalMu.Unlock()
	})
}

// finishRemovalScope is called before the outer reload releases its completion
// barrier. Successful removals are final only if the whole reload was accepted.
func (s *Server) finishRemovalScope(scope *reloadAcceptance, accepted bool) {
	s.removalMu.Lock()
	var rejected map[*process.Process]*pluginRemoval
	for proc, attempt := range s.removals {
		if attempt.scope != scope {
			continue
		}
		if accepted && attempt.stopped {
			delete(s.removals, proc)
			continue
		}
		if rejected == nil {
			rejected = make(map[*process.Process]*pluginRemoval)
		}
		rejected[proc] = attempt
	}
	s.removalMu.Unlock()
	for proc, attempt := range rejected {
		s.recoverPluginRemoval(proc, attempt)
	}
}

func (s *Server) recoverPluginRemoval(proc *process.Process, attempt *pluginRemoval) {
	s.removalMu.Lock()
	if attempt.recovering {
		s.removalMu.Unlock()
		return
	}
	attempt.recovering = true
	s.removalMu.Unlock()
	s.wg.Go(func() {
		ctx := s.Context()
		select {
		case <-attempt.done:
		case <-ctx.Done():
			return
		}
		if attempt.scope != nil {
			select {
			case <-attempt.scope.done:
			case <-ctx.Done():
				return
			}
		}
		if err := s.txLock.acquire(ctx); err != nil {
			return
		}
		defer s.txLock.release()
		if err := s.completeRemovalRecovery(ctx, proc, attempt); err != nil {
			logger().Error("plugin removal recovery failed", "plugin", proc.Name(), "error", err)
		}
	})
}

func (s *Server) completeRemovalRecovery(ctx context.Context, proc *process.Process, attempt *pluginRemoval) error {
	err := s.restoreRemovedPlugin(ctx, proc, attempt)
	s.removalMu.Lock()
	attempt.recovering = false
	attempt.recoveryErr = err
	if err == nil {
		delete(s.removals, proc)
	}
	s.removalMu.Unlock()
	return err
}

// retryRemovalRecovery runs under transaction exclusion before another reload.
// A failed restoration keeps its ownership until an explicit retry succeeds.
func (s *Server) retryRemovalRecovery(ctx context.Context) error {
	s.removalMu.Lock()
	var pending map[*process.Process]*pluginRemoval
	for proc, attempt := range s.removals {
		if !attempt.recovering && attempt.recoveryErr != nil {
			attempt.recovering = true
			if pending == nil {
				pending = make(map[*process.Process]*pluginRemoval)
			}
			pending[proc] = attempt
		}
	}
	s.removalMu.Unlock()
	var result error
	for proc, attempt := range pending {
		result = errors.Join(result, s.completeRemovalRecovery(ctx, proc, attempt))
	}
	return result
}

func removalConnectionFailed(proc *process.Process, err error) bool {
	conn := proc.Conn()
	if !proc.Running() || conn == nil {
		return true
	}
	if conn.HasBridge() {
		return errors.Is(err, rpc.ErrBridgeFailed) || errors.Is(err, rpc.ErrBridgeClosed)
	}
	var refusal *rpc.RPCCallError
	return err != nil && !errors.As(err, &refusal)
}

// restoreRemovedPlugin runs under the same exclusion as config reload and reads
// the current committed tree, never the rejected candidate captured by removal.
func (s *Server) restoreRemovedPlugin(ctx context.Context, proc *process.Process, attempt *pluginRemoval) error {
	if attempt.err == nil || removalConnectionFailed(proc, attempt.err) {
		s.finishPluginRemoval(proc, attempt)
	}
	var tree map[string]any
	if s.reactor != nil {
		tree = s.reactor.GetConfigTree()
	}
	for pending := s.txLock.pendingCompensation; pending != nil; {
		pending.mu.Lock()
		if pending.compensation != nil {
			tree = pending.compensation.before
		}
		previous := pending.previous
		pending.mu.Unlock()
		if previous == nil {
			break
		}
		previous.mu.Lock()
		rejected := previous.finished && !previous.accepted
		previous.mu.Unlock()
		if !rejected {
			break
		}
		pending = previous
	}
	var roots []string
	for _, root := range registry.ConfigRootsMap()[proc.Name()] {
		if navigateNestedMap(tree, root) != nil {
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 && !s.hasConfiguredPlugin(proc.Name()) {
		if pm := s.procManager.Load(); attempt.stopped && pm != nil {
			return s.stopOrphanedDependencies(pm, map[string]bool{proc.Name(): true})
		}
		return nil
	}
	if attempt.stopped {
		if len(roots) != 0 {
			recoveryCtx := context.WithValue(ctx, removalRecoveryContextKey{}, true)
			_, err := s.autoLoadForNewConfigPaths(recoveryCtx, tree, roots)
			return err
		}
		return s.runPluginRecoveryPhase([]plugin.PluginConfig{proc.Config()}, tree)
	}
	// Only an ordinary callback refusal leaves a usable SDK loop. Redeliver
	// committed configuration so a producer that paused its worker resumes.
	reg := proc.Registration()
	conn := proc.Conn()
	if reg == nil || conn == nil {
		return fmt.Errorf("cannot restore plugin %q without its registration and connection", proc.Name())
	}
	sections, err := config.BuildPluginConfigSections(tree, reg.WantsConfigRoots)
	if err != nil {
		return err
	}
	timeout := proc.Config().StageTimeout
	if timeout == 0 {
		timeout = stageTimeoutFromEnv()
	}
	configureCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := conn.SendConfigure(configureCtx, sections, s.advertiseClaims(proc.Name())); err != nil {
		if !removalConnectionFailed(proc, err) {
			return err
		}
		// The generation can disconnect after refusing Bye but before Configure.
		s.finishPluginRemoval(proc, attempt)
		return s.restoreRemovedPlugin(ctx, proc, attempt)
	}
	return nil
}
