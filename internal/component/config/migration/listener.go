// Design: docs/architecture/config/syntax.md -- listener config migration transformations
// Overview: migrate.go -- migration pipeline

package migration

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// hasBGPListenLeaf detects the ExaBGP legacy `bgp { listen "..." }` leaf.
func hasBGPListenLeaf(tree *config.Tree) bool {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return false
	}
	_, ok := bgp.Get("listen")
	return ok
}

// removeBGPListenLeaf removes the `bgp { listen }` leaf.
func removeBGPListenLeaf(tree *config.Tree) (*config.Tree, error) {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return tree, nil
	}
	bgp.Delete("listen")
	return tree, nil
}

// hasTCPPortLeaf detects `environment { tcp { port N } }`.
func hasTCPPortLeaf(tree *config.Tree) bool {
	env := tree.GetContainer("environment")
	if env == nil {
		return false
	}
	tcp := env.GetContainer("tcp")
	if tcp == nil {
		return false
	}
	_, ok := tcp.Get("port")
	return ok
}

// removeTCPPortLeaf removes `environment > tcp > port`.
func removeTCPPortLeaf(tree *config.Tree) (*config.Tree, error) {
	env := tree.GetContainer("environment")
	if env == nil {
		return tree, nil
	}
	tcp := env.GetContainer("tcp")
	if tcp == nil {
		return tree, nil
	}
	tcp.Delete("port")
	return tree, nil
}

// hasEnvBGPConnect detects `environment { bgp { connect ... } }`.
func hasEnvBGPConnect(tree *config.Tree) bool {
	env := tree.GetContainer("environment")
	if env == nil {
		return false
	}
	bgp := env.GetContainer("bgp")
	if bgp == nil {
		return false
	}
	_, ok := bgp.Get("connect")
	return ok
}

// removeEnvBGPConnect removes `environment > bgp > connect`.
func removeEnvBGPConnect(tree *config.Tree) (*config.Tree, error) {
	env := tree.GetContainer("environment")
	if env == nil {
		return tree, nil
	}
	bgp := env.GetContainer("bgp")
	if bgp == nil {
		return tree, nil
	}
	bgp.Delete("connect")
	return tree, nil
}

// hasEnvBGPAccept detects `environment { bgp { accept ... } }`.
func hasEnvBGPAccept(tree *config.Tree) bool {
	env := tree.GetContainer("environment")
	if env == nil {
		return false
	}
	bgp := env.GetContainer("bgp")
	if bgp == nil {
		return false
	}
	_, ok := bgp.Get("accept")
	return ok
}

// removeEnvBGPAccept removes `environment > bgp > accept`.
func removeEnvBGPAccept(tree *config.Tree) (*config.Tree, error) {
	env := tree.GetContainer("environment")
	if env == nil {
		return tree, nil
	}
	bgp := env.GetContainer("bgp")
	if bgp == nil {
		return tree, nil
	}
	bgp.Delete("accept")
	return tree, nil
}

// hasHubServerHost detects plugin hub server entries with `host` leaf (renamed to `ip`).
func hasHubServerHost(tree *config.Tree) bool {
	plug := tree.GetContainer("plugin")
	if plug == nil {
		return false
	}
	hub := plug.GetContainer("hub")
	if hub == nil {
		return false
	}
	for _, entry := range hub.GetListOrdered("server") {
		if _, ok := entry.Value.Get("host"); ok {
			return true
		}
	}
	return false
}

// renameHubServerHost renames `host` to `ip` in all plugin hub server entries.
func renameHubServerHost(tree *config.Tree) (*config.Tree, error) {
	plug := tree.GetContainer("plugin")
	if plug == nil {
		return tree, nil
	}
	hub := plug.GetContainer("hub")
	if hub == nil {
		return tree, nil
	}
	for _, entry := range hub.GetListOrdered("server") {
		if v, ok := entry.Value.Get("host"); ok {
			entry.Value.Set("ip", v)
			entry.Value.Delete("host")
		}
	}
	return tree, nil
}

// topicToSubsystem maps ExaBGP boolean topic names to Ze subsystem paths.
// Used by hasLogBooleans and migrateLogBooleans.
var topicToSubsystem = map[string]string{
	"packets":       "bgp.wire",
	"rib":           "plugin.rib",
	"configuration": "config",
	"reactor":       "bgp.reactor",
	"daemon":        "daemon",
	"processes":     "plugin",
	"network":       "bgp.wire",
	"statistics":    "bgp.metrics",
	"message":       "bgp.wire",
	"timers":        "bgp.reactor",
	"routes":        "plugin.rib",
	"parser":        "config",
}

// hasLogBooleans detects boolean topics in `environment { log { <topic> true/false; } }`.
func hasLogBooleans(tree *config.Tree) bool {
	env := tree.GetContainer("environment")
	if env == nil {
		return false
	}
	container := env.GetContainer("lo" + "g")
	if container == nil {
		return false
	}
	for topic := range topicToSubsystem {
		if _, ok := container.Get(topic); ok {
			return true
		}
	}
	return false
}

// migrateLogBooleans converts boolean topics to subsystem level syntax.
// true -> debug, false -> disabled. When multiple topics map to the same subsystem,
// "true" (debug) wins over "false" (disabled).
func migrateLogBooleans(tree *config.Tree) (*config.Tree, error) { //nolint:unparam // signature required by Transformation.Apply
	env := tree.GetContainer("environment")
	if env == nil {
		return tree, nil
	}
	container := env.GetContainer("lo" + "g")
	if container == nil {
		return tree, nil
	}

	// Collect subsystem levels. "debug" wins over "disabled" for duplicate subsystems.
	subsystems := make(map[string]string)

	for topic, subsystem := range topicToSubsystem {
		val, ok := container.Get(topic)
		if !ok {
			continue
		}
		container.Delete(topic)

		level := "disabled"
		if val == "true" {
			level = "debug"
		}

		// "debug" wins over "disabled" when multiple topics map to same subsystem.
		if existing, exists := subsystems[subsystem]; exists {
			if existing == "debug" {
				continue
			}
		}
		subsystems[subsystem] = level
	}

	for subsystem, level := range subsystems {
		container.Set(subsystem, level)
	}

	return tree, nil
}

// listenerContainers are the service containers that use the flat host+port format.
var listenerContainers = []string{"web", "ssh", "mcp", "looking-glass", "telemetry"}

// hasListenerFlatFormat detects flat `host`+`port` format in listener containers.
func hasListenerFlatFormat(tree *config.Tree) bool {
	env := tree.GetContainer("environment")
	if env == nil {
		return false
	}
	for _, name := range listenerContainers {
		svc := env.GetContainer(name)
		if svc == nil {
			continue
		}
		if _, ok := svc.Get("host"); ok {
			return true
		}
	}
	return false
}

// migrateListenerToList converts flat host+port to server list with enabled true.
// Input format:  web { host 0.0.0.0; port 3443; }.
// Output format: web { enabled true; server main { ip 0.0.0.0; port 3443; } }.
func migrateListenerToList(tree *config.Tree) (*config.Tree, error) { //nolint:unparam // signature required by Transformation.Apply
	env := tree.GetContainer("environment")
	if env == nil {
		return tree, nil
	}

	for _, name := range listenerContainers {
		svc := env.GetContainer(name)
		if svc == nil {
			continue
		}
		host, hasHost := svc.Get("host")
		if !hasHost {
			continue
		}
		port, _ := svc.Get("port")

		// Remove old leaves.
		svc.Delete("host")
		svc.Delete("port")

		// Add enabled true.
		svc.Set("enabled", "true")

		// Create server main with ip and port.
		srv := config.NewTree()
		srv.Set("ip", host)
		if port != "" {
			srv.Set("port", port)
		}
		svc.AddListEntry("server", "main", srv)
	}

	return tree, nil
}
