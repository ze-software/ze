// Design: docs/guide/command-reference.md -- clear verb for interface counters
// Related: cmd.go -- sibling iface RPC handlers (interface lifecycle)

package cmd

import (
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-clear:interface-counters",
			Handler:    handleClearInterfaceCounters,
		},
	)
}

// handleClearInterfaceCounters zeros RX/TX counters.
//
// Canonical grammar (action before identifier):
//
//	clear interface counters             args=[]                   -> all
//	clear interface counters             args=["counters"]         -> all
//	clear interface counters <name>      args=["counters", <name>] -> one
//
// Deprecated grammars (accepted with deprecation warning):
//
//	clear interface <name> counters      args=[<name>, "counters"] -> one
//	clear interface <name>               args=[<name>]             -> one
func handleClearInterfaceCounters(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const (
		usage  = "usage: clear interface counters [<name>]"
		kwCtrs = "counters"
	)

	name := ""
	deprecated := false

	switch len(args) {
	case 0:
		// all
	case 1:
		if args[0] == kwCtrs {
			// "clear interface counters" -> all
		} else {
			// Deprecated: "clear interface <name>" -> one
			name = args[0]
			deprecated = true
		}
	case 2:
		switch {
		case args[0] == kwCtrs:
			// Canonical: "clear interface counters <name>" -> one
			name = args[1]
		case args[1] == kwCtrs:
			// Deprecated: "clear interface <name> counters" -> one
			name = args[0]
			deprecated = true
		default:
			return &plugin.Response{Status: plugin.StatusError, Data: usage}, nil
		}
	default:
		return &plugin.Response{Status: plugin.StatusError, Data: usage}, nil
	}

	if err := iface.ResetCounters(name); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error via Response
	}

	scope := name
	if scope == "" {
		scope = "all"
	}
	resp := &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"cleared": scope,
		},
	}
	if deprecated {
		newForm := "clear interface counters"
		if name != "" {
			newForm += " " + name
		}
		if data, ok := resp.Data.(map[string]any); ok {
			data["deprecated"] = "use: " + newForm
		}
	}
	return resp, nil
}
