// Design: docs/architecture/api/process-protocol.md — plugin auto-loading
// Overview: startup.go — plugin startup phases
// Related: config.go — ServerConfig with ConfiguredFamilies, ConfiguredCustomEvents, ConfiguredCustomSendTypes

package server

import (
	"context"
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// getUnclaimedFamilyPlugins returns plugins to auto-load for configured families
// that are NOT claimed by any explicit plugin.
// Uses registry.LookupFamily for family-based detection (not name-based).
func (s *Server) getUnclaimedFamilyPlugins() []plugin.PluginConfig {
	seen := make(map[string]bool)
	var plugins []plugin.PluginConfig

	for _, family := range s.config.ConfiguredFamilies {
		// Family-based check: skip if already claimed by explicit plugin
		if s.registry.LookupFamily(family) != "" {
			logger().Debug("family already claimed, skipping auto-load",
				"family", family, "claimed_by", s.registry.LookupFamily(family))
			continue
		}

		// Get internal plugin for this family
		pluginName := plugin.GetPluginForFamily(family)
		if pluginName == "" {
			continue // No internal plugin for this family
		}

		// Avoid duplicates
		if seen[pluginName] {
			continue
		}
		seen[pluginName] = true

		logger().Debug("auto-loading plugin for unclaimed family",
			"plugin", pluginName, "family", family)

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
// by registered plugins. A plugin with ConfigRoots: ["fib.kernel"] loads when the
// config has fib { kernel { } }.
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
func (s *Server) autoLoadForNewConfigPaths(_ context.Context, newTree map[string]any, addedRoots []string) {
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
		return
	}

	resolved, err := registry.ResolveDependencies(needed)
	if err != nil {
		logger().Error("config reload: dependency resolution failed, aborting auto-load",
			"plugins", needed, "error", err)
		return
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
		return
	}

	if s.reactor != nil {
		s.reactor.AddAPIProcessCount(len(plugins))
	}

	if err := s.runPluginPhase(plugins); err != nil {
		logger().Error("config reload: auto-load plugin startup failed", "error", err)
		// Compensate the process count to avoid blocking "all plugins ready".
		if s.reactor != nil {
			s.reactor.AddAPIProcessCount(-len(plugins))
		}
	}
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

// autoStopForRemovedConfigPaths stops plugins whose config sections were removed.
// Matches removed root keys against ConfigRoots, stops matching plugins.
// Skips explicitly configured plugins (asymmetry prevention).
// Dependencies are also stopped if no other running plugin depends on them.
// Stopped processes are removed from the ProcessManager to prevent stale entries.
func (s *Server) autoStopForRemovedConfigPaths(removedRoots []string) {
	removedSet := make(map[string]bool, len(removedRoots))
	for _, r := range removedRoots {
		removedSet[r] = true
	}

	configRootsMap := registry.ConfigRootsMap()
	pm := s.procManager.Load()
	if pm == nil {
		return
	}

	// Find plugins that were config-loaded and whose config root is now removed.
	var toStop []string
	for pluginName, roots := range configRootsMap {
		// Skip explicitly configured plugins -- they should not be auto-stopped.
		if s.hasConfiguredPlugin(pluginName) {
			continue
		}
		for _, root := range roots {
			if !removedSet[root] && !parentRemoved(root, removedSet) {
				continue
			}
			if pm.GetProcess(pluginName) == nil {
				continue
			}
			toStop = append(toStop, pluginName)
			logger().Info("config reload: stopping plugin for removed config path",
				"plugin", pluginName, "path", root)
		}
	}

	stoppedSet := make(map[string]bool, len(toStop))
	for _, name := range toStop {
		if proc := pm.GetProcess(name); proc != nil {
			proc.Stop()
			pm.RemoveProcess(name)
			stoppedSet[name] = true
		}
	}

	// Stop orphaned dependencies: plugins that were loaded only because a
	// now-stopped plugin depended on them. A dependency is orphaned if no
	// remaining running plugin declares it in Dependencies.
	if len(stoppedSet) > 0 {
		s.stopOrphanedDependencies(pm, stoppedSet)
	}
}

// stopOrphanedDependencies stops dependency-only plugins that have no remaining dependents.
// Loops until no more orphans are found (handles transitive dependency chains).
// Skips explicitly configured plugins.
func (s *Server) stopOrphanedDependencies(pm *process.ProcessManager, stopped map[string]bool) {
	for {
		newlyStopped := false

		// Collect all dependencies of stopped plugins.
		candidates := make(map[string]bool)
		for name := range stopped {
			reg := registry.Lookup(name)
			if reg == nil {
				continue
			}
			for _, dep := range reg.Dependencies {
				candidates[dep] = true
			}
		}

		for candidate := range candidates {
			if stopped[candidate] {
				continue
			}
			// Skip explicitly configured plugins.
			if s.hasConfiguredPlugin(candidate) {
				continue
			}
			proc := pm.GetProcess(candidate)
			if proc == nil {
				continue
			}

			hasDependent := false
			for _, p := range pm.AllProcesses() {
				if stopped[p.Name()] || p.Name() == candidate {
					continue
				}
				reg := registry.Lookup(p.Name())
				if reg != nil && slices.Contains(reg.Dependencies, candidate) {
					hasDependent = true
					break
				}
			}

			if !hasDependent {
				logger().Info("config reload: stopping orphaned dependency", "plugin", candidate)
				proc.Stop()
				pm.RemoveProcess(candidate)
				stopped[candidate] = true
				newlyStopped = true
			}
		}

		if !newlyStopped {
			break
		}
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
