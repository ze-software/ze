// Design: plan/spec-isis-4-component-config.md -- config resolution unit tests
//
// VALIDATES: the `isis` config subtree (root-wrapped, string-typed leaves, keyed
// lists) parses into typed Config with YANG defaults applied; NET-only config
// derives the System ID; a missing NET is rejected; the NET and system-id
// validators accept valid inputs and reject malformed ones.
package isis

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

func sec(data string) []configSection {
	return []configSection{{Root: "isis", Data: data}}
}

// TestISISConfigResolve: a full isis subtree resolves to typed structs.
func TestISISConfigResolve(t *testing.T) {
	data := `{"isis":{` +
		`"net":"49.0001.0000.0000.0001.00",` +
		`"level":"l2",` +
		`"lsp-lifetime":"600",` +
		`"lsp-refresh-interval":"500",` +
		`"overload":"true",` +
		`"hostname":"r1",` +
		`"interfaces":{"interface":{"eth0":{"name":"eth0","metric":"100","hello-interval":"3","hold-multiplier":"4","priority":"7","circuit-type":"point-to-point","level":"l1","passive":"false","enabled":"true","address-family":{"ipv4-unicast":{"af":"ipv4-unicast"},"ipv6-unicast":{"af":"ipv6-unicast"}},"level-1":{"metric":"50","priority":"5","auth-key-chain":"area-key"}}}},` +
		`"key-chains":{"area-key":{"name":"area-key","key":{"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t"}}}},` +
		`"level-1":{"auth-key-chain":"area-key"},` +
		`"level-2":{"auth-key-chain":"domain-key"}}}`

	cfg, err := parseISISConfig(sec(data))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	if len(cfg.NETs) != 1 {
		t.Fatalf("NETs = %d, want 1", len(cfg.NETs))
	}
	if got := cfg.NETs[0].String(); got != "49.0001.0000.0000.0001.00" {
		t.Errorf("NET = %q", got)
	}
	if cfg.Level != LevelL2 {
		t.Errorf("Level = %v, want l2", cfg.Level)
	}
	if cfg.LSPLifetime != 600 || cfg.LSPRefreshInterval != 500 {
		t.Errorf("lifetimes = %d/%d, want 600/500", cfg.LSPLifetime, cfg.LSPRefreshInterval)
	}
	if !cfg.Overload {
		t.Error("Overload = false, want true")
	}
	if cfg.Hostname != "r1" {
		t.Errorf("Hostname = %q, want r1", cfg.Hostname)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("Interfaces = %d, want 1", len(cfg.Interfaces))
	}
	ic := cfg.Interfaces[0]
	if ic.Name != "eth0" || ic.Metric != 100 || ic.HelloInterval != 3 || ic.HoldMult != 4 || ic.Priority != 7 {
		t.Errorf("iface = %+v", ic)
	}
	if ic.CircuitType != CircuitPointToPoint {
		t.Errorf("circuit-type = %v, want point-to-point", ic.CircuitType)
	}
	if ic.Level != LevelL1 {
		t.Errorf("iface level = %v, want l1", ic.Level)
	}
	if ic.Level1.Metric != 50 || ic.Level1.Priority != 5 || ic.Level1.AuthKeyChain != "area-key" {
		t.Errorf("level-1 override = %+v", ic.Level1)
	}
	if len(ic.AddressFamily) != 2 || ic.AddressFamily[0] != "ipv4-unicast" || ic.AddressFamily[1] != "ipv6-unicast" {
		t.Errorf("address-family = %v", ic.AddressFamily)
	}
	if len(cfg.KeyChains) != 1 || cfg.KeyChains[0].Name != "area-key" || len(cfg.KeyChains[0].Keys) != 1 {
		t.Fatalf("key-chains = %+v", cfg.KeyChains)
	}
	k := cfg.KeyChains[0].Keys[0]
	if k.KeyID != 1 || k.Algorithm != "hmac-sha-256" || k.Secret != "s3cr3t" {
		t.Errorf("key = %+v", k)
	}
	if cfg.Level1AuthKeyChain != "area-key" || cfg.Level2AuthKeyChain != "domain-key" {
		t.Errorf("per-level chains = %q/%q", cfg.Level1AuthKeyChain, cfg.Level2AuthKeyChain)
	}
}

// TestISISConfigDefaults: omitted leaves resolve to YANG defaults.
func TestISISConfigDefaults(t *testing.T) {
	data := `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{}}}}}`
	cfg, err := parseISISConfig(sec(data))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	if cfg.Level != LevelL1L2 {
		t.Errorf("Level default = %v, want l1-l2", cfg.Level)
	}
	if cfg.LSPLifetime != 1200 {
		t.Errorf("lsp-lifetime default = %d, want 1200", cfg.LSPLifetime)
	}
	if cfg.LSPRefreshInterval != 900 {
		t.Errorf("lsp-refresh-interval default = %d, want 900", cfg.LSPRefreshInterval)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("Interfaces = %d, want 1", len(cfg.Interfaces))
	}
	ic := cfg.Interfaces[0]
	if ic.Name != "eth0" {
		t.Errorf("iface name = %q", ic.Name)
	}
	if !ic.Enabled {
		t.Error("Enabled default = false, want true")
	}
	if ic.Passive {
		t.Error("Passive default = true, want false")
	}
	if ic.CircuitType != CircuitBroadcast {
		t.Errorf("circuit-type default = %v, want broadcast", ic.CircuitType)
	}
	if ic.Metric != 10 {
		t.Errorf("metric default = %d, want 10", ic.Metric)
	}
	if ic.HelloInterval != 10 {
		t.Errorf("hello-interval default = %d, want 10", ic.HelloInterval)
	}
	if ic.HoldMult != 3 {
		t.Errorf("hold-multiplier default = %d, want 3", ic.HoldMult)
	}
	if ic.Priority != 64 {
		t.Errorf("priority default = %d, want 64", ic.Priority)
	}
}

// TestISISConfigValidate: NET-only validates and derives the System ID;
// missing NET is rejected.
func TestISISConfigValidate(t *testing.T) {
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00"}}`))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(net-only): %v", err)
	}
	// AC-9: the System ID is the 6 octets before the NSEL of the first NET.
	wantSID, _ := types.ParseSystemID("0000.0000.0001")
	if cfg.SystemID != wantSID {
		t.Errorf("derived system-id = %v, want %v", cfg.SystemID, wantSID)
	}

	// AC-3: no net -> rejected.
	empty, err := parseISISConfig(sec(`{"isis":{"hostname":"r1"}}`))
	if err != nil {
		t.Fatalf("parseISISConfig(no net): %v", err)
	}
	if err := validateConfig(empty); err == nil {
		t.Error("validateConfig(no net) = nil, want ErrNoNET")
	}

	// An explicit system-id that matches the NET is accepted.
	ok, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","system-id":"0000.0000.0001"}}`))
	if err != nil {
		t.Fatalf("parseISISConfig(matching sid): %v", err)
	}
	if err := validateConfig(ok); err != nil {
		t.Errorf("validateConfig(matching sid): %v", err)
	}
	// A mismatching explicit system-id is rejected.
	bad, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","system-id":"0000.0000.0002"}}`))
	if err != nil {
		t.Fatalf("parseISISConfig(mismatch sid): %v", err)
	}
	if err := validateConfig(bad); err == nil {
		t.Error("validateConfig(mismatch sid) = nil, want ErrSystemIDMismatch")
	}
}

// TestISISConfigInvalidNET: a structurally invalid NET is rejected at parse.
func TestISISConfigInvalidNET(t *testing.T) {
	cases := map[string]string{
		"bad hex":    `{"isis":{"net":"zz.0001.0000.0000.0001.00"}}`,
		"too short":  `{"isis":{"net":"49.0001.00"}}`,
		"odd nibble": `{"isis":{"net":"490.001.0000.0000.0001.00"}}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseISISConfig(sec(data)); err == nil {
				t.Errorf("parseISISConfig(%s) = nil, want error", name)
			}
		})
	}
}

// TestISISConfigBoundaries asserts the YANG declares the exact numeric ranges
// from the spec Boundary Tests table. YANG native `range` validation is the
// enforcement point for these leaves (the component never sees an out-of-range
// value -- the schema validator rejects it before OnConfigVerify), so the
// boundary check is that the schema declares the right bounds. Reading the YANG
// from disk keeps the test independent of the generated embed.go.
func TestISISConfigBoundaries(t *testing.T) {
	src, err := os.ReadFile("yang/ze-isis-conf.yang")
	if err != nil {
		t.Fatalf("read yang: %v", err)
	}
	yang := string(src)

	// Each row of the spec Boundary Tests table maps to a `range` declaration.
	wantRanges := []struct {
		field string
		decl  string
	}{
		{"metric (wide, 1..16777215)", `range "1..16777215"`},
		{"dis priority (0..127)", `range "0..127"`},
		{"hold-multiplier (1..255)", `range "1..255"`},
		{"hello-interval (1..65535)", `range "1..65535"`},
	}
	for _, w := range wantRanges {
		if !strings.Contains(yang, w.decl) {
			t.Errorf("YANG missing %s: expected a %s declaration", w.field, w.decl)
		}
	}
	// lsp-lifetime and lsp-refresh-interval also use 1..65535 (shared with
	// hello-interval); the count of 1..65535 declarations must cover all three
	// per-instance uses plus the per-level hello-interval overrides.
	if n := strings.Count(yang, `range "1..65535"`); n < 3 {
		t.Errorf(`got %d "1..65535" range declarations, want >= 3 (lsp-lifetime, lsp-refresh-interval, hello-interval)`, n)
	}

	// The Go default constants must equal the YANG defaults (single source of
	// truth is the YANG; these constants mirror it).
	for _, d := range []struct {
		field string
		decl  string
	}{
		{"lsp-lifetime", "default 1200"},
		{"lsp-refresh-interval", "default 900"},
		{"metric", "default 10"},
		{"hold-multiplier", "default 3"},
		{"priority", "default 64"},
		{"hello-interval", "default 10"},
	} {
		if !strings.Contains(yang, d.decl) {
			t.Errorf("YANG missing %s %q", d.field, d.decl)
		}
	}
	// Go constants stay in lock-step with the YANG defaults.
	if DefaultLSPLifetime != 1200 || DefaultLSPRefreshInterval != 900 ||
		DefaultMetric != 10 || DefaultHoldMultiplier != 3 ||
		DefaultPriority != 64 || DefaultHelloInterval != 10 {
		t.Errorf("Go defaults drifted from YANG: lifetime=%d refresh=%d metric=%d holdmult=%d prio=%d hello=%d",
			DefaultLSPLifetime, DefaultLSPRefreshInterval, DefaultMetric,
			DefaultHoldMultiplier, DefaultPriority, DefaultHelloInterval)
	}
	if MaxWideMetric != 16777215 {
		t.Errorf("MaxWideMetric = %d, want 16777215 (RFC 5305 24-bit)", MaxWideMetric)
	}
}

// TestISISConfigEnabledCircuits: only enabled, non-passive interfaces open a
// circuit.
func TestISISConfigEnabledCircuits(t *testing.T) {
	data := `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{` +
		`"eth0":{"enabled":"true","passive":"false"},` +
		`"eth1":{"enabled":"true","passive":"true"},` +
		`"eth2":{"enabled":"false","passive":"false"}}}}}`
	cfg, err := parseISISConfig(sec(data))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	circuits := cfg.EnabledCircuits()
	if len(circuits) != 1 || circuits[0].Name != "eth0" {
		t.Errorf("EnabledCircuits = %+v, want [eth0]", circuits)
	}
}
