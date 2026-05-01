// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: reload_tx.go — transaction coordinator wiring
// Related: config_tx_bridge.go — engine-side RPC bridge for per-plugin verify/apply events

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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

// SetConfigLoader sets the function used by ReloadFromDisk to load the config tree.
func (s *Server) SetConfigLoader(loader ConfigLoader) {
	s.configLoader = loader
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
		return fmt.Errorf("no config loader configured")
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
		return fmt.Errorf("no reactor configured")
	}

	logger().Info("config reload started")

	// Get running config.
	running := s.reactor.GetConfigTree()

	// Compute diff.
	diff := diffMaps(running, newTree)
	if len(diff.added) == 0 && len(diff.removed) == 0 && len(diff.changed) == 0 {
		logger().Info("config reload: no changes")
		return nil // No changes
	}

	logger().Debug("config reload diff",
		"added", len(diff.added), "removed", len(diff.removed), "changed", len(diff.changed))

	// Collect removed config keys for deferred stop (after verify+apply succeeds).
	// Stopping plugins before the transaction is proven risks divergence: if
	// verify/apply fails, the stopped plugins are gone and cannot be restarted
	// with their old config.
	var removedKeys []string
	if len(diff.removed) > 0 {
		removedKeys = make([]string, 0, len(diff.removed))
		for k := range diff.removed {
			removedKeys = append(removedKeys, k)
		}
	}

	// Auto-load plugins for newly added config sections.
	// When a user adds fib { kernel { } } to their config, the fib-kernel plugin
	// needs to start before we can send it config.
	var autoLoaded []string
	if len(diff.added) > 0 {
		addedKeys := make([]string, 0, len(diff.added))
		for k := range diff.added {
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
	if err := s.runTxCoordinator(ctx, affected, diff); err != nil {
		logger().Warn("config reload: transaction failed", "error", err)
		if len(autoLoaded) > 0 {
			logger().Info("config reload: stopping auto-loaded plugins after failed transaction", "plugins", autoLoaded)
			s.autoStopForRemovedConfigPaths(autoLoaded)
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

// configDiff holds the difference between two config maps.
// Local to the plugin package to avoid import cycles with internal/config.
type configDiff struct {
	added   map[string]any
	removed map[string]any
	changed map[string]diffPair
}

// diffPair holds old and new values for a changed key.
type diffPair struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// diffMaps computes a deep diff between two map[string]any.
// Nested maps are compared recursively with slash-separated key paths.
// Equivalent to config.DiffMaps — duplicated here to avoid import cycle.
func diffMaps(old, newMap map[string]any) *configDiff {
	diff := &configDiff{
		added:   make(map[string]any),
		removed: make(map[string]any),
		changed: make(map[string]diffPair),
	}
	diffMapsRecursive(old, newMap, "", diff)
	return diff
}

// diffMapsRecursive performs recursive comparison with path prefix.
func diffMapsRecursive(old, newMap map[string]any, prefix string, diff *configDiff) {
	if old == nil {
		old = make(map[string]any)
	}
	if newMap == nil {
		newMap = make(map[string]any)
	}

	for k, oldVal := range old {
		key := diffJoinPath(prefix, k)
		if _, exists := newMap[k]; !exists {
			diff.removed[key] = oldVal
		}
	}

	for k, newVal := range newMap {
		key := diffJoinPath(prefix, k)
		oldVal, exists := old[k]

		if !exists {
			diff.added[key] = newVal
			continue
		}

		oldMap, oldIsMap := oldVal.(map[string]any)
		newSubMap, newIsMap := newVal.(map[string]any)

		switch {
		case oldIsMap && newIsMap:
			diffMapsRecursive(oldMap, newSubMap, key, diff)
		case oldIsMap != newIsMap:
			diff.changed[key] = diffPair{Old: oldVal, New: newVal}
		case !reflect.DeepEqual(oldVal, newVal):
			diff.changed[key] = diffPair{Old: oldVal, New: newVal}
		}
	}
}

// diffJoinPath joins prefix and key with the config path separator.
func diffJoinPath(prefix, key string) string {
	return config.AppendPath(prefix, key)
}

// rootHasChanges returns true if the diff contains changes under the given root.
// Checks config key paths: "bgp" matches "bgp", "bgp/peer", "bgp/peer/foo", etc.
func rootHasChanges(diff *configDiff, root string) bool {
	if root == "*" {
		return len(diff.added) > 0 || len(diff.removed) > 0 || len(diff.changed) > 0
	}

	prefix := root + config.PathSep
	for k := range diff.added {
		if k == root || strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range diff.removed {
		if k == root || strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range diff.changed {
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

// buildDiffSections converts a configDiff into per-root ConfigDiffSections.
// Groups flat config keys (e.g., "bgp/peer/foo") by their top-level root ("bgp").
func buildDiffSections(diff *configDiff) []rpc.ConfigDiffSection {
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

	for k, v := range diff.added {
		r := ensure(topLevelRoot(k))
		r.added[k] = v
	}
	for k, v := range diff.removed {
		r := ensure(topLevelRoot(k))
		r.removed[k] = v
	}
	for k, v := range diff.changed {
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
