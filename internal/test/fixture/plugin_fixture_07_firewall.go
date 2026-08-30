package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func firewallGlobalOptions07(ctx context.Context, p *sdk.Plugin) error {
	expected := map[string]string{
		"net.ipv4.icmp_echo_ignore_all": "0",
		"net.ipv4.tcp_syncookies":       "0",
		"net.ipv4.conf.all.rp_filter":   "1",
	}
	result := until07(ctx, p, "show sysctl", 40, 250*time.Millisecond, func(result commandResult07) bool {
		have := map[string]map[string]any{}
		for _, row := range array07(result.data) {
			entry := object07(row)
			if key, ok := entry["key"].(string); ok {
				have[key] = entry
			}
		}
		for key, value := range expected {
			if have[key]["value"] != value {
				return false
			}
		}
		return result.status == statusDone
	})
	if result.status != statusDone {
		return fmt.Errorf("show sysctl status=%s error=%w", result.status, result.err)
	}
	byKey := map[string]map[string]any{}
	for _, row := range array07(result.data) {
		entry := object07(row)
		if key, ok := entry["key"].(string); ok {
			byKey[key] = entry
		}
	}
	for key, value := range expected {
		entry := byKey[key]
		if entry == nil || entry["value"] != value || entry["source"] != "firewall:global-options" {
			return fmt.Errorf("sysctl %s = %s, want value=%s source=firewall:global-options", key, text07(entry), value)
		}
	}
	return waitEOR07(ctx, p, 1)
}

func firewallMetrics07(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("firewall metrics fixture requires the telemetry port")
	}
	pid, err := waitDaemon07(ctx)
	if err != nil {
		return err
	}
	var body string
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool { body, err = fetch07(ctx, args[0]); return err == nil }) {
		return fmt.Errorf("the Prometheus endpoint on 127.0.0.1:%s never answered", args[0])
	}
	for _, family := range []string{"go_goroutines", "process_start_time_seconds"} {
		if !strings.Contains(body, family) {
			return fmt.Errorf("serving registry missing runtime family %s", family)
		}
	}
	wanted := []string{"ze_firewall_apply_timeout_total", "ze_firewall_apply_duration_seconds"}
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		body, err = fetch07(ctx, args[0])
		return err == nil && strings.Contains(body, wanted[0]) && strings.Contains(body, wanted[1])
	}) {
		return fmt.Errorf("missing firewall metrics from exposed registry")
	}
	fmt.Fprintln(os.Stdout, "firewall metrics exposed") //nolint:errcheck // progress output
	return terminate07(pid)
}
