// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- clear vpn ipsec sa handler

package cmd

import (
	"github.com/ze-software/ze/internal/component/ike/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-clear:vpn-ipsec-sa", Handler: handleClearIPsecSA},
	)
}

func handleClearIPsecSA(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	for i := range args {
		if args[i] != "peer" {
			continue
		}
		if i+1 >= len(args) {
			return &plugin.Response{Status: plugin.StatusError, Error: "clear vpn ipsec sa peer: missing peer name"}, nil
		}
		name := args[i+1]
		if name == "" || len(name) > 255 {
			return &plugin.Response{Status: plugin.StatusError, Error: "clear vpn ipsec sa peer: name must be 1-255 characters"}, nil
		}
		ok := engine.TerminatePeerSA(name)
		if !ok {
			return &plugin.Response{Status: plugin.StatusError, Error: "peer not found: " + name}, nil
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"action": "clear-peer", "peer": name},
		}, nil
	}

	count := engine.TerminateAllSAs()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"action": "clear-all", "terminated": count},
	}, nil
}
