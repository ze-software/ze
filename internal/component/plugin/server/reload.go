// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: reload_tx.go — transaction coordinator wiring
// Related: config_tx_bridge.go — engine-side RPC bridge for per-plugin verify/apply events

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var (
	errNoConfigLoaderConfigured = errors.New("no config loader configured")
	errNoReactorConfigured      = errors.New("no reactor configured")
)

// affectedPlugin pairs a plugin process with the config sections that changed
// under its declared roots. Shared between reload.go (which builds the slice
// by walking WantsConfigRoots over the diff) and reload_tx.go (which converts
// it into transaction participants + diffs).
type affectedPlugin struct {
	proc     *process.Process
	sections []rpc.ConfigSection
}

type reloadCallerContextKey struct{}

func contextWithReloadCaller(ctx context.Context, proc *process.Process) context.Context {
	if proc == nil {
		return ctx
	}
	return context.WithValue(ctx, reloadCallerContextKey{}, proc)
}

func reloadCallerFromContext(ctx context.Context) *process.Process {
	proc, _ := ctx.Value(reloadCallerContextKey{}).(*process.Process)
	return proc
}

// txLock enforces one config transaction at a time.
// CLI/API commits are rejected when locked. SIGHUP is queued.
//
// It also holds the handle Stop needs to stand down a running transaction.
// cancel stops it, and done reports when it has unwound. The transaction owns
// the plugin connections cleanup is about to close. Closed under it, the
// bridge reads every connection as a crashed plugin (config_tx_bridge.go,
// the conn == nil arm of subscribePhase).
type txLock struct {
	mu     sync.Mutex
	locked bool
	sighup bool
	cancel context.CancelCauseFunc
	done   chan struct{}
}

// tryAcquire attempts to acquire the transaction lock. Returns false if already held.
func (l *txLock) tryAcquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locked {
		return false
	}
	l.locked = true
	l.done = make(chan struct{})
	return true
}

// setCancel records the cancel function of the transaction now holding the
// lock. The holder calls it right after tryAcquire.
func (l *txLock) setCancel(cancel context.CancelCauseFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cancel = cancel
}

// inFlight returns the running transaction's cancel function and its done
// channel, or (nil, nil) when no transaction holds the lock. cancel is nil in
// the window between tryAcquire and setCancel. done is not nil there, so a
// caller that waits on done still covers that window.
func (l *txLock) inFlight() (context.CancelCauseFunc, <-chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cancel, l.done
}

// release releases the transaction lock and wakes anyone waiting on done.
func (l *txLock) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.done != nil {
		close(l.done)
		l.done = nil
	}
	l.cancel = nil
	l.locked = false
}

// queueSIGHUP records that a SIGHUP was received during an active transaction.
func (l *txLock) queueSIGHUP() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sighup = true
}

// drainSIGHUP clears the queued SIGHUP flag and returns whether one was queued.
func (l *txLock) drainSIGHUP() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	had := l.sighup
	l.sighup = false
	return had
}

// QueueSIGHUP queues a SIGHUP for later processing if a transaction is active.
func (s *Server) QueueSIGHUP() {
	s.txLock.queueSIGHUP()
}

// DrainSIGHUP returns true if a SIGHUP was queued and clears the flag.
func (s *Server) DrainSIGHUP() bool {
	return s.txLock.drainSIGHUP()
}

// ErrReloadInProgress is returned when a config reload is attempted while
// another is already running. Callers can check this with errors.Is to
// decide whether to queue the reload (SIGHUP) or reject it (CLI/API).
var ErrReloadInProgress = errors.New("config reload already in progress")

// ConfigLoader loads a new config tree from disk or other source.
// Returns the parsed config tree or an error.
// Set on Server.configLoader before calling ReloadFromDisk.
type ConfigLoader func() (map[string]any, error)

// FullReloadFunc runs the hub-level reload path for commits triggered through RPC.
// It includes plugin transactions plus ConfigProvider, engine, and subsystem refresh.
type FullReloadFunc func(context.Context) error

// SetConfigLoader sets the function used by ReloadFromDisk to load the config tree.
func (s *Server) SetConfigLoader(loader ConfigLoader) {
	s.configLoader = loader
}

// SetFullReloadFunc sets the function used by daemon-reload RPC when the hub is wired.
func (s *Server) SetFullReloadFunc(fn FullReloadFunc) {
	s.fullReload = fn
}

// hasFullReloadFunc reports whether a hub-level reload hook has been set.
func (s *Server) hasFullReloadFunc() bool {
	return s.fullReload != nil
}

// reloadFull runs the hub-level reload hook.
func (s *Server) reloadFull(ctx context.Context) error {
	if s.fullReload == nil {
		return errNoConfigLoaderConfigured
	}
	return s.fullReload(ctx)
}

// HasConfigLoader reports whether a config loader has been set.
// Used by SIGHUP handler to decide between coordinator path and direct reload.
func (s *Server) HasConfigLoader() bool {
	return s.configLoader != nil
}

// ReloadFromDisk loads the config from the configured loader and triggers reload.
// Returns error if the loader is not set, parsing fails, or reload fails.
func (s *Server) ReloadFromDisk(ctx context.Context) error {
	if s.configLoader == nil {
		return errNoConfigLoaderConfigured
	}

	newTree, err := s.configLoader()
	if err != nil {
		return fmt.Errorf("config parse error: %w", err)
	}

	return s.reloadConfig(ctx, newTree)
}

// ReloadConfig orchestrates config reload across all config-interested plugins.
// Follows verify→apply protocol: all plugins must verify before any apply.
// Returns nil if there are no changes, or if verify→apply succeeds.
// Returns error if verify fails for any plugin, or if a reload is already in progress.
func (s *Server) ReloadConfig(ctx context.Context, newTree map[string]any) error {
	return s.reloadConfig(ctx, newTree)
}

// txShutdownGrace bounds how long Stop waits for a canceled transaction to
// unwind before it closes the plugin connections anyway. A canceled
// transaction emits no event and returns on its next select, so this is a
// backstop and not a budget. It matches the daemon's own 3-second shutdown
// budget (cmd/ze/hub/main.go, stopCtx). A shutdown that can hang forever on a
// stuck reload is worse than the noise the wait removes.
const txShutdownGrace = 3 * time.Second

// stopTransaction cancels the in-flight config transaction, if there is one,
// and waits for it to unwind. Stop calls it BEFORE cleanup closes the plugin
// connections, because the transaction is still using them.
//
// The cause is transaction.ErrShutdown, which is what makes the orchestrator
// skip its rollback and its broken-plugin restart (orchestrator.go,
// canceledByShutdown). Cancellation alone would not: an apply interrupted for
// any other reason must still roll back.
func (s *Server) stopTransaction(grace time.Duration) {
	cancel, done := s.txLock.inFlight()
	if done == nil {
		return
	}
	logger().Info("shutdown: canceling in-flight config transaction")
	if cancel != nil {
		cancel(transaction.ErrShutdown)
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		logger().Warn("shutdown: config transaction did not unwind in time, closing plugin connections anyway",
			"grace", grace)
	}
}

// reloadConfig is the internal implementation of ReloadConfig.
func (s *Server) reloadConfig(ctx context.Context, newTree map[string]any) error {
	// Prevent concurrent reloads via transaction lock.
	if !s.txLock.tryAcquire() {
		return ErrReloadInProgress
	}
	// Publish the cancel handle so Stop can stand this transaction down
	// instead of pulling the plugin connections out from under it.
	ctx, cancelTx := context.WithCancelCause(ctx)
	s.txLock.setCancel(cancelTx)
	defer func() {
		cancelTx(nil)
		s.txLock.release()
	}()

	if s.reactor == nil {
		return errNoReactorConfigured
	}

	logger().Info("config reload started")

	// Get running config.
	running := s.reactor.GetConfigTree()

	// Compute diff.
	diff := config.DiffMaps(running, newTree)
	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Changed) == 0 {
		logger().Info("config reload: no changes")
		return nil // No changes
	}

	logger().Debug("config reload diff",
		"added", len(diff.Added), "removed", len(diff.Removed), "changed", len(diff.Changed))

	if err := s.reactor.VerifyConfig(newTree); err != nil {
		return fmt.Errorf("reactor config verify: %w", err)
	}

	// Collect removed config keys for deferred stop (after verify+apply succeeds).
	// Stopping plugins before the transaction is proven risks divergence: if
	// verify/apply fails, the stopped plugins are gone and cannot be restarted
	// with their old config.
	var removedKeys []string
	if len(diff.Removed) > 0 {
		removedKeys = make([]string, 0, len(diff.Removed))
		for k := range diff.Removed {
			removedKeys = append(removedKeys, k)
		}
	}

	preflightStopped := s.collectProcessesForRemovedConfigPaths(removedKeys)
	if caller := reloadCallerFromContext(ctx); caller != nil && preflightStopped[caller.Name()] {
		return fmt.Errorf("config reload would stop calling plugin %q", caller.Name())
	}

	// Auto-load plugins for newly added config sections.
	// When a user adds fib { kernel { } } to their config, the fib-kernel plugin
	// needs to start before we can send it config.
	var autoLoaded []string
	if len(diff.Added) > 0 {
		addedKeys := make([]string, 0, len(diff.Added))
		for k := range diff.Added {
			addedKeys = append(addedKeys, k)
		}
		var autoLoadErr error
		autoLoaded, autoLoadErr = s.autoLoadForNewConfigPaths(ctx, newTree, addedKeys)
		if autoLoadErr != nil {
			return autoLoadErr
		}
	}

	// Find affected plugins: those with WantsConfigRoots matching changed roots.
	var affected []affectedPlugin

	if pm := s.procManager.Load(); pm != nil {
		for _, proc := range pm.AllProcesses() {
			reg := proc.Registration()
			if reg == nil || len(reg.WantsConfigRoots) == 0 {
				continue
			}

			// Build sections only for roots that have changes.
			var sections []rpc.ConfigSection
			for _, root := range reg.WantsConfigRoots {
				if !rootHasChanges(diff, root) {
					continue
				}
				subtree := ExtractConfigSubtree(newTree, root)
				if subtree == nil {
					// Root was removed from new config — send empty object
					// so the plugin can verify/handle the removal.
					sections = append(sections, rpc.ConfigSection{Root: root, Data: "{}"})
					continue
				}
				jsonBytes, err := json.Marshal(subtree)
				if err != nil {
					logger().Error("config reload: marshal config subtree", "root", root, "error", err)
					continue
				}
				sections = append(sections, rpc.ConfigSection{Root: root, Data: string(jsonBytes)})
			}

			if len(sections) > 0 {
				affected = append(affected, affectedPlugin{proc: proc, sections: sections})
			}
		}
	}

	if len(affected) == 0 {
		if err := s.reactor.ApplyConfigDiff(newTree); err != nil {
			return fmt.Errorf("reactor config apply: %w", err)
		}
		// No plugins care about these changes. Recompute orphan ownership after
		// auto-load, stop removed config owners, then update the running config.
		logger().Info("config reload: no affected plugins, updating config")
		s.stopCollectedProcesses(s.collectProcessesForRemovedConfigPaths(removedKeys))
		s.reactor.SetConfigTree(newTree)
		return nil
	}

	// Transaction path: drive the stream-based TxCoordinator via an RPC
	// bridge that translates per-plugin verify/apply events into the
	// existing SDK callback RPCs. The bridge collects acks back onto the
	// stream so the orchestrator's state machine (tiered deadlines,
	// reverse-tier rollback, broken plugin restart) can run unchanged.
	//
	// Crash handling moves from three hand-rolled checkpoints to the
	// bridge's lookupProcess + Conn() checks, which translate missing or
	// closed connections into verify-failed / apply-failed / rollback-ok
	// (CodeBroken) acks; the orchestrator reacts to those acks via the
	// same state machine it uses for real plugin-reported failures.
	logger().Info("config reload: verify+apply phase", "plugins", len(affected))
	if err := s.runTxCoordinator(ctx, affected, diff, running, newTree); err != nil {
		if errors.Is(err, transaction.ErrShutdown) {
			// Not a failure: Stop stood this transaction down. A WARN here
			// would say the reload was refused when it was interrupted.
			// The auto-loaded plugins below are not stopped either, because
			// cleanup is about to kill those processes.
			logger().Info("config reload: canceled by shutdown", "error", err)
			return err
		}
		logger().Warn("config reload: transaction failed", "error", err)
		if len(autoLoaded) > 0 {
			logger().Info("config reload: stopping auto-loaded plugins after failed transaction", "plugins", autoLoaded)
			s.autoStopPluginNames(autoLoaded)
		}
		return err
	}

	// Transaction committed. Recompute against the post-auto-load process set
	// so new dependents keep shared dependencies alive.
	s.stopCollectedProcesses(s.collectProcessesForRemovedConfigPaths(removedKeys))

	if err := s.reactor.ApplyConfigDiff(newTree); err != nil {
		return fmt.Errorf("reactor config apply: %w", err)
	}
	// Update running config tree after the orchestrator commits. Plugins
	// have already persisted their per-root state via apply; reconciling
	// the reactor's config view happens last so the BGP peer reconcile
	// step sees every other plugin's apply in place.
	s.reactor.SetConfigTree(newTree)
	logger().Info("config reload completed")
	return nil
}

// rootHasChanges returns true if the diff contains changes under the given root.
// Checks config key paths: "bgp" matches "bgp", "bgp/peer", "bgp/peer/foo", etc.
func rootHasChanges(diff *config.ConfigDiff, root string) bool {
	if root == "*" {
		return len(diff.Added) > 0 || len(diff.Removed) > 0 || len(diff.Changed) > 0
	}

	prefix := root + config.PathSep
	for k := range diff.Added {
		if k == root || strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range diff.Removed {
		if k == root || strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range diff.Changed {
		if k == root || strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

// wildcardConfigRoot is the declared root meaning "every root in this reload".
// It never groups diff keys (it is not a real path); expandWildcardRoots resolves
// it to the concrete section roots once those exist.
const wildcardConfigRoot = "*"

// diffRootData groups diff entries by their top-level root key.
type diffRootData struct {
	added   map[string]any
	removed map[string]any
	changed map[string]any
}

// buildDiffSections converts a config.ConfigDiff into per-root ConfigDiffSections,
// grouping flat config keys by their top-level root ("bgp/peer/foo" -> "bgp").
//
// declaredRoots are the config roots the reload's participants actually declared.
// A key is grouped under the LONGEST declared root that is a prefix of it, and
// falls back to its top-level root when no declared root claims it.
//
// The declared-root pass exists because the transaction orchestrator matches a
// participant to its diff by EXACT map lookup -- filterDiffs does
// `allDiffs[root]` for each of Participant.ConfigRoots
// (internal/component/config/transaction/orchestrator.go:487-503) -- and both
// runVerify (:340-344) and runApply (:397-401) `continue` past any participant
// whose filtered diff is empty.
//
// Grouping solely by top-level root therefore made every plugin with a NESTED
// config root unreconfigurable by SIGHUP: reload.go put it in `affected`
// (rootHasChanges is prefix-aware, :297-319) and built a correctly-scoped verify
// section for it, but the diff was filed under "ddos" while the participant
// declared "ddos/local", so the exact lookup missed, the orchestrator skipped it
// for verify AND apply, and the reload reported success having told the plugin
// nothing. Eight plugins declare nested roots today: ddos/{detect,local,observe,
// flowspec,flowtriq}, anomaly/{detect,shape}, traffic/usage. It surfaced as
// test/plugin/ddos-transit-forward-drop.ci Phase B, where a SIGHUP flipping
// `ddos { local { forward-mitigation false } }` left the old value mitigating.
//
// Longest-prefix (not first-match) is what keeps a parent and child root that are
// both declared from stealing each other's keys.
func buildDiffSections(diff *config.ConfigDiff, declaredRoots []string) []rpc.ConfigDiffSection {
	roots := make(map[string]*diffRootData)

	ensure := func(root string) *diffRootData {
		if r, ok := roots[root]; ok {
			return r
		}
		r := &diffRootData{
			added:   make(map[string]any),
			removed: make(map[string]any),
			changed: make(map[string]any),
		}
		roots[root] = r
		return r
	}

	groupFor := func(key string) string {
		return groupRootFor(key, declaredRoots)
	}

	for k, v := range diff.Added {
		r := ensure(groupFor(k))
		r.added[k] = v
	}
	for k, v := range diff.Removed {
		r := ensure(groupFor(k))
		r.removed[k] = v
	}
	for k, v := range diff.Changed {
		r := ensure(groupFor(k))
		r.changed[k] = v
	}

	sections := make([]rpc.ConfigDiffSection, 0, len(roots))
	for root, data := range roots {
		s := rpc.ConfigDiffSection{Root: root}
		if len(data.added) > 0 {
			j, err := json.Marshal(data.added)
			if err != nil {
				logger().Error("config reload: marshal diff added", "root", root, "error", err)
			} else {
				s.Added = string(j)
			}
		}
		if len(data.removed) > 0 {
			j, err := json.Marshal(data.removed)
			if err != nil {
				logger().Error("config reload: marshal diff removed", "root", root, "error", err)
			} else {
				s.Removed = string(j)
			}
		}
		if len(data.changed) > 0 {
			j, err := json.Marshal(data.changed)
			if err != nil {
				logger().Error("config reload: marshal diff changed", "root", root, "error", err)
			} else {
				s.Changed = string(j)
			}
		}
		sections = append(sections, s)
	}

	return sections
}

// topLevelRoot extracts the first segment of a config key path.
// "bgp/peer/foo" → "bgp", "environment" → "environment".
func topLevelRoot(key string) string {
	root, _, _ := strings.Cut(key, config.PathSep)
	return root
}

// groupRootFor picks the diff-section root a config key belongs to: the LONGEST
// declared root that equals the key or is a path-prefix of it, else the key's
// top-level root.
//
// Matching is on whole path segments, so "ddos/localhost/x" is NOT claimed by a
// declared "ddos/local" -- a plain strings.HasPrefix would file it there and hand
// one plugin another's keys.
func groupRootFor(key string, declaredRoots []string) string {
	best := ""
	for _, root := range declaredRoots {
		if root == "" || root == wildcardConfigRoot {
			continue
		}
		if key != root && !strings.HasPrefix(key, root+config.PathSep) {
			continue
		}
		if len(root) > len(best) {
			best = root
		}
	}
	if best != "" {
		return best
	}
	return topLevelRoot(key)
}
