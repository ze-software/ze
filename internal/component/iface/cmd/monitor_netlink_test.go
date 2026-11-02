// Design: docs/architecture/iface/netlink-monitor.md -- wiring tests for netlink monitor

package cmd

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestNetlinkMonitor_Wiring(t *testing.T) {
	h, args := streamingLookup(t, "monitor system netlink route")
	if h == nil {
		t.Fatal("monitor system netlink not registered as streaming handler")
	}
	if len(args) != 1 || args[0] != "route" {
		t.Errorf("expected args [route], got %v", args)
	}
}

func TestNetlinkMonitorLink_Wiring(t *testing.T) {
	h, args := streamingLookup(t, "monitor system netlink link")
	if h == nil {
		t.Fatal("monitor system netlink not registered as streaming handler")
	}
	if len(args) != 1 || args[0] != "link" {
		t.Errorf("expected args [link], got %v", args)
	}
}

func TestNetlinkMonitorAll_Wiring(t *testing.T) {
	h, _ := streamingLookup(t, "monitor system netlink")
	if h == nil {
		t.Fatal("monitor system netlink not registered as streaming handler")
	}
}

func TestNetlinkMonitorDetectedAsStreaming(t *testing.T) {
	if !pluginserver.IsStreamingCommand("monitor system netlink route") {
		t.Error("monitor system netlink route should be detected as streaming command")
	}
	if !pluginserver.IsStreamingCommand("monitor system netlink") {
		t.Error("monitor system netlink should be detected as streaming command")
	}
}

func TestNetlinkMonitorRPCRegistered(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-iface:monitor-system-netlink" {
			if r.Handler == nil {
				t.Error("ze-iface:monitor-system-netlink handler must not be nil")
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ze-iface:monitor-system-netlink not registered via pluginserver.RegisterRPCs")
	}
}

func TestNetlinkMonitorInvalidGroup(t *testing.T) {
	// The model declares no leaf for this command, so the group word reaches
	// the handler unjudged and the handler's own check is what refuses it.
	args, err := command.ValidateArgs([]string{"bogus"}, nil, nil)
	if err != nil {
		t.Fatalf("ValidateArgs with no definitions: %v", err)
	}
	err = streamNetlinkMonitor(context.TODO(), nil, nil, "", args)
	if err == nil {
		t.Fatal("expected error for invalid group")
	}
	if err.Error() != "unknown netlink group (valid: route, link, address, all)" {
		t.Errorf("unexpected error: %v", err)
	}
}

// streamingLookup is GetStreamingHandlerForCommand for a test that expects the
// arguments to be accepted: it fails the test on a refusal and answers the
// judged tokens.
func streamingLookup(t *testing.T, input string) (pluginserver.StreamingHandler, []string) {
	t.Helper()
	handler, validated, err := pluginserver.GetStreamingHandlerForCommand(input)
	if err != nil {
		t.Fatalf("GetStreamingHandlerForCommand(%q): %v", input, err)
	}
	return handler, validated.Tokens()
}

// TestNetlinkMonitorStreamGroup proves the group word of `monitor system
// netlink` reaches the group the streaming handler subscribes to. The plugin
// fixture monitor-system-netlink proved this through the one-shot echo
// (`{"status":"monitor-configured","group":"route"}`); a streaming-only
// command is refused on a one-shot route, so the proof lives here. Method:
// take the arguments the streaming executor extracts from the full command
// line (streamingLookup), and parse them with netlinkGroupFromArgs, the parser
// streamNetlinkMonitor runs on them.
func TestNetlinkMonitorStreamGroup(t *testing.T) {
	tests := []struct {
		line  string
		group string
	}{
		{"monitor system netlink", netlinkGroupAll},
		{"monitor system netlink route", netlinkGroupRoute},
		{"monitor system netlink LINK", netlinkGroupLink},
		{"monitor system netlink address", netlinkGroupAddress},
	}
	for _, tt := range tests {
		_, args := streamingLookup(t, tt.line)
		group, err := netlinkGroupFromArgs(args)
		if err != nil {
			t.Errorf("%q: %v", tt.line, err)
			continue
		}
		if group != tt.group {
			t.Errorf("%q: group %q, want %q", tt.line, group, tt.group)
		}
	}
}
