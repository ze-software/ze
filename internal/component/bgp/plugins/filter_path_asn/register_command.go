// Design: docs/architecture/bgp/filter-path-asn.md -- YANG command forwarding for the reject-asn filter
// Detail: command.go -- the answers these three commands forward to
// Related: yang/ze-filter-path-asn-cmd.yang -- the command nodes these wire methods serve
//
// The three `show bgp reject-asn` command nodes, owned by this plugin so
// removing it removes the commands with the handlers and the config schema.
//
// The spec calls this file cmd_path_asn.go. It is named register_command.go
// because the pretool-writeedit gate refuses a Register call inside init()
// outside a file whose name starts with "register", and register.go already
// carries the plugin registration.

package filter_path_asn

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	columnASN  = "asn"
	columnName = "name"
)

func init() {
	registerRejectASNShapes()

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:reject-asn",
			Handler:       forwardShowRejectASN,
			PluginCommand: cmdShowRejectASN,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:reject-asn-name",
			Handler:       forwardShowRejectASNName,
			PluginCommand: cmdShowRejectASNName,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-show:reject-asn-known-transit-free",
			Handler:       forwardShowRejectASNTransitFree,
			PluginCommand: cmdShowRejectASNTransitFree,
		},
	)
}

// registerRejectASNShapes declares what each answer holds, so the pipe
// operators it cannot support are refused before it runs and the ones it can are
// published.
//
// The commands are served by the bgp-filter-path-asn plugin PROCESS and
// declared here by an in-core shim, because a declaration is read in the process
// that holds the command registry (filter_irr's registerIRRShapes takes the same
// route for the same reason).
func registerRejectASNShapes() {
	// `show bgp reject-asn` answers one row per configured list, under "lists"
	// (showRejectASN, command.go). Each row carries its own nested entry rows,
	// so two column orders are declared and the renderer applies the one naming
	// the most keys of the record in hand (bestColumnOrder,
	// internal/component/command/pipe_table.go).
	command.RegisterShape([]string{cmdShowRejectASN}, command.ShapeTab)
	command.RegisterColumns([]string{cmdShowRejectASN},
		// The list row reads as the answer to "what is this list, and is it
		// doing anything": its name, then the peers that name it, then what it
		// holds. A list attached to no peer is the case the two counts exist
		// for, so they come before the contents.
		command.ColumnOrder{columnName, "import-peers", "export-peers", "entries", "patterns"},
		// The entry row is one ASN: which one, where it is refused, and which
		// network it is. The annotation is last because it is context for a
		// decision the first two columns state.
		command.ColumnOrder{columnASN, "positions", "network"},
	)

	// `show bgp reject-asn name <name>` answers ONE list rather than a set of
	// them (showRejectASNName, command.go), so every row operator is refused by
	// name: `| first 1` over a single list would answer its first key and drop
	// the rest of what the operator asked for.
	command.RegisterShape([]string{cmdShowRejectASNName}, command.ShapeDoc)
	command.RegisterColumns([]string{cmdShowRejectASNName},
		command.ColumnOrder{columnName, "import-peers", "export-peers", "entries", "patterns"},
		command.ColumnOrder{columnASN, "positions", "network"},
	)

	// `show bgp reject-asn known transit-free` answers one document: the curated
	// provenance, the networks, and the block to paste (showKnownTransitFree,
	// command.go). "block" is an array of config LINES and "sources" an array of
	// URLs, so neither is a record a column order can be read against, and a row
	// operator would cut the block in half.
	command.RegisterShape([]string{cmdShowRejectASNTransitFree}, command.ShapeDoc)
	command.RegisterColumns([]string{cmdShowRejectASNTransitFree},
		command.ColumnOrder{"curated", "sources", "networks", "block"},
		// The network row, for the `| json` consumer and for the table an
		// operator gets by selecting it.
		command.ColumnOrder{columnASN, columnName, "contested"},
	)

	// None of the three carries an address field. `| resolve` and `| origin`
	// decorate a map value that parses as an address (pipe_resolve.go,
	// pipe_origin.go), and every value here is an ASN, a network name, a
	// position word or a config line. Declaring one would publish two operators
	// on an answer neither can change.
	command.RegisterAddressFields([]string{cmdShowRejectASN, cmdShowRejectASNName, cmdShowRejectASNTransitFree})
}

func forwardShowRejectASN(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowRejectASN, args, ctx.PeerSelector())
}

// forwardShowRejectASNName carries the list name to the plugin as its first
// argument.
//
// The name arrives as a SELECTOR rather than a positional. `name` is the last
// token of the command path AND the name of the leaf under it, so
// matchCommandTokens binds the value that follows it to that leaf and hands the
// handler an empty args slice (internal/component/plugin/server/command.go,
// "Explicit typed selectors"). That binding is what the keyword-before-value
// grammar buys, and reading args alone would send the plugin no name at all.
func forwardShowRejectASNName(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if name := ctx.Selector(columnName); name != "" {
		args = []string{name}
	}
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowRejectASNName, args, ctx.PeerSelector())
}

func forwardShowRejectASNTransitFree(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return ctx.Dispatcher().ForwardToPlugin(ctx, cmdShowRejectASNTransitFree, args, ctx.PeerSelector())
}
