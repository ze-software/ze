// Design: docs/architecture/api/process-protocol.md — plugin auto-loading
// Overview: startup.go — plugin startup phases
// Related: config.go — ServerConfig with ConfiguredFamilies, ConfiguredCustomEvents, ConfiguredCustomSendTypes

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
	return s.getUnclaimedPluginsForTokens(s.config.ConfiguredCustomEvents, plugin.GetPluginForEventType, "event type")
}

// getUnclaimedSendTypePlugins returns plugins to auto-load for custom send types
// referenced in peer process send config but not enabled by any explicitly configured plugin.
// For example, send [ enhanced-refresh ] triggers auto-loading bgp-route-refresh.
func (s *Server) getUnclaimedSendTypePlugins() []plugin.PluginConfig {
	return s.getUnclaimedPluginsForTokens(s.config.ConfiguredCustomSendTypes, plugin.GetPluginForSendType, "send type")
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
		if s.hasConfiguredPlugin(name) || (pm2 != nil && pm2.GetProcess(name) != nil) {
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
