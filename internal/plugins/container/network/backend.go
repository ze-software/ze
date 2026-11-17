// Design: docs/architecture/container/network.md -- the backend rule V-19
// Related: verify.go -- the parse that calls it

package network

import (
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// interfaceBackendDefault and firewallBackendDefault are the defaults of
	// the two backend leaves, as their schemas state them.
	interfaceBackendDefault = "netlink"
	firewallBackendDefault  = "nft"
	// interfaceBackendLeafPath names the leaf the gate's refusal tells the
	// operator to change.
	interfaceBackendLeafPath = "/interface/backend"
	// translationBackend is the only firewall backend that carries the
	// translation tables: the VPP firewall verifier refuses every regular
	// chain, and each network's outbound chain is one.
	translationBackend = "nft"
)

// backendGateSchemaKeeper names the gate's schema in the refusal of a module
// publish that comes too late (yang.PublishExternalModules).
const backendGateSchemaKeeper = "the container network backend gate schema"

// The gate's schema is loaded once per process, on the first config that
// holds a network.
var (
	backendGateSchemaOnce sync.Once
	backendGateSchema     *config.Schema
	backendGateSchemaErr  error
)

// checkBackends is rule V-19. The container network list carries
// `ze:backend "netlink"`, so the commit-time gate refuses a network when the
// interface backend cannot create a bridge and a link; and a network that
// translates needs the firewall backend that carries the tables.
func checkBackends(read delivery, cfg Config) error {
	interfaceBackend := backendLeaf(read.interfaces, interfaceBackendDefault)
	if err := runBackendGate(read.containerData, interfaceBackend); err != nil {
		return err
	}
	firewallBackend := backendLeaf(read.firewall, firewallBackendDefault)
	if firewallBackend == translationBackend {
		return nil
	}
	for _, network := range cfg.Networks {
		if network.Masquerade {
			return opererr.New(
				"container network "+network.Name+" needs a backend this configuration does not select: firewall backend "+firewallBackend+" cannot carry its translation tables",
				"outbound translation is a table of a regular chain per network, and only the nft backend carries one",
				"set firewall backend nft, or set container network "+network.Name+" masquerade false",
			)
		}
	}
	return nil
}

// backendLeaf answers the backend leaf of a component's section, or the
// schema's default when the leaf is absent.
func backendLeaf(body map[string]any, fallback string) string {
	if word, isWord := body[leafBackend].(string); isWord {
		return word
	}
	return fallback
}

// runBackendGate walks the container section with the interface backend, as
// the interface and firewall components walk their own sections.
func runBackendGate(containerData, interfaceBackend string) error {
	yang.KeepsModuleSet(backendGateSchemaKeeper)
	backendGateSchemaOnce.Do(func() {
		backendGateSchema, backendGateSchemaErr = config.YANGSchema()
	})
	if backendGateSchemaErr != nil {
		return opererr.Wrap(backendGateSchemaErr,
			"the container network backend check could not load the config schema",
			"ze refuses a container network it cannot judge against the interface backend",
			"ze show doctor",
		)
	}
	refusals := config.ValidateBackendFeaturesJSON(containerData, backendGateSchema, containerRoot, interfaceBackend, interfaceBackendLeafPath)
	if len(refusals) == 0 {
		return nil
	}
	var problem textbuf.Buffer
	problem.Str("a container network needs a backend this configuration does not select: ")
	for index, refusal := range refusals {
		if index > 0 {
			problem.Str("; ")
		}
		problem.Str(refusal.Error())
	}
	return opererr.New(
		problem.String(),
		"the interface backend "+interfaceBackend+" cannot create the network's bridge and its containers' links",
		"set interface backend netlink",
	)
}
