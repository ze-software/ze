package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const fixture06PollDelay = 250 * time.Millisecond

func init() {
	observers := map[string]ObserverScenario{
		"plugin/ddos-show":                      fixture06DDOSShow,
		"plugin/deadpeer-holddown":              fixture06DeadpeerHolddown,
		"plugin/decode-mp-reach":                fixture06DecodeMPReach,
		"plugin/decode-mp-unreach":              fixture06DecodeMPUnreach,
		"plugin/decode-update":                  fixture06DecodeUpdate,
		"plugin/diag-capture":                   fixture06DiagCapture,
		"plugin/dispatch-command-single-decode": fixture06DispatchSingleDecode,
		"plugin/dns-cache-show":                 fixture06DNSCacheShow,
		"plugin/dns-lookup-show":                fixture06DNSLookupShow,
		"plugin/doctor-owner-check-show":        fixture06DoctorOwnerCheck,
		"plugin/enricher-external-show-checker": fixture06EnricherExternalChecker,
		"plugin/enricher-fakeenrich-show":       fixture06EnricherFakeChecker,
		"plugin/eor-per-family":                 fixture06EORPerFamily,
		"plugin/errors-config-abort-show":       fixture06ErrorsConfigAbort,
		"plugin/errors-received-show":           fixture06ErrorsReceived,
		"plugin/errors-sent-show":               fixture06ErrorsSent,
		"plugin/event-monitor-basic":            fixture06EventMonitorBasic,
		"plugin/event-monitor-exclude":          fixture06EventMonitorExclude,
		"plugin/event-monitor-include":          fixture06EventMonitorInclude,
		"plugin/event-monitor-peer":             fixture06EventMonitorPeer,
		"plugin/event-predicate-wait":           fixture06EventPredicateWait,
		"plugin/explicit-plugin-config":         fixture06ExplicitPluginConfig,
		"plugin/fib-blackhole":                  fixture06FIBBlackhole,
		"plugin/fib-ecmp-realtime":              fixture06FIBECMPRealtime,
		"plugin/fib-ecmp":                       fixture06FIBECMP,
		"plugin/fib-metric":                     fixture06FIBMetric,
		"plugin/fib-mpls-kernel":                fixture06FIBMPLSKernel,
		"plugin/fib-recursive":                  fixture06FIBRecursive,
	}
	for name, scenario := range observers {
		Register(name, func(ctx context.Context, _ []string) error {
			return Observe(ctx, name, fixture06Registration(name), scenario)
		})
	}

	Register("plugin/enricher-external-show-provider", fixture06EnricherExternalProvider)
	Register("plugin/exabgp-bridge-helper", fixture06ExaBGPHelper)
	Register("plugin/exabgp-bridge-bare-helper", fixture06ExaBGPBareHelper)
	Register("plugin/exec-answer-unconditional-frame-check", fixture06FrameCheck)
}

func fixture06Registration(name string) sdk.Registration {
	switch name {
	case "plugin/enricher-external-show-checker":
		return sdk.Registration{}
	case "plugin/errors-config-abort-show":
		return sdk.Registration{WantsConfig: []string{namespaceBGP}}
	case "plugin/explicit-plugin-config":
		return sdk.Registration{Families: []sdk.FamilyDecl{
			{Name: "ipv4/flow", Mode: modeDecode, AFI: 1, SAFI: 133},
			{Name: "ipv6/flow", Mode: modeDecode, AFI: 2, SAFI: 133},
			{Name: "ipv4/flow-vpn", Mode: modeDecode, AFI: 1, SAFI: 134},
			{Name: "ipv6/flow-vpn", Mode: modeDecode, AFI: 2, SAFI: 134},
			{Name: familyIPv4Unicast, Mode: modeBoth, AFI: 1, SAFI: 1},
		}}
	default:
		return sdk.Registration{}
	}
}

func fixture06DispatchObject(ctx context.Context, p *sdk.Plugin, command string) (map[string]any, error) {
	var value map[string]any
	status, err := Dispatch(ctx, p, command, &value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", command, err)
	}
	if status != statusDone {
		return nil, fmt.Errorf("%s: status=%s", command, status)
	}
	if value == nil {
		return nil, fmt.Errorf("%s: expected object", command)
	}
	return value, nil
}

func fixture06DispatchArray(ctx context.Context, p *sdk.Plugin, command string) ([]map[string]any, error) {
	var value []map[string]any
	status, err := Dispatch(ctx, p, command, &value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", command, err)
	}
	if status != statusDone {
		return nil, fmt.Errorf("%s: status=%s", command, status)
	}
	return value, nil
}

func fixture06DispatchDone(ctx context.Context, p *sdk.Plugin, command string) error {
	status, _, err := p.DispatchCommand(ctx, command)
	if err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	if status != statusDone {
		return fmt.Errorf("%s: status=%s", command, status)
	}
	return nil
}

func fixture06PollObject(ctx context.Context, p *sdk.Plugin, command string, attempts int, predicate func(map[string]any) bool) (map[string]any, error) {
	var last map[string]any
	var lastErr error
	if !Poll(ctx, attempts, fixture06PollDelay, func() bool {
		last, lastErr = fixture06DispatchObject(ctx, p, command)
		return lastErr == nil && predicate(last)
	}) {
		if lastErr != nil {
			return nil, lastErr
		}
		return last, fmt.Errorf("%s: predicate not satisfied after %d attempts", command, attempts)
	}
	return last, nil
}

// fixture06RIBCommand is the only command fixture06PollArray polls.
const fixture06RIBCommand = "show rib"

func fixture06PollArray(ctx context.Context, p *sdk.Plugin, attempts int, predicate func([]map[string]any) bool) ([]map[string]any, error) {
	var last []map[string]any
	var lastErr error
	if !Poll(ctx, attempts, fixture06PollDelay, func() bool {
		last, lastErr = fixture06DispatchArray(ctx, p, fixture06RIBCommand)
		return lastErr == nil && predicate(last)
	}) {
		if lastErr != nil {
			return nil, lastErr
		}
		return last, fmt.Errorf("%s: predicate not satisfied after %d attempts", fixture06RIBCommand, attempts)
	}
	return last, nil
}

func fixture06Number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func fixture06StringSlice(v any) ([]string, bool) {
	values, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i], ok = value.(string)
		if !ok {
			return nil, false
		}
	}
	return out, true
}

func fixture06WaitEOR(ctx context.Context, p *sdk.Plugin, minimum float64) error {
	_, err := fixture06PollObject(ctx, p, "show bgp peer peer1 detail", 60, func(data map[string]any) bool {
		peers, ok := data["peers"].(map[string]any)
		if !ok {
			return false
		}
		for _, raw := range peers {
			peer, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if count, ok := fixture06Number(peer["eor-sent"]); ok && count >= minimum {
				return true
			}
		}
		return false
	})
	if err != nil {
		return fmt.Errorf("peer1 never sent %.0f End-of-RIB marker(s): %w", minimum, err)
	}
	return nil
}

func fixture06DDOSShow(ctx context.Context, p *sdk.Plugin) error {
	check := func(command string, keys ...string) (map[string]any, error) {
		data, err := fixture06DispatchObject(ctx, p, command)
		if err != nil {
			return nil, err
		}
		if data["enabled"] != true {
			return nil, fmt.Errorf("%s: enabled != true: %v", command, data)
		}
		for _, key := range keys {
			if _, ok := data[key]; !ok {
				return nil, fmt.Errorf("%s: missing %q in %v", command, key, data)
			}
		}
		fmt.Fprintf(os.Stderr, "OK: %s -> %v\n", command, data)
		return data, nil
	}
	status, err := check("show ddos status", "active-attacks", "incidents")
	if err != nil {
		return err
	}
	if active, ok := fixture06Number(status["active-attacks"]); !ok || active != 0 {
		return fmt.Errorf("show ddos status: active-attacks != 0: %v", status)
	}
	incidents, err := check("show ddos incidents", "incidents")
	if err != nil {
		return err
	}
	if _, ok := incidents["incidents"].([]any); !ok {
		return fmt.Errorf("show ddos incidents: incidents is not a list: %v", incidents)
	}
	local, err := check("show ddos local", "active")
	if err != nil {
		return err
	}
	if local["active"] != false {
		return fmt.Errorf("show ddos local: active != false: %v", local)
	}
	flowspec, err := check("show ddos flowspec", "active")
	if err != nil {
		return err
	}
	if flowspec["active"] != false {
		return fmt.Errorf("show ddos flowspec: active != false: %v", flowspec)
	}
	return nil
}

func fixture06DeadpeerHolddown(ctx context.Context, p *sdk.Plugin) error {
	const metric = `ze_peer_session_flaps_total{peer="127.0.0.1"}`
	_, err := fixture06PollObject(ctx, p, "show metrics values", 20, func(data map[string]any) bool {
		text, _ := data["metrics"].(string)
		for line := range strings.SplitSeq(text, "\n") {
			if strings.HasPrefix(line, "#") || !strings.Contains(line, metric) {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}
			value, parseErr := strconv.ParseFloat(parts[len(parts)-1], 64)
			return parseErr == nil && value >= 1
		}
		return false
	})
	if err != nil {
		return errors.New("silent peer still up 5.0s after establishing, with a 3s hold time: RFC 4271 Section 8.2.2 Event 10 requires teardown on the first expiry")
	}
	fmt.Fprintln(os.Stderr, "OK: silent peer torn down on its first hold expiry, within 5.0s")
	return nil
}

func fixture06DNSCacheShow(ctx context.Context, p *sdk.Plugin) error {
	checkShape := func(command string, expected []string) (map[string]any, error) {
		data, err := fixture06DispatchObject(ctx, p, command)
		if err != nil {
			return nil, err
		}
		actual := make([]string, 0, len(data))
		for key := range data {
			actual = append(actual, key)
		}
		sort.Strings(actual)
		sort.Strings(expected)
		if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
			return nil, fmt.Errorf("%s: keys=%v, want %v", command, actual, expected)
		}
		fmt.Fprintf(os.Stderr, "OK: %s returned keys %v\n", command, actual)
		return data, nil
	}
	stats, err := checkShape("show dns cache stats", []string{"capacity", fieldEntries, "evictions", "expired", "hit-rate", "hits", "miss-rate", "misses"})
	if err != nil {
		return err
	}
	for key, value := range stats {
		if _, ok := fixture06Number(value); !ok {
			return fmt.Errorf("show dns cache stats: %s=%v, want number", key, value)
		}
	}
	for _, test := range []struct {
		command string
		keys    []string
	}{
		{"show dns cache list", []string{fieldCount, fieldEntries}},
		{"show dns cache record localhost", []string{fieldCount, fieldEntries, "filter"}},
	} {
		data, shapeErr := checkShape(test.command, test.keys)
		if shapeErr != nil {
			return shapeErr
		}
		if _, ok := fixture06Number(data["count"]); !ok {
			return fmt.Errorf("%s: count=%v, want number", test.command, data["count"])
		}
		if _, ok := data["entries"].([]any); !ok {
			return fmt.Errorf("%s: entries=%v, want list", test.command, data["entries"])
		}
		if test.command == "show dns cache record localhost" && data["filter"] != "localhost" {
			return fmt.Errorf("%s: filter=%v", test.command, data["filter"])
		}
	}
	return nil
}

func fixture06DNSLookupShow(ctx context.Context, p *sdk.Plugin) error {
	data, err := fixture06DispatchObject(ctx, p, "show dns lookup localhost type A")
	if err != nil {
		return err
	}
	if data["name"] != "localhost" {
		return fmt.Errorf("dns lookup name=%v, want localhost", data["name"])
	}
	if _, ok := data["records"]; !ok {
		return fmt.Errorf("dns lookup missing records: %v", data)
	}
	if _, ok := data["query-time-ms"]; !ok {
		return fmt.Errorf("dns lookup missing query-time-ms: %v", data)
	}
	fmt.Fprintf(os.Stderr, "OK: dns lookup name=%v records=%v\n", data["name"], data["records"])
	return nil
}

func fixture06DoctorOwnerCheck(ctx context.Context, p *sdk.Plugin) error {
	data, err := fixture06PollObject(ctx, p, "show doctor doctor-missing-plugin.conf", 60, func(map[string]any) bool { return true })
	if err != nil {
		return err
	}
	diagnostics, ok := data["diagnostics"].([]any)
	if !ok {
		return fmt.Errorf("show doctor diagnostics=%v, want list", data["diagnostics"])
	}
	for _, raw := range diagnostics {
		diagnostic, _ := raw.(map[string]any)
		if diagnostic["code"] == "doctor-plugin-missing" {
			return nil
		}
	}
	return fmt.Errorf("show doctor missing owner-registered plugin check: %v", diagnostics)
}

func fixture06EnricherExternalProvider(ctx context.Context, _ []string) error {
	p, err := newObserver("plugin/enricher-external-show-provider")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // fixture teardown
	p.OnEnrichShow(func(command, key, _ string, _ map[string]any) (map[string]any, error) {
		if command == "show test enrich" && key == "ext-cos" {
			return map[string]any{"cos-profile": profileResidential, "ext-source": "enricher-provider"}, nil
		}
		return map[string]any{}, nil
	})
	return p.Run(ctx, sdk.Registration{Enrichers: []sdk.EnricherDecl{{Command: "show test enrich", Key: "ext-cos"}}})
}

func fixture06EnricherExternalChecker(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06WaitEOR(ctx, p, 1); err != nil {
		return err
	}
	data, err := fixture06PollObject(ctx, p, "show test enrich", 60, func(data map[string]any) bool {
		return data["cos-profile"] == profileResidential && data["ext-source"] == "enricher-provider"
	})
	if err != nil {
		return fmt.Errorf("show test enrich external data missing: %v: %w", data, err)
	}
	return nil
}

func fixture06EnricherFakeChecker(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06WaitEOR(ctx, p, 1); err != nil {
		return err
	}
	data, err := fixture06PollObject(ctx, p, "show test enrich", 60, func(data map[string]any) bool {
		return data["fakeenrich"] == "present"
	})
	if err != nil {
		return fmt.Errorf("expected fakeenrich=present, got %v: %w", data, err)
	}
	return nil
}

func fixture06EORPerFamily(ctx context.Context, p *sdk.Plugin) error {
	return fixture06WaitEOR(ctx, p, 2)
}

func fixture06ExplicitPluginConfig(ctx context.Context, p *sdk.Plugin) error {
	announced, _, err := p.UpdateRoute(ctx, "*", "update text origin igp local-preference 100 nhop 1.2.3.4 nlri ipv4/unicast add 99.99.99.0/24")
	if err != nil {
		return err
	}
	if announced != 1 {
		return fmt.Errorf("marker update announced %d NLRIs, want 1", announced)
	}
	return fixture06DispatchDone(ctx, p, "request quiesce")
}
