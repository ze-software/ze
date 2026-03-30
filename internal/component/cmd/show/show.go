// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Overview: doc.go -- show verb package registration

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:       "ze-show:bgp-peer",
			Handler:          peer.HandleBgpPeerDetail,
			RequiresSelector: true,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:bgp-warnings",
			Handler:    peer.HandleBgpWarnings,
		},
	)
}
