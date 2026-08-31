// VALIDATES: `withdraw` is three commands, one per form, each with its own wire
// method and its own generated usage line.
// PREVENTS: four grammars behind one handler, which no model can state, no
// completion can offer, and no generated usage line can render.

package announce

import (
	"sort"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/announce/yang"
	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// withdrawTree builds a command tree over the announce module alone, so the
// assertions below read the module a reviewer can open rather than whatever the
// rest of the binary happened to register.
func withdrawTree(t *testing.T) *command.Node {
	t.Helper()
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load the embedded modules: %v", err)
	}
	if err := loader.AddModuleFromText("ze-cli-announce-cmd", yang.ZeCliAnnounceCmdYANG); err != nil {
		t.Fatalf("load the announce module: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve the announce module: %v", err)
	}
	return configyang.BuildCommandTree(loader)
}

// TestWithdrawFormsAreSeparateCommands proves the split reached both halves:
// the model declares three commands and the plugin registers three handlers.
//
// The negative half is the point. `handleWithdraw` parsed four tail grammars
// behind one wire method, and keeping it beside the three would leave the model
// stating one thing and the daemon accepting another (ai/rules/no-layering.md).
func TestWithdrawFormsAreSeparateCommands(t *testing.T) {
	registered := map[string]bool{}
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		registered[reg.WireMethod] = true
	}
	if registered["ze-bgp:withdraw"] {
		t.Error("ze-bgp:withdraw is still registered beside the three forms that replaced it")
	}
	for _, method := range []string{"ze-bgp:withdraw-tag", "ze-bgp:withdraw-id", "ze-bgp:withdraw-all"} {
		if !registered[method] {
			t.Errorf("%s is not registered, so a command the model declares reaches no handler", method)
		}
	}

	root := withdrawTree(t)
	withdraw := root.Children["withdraw"]
	if withdraw == nil {
		t.Fatal("the module declares no withdraw container")
	}
	if withdraw.WireMethod != "" {
		t.Errorf("withdraw still carries the wire method %q; it is a grouping node now", withdraw.WireMethod)
	}

	names := make([]string, 0, len(withdraw.Children))
	for name := range withdraw.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) != 3 || names[0] != "all" || names[1] != "id" || names[2] != "tag" {
		t.Fatalf("withdraw lists %v as subcommands, want [all id tag]", names)
	}
}

// TestWithdrawFormsRenderTheirOwnUsage proves each form states its grammar in
// the model, at both paths it is reachable at.
//
// The tag value is OPTIONAL, which the authored prose got wrong: handleWithdrawTag
// defaults it to "*". `withdraw all` takes no tail at all. It carried a
// `selector <pattern>` leaf until 2026-08-31, when the peer prefix replaced it
// so that one scope reaches all three forms rather than one.
func TestWithdrawFormsRenderTheirOwnUsage(t *testing.T) {
	root := withdrawTree(t)
	for _, tc := range []struct {
		path []string
		want string
	}{
		{path: []string{"withdraw", "tag"}, want: "withdraw tag <key> [value <value>]"},
		{path: []string{"withdraw", "id"}, want: "withdraw id <id>"},
		{path: []string{"withdraw", "all"}, want: "withdraw all"},
		{path: []string{"peer", "withdraw", "tag"}, want: "peer <selector> withdraw tag <key> [value <value>]"},
		{path: []string{"peer", "withdraw", "id"}, want: "peer <selector> withdraw id <id>"},
		{path: []string{"peer", "withdraw", "all"}, want: "peer <selector> withdraw all"},
	} {
		node := command.FindNode(root, tc.path)
		if node == nil {
			t.Errorf("the model declares no %v command", tc.path)
			continue
		}
		if got := command.UsageLine(command.Usage(tc.path, node)); got != tc.want {
			t.Errorf("%v renders %q, want %q", tc.path, got, tc.want)
		}
	}
}
