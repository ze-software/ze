package healthcheck

import (
	"context"
	"strings"
	"testing"
)

func TestConfigParseBasic(t *testing.T) {
	jsonData := `{"bgp":{"healthcheck":{"probe":{"dns":{"command":"true","group":"hc-dns"}}}}}`
	probes, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(probes) != 1 {
		t.Fatalf("probes = %d, want 1", len(probes))
	}
	p := probes[0]
	if p.Name != "dns" {
		t.Errorf("Name = %q, want dns", p.Name)
	}
	if p.Command != "true" {
		t.Errorf("Command = %q, want true", p.Command)
	}
	if p.Group != "hc-dns" {
		t.Errorf("Group = %q, want hc-dns", p.Group)
	}
}

func TestConfigDefaults(t *testing.T) {
	jsonData := `{"bgp":{"healthcheck":{"probe":{"dns":{"command":"true","group":"hc-dns"}}}}}`
	probes, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	p := probes[0]
	if p.Interval != 5 {
		t.Errorf("Interval = %d, want 5", p.Interval)
	}
	if p.FastInterval != 1 {
		t.Errorf("FastInterval = %d, want 1", p.FastInterval)
	}
	if p.Timeout != 5 {
		t.Errorf("Timeout = %d, want 5", p.Timeout)
	}
	if p.Rise != 3 {
		t.Errorf("Rise = %d, want 3", p.Rise)
	}
	if p.Fall != 3 {
		t.Errorf("Fall = %d, want 3", p.Fall)
	}
	if p.UpMetric != 100 {
		t.Errorf("UpMetric = %d, want 100", p.UpMetric)
	}
	if p.DownMetric != 1000 {
		t.Errorf("DownMetric = %d, want 1000", p.DownMetric)
	}
	if p.DisabledMetric != 500 {
		t.Errorf("DisabledMetric = %d, want 500", p.DisabledMetric)
	}
	if p.WithdrawOnDown {
		t.Error("WithdrawOnDown should default to false")
	}
	if p.Debounce {
		t.Error("Debounce should default to false")
	}
}

func TestConfigMissingCommand(t *testing.T) {
	jsonData := `{"bgp":{"healthcheck":{"probe":{"dns":{"group":"hc-dns"}}}}}`
	_, err := parseConfig(jsonData)
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestConfigMissingGroup(t *testing.T) {
	jsonData := `{"bgp":{"healthcheck":{"probe":{"dns":{"command":"true"}}}}}`
	_, err := parseConfig(jsonData)
	if err == nil {
		t.Fatal("expected error for missing group")
	}
}

func TestConfigGroupNameMed(t *testing.T) {
	jsonData := `{"bgp":{"healthcheck":{"probe":{"dns":{"command":"true","group":"med"}}}}}`
	_, err := parseConfig(jsonData)
	if err == nil {
		t.Fatal("expected error for group name 'med'")
	}
}

func TestConfigDuplicateGroup(t *testing.T) {
	jsonData := `{"bgp":{"healthcheck":{"probe":{
		"dns":{"command":"true","group":"hc"},
		"web":{"command":"true","group":"hc"}
	}}}}`
	_, err := parseConfig(jsonData)
	if err == nil {
		t.Fatal("expected error for duplicate group")
	}
}

func TestConfigNoHealthcheck(t *testing.T) {
	jsonData := `{"bgp":{"peer":{}}}`
	probes, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(probes) != 0 {
		t.Errorf("probes = %d, want 0", len(probes))
	}
}

func TestConfigCustomValues(t *testing.T) {
	jsonData := `{"bgp":{"healthcheck":{"probe":{"dns":{
		"command":"curl localhost",
		"group":"hc-dns",
		"interval":10,
		"fast-interval":2,
		"timeout":3,
		"rise":5,
		"fall":2,
		"withdraw-on-down":true,
		"debounce":true,
		"up-metric":50,
		"down-metric":2000,
		"disabled-metric":750
	}}}}}`
	probes, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	p := probes[0]
	if p.Interval != 10 {
		t.Errorf("Interval = %d, want 10", p.Interval)
	}
	if p.FastInterval != 2 {
		t.Errorf("FastInterval = %d, want 2", p.FastInterval)
	}
	if p.Timeout != 3 {
		t.Errorf("Timeout = %d, want 3", p.Timeout)
	}
	if p.Rise != 5 {
		t.Errorf("Rise = %d, want 5", p.Rise)
	}
	if p.Fall != 2 {
		t.Errorf("Fall = %d, want 2", p.Fall)
	}
	if !p.WithdrawOnDown {
		t.Error("WithdrawOnDown should be true")
	}
	if !p.Debounce {
		t.Error("Debounce should be true")
	}
	if p.UpMetric != 50 {
		t.Errorf("UpMetric = %d, want 50", p.UpMetric)
	}
	if p.DownMetric != 2000 {
		t.Errorf("DownMetric = %d, want 2000", p.DownMetric)
	}
	if p.DisabledMetric != 750 {
		t.Errorf("DisabledMetric = %d, want 750", p.DisabledMetric)
	}
}

func TestExternalModeRejectsIPSetup(t *testing.T) {
	mgr := &probeManager{
		probes:   make(map[string]*runningProbe),
		internal: false,
		dispatchFn: func(_ context.Context, _ string) (string, string, error) {
			return "done", "", nil
		},
	}

	configs := []ProbeConfig{{
		Name:        "dns",
		Command:     "true",
		Group:       "hc-dns",
		IPInterface: "lo",
		IPs:         []string{"10.0.0.1/32"},
	}}

	err := mgr.validateConfig(configs)
	if err == nil {
		t.Fatal("expected error for ip-setup in external mode")
	}
	if !strings.Contains(err.Error(), "internal plugin mode") {
		t.Errorf("error = %q, want mention of internal plugin mode", err)
	}
}

func TestExternalModeAcceptsNoIPSetup(t *testing.T) {
	mgr := &probeManager{
		probes:   make(map[string]*runningProbe),
		internal: false,
		dispatchFn: func(_ context.Context, _ string) (string, string, error) {
			return "done", "", nil
		},
	}

	configs := []ProbeConfig{{
		Name:    "dns",
		Command: "true",
		Group:   "hc-dns",
	}}

	err := mgr.validateConfig(configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInternalModeAcceptsIPSetup(t *testing.T) {
	mgr := &probeManager{
		probes:   make(map[string]*runningProbe),
		internal: true,
		dispatchFn: func(_ context.Context, _ string) (string, string, error) {
			return "done", "", nil
		},
	}

	configs := []ProbeConfig{{
		Name:        "dns",
		Command:     "true",
		Group:       "hc-dns",
		IPInterface: "lo",
		IPs:         []string{"10.0.0.1/32"},
	}}

	err := mgr.validateConfig(configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
