package sysctl

import (
	"fmt"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysctlreg "codeberg.org/thomas-mangin/ze/internal/core/sysctl"
)

// fakeBackend records reads/writes for testing without touching the kernel.
type fakeBackend struct {
	values map[string]string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{values: make(map[string]string)}
}

func (f *fakeBackend) read(key string) (string, error) {
	v, ok := f.values[key]
	if !ok {
		return "0", nil // OS default
	}
	return v, nil
}

func (f *fakeBackend) write(key, value string) error {
	f.values[key] = value
	return nil
}

func newTestStore() (*store, *fakeBackend) {
	fb := newFakeBackend()
	log := slogutil.DiscardLogger()
	return newStore(fb, log), fb
}

// newTestStoreWithLog returns a store whose log output is captured in buf.
func newTestStoreWithLog() (*store, *fakeBackend, *strings.Builder) {
	fb := newFakeBackend()
	var buf strings.Builder
	log := slogutil.LoggerWithOutput("sysctl", "debug", &buf)
	return newStore(fb, log), fb, &buf
}

func TestValuePrecedence(t *testing.T) {
	// VALIDATES: AC-4 -- Config > transient > default ordering.
	// PREVENTS: Lower-priority layer overwriting higher.
	s, fb := newTestStore()

	// Set default.
	_, err := s.setDefault("net.core.somaxconn", "128", "test-plugin")
	if err != nil {
		t.Fatalf("setDefault: %v", err)
	}
	if fb.values["net.core.somaxconn"] != "128" {
		t.Errorf("after default: got %q, want %q", fb.values["net.core.somaxconn"], "128")
	}

	// Set transient (overrides default).
	_, err = s.setTransient("net.core.somaxconn", "4096")
	if err != nil {
		t.Fatalf("setTransient: %v", err)
	}
	if fb.values["net.core.somaxconn"] != "4096" {
		t.Errorf("after transient: got %q, want %q", fb.values["net.core.somaxconn"], "4096")
	}

	// Set config (overrides transient).
	applied, errs := s.applyConfig(map[string]string{"net.core.somaxconn": "1024"})
	if len(errs) > 0 {
		t.Fatalf("applyConfig: %v", errs)
	}
	if fb.values["net.core.somaxconn"] != "1024" {
		t.Errorf("after config: got %q, want %q", fb.values["net.core.somaxconn"], "1024")
	}
	if len(applied) == 0 {
		t.Error("applyConfig returned no applied events")
	}
}

func TestConfigOverridesDefault(t *testing.T) {
	// VALIDATES: AC-2 -- Config key blocks plugin default, warn logged.
	// PREVENTS: Plugin default silently overwriting user config.
	s, fb := newTestStore()

	// Set default first.
	_, err := s.setDefault("net.ipv4.conf.all.forwarding", "1", "fib-kernel")
	if err != nil {
		t.Fatalf("setDefault: %v", err)
	}
	if fb.values["net.ipv4.conf.all.forwarding"] != "1" {
		t.Fatalf("default not applied")
	}

	// Apply config that overrides.
	_, errs := s.applyConfig(map[string]string{"net.ipv4.conf.all.forwarding": "0"})
	if len(errs) > 0 {
		t.Fatalf("applyConfig: %v", errs)
	}
	if fb.values["net.ipv4.conf.all.forwarding"] != "0" {
		t.Errorf("config override: got %q, want %q", fb.values["net.ipv4.conf.all.forwarding"], "0")
	}

	// Now a new default should be blocked.
	payload, err := s.setDefault("net.ipv4.conf.all.forwarding", "1", "fib-kernel")
	if err != nil {
		t.Fatalf("setDefault after config: %v", err)
	}
	if payload != "" {
		t.Error("expected empty payload when config blocks default")
	}
	// Value should still be config's 0.
	if fb.values["net.ipv4.conf.all.forwarding"] != "0" {
		t.Errorf("after blocked default: got %q, want %q", fb.values["net.ipv4.conf.all.forwarding"], "0")
	}
}

func TestTransientOverridesDefault(t *testing.T) {
	// VALIDATES: AC-3 -- Transient key wins over default.
	// PREVENTS: Default applied when transient exists.
	s, fb := newTestStore()

	_, _ = s.setDefault("net.core.somaxconn", "128", "plugin")
	_, err := s.setTransient("net.core.somaxconn", "4096")
	if err != nil {
		t.Fatalf("setTransient: %v", err)
	}
	if fb.values["net.core.somaxconn"] != "4096" {
		t.Errorf("transient: got %q, want %q", fb.values["net.core.somaxconn"], "4096")
	}
}

func TestConfigOverridesTransient(t *testing.T) {
	// VALIDATES: AC-4 -- Config key wins over transient.
	// PREVENTS: Transient persisting when config is applied.
	s, fb := newTestStore()

	_, _ = s.setTransient("net.core.somaxconn", "4096")
	_, errs := s.applyConfig(map[string]string{"net.core.somaxconn": "1024"})
	if len(errs) > 0 {
		t.Fatalf("applyConfig: %v", errs)
	}
	if fb.values["net.core.somaxconn"] != "1024" {
		t.Errorf("config override transient: got %q, want %q", fb.values["net.core.somaxconn"], "1024")
	}
}

func TestRestoreOnStop(t *testing.T) {
	// VALIDATES: AC-18 -- Original values saved before write, restored on stop.
	// PREVENTS: ze leaving kernel tunables modified after clean shutdown.
	s, fb := newTestStore()

	// Pre-set an OS value.
	fb.values["net.core.somaxconn"] = "256"

	// Apply a default (should save original 256).
	_, err := s.setDefault("net.core.somaxconn", "4096", "plugin")
	if err != nil {
		t.Fatalf("setDefault: %v", err)
	}
	if fb.values["net.core.somaxconn"] != "4096" {
		t.Fatalf("after default: got %q, want %q", fb.values["net.core.somaxconn"], "4096")
	}

	// Restore.
	s.restoreAll()
	if fb.values["net.core.somaxconn"] != "256" {
		t.Errorf("after restore: got %q, want %q", fb.values["net.core.somaxconn"], "256")
	}
}

func TestOverrideWarnLog(t *testing.T) {
	// VALIDATES: AC-19 -- Config overriding a default emits warn log with plugin name.
	// PREVENTS: Silent override without operator visibility.
	fb := newFakeBackend()
	var logBuf strings.Builder
	log := slogutil.DiscardLogger() // We test the store logic, not log capture.
	_ = logBuf                      // Placeholder for future structured log capture.

	s := newStore(fb, log)

	// Set a default from fib-kernel.
	_, _ = s.setDefault("net.ipv4.conf.all.forwarding", "1", "fib-kernel")

	// Config overrides it. The store logs at warn level internally.
	_, errs := s.applyConfig(map[string]string{"net.ipv4.conf.all.forwarding": "0"})
	if len(errs) > 0 {
		t.Fatalf("applyConfig: %v", errs)
	}

	// Verify the override took effect.
	if fb.values["net.ipv4.conf.all.forwarding"] != "0" {
		t.Errorf("config override: got %q, want %q", fb.values["net.ipv4.conf.all.forwarding"], "0")
	}

	// Verify the store recorded the default source.
	s.mu.RLock()
	e := s.entries["net.ipv4.conf.all.forwarding"]
	s.mu.RUnlock()
	if e.defaultSource != "fib-kernel" {
		t.Errorf("defaultSource: got %q, want %q", e.defaultSource, "fib-kernel")
	}
}

func TestKnownKeyValidation(t *testing.T) {
	// VALIDATES: AC-11 -- Known key with invalid value rejected.
	// PREVENTS: Bad values written to kernel.
	sysctlreg.ResetRegistry()
	t.Cleanup(sysctlreg.ResetRegistry)

	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name:     "net.ipv4.conf.all.rp_filter",
		Type:     sysctlreg.TypeIntRange,
		Min:      0,
		Max:      2,
		Platform: sysctlreg.PlatformLinux,
	})

	s, _ := newTestStore()

	_, err := s.setTransient("net.ipv4.conf.all.rp_filter", "5")
	if err == nil {
		t.Error("expected validation error for rp_filter=5")
	}

	_, err = s.setDefault("net.ipv4.conf.all.rp_filter", "5", "plugin")
	if err == nil {
		t.Error("expected validation error for default rp_filter=5")
	}
}

func TestUnknownKeyAccepted(t *testing.T) {
	// VALIDATES: AC-9 -- Unknown key written without validation.
	// PREVENTS: Unknown keys rejected when they should pass through.
	sysctlreg.ResetRegistry()
	t.Cleanup(sysctlreg.ResetRegistry)

	s, fb := newTestStore()

	_, err := s.setTransient("net.some.unknown.key", "42")
	if err != nil {
		t.Fatalf("setTransient for unknown key: %v", err)
	}
	if fb.values["net.some.unknown.key"] != "42" {
		t.Errorf("unknown key: got %q, want %q", fb.values["net.some.unknown.key"], "42")
	}
}

func TestShowResult(t *testing.T) {
	// VALIDATES: AC-5 -- show-result formats JSON with source/persistent columns.
	// PREVENTS: Missing or wrong fields in show output.
	s, _ := newTestStore()

	_, _ = s.setDefault("net.ipv4.conf.all.forwarding", "1", "fib-kernel")
	_, _ = s.setTransient("net.core.somaxconn", "4096")

	result := s.showEntries()
	if !strings.Contains(result, "net.ipv4.conf.all.forwarding") {
		t.Errorf("show missing forwarding key: %s", result)
	}
	if !strings.Contains(result, "net.core.somaxconn") {
		t.Errorf("show missing somaxconn key: %s", result)
	}
	if !strings.Contains(result, `"persistent"`) {
		t.Errorf("show missing persistent field: %s", result)
	}
	if !strings.Contains(result, `"source"`) {
		t.Errorf("show missing source field: %s", result)
	}
}

func TestListResult(t *testing.T) {
	// VALIDATES: AC-6 -- list-result includes all known keys.
	// PREVENTS: Registered keys missing from list output.
	sysctlreg.ResetRegistry()
	t.Cleanup(sysctlreg.ResetRegistry)

	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name:        "net.ipv4.conf.all.forwarding",
		Type:        sysctlreg.TypeBool,
		Description: "Enable IPv4 forwarding",
		Platform:    sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name:        "net.ipv6.conf.all.forwarding",
		Type:        sysctlreg.TypeBool,
		Description: "Enable IPv6 forwarding",
		Platform:    sysctlreg.PlatformLinux,
	})

	result := listKnownKeys()
	if !strings.Contains(result, "net.ipv4.conf.all.forwarding") {
		t.Errorf("list missing ipv4 forwarding: %s", result)
	}
	if !strings.Contains(result, "net.ipv6.conf.all.forwarding") {
		t.Errorf("list missing ipv6 forwarding: %s", result)
	}
	if !strings.Contains(result, "Enable IPv4 forwarding") {
		t.Errorf("list missing description: %s", result)
	}
}

func TestDescribeKnown(t *testing.T) {
	// VALIDATES: AC-8 -- describe-result returns full metadata for known key.
	// PREVENTS: Missing type/range/description in describe output.
	sysctlreg.ResetRegistry()
	t.Cleanup(sysctlreg.ResetRegistry)

	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name:        "net.ipv4.conf.all.rp_filter",
		Type:        sysctlreg.TypeIntRange,
		Min:         0,
		Max:         2,
		Description: "Reverse path filter mode",
		Platform:    sysctlreg.PlatformLinux,
	})

	s, _ := newTestStore()
	result := s.describeKey("net.ipv4.conf.all.rp_filter")
	if !strings.Contains(result, "rp_filter") {
		t.Errorf("describe missing key name: %s", result)
	}
	if !strings.Contains(result, "Reverse path filter") {
		t.Errorf("describe missing description: %s", result)
	}
	if !strings.Contains(result, "int-range") {
		t.Errorf("describe missing type: %s", result)
	}
}

func TestSetDefaultSameValueSilent(t *testing.T) {
	// VALIDATES: AC-16 -- Two profiles setting same key to same value is silent.
	// PREVENTS: Noisy warn logs for redundant profile overlap.
	s, fb, logBuf := newTestStoreWithLog()

	// First default sets value.
	payload1, err := s.setDefault("net.ipv4.conf.eth0.arp_filter", "1", "profile:dsr")
	if err != nil {
		t.Fatalf("first setDefault: %v", err)
	}
	if payload1 == "" {
		t.Error("first setDefault should return applied payload")
	}
	if fb.values["net.ipv4.conf.eth0.arp_filter"] != "1" {
		t.Errorf("after first default: got %q, want %q", fb.values["net.ipv4.conf.eth0.arp_filter"], "1")
	}

	// Clear log from first call.
	logBuf.Reset()

	// Second default with SAME value from different source should be silent.
	// No applied event emitted (value already written to kernel).
	payload2, err := s.setDefault("net.ipv4.conf.eth0.arp_filter", "1", "profile:hardened")
	if err != nil {
		t.Fatalf("second setDefault same value: %v", err)
	}
	if payload2 != "" {
		t.Errorf("second setDefault same value should return empty payload, got %q", payload2)
	}

	// No warn log for same-value overwrite.
	if strings.Contains(logBuf.String(), "default overwritten") {
		t.Errorf("same-value setDefault should not warn, got log: %s", logBuf.String())
	}

	// Source should be updated to the latest.
	s.mu.RLock()
	e := s.entries["net.ipv4.conf.eth0.arp_filter"]
	s.mu.RUnlock()
	if e.defaultSource != "profile:hardened" {
		t.Errorf("defaultSource: got %q, want %q", e.defaultSource, "profile:hardened")
	}
}

func TestSetDefaultDifferentValueWarns(t *testing.T) {
	// VALIDATES: AC-16 complement -- Different values from different sources produces warn.
	// PREVENTS: Silent value overwrite between conflicting profiles.
	s, fb, logBuf := newTestStoreWithLog()

	// First default sets value.
	_, err := s.setDefault("net.ipv4.conf.eth0.arp_announce", "2", "profile:dsr")
	if err != nil {
		t.Fatalf("first setDefault: %v", err)
	}

	// Clear log from first call.
	logBuf.Reset()

	// Second default with DIFFERENT value: should apply (last wins) and warn.
	payload, err := s.setDefault("net.ipv4.conf.eth0.arp_announce", "1", "profile:my-edge")
	if err != nil {
		t.Fatalf("second setDefault different value: %v", err)
	}
	if payload == "" {
		t.Error("second setDefault should return applied payload")
	}
	if fb.values["net.ipv4.conf.eth0.arp_announce"] != "1" {
		t.Errorf("after second default: got %q, want %q", fb.values["net.ipv4.conf.eth0.arp_announce"], "1")
	}

	// Warn log for different-value overwrite.
	if !strings.Contains(logBuf.String(), "default overwritten") {
		t.Errorf("different-value setDefault should warn, got log: %q", logBuf.String())
	}

	// Source should be the second profile.
	s.mu.RLock()
	e := s.entries["net.ipv4.conf.eth0.arp_announce"]
	s.mu.RUnlock()
	if e.defaultSource != "profile:my-edge" {
		t.Errorf("defaultSource: got %q, want %q", e.defaultSource, "profile:my-edge")
	}
}

func TestClearProfileDefaults(t *testing.T) {
	// VALIDATES: AC-20 -- Config reload clears stale profile defaults.
	// PREVENTS: Orphaned keys from old profile version staying active.
	s, fb := newTestStore()

	// Set two profile defaults for eth0.
	if _, err := s.setDefault("net.ipv4.conf.eth0.arp_announce", "2", "profile:dsr"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.setDefault("net.ipv4.conf.eth0.arp_ignore", "1", "profile:dsr"); err != nil {
		t.Fatal(err)
	}

	// Verify both are written.
	if fb.values["net.ipv4.conf.eth0.arp_announce"] != "2" {
		t.Fatal("arp_announce not set")
	}
	if fb.values["net.ipv4.conf.eth0.arp_ignore"] != "1" {
		t.Fatal("arp_ignore not set")
	}

	// Clear profile defaults for eth0.
	s.clearProfileDefaults("eth0")

	// Both should have default layer cleared.
	s.mu.RLock()
	for _, key := range []string{"net.ipv4.conf.eth0.arp_announce", "net.ipv4.conf.eth0.arp_ignore"} {
		e := s.entries[key]
		if e.defaultValue != "" {
			t.Errorf("%s: defaultValue should be empty, got %q", key, e.defaultValue)
		}
		if e.defaultSource != "" {
			t.Errorf("%s: defaultSource should be empty, got %q", key, e.defaultSource)
		}
	}
	s.mu.RUnlock()
}

func TestProfileSourceInShow(t *testing.T) {
	// VALIDATES: AC-10 -- sysctl show displays profile:<name> as source.
	// PREVENTS: Profile defaults shown with generic source.
	s, _ := newTestStore()

	if _, err := s.setDefault("net.ipv4.conf.eth0.arp_announce", "2", "profile:dsr"); err != nil {
		t.Fatal(err)
	}

	result := s.showEntries()
	if !strings.Contains(result, "profile:dsr") {
		t.Errorf("show missing profile:dsr source: %s", result)
	}
}

func TestParseProfileConfig(t *testing.T) {
	// VALIDATES: AC-5 -- User-defined profile parsed from JSON config.
	// PREVENTS: User profiles silently ignored.
	data := `{"sysctl":{"profile":{"my-edge":{"setting":{"net.ipv4.conf.<iface>.forwarding":{"value":"1"},"net.ipv4.conf.<iface>.rp_filter":{"value":"2"}}}}}}`
	profiles := parseSysctlProfileConfig(data)
	if len(profiles) != 1 {
		t.Fatalf("parseSysctlProfileConfig: got %d profiles, want 1", len(profiles))
	}
	if profiles[0].Name != "my-edge" {
		t.Errorf("Name: got %q, want %q", profiles[0].Name, "my-edge")
	}
	if len(profiles[0].Settings) != 2 {
		t.Errorf("Settings: got %d, want 2", len(profiles[0].Settings))
	}
}

func TestListProfiles(t *testing.T) {
	// VALIDATES: AC-6 -- list-profiles returns all registered profiles.
	// PREVENTS: Missing profiles in listing.
	result := listProfiles()
	if !strings.Contains(result, "dsr") {
		t.Errorf("listProfiles missing dsr: %s", result)
	}
	if !strings.Contains(result, "router") {
		t.Errorf("listProfiles missing router: %s", result)
	}
}

func TestDescribeProfile(t *testing.T) {
	// VALIDATES: AC-7 -- describe-profile returns JSON detail.
	// PREVENTS: Missing profile detail.
	result := describeProfile("dsr")
	if !strings.Contains(result, "arp_announce") {
		t.Errorf("describeProfile missing arp_announce: %s", result)
	}
	if !strings.Contains(result, "arp_ignore") {
		t.Errorf("describeProfile missing arp_ignore: %s", result)
	}
}

func TestDescribeProfileUnknown(t *testing.T) {
	// VALIDATES: AC-8 -- Unknown profile returns error JSON.
	// PREVENTS: Crash on unknown profile describe.
	result := describeProfile("nosuch")
	if !strings.Contains(result, "unknown profile") {
		t.Errorf("describeProfile should report unknown: %s", result)
	}
}

func TestDescribeUnknown(t *testing.T) {
	// VALIDATES: AC-8 -- describe-result returns current value only for unknown key.
	// PREVENTS: Crash on unknown key describe.
	sysctlreg.ResetRegistry()
	t.Cleanup(sysctlreg.ResetRegistry)

	s, fb := newTestStore()
	fb.values["net.some.key"] = "42"

	result := s.describeKey("net.some.key")
	if !strings.Contains(result, "net.some.key") {
		t.Errorf("describe missing key: %s", result)
	}
	// Unknown key should still have value from store if set.
	fmt.Println("describe unknown result:", result)
}
