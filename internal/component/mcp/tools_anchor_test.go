package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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
// which validateCommandArgs accepts and Dispatch still refuses with "requires
// a selector", because only matchCommandTokens binds an anchored value and it
// reads the bare token after the anchor (anchoredDef). A stub dispatcher
// cannot see that refusal, so this test runs the dispatcher itself.
func TestDispatchGeneratedBindsAnAnchoredValueThroughTheDispatcher(t *testing.T) {
	const name = "peer announce unicast"
	selector := command.ArgDef{Name: "selector", Kind: command.ArgString, Mandatory: true, Anchor: "peer"}
	prefix := command.ArgDef{Name: "prefix", Kind: command.ArgString, Mandatory: true}

	d := pluginserver.NewDispatcher()
	var gotSelector string
	var gotArgs []string
	d.RegisterWithOptions(name, func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
		gotSelector = ctx.PeerSelector()
		gotArgs = args
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"announced": "1"}), nil
	}, "Announce a unicast prefix", pluginserver.RegisterOptions{
		RequiresSelector: true,
		ArgDefs:          []command.ArgDef{selector, prefix},
	})

	var gotInput string
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
			gotInput = input
			return d.Dispatch(&pluginserver.CommandContext{}, input)
		},
		commands: func() []CommandInfo {
			return []CommandInfo{{Name: name, Params: []ParamInfo{
				{Name: "selector", Type: "string", Required: true, Anchor: "peer"},
				{Name: "prefix", Type: "string", Required: true},
			}}}
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
		commands: func() []CommandInfo {
			return []CommandInfo{{Name: name, Params: []ParamInfo{{Name: "selector", Type: "string", Anchor: "peer"}}}}
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
