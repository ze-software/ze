package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type plugin08Scenario = ObserverScenario

func registerPlugin08(name, pluginName string, scenario plugin08Scenario) {
	Register(name, func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("%s: unexpected arguments: %v", name, args)
		}
		return Observe(ctx, pluginName, sdk.Registration{}, scenario)
	})
}

func command08(ctx context.Context, p *sdk.Plugin, command string) (string, json.RawMessage, error) {
	var raw json.RawMessage
	status, err := Dispatch(ctx, p, command, &raw)
	return status, raw, err
}

func requireDone08(ctx context.Context, p *sdk.Plugin, command string) (json.RawMessage, error) {
	status, raw, err := command08(ctx, p, command)
	if err != nil {
		return raw, fmt.Errorf("%s: %w", command, err)
	}
	if status != statusDone {
		return raw, fmt.Errorf("%s: status=%s data=%s", command, status, raw)
	}
	return raw, nil
}

func decode08(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty response")
	}
	if err := json.Unmarshal(raw, dst); err == nil {
		return nil
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return err
	}
	return json.Unmarshal([]byte(encoded), dst)
}

func object08(raw json.RawMessage) (map[string]any, error) {
	var out map[string]any
	if err := decode08(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func list08(raw json.RawMessage) ([]map[string]any, error) {
	var out []map[string]any
	if err := decode08(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func number08(v any) float64 {
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
	case uint64:
		return float64(n)
	default:
		return 0
	}
}

func peerCounter08(ctx context.Context, p *sdk.Plugin, peer, counter string) (float64, bool) {
	status, raw, err := command08(ctx, p, "show bgp peer "+peer+" detail")
	if err != nil || status != statusDone {
		return 0, false
	}
	obj, err := object08(raw)
	if err != nil {
		return 0, false
	}
	peers, ok := obj["peers"].(map[string]any)
	if !ok {
		return 0, false
	}
	for _, value := range peers {
		row, ok := value.(map[string]any)
		if ok {
			return number08(row[counter]), true
		}
	}
	return 0, false
}

func waitPeerCounter08(ctx context.Context, p *sdk.Plugin, peer, counter string, minimum float64, attempts int) bool {
	return Poll(ctx, attempts, 250*time.Millisecond, func() bool {
		value, ok := peerCounter08(ctx, p, peer, counter)
		return ok && value >= minimum
	})
}

func waitPeerEOR08(ctx context.Context, p *sdk.Plugin) error {
	if !waitPeerCounter08(ctx, p, "peer1", "eor-sent", 1, 40) {
		return fmt.Errorf("peer1 initial-sync EOR never reached the wire")
	}
	return nil
}

func metrics08(ctx context.Context, p *sdk.Plugin) (string, error) {
	raw, err := requireDone08(ctx, p, "show metrics values")
	if err != nil {
		return "", err
	}
	obj, err := object08(raw)
	if err != nil {
		return "", fmt.Errorf("show metrics values: %w", err)
	}
	text, _ := obj["metrics"].(string)
	if text == "" {
		return "", fmt.Errorf("show metrics values returned empty metrics")
	}
	return text, nil
}

func metricsScenario08(required ...string) plugin08Scenario {
	return func(ctx context.Context, p *sdk.Plugin) error {
		if err := waitPeerEOR08(ctx, p); err != nil {
			return err
		}
		text, err := metrics08(ctx, p)
		if err != nil {
			return err
		}
		var missing []string
		for _, name := range required {
			if !strings.Contains(text, name) {
				missing = append(missing, name)
			}
		}
		if len(missing) != 0 {
			return fmt.Errorf("missing metrics %v in %d-byte output", missing, len(text))
		}
		fmt.Fprintf(os.Stderr, "OK: metrics present in %d bytes\n", len(text))
		return nil
	}
}

func init() {
	registerPlugin08("plugin/forward-backpressure", "backpressure-test", forwardBackpressure08)
	registerPlugin08("plugin/forward-congestion-overflow-metrics", "overflow-metrics-test", metricsScenario08("ze_bgp_pool_used_ratio", "ze_forward_workers_active"))
	registerPlugin08("plugin/forward-congestion-teardown-metrics", "congestion-teardown-test", forwardCongestionTeardown08)
	registerPlugin08("plugin/forward-mpreach-nexthop-self-two-peer", "mpreach-nexthop-test", forwardMPReach08)
	registerPlugin08("plugin/forward-overflow-two-tier", "overflow-test", forwardLoad08(50, []string{metricPoolUsedRatio}, "overflow"))
	registerPlugin08("plugin/forward-two-tier-under-load", "two-tier-load-test", forwardLoad08(80, []string{metricPoolUsedRatio, "ze_forward_workers_active"}, "two-tier"))
	registerPlugin08("plugin/forward-write-deadline", "deadline-test", forwardDeadline08)
	registerPlugin08("plugin/geodns-dot-pki", "geodns-dot-pki-test", geodnsDotPKI08)
	registerPlugin08("plugin/geodns-show", "geodns-show-test", geodnsShow08)
	registerPlugin08("plugin/gnmi-show", "gnmi-show-test", gnmiShow08)
	registerPlugin08("plugin/gr-cli-restart", "gr-cli-test", grCLIRestart08)
	registerPlugin08("plugin/grpc-execute", "grpc-test", grpcExecute08)
	registerPlugin08("plugin/health-components-show", "health-show-test", healthShow08)
	registerPlugin08("plugin/iface-kernel-read-dispatch", "iface-kernel-read-test", ifaceKernelRead08)
	registerPlugin08("plugin/iface-rate-json", "iface-rate-json-test", ifaceRateJSON08)
	registerPlugin08("plugin/interface-rate-show", "interface-rate-show-test", interfaceRateShow08)
}

func forwardBackpressure08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	for _, event := range []string{"congested", "resumed"} {
		raw, err := requireDone08(ctx, p, "request subscribe bgp event "+event)
		if err != nil {
			return err
		}
		obj, err := object08(raw)
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", event, err)
		}
		if obj["namespace"] != namespaceBGP || obj["event"] != event {
			return fmt.Errorf("subscribe %s: expected bgp/%s, got %v/%v", event, event, obj["namespace"], obj["event"])
		}
		fmt.Fprintf(os.Stderr, "OK: subscribed to bgp event %s\n", event)
	}
	fmt.Fprintln(os.Stderr, "OK: both congestion events subscribable")
	return nil
}

func forwardCongestionTeardown08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	if _, err := requireDone08(ctx, p, "request quiesce"); err != nil {
		return err
	}
	var text string
	if !Poll(ctx, 10, 500*time.Millisecond, func() bool {
		var err error
		text, err = metrics08(ctx, p)
		return err == nil && text != ""
	}) {
		return fmt.Errorf("empty metrics after poll")
	}
	for _, name := range []string{"ze_forward_buffer_denied_total", "ze_forward_congestion_teardown_total", metricPoolUsedRatio} {
		if !strings.Contains(text, name) {
			return fmt.Errorf("missing congestion metric %s", name)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: congestion teardown metrics present")
	return nil
}

func forwardMPReach08(ctx context.Context, p *sdk.Plugin) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
	}
	if _, err := requireDone08(ctx, p, "request fakel2tp emit add ipv4/multicast 224.1.1.0/24"); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Second):
		return nil
	}
}

func forwardLoad08(expected int, required []string, label string) plugin08Scenario {
	return func(ctx context.Context, p *sdk.Plugin) error {
		var total float64
		if !Poll(ctx, 80, 250*time.Millisecond, func() bool {
			status, raw, err := command08(ctx, p, "show bgp adj-rib-in status")
			if err != nil || status != statusDone {
				return false
			}
			obj, err := object08(raw)
			if err != nil {
				return false
			}
			total = number08(obj["total-routes"])
			return total >= float64(expected)
		}) {
			return fmt.Errorf("adj-rib-in has %.0f/%d routes", total, expected)
		}
		fmt.Fprintf(os.Stderr, "OK: %.0f routes in adj-rib-in\n", total)
		if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
			status, _, err := command08(ctx, p, "request peer * flush")
			return err == nil && status == statusDone
		}) {
			return fmt.Errorf("forward-pool flush did not complete")
		}
		fmt.Fprintln(os.Stderr, "OK: forward pool flushed to all peers")
		text, err := metrics08(ctx, p)
		if err != nil {
			return err
		}
		for _, name := range required {
			if !strings.Contains(text, name) {
				return fmt.Errorf("%s metric missing: %s", label, name)
			}
		}
		fmt.Fprintf(os.Stderr, "OK: %s metrics present\n", label)
		return nil
	}
}

func forwardDeadline08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return fmt.Errorf("ze did not send the End-of-RIB to the peer: %w", err)
	}
	status, _, err := command08(ctx, p, "peer * update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24")
	if err != nil && status != statusError {
		return err
	}
	fmt.Fprintf(os.Stderr, "update status=%s\n", status)
	if status != statusDone && status != statusError {
		return fmt.Errorf("unexpected update status=%s", status)
	}
	if !waitPeerCounter08(ctx, p, "peer1", "updates-sent", 2, 40) {
		return fmt.Errorf("the announced route never reached the peer")
	}
	fmt.Fprintln(os.Stderr, "OK: forward path operational with write deadline")
	return nil
}

func geodnsDotPKI08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		dialer := net.Dialer{Timeout: time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:18855")
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}) {
		return fmt.Errorf("dot: listener not bound on 127.0.0.1:18855 (pki certificate reference did not resolve)")
	}
	fmt.Fprintln(os.Stderr, "OK: geodns dot bound with pki certificate")
	return nil
}

func geodnsShow08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	raw, err := requireDone08(ctx, p, "show geodns")
	if err != nil {
		return err
	}
	obj, err := object08(raw)
	if err != nil {
		return fmt.Errorf("geodns: expected object: %w", err)
	}
	if obj["enabled"] != true {
		return fmt.Errorf("geodns: expected enabled=true, got %v", obj["enabled"])
	}
	if _, ok := obj["listeners"]; !ok {
		return fmt.Errorf("geodns: missing listeners in %v", obj)
	}
	fmt.Fprintf(os.Stderr, "OK: geodns enabled=true listeners=%v\n", obj["listeners"])
	return nil
}

func gnmiShow08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	raw, err := requireDone08(ctx, p, "show gnmi")
	if err != nil {
		return err
	}
	obj, err := object08(raw)
	if err != nil {
		return fmt.Errorf("gnmi: expected object: %w", err)
	}
	if _, ok := obj["enabled"]; !ok {
		return fmt.Errorf("gnmi: missing enabled in %v", obj)
	}
	fmt.Fprintf(os.Stderr, "OK: show gnmi enabled=%v\n", obj["enabled"])
	return nil
}

func grCLIRestart08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	if _, err := requireDone08(ctx, p, "request shutdown"); err != nil && !strings.Contains(err.Error(), "EOF") {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: lifecycle dispatch works")
	return nil
}

func grpcExecute08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	if _, err := requireDone08(ctx, p, "show version"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: gRPC-path execute returned status=done")
	raw, err := requireDone08(ctx, p, "show bgp")
	if err != nil {
		return err
	}
	obj, err := object08(raw)
	if err != nil {
		return err
	}
	if _, ok := obj["peers-configured"]; !ok {
		return fmt.Errorf("show bgp has no peers-configured: %v", obj)
	}
	peers, _ := obj["peers"].([]any)
	found := false
	for _, value := range peers {
		row, _ := value.(map[string]any)
		if row["address"] == addrLoopback {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("show bgp does not list peer 127.0.0.1: %v", obj)
	}
	fmt.Fprintln(os.Stderr, "OK: gRPC-path summary returned peer data")
	return nil
}

func healthShow08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	raw, err := requireDone08(ctx, p, "show health")
	if err != nil {
		return err
	}
	obj, err := object08(raw)
	if err != nil {
		return err
	}
	components, _ := obj["components"].([]any)
	names := make(map[string]bool)
	for _, value := range components {
		row, _ := value.(map[string]any)
		if name, ok := row["name"].(string); ok {
			names[name] = true
		}
	}
	for _, required := range []string{namespaceBGP, "fib", "firewall", sectionPlugins} {
		if !names[required] {
			return fmt.Errorf("missing health component %s; got %v", required, names)
		}
	}
	fmt.Fprintf(os.Stderr, "OK: show health has %d components including bgp, fib, firewall, plugins\n", len(components))
	return nil
}

func ifaceKernelRead08(ctx context.Context, p *sdk.Plugin) error {
	for _, command := range []string{"show route", "show route 10.0.0.0/8", "show route default", "show route limit 5", "show neighbor", "show neighbor ipv4", "show neighbor ipv6", "show arp"} {
		status, raw, err := command08(ctx, p, command)
		if status != statusDone && strings.Contains(strings.ToLower(string(raw)+fmt.Sprint(err)), "unknown command") {
			return fmt.Errorf("%s: not wired: %s %w", command, raw, err)
		}
	}
	fmt.Fprintln(os.Stderr, "OK iface kernel-read dispatch verified")
	return nil
}

func ifaceRateJSON08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	status, raw, err := command08(ctx, p, "show interface rate")
	message := string(raw) + fmt.Sprint(err)
	if status != statusError {
		return fmt.Errorf("interface rate: expected error, got status=%s", status)
	}
	if !strings.Contains(message, "rate tracker not running") && !strings.Contains(message, "no backend loaded") {
		return fmt.Errorf("interface rate: expected rate tracker error, got %s", message)
	}
	raw, err = requireDone08(ctx, p, "system help")
	if err != nil {
		return err
	}
	obj, err := object08(raw)
	if err != nil {
		return fmt.Errorf("system help: invalid JSON: %w", err)
	}
	if _, ok := obj["commands"]; !ok {
		return fmt.Errorf("system help: missing commands in %v", obj)
	}
	if len(raw) > 1 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return fmt.Errorf("system help: data is double-encoded: %.100s", raw)
	}
	return nil
}

func interfaceRateShow08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	status, raw, err := command08(ctx, p, "show interface rate")
	message := string(raw) + fmt.Sprint(err)
	if status == "" || strings.Contains(message, "unknown command") || !strings.Contains(message, "rate tracker not running") {
		return fmt.Errorf("interface-rate: unexpected response status=%s data=%s", status, message)
	}
	fmt.Fprintf(os.Stderr, "OK: show interface rate resolves: %s\n", message)
	status, raw, err = command08(ctx, p, "show interface rate zz-not-an-interface0")
	message = string(raw) + fmt.Sprint(err)
	if status != statusError || !strings.Contains(message, "interface not found: zz-not-an-interface0") {
		return fmt.Errorf("interface-rate zz-not-an-interface0: expected named refusal, status=%s data=%s", status, message)
	}
	fmt.Fprintf(os.Stderr, "OK: show interface rate zz-not-an-interface0 refused gracefully: %s\n", message)
	return nil
}
