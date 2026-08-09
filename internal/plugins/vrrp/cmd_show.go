// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- show/clear command surface
//
// Three halves make a command work, and all three live in this plugin so the
// removal test holds: the YANG tree (yang/ze-vrrp-cmd.yang) declares the path
// and binds a wire method; the proxies below register those wire methods with
// the daemon and forward to this plugin's process; and the engine answers via
// OnExecuteCommand.
//
// The proxy MUST forward, never re-dispatch: dispatching the same command string
// would route it straight back to this handler and recurse until the stack dies
// (the isis contract, internal/plugins/isis/cmd_show.go:106-116).
package vrrp

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// errNoInterfaceSelector is returned when the interface view is asked for
// without a name. The grammar requires `name <name>`, so this is a programmatic
// caller's error, not an operator typo (the CLI would not dispatch without it).
var errNoInterfaceSelector = errors.New("vrrp: show vrrp interface requires a selector: show vrrp interface name <interface>")

// errUnknownCommand names a command this plugin declared but does not answer,
// which can only mean the CommandDecl list and this switch drifted apart.
func errUnknownCommand(command string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("vrrp: unknown command ").Quoted(command).String())
}

// Command paths. These strings are the contract between the YANG `ze:command`
// nodes, the proxy registrations, and the engine's dispatch switch.
const (
	cmdShowVRRP           = "show vrrp"
	cmdShowVRRPInterface  = "show vrrp interface"
	cmdShowVRRPStatistics = "show vrrp statistics"
	cmdClearVRRPStats     = "clear vrrp statistics"
)

// Wire methods, kebab-case per ai/patterns/cli-command.md.
const (
	wireShowVRRP           = "ze-show:vrrp"
	wireShowVRRPInterface  = "ze-show:vrrp-interface"
	wireShowVRRPStatistics = "ze-show:vrrp-statistics"
	wireClearVRRPStats     = "ze-clear:vrrp-statistics"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireShowVRRP, Handler: forwardNoArgs(cmdShowVRRP), PluginCommand: cmdShowVRRP},
		pluginserver.RPCRegistration{WireMethod: wireShowVRRPInterface, Handler: forwardWithArgs(cmdShowVRRPInterface), PluginCommand: cmdShowVRRPInterface},
		pluginserver.RPCRegistration{WireMethod: wireShowVRRPStatistics, Handler: forwardNoArgs(cmdShowVRRPStatistics), PluginCommand: cmdShowVRRPStatistics},
		pluginserver.RPCRegistration{WireMethod: wireClearVRRPStats, Handler: forwardNoArgs(cmdClearVRRPStats), PluginCommand: cmdClearVRRPStats},
	)
}

// forwardNoArgs proxies a command that takes no arguments (the nouns are baked
// into the command string by the grammar), rejecting extras rather than
// silently ignoring them.
func forwardNoArgs(command string) pluginserver.Handler {
	return func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		if len(args) > 0 {
			var tb textbuf.Buffer
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  tb.Str("unexpected argument; ").Str(command).Str(" takes none").String(),
			}, nil
		}
		return forward(ctx, command, nil)
	}
}

// forwardWithArgs proxies a command that carries a selector value
// (`show vrrp interface name <name>`).
func forwardWithArgs(command string) pluginserver.Handler {
	return func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		return forward(ctx, command, args)
	}
}

// forward hands the command to this plugin's process.
//
// ForwardToPlugin, never Dispatch: dispatching the same command string would
// route it back to this handler and recurse (the isis contract,
// internal/plugins/isis/cmd_show.go:106-116).
func forward(ctx *pluginserver.CommandContext, command string, args []string) (*plugin.Response, error) {
	d := ctx.Dispatcher()
	if d == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "dispatcher unavailable"}, nil
	}
	return d.ForwardToPlugin(ctx, command, args, ctx.PeerSelector())
}

// commandDecls are the commands this plugin answers (SDK Stage 1).
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: cmdShowVRRP},
		{Name: cmdShowVRRPInterface},
		{Name: cmdShowVRRPStatistics},
		{Name: cmdClearVRRPStats},
	}
}

// handleCommand answers one command from the engine side.
func handleCommand(eng *engine, command string, args []string) (string, any, error) {
	switch command {
	case cmdShowVRRP:
		return rpc.StatusDone, map[string]any{"vrrp": eng.snapshots()}, nil

	case cmdShowVRRPInterface:
		name := selectorValue(args)
		if name == "" {
			return rpc.StatusError, nil, errNoInterfaceSelector
		}
		return rpc.StatusDone, map[string]any{"vrrp": eng.snapshotsForInterface(name)}, nil

	case cmdShowVRRPStatistics:
		return rpc.StatusDone, map[string]any{"vrrp-statistics": eng.statistics()}, nil

	case cmdClearVRRPStats:
		cleared := eng.clearStatistics()
		return rpc.StatusDone, map[string]any{"cleared": cleared}, nil

	default:
		return rpc.StatusError, nil, errUnknownCommand(command)
	}
}

// selectorValue extracts the value after a `name` selector keyword.
//
// The grammar is `show vrrp interface name <name>` (ai/rules/cli.md:
// a typed selector keyword before any free-form value), so the value is
// whatever follows "name".
func selectorValue(args []string) string {
	for i, a := range args {
		if a == selectorKeyword {
			if i+1 < len(args) {
				return args[i+1]
			}
			// `... name` with nothing after it is an incomplete command, NOT a
			// request for an interface called "name". Falling through to the
			// bare-value branch would resolve exactly that ambiguity the wrong
			// way, which is what the typed-selector grammar exists to prevent.
			return ""
		}
	}
	// Tolerate a bare value for programmatic senders that already stripped the
	// selector keyword.
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		return args[0]
	}
	return ""
}

// selectorKeyword types the interface selector (ai/rules/cli.md).
const selectorKeyword = "name"
