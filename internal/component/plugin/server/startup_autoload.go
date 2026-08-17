// Design: docs/architecture/api/process-protocol.md — plugin auto-loading
// Overview: startup.go — plugin startup phases
// Related: config.go — ServerConfig with ConfiguredFamilies, ConfiguredCustomEvents, ConfiguredCustomSendTypes

package server

import (
	"context"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// getUnclaimedFamilyPlugins returns plugins to auto-load for configured families
// that are NOT claimed by any explicit plugin.
// Uses registry.LookupFamily for family-based detection (not name-based).
func (s *Server) getUnclaimedFamilyPlugins() []plugin.PluginConfig {
	seen := make(map[string]bool)
	var plugins []plugin.PluginConfig

	for _, fam := range s.config.ConfiguredFamilies {
		// Family-based check: skip if already claimed by explicit plugin
		if s.registry.LookupFamily(fam) != "" {
			logger().Debug("fam already claimed, skipping auto-load",
				"family", fam, "claimed_by", s.registry.LookupFamily(fam))
			continue
		}

		// Get internal plugin for this fam
		pluginName := plugin.GetPluginForFamily(fam)
		if pluginName == "" {
			continue // No internal plugin for this fam
		}

		// Avoid duplicates
		if seen[pluginName] {
			continue
		}
		seen[pluginName] = true

		logger().Debug("auto-loading plugin for unclaimed fam",
			"plugin", pluginName, "family", fam)

		plugins = append(plugins, plugin.PluginConfig{
			Name:     pluginName,
			Encoder:  "json",
			Internal: true,
		})
	}

	return plugins
}

// getUnclaimedEventTypePlugins returns plugins to auto-load for custom event types
// referenced in peer process receive config but not produced by any explicitly configured plugin.
// For example, receive [ update-rpki ] triggers auto-loading bgp-rpki-decorator and its
// dependency bgp-rpki. Dependencies are resolved transitively via registry.ResolveDependencies.
func (s *Server) getUnclaimedEventTypePlugins() []plugin.PluginConfig {
	return s.getUnclaimedPluginsForTokens(s.config.ConfiguredCustomEvents, plugin.GetPluginForEventType, "event type")
}

// getUnclaimedSendTypePlugins returns plugins to auto-load for custom send types
// referenced in peer process send config but not enabled by any explicitly configured plugin.
// For example, send [ enhanced-refresh ] triggers auto-loading bgp-route-refresh.
func (s *Server) getUnclaimedSendTypePlugins() []plugin.PluginConfig {
	return s.getUnclaimedPluginsForTokens(s.config.ConfiguredCustomSendTypes, plugin.GetPluginForSendType, "send type")
}

// getConfigPathPlugins returns plugins to auto-load based on config section presence.
// Matches ConfiguredPaths (present config containers) against ConfigRoots declared
// by registered plugins. Paths on both sides use the config package's PathSep ("/").
// A plugin with ConfigRoots: ["fib/kernel"] loads when the config has
// fib { kernel { } }.
func (s *Server) getConfigPathPlugins() []plugin.PluginConfig {
	if len(s.config.ConfiguredPaths) == 0 {
		return nil
	}

	// Build set of present paths for fast lookup.
	presentPaths := make(map[string]bool, len(s.config.ConfiguredPaths))
	for _, p := range s.config.ConfiguredPaths {
		presentPaths[p] = true
	}

	// Find plugins whose ConfigRoots match a present path.
	configRootsMap := registry.ConfigRootsMap()
	var needed []string
	seen := make(map[string]bool)

	for pluginName, roots := range configRootsMap {
		for _, root := range roots {
			if !presentPaths[root] {
				continue
			}

			pm := s.procManager.Load()
			if s.isPluginLoaded(pluginName) || s.hasConfiguredPlugin(pluginName) || (pm != nil && pm.GetProcess(pluginName) != nil) {
				continue
			}

			if !seen[pluginName] {
				seen[pluginName] = true
				needed = append(needed, pluginName)
				logger().Debug("auto-loading plugin for config path",
					"plugin", pluginName, "path", root)
			}
		}
	}

	if len(needed) == 0 {
		return nil
	}

	// Resolve transitive dependencies.
	resolved, err := registry.ResolveDependencies(needed)
	if err != nil {
		logger().Warn("config-path auto-load: dependency resolution failed",
			"plugins", needed, "error", err)
		resolved = needed
	}

	var plugins []plugin.PluginConfig
	pm2 := s.procManager.Load()
	for _, name := range resolved {
		if s.isPluginLoaded(name) || s.hasConfiguredPlugin(name) || (pm2 != nil && pm2.GetProcess(name) != nil) {
			continue
		}
		plugins = append(plugins, plugin.PluginConfig{
			Name:     name,
			Encoder:  "json",
			Internal: true,
		})
	}

	return plugins
}

// autoLoadForNewConfigPaths starts plugins for newly added config sections.
// Called during config reload when the diff shows new top-level keys.
// Navigates into the nested config tree using paths from the diff,
// matches against ConfigRoots, starts matching plugins via runPluginPhase.
// Returns the names of successfully started plugins so the caller can roll them
// back if the subsequent transaction fails. Startup failures are returned so
// reload fails closed instead of accepting config without its owner plugin.
func (s *Server) autoLoadForNewConfigPaths(_ context.Context, newTree map[string]any, addedRoots []string) ([]string, error) {
	// Build the set of all new paths by navigating into the nested tree.
	// diff keys are slash-separated (e.g., "fib/kernel"), so we split and descend.
	newPaths := make([]string, 0, len(addedRoots))
	for _, root := range addedRoots {
		newPaths = append(newPaths, root)
		sub := navigateNestedMap(newTree, root)
		if sub != nil {
			collectContainerMapPaths(sub, root, &newPaths)
		}
	}

	// Match against ConfigRoots from registered plugins.
	presentPaths := make(map[string]bool, len(newPaths))
	for _, p := range newPaths {
		presentPaths[p] = true
	}

	configRootsMap := registry.ConfigRootsMap()
	pm := s.procManager.Load()
	var needed []string
	seen := make(map[string]bool)

	for pluginName, roots := range configRootsMap {
		for _, root := range roots {
			if !presentPaths[root] {
				continue
			}
			if s.hasConfiguredPlugin(pluginName) || (pm != nil && pm.GetProcess(pluginName) != nil) {
				continue
			}
			if !seen[pluginName] {
				seen[pluginName] = true
				needed = append(needed, pluginName)
				logger().Info("config reload: auto-loading plugin for new config path",
					"plugin", pluginName, "path", root)
			}
		}
	}

	if len(needed) == 0 {
		return nil, nil
	}

	resolved, err := registry.ResolveDependencies(needed)
	if err != nil {
		logger().Error("config reload: dependency resolution failed, aborting auto-load",
			"plugins", needed, "error", err)
		return nil, fmt.Errorf("config-path auto-load dependency resolution: %w", err)
	}

	var plugins []plugin.PluginConfig
	for _, name := range resolved {
		if s.hasConfiguredPlugin(name) || (pm != nil && pm.GetProcess(name) != nil) {
			continue
		}
		plugins = append(plugins, plugin.PluginConfig{
			Name:     name,
			Encoder:  "json",
			Internal: true,
		})
	}

	if len(plugins) == 0 {
		return nil, nil
	}

	if s.reactor != nil {
		s.reactor.AddAPIProcessCount(len(plugins))
	}

	if err := s.runPluginPhase(plugins); err != nil {
		logger().Error("config reload: auto-load plugin startup failed", "error", err)
		if s.reactor != nil {
			s.reactor.AddAPIProcessCount(-len(plugins))
		}
		started := make([]string, len(plugins))
		for i, p := range plugins {
			started[i] = p.Name
		}
		s.autoStopPluginNames(started)
		return nil, fmt.Errorf("config-path auto-load startup: %w", err)
	}

	// Signal post-startup to newly loaded plugins.
	if s.reactor != nil {
		s.reactor.SignalPluginStartupComplete()
	}

	started := make([]string, len(plugins))
	for i, p := range plugins {
		started[i] = p.Name
	}
	return started, nil
}

// navigateNestedMap descends into a nested map using a config path (e.g., "bgp/peer").
// Returns the map at the path, or nil if the path doesn't exist or isn't a map.
func navigateNestedMap(m map[string]any, dottedPath string) map[string]any {
	parts := config.SplitPath(dottedPath)
	current := m
	for _, part := range parts {
		v, ok := current[part]
		if !ok {
			return nil
		}
		sub, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		current = sub
	}
	return current
}

// collectContainerMapPaths recursively collects config container paths from a map.
// Only descends into map[string]any children (containers), skipping leaf values.
func collectContainerMapPaths(m map[string]any, prefix string, paths *[]string) {
	for k, v := range m {
		sub, ok := v.(map[string]any)
		if !ok {
			continue // Leaf value, not a container.
		}
		path := config.AppendPath(prefix, k)
		*paths = append(*paths, path)
		collectContainerMapPaths(sub, path, paths)
	}
}

// collectProcessesForRemovedConfigPaths returns every process that removal of
// the config roots would stop, including transitive orphan dependencies.
// It does not change process or registry state.
func (s *Server) collectProcessesForRemovedConfigPaths(removedRoots []string) map[string]bool {
	removedSet := make(map[string]bool, len(removedRoots))
	for _, root := range removedRoots {
		removedSet[root] = true
	}

	pm := s.procManager.Load()
	if pm == nil {
		return nil
	}

	stopped := make(map[string]bool)
	for pluginName, roots := range registry.ConfigRootsMap() {
		if s.hasConfiguredPlugin(pluginName) || pm.GetProcess(pluginName) == nil {
			continue
		}
		for _, root := range roots {
			if removedSet[root] || parentRemoved(root, removedSet) {
				stopped[pluginName] = true
				break
			}
		}
	}
	return s.collectOrphanedDependencies(pm, stopped)
}

// stopCollectedProcesses tears down a stop set produced before the reload
// transaction starts.
func (s *Server) stopCollectedProcesses(stopped map[string]bool) {
	pm := s.procManager.Load()
	if pm == nil {
		return
	}
	for name := range stopped {
		proc := pm.GetProcess(name)
		if proc == nil {
			continue
		}
		logger().Info("config reload: stopping plugin for removed config", "plugin", name)
		s.rollbackStartupProcess(proc)
	}
}

// autoStopForRemovedConfigPaths stops plugins whose config sections were removed.
func (s *Server) autoStopForRemovedConfigPaths(removedRoots []string) {
	s.stopCollectedProcesses(s.collectProcessesForRemovedConfigPaths(removedRoots))
}

// autoStopPluginNames stops the exact plugin processes that a failed reload
// auto-loaded. Unlike autoStopForRemovedConfigPaths, the input is already a
// plugin name list, not config roots such as "fib/kernel".
func (s *Server) autoStopPluginNames(pluginNames []string) {
	pm := s.procManager.Load()
	if pm == nil {
		return
	}

	for _, name := range pluginNames {
		if s.hasConfiguredPlugin(name) {
			continue
		}
		proc := pm.GetProcess(name)
		if proc == nil {
			continue
		}
		logger().Info("config reload: stopping auto-loaded plugin", "plugin", name)
		s.rollbackStartupProcess(proc)
	}
}

// collectOrphanCandidates returns the set of plugin names that the stopped
// plugins depended on (hard or optional). Pure helper -- no process state,
// no side effects -- so it can be unit-tested without the full server wiring.
// Takes a lookup function so tests can inject a registry stub.
//
// An optional dep is collected identically to a hard dep: if plugin X was
// pulled in only because plugin Y declared it optionally and Y is now gone,
// X is orphan-eligible.
func collectOrphanCandidates(stopped map[string]bool, lookup func(string) *registry.Registration) map[string]bool {
	candidates := make(map[string]bool)
	for name := range stopped {
		reg := lookup(name)
		if reg == nil {
			continue
		}
		for _, dep := range reg.Dependencies {
			candidates[dep] = true
		}
		for _, dep := range reg.OptionalDependencies {
			candidates[dep] = true
		}
	}
	return candidates
}

// pluginDependsOn reports whether plugin `dependent` declares `candidate` in
// either its hard or optional dependency list. Pure helper for testability.
func pluginDependsOn(reg *registry.Registration, candidate string) bool {
	if reg == nil {
		return false
	}
	return slices.Contains(reg.Dependencies, candidate) ||
		slices.Contains(reg.OptionalDependencies, candidate)
}

// collectOrphanedDependencies expands stopped with dependency-only plugins
// that have no remaining dependents. It does not change process state.
func (s *Server) collectOrphanedDependencies(pm *process.ProcessManager, stopped map[string]bool) map[string]bool {
	result := make(map[string]bool, len(stopped))
	for name := range stopped {
		result[name] = true
	}

	for {
		added := false
		for candidate := range collectOrphanCandidates(result, registry.Lookup) {
			if result[candidate] || s.hasConfiguredPlugin(candidate) || pm.GetProcess(candidate) == nil {
				continue
			}

			hasDependent := false
			for _, proc := range pm.AllProcesses() {
				if result[proc.Name()] || proc.Name() == candidate {
					continue
				}
				if pluginDependsOn(registry.Lookup(proc.Name()), candidate) {
					hasDependent = true
					break
				}
			}
			if !hasDependent {
				result[candidate] = true
				added = true
			}
		}
		if !added {
			return result
		}
	}
}

// stopOrphanedDependencies stops dependency-only plugins that have no remaining dependents.
func (s *Server) stopOrphanedDependencies(pm *process.ProcessManager, stopped map[string]bool) {
	for name := range s.collectOrphanedDependencies(pm, stopped) {
		if stopped[name] {
			continue
		}
		proc := pm.GetProcess(name)
		if proc == nil {
			continue
		}
		logger().Info("config reload: stopping orphaned dependency", "plugin", name)
		s.rollbackStartupProcess(proc)
		stopped[name] = true
	}
}

// parentRemoved checks if any parent path of a config path was removed.
func parentRemoved(path string, removed map[string]bool) bool {
	for i, c := range path {
		if c == '/' && removed[path[:i]] {
			return true
		}
	}
	return false
}

// getUnclaimedPluginsForTokens is the shared implementation for auto-loading plugins
// based on token-to-plugin lookup (event types, send types). The lookupFn maps a token
// to the plugin name that provides it. The kind string is used in log messages.
func (s *Server) getUnclaimedPluginsForTokens(tokens []string, lookupFn func(string) string, kind string) []plugin.PluginConfig {
	var needed []string
	seen := make(map[string]bool)

	for _, token := range tokens {
		pluginName := lookupFn(token)
		if pluginName == "" {
			logger().Debug("no plugin provides "+kind+", skipping", kind, token)
			continue
		}

		// Skip if already configured or running
		pm := s.procManager.Load()
		if s.hasConfiguredPlugin(pluginName) || (pm != nil && pm.GetProcess(pluginName) != nil) {
			logger().Debug(kind+" plugin already configured, skipping auto-load",
				kind, token, "plugin", pluginName)
			continue
		}

		if !seen[pluginName] {
			seen[pluginName] = true
			needed = append(needed, pluginName)
		}
	}

	if len(needed) == 0 {
		return nil
	}

	// Resolve transitive dependencies.
	resolved, err := registry.ResolveDependencies(needed)
	if err != nil {
		logger().Warn(kind+" auto-load: dependency resolution failed, loading without dependencies",
			"plugins", needed, "error", err)
		resolved = needed
	}

	// Build plugin configs for all resolved names not already running.
	var plugins []plugin.PluginConfig
	pm2 := s.procManager.Load()
	for _, name := range resolved {
		if s.isPluginLoaded(name) || s.hasConfiguredPlugin(name) || (pm2 != nil && pm2.GetProcess(name) != nil) {
			continue
		}

		logger().Debug("auto-loading plugin for custom "+kind,
			"plugin", name)

		plugins = append(plugins, plugin.PluginConfig{
			Name:     name,
			Encoder:  "json",
			Internal: true,
		})
	}

	return plugins
}
