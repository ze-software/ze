package iface

import (
	"strings"
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

// TestBackendGateVppRefusesTunnelTTL drives the commit entry point, not the
// walker, and proves both polarities of the ttl leaf's netlink-only
// annotation: an explicitly configured ttl on a vpp-backed gre, gretap or
// ipip tunnel is refused by path, and the same tunnel without a ttl commits.
//
// The entry point matters. verifyIfaceConfig is what OnConfigVerify calls, and
// inside it parseAndVerifyIfaceSections runs validateBackendGate BEFORE
// parseIfaceSections, so the gate reads what the operator wrote and never the
// schema default that parseTunnelEntry materializes afterwards. A test on the
// walker alone would prove neither half of that ordering.
//
// VALIDATES: R-3 of spec-tunnel-ttl-default, settled as refuse rather than
//
//	warn. An explicit ttl reached no vpp device and reported success,
//	which is the value that is silently wrong that ai/rules/principles.md
//	forbids.
//
// PREVENTS: the leaf reading as authoritative on a backend that drops it.
//
//	GreTunnelV2 and IpipTunnel carry no hop-limit value, so no value an
//	operator writes can reach a vpp-programmed tunnel.
func TestBackendGateVppRefusesTunnelTTL(t *testing.T) {
	kinds := []struct {
		name string
		body string
	}{
		{"gre", `"gre":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"}%s}`},
		{"gretap", `"gretap":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"}%s}`},
		{"ipip", `"ipip":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"}%s}`},
	}
	config := func(backend, body string) string {
		return `{"interface":{"backend":"` + backend + `","tunnel":{"t0":{"name":"t0","encapsulation":{` + body + `}}}}}`
	}

	for _, kind := range kinds {
		body := func(extra string) string { return strings.Replace(kind.body, "%s", extra, 1) }

		t.Run("refuse_explicit_ttl_"+kind.name, func(t *testing.T) {
			err := verifyIfaceConfig(gateSection(config("vpp", body(`,"ttl":"200"`))))
			if err == nil {
				t.Fatalf("vpp must refuse an explicit ttl on a %s tunnel: the value reaches no device", kind.name)
			}
			for _, want := range []string{"encapsulation/" + kind.name + "/ttl", `"vpp"`, "netlink"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})

		t.Run("refuse_explicit_inherit_"+kind.name, func(t *testing.T) {
			// 0 is a value like any other here: the vpp backend sets no
			// hop-limit copy flag either, so inherit does not reach the
			// device and must not be accepted as though it did.
			if err := verifyIfaceConfig(gateSection(config("vpp", body(`,"ttl":"0"`)))); err == nil {
				t.Errorf("vpp must refuse an explicit ttl 0 on a %s tunnel", kind.name)
			}
		})

		t.Run("accept_unset_ttl_"+kind.name, func(t *testing.T) {
			// The schema default is materialized after the gate has run, so a
			// tunnel that names no ttl still commits under vpp.
			if err := verifyIfaceConfig(gateSection(config("vpp", body("")))); err != nil {
				t.Errorf("vpp must accept a %s tunnel that names no ttl: %v", kind.name, err)
			}
		})

		t.Run("netlink_still_accepts_explicit_ttl_"+kind.name, func(t *testing.T) {
			// The annotation names the backend that does not implement the
			// leaf. It is not a ban on the leaf.
			if err := verifyIfaceConfig(gateSection(config("netlink", body(`,"ttl":"200"`)))); err != nil {
				t.Errorf("netlink must still accept an explicit ttl on a %s tunnel: %v", kind.name, err)
			}
		})
	}

	// sit carries no annotation of its own, so the whole kind is already
	// refused under vpp by the tunnel list. Its ttl leaf is deliberately NOT
	// annotated: a leaf-level refusal there would name the leaf where the kind
	// is the problem, and CreateTunnel rejects the kind at apply too.
	t.Run("sit_refused_by_kind_not_by_ttl", func(t *testing.T) {
		err := verifyIfaceConfig(gateSection(config("vpp", `"sit":{"local":{"ip":"192.0.2.1"},"remote":{"ip":"192.0.2.2"},"ttl":"200"}`)))
		if err == nil {
			t.Fatal("vpp must refuse a sit tunnel")
		}
		if strings.Contains(err.Error(), "sit/ttl") {
			t.Errorf("the sit refusal must name the kind, not the ttl leaf: %v", err)
		}
	})
}
