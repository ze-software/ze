package fixture

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPluginFixture08RegistersEveryDriver(t *testing.T) {
	for _, name := range []string{
		"plugin/flowspec-metrics-registered",
		"plugin/forked-route-install-kernel",
		"plugin/forked-route-install",
		"plugin/forward-backpressure",
		"plugin/forward-congestion-overflow-metrics",
		"plugin/forward-congestion-teardown-metrics",
		"plugin/forward-mpreach-nexthop-self-two-peer",
		"plugin/forward-overflow-two-tier",
		"plugin/forward-two-tier-under-load",
		"plugin/forward-write-deadline",
		"plugin/geodns-dot-pki",
		"plugin/geodns-show",
		"plugin/gnmi-show",
		"plugin/gr-cli-restart",
		"plugin/grpc-execute",
		"plugin/health-components-show",
		"plugin/hub-external-plugin-acceptor",
		"plugin/iface-bridge-mac-match-apply",
		"plugin/iface-bridge-member-selector-apply",
		"plugin/iface-ensure-rollback",
		"plugin/iface-kernel-read-dispatch",
		"plugin/iface-learned-route-metric",
		"plugin/iface-link-flap-during-commit",
		"plugin/iface-mac-match-address-apply",
		"plugin/iface-osname-alias-apply",
		"plugin/iface-rate-json",
		"plugin/iface-route-protocol-name",
		"plugin/iface-tunnel-kinds",
		"plugin/iface-tunnel-restart-boot",
		"plugin/iface-tunnel-restart-wait",
		"plugin/iface-tunnel-restart-check",
		"plugin/iface-verbs",
		"plugin/interface-errors-show",
		"plugin/interface-rate-show",
	} {
		driversMu.RLock()
		driver := drivers[name]
		driversMu.RUnlock()
		if driver == nil {
			t.Errorf("fixture %q is not registered", name)
		}
	}
}

func TestIfaceTunnelRestartWaitReportsRecordedIndex(t *testing.T) {
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previousDir)
	if err := os.WriteFile("restart-ifindex", []byte("42"), 0644); err != nil {
		t.Fatal(err)
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previousStdout := os.Stdout
	os.Stdout = writer
	runErr := ifaceTunnelRestartWait08(context.Background(), nil)
	_ = writer.Close()
	os.Stdout = previousStdout
	output, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if runErr != nil || readErr != nil {
		t.Fatalf("run error=%v read error=%v", runErr, readErr)
	}
	if got := string(output); !strings.Contains(got, "OK: tunnel restart pass 1 recorded ifindex 42") {
		t.Fatalf("stdout=%q", got)
	}
}
