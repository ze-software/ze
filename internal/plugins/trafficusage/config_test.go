// VALIDATES: traffic-usage config parsing -- enable, the interfaces keyed list
// (enabled gating plus per-interface override of track-ip/stale-timeout/
// max-entries with global fallback), the interval/stale-timeout/max-entries
// ranges, and the interface-name check.
// PREVENTS: silently dropping daemon-delivered (string) values, ignoring a
// per-interface override, or accepting an out-of-range setting.

package trafficusage

import (
	"strings"
	"testing"
	"time"
)

func TestParseConfig(t *testing.T) {
	// interfaces is a keyed list (OSPF/ISIS shape); eth2 is disabled and must be
	// excluded. Map iteration order is unspecified, so assert on the set.
	data := `{"traffic":{"usage":{"enabled":true,"interfaces":{"interface":{"eth0":{},"eth1":{},"eth2":{"enabled":false}}},"interval":2000,"stale-timeout":600000,"track-ip":true,"max-entries":4096}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Error("Enabled = false, want true")
	}
	got := map[string]bool{}
	for _, n := range cfg.Interfaces {
		got[n.Name] = true
	}
	if len(cfg.Interfaces) != 2 || !got["eth0"] || !got["eth1"] || got["eth2"] {
		t.Errorf("Interfaces = %v, want {eth0, eth1} (eth2 disabled)", cfg.Interfaces)
	}
	if cfg.Interval != 2*time.Second {
		t.Errorf("Interval = %v, want 2s", cfg.Interval)
	}
	if cfg.StaleTimeout != 10*time.Minute {
		t.Errorf("StaleTimeout = %v, want 10m", cfg.StaleTimeout)
	}
	if !cfg.TrackIP {
		t.Error("TrackIP = false, want true")
	}
	if cfg.MaxEntries != 4096 {
		t.Errorf("MaxEntries = %d, want 4096", cfg.MaxEntries)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestParseConfigDefaults(t *testing.T) {
	// Daemon delivers leaves as JSON strings; defaults must still apply for
	// leaves not present. A keyed-list entry with no body is enabled by default.
	data := `{"traffic":{"usage":{"enabled":"true","interfaces":{"interface":{"eth0":{}}}}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Error("Enabled = false, want true")
	}
	if len(cfg.Interfaces) != 1 || cfg.Interfaces[0].Name != "eth0" {
		t.Errorf("Interfaces = %v, want [eth0]", cfg.Interfaces)
	}
	if cfg.Interval != defaultInterval {
		t.Errorf("Interval = %v, want default %v", cfg.Interval, defaultInterval)
	}
	if cfg.StaleTimeout != defaultStaleTimeout {
		t.Errorf("StaleTimeout = %v, want default %v", cfg.StaleTimeout, defaultStaleTimeout)
	}
	if cfg.MaxEntries != defaultMaxEntries {
		t.Errorf("MaxEntries = %d, want default %d", cfg.MaxEntries, defaultMaxEntries)
	}
	if cfg.TrackIP {
		t.Error("TrackIP = true, want default false")
	}
}

func TestParseConfigEmpty(t *testing.T) {
	for _, data := range []string{``, `{}`, `{"traffic":{"usage":{}}}`, `{"traffic":{"usage":{"enabled":false}}}`} {
		cfg, err := ParseConfig(data)
		if err != nil {
			t.Fatalf("ParseConfig(%q): %v", data, err)
		}
		if !cfg.IsEmpty() {
			t.Errorf("ParseConfig(%q).IsEmpty() = false, want true", data)
		}
	}
}

func TestParseConfigInterfaceDisabled(t *testing.T) {
	// An interface present but enabled=false is not accounted on.
	cfg, err := ParseConfig(`{"traffic":{"usage":{"enabled":true,"interfaces":{"interface":{"eth0":{"enabled":false}}}}}}`)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.Interfaces) != 0 {
		t.Errorf("Interfaces = %v, want empty (eth0 disabled)", cfg.Interfaces)
	}
}

func TestParseConfigNoInterface(t *testing.T) {
	cfg, err := ParseConfig(`{"traffic":{"usage":{"enabled":true}}}`)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() = nil, want error for enable with no enabled interface")
	}
}

// oneIface returns a traffic-usage config JSON with a single enabled interface
// (eth0) plus the given extra leaf body.
func oneIface(extra string) string {
	return `{"traffic":{"usage":{"enabled":true,"interfaces":{"interface":{"eth0":{}}}` + extra + `}}}`
}

func TestParseConfigIntervalTooSmall(t *testing.T) {
	cfg, err := ParseConfig(oneIface(`,"interval":99`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() = nil, want error for interval 99ms < 100ms")
	}
}

func TestParseConfigIntervalBoundary(t *testing.T) {
	// 100ms is the smallest valid interval; 3600000ms (1h) the largest.
	for _, ms := range []uint32{100, 3600000} {
		cfg, err := ParseConfig(oneIface(`,"interval":` + itoa(ms)))
		if err != nil {
			t.Fatalf("ParseConfig: %v", err)
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() interval=%dms: %v, want nil", ms, err)
		}
	}
}

func TestParseConfigStaleTimeoutZeroDisables(t *testing.T) {
	cfg, err := ParseConfig(oneIface(`,"stale-timeout":0`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.StaleTimeout != 0 {
		t.Errorf("StaleTimeout = %v, want 0 (disabled)", cfg.StaleTimeout)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() stale-timeout=0: %v, want nil", err)
	}
}

func TestParseConfigMaxEntriesBounds(t *testing.T) {
	// 0 is invalid (uint32, LRU map needs >= 1 entry).
	cfg, err := ParseConfig(oneIface(`,"max-entries":0`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() = nil, want error for max-entries 0")
	}
	// 1 and 4294967295 are the boundary valid values.
	for _, n := range []uint32{1, 4294967295} {
		cfg, err := ParseConfig(oneIface(`,"max-entries":` + itoa(n)))
		if err != nil {
			t.Fatalf("ParseConfig: %v", err)
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() max-entries=%d: %v, want nil", n, err)
		}
	}
}

func TestParseConfigPerInterfaceOverride(t *testing.T) {
	// Globals: track-ip true, stale-timeout 600000ms (10m), max-entries 4096.
	// eth0 inherits all of them; eth1 overrides each.
	data := `{"traffic":{"usage":{"enabled":true,"track-ip":true,"stale-timeout":600000,"max-entries":4096,"interfaces":{"interface":{"eth0":{},"eth1":{"track-ip":false,"stale-timeout":120000,"max-entries":1024}}}}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	byName := map[string]InterfaceConfig{}
	for _, ifc := range cfg.Interfaces {
		byName[ifc.Name] = ifc
	}
	eth0, ok0 := byName["eth0"]
	eth1, ok1 := byName["eth1"]
	if !ok0 || !ok1 {
		t.Fatalf("Interfaces = %+v, want eth0 and eth1", cfg.Interfaces)
	}
	if !eth0.TrackIP || eth0.StaleTimeout != 10*time.Minute || eth0.MaxEntries != 4096 {
		t.Errorf("eth0 = %+v, want inherited globals (track-ip true, 10m, 4096)", eth0)
	}
	if eth1.TrackIP || eth1.StaleTimeout != 2*time.Minute || eth1.MaxEntries != 1024 {
		t.Errorf("eth1 = %+v, want overrides (track-ip false, 2m, 1024)", eth1)
	}
}

func TestValidInterfaceName(t *testing.T) {
	// Names are ze logical names (resolved to the OS device), so they are not
	// bound to IFNAMSIZ; up to 255 chars, leading alphanumeric.
	valid := []string{"eth0", "eth0.100", "br-lan", "wg0", "a", "wan-uplink-primary", strings.Repeat("a", 255)}
	for _, n := range valid {
		if !validInterfaceName(n) {
			t.Errorf("validInterfaceName(%q) = false, want true", n)
		}
	}
	// Empty, leading non-alphanumeric, whitespace, path separator, and >255 chars
	// are rejected.
	invalid := []string{"", "-eth0", ".eth0", "_x", "@x", "eth 0", "eth/0", strings.Repeat("a", 256)}
	for _, n := range invalid {
		if validInterfaceName(n) {
			t.Errorf("validInterfaceName(%q) = true, want false", n)
		}
	}
}

// itoa is a tiny test-only uint32 -> string to keep table literals readable.
func itoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
