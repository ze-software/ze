package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestDispatchGeneratedBindsAnAnchoredValueThroughTheDispatcher: a tool call
// carrying the value of an argument a container ABOVE the command declares
// reaches the REAL dispatcher where it binds it, bare after the anchor keyword,
// so a command that requires a selector runs with the one the client supplied.
//
// The shape is `peer <selector> announce unicast <prefix>`, the peer-scoped
// announce path: the selector is anchored to `peer` and the prefix follows the
// command. The lister carries the anchor the hub reads from the registered
// command (anchoredParams, cmd/ze/hub).
//
// VALIDATES: the call dispatches `peer 192.0.2.9 announce unicast prefix
// 198.51.100.0/24`, and the handler sees the selector.
// PREVENTS: the keyword form `peer announce unicast selector 192.0.2.9 ...`,
// which command.ValidateArgs accepts and Dispatch still refuses with "requires
// a selector", because only matchCommandTokens binds an anchored value and it
// reads the bare token after the anchor (anchoredDef). A stub dispatcher
// cannot see that refusal, so this test runs the dispatcher itself.
func TestDispatchGeneratedBindsAnAnchoredValueThroughTheDispatcher(t *testing.T) {
	const name = "peer announce unicast"
	selector := commandtest.Must(command.NewStringArg("selector", nil, nil, command.ArgOptions{Mandatory: true, Anchor: "peer"}))
	prefix := commandtest.Must(command.NewStringArg("prefix", nil, nil, command.ArgOptions{Mandatory: true}))

	d := pluginserver.NewDispatcher()
	var gotSelector string
	var gotArgs []string
	if err := d.RegisterWithOptions(name, func(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		args := validated.Tokens()
		gotSelector = ctx.PeerSelector()
		gotArgs = args
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"announced": "1"}), nil
	}, "Announce a unicast prefix", pluginserver.RegisterOptions{
		RequiresSelector: true,
		ArgDefs:          []command.ArgDef{selector, prefix},
	}); err != nil {
		t.Fatal(err)
	}

	var gotInput string
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
			gotInput = input
			return d.Dispatch(&pluginserver.CommandContext{}, input)
		},
		commands: func() ([]CommandInfo, error) {
			return []CommandInfo{{Name: name, Params: []ParamInfo{
				{Name: "selector", Type: "string", Required: true, Anchor: "peer"},
				{Name: "prefix", Type: "string", Required: true},
			}}}, nil
		},
	}

	args, err := json.Marshal(map[string]string{"action": "unicast", "selector": "192.0.2.9", "prefix": "198.51.100.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	result := s.dispatchGenerated("peer announce", map[string]bool{"unicast": false}, args)

	if _, isErr := result["isError"]; isErr {
		t.Fatalf("the call was refused: %v (dispatched %q)", result, gotInput)
	}
	if gotInput != "peer 192.0.2.9 announce unicast prefix 198.51.100.0/24" {
		t.Errorf("dispatched %q, want the selector bare after peer and the prefix as keyword and value", gotInput)
	}
	if gotSelector != "192.0.2.9" {
		t.Errorf("selector = %q, want the one the client supplied", gotSelector)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "prefix" || gotArgs[1] != "198.51.100.0/24" {
		t.Errorf("handler args = %v, want the prefix pair", gotArgs)
	}
}

// TestDispatchGeneratedRefusesPeerBesideAnAnchoredSelector: the reserved
// `peer` argument and a typed parameter anchored to `peer` name one slot, so a
// call carrying both is refused by name rather than dispatched with two values
// in it.
//
// VALIDATES: nothing is dispatched and the refusal names the parameter.
// PREVENTS: `peer 10.0.0.1 192.0.2.9 announce unicast`, which the dispatcher
// reads as an unknown command.
func TestDispatchGeneratedRefusesPeerBesideAnAnchoredSelector(t *testing.T) {
	const name = "peer announce unicast"
	dispatched := false
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			dispatched = true
			return plugin.NewResponse(plugin.StatusDone, nil), nil
		},
		commands: func() ([]CommandInfo, error) {
			return []CommandInfo{{Name: name, Params: []ParamInfo{{Name: "selector", Type: "string", Anchor: "peer"}}}}, nil
		},
	}
	args, err := json.Marshal(map[string]string{"action": "unicast", "peer": "10.0.0.1", "selector": "192.0.2.9"})
	if err != nil {
		t.Fatal(err)
	}
	result := s.dispatchGenerated("peer announce", map[string]bool{"unicast": true}, args)
	if _, isErr := result["isError"]; !isErr {
		t.Errorf("expected a refusal, got %v", result)
	}
	if dispatched {
		t.Error("a call naming the peer twice must not be dispatched")
	}
}

// TestWriteInvocationPlacesAnchoredValues: the MCP server hands
// command.WriteInvocation only the name and the anchor its lister registered
// (D-7), and the command it writes is the one the dispatcher binds.
//
// VALIDATES: invocationArgs projects each ParamInfo to its name and anchor, and
// WriteInvocation over that input writes the anchored value bare after its
// keyword and every other value after the command in keyword form.
// PREVENTS: an untyped argument definition built only to carry two strings,
// which would claim the argument accepts any value.
func TestWriteInvocationPlacesAnchoredValues(t *testing.T) {
	const name = "peer announce unicast"
	s := &server{commands: func() ([]CommandInfo, error) {
		return []CommandInfo{{Name: name, Params: []ParamInfo{
			{Name: "selector", Type: "string", Required: true, Anchor: "peer"},
			{Name: "prefix", Type: "string", Required: true},
		}}}, nil
	}}
	args, err := s.invocationArgs(name)
	if err != nil {
		t.Fatal(err)
	}
	want := []command.InvocationArg{{Name: "selector", Anchor: "peer"}, {Name: "prefix"}}
	if len(args) != len(want) || args[0] != want[0] || args[1] != want[1] {
		t.Fatalf("invocationArgs = %+v, want %+v", args, want)
	}
	var tb textbuf.Buffer
	values := map[string]string{"selector": "192.0.2.9", "prefix": "198.51.100.0/24"}
	if err := command.WriteInvocation(&tb, []string{"peer", "announce", "unicast"}, args, values); err != nil {
		t.Fatal(err)
	}
	if got, wantCmd := tb.String(), "peer 192.0.2.9 announce unicast prefix 198.51.100.0/24"; got != wantCmd {
		t.Fatalf("command = %q, want %q", got, wantCmd)
	}
}
