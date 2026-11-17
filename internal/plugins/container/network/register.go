// Design: docs/architecture/container/network.md -- how the network package takes part in the container plugin
// Related: register_tables.go -- the firewall table reservation
// Related: verify.go -- the parse registered here
// Related: apply.go -- the apply registered here
// Related: attach.go -- the participant registered here
//
// Package network gives a container a network: a bridge per container
// network, one link per attachment, and outbound translation. It runs in the
// container plugin's process and reaches the interface and firewall
// components only through the engine client, so it works internal and
// forked. The container package never imports it: it registers from init().
package network

import (
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/pkg/zefs"
)

// registered is the manager the container plugin's process uses.
var registered = newManager()

//nolint:gochecknoinits // registration: the package fills the container plugin's seams and is discovered through the generated imports
func init() {
	// The grant is made from init(), so every process of the ze binary holds
	// it, the forked plugin's and the daemon's alike. It covers the bridges
	// (zcn-NAME) and the host ends the interface component composes
	// (zc-INDEX-SLOT).
	iface.MustGrantOwnedDeviceNames(container.PluginName, devicePrefix)
	// The address record of each network is runtime state of the plugin.
	statestore.RegisterPluginKeys(container.PluginName, zefs.KeyContainerNetworkAddress)
	// The rules read the interface and firewall config: a bridge name or an
	// address the interface config holds, and the two backend leaves.
	container.RegisterSiblingConfigRead(interfaceRoot)
	container.RegisterSiblingConfigRead(firewallRoot)
	container.RegisterConfigSibling(participantName, parseSections, registered.apply)
	container.RegisterEngineClientReceiver(registered.receive)
	container.RegisterParticipant(&participant{manager: registered})
}
