// Design: docs/architecture/api/commands.md — update verb RPC registration
// Overview: doc.go — update verb package registration

package update

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:       "ze-update:bgp-peer-prefix",
			Handler:          peer.HandleBgpPeerPrefixUpdate,
			RequiresSelector: true,
		},
	)
}
