// Design: docs/architecture/api/process-protocol.md — plugin auto-loading
// Overview: startup.go — plugin startup phases
// Related: config.go — ServerConfig with ConfiguredFamilies and ConfiguredCustomEvents

package server

import (
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
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
	// Collect producing plugins for each custom event type.
	var needed []string
	seen := make(map[string]bool)

	for _, eventType := range s.config.ConfiguredCustomEvents {
		pluginName := plugin.GetPluginForEventType(eventType)
		if pluginName == "" {
			logger().Debug("no plugin produces event type, skipping", "event", eventType)
			continue
		}

		// Skip if already configured or running
		if s.hasConfiguredPlugin(pluginName) || (s.procManager != nil && s.procManager.GetProcess(pluginName) != nil) {
			logger().Debug("event type plugin already configured, skipping auto-load",
				"event", eventType, "plugin", pluginName)
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

	// Resolve transitive dependencies (e.g., bgp-rpki-decorator depends on bgp-rpki).
	resolved, err := registry.ResolveDependencies(needed)
	if err != nil {
		logger().Warn("event type auto-load: dependency resolution failed, loading without dependencies",
			"plugins", needed, "error", err)
		resolved = needed
	}

	// Build plugin configs for all resolved names not already running.
	var plugins []plugin.PluginConfig
	for _, name := range resolved {
		if s.hasConfiguredPlugin(name) || (s.procManager != nil && s.procManager.GetProcess(name) != nil) {
			continue
		}

		logger().Debug("auto-loading plugin for custom event type",
			"plugin", name)

		plugins = append(plugins, plugin.PluginConfig{
			Name:     name,
			Encoder:  "json",
			Internal: true,
		})
	}

	return plugins
}
