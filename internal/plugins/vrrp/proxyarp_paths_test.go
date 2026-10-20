// Design: docs/guide/vrrp.md -- proxy ARP is refused on a VRRP interface
// Related: proxyarp.go -- validateNoProxyARP and refuseLiveProxyARP, the refusals under test
// Related: register.go -- the ConfigReads declaration and the sysctl write guard registration

package vrrp

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
)

// vlanProxyARPSections is proxyARPSections for a unit tagged VLAN 100.
func vlanProxyARPSections(sysctl string) []configSection {
	return proxyARPSections("", `"vlan-id":"100",`, sysctl)
}

// sysctlSetting renders a candidate sysctl section holding one global setting.
func sysctlSetting(key, value string) string {
	return `{"sysctl":{"setting":{"` + key + `":{"value":"` + value + `"}}}}`
}

// withKernelDevice makes kernelDeviceOf answer kernel for every logical name
// for the duration of the test.
func withKernelDevice(t *testing.T, kernel string) {
	t.Helper()
	saved := kernelDeviceOf
	kernelDeviceOf = func(string) (string, error) { return kernel, nil }
	t.Cleanup(func() { kernelDeviceOf = saved })
}

// TestProxyARPRefusalCoversEveryDeviceSpelling proves config verify refuses
// proxy ARP however the sysctl key names the VRRP unit's device: proxy_arp or
// proxy_arp_pvlan, the logical name, the kernel name the interface resolves
// to, and the VLAN device in its dotted or its slash spelling.
//
// Method: each case builds one VRRP IPv4 unit and one global sysctl setting
// and expects a refusal naming the key; kernelDeviceOf answers enp1s0 for eth0.
func TestProxyARPRefusalCoversEveryDeviceSpelling(t *testing.T) {
	withKernelDevice(t, "enp1s0")
	cases := []struct {
		name     string
		sections []configSection
		key      string
	}{
		{"pvlan-device", proxyARPSections("", "", sysctlSetting("net.ipv4.conf.eth0.proxy_arp_pvlan", "1")), "net.ipv4.conf.eth0.proxy_arp_pvlan"},
		{"pvlan-all", proxyARPSections("", "", sysctlSetting("net.ipv4.conf.all.proxy_arp_pvlan", "1")), "net.ipv4.conf.all.proxy_arp_pvlan"},
		{"kernel-name", proxyARPSections("", "", sysctlSetting("net.ipv4.conf.enp1s0.proxy_arp", "1")), "net.ipv4.conf.enp1s0.proxy_arp"},
		{"vlan-dotted", vlanProxyARPSections(sysctlSetting("net.ipv4.conf.eth0.100.proxy_arp", "1")), "net.ipv4.conf.eth0.100.proxy_arp"},
		{"vlan-slash", vlanProxyARPSections(sysctlSetting("net.ipv4.conf.eth0/100.proxy_arp", "1")), "net.ipv4.conf.eth0/100.proxy_arp"},
		{"vlan-kernel-slash", vlanProxyARPSections(sysctlSetting("net.ipv4.conf.enp1s0/100.proxy_arp_pvlan", "1")), "net.ipv4.conf.enp1s0/100.proxy_arp_pvlan"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNoProxyARP(tc.sections)
			if err == nil {
				t.Fatal("proxy ARP on a VRRP interface was accepted")
			}
			for _, want := range []string{"group gw", tc.key, "Section 8.1.3", "turn proxy ARP off"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestProxyARPRefusalLeavesOtherDevicesAlone proves the wider match did not
// widen the refusal: proxy ARP on another device, on the untagged parent of a
// VLAN unit, or set to 0 still passes.
//
// Method: the same unit shapes as the refusal test, with a setting that names
// a device the unit does not run on, or turns proxy ARP off.
func TestProxyARPRefusalLeavesOtherDevicesAlone(t *testing.T) {
	withKernelDevice(t, "enp1s0")
	cases := []struct {
		name     string
		sections []configSection
	}{
		{"other-device", proxyARPSections("", "", sysctlSetting("net.ipv4.conf.eth1.proxy_arp", "1"))},
		{"other-device-pvlan", proxyARPSections("", "", sysctlSetting("net.ipv4.conf.eth1.proxy_arp_pvlan", "1"))},
		{"vlan-parent", vlanProxyARPSections(sysctlSetting("net.ipv4.conf.eth0.proxy_arp", "1"))},
		{"pvlan-zero", proxyARPSections("", "", sysctlSetting("net.ipv4.conf.eth0.proxy_arp_pvlan", "0"))},
		{"unrelated-leaf", proxyARPSections("", "", sysctlSetting("net.ipv4.conf.eth0.proxy_arp_x", "1"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateNoProxyARP(tc.sections); err != nil {
				t.Errorf("a VRRP unit without proxy ARP was refused: %v", err)
			}
		})
	}
}

// TestRuntimeSetSysctlProxyARPRefused proves a runtime `set sysctl` that turns
// proxy ARP on for a device a running IPv4 VRRP group uses is refused through
// the guard the plugin registers, and that every other write passes.
//
// Method: install an engine holding one IPv4 group on eth0 VLAN 100 (kernel
// device enp1s0.100) and one IPv6 group on eth1 as the live engine, then ask
// sysctlreg.CheckWrite, the call the sysctl store makes before it writes.
func TestRuntimeSetSysctlProxyARPRefused(t *testing.T) {
	eng := &engine{instances: map[string]*instance{
		"v4": {spec: GroupSpec{Interface: "eth0", Unit: "0", Family: familyIPv4, Name: "gw", VLANID: 100, ParentDevice: "enp1s0.100"}},
		"v6": {spec: GroupSpec{Interface: "eth1", Unit: "0", Family: familyIPv6, Name: "gw6", ParentDevice: "eth1"}},
	}}
	liveEngine.Store(eng)
	t.Cleanup(func() { liveEngine.Store(nil) })

	for _, key := range []string{
		"net.ipv4.conf.eth0.100.proxy_arp",
		"net.ipv4.conf.eth0/100.proxy_arp",
		"net.ipv4.conf.enp1s0.100.proxy_arp_pvlan",
		"net.ipv4.conf.enp1s0/100.proxy_arp",
		"net.ipv4.conf.all.proxy_arp",
		"net.ipv4.conf.default.proxy_arp_pvlan",
	} {
		err := sysctlreg.CheckWrite(key, "1")
		if err == nil {
			t.Errorf("set sysctl %s 1 was accepted on a VRRP device", key)
			continue
		}
		for _, want := range []string{key, "group gw", "Section 8.1.3", "remove the VRRP group"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal %q does not name %q", err, want)
			}
		}
	}
	for _, write := range [][2]string{
		{"net.ipv4.conf.eth0.100.proxy_arp", "0"},
		{"net.ipv4.conf.eth1.proxy_arp", "1"},
		{"net.ipv4.conf.eth0.proxy_arp", "1"},
		{"net.ipv4.conf.eth0.100.forwarding", "1"},
	} {
		if err := sysctlreg.CheckWrite(write[0], write[1]); err != nil {
			t.Errorf("set sysctl %s %s was refused: %v", write[0], write[1], err)
		}
	}

	liveEngine.Store(nil)
	if err := sysctlreg.CheckWrite("net.ipv4.conf.all.proxy_arp", "1"); err != nil {
		t.Errorf("with no VRRP engine running, a proxy ARP write was refused: %v", err)
	}
}

// TestVRRPReadsSysctlWithoutOwningIt proves the plugin receives the sysctl
// section on every config path without loading on sysctl config alone.
//
// Method: the registry row lists sysctl among the delivered roots
// (ConfigSectionRoots, which joinConfigReads hands the daemon's commit and
// reload) and not among the roots auto-loading reads (ConfigRootsMap).
func TestVRRPReadsSysctlWithoutOwningIt(t *testing.T) {
	reg := registry.Lookup(pluginName)
	if reg == nil {
		t.Fatal("vrrp is not registered")
	}
	if !slices.Contains(reg.ConfigSectionRoots(), sysctlRoot) {
		t.Errorf("vrrp does not receive the sysctl section: %v", reg.ConfigSectionRoots())
	}
	if slices.Contains(registry.ConfigRootsMap()[pluginName], sysctlRoot) {
		t.Error("a config holding only sysctl would auto-load vrrp")
	}
}
