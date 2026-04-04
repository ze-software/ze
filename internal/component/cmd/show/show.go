// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Overview: doc.go -- show verb package registration

package show

import (
	"encoding/json"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
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
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:interface",
			Handler:    handleShowInterface,
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

// handleShowInterface lists all interfaces or shows one by name.
// Args: optional interface name.
func handleShowInterface(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) > 0 {
		info, err := iface.GetInterface(args[0])
		if err != nil {
			return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
		}
		data, jsonErr := json.Marshal(info)
		if jsonErr != nil {
			return nil, fmt.Errorf("show interface: marshal: %w", jsonErr)
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
	}

	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	data, jsonErr := json.Marshal(ifaces)
	if jsonErr != nil {
		return nil, fmt.Errorf("show interface: marshal: %w", jsonErr)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}
