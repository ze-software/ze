// Design: docs/architecture/config/syntax.md — BGP plugin extraction from config
// Overview: loader.go — reactor loading calls plugin extraction

package bgpconfig

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// ExtractPluginsFromTree extracts plugin configurations from a parsed config tree.
// Returns []reactor.PluginConfig ready for reactor consumption.
//
// Sources:
//  1. Explicit plugins: plugin { external <name> { run; encoder; timeout; } }
//  2. Inline plugins: peer process bindings with "run" defined (discovered after template resolution)
func ExtractPluginsFromTree(tree *config.Tree) ([]reactor.PluginConfig, error) {
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
			if pc.Encoder == config.EncoderText {
				pc.ReceiveUpdate = true
			}

			markInternalPlugin(&pc)
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

// minTokenLength is the minimum length for hub auth tokens.
const minTokenLength = 32

// ExtractHubConfig extracts plugin hub transport config from a parsed config tree.
// Returns zero-value HubConfig with no servers/clients if no hub block is present.
// Reads from: plugin { hub { server <name> { host; port; secret; }; client <name> { host; port; secret; } } }.
func ExtractHubConfig(tree *config.Tree) (plugin.HubConfig, error) {
	pluginContainer := tree.GetContainer("plugin")
	if pluginContainer == nil {
		return plugin.HubConfig{}, nil
	}
	hubContainer := pluginContainer.GetContainer("hub")
	if hubContainer == nil {
		return plugin.HubConfig{}, nil
	}

	var hub plugin.HubConfig

	// Parse named server blocks: server <name> { host; port; secret; client <name> { secret; } }
	for _, entry := range hubContainer.GetListOrdered("server") {
		srv, err := extractServerConfig(entry.Key, entry.Value)
		if err != nil {
			return plugin.HubConfig{}, fmt.Errorf("hub server %q: %w", entry.Key, err)
		}
		hub.Servers = append(hub.Servers, srv)
	}

	// Parse hub-level client blocks: client <name> { host; port; secret; }
	for _, entry := range hubContainer.GetListOrdered("client") {
		cli, err := extractClientConfig(entry.Key, entry.Value)
		if err != nil {
			return plugin.HubConfig{}, fmt.Errorf("hub client %q: %w", entry.Key, err)
		}
		hub.Clients = append(hub.Clients, cli)
	}

	return hub, nil
}

// extractServerConfig parses a named server block.
func extractServerConfig(name string, tree *config.Tree) (plugin.HubServerConfig, error) {
	srv := plugin.HubServerConfig{Name: name}

	if host, ok := tree.Get("host"); ok {
		srv.Host = host
	}

	if portStr, ok := tree.Get("port"); ok {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return srv, fmt.Errorf("invalid port %q: %w", portStr, err)
		}
		srv.Port = uint16(port)
	}

	if secret, ok := tree.Get("secret"); ok && secret != "" {
		if len(secret) < minTokenLength {
			return srv, fmt.Errorf("secret too short: minimum %d characters, got %d", minTokenLength, len(secret))
		}
		srv.Secret = secret
	}

	// Parse nested client entries: client <name> { secret "..."; }
	clients := tree.GetList("client")
	if len(clients) > 0 {
		srv.Clients = make(map[string]string, len(clients))
		for clientName, clientTree := range clients {
			clientSecret, ok := clientTree.Get("secret")
			if !ok || clientSecret == "" {
				return srv, fmt.Errorf("client %q: secret required", clientName)
			}
			if len(clientSecret) < minTokenLength {
				return srv, fmt.Errorf("client %q: secret too short: minimum %d characters, got %d", clientName, minTokenLength, len(clientSecret))
			}
			srv.Clients[clientName] = clientSecret
		}
	}

	return srv, nil
}

// extractClientConfig parses a hub-level client block (outbound connection).
func extractClientConfig(name string, tree *config.Tree) (plugin.HubClientConfig, error) {
	cli := plugin.HubClientConfig{Name: name}

	if host, ok := tree.Get("host"); ok {
		cli.Host = host
	}

	if portStr, ok := tree.Get("port"); ok {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return cli, fmt.Errorf("invalid port %q: %w", portStr, err)
		}
		cli.Port = uint16(port)
	}

	if secret, ok := tree.Get("secret"); ok && secret != "" {
		if len(secret) < minTokenLength {
			return cli, fmt.Errorf("secret too short: minimum %d characters, got %d", minTokenLength, len(secret))
		}
		cli.Secret = secret
	}

	return cli, nil
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
			pc := reactor.PluginConfig{
				Name:          name,
				Run:           run,
				Encoder:       config.EncoderText, // Default to text encoder
				ReceiveUpdate: true,               // Default: receive updates
			}
			markInternalPlugin(&pc)
			plugins = append(plugins, pc)
		}
	}

	return plugins
}

// markInternalPlugin sets Internal=true if Run resolves to an internal plugin.
// Uses ResolvePlugin for validation — rejects unknown internal names (e.g., "ze.typo").
func markInternalPlugin(pc *reactor.PluginConfig) {
	if pc.Run == "" {
		return
	}
	resolved, err := plugin.ResolvePlugin(pc.Run)
	if err != nil {
		return
	}
	if resolved.Type == plugin.PluginTypeInternal {
		pc.Internal = true
	}
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
			if err := validatePeerProcessRefs(peerTree, pluginNames, fmt.Sprintf("bgp.group %s peer %s", groupName, addr)); err != nil {
				return err
			}
		}
	}

	// Walk standalone peer process bindings.
	for _, entry := range bgpContainer.GetListOrdered("peer") {
		addr := entry.Key
		peerTree := entry.Value
		if err := validatePeerProcessRefs(peerTree, pluginNames, fmt.Sprintf("bgp.peer %s", addr)); err != nil {
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
		// Skip inline plugins (have run defined)
		if run, ok := processTree.Get("run"); ok && run != "" {
			continue
		}
		if !pluginNames[name] {
			return fmt.Errorf("%s: undefined plugin %q in process binding", context, name)
		}
	}
	return nil
}
