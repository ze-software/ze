// Design: docs/architecture/config/syntax.md — BGP plugin reference validation
// Overview: loader.go — reactor loading calls plugin reference validation

package bgpconfig

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

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
			if err := validatePeerProcessRefs(peerTree, pluginNames, "bgp/group "+groupName+" peer "+addr); err != nil {
				return err
			}
		}
	}

	// Walk standalone peer process bindings.
	for _, entry := range bgpContainer.GetListOrdered("peer") {
		addr := entry.Key
		peerTree := entry.Value
		if err := validatePeerProcessRefs(peerTree, pluginNames, "bgp/peer "+addr); err != nil {
			return err
		}
	}

	return nil
}

// validatePeerProcessRefs checks that every process binding on a peer names a
// plugin that exists: one the config declares, one `--plugin` loaded, or one
// ze's own registry holds. context is the error message prefix
// (e.g., "bgp.peer 10.0.0.1").
//
// The registry is consulted because a peer's process reference states this
// peer's relationship to a program, while `bus { endpoint ... }` states how to
// start one, and a plugin ze auto-loads from a config root is started without
// an explicit endpoint declaration (for example `redistribute-orchestrator` is
// auto-loaded by the `redistribute { }` root rather than declared in `bus`).
//
// The guard keeps its purpose. A typo is still refused, because a misspelled
// name is in neither the config nor the registry.
func validatePeerProcessRefs(peerTree *config.Tree, pluginNames map[string]bool, context string) error {
	attach := peerTree.GetContainer("attach")
	if attach == nil {
		return nil
	}
	processList := attach.GetList("process")
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
		if !pluginNames[name] && !registry.Has(name) {
			return fmt.Errorf("%s: undefined plugin %q in process binding", context, name)
		}
	}
	return nil
}
