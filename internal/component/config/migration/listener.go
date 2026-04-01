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
