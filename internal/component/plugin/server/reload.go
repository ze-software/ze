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

	"github.com/ze-software/ze/internal/component/config"
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

// txLock enforces one config transaction at a time.
// CLI/API commits are rejected when locked. SIGHUP is queued.
type txLock struct {
	mu     sync.Mutex
	locked bool
	sighup bool
}

// tryAcquire attempts to acquire the transaction lock. Returns false if already held.
func (l *txLock) tryAcquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locked {
		return false
	}
	l.locked = true
	return true
}

// release releases the transaction lock.
func (l *txLock) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
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

// TxLocked reports whether a config transaction is in progress.
func (s *Server) TxLocked() bool {
	s.txLock.mu.Lock()
	defer s.txLock.mu.Unlock()
	return s.txLock.locked
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

// HasFullReloadFunc reports whether a hub-level reload hook has been set.
func (s *Server) HasFullReloadFunc() bool {
	return s.fullReload != nil
}

// ReloadFull runs the hub-level reload hook.
func (s *Server) ReloadFull(ctx context.Context) error {
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

// reloadConfig is the internal implementation of ReloadConfig.
func (s *Server) reloadConfig(ctx context.Context, newTree map[string]any) error {
	// Prevent concurrent reloads via transaction lock.
	if !s.txLock.tryAcquire() {
		return ErrReloadInProgress
	}
	defer s.txLock.release()

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
		// No plugins care about these changes — just update.
		logger().Info("config reload: no affected plugins, updating config")
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
		logger().Warn("config reload: transaction failed", "error", err)
		if len(autoLoaded) > 0 {
			logger().Info("config reload: stopping auto-loaded plugins after failed transaction", "plugins", autoLoaded)
			s.autoStopPluginNames(autoLoaded)
		}
		return err
	}

	// Transaction committed. Now stop plugins whose config sections were
	// removed. Deferred to here so a failed verify/apply does not leave
	// plugins stopped with no way to restore them.
	if len(removedKeys) > 0 {
		s.autoStopForRemovedConfigPaths(removedKeys)
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

// diffRootData groups diff entries by their top-level root key.
type diffRootData struct {
	added   map[string]any
	removed map[string]any
	changed map[string]any
}

// buildDiffSections converts a config.ConfigDiff into per-root ConfigDiffSections.
// Groups flat config keys (e.g., "bgp/peer/foo") by their top-level root ("bgp").
func buildDiffSections(diff *config.ConfigDiff) []rpc.ConfigDiffSection {
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

	for k, v := range diff.Added {
		r := ensure(topLevelRoot(k))
		r.added[k] = v
	}
	for k, v := range diff.Removed {
		r := ensure(topLevelRoot(k))
		r.removed[k] = v
	}
	for k, v := range diff.Changed {
		r := ensure(topLevelRoot(k))
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
