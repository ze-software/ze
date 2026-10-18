// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- clear vpn ipsec sa handler

package cmd

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/ike/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// responseKeyAction names the member of a clear-command response that says
// which clear was performed. It is not the `action` of an XFRM policy
// (show_dataplane.go), which says allow or block.
const responseKeyAction = "action"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-ike:clear-vpn-ipsec-sa", Handler: handleClearIPsecSAAll},
		pluginserver.RPCRegistration{WireMethod: "ze-ike:clear-vpn-ipsec-sa-peer", Handler: handleClearIPsecSAPeer},
	)
}

// handleClearIPsecSAAll answers `clear vpn ipsec sa all`. The bare
// `clear vpn ipsec sa` reaches no handler: the dispatcher refuses it as a
// prefix, so every SA is terminated only when the operator types all.
func handleClearIPsecSAAll(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if len(validated.Tokens()) != 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear vpn ipsec sa all: unexpected arguments; to clear one peer type clear vpn ipsec sa peer <name>"}, nil
	}
	count := engine.TerminateAllSAs()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{responseKeyAction: "clear-all", "terminated": count},
	}, nil
}

// handleClearIPsecSAPeer answers `clear vpn ipsec sa peer <name>`, and the
// same command with the leaf keyword typed, `clear vpn ipsec sa peer name
// <name>`: the YANG leaf name carries the value either way.
func handleClearIPsecSAPeer(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	name, found := validated.Value("name")
	if !found {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear vpn ipsec sa peer: give exactly one peer name, as in clear vpn ipsec sa peer <name>"}, nil
	}
	// The value alone is one token and the keyword form two, so a third token
	// is a word this command does not take.
	if len(validated.Tokens()) > 2 {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear vpn ipsec sa peer: unexpected words after the peer name; type clear vpn ipsec sa peer <name>"}, nil
	}
	if name == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear vpn ipsec sa peer: name must be 1-255 characters"}, nil
	}
	if len(name) > 255 {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear vpn ipsec sa peer: name must be 1-255 characters"}, nil
	}
	if !engine.TerminatePeerSA(name) {
		return &plugin.Response{Status: plugin.StatusError, Error: "clear vpn ipsec sa peer: peer not found: " + name + "; show vpn ipsec sa lists the peers"}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{responseKeyAction: "clear-peer", "peer": name},
	}, nil
}
