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
			Help:             "Show peer(s) details",
			ReadOnly:         true,
			RequiresSelector: true,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:bgp-warnings",
			Handler:    peer.HandleBgpWarnings,
			Help:       "Show active prefix warnings (stale data, threshold exceeded)",
			ReadOnly:   true,
		},
	)
}
