package geodns

import (
	"net/netip"
	"strings"
	"testing"
)

const fullConfig = `{
	"service": {
		"geodns": {
			"enabled": "true",
			"listener": {
				"v4": { "ip": "127.0.0.1", "port": "5300" },
				"v6": { "ip": "::1", "port": "5300" }
			},
			"default-ttl": "300",
			"client-ip-source": "edns0-then-packet",
			"zone": ["geodns.example."],
			"nameserver": ["82.219.4.22"],
			"soa": { "contact": "hostmaster", "serial-mode": "auto-epoch", "refresh": "3600" },
			"host-set": {
				"internal": { "host": { "proxy.geodns.example.": { "address": ["10.0.0.1"] } } },
				"external": { "host": { "proxy.geodns.example.": { "address": ["10.0.0.2"] } } }
			},
			"source": {
				"82.219.0.0/16": { "host-set": "internal" },
				"0.0.0.0/0": { "host-set": "external" }
			}
		}
	}
}`

// VALIDATES: parseConfig maps every YANG leaf to its struct field and types.
// PREVENTS: a silently-dropped or mistyped setting reaching the resolver.
func TestParseConfig(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig(fullConfig)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected enabled=true")
	}
	if len(cfg.Listeners) != 2 {
		t.Fatalf("listeners = %v, want 2 entries", cfg.Listeners)
	}
	// Listeners are sorted by name (v4, v6).
	if cfg.Listeners[0].IP != netip.MustParseAddr("127.0.0.1") || cfg.Listeners[0].Port != 5300 {
		t.Errorf("listener[0] = %v, want 127.0.0.1:5300", cfg.Listeners[0])
	}
	if cfg.Listeners[1].IP != netip.MustParseAddr("::1") || cfg.Listeners[1].Port != 5300 {
		t.Errorf("listener[1] = %v, want [::1]:5300", cfg.Listeners[1])
	}
	if cfg.DefaultTTL != 300 {
		t.Errorf("default-ttl = %d, want 300", cfg.DefaultTTL)
	}
	if cfg.ClientIPSource != "edns0-then-packet" {
		t.Errorf("client-ip-source = %q", cfg.ClientIPSource)
	}
	if len(cfg.Zones) != 1 || cfg.Zones[0] != "geodns.example." {
		t.Errorf("zones = %v", cfg.Zones)
	}
	if len(cfg.Nameservers) != 1 || cfg.Nameservers[0] != netip.MustParseAddr("82.219.4.22") {
		t.Errorf("nameservers = %v", cfg.Nameservers)
	}
	if cfg.SOA.Contact != "hostmaster" || cfg.SOA.SerialMode != "auto-epoch" || cfg.SOA.Refresh != 3600 {
		t.Errorf("soa = %+v", cfg.SOA)
	}
	if len(cfg.HostSets) != 2 {
		t.Fatalf("host-sets = %d, want 2", len(cfg.HostSets))
	}
}

// VALIDATES: several source prefixes can reference one shared host-set.
// PREVENTS: regressing to per-prefix duplicated records (the reference's
// "internal" maps many prefixes to one set).
func TestParseConfigSharedHostSet(t *testing.T) {
	t.Parallel()
	data := `{"service":{"geodns":{
		"enabled":"true","zone":["g.example."],"nameserver":["10.0.0.9"],
		"host-set":{"shared":{"host":{"a.g.example.":{"address":["10.0.0.1"]}}}},
		"source":{"82.219.0.0/16":{"host-set":"shared"},"10.0.0.0/8":{"host-set":"shared"},"0.0.0.0/0":{"host-set":"shared"}}
	}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.Sources) != 3 {
		t.Fatalf("sources = %d, want 3", len(cfg.Sources))
	}
	for _, s := range cfg.Sources {
		if s.HostSet != "shared" {
			t.Errorf("source %v references %q, want shared", s.Prefix, s.HostSet)
		}
	}
}

// VALIDATES: no listener configured defaults to 127.0.0.1:5300 and ::1:5300.
// PREVENTS: a daemon that binds nothing when the operator omits the listener.
func TestParseConfigDefaultListeners(t *testing.T) {
	t.Parallel()
	data := `{"service":{"geodns":{"enabled":"true","zone":["g.example."],"nameserver":["10.0.0.9"],
		"host-set":{"external":{"host":{"a.g.example.":{"address":["10.0.0.1"]}}}},
		"source":{"0.0.0.0/0":{"host-set":"external"}}}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.Listeners) != 2 {
		t.Fatalf("default listeners = %v, want 127.0.0.1:5300 and [::1]:5300", cfg.Listeners)
	}
	for _, l := range cfg.Listeners {
		if l.Port != 5300 {
			t.Errorf("default listener %v port = %d, want 5300", l.IP, l.Port)
		}
	}
}

// VALIDATES: one host line with a v4 and a v6 address, no explicit type, yields
// both an A and an AAAA record (per-address auto-detection).
// PREVENTS: dual-stack names losing one family.
func TestParseConfigMixedAddresses(t *testing.T) {
	t.Parallel()
	data := `{"service":{"geodns":{"enabled":"true","zone":["g.example."],"nameserver":["10.0.0.9"],
		"host-set":{"s":{"host":{"a.g.example.":{"address":["1.2.3.4","2a02:b80::1"]}}}},
		"source":{"0.0.0.0/0":{"host-set":"s"}}}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	recs := cfg.HostSets["s"].Hosts["a.g.example."]
	var haveA, haveAAAA bool
	for _, r := range recs {
		switch r.Kind {
		case kindA:
			haveA = true
		case kindAAAA:
			haveAAAA = true
		default:
		}
	}
	if !haveA || !haveAAAA {
		t.Errorf("records = %+v, want one A and one AAAA", recs)
	}
}

// VALIDATES: every malformed input is rejected with a named error and aborts.
// PREVENTS: a bad record silently loading or crashing the resolver.
func TestParseConfigRejects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data string
		want string
	}{
		{"bad ip", `{"service":{"geodns":{"zone":["g.example."],"host-set":{"s":{"host":{"a.g.example.":{"address":["not-an-ip"]}}}},"source":{"0.0.0.0/0":{"host-set":"s"}}}}}`, "address"},
		{"ttl too big", `{"service":{"geodns":{"zone":["g.example."],"default-ttl":"2147483648"}}}`, "ttl"},
		{"default-ttl zero", `{"service":{"geodns":{"zone":["g.example."],"default-ttl":"0"}}}`, "default-ttl"},
		{"zone suffix", `{"service":{"geodns":{"zone":["g.example."],"host-set":{"s":{"host":{"a.other.":{"address":["10.0.0.1"]}}}},"source":{"0.0.0.0/0":{"host-set":"s"}}}}}`, "zone"},
		{"srv too few", `{"service":{"geodns":{"zone":["g.example."],"host-set":{"s":{"host":{"_x._tcp.g.example.":{"type":"SRV","srv":{"priority":"0"}}}}},"source":{"0.0.0.0/0":{"host-set":"s"}}}}}`, "srv"},
		{"missing host-set ref", `{"service":{"geodns":{"zone":["g.example."],"source":{"0.0.0.0/0":{"host-set":"nope"}}}}}`, "host-set"},
		{"too many nameservers", `{"service":{"geodns":{"zone":["g.example."],"nameserver":["1.1.1.1","1.1.1.2","1.1.1.3","1.1.1.4","1.1.1.5","1.1.1.6","1.1.1.7","1.1.1.8","1.1.1.9","1.1.1.10"]}}}`, "nameserver"},
		{"bad enum", `{"service":{"geodns":{"zone":["g.example."],"client-ip-source":"bogus"}}}`, "client-ip-source"},
		{"bad prefix", `{"service":{"geodns":{"zone":["g.example."],"host-set":{"s":{"host":{}}},"source":{"not-a-cidr":{"host-set":"s"}}}}}`, "prefix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseConfig(tc.data)
			if err == nil {
				t.Fatalf("expected error mentioning %q, got nil", tc.want)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}
