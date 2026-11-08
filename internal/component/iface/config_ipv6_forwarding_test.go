// Design: docs/guide/configuration.md -- IPv6 Router Advertisements, forwarding
//
// VALIDATES: a unit with ipv6 forwarding true makes the interface component
// hold net.ipv6.conf.all.forwarding at 1, after every per-device value, and a
// config where no unit forwards withdraws that default instead.
// PREVENTS: a CPE whose LAN and WAN units say forwarding true while Linux
// drops every forwarded IPv6 packet, because ip6_forward reads only
// all.forwarding (or the ingress device's force_forwarding).

package iface

import (
	"encoding/json"
	"testing"

	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
)

// sysctlEmission is one sysctl event applyConfig sent: a default (key, value,
// source) or a clear-source-defaults (source alone).
type sysctlEmission struct {
	eventType string
	key       string
	value     string
	source    string
}

func sysctlEmissions(t *testing.T, bus *testEventBus) []sysctlEmission {
	t.Helper()
	var out []sysctlEmission
	for _, e := range bus.emissions {
		if e.namespace != sysctlevents.Namespace {
			continue
		}
		payload, ok := e.data.(string)
		if !ok {
			t.Fatalf("sysctl payload is %T, want a JSON string", e.data)
		}
		var kv struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Source string `json:"source"`
		}
		if err := json.Unmarshal([]byte(payload), &kv); err != nil {
			t.Fatalf("sysctl payload %q: %v", payload, err)
		}
		out = append(out, sysctlEmission{eventType: e.eventType, key: kv.Key, value: kv.Value, source: kv.Source})
	}
	return out
}

func applyWithBus(t *testing.T, input string) []sysctlEmission {
	t.Helper()
	bus := &testEventBus{}
	SetEventBus(bus)
	t.Cleanup(func() { SetEventBus(nil) })

	cfg := mustParseIfaceJSON(t, input)
	if errs := applyConfig(cfg, nil, &fakeBackend{}); len(errs) > 0 {
		t.Fatalf("applyConfig: %v", errs)
	}
	return sysctlEmissions(t, bus)
}

// TestIPv6ForwardingUnitTurnsOnTheGlobalSwitch applies a config where one unit
// forwards IPv6 and another says nothing, and reads the sysctl events.
func TestIPv6ForwardingUnitTurnsOnTheGlobalSwitch(t *testing.T) {
	got := applyWithBus(t, `{"interface": {"dummy": {
		"lan0": {"unit": {"0": {"ipv6": {"forwarding": "true"}}}},
		"wan0": {"unit": {"0": {}}}
	}}}`)

	perDevice, global := -1, -1
	for i, e := range got {
		if e.eventType != sysctlevents.EventDefault {
			continue
		}
		switch e.key {
		case "net.ipv6.conf.lan0.forwarding":
			perDevice = i
			if e.value != "1" {
				t.Errorf("lan0 forwarding = %q, want 1", e.value)
			}
		case "net.ipv6.conf.all.forwarding":
			global = i
			if e.value != "1" {
				t.Errorf("all.forwarding = %q, want 1", e.value)
			}
			if e.source != sourceIPv6Forwarding {
				t.Errorf("all.forwarding source = %q, want %q", e.source, sourceIPv6Forwarding)
			}
		case "net.ipv6.conf.wan0.forwarding":
			t.Errorf("wan0 forwarding emitted (%q) though its config says nothing", e.value)
		}
	}
	if perDevice < 0 {
		t.Fatalf("no net.ipv6.conf.lan0.forwarding default emitted: %+v", got)
	}
	if global < 0 {
		t.Fatalf("no net.ipv6.conf.all.forwarding default emitted while lan0 forwards: %+v", got)
	}
	if global < perDevice {
		t.Errorf("all.forwarding (event %d) emitted before lan0's forwarding (event %d), want after", global, perDevice)
	}
	for _, e := range got {
		if e.eventType == sysctlevents.EventClearSourceDefaults && e.source == sourceIPv6Forwarding {
			t.Errorf("the IPv6 forwarding switch was withdrawn while lan0 forwards")
		}
	}
}

// TestNoIPv6ForwardingUnitWithdrawsTheGlobalSwitch applies configs where no
// unit forwards IPv6, one with forwarding false and one with no ipv6 leaf,
// and expects the switch's default withdrawn and never set.
func TestNoIPv6ForwardingUnitWithdrawsTheGlobalSwitch(t *testing.T) {
	for name, input := range map[string]string{
		"forwarding false": `{"interface": {"dummy": {"lan0": {"unit": {"0": {"ipv6": {"forwarding": "false"}}}}}}}`,
		"no ipv6 leaf":     `{"interface": {"dummy": {"lan0": {"unit": {"0": {}}}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			got := applyWithBus(t, input)
			withdrawn := false
			for _, e := range got {
				if e.eventType == sysctlevents.EventDefault && e.key == "net.ipv6.conf.all.forwarding" {
					t.Errorf("all.forwarding default %q emitted though no unit forwards", e.value)
				}
				if e.eventType == sysctlevents.EventClearSourceDefaults && e.source == sourceIPv6Forwarding {
					withdrawn = true
				}
			}
			if !withdrawn {
				t.Errorf("no clear-source-defaults for %q: a switch an earlier config turned on stays on (%+v)", sourceIPv6Forwarding, got)
			}
		})
	}
}

// TestDisabledUnitDoesNotTurnOnTheGlobalSwitch proves a disabled unit's
// forwarding leaf counts for nothing: Ze configures nothing on it.
func TestDisabledUnitDoesNotTurnOnTheGlobalSwitch(t *testing.T) {
	got := applyWithBus(t, `{"interface": {"dummy": {
		"lan0": {"unit": {"0": {"disable": "true", "ipv6": {"forwarding": "true"}}}}
	}}}`)
	for _, e := range got {
		if e.eventType == sysctlevents.EventDefault && e.key == "net.ipv6.conf.all.forwarding" {
			t.Errorf("all.forwarding default emitted for a disabled unit")
		}
	}
}
