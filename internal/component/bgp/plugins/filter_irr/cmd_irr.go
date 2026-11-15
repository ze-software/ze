// Design: docs/architecture/bgp/filter-irr.md -- YANG command forwarding for IRR filter plugin.
// Owned by bgp-filter-irr so removing the plugin removes these command nodes.

package filter_irr

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	cmdShowIRR       = "show bgp irr"
	cmdShowIRRPrefix = "show bgp irr prefix"
	cmdShowIRRCheck  = "show bgp irr check"
	columnASN        = "asn"
)

func init() {
	registerIRRShapes()

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-irr-status",
			Handler:       forwardShowIRR,
			PluginCommand: cmdShowIRR,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-irr-prefix",
			Handler:       forwardShowIRRPrefix,
			PluginCommand: cmdShowIRRPrefix,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-bgp:show-irr-check",
			Handler:       forwardShowIRRCheck,
			PluginCommand: cmdShowIRRCheck,
		},
	)
}

// registerIRRShapes declares what the three `show bgp irr` answers hold, so the
// operators each one cannot support are refused before it runs and the ones it
// can are published.
//
// The commands are served by the bgp-filter-irr plugin PROCESS and registered
// here by an in-core shim, and a declaration is read in the process holding the
// registry. So the three declare today, with no change to the plugin wire
// contract and no change to the answers themselves.
//
// The BGP peer command plugin declares `show bgp irr` empty, as a child of
// `show bgp`. An empty declaration is a floor and never overrides a value, so
// the two agree whatever order the packages initialize in
// (declarationRegistry.declare, internal/component/command/column_order.go).
func registerIRRShapes() {
	// `show bgp irr` answers one row for each configured ASN, under "entries"
	// (showIRR, command.go). The refresh schedule is the irr plugin's, and
	// `show irr` answers it.
	command.RegisterShape([]string{cmdShowIRR}, command.ShapeTab)

	// An entry reads as the answer to one question: whose prefixes, from which
	// set, is a stored copy enforced, how many prefixes it holds, which stored
	// copy it is (the server it came from, the generation the irr plugin gave
	// it, when the server last answered and how old its oldest prefixes are),
	// and which peers it filters. "error" follows "status" because it is the
	// reason for it, and it is written only when the status is "error".
	command.RegisterColumns([]string{cmdShowIRR},
		command.ColumnOrder{
			columnASN, "as-set", "status", "error",
			"ipv4-count", "ipv6-count", "server", "generation", "checked-at", "refreshed-at", "peers",
		},
	)

	// `show bgp irr prefix` answers the prefixes one peer's ASN resolves to,
	// under "prefixes", beside the ASN and the set they came from
	// (renderPrefixes, command.go).
	//
	// The rows are bare PREFIX STRINGS rather than records, so there is nothing
	// for a column order to read them against and the shape is `map` rather
	// than `tab`. `| count` and `| first` still act on them; `| fill`, which
	// re-sequences columns, is refused by name.
	command.RegisterShape([]string{cmdShowIRRPrefix}, command.ShapeMap)
	command.RegisterColumns([]string{cmdShowIRRPrefix},
		command.ColumnOrder{columnASN, "as-set", "prefixes"},
	)

	// `show bgp irr check` answers ONE verdict for one peer and one prefix
	// (showIRRCheck, command.go), so every row operator is refused by name
	// before the command runs: `| first 1` used to answer the prefix alone,
	// dropping the verdict the operator asked for.
	command.RegisterShape([]string{cmdShowIRRCheck}, command.ShapeDoc)
	command.RegisterColumns([]string{cmdShowIRRCheck},
		command.ColumnOrder{"prefix", columnASN, "accepted", "matched-entry"},
	)

	// None of the three declares an address field, and each has a value that
	// looks like one. "peers" is an ARRAY of address strings inside an entry
	// row, "prefixes" is an array of prefix strings, and "prefix" is one prefix
	// string. resolveJSON and originJSON decorate a map VALUE that passes
	// netip.ParseAddr and nothing else (internal/component/command/pipe_resolve.go,
	// pipe_origin.go): an array element is walked past and a prefix does not
	// parse. Declaring any of them would publish `| resolve` and `| origin` on
	// an answer neither operator can change.
	command.RegisterAddressFields([]string{cmdShowIRR, cmdShowIRRPrefix, cmdShowIRRCheck})
}

func forwardShowIRR(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRR, args, ctx.PeerSelector())
}

func forwardShowIRRPrefix(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRRPrefix, args, ctx.PeerSelector())
}

func forwardShowIRRCheck(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowIRRCheck, args, ctx.PeerSelector())
}
