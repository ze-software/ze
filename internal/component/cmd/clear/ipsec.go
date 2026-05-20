// Design: plan/spec-ipsec-10-cli-diag.md -- clear vpn ipsec sa handler

package clear

import (
	"codeberg.org/thomas-mangin/ze/internal/component/ike/engine"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
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
			return &plugin.Response{Status: plugin.StatusError, Data: "clear vpn ipsec sa peer: missing peer name"}, nil
		}
		name := args[i+1]
		if name == "" || len(name) > 255 {
			return &plugin.Response{Status: plugin.StatusError, Data: "clear vpn ipsec sa peer: name must be 1-255 characters"}, nil
		}
		ok := engine.TerminatePeerSA(name)
		if !ok {
			return &plugin.Response{Status: plugin.StatusError, Data: "peer not found: " + name}, nil
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   map[string]any{"action": "clear-peer", "peer": name},
		}, nil
	}

	count := engine.TerminateAllSAs()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   map[string]any{"action": "clear-all", "terminated": count},
	}, nil
}
