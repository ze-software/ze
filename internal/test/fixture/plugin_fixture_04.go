package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/bgp-rs-reactor-fastpath-fallback", rsObserver04(1, "10.0.0.0/24", false))
	Register("plugin/bgp-rs-reactor-fastpath", rsObserver04(2, "10.0.0.0/24", true))
	Register("plugin/bgp-local-as-options", rsObserver04(5, "10.0.0.0/24", true))
	Register("plugin/bgp-rs-relay-aspath-transparency", rsObserver04(3, "10.0.0.0/24", true))
	Register("plugin/bgp-rs-replaying-gate", rsObserver04(1, "", true))
	Register("plugin/prefixsid-ebgp-egress-boundary", rsObserver04(3, "10.0.0.0/24", true))
	Register("plugin/bgp-summary-flat-payload", observe04(summaryFlat04))
	Register("plugin/bgp-summary-route-counts", observe04(summaryCounts04))
	Register("plugin/bmp-lg-bestpath-isolation", bmpBestpath04)
	Register("plugin/bmp-lg-disconnect", bmpDisconnect04)
	Register("plugin/bmp-lg-ingest", bmpIngest04)
	Register("plugin/bmp-locrib", observe04(markerObserver04(false, 100, 100*time.Millisecond)))
	Register("plugin/bmp-locrib-collector", bmpCollector04("locrib"))
	Register("plugin/bmp-receiver-messages", bmpMessages04)
	Register("plugin/bmp-receiver-session", bmpReceiverSessionDriver04)
	Register("plugin/bmp-sender-peer-up-open", observe04(markerObserver04(true, 60, 250*time.Millisecond)))
	Register("plugin/bmp-sender-peer-up-open-collector", bmpCollector04("peer-up"))
	Register("plugin/bmp-sender-route-mirroring", observe04(markerObserver04(false, 60, 250*time.Millisecond)))
	Register("plugin/bmp-sender-route-mirroring-collector", bmpCollector04("mirroring"))
	Register("plugin/bmp-sender-route-monitoring", observe04(markerObserver04(false, 60, 250*time.Millisecond)))
	Register("plugin/bmp-sender-route-monitoring-collector", bmpCollector04("monitoring"))
	Register("plugin/bmp-sessions-show", bmpSessions04)
	Register("plugin/capture-interface-show", observe04(captureInterface04))
	Register("plugin/clear-dns-cache", observe04(clearDNS04))
	Register("plugin/cli-commit-reject-plugin", commitRejectPlugin04)
	Register("plugin/cli-commit-reject-driver", cliCommitDriver04(true))
	Register("plugin/cli-commit-transactional", cliCommitDriver04(false))
	Register("plugin/cli-grammar-action-first", observe04(grammarActionFirst04))
	Register("plugin/cli-log-set", observe04(logSet04))
	Register("plugin/cli-log-show", observe04(logShow04))
	Register("plugin/cli-metrics-deep-show", observe04(metricsDeepShow04))
	Register("plugin/cli-metrics-list-deep", observe04(metricsListDeep04))
	Register("plugin/cli-metrics-list", observe04(metricsList04))
	Register("plugin/cli-metrics-plugin-health", observe04(metricsPluginHealth04))
	Register("plugin/cli-metrics-show", observe04(metricsShow04))
	Register("plugin/cli-run-command-peer", observe04(runCommandPeer04))
	Register("plugin/cli-run-command", observe04(runCommand04))
	Register("plugin/cli-summary-show", observe04(summaryShow04))
	Register("plugin/community-attributes-json", communityAttributes04)
}

func observe04(s ObserverScenario) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("unexpected arguments: %v", args)
		}
		return Observe(ctx, "fixture-plugin-04", sdk.Registration{}, s)
	}
}

func command04(ctx context.Context, p *sdk.Plugin, command string) (string, any, error) {
	status, raw, err := p.DispatchCommand(ctx, command)
	if err != nil {
		if status == statusError || strings.HasPrefix(err.Error(), "rpc error:") {
			return statusError, err.Error(), nil
		}
		return status, nil, err
	}
	if len(raw) == 0 {
		return status, nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return status, nil, fmt.Errorf("decode %q: %w", command, err)
	}
	if text, ok := value.(string); ok && json.Valid([]byte(text)) {
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			return status, nil, fmt.Errorf("decode nested %q: %w", command, err)
		}
	}
	return status, value, nil
}

func commandMap04(ctx context.Context, p *sdk.Plugin, command string) (string, map[string]any, error) {
	status, value, err := command04(ctx, p, command)
	if err != nil {
		return status, nil, err
	}
	m, _ := value.(map[string]any)
	return status, m, nil
}

func requireDone04(ctx context.Context, p *sdk.Plugin, command string) (map[string]any, error) {
	status, data, err := commandMap04(ctx, p, command)
	if err != nil {
		return nil, err
	}
	if status != statusDone {
		return nil, fmt.Errorf("%s: status=%s data=%v", command, status, data)
	}
	return data, nil
}

func number04(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func stringSlice04(v any) []string {
	rows, _ := v.([]any)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if s, ok := row.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func mapSlice04(v any) []map[string]any {
	rows, _ := v.([]any)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if m, ok := row.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// peerAddress04 is the address of the only peer these fixtures configure.
const peerAddress04 = "127.0.0.1"

func findPeer04(summary map[string]any) map[string]any {
	for _, peer := range mapSlice04(summary["peers"]) {
		if peer["address"] == peerAddress04 {
			return peer
		}
	}
	return nil
}

func pollCommand04(ctx context.Context, p *sdk.Plugin, attempts int, command string, pred func(string, any) bool) (string, any, error) {
	var status string
	var value any
	var lastErr error
	ok := Poll(ctx, attempts, 200*time.Millisecond, func() bool {
		status, value, lastErr = command04(ctx, p, command)
		return lastErr == nil && pred(status, value)
	})
	if !ok {
		if lastErr != nil {
			return status, value, lastErr
		}
		return status, value, fmt.Errorf("%s did not reach the required state: status=%s data=%v", command, status, value)
	}
	return status, value, nil
}

func waitPeerEOR04(ctx context.Context, p *sdk.Plugin) error {
	_, _, err := pollCommand04(ctx, p, 40, "show bgp", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		peer := findPeer04(data)
		return status == statusDone && peer != nil && number04(peer["eor-sent"]) >= 1
	})
	return err
}

func testBudgetDuration04(fallback time.Duration, share float64) time.Duration {
	for _, key := range []string{envTestBudgetDotted, envTestBudgetLower, envTestBudgetUpper} {
		if raw := os.Getenv(key); raw != "" {
			if budget, err := time.ParseDuration(raw); err == nil {
				return time.Duration(float64(budget) * share)
			}
		}
	}
	return fallback
}
