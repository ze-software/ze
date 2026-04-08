// Design: docs/architecture/config/syntax.md — BGP plugin extraction from config
// Overview: loader.go — reactor loading calls plugin extraction

package bgpconfig

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// extractBGPInlinePlugins extracts inline plugin configs from BGP peer process bindings.
// Registered with config.RegisterPluginExtractor so config.LoadConfig discovers them.
func extractBGPInlinePlugins(tree *config.Tree) ([]plugin.PluginConfig, error) {
	if tree.GetContainer("bgp") == nil {
		return nil, nil
	}
	bgpTree, err := ResolveBGPTree(tree)
	if err != nil {
		return nil, fmt.Errorf("resolve templates for plugin extraction: %w", err)
	}
	return extractInlinePluginsFromMap(bgpTree), nil
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
			run, _ := procMap["run"].(string)
			use, _ := procMap["use"].(string)
			if run == "" && use == "" {
				continue
			}
			if use != "" {
				run = use
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			pc := reactor.PluginConfig{
				Name:          name,
				Run:           run,
				Encoder:       config.EncoderText,
				ReceiveUpdate: true,
			}
			config.MarkInternalPlugin(&pc)
			plugins = append(plugins, pc)
		}
	}

	return plugins
}

// ValidatePluginReferences checks that all process binding plugin references
// point to declared plugins. Skip bindings with inline Run (defines plugin inline).
// Checks both standalone peers and peers inside groups.
func ValidatePluginReferences(tree *config.Tree, plugins []reactor.PluginConfig) error {
	bgpContainer := tree.GetContainer("bgp")
	if bgpContainer == nil {
		return nil
	}

	// Build set of known plugin names
	pluginNames := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		pluginNames[p.Name] = true
	}

	// Walk grouped peer process bindings.
	for _, groupEntry := range bgpContainer.GetListOrdered("group") {
		groupName := groupEntry.Key
		groupTree := groupEntry.Value
		for _, peerEntry := range groupTree.GetListOrdered("peer") {
			addr := peerEntry.Key
			peerTree := peerEntry.Value
			if err := validatePeerProcessRefs(peerTree, pluginNames, fmt.Sprintf("bgp/group %s peer %s", groupName, addr)); err != nil {
				return err
			}
		}
	}

	// Walk standalone peer process bindings.
	for _, entry := range bgpContainer.GetListOrdered("peer") {
		addr := entry.Key
		peerTree := entry.Value
		if err := validatePeerProcessRefs(peerTree, pluginNames, fmt.Sprintf("bgp/peer %s", addr)); err != nil {
			return err
		}
	}

	return nil
}

// validatePeerProcessRefs checks that all process binding references in a peer tree
// point to declared plugins. context is the error message prefix (e.g., "bgp.peer 10.0.0.1").
func validatePeerProcessRefs(peerTree *config.Tree, pluginNames map[string]bool, context string) error {
	processList := peerTree.GetList("process")
	for name, processTree := range processList {
		if name == config.KeyDefault {
			continue
		}
		// Skip inline plugins (have run or use defined)
		if run, ok := processTree.Get("run"); ok && run != "" {
			continue
		}
		if use, ok := processTree.Get("use"); ok && use != "" {
			continue
		}
		if !pluginNames[name] {
			return fmt.Errorf("%s: undefined plugin %q in process binding", context, name)
		}
	}
	return nil
}
