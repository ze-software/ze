// Design: docs/guide/vrrp.md -- proxy ARP is refused on a VRRP interface
// Related: proxyarp.go -- validateNoProxyARP, the refusal under test
// RFC: rfc/short/rfc5798.md -- RFC5798-8.1.3-1
// RFC: rfc/short/rfc9568.md -- RFC9568-8.1.3-1

package vrrp

import (
	"strings"
	"testing"
)

// proxyARPSections builds an interface section holding one ethernet unit with
// the given ipv4 and unit extras, plus an optional sysctl section.
func proxyARPSections(ipv4Extra, unitExtra, sysctl string) []configSection {
	iface := `{"interface":{"ethernet":{"eth0":{"unit":{"0":{` + unitExtra +
		`"ipv4":{` + ipv4Extra + `"vrrp":{"group":{"gw":{"vrid":"10"}}}}}}}}}}`
	out := []configSection{{Root: configRoot, Data: iface}}
	if sysctl != "" {
		out = append(out, configSection{Root: sysctlRoot, Data: sysctl})
	}
	return out
}

// TestProxyARPOnVRRPInterfaceRefused proves every config source that turns
// proxy ARP on for a device an IPv4 VRRP group uses is refused at verify.
//
// Method: one unit with an IPv4 group, and proxy ARP turned on by the ipv4
// leaf, the builtin proxy profile, a user profile from the candidate sysctl
// section, and a global sysctl setting for the device, all, and default. Each
// refusal names the group, the source, the RFC section and the fix.
func TestProxyARPOnVRRPInterfaceRefused(t *testing.T) {
	// RFC requirement: RFC5798-8.1.3-1 positive -- a VRRP IPv4 group whose interface unit turns proxy ARP on (ipv4 proxy-arp, a sysctl-profile setting proxy_arp, or a sysctl setting for the device, all or default) is refused at config verify, so ze never runs proxy ARP that would answer with a MAC other than the virtual router MAC (validateNoProxyARP proxyarp.go).
	// RFC requirement: RFC9568-8.1.3-1 positive -- a VRRP IPv4 group whose interface unit turns proxy ARP on (ipv4 proxy-arp, a sysctl-profile setting proxy_arp, or a sysctl setting for the device, all or default) is refused at config verify, so ze never runs proxy ARP that would answer with a MAC other than the virtual router MAC (validateNoProxyARP proxyarp.go).
	userProfile := `{"sysctl":{"profile":{"bridge":{"setting":{"net.ipv4.conf.<iface>.proxy_arp":{"value":"1"}}}}}}`
	cases := []struct {
		name, ipv4, unit, sysctl, source string
	}{
		{"leaf", `"proxy-arp":"true",`, "", "", "ipv4 proxy-arp true"},
		{"builtin-profile", "", `"sysctl-profile":["proxy"],`, "", "sysctl-profile proxy"},
		{"user-profile", "", `"sysctl-profile":["bridge"],`, userProfile, "sysctl-profile bridge"},
		{"device-setting", "", "", `{"sysctl":{"setting":{"net.ipv4.conf.eth0.proxy_arp":{"value":"1"}}}}`, "net.ipv4.conf.eth0.proxy_arp"},
		{"all-setting", "", "", `{"sysctl":{"setting":{"net.ipv4.conf.all.proxy_arp":{"value":"1"}}}}`, "net.ipv4.conf.all.proxy_arp"},
		{"default-setting", "", "", `{"sysctl":{"setting":{"net.ipv4.conf.default.proxy_arp":{"value":"1"}}}}`, "net.ipv4.conf.default.proxy_arp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNoProxyARP(proxyARPSections(tc.ipv4, tc.unit, tc.sysctl))
			if err == nil {
				t.Fatal("proxy ARP on a VRRP interface was accepted")
			}
			for _, want := range []string{"group gw", tc.source, "Section 8.1.3", "virtual router MAC", "turn proxy ARP off"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestNoProxyARPOnVRRPInterfaceAccepted proves the refusal is about proxy ARP
// alone: the same unit with proxy ARP off, or set by nothing, passes.
//
// Method: the leaf set false, a profile that does not touch proxy_arp, a
// global setting of 0, and a unit with no extra config each pass.
func TestNoProxyARPOnVRRPInterfaceAccepted(t *testing.T) {
	// RFC requirement: RFC5798-8.1.3-1 negative -- the same VRRP IPv4 unit with proxy ARP off (ipv4 proxy-arp false, a profile without proxy_arp, a sysctl proxy_arp setting of 0, or nothing set) is accepted at config verify (validateNoProxyARP proxyarp.go).
	// RFC requirement: RFC9568-8.1.3-1 negative -- the same VRRP IPv4 unit with proxy ARP off (ipv4 proxy-arp false, a profile without proxy_arp, a sysctl proxy_arp setting of 0, or nothing set) is accepted at config verify (validateNoProxyARP proxyarp.go).
	cases := []struct {
		name, ipv4, unit, sysctl string
	}{
		{"nothing", "", "", ""},
		{"leaf-false", `"proxy-arp":"false",`, "", ""},
		{"router-profile", "", `"sysctl-profile":["router"],`, ""},
		{"all-zero", "", "", `{"sysctl":{"setting":{"net.ipv4.conf.all.proxy_arp":{"value":"0"}}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateNoProxyARP(proxyARPSections(tc.ipv4, tc.unit, tc.sysctl)); err != nil {
				t.Errorf("a VRRP unit without proxy ARP was refused: %v", err)
			}
		})
	}
}
