package config

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/reactor"
)

// ExtractPluginsFromTree extracts plugin configurations from a parsed config tree.
// Returns []reactor.PluginConfig ready for reactor consumption.
//
// Sources:
//  1. Explicit plugins: plugin { external <name> { run; encoder; timeout; } }
//  2. Inline plugins: peer process bindings with "run" defined (discovered after template resolution)
func ExtractPluginsFromTree(tree *Tree) ([]reactor.PluginConfig, error) {
	var plugins []reactor.PluginConfig

	// Source 1: Explicit plugins from plugin { external <name> { ... } }
	if pluginContainer := tree.GetContainer("plugin"); pluginContainer != nil {
		for name, proc := range pluginContainer.GetList("external") {
			// Reject reserved names (underscore prefix used internally)
			if strings.HasPrefix(name, "_") {
				return nil, fmt.Errorf("plugin name %q: names starting with underscore are reserved", name)
			}
			pc := reactor.PluginConfig{Name: name}
			if v, ok := proc.Get("run"); ok {
				pc.Run = v
			}
			if v, ok := proc.Get("encoder"); ok {
				pc.Encoder = v
			}
			if v, ok := proc.Get("timeout"); ok {
				d, err := time.ParseDuration(v)
				if err != nil {
					return nil, fmt.Errorf("plugin %q: invalid timeout %q: %w", name, v, err)
				}
				if d < 0 {
					return nil, fmt.Errorf("plugin %q: timeout must be positive, got %q", name, v)
				}
				pc.StageTimeout = d
			}
			// Default: text encoder plugins receive updates
			if pc.Encoder == EncoderText {
				pc.ReceiveUpdate = true
			}
			plugins = append(plugins, pc)
		}
	}

	// Source 2: Inline plugins from peer process bindings with "run" defined.
	// Resolve templates first so inherited process bindings are visible.
	// Skip if no bgp block (plugin-only configs are valid).
	var inlinePlugins []reactor.PluginConfig
	if tree.GetContainer("bgp") != nil {
		bgpTree, err := ResolveBGPTree(tree)
		if err != nil {
			return nil, fmt.Errorf("resolve templates for plugin extraction: %w", err)
		}
		inlinePlugins = extractInlinePluginsFromMap(bgpTree)
	}

	// Build set of explicit plugin names for dedup
	explicit := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		explicit[p.Name] = true
	}

	// Append inline plugins not already declared as explicit
	for _, ip := range inlinePlugins {
		if !explicit[ip.Name] {
			plugins = append(plugins, ip)
			explicit[ip.Name] = true
		}
	}

	return plugins, nil
}

// extractInlinePluginsFromMap finds inline plugins in the resolved bgpTree map.
// An inline plugin is a process binding with "run" defined inside a peer config.
func extractInlinePluginsFromMap(bgpTree map[string]any) []reactor.PluginConfig {
	peers, ok := bgpTree["peer"].(map[string]any)
	if !ok {
		return nil
	}

	seen := make(map[string]bool)
	var plugins []reactor.PluginConfig

	for _, peerVal := range peers {
		peerMap, ok := peerVal.(map[string]any)
		if !ok {
			continue
		}
		processes, ok := peerMap["process"].(map[string]any)
		if !ok {
			continue
		}
		for name, procVal := range processes {
			procMap, ok := procVal.(map[string]any)
			if !ok {
				continue
			}
			run, ok := procMap["run"].(string)
			if !ok || run == "" {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			plugins = append(plugins, reactor.PluginConfig{
				Name:          name,
				Run:           run,
				Encoder:       EncoderText, // Default to text encoder
				ReceiveUpdate: true,        // Default: receive updates
			})
		}
	}

	return plugins
}

// ValidatePluginReferences checks that all process binding plugin references
// point to declared plugins. Skip bindings with inline Run (defines plugin inline).
func ValidatePluginReferences(tree *Tree, plugins []reactor.PluginConfig) error {
	bgpContainer := tree.GetContainer("bgp")
	if bgpContainer == nil {
		return nil
	}

	// Build set of known plugin names
	pluginNames := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		pluginNames[p.Name] = true
	}

	// Walk peer process bindings
	for _, entry := range bgpContainer.GetListOrdered("peer") {
		addr := entry.Key
		peerTree := entry.Value
		processList := peerTree.GetList("process")
		for name, processTree := range processList {
			if name == KeyDefault {
				continue
			}
			// Skip inline plugins (have run defined)
			if run, ok := processTree.Get("run"); ok && run != "" {
				continue
			}
			if !pluginNames[name] {
				return fmt.Errorf("bgp.peer %s: undefined plugin %q in process binding", addr, name)
			}
		}
	}

	return nil
}
