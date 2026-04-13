// Design: docs/architecture/core-design.md -- sysctl profile registry
// Related: known.go -- key registry that profiles reference

package sysctl

import (
	"sort"
	"strings"
	"sync"
)

// ProfileSetting is a single key/value pair within a profile.
// Keys may contain <iface> placeholders resolved at apply time.
type ProfileSetting struct {
	Key   string
	Value string
}

// ProfileDef describes a named sysctl profile: a reusable collection of
// kernel tunables applied together to an interface unit.
type ProfileDef struct {
	Name        string
	Description string
	Builtin     bool // true for built-in profiles registered at init
	Settings    []ProfileSetting
}

var (
	profileMu       sync.RWMutex
	profileRegistry = map[string]ProfileDef{}
)

// MustRegisterProfile registers a built-in profile at init time.
// Panics on duplicate registration, which indicates a programming bug.
// Follows the same pattern as MustRegister for known keys.
// Use RegisterProfile for user-defined profiles that may override built-ins.
func MustRegisterProfile(p ProfileDef) {
	profileMu.Lock()
	defer profileMu.Unlock()

	if _, exists := profileRegistry[p.Name]; exists {
		panic("BUG: sysctl duplicate profile registration: " + p.Name)
	}
	profileRegistry[p.Name] = p
}

// RegisterProfile registers a user-defined profile. Overwrites any
// existing profile with the same name (user profiles override built-ins).
func RegisterProfile(p ProfileDef) {
	profileMu.Lock()
	defer profileMu.Unlock()

	profileRegistry[p.Name] = p
}

// DeregisterProfile removes a user-defined profile. Built-in profiles
// cannot be deregistered (they are restored on next init).
func DeregisterProfile(name string) {
	profileMu.Lock()
	defer profileMu.Unlock()

	if p, ok := profileRegistry[name]; ok && !p.Builtin {
		delete(profileRegistry, name)
	}
}

// LookupProfile returns the profile definition, or false if not found.
func LookupProfile(name string) (ProfileDef, bool) {
	profileMu.RLock()
	defer profileMu.RUnlock()

	p, ok := profileRegistry[name]
	return p, ok
}

// AllProfiles returns a copy of all registered profiles, sorted by name.
func AllProfiles() []ProfileDef {
	profileMu.RLock()
	defer profileMu.RUnlock()

	result := make([]ProfileDef, 0, len(profileRegistry))
	for _, p := range profileRegistry {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ResolveProfileSettings substitutes <iface> in setting keys with the
// actual interface name. Returns a new slice; the original is unchanged.
func ResolveProfileSettings(settings []ProfileSetting, ifaceName string) []ProfileSetting {
	resolved := make([]ProfileSetting, len(settings))
	for i, s := range settings {
		resolved[i] = ProfileSetting{
			Key:   strings.ReplaceAll(s.Key, "<iface>", ifaceName),
			Value: s.Value,
		}
	}
	return resolved
}

// builtinProfiles defines the 5 built-in sysctl profiles.
// Registered at init time in profiles_register.go.
var builtinProfiles = []ProfileDef{
	{
		Name:        "dsr",
		Description: "Direct Server Return: ARP tuning for loopback VIP",
		Builtin:     true,
		Settings: []ProfileSetting{
			{Key: "net.ipv4.conf.<iface>.arp_announce", Value: "2"},
			{Key: "net.ipv4.conf.<iface>.arp_ignore", Value: "1"},
		},
	},
	{
		Name:        "router",
		Description: "Enable IPv4 and IPv6 forwarding",
		Builtin:     true,
		Settings: []ProfileSetting{
			{Key: "net.ipv4.conf.<iface>.forwarding", Value: "1"},
			{Key: "net.ipv6.conf.<iface>.forwarding", Value: "1"},
		},
	},
	{
		Name:        "hardened",
		Description: "Anti-spoofing: strict RPF, log martians, ARP filter",
		Builtin:     true,
		Settings: []ProfileSetting{
			{Key: "net.ipv4.conf.<iface>.rp_filter", Value: "1"},
			{Key: "net.ipv4.conf.<iface>.log_martians", Value: "1"},
			{Key: "net.ipv4.conf.<iface>.arp_filter", Value: "1"},
		},
	},
	{
		Name:        "multihomed",
		Description: "Prevent ARP flux on multi-NIC hosts",
		Builtin:     true,
		Settings: []ProfileSetting{
			{Key: "net.ipv4.conf.<iface>.arp_filter", Value: "1"},
		},
	},
	{
		Name:        "proxy",
		Description: "Proxy ARP: answer ARP for non-local IPs",
		Builtin:     true,
		Settings: []ProfileSetting{
			{Key: "net.ipv4.conf.<iface>.proxy_arp", Value: "1"},
			{Key: "net.ipv4.conf.<iface>.arp_accept", Value: "1"},
		},
	},
}

// ResetProfiles clears all profile registrations. Only for use in tests.
func ResetProfiles() {
	profileMu.Lock()
	defer profileMu.Unlock()
	profileRegistry = map[string]ProfileDef{}
}
