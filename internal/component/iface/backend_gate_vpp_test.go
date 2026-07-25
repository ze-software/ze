package iface

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// gateSection wraps interface config JSON as the single ConfigSection the
// commit-time backend gate receives.
func gateSection(data string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootInterface, Data: data}}
}

// TestBackendGateVppTunnelKinds is the end-to-end proof (against the real
// ze-iface-conf.yang schema) that the per-kind ze:backend widening lets the
// vpp backend accept exactly gre/gretap/ipip/vxlan while the untouched
// netlink-only kinds (e.g. sit) are still rejected at commit under vpp.
// VALIDATES: AC-2, AC-3, R-2 -- per-kind annotation, exact-or-reject preserved.
// PREVENTS: a list-wide widening that would accept kinds VPP cannot program.
func TestBackendGateVppTunnelKinds(t *testing.T) {
	accept := []struct {
		name string
		body string
	}{
		{"gre", `"gre":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"}}`},
		{"gretap", `"gretap":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"}}`},
		{"ipip", `"ipip":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"}}`},
		{"vxlan", `"vxlan":{"local":{"ip":"10.0.0.1"},"remote":{"ip":"10.0.0.2"},"vni":"100"}`},
	}
	for _, tc := range accept {
		t.Run("accept_"+tc.name, func(t *testing.T) {
			data := `{"interface":{"backend":"vpp","tunnel":{"t0":{"name":"t0","encapsulation":{` + tc.body + `}}}}}`
			if err := validateBackendGate(gateSection(data), "vpp"); err != nil {
				t.Errorf("vpp should accept %s tunnel: %v", tc.name, err)
			}
		})
	}

	reject := []struct {
		name string
		body string
	}{
		{"sit", `"sit":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"}}`},
		{"ip6tnl", `"ip6tnl":{"local":{"ip":"2001:db8::1"},"remote":{"ip":"2001:db8::2"}}`},
	}
	for _, tc := range reject {
		t.Run("reject_"+tc.name, func(t *testing.T) {
			data := `{"interface":{"backend":"vpp","tunnel":{"t0":{"name":"t0","encapsulation":{` + tc.body + `}}}}}`
			if err := validateBackendGate(gateSection(data), "vpp"); err == nil {
				t.Errorf("vpp should reject %s tunnel (netlink-only)", tc.name)
			}
		})
	}

	// The same gre config under the netlink backend must still be accepted:
	// widening added vpp, it did not remove netlink.
	t.Run("netlink_still_accepts_gre", func(t *testing.T) {
		data := `{"interface":{"backend":"netlink","tunnel":{"t0":{"name":"t0","encapsulation":{"gre":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"}}}}}}}`
		if err := validateBackendGate(gateSection(data), "netlink"); err != nil {
			t.Errorf("netlink should still accept gre tunnel: %v", err)
		}
	})
}

// TestBackendGateVppMirror is the end-to-end proof (against the real schema)
// that the mirror container's ze:backend widening to "netlink vpp" lets the vpp
// backend accept a mirror config while netlink still accepts it too.
// VALIDATES: AC-4 -- mirror ze:backend widened to include vpp.
// PREVENTS: a mirror-under-vpp config being rejected at commit after SPAN wiring.
func TestBackendGateVppMirror(t *testing.T) {
	mirrorData := func(backend string) string {
		return `{"interface":{"backend":"` + backend + `","ethernet":{"xe0":{"name":"xe0","unit":{"u0":{"name":"u0","mirror":{"ingress":"xe1"}}}}}}}`
	}
	t.Run("vpp_accepts_mirror", func(t *testing.T) {
		if err := validateBackendGate(gateSection(mirrorData("vpp")), "vpp"); err != nil {
			t.Errorf("vpp should accept mirror after widening: %v", err)
		}
	})
	t.Run("netlink_still_accepts_mirror", func(t *testing.T) {
		if err := validateBackendGate(gateSection(mirrorData("netlink")), "netlink"); err != nil {
			t.Errorf("netlink should still accept mirror: %v", err)
		}
	})
}

// TestBackendGateVppWireguard is the end-to-end proof (against the real schema)
// that the wireguard list's ze:backend widening to "netlink vpp" lets the vpp
// backend accept a wireguard config while netlink still accepts it too.
// VALIDATES: AC-5 -- wireguard ze:backend widened to include vpp.
// PREVENTS: a wireguard-under-vpp config being rejected at commit after the
//
//	wireguard plugin binary-API wiring landed.
func TestBackendGateVppWireguard(t *testing.T) {
	wgData := func(backend string) string {
		return `{"interface":{"backend":"` + backend + `","wireguard":{"wg0":{"name":"wg0","listen-port":"51820"}}}}`
	}
	t.Run("vpp_accepts_wireguard", func(t *testing.T) {
		if err := validateBackendGate(gateSection(wgData("vpp")), "vpp"); err != nil {
			t.Errorf("vpp should accept wireguard after widening: %v", err)
		}
	})
	t.Run("netlink_still_accepts_wireguard", func(t *testing.T) {
		if err := validateBackendGate(gateSection(wgData("netlink")), "netlink"); err != nil {
			t.Errorf("netlink should still accept wireguard: %v", err)
		}
	})
}
