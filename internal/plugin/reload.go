package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

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
	// Prevent concurrent reloads.
	if !s.reloadMu.TryLock() {
		return fmt.Errorf("config reload already in progress")
	}
	defer s.reloadMu.Unlock()

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

	// Find affected plugins: those with WantsConfigRoots matching changed roots.
	type affectedPlugin struct {
		proc     *Process
		sections []rpc.ConfigSection
	}

	var affected []affectedPlugin

	if s.procManager != nil {
		s.procManager.mu.RLock()
		for _, proc := range s.procManager.processes {
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
				subtree := extractConfigSubtree(newTree, root)
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
		s.procManager.mu.RUnlock()
	}

	// Verify reactor (BGP peer settings) if bgp root has changes.
	bgpChanged := rootHasChanges(diff, "bgp")
	if bgpChanged {
		bgpSubtree, _ := newTree["bgp"].(map[string]any)
		if bgpSubtree == nil {
			bgpSubtree = map[string]any{}
		}
		if err := s.reactor.VerifyConfig(bgpSubtree); err != nil {
			logger().Warn("config reload: reactor verify failed", "error", err)
			return fmt.Errorf("config verify failed: reactor: %w", err)
		}
	}

	if len(affected) == 0 && !bgpChanged {
		// No plugins care about these changes and no BGP changes — just update.
		logger().Info("config reload: no affected plugins, updating config")
		s.reactor.SetConfigTree(newTree)
		return nil
	}

	// Plugin crash handling during reload:
	//
	// Three crash detection points protect the verify→apply protocol:
	//
	// 1. Verify phase (below): ConnB()==nil → verify error. A crashed plugin that
	//    registered WantsConfigRoots cannot validate the change, so we abort.
	//
	// 2. Pre-apply alive check: After all plugins pass verify, re-check ConnB().
	//    A plugin can die between verify and apply — applying to a subset is unsafe.
	//
	// 3. Apply phase: ConnB()==nil → apply error collected and returned.
	//    At this point some plugins may have already applied, so we continue
	//    and collect errors rather than aborting.

	// Verify phase: ask all affected plugins to validate the new config.
	if len(affected) > 0 {
		logger().Info("config reload: verify phase", "plugins", len(affected))
		var verifyErrors []string
		for _, ap := range affected {
			connB := ap.proc.ConnB()
			if connB == nil {
				verifyErrors = append(verifyErrors, fmt.Sprintf("%s: verify failed: plugin connection closed (crashed?)", ap.proc.Name()))
				continue
			}

			out, err := connB.SendConfigVerify(ctx, ap.sections)
			if err != nil {
				verifyErrors = append(verifyErrors, fmt.Sprintf("%s: %v", ap.proc.Name(), err))
				continue
			}
			if out.Status == statusError {
				verifyErrors = append(verifyErrors, fmt.Sprintf("%s: %s", ap.proc.Name(), out.Error))
			}
		}

		if len(verifyErrors) > 0 {
			logger().Warn("config reload: verify failed", "errors", len(verifyErrors))
			return fmt.Errorf("config verify failed: %s", strings.Join(verifyErrors, "; "))
		}
	}

	// Pre-apply alive check: re-verify all affected plugins still have connB.
	// A plugin could die between verify and apply — sending apply to a subset is unsafe.
	if len(affected) > 0 {
		var deadPlugins []string
		for _, ap := range affected {
			if ap.proc.ConnB() == nil {
				deadPlugins = append(deadPlugins, ap.proc.Name())
			}
		}
		if len(deadPlugins) > 0 {
			logger().Warn("config reload: plugins died between verify and apply", "plugins", deadPlugins)
			return fmt.Errorf("config apply aborted: plugins died after verify: %s", strings.Join(deadPlugins, ", "))
		}
	}

	// Apply phase: send diffs to plugins, then apply reactor peer changes.
	// Errors are collected and returned to the caller, but SetConfigTree still happens
	// because the reactor has already verified and some plugins may have applied.
	var applyErrors []string
	if len(affected) > 0 {
		logger().Info("config reload: apply phase", "plugins", len(affected))
		diffSections := buildDiffSections(diff)

		for _, ap := range affected {
			connB := ap.proc.ConnB()
			if connB == nil {
				applyErrors = append(applyErrors, fmt.Sprintf("%s: apply failed: plugin connection closed (crashed?)", ap.proc.Name()))
				continue
			}

			// Filter diff sections to only roots this plugin cares about.
			// "*" means the plugin wants all roots.
			roots := ap.proc.Registration().WantsConfigRoots
			wantsAll := slices.Contains(roots, "*")
			var pluginDiffSections []rpc.ConfigDiffSection
			for _, ds := range diffSections {
				if wantsAll || slices.Contains(roots, ds.Root) {
					pluginDiffSections = append(pluginDiffSections, ds)
				}
			}

			if len(pluginDiffSections) == 0 {
				continue
			}

			out, err := connB.SendConfigApply(ctx, pluginDiffSections)
			if err != nil {
				logger().Error("config apply RPC failed", "plugin", ap.proc.Name(), "error", err)
				applyErrors = append(applyErrors, fmt.Sprintf("%s: %v", ap.proc.Name(), err))
			} else if out.Status == statusError {
				logger().Error("config apply rejected", "plugin", ap.proc.Name(), "error", out.Error)
				applyErrors = append(applyErrors, fmt.Sprintf("%s: %s", ap.proc.Name(), out.Error))
			}
		}
	}

	// Apply reactor peer changes after plugins have applied.
	if bgpChanged {
		bgpSubtree, _ := newTree["bgp"].(map[string]any)
		if bgpSubtree == nil {
			bgpSubtree = map[string]any{}
		}
		if err := s.reactor.ApplyConfigDiff(bgpSubtree); err != nil {
			logger().Error("config reload: reactor apply failed", "error", err)
			applyErrors = append(applyErrors, fmt.Sprintf("reactor: %v", err))
		}
	}

	// Update running config even if some plugins had apply errors.
	// The reactor has already applied; the config tree must reflect the new state.
	s.reactor.SetConfigTree(newTree)
	logger().Info("config reload completed")

	if len(applyErrors) > 0 {
		return fmt.Errorf("config apply partial failure: %s", strings.Join(applyErrors, "; "))
	}

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
// Nested maps are compared recursively with dotted key paths.
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

// diffJoinPath joins prefix and key with dot separator.
func diffJoinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// rootHasChanges returns true if the diff contains changes under the given root.
// Checks dotted key paths: "bgp" matches "bgp", "bgp.peer", "bgp.peer.foo", etc.
func rootHasChanges(diff *configDiff, root string) bool {
	if root == "*" {
		return len(diff.added) > 0 || len(diff.removed) > 0 || len(diff.changed) > 0
	}

	prefix := root + "."
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
// Groups flat dotted keys (e.g., "bgp.peer.foo") by their top-level root ("bgp").
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

// topLevelRoot extracts the first segment of a dotted key path.
// "bgp.peer.foo" → "bgp", "environment" → "environment".
func topLevelRoot(key string) string {
	root, _, _ := strings.Cut(key, ".")
	return root
}
