// Design: docs/architecture/iface/netlink-monitor.md -- wiring tests for netlink monitor

package cmd

import (
	"context"
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestNetlinkMonitor_Wiring(t *testing.T) {
	h, args := pluginserver.GetStreamingHandlerForCommand("monitor system netlink route")
	if h == nil {
		t.Fatal("monitor system netlink not registered as streaming handler")
	}
	if len(args) != 1 || args[0] != "route" {
		t.Errorf("expected args [route], got %v", args)
	}
}

func TestNetlinkMonitorLink_Wiring(t *testing.T) {
	h, args := pluginserver.GetStreamingHandlerForCommand("monitor system netlink link")
	if h == nil {
		t.Fatal("monitor system netlink not registered as streaming handler")
	}
	if len(args) != 1 || args[0] != "link" {
		t.Errorf("expected args [link], got %v", args)
	}
}

func TestNetlinkMonitorAll_Wiring(t *testing.T) {
	h, _ := pluginserver.GetStreamingHandlerForCommand("monitor system netlink")
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
		if r.WireMethod == "ze-monitor:system-netlink" {
			if r.Handler == nil {
				t.Error("ze-monitor:system-netlink handler must not be nil")
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ze-monitor:system-netlink not registered via pluginserver.RegisterRPCs")
	}
}

func TestNetlinkMonitorInvalidGroup(t *testing.T) {
	err := streamNetlinkMonitor(context.TODO(), nil, nil, "", []string{"bogus"})
	if err == nil {
		t.Fatal("expected error for invalid group")
	}
	if err.Error() != "unknown netlink group (valid: route, link, address, all)" {
		t.Errorf("unexpected error: %v", err)
	}
}
