package yang

import (
	"strings"
	"testing"
)

// TestVRRPCmdSchemaOwnsShowVRRP is the plugin OWNER half of the
// plugin-self-containment invariant (ai/rules/plugins.md): the
// vrrp command schema declares every `show vrrp ...` / `clear vrrp ...` node, so
// removing the vrrp plugin removes the whole command surface with it. The
// central show/clear schema packages assert the mirror image (they declare none
// of these).
//
// VALIDATES: spec-vrrp-5 -- the show/clear command tree is owned in-plugin.
// PREVENTS: a regression where a vrrp command node is declared centrally (which
// would leave a handler-less command after the plugin is removed) or dropped
// from the plugin schema (which would unwire the command).
func TestVRRPCmdSchemaOwnsShowVRRP(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:vrrp"`,
		`ze:command "ze-show:vrrp-interface"`,
		`ze:command "ze-show:vrrp-statistics"`,
		`ze:command "ze-clear:vrrp-statistics"`,
		"container vrrp",
	} {
		if !strings.Contains(ZeVrrpCmdYANG, want) {
			t.Errorf("ze-vrrp-cmd.yang must declare %q so removing vrrp removes the surface", want)
		}
	}
}

// TestVRRPConfSchemaGatesNetlinkBackend proves the config schema carries the
// `ze:backend "netlink"` annotation on its vrrp containers, so the iface backend
// gate rejects a vrrp group configured under a VPP-backed interface at schema
// level -- the belt to the plugin verifier's braces (spec-vrrp-5 A-4 / AC-5).
//
// VALIDATES: spec-vrrp-5 A-4 -- the merged schema gates vrrp to netlink.
// PREVENTS: the annotation being dropped, which would let a VPP tree carrying a
// vrrp group reach the engine instead of being rejected at validation.
func TestVRRPConfSchemaGatesNetlinkBackend(t *testing.T) {
	// One annotation for the ipv4 vrrp container, one for ipv6.
	if n := strings.Count(ZeVrrpConfYANG, `ze:backend "netlink"`); n < 2 {
		t.Errorf("ze-vrrp-conf.yang has %d `ze:backend \"netlink\"` annotations, want >= 2 (ipv4 + ipv6 vrrp containers)", n)
	}
}
