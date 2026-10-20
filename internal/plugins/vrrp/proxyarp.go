// Design: docs/guide/vrrp.md -- proxy ARP is refused on a VRRP interface
// RFC: rfc/short/rfc5798.md -- RFC5798-8.1.3-1
// RFC: rfc/short/rfc9568.md -- RFC9568-8.1.3-1
// Related: groups.go -- extractGroupSpecs, the same interface walk
// Related: register.go -- verifyVRRPConfigSections, parseAndVerify and the sysctl write guard registration

package vrrp

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/configvalue"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
)

// sysctlRoot is the config root of the sysctl component. The plugin reads it
// (registry ConfigReads, never ConfigRoots) only to learn whether a setting or
// a profile turns proxy ARP on, so a config holding sysctl alone does not load
// the plugin.
const sysctlRoot = "sysctl"

// proxyARPPrefix opens every per-device IPv4 sysctl key.
const proxyARPPrefix = "net.ipv4.conf."

// proxyARPLeaves are the per-device sysctls that make Linux answer an ARP
// request for an address the device does not hold, with the device's own MAC
// (Linux limitation LL-PROXY-ARP-MAC, docs/architecture/linux-limitations.md).
//
// proxy_arp is the classic form. proxy_arp_pvlan (private VLAN proxy ARP,
// RFC 3069) answers a request for another host on the SAME device; Linux takes
// one reply branch for both. Each is on for a device when the device's value
// or the `all` value is on (Linux limitation LL-IPV4-CONF-ALL-WINS).
var proxyARPLeaves = []string{"proxy_arp", "proxy_arp_pvlan"}

// proxyARPKeyDevice names the device a proxy ARP sysctl key sets, or reports
// false when the key is not one. The device keeps a VLAN's dots
// (net.ipv4.conf.eth0.100.proxy_arp): the leaf never holds a dot, so the key
// splits at its last one. sysctl(8)'s slash spelling of the same device
// (net.ipv4.conf.eth0/100.proxy_arp) answers the dotted name.
func proxyARPKeyDevice(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, proxyARPPrefix)
	if !ok {
		return "", false
	}
	dot := strings.LastIndexByte(rest, '.')
	if dot < 1 {
		return "", false
	}
	if !slices.Contains(proxyARPLeaves, rest[dot+1:]) {
		return "", false
	}
	return strings.ReplaceAll(rest[:dot], "/", "."), true
}

// proxyARPOn reports whether a sysctl key and value turn proxy ARP on.
func proxyARPOn(key, value string) bool {
	if _, ok := proxyARPKeyDevice(key); !ok {
		return false
	}
	return strings.TrimSpace(value) != "0"
}

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
// address in the Proxy ARP message." Proxy ARP on the parent answers with the
// parent's own MAC (Linux limitation LL-PROXY-ARP-MAC,
// docs/architecture/linux-limitations.md), so a host would learn a MAC that
// does not move on failover. Answering from the virtual-MAC macvlan instead is
// untested, and the owner declined proxy ARP on a VRRP router (2026-10-20), so
// ze refuses the configuration that would turn it on, at ze config verify,
// commit and reload.
//
// Four sources turn proxy ARP on: the unit's `ipv4 proxy-arp` leaf, a
// `sysctl-profile` the unit names, a global `sysctl setting` for the unit's
// device, and a global setting for `all` or `default`, each for proxy_arp or
// proxy_arp_pvlan. The device is matched under its logical name and under the
// kernel name kernelDeviceOf answers for it, so a setting written against either
// is caught. A resolve error leaves the logical name alone: config verify judges
// the config, and must not refuse it because a NIC has not enumerated yet.
// `default` covers the virtual-MAC macvlan too: ze creates it, so it takes the
// `default` value, and `all` turns proxy ARP on for every device.
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
				source := proxyARPSource(unitCfg, unitDevices(name, vlanID, kernelDeviceOf), candidate)
				if source == "" {
					continue
				}
				return fmt.Errorf("vrrp: interface %s unit %s group %s: proxy ARP is on (%s); "+
					"RFC 5798 and RFC 9568 Section 8.1.3 require a proxy ARP reply on a VRRP router to carry the virtual router MAC, "+
					"but proxy ARP there answers with the device's own MAC (Linux limitation LL-PROXY-ARP-MAC, "+
					"docs/architecture/linux-limitations.md), so hosts would keep a MAC that does not move on failover; "+
					"turn proxy ARP off there, or run this VRRP group on an interface without proxy ARP",
					name, unit, group, source)
			}
		}
	}
	return nil
}

// unitDevices names the devices a unit runs on: its logical name first, then
// the kernel name resolve answers for the interface, when that differs.
func unitDevices(name string, vlanID uint16, resolve deviceResolver) []string {
	devices := []string{unitDeviceName(name, vlanID)}
	if resolve == nil {
		return devices
	}
	kernel, err := resolve(name)
	if err != nil {
		return devices
	}
	if kernel == "" {
		return devices
	}
	if dev := unitDeviceName(kernel, vlanID); dev != devices[0] {
		devices = append(devices, dev)
	}
	return devices
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
// devices, in the operator's terms, or "" when nothing does.
func proxyARPSource(unitCfg map[string]any, devices []string, candidate sysctlCandidate) string {
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
		for _, s := range sysctlreg.ResolveProfileSettings(settings, devices[0]) {
			if proxyARPOn(s.Key, s.Value) {
				return "sysctl-profile " + profile + " on this unit, which sets " + s.Key
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(candidate.settings)) {
		value := candidate.settings[key]
		if !proxyARPOn(key, value) {
			continue
		}
		if proxyARPTargets(key, devices) {
			return "sysctl setting " + key + " " + value
		}
	}
	return ""
}

// proxyARPTargets reports whether a proxy ARP key sets one of devices, or all
// or default, which reach every device.
func proxyARPTargets(key string, devices []string) bool {
	device, ok := proxyARPKeyDevice(key)
	if !ok {
		return false
	}
	if device == "all" {
		return true
	}
	if device == "default" {
		return true
	}
	return slices.Contains(devices, device)
}

// kernelDeviceOf answers the kernel device a logical interface name means. It is
// iface.ResolveDevice (register.go), held here so this file keeps no iface
// import; a test that needs a hardware selector replaces it and restores it.
var kernelDeviceOf deviceResolver

// liveEngine is the engine runVRRPEngine is running, or nil when the plugin
// runs none. The sysctl write guard reads it: nil means no VRRP group runs in
// this process, so no runtime write can break one, which is a real answer and
// not a missing one.
var liveEngine atomic.Pointer[engine]

// proxyARPUnit is one running IPv4 virtual router and the names of the device
// it runs on, as a runtime sysctl key may spell them.
type proxyARPUnit struct {
	Interface, Unit, Group string
	Devices                []string
}

// refuseLiveProxyARP is the sysctl write guard the plugin registers
// (register.go). A runtime `set sysctl` that turns proxy ARP on for a device a
// running IPv4 group uses, or for all or default, is refused for the reason
// config verify refuses it (validateNoProxyARP). Safe for concurrent use; it
// takes the engine lock, so it waits while an apply runs.
func refuseLiveProxyARP(key, value string) error {
	eng := liveEngine.Load()
	if eng == nil {
		return nil
	}
	return refuseProxyARPWrite(eng.proxyARPUnits(), key, value)
}

// refuseProxyARPWrite judges one runtime write against the running units.
func refuseProxyARPWrite(units []proxyARPUnit, key, value string) error {
	if !proxyARPOn(key, value) {
		return nil
	}
	for _, u := range units {
		if !proxyARPTargets(key, u.Devices) {
			continue
		}
		return fmt.Errorf("vrrp: set sysctl %s %s refused: interface %s unit %s runs VRRP group %s, "+
			"and RFC 5798 and RFC 9568 Section 8.1.3 require a proxy ARP reply on a VRRP router to carry the virtual router MAC, "+
			"but proxy ARP there answers with the device's own MAC (Linux limitation LL-PROXY-ARP-MAC, "+
			"docs/architecture/linux-limitations.md), so hosts would keep a MAC that does not move on failover; "+
			"leave proxy ARP off there, or remove the VRRP group from that interface first",
			key, value, u.Interface, u.Unit, u.Group)
	}
	return nil
}

// proxyARPUnits lists the running IPv4 virtual routers, sorted so a refusal
// names the same group on every run.
func (e *engine) proxyARPUnits() []proxyARPUnit {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]proxyARPUnit, 0, len(e.instances))
	for _, in := range e.instances {
		if u, ok := in.proxyARPUnit(); ok {
			out = append(out, u)
		}
	}
	slices.SortFunc(out, func(a, b proxyARPUnit) int {
		if c := strings.Compare(a.Interface, b.Interface); c != 0 {
			return c
		}
		if c := strings.Compare(a.Unit, b.Unit); c != 0 {
			return c
		}
		return strings.Compare(a.Group, b.Group)
	})
	return out
}

// proxyARPUnit answers this instance's unit when it is an IPv4 group: the
// logical unit device, and the kernel device apply bound it to.
func (in *instance) proxyARPUnit() (proxyARPUnit, bool) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.spec.Family != familyIPv4 {
		return proxyARPUnit{}, false
	}
	devices := []string{unitDeviceName(in.spec.Interface, in.spec.VLANID)}
	if in.spec.ParentDevice != "" && in.spec.ParentDevice != devices[0] {
		devices = append(devices, in.spec.ParentDevice)
	}
	return proxyARPUnit{Interface: in.spec.Interface, Unit: in.spec.Unit, Group: in.spec.Name, Devices: devices}, true
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
