// Design: docs/architecture/api/commands.md — set verb RPC registration
// Overview: doc.go — set verb package registration

package set

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:       "ze-set:bgp-peer-with",
			Handler:          peer.HandleBgpPeerWith,
			Help:             "Set peer with configuration (asn, local-as, timer, etc.)",
			RequiresSelector: true,
		},
		pluginserver.RPCRegistration{
			WireMethod:       "ze-set:bgp-peer-save",
			Handler:          peer.HandleBgpPeerSave,
			Help:             "Save peer(s) to config file",
			RequiresSelector: true,
		},
	)
}
