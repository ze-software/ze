// Design: plan/learned/958-ospf-4-component-config.md -- config resolution unit tests
//
// VALIDATES: the `ospf` config subtree (root-wrapped JSON, string leaves,
// keyed lists) resolves into typed structs with defaults, derives router-id
// from interfaces when omitted, rejects underivable router-id and undeclared
// area bindings, and keeps passive interfaces enrolled but not active.
package ospf

import (
	"errors"
	"os"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

type staticRouterIDSource []iface.InterfaceInfo

func (s staticRouterIDSource) Interfaces() ([]iface.InterfaceInfo, error) {
	return []iface.InterfaceInfo(s), nil
}

func ospfSec(data string) []configSection { return []configSection{{Root: "ospf", Data: data}} }

func TestOSPFConfigRejectsInvalidEnumsAndCost(t *testing.T) {
	// The parser validates the area-type/network-type YANG enums and the 16-bit interface cost
	// range instead of silently downgrading an unrecognized enum (to normal/broadcast) or
	// truncating an out-of-range cost (65536 -> 0). This defends the non-YANG doctor/verifier
	// parse paths where the YANG enum/range constraints are not applied first.
	bad := []struct {
		name, json string
	}{
		{"area-type", `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.1":{"area-id":"0.0.0.1","area-type":"bogus"}}}}}`},
		{"network-type", `{"ospf":{"router-id":"10.0.0.1","interfaces":{"interface":{"eth0":{"area":"0.0.0.0","network-type":"bogus"}}}}}`},
		{"cost-over-max", `{"ospf":{"router-id":"10.0.0.1","interfaces":{"interface":{"eth0":{"area":"0.0.0.0","cost":"65536"}}}}}`},
	}
	for _, c := range bad {
		if _, err := parseOSPFConfig(ospfSec(c.json), nil); err == nil {
			t.Errorf("%s: parseOSPFConfig accepted invalid input, want an error", c.name)
		}
	}
	// The full YANG-valid set still parses: a loopback network-type and the boundary cost.
	if _, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","interfaces":{"interface":{"eth0":{"area":"0.0.0.0","network-type":"loopback","cost":"65535"}}}}}`), nil); err != nil {
		t.Errorf("valid loopback network-type / boundary cost rejected: %v", err)
	}
}

// VALIDATES: spec-ospf-13 -- the RFC 6987 `max-metric router-lsa` leaves (always,
// on-startup, on-shutdown durations) resolve into maxMetricConfig.
func TestOSPFMaxMetricConfig(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","max-metric":{"router-lsa":{"always":"true","on-startup":"300","on-shutdown":"60"}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if !cfg.MaxMetric.RouterLSAAlways || cfg.MaxMetric.OnStartupSec != 300 || cfg.MaxMetric.OnShutdownSec != 60 {
		t.Fatalf("max-metric = %+v", cfg.MaxMetric)
	}
}

// VALIDATES: spec-ospf-11 -- the NSSA area `nssa { translate-role / stability-interval
// / default-originate }` leaves resolve into areaConfig with RFC 3101 defaults
// (candidate / 40s / false), the stability-interval boundary (65535) parses, and an
// invalid translate-role is rejected by validateConfig.
func TestOSPFNSSAAreaConfig(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.1":{"area-id":"0.0.0.1","area-type":"nssa","nssa":{"translate-role":"always","stability-interval":"65535","default-originate":"true"}}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.1"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if len(cfg.Areas) != 1 {
		t.Fatalf("Areas = %d, want 1", len(cfg.Areas))
	}
	a := cfg.Areas[0]
	if a.AreaType != areaTypeNSSA || a.NSSATranslateRole != translateRoleAlways || a.NSSAStabilityInterval != 65535 || !a.NSSADefaultOriginate {
		t.Fatalf("nssa config = %+v", a)
	}

	// Defaults: an nssa area with no `nssa` container gets RFC 3101 defaults.
	def, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.2":{"area-id":"0.0.0.2","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.2"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(defaults): %v", err)
	}
	d := def.Areas[0]
	if d.NSSATranslateRole != translateRoleCandidate || d.NSSAStabilityInterval != DefaultNSSAStabilityInterval || d.NSSADefaultOriginate {
		t.Fatalf("nssa defaults = %+v", d)
	}

	// Invalid translate-role is rejected at validation.
	bad, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0","area-type":"nssa","nssa":{"translate-role":"bogus"}}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(bad role): %v", err)
	}
	if err := validateConfig(bad); !errors.Is(err, ErrInvalidNSSARole) {
		t.Fatalf("validateConfig(bad role) = %v, want ErrInvalidNSSARole", err)
	}
}

func TestOSPFConfigResolve(t *testing.T) {
	data := `{"ospf":{` +
		`"router-id":"10.0.0.1",` +
		`"reference-bandwidth":"200000",` +
		`"maximum-paths":"16",` +
		`"default-information":{"originate":"true","always":"true","metric":"7","metric-type":"type-1"},` +
		`"timers":{"spf-delay-ms":"10","spf-hold-ms":"20","spf-max-hold-ms":"30","min-ls-interval-ms":"40","min-ls-arrival-ms":"50"},` +
		`"redistribute":{"static":{"source":"static","metric":"33","metric-type":"type-2","tag":"99"}},` +
		`"areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0","area-type":"stub","no-summary":"true","default-cost":"3","authentication":{"key-chain":"area-key"},"ranges":{"range":{"10.0.0.0/16":{"prefix":"10.0.0.0/16","advertise":"not-advertise","cost":"11"}}}}}},` +
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0.0.0.0","network-type":"point-to-point","cost":"100","hello-interval":"5","dead-interval":"20","priority":"0","passive":"false","mtu-ignore":"true","retransmit-interval":"6","transmit-delay":"2","authentication":{"mode":"md5","key-chain":"area-key"}}}},` +
		`"key-chains":{"area-key":{"name":"area-key","key":{"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t","send-lifetime":{"start":"2026-01-01T00:00:00Z","end":"2027-01-01T00:00:00Z"}}}}}` +
		`}}`
	cfg, err := parseOSPFConfig(ospfSec(data), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
	if got := cfg.RouterID.String(); got != "10.0.0.1" {
		t.Errorf("RouterID = %s, want 10.0.0.1", got)
	}
	if cfg.ReferenceBandwidth != 200000 || cfg.MaximumPaths != 16 {
		t.Errorf("root defaults/overrides = ref %d paths %d", cfg.ReferenceBandwidth, cfg.MaximumPaths)
	}
	if !cfg.DefaultInformation.Originate || !cfg.DefaultInformation.Always || cfg.DefaultInformation.Metric != 7 || cfg.DefaultInformation.MetricType != metricType1 {
		t.Errorf("default-information = %+v", cfg.DefaultInformation)
	}
	if cfg.Timers.SPFDelayMS != 10 || cfg.Timers.SPFHoldMS != 20 || cfg.Timers.SPFMaxHoldMS != 30 || cfg.Timers.MinLSIntervalMS != 40 || cfg.Timers.MinLSArrivalMS != 50 {
		t.Errorf("timers = %+v", cfg.Timers)
	}
	if len(cfg.Redistribute) != 1 || cfg.Redistribute[0].Source != redistributeStatic || cfg.Redistribute[0].Metric != 33 || cfg.Redistribute[0].Tag != 99 {
		t.Errorf("redistribute = %+v", cfg.Redistribute)
	}
	if len(cfg.Areas) != 1 {
		t.Fatalf("Areas = %d, want 1", len(cfg.Areas))
	}
	area := cfg.Areas[0]
	if !area.AreaID.IsBackbone() || area.AreaType != areaTypeStub || !area.NoSummary || area.DefaultCost != 3 || area.AuthKeyChain != "area-key" {
		t.Errorf("area = %+v", area)
	}
	if len(area.Ranges) != 1 || area.Ranges[0].Advertise || area.Ranges[0].Cost != 11 || !area.Ranges[0].HasCost {
		t.Errorf("ranges = %+v", area.Ranges)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("Interfaces = %d, want 1", len(cfg.Interfaces))
	}
	ic := cfg.Interfaces[0]
	if ic.Name != "eth0" || ic.NetworkType != networkPointToPoint || ic.Cost != 100 || !ic.HasCost || ic.HelloInterval != 5 || ic.DeadInterval != 20 || ic.Priority != 0 || ic.Passive || !ic.MTUIgnore || ic.RetransmitInterval != 6 || ic.TransmitDelay != 2 {
		t.Errorf("interface = %+v", ic)
	}
	if ic.Authentication.Mode != authAlgorithmMD5 || ic.Authentication.KeyChain != "area-key" {
		t.Errorf("auth = %+v", ic.Authentication)
	}
	if len(cfg.KeyChains) != 1 || len(cfg.KeyChains[0].Keys) != 1 {
		t.Fatalf("key-chains = %+v", cfg.KeyChains)
	}
	key := cfg.KeyChains[0].Keys[0]
	if key.KeyID != 1 || key.Algorithm != "hmac-sha-256" || key.Secret != "s3cr3t" || key.SendLifetime.Start == "" {
		t.Errorf("key = %+v", key)
	}
}

func TestOSPFConfigDefaults(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.ReferenceBandwidth != DefaultReferenceBandwidth || cfg.MaximumPaths != DefaultMaximumPaths {
		t.Errorf("root defaults = ref %d paths %d", cfg.ReferenceBandwidth, cfg.MaximumPaths)
	}
	if cfg.DefaultInformation.Metric != DefaultDefaultMetric || cfg.DefaultInformation.MetricType != metricType2 {
		t.Errorf("default-information defaults = %+v", cfg.DefaultInformation)
	}
	if cfg.Timers.SPFDelayMS != DefaultSPFDelayMS || cfg.Timers.SPFHoldMS != DefaultSPFHoldMS || cfg.Timers.SPFMaxHoldMS != DefaultSPFMaxHoldMS || cfg.Timers.MinLSIntervalMS != DefaultMinLSIntervalMS || cfg.Timers.MinLSArrivalMS != DefaultMinLSArrivalMS {
		t.Errorf("timer defaults = %+v", cfg.Timers)
	}
	if len(cfg.Areas) != 1 || cfg.Areas[0].AreaType != areaTypeNormal || cfg.Areas[0].DefaultCost != DefaultAreaCost {
		t.Errorf("area defaults = %+v", cfg.Areas)
	}
	ic := cfg.Interfaces[0]
	if !ic.Enabled || ic.Passive || ic.NetworkType != networkBroadcast || ic.HelloInterval != DefaultHelloInterval || ic.DeadInterval != DefaultDeadInterval || ic.Priority != DefaultPriority || ic.RetransmitInterval != DefaultRetransmitInterval || ic.TransmitDelay != DefaultTransmitDelay || ic.Authentication.Mode != authModeInherit {
		t.Errorf("interface defaults = %+v", ic)
	}
}

func TestOSPFConfigValidate(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(configured): %v", err)
	}

	noAddr, err := parseOSPFConfig(ospfSec(`{"ospf":{"areas":{"area":{"0":{"area-id":"0"}}}}}`), staticRouterIDSource{})
	if err != nil {
		t.Fatalf("parseOSPFConfig(no addr): %v", err)
	}
	if err := validateConfig(noAddr); !errors.Is(err, ErrRouterIDRequired) {
		t.Fatalf("validateConfig(no router-id) = %v, want ErrRouterIDRequired", err)
	}

	withLoop, err := parseOSPFConfig(ospfSec(`{"ospf":{"areas":{"area":{"0":{"area-id":"0"}}}}}`), staticRouterIDSource{
		{Name: "eth0", Type: "ethernet", Addresses: []iface.AddrInfo{{Address: "203.0.113.10", Family: "ipv4"}}},
		{Name: "lo", Type: "loopback", Addresses: []iface.AddrInfo{{Address: "10.0.0.9", Family: "ipv4"}}},
	})
	if err != nil {
		t.Fatalf("parseOSPFConfig(loop): %v", err)
	}
	if got := withLoop.RouterID.String(); got != "10.0.0.9" {
		t.Errorf("derived loopback RouterID = %s, want 10.0.0.9", got)
	}

	withoutLoop, ok := deriveRouterIDFromInterfaces([]iface.InterfaceInfo{
		{Name: "eth0", Addresses: []iface.AddrInfo{{Address: "192.0.2.1", Family: "ipv4"}}},
		{Name: "eth1", Addresses: []iface.AddrInfo{{Address: "198.51.100.8", Family: "ipv4"}}},
	})
	if !ok || withoutLoop.String() != "198.51.100.8" {
		t.Errorf("highest non-loopback RouterID = %s ok=%v, want 198.51.100.8 true", withoutLoop.String(), ok)
	}

	badArea, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"1"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(bad area): %v", err)
	}
	if err := validateConfig(badArea); !errors.Is(err, ErrUndeclaredArea) {
		t.Fatalf("validateConfig(undeclared area) = %v, want ErrUndeclaredArea", err)
	}

	duplicateArea, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"},"0.0.0.0":{"area-id":"0.0.0.0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(duplicate area): %v", err)
	}
	if err := validateConfig(duplicateArea); !errors.Is(err, ErrDuplicateArea) {
		t.Fatalf("validateConfig(duplicate area) = %v, want ErrDuplicateArea", err)
	}

	if _, err := parseRange(listEntry{key: "2001:db8::/32", data: map[string]any{}}); !errors.Is(err, ErrNonIPv4Range) {
		t.Fatalf("parseRange(IPv6) = %v, want ErrNonIPv4Range", err)
	}
}

func TestOSPFConfigBoundaries(t *testing.T) {
	src, err := os.ReadFile("yang/ze-ospf-conf.yang")
	if err != nil {
		t.Fatalf("read yang: %v", err)
	}
	yang := string(src)
	for _, decl := range []string{
		`range "1..65535"`,
		`range "0..255"`,
		`range "1..3600"`, // transmit-delay: RFC 2328 InfTransDelay must be > 0 (AC-7)
		`range "1..32"`,
		`range "0..16777215"`,
		`range "1..4294967"`,
	} {
		if !strings.Contains(yang, decl) {
			t.Errorf("YANG missing %s", decl)
		}
	}
	for _, decl := range []string{"default 100000", "default 8", "default 10", "default 40", "default 1", "default 5"} {
		if !strings.Contains(yang, decl) {
			t.Errorf("YANG missing %q", decl)
		}
	}
	if DefaultReferenceBandwidth != 100000 || DefaultMaximumPaths != 8 || DefaultHelloInterval != 10 || DefaultDeadInterval != 40 || DefaultPriority != 1 || DefaultRetransmitInterval != 5 || DefaultTransmitDelay != 1 {
		t.Errorf("Go defaults drifted from YANG")
	}
}

func TestOSPFInterfaceEnrolment(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","enabled":"true","passive":"false"},"eth1":{"area":"0","enabled":"true","passive":"true"},"lo":{"area":"0","enabled":"true","network-type":"loopback"},"eth2":{"area":"0","enabled":"false","passive":"false"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	enrolled := cfg.enrolledInterfaces()
	if len(enrolled) != 3 || enrolled[0].Name != "eth0" || enrolled[1].Name != "eth1" || enrolled[2].Name != "lo" {
		t.Fatalf("enrolledInterfaces = %+v, want eth0, passive eth1, and loopback lo", enrolled)
	}
	active := cfg.activeInterfaces()
	if len(active) != 1 || active[0].Name != "eth0" {
		t.Fatalf("activeInterfaces = %+v, want only eth0", active)
	}
	areas := newAreas(cfg)
	backbone, _ := types.ParseAreaID("0")
	if got := len(areas[backbone].interfaces); got != 3 {
		t.Fatalf("area interfaces = %d, want 3 enrolled interfaces", got)
	}
}

// rolloverCfg builds a present, valid-router-id config carrying one key chain whose
// keys have the given send-lifetimes, for exercising validateKeyRollover via validateConfig.
func rolloverCfg(keys ...keyConfig) ospfConfig {
	return ospfConfig{
		present:   true,
		RouterID:  ridOf("1.1.1.1"),
		KeyChains: []keyChainConfig{{Name: "kc1", Keys: keys}},
	}
}

// TestKeyRolloverOverlapAccepted drives AC-17: a chain whose successive send-lifetimes
// overlap (key 2 starts at or before key 1 ends) has no signing gap and validates.
func TestKeyRolloverOverlapAccepted(t *testing.T) {
	cfg := rolloverCfg(
		keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "a", SendLifetime: lifetimeConfig{Start: "2026-01-01T00:00:00Z", End: "2026-06-01T00:00:00Z"}},
		keyConfig{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "b", SendLifetime: lifetimeConfig{Start: "2026-05-01T00:00:00Z", End: "2026-12-01T00:00:00Z"}},
	)
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("overlapping send-lifetimes must validate, got %v", err)
	}
}

// TestKeyRolloverGapRejected drives AC-17: a chain with a coverage gap between
// key 1's send-end and key 2's send-start is rejected with ErrKeyRolloverGap.
func TestKeyRolloverGapRejected(t *testing.T) {
	cfg := rolloverCfg(
		keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "a", SendLifetime: lifetimeConfig{Start: "2026-01-01T00:00:00Z", End: "2026-02-01T00:00:00Z"}},
		keyConfig{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "b", SendLifetime: lifetimeConfig{Start: "2026-03-01T00:00:00Z", End: "2026-04-01T00:00:00Z"}},
	)
	if err := validateConfig(cfg); !errors.Is(err, ErrKeyRolloverGap) {
		t.Fatalf("send-lifetime gap = %v, want ErrKeyRolloverGap", err)
	}
}

// TestKeyRolloverUnsetLifetimeNoGap confirms the unset-lifetime default: keys without
// a send-lifetime are unbounded and never create a rollover gap.
func TestKeyRolloverUnsetLifetimeNoGap(t *testing.T) {
	cfg := rolloverCfg(
		keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "a"},
		keyConfig{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "b"},
	)
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("unset lifetimes must validate (always-valid), got %v", err)
	}
}

// TestKeyLifetimeBadFormatRejected drives AC-17: a non-RFC3339 lifetime timestamp is
// rejected at validation rather than silently ignored.
func TestKeyLifetimeBadFormatRejected(t *testing.T) {
	cfg := rolloverCfg(
		keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "a", SendLifetime: lifetimeConfig{Start: "not-a-timestamp"}},
	)
	if err := validateConfig(cfg); !errors.Is(err, ErrKeyLifetimeFormat) {
		t.Fatalf("bad lifetime = %v, want ErrKeyLifetimeFormat", err)
	}
}

// TestSimplePasswordLengthBoundary drives RFC 2328 App D: the AuType 1 authentication field
// is 8 octets, so an 8-octet simple-password secret is the maximum (accepted) and a 9-octet
// one is rejected rather than silently truncated.
func TestSimplePasswordLengthBoundary(t *testing.T) {
	if err := validateConfig(rolloverCfg(keyConfig{KeyID: 1, Algorithm: "simple", Secret: "12345678"})); err != nil {
		t.Fatalf("8-octet simple password must validate, got %v", err)
	}
	if err := validateConfig(rolloverCfg(keyConfig{KeyID: 1, Algorithm: "simple", Secret: "123456789"})); !errors.Is(err, ErrSimplePasswordLen) {
		t.Fatalf("9-octet simple password = %v, want ErrSimplePasswordLen", err)
	}
}

// TestParseAddressFamilyV6 drives the OSPFv3 config surface: `ospf { address-family ipv6 { ... } }`
// parses into cfg.V6 with its own areas/interfaces + Instance ID, inherits the parent Router ID,
// and the dual-family config validates.
func TestParseAddressFamilyV6(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}},"address-family":{"ipv6":{"instance-id":7,"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth1":{"area":"0","network-type":"point-to-point"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil {
		t.Fatal("address-family ipv6 was not parsed into cfg.V6")
	}
	if !cfg.V6.present {
		t.Error("cfg.V6.present = false, want true")
	}
	if cfg.V6.InstanceID != 7 {
		t.Errorf("cfg.V6.InstanceID = %d, want 7", cfg.V6.InstanceID)
	}
	if cfg.V6.RouterID != ridOf("10.0.0.1") {
		t.Errorf("cfg.V6.RouterID = %v, want inherited 10.0.0.1", cfg.V6.RouterID)
	}
	if len(cfg.V6.Interfaces) != 1 || cfg.V6.Interfaces[0].Name != "eth1" {
		t.Errorf("cfg.V6.Interfaces = %+v, want [eth1]", cfg.V6.Interfaces)
	}
	if len(cfg.V6.Areas) != 1 {
		t.Errorf("cfg.V6.Areas = %d, want 1", len(cfg.V6.Areas))
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(dual-family): %v", err)
	}
}
