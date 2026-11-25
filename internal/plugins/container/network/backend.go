// Design: docs/architecture/container/network.md -- the backend rule V-19
// Related: verify.go -- the parse that calls it

package network

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
)

// translationBackend is the only firewall backend that carries the
// translation tables: the VPP firewall verifier refuses every regular
// chain, and each network's outbound chain is one.
const translationBackend = "kernel"

// checkBackends is rule V-19's semantic firewall check. The engine-owned
// ze:backend walk already judges the container network list against the
// interface data-plane selector; this check keeps the semantic rules for
// outbound masquerade and published ports, reading the firewall backend
// through the config component's selector resolution so no schema default is
// hardcoded here.
func checkBackends(read delivery, cfg Config) error {
	firewallBackend, err := resolveFirewallBackend(read.firewall)
	if err != nil {
		return err
	}
	if firewallBackend == translationBackend {
		return nil
	}
	for _, network := range cfg.Networks {
		if network.Masquerade {
			return opererr.New(
				"container network "+network.Name+" needs a backend this configuration does not select: firewall backend "+firewallBackend+" cannot carry its translation tables",
				"outbound translation is a table of a regular chain per network, and only the kernel backend carries one",
				"set firewall backend kernel, or set container network "+network.Name+" masquerade false",
			)
		}
	}
	for _, instance := range cfg.Instances {
		if len(instance.Ports) > 0 {
			return opererr.New(
				"container "+instance.Name+" publishes a port and firewall backend "+firewallBackend+" cannot carry its translation",
				"a published port is a destination translation that matches destination-local, and only the kernel backend carries that match",
				"set firewall backend kernel, or delete container instance "+instance.Name+" port "+instance.Ports[0].Name,
			)
		}
	}
	return nil
}

// resolveFirewallBackend resolves the firewall backend through the config
// component's declared selector resolution, reading the written leaf from
// firewallSection or falling back to its YANG schema default.
func resolveFirewallBackend(firewallSection map[string]any) (string, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return "", opererr.Wrap(err,
			"the container network backend check could not load the config schema",
			"ze refuses a container network it cannot judge against the firewall backend",
			"ze config validate",
		)
	}
	tree := config.NewTree()
	if word, isWord := firewallSection[leafBackend].(string); isWord && word != "" {
		tree.GetOrCreateContainer(firewallRoot).Set(leafBackend, word)
	}
	backend, _, err := config.ResolveDataPlaneSelector(tree, schema, firewallRoot)
	if err != nil {
		return "", opererr.Wrap(err,
			"the container network backend check could not resolve the firewall backend",
			"ze refuses a container network it cannot judge against the firewall backend",
			"ze config validate",
		)
	}
	return backend, nil
}
