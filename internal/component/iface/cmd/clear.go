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

// handleClearInterfaceCounters zeros RX/TX counters. Backends that
// cannot physically reset counters (Linux netlink) fall back to a
// per-interface baseline handled by iface.ResetCounters -- from the
// operator's viewpoint counters read zero immediately after this call
// regardless of which path fired.
//
// Accepted grammars (paralleling `show interface <name> counters`):
//
//	clear interface counters             args=[]                  -> all
//	clear interface counters             args=["counters"]        -> all
//	clear interface <name> counters      args=[<name>, "counters"]-> one
//	clear interface <name>               args=[<name>]            -> one
//
// Anything else is a usage error so the operator gets a clear message
// rather than silently clearing "all" on a typo.
//
// Keyword/name ambiguity: if an interface is literally named "counters",
// bare `clear interface counters` (args=["counters"]) is treated as the
// keyword and clears ALL interfaces (including the one named counters).
// Target that specific interface via the tolerated
// `clear interface counters counters` form (args=["counters","counters"]),
// or rename the interface. Not an issue in practice -- no operator
// names an iface "counters" -- but documented so the grammar is honest.
//
// The response always distinguishes "cleared: all" from "cleared: <name>"
// so scripts can assert against it without reparsing args.
func handleClearInterfaceCounters(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const (
		usage  = "usage: clear interface [<name>] counters"
		kwCtrs = "counters"
	)
	name := ""
	switch len(args) {
	case 0:
		// all
	case 1:
		if args[0] != kwCtrs {
			name = args[0]
		}
	case 2:
		switch {
		case args[1] == kwCtrs:
			name = args[0]
		case args[0] == kwCtrs:
			// redundant but tolerated: `counters <name>`
			name = args[1]
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
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"cleared": scope,
		},
	}, nil
}
