// Design: docs/architecture/api/commands.md — del verb RPC registration
// Overview: doc.go — del verb package registration

package del

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:       "ze-del:bgp-peer",
			Handler:          peer.HandleBgpPeerRemove,
			Help:             "Remove a peer dynamically",
			RequiresSelector: true,
		},
	)
}
