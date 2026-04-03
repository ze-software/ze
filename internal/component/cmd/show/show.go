// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Overview: doc.go -- show verb package registration

package show

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:version",
			Handler:    handleShowVersion,
		},
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

// handleShowVersion returns the ze version and build date.
func handleShowVersion(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	v, d := pluginserver.GetVersion()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   fmt.Sprintf("ze %s (built %s)", v, d),
	}, nil
}
