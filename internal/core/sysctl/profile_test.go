package sysctl

import (
	"sort"
	"strings"
	"testing"
)

func TestMustRegisterProfile(t *testing.T) {
	// VALIDATES: AC-6 -- Profile registration and lookup.
	// PREVENTS: Profile silently lost after registration.
	ResetProfiles()
	t.Cleanup(ResetProfiles)

	MustRegisterProfile(ProfileDef{
		Name:        "test-profile",
		Description: "Test profile for unit tests",
		Settings: []ProfileSetting{
			{Key: "net.ipv4.conf.<iface>.forwarding", Value: "1"},
		},
	})

	p, ok := LookupProfile("test-profile")
	if !ok {
		t.Fatal("registered profile not found via LookupProfile")
	}
	if p.Name != "test-profile" {
		t.Errorf("Name: got %q, want %q", p.Name, "test-profile")
	}
	if len(p.Settings) != 1 {
		t.Fatalf("Settings: got %d, want 1", len(p.Settings))
	}
	if p.Settings[0].Key != "net.ipv4.conf.<iface>.forwarding" {
		t.Errorf("Settings[0].Key: got %q", p.Settings[0].Key)
	}
	if p.Settings[0].Value != "1" {
		t.Errorf("Settings[0].Value: got %q", p.Settings[0].Value)
	}
}

func TestDuplicateProfileRegistration(t *testing.T) {
	// VALIDATES: Built-in duplicate panics (programming bug).
	// PREVENTS: Silent overwrite of a profile definition.
	ResetProfiles()
	t.Cleanup(ResetProfiles)

	MustRegisterProfile(ProfileDef{
		Name:     "dup-test",
		Settings: []ProfileSetting{{Key: "k", Value: "v"}},
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate MustRegisterProfile, got nil")
		}
	}()

	MustRegisterProfile(ProfileDef{
		Name:     "dup-test",
		Settings: []ProfileSetting{{Key: "k", Value: "v2"}},
	})
}

func TestUserProfileOverridesBuiltin(t *testing.T) {
	// VALIDATES: AC-13 -- User-defined profile overrides built-in with same name.
	// PREVENTS: User profile silently ignored when built-in exists.
	ResetProfiles()
	t.Cleanup(ResetProfiles)

	MustRegisterProfile(ProfileDef{
		Name:     "override-me",
		Builtin:  true,
		Settings: []ProfileSetting{{Key: "k", Value: "builtin-val"}},
	})

	// RegisterProfile (non-Must) overwrites built-in.
	RegisterProfile(ProfileDef{
		Name:     "override-me",
		Settings: []ProfileSetting{{Key: "k", Value: "user-val"}},
	})

	p, ok := LookupProfile("override-me")
	if !ok {
		t.Fatal("profile not found after override")
	}
	if p.Settings[0].Value != "user-val" {
		t.Errorf("Settings[0].Value: got %q, want %q", p.Settings[0].Value, "user-val")
	}
	if p.Builtin {
		t.Error("overridden profile should not be marked builtin")
	}
}

func TestLookupProfileUnknown(t *testing.T) {
	// VALIDATES: AC-8 -- Unknown profile name returns false.
	// PREVENTS: False positive lookup for nonexistent profile.
	ResetProfiles()
	t.Cleanup(ResetProfiles)

	if _, ok := LookupProfile("nosuch"); ok {
		t.Error("expected LookupProfile to return false for unknown profile")
	}
}

func TestAllProfiles(t *testing.T) {
	// VALIDATES: AC-6 -- AllProfiles returns all registered profiles.
	// PREVENTS: Profiles missing from listing.
	ResetProfiles()
	t.Cleanup(ResetProfiles)

	MustRegisterProfile(ProfileDef{
		Name:     "alpha",
		Settings: []ProfileSetting{{Key: "k1", Value: "v1"}},
	})
	MustRegisterProfile(ProfileDef{
		Name:     "beta",
		Settings: []ProfileSetting{{Key: "k2", Value: "v2"}},
	})

	all := AllProfiles()
	if len(all) != 2 {
		t.Fatalf("AllProfiles: got %d, want 2", len(all))
	}

	names := []string{all[0].Name, all[1].Name}
	sort.Strings(names)
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("AllProfiles names: got %v", names)
	}
}

// resetAndReregisterBuiltins clears profiles and re-registers built-ins.
// Needed because other tests call ResetProfiles in cleanup.
func resetAndReregisterBuiltins(t *testing.T) {
	t.Helper()
	ResetProfiles()
	for _, p := range builtinProfiles {
		MustRegisterProfile(p)
	}
	t.Cleanup(ResetProfiles)
}

func TestBuiltinDSR(t *testing.T) {
	// VALIDATES: AC-1 -- DSR profile has arp_announce=2, arp_ignore=1.
	// PREVENTS: DSR profile missing or wrong values.
	resetAndReregisterBuiltins(t)
	p, ok := LookupProfile("dsr")
	if !ok {
		t.Fatal("built-in dsr profile not found")
	}
	if !p.Builtin {
		t.Error("dsr profile should be marked builtin")
	}

	want := map[string]string{
		"net.ipv4.conf.<iface>.arp_announce": "2",
		"net.ipv4.conf.<iface>.arp_ignore":   "1",
	}
	got := settingsMap(p.Settings)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("dsr %s: got %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("dsr settings count: got %d, want %d", len(got), len(want))
	}
}

func TestBuiltinRouter(t *testing.T) {
	// VALIDATES: Router profile has ipv4+ipv6 forwarding=1.
	// PREVENTS: Missing forwarding keys.
	resetAndReregisterBuiltins(t)
	p, ok := LookupProfile("router")
	if !ok {
		t.Fatal("built-in router profile not found")
	}

	want := map[string]string{
		"net.ipv4.conf.<iface>.forwarding": "1",
		"net.ipv6.conf.<iface>.forwarding": "1",
	}
	got := settingsMap(p.Settings)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("router %s: got %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("router settings count: got %d, want %d", len(got), len(want))
	}
}

func TestBuiltinHardened(t *testing.T) {
	// VALIDATES: Hardened profile has rp_filter=1, log_martians=1, arp_filter=1.
	// PREVENTS: Missing anti-spoofing keys.
	resetAndReregisterBuiltins(t)
	p, ok := LookupProfile("hardened")
	if !ok {
		t.Fatal("built-in hardened profile not found")
	}

	want := map[string]string{
		"net.ipv4.conf.<iface>.rp_filter":    "1",
		"net.ipv4.conf.<iface>.log_martians": "1",
		"net.ipv4.conf.<iface>.arp_filter":   "1",
	}
	got := settingsMap(p.Settings)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("hardened %s: got %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("hardened settings count: got %d, want %d", len(got), len(want))
	}
}

func TestBuiltinMultihomed(t *testing.T) {
	// VALIDATES: Multihomed profile has arp_filter=1.
	// PREVENTS: Missing ARP flux prevention.
	resetAndReregisterBuiltins(t)
	p, ok := LookupProfile("multihomed")
	if !ok {
		t.Fatal("built-in multihomed profile not found")
	}

	want := map[string]string{
		"net.ipv4.conf.<iface>.arp_filter": "1",
	}
	got := settingsMap(p.Settings)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("multihomed %s: got %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("multihomed settings count: got %d, want %d", len(got), len(want))
	}
}

func TestBuiltinProxy(t *testing.T) {
	// VALIDATES: Proxy profile has proxy_arp=1, arp_accept=1.
	// PREVENTS: Missing proxy ARP keys.
	resetAndReregisterBuiltins(t)
	p, ok := LookupProfile("proxy")
	if !ok {
		t.Fatal("built-in proxy profile not found")
	}

	want := map[string]string{
		"net.ipv4.conf.<iface>.proxy_arp":  "1",
		"net.ipv4.conf.<iface>.arp_accept": "1",
	}
	got := settingsMap(p.Settings)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("proxy %s: got %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("proxy settings count: got %d, want %d", len(got), len(want))
	}
}

func TestConflictRegistration(t *testing.T) {
	// VALIDATES: Conflict table registration and lookup.
	// PREVENTS: Conflict rules silently lost.
	ResetConflicts()
	t.Cleanup(ResetConflicts)

	RegisterConflict(ConflictRule{
		KeyA:   "arp_ignore",
		ValueA: "1",
		KeyB:   "proxy_arp",
		ValueB: "1",
		Reason: "test conflict",
	})

	rules := AllConflicts()
	if len(rules) != 1 {
		t.Fatalf("AllConflicts: got %d, want 1", len(rules))
	}
	if rules[0].KeyA != "arp_ignore" {
		t.Errorf("KeyA: got %q", rules[0].KeyA)
	}
}

func TestCheckConflicts(t *testing.T) {
	// VALIDATES: AC-4 -- Detects arp_ignore + proxy_arp conflict.
	// PREVENTS: Conflicting profiles applied without warning.
	ResetConflicts()
	t.Cleanup(ResetConflicts)

	RegisterConflict(ConflictRule{
		KeyA:   "arp_ignore",
		ValueA: "1",
		KeyB:   "proxy_arp",
		ValueB: "1",
		Reason: "arp_ignore contradicts proxy_arp",
	})

	// Both conflicting keys active.
	active := map[string]string{
		"net.ipv4.conf.eth0.arp_ignore": "1",
		"net.ipv4.conf.eth0.proxy_arp":  "1",
	}
	conflicts := CheckConflicts(active)
	if len(conflicts) != 1 {
		t.Fatalf("CheckConflicts: got %d, want 1", len(conflicts))
	}
	if conflicts[0].Reason != "arp_ignore contradicts proxy_arp" {
		t.Errorf("Reason: got %q", conflicts[0].Reason)
	}
}

func TestCheckConflictsNoMatch(t *testing.T) {
	// VALIDATES: No false positives for non-conflicting profiles.
	// PREVENTS: Spurious warnings for dsr + hardened.
	ResetConflicts()
	t.Cleanup(ResetConflicts)

	RegisterConflict(ConflictRule{
		KeyA:   "arp_ignore",
		ValueA: "1",
		KeyB:   "proxy_arp",
		ValueB: "1",
		Reason: "arp_ignore contradicts proxy_arp",
	})

	// DSR + hardened: no proxy_arp, so no conflict.
	active := map[string]string{
		"net.ipv4.conf.eth0.arp_ignore":   "1",
		"net.ipv4.conf.eth0.arp_announce": "2",
		"net.ipv4.conf.eth0.rp_filter":    "1",
		"net.ipv4.conf.eth0.log_martians": "1",
		"net.ipv4.conf.eth0.arp_filter":   "1",
	}
	conflicts := CheckConflicts(active)
	if len(conflicts) != 0 {
		t.Errorf("CheckConflicts: got %d conflicts, want 0: %v", len(conflicts), conflicts)
	}
}

// settingsMap converts a settings slice to a map for easier assertion.
func settingsMap(s []ProfileSetting) map[string]string {
	m := make(map[string]string, len(s))
	for _, setting := range s {
		m[setting.Key] = setting.Value
	}
	return m
}

func TestTemplateSubstitution(t *testing.T) {
	// VALIDATES: AC-11 -- <iface> replaced with actual interface name.
	// PREVENTS: Literal <iface> written to kernel.
	ResetProfiles()
	t.Cleanup(ResetProfiles)

	MustRegisterProfile(ProfileDef{
		Name: "sub-test",
		Settings: []ProfileSetting{
			{Key: "net.ipv4.conf.<iface>.forwarding", Value: "1"},
			{Key: "net.ipv4.conf.<iface>.arp_filter", Value: "1"},
		},
	})

	p, _ := LookupProfile("sub-test")
	resolved := ResolveProfileSettings(p.Settings, "eth0")

	if len(resolved) != 2 {
		t.Fatalf("resolved: got %d, want 2", len(resolved))
	}

	for _, s := range resolved {
		if strings.Contains(s.Key, "<iface>") {
			t.Errorf("unresolved template in key: %q", s.Key)
		}
		if !strings.Contains(s.Key, "eth0") {
			t.Errorf("interface name missing from key: %q", s.Key)
		}
	}
}
