// Design: docs/guide/vrrp.md -- proxy ARP is refused on a VRRP interface
// RFC: rfc/short/rfc5798.md -- RFC5798-8.1.3-1
// RFC: rfc/short/rfc9568.md -- RFC9568-8.1.3-1
// Related: groups.go -- extractGroupSpecs, the same interface walk
// Related: register.go -- verifyVRRPConfigSections and parseAndVerify, the callers

package vrrp

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/configvalue"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
)

// sysctlRoot is the config root of the sysctl component. The plugin reads it
// only to learn whether a setting or a profile turns proxy ARP on.
const sysctlRoot = "sysctl"

// proxyARPSuffix ends every sysctl key that turns proxy ARP on for a device:
// net.ipv4.conf.<device>.proxy_arp, including the all and default pseudo-devices.
const proxyARPSuffix = ".proxy_arp"

// sysctlCandidate is what the candidate sysctl section declares: the global
// settings by key, and the user profiles by name.
type sysctlCandidate struct {
	settings map[string]string
	profiles map[string][]sysctlreg.ProfileSetting
}

// validateNoProxyARP refuses proxy ARP on any device an IPv4 VRRP group uses.
//
// RFC 5798 Section 8.1.3 and RFC 9568 Section 8.1.3: "If Proxy ARP is to be used
// on a VRRP Router, then the VRRP Router MUST advertise the Virtual Router MAC
// address in the Proxy ARP message." Linux answers a proxy ARP request with the
// replying device's own MAC and ze installs nothing that changes it, so a host
// would learn a MAC that does not move on failover. Ze cannot meet the MUST, so it
// refuses the configuration that would reach it, at ze config verify and commit.
//
// Four sources turn proxy ARP on: the unit's `ipv4 proxy-arp` leaf, a
// `sysctl-profile` the unit names, a global `sysctl setting` for the unit's
// device, and a global setting for `all` or `default`. The last covers the
// virtual-MAC macvlan too: ze creates it, so it takes the `default` value, and
// `all` turns proxy ARP on for every device.
func validateNoProxyARP(sections []configSection) error {
	candidate, err := parseSysctlCandidate(sections)
	if err != nil {
		return err
	}
	for _, s := range sections {
		if s.Root != configRoot {
			continue
		}
		tree, err := sectionTree(s)
		if err != nil {
			return err
		}
		if err := walkProxyARPUnits(tree, candidate); err != nil {
			return err
		}
	}
	return nil
}

// walkProxyARPUnits visits every unit that holds an IPv4 VRRP group, in a
// stable order so the first refusal is the same on every run.
func walkProxyARPUnits(tree map[string]any, candidate sysctlCandidate) error {
	for _, ifType := range slices.Sorted(maps.Keys(tree)) {
		byName, ok := tree[ifType].(map[string]any)
		if !ok {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(byName)) {
			ifCfg, ok := byName[name].(map[string]any)
			if !ok {
				continue
			}
			units, ok := ifCfg["unit"].(map[string]any)
			if !ok {
				continue
			}
			for _, unit := range slices.Sorted(maps.Keys(units)) {
				unitCfg, ok := units[unit].(map[string]any)
				if !ok {
					continue
				}
				group := firstIPv4Group(unitCfg)
				if group == "" {
					continue
				}
				vlanID, err := unitVLANID(name, unitCfg)
				if err != nil {
					return err
				}
				source := proxyARPSource(unitCfg, unitDeviceName(name, vlanID), candidate)
				if source == "" {
					continue
				}
				return fmt.Errorf("vrrp: interface %s unit %s group %s: proxy ARP is on (%s); "+
					"RFC 5798 and RFC 9568 Section 8.1.3 require a proxy ARP reply on a VRRP router to carry the virtual router MAC, "+
					"but the kernel answers with the device's own MAC, so hosts would keep a MAC that does not move on failover; "+
					"turn proxy ARP off there, or run this VRRP group on an interface without proxy ARP",
					name, unit, group, source)
			}
		}
	}
	return nil
}

// firstIPv4Group names the first IPv4 VRRP group of a unit, or "" when the unit
// has none. Proxy ARP answers IPv4 ARP only, so an IPv6 group is not affected.
func firstIPv4Group(unitCfg map[string]any) string {
	famCfg, ok := unitCfg[familyIPv4].(map[string]any)
	if !ok {
		return ""
	}
	vrrpCfg, ok := famCfg["vrrp"].(map[string]any)
	if !ok {
		return ""
	}
	groups, ok := vrrpCfg["group"].(map[string]any)
	if !ok {
		return ""
	}
	names := slices.Sorted(maps.Keys(groups))
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// proxyARPSource names the config that turns proxy ARP on for this unit's
// device, in the operator's terms, or "" when nothing does.
func proxyARPSource(unitCfg map[string]any, device string, candidate sysctlCandidate) string {
	if famCfg, ok := unitCfg[familyIPv4].(map[string]any); ok {
		if v, ok := famCfg["proxy-arp"].(string); ok && v == "true" {
			return "ipv4 proxy-arp true on this unit"
		}
	}
	for _, profile := range configvalue.LeafList(unitCfg["sysctl-profile"]) {
		settings, ok := candidate.profiles[profile]
		if !ok {
			def, found := sysctlreg.LookupProfile(profile)
			if !found {
				continue
			}
			settings = def.Settings
		}
		for _, s := range sysctlreg.ResolveProfileSettings(settings, device) {
			if proxyARPOn(s.Key, s.Value) {
				return "sysctl-profile " + profile + " on this unit, which sets " + s.Key
			}
		}
	}
	for _, target := range []string{device, "all", "default"} {
		key := "net.ipv4.conf." + target + proxyARPSuffix
		if v, ok := candidate.settings[key]; ok && proxyARPOn(key, v) {
			return "sysctl setting " + key + " " + v
		}
	}
	return ""
}

// proxyARPOn reports whether a sysctl key and value turn proxy ARP on.
func proxyARPOn(key, value string) bool {
	if !strings.HasSuffix(key, proxyARPSuffix) {
		return false
	}
	return strings.TrimSpace(value) != "0"
}

// parseSysctlCandidate reads the global settings and the user profiles from the
// candidate sysctl section, so a profile defined in the same commit is judged
// by its candidate content rather than by what the running daemon registered.
func parseSysctlCandidate(sections []configSection) (sysctlCandidate, error) {
	out := sysctlCandidate{settings: map[string]string{}, profiles: map[string][]sysctlreg.ProfileSetting{}}
	for _, s := range sections {
		if s.Root != sysctlRoot {
			continue
		}
		var outer map[string]any
		if err := json.Unmarshal([]byte(s.Data), &outer); err != nil {
			return out, fmt.Errorf("vrrp: decode sysctl config section: %w", err)
		}
		tree, ok := outer[sysctlRoot].(map[string]any)
		if !ok {
			tree = outer
		}
		readSettingValues(tree["setting"], func(key, value string) { out.settings[key] = value })
		profiles, ok := tree["profile"].(map[string]any)
		if !ok {
			continue
		}
		for name, raw := range profiles {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			var settings []sysctlreg.ProfileSetting
			readSettingValues(entry["setting"], func(key, value string) {
				settings = append(settings, sysctlreg.ProfileSetting{Key: key, Value: value})
			})
			out.profiles[name] = settings
		}
	}
	return out, nil
}

// readSettingValues calls add for every `setting <key> { value <v> }` entry.
func readSettingValues(raw any, add func(key, value string)) {
	settings, ok := raw.(map[string]any)
	if !ok {
		return
	}
	for key, entry := range settings {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := m["value"].(string); ok {
			add(key, v)
		}
	}
}
