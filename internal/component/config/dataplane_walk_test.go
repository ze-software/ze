// VALIDATES: engine-owned whole-tree ze:backend walk across both config doors
// (LoadConfig and VerifyPluginConfig) and ze doctor (doctor-dataplane-mismatch),
// covering AC-9, AC-10, R-3, and the U-12 / U-13 moved assertions from iface,
// firewall, traffic, container, and vrrp.
// PREVENTS: nodes outside family roots escaping the walk, materialized schema
// defaults causing false refusals (tunnel ttl), or lost per-cause / HostVerdict
// propagation after deleting the per-component gate call sites.

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func registerRealYANGModuleForTest(t *testing.T, moduleFile, relPath string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	t.Cleanup(configyang.RegisterModuleForTest(moduleFile, string(raw)))
}

// TestBackendWalkJudgesNodeOutsideFamilyRoot proves R-3 (and U-13 container
// interface-backend gate): a node outside every declared selector root (the
// container network list) is judged against the primary data-plane selector at
// LoadConfig, VerifyPluginConfig, and ze doctor.
//
// VALIDATES: AC-9, AC-10, R-3 -- container network list judged by interface backend.
// PREVENTS: nodes outside family roots bypassing ze:backend enforcement.
func TestBackendWalkJudgesNodeOutsideFamilyRoot(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	registerRealYANGModuleForTest(t, "ze-container-conf.yang", "../../plugins/container/yang/ze-container-conf.yang")
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, []string{"kernel", "vpp"})

	vppCfg := `interface {
    backend vpp;
}
container {
    network lan {
        prefix [ 10.88.0.0/24 ];
    }
}
`
	_, loadErr := LoadConfig(vppCfg, "test.conf", nil)
	if loadErr == nil {
		t.Fatal("LoadConfig accepted container network under interface backend vpp")
	}
	for _, want := range []string{"/container/network/lan", `"vpp"`, "supported: kernel", "set interface backend kernel"} {
		if !strings.Contains(loadErr.Error(), want) {
			t.Errorf("LoadConfig refusal missing %q:\n%s", want, loadErr)
		}
	}

	vppTree := mustParsePluginVerifyConfig(t, vppCfg)
	verifyErrs := VerifyPluginConfig(vppTree)
	if len(verifyErrs) != 1 {
		t.Fatalf("VerifyPluginConfig errors = %v, want 1", verifyErrs)
	}
	if verifyErrs[0].Error() != loadErr.Error() {
		t.Errorf("VerifyPluginConfig vs LoadConfig differ:\nverify: %s\nload:   %s", verifyErrs[0], loadErr)
	}

	diags := checkDataPlaneCompatibility(diagnostic.DoctorCheckContext{Tree: vppTree})
	if len(diags) != 1 {
		t.Fatalf("doctor diagnostics = %+v, want 1", diags)
	}
	if diags[0].Code != CodeDoctorDataPlaneMismatch {
		t.Errorf("doctor code = %q, want %q", diags[0].Code, CodeDoctorDataPlaneMismatch)
	}
	if !strings.Contains(diags[0].Message, "/container/network/lan") {
		t.Errorf("doctor message missing /container/network/lan: %q", diags[0].Message)
	}
	if diags[0].Fix != "set interface backend kernel" {
		t.Errorf("doctor fix = %q, want %q", diags[0].Fix, "set interface backend kernel")
	}

	kernelCfg := `interface {
    backend kernel;
}
container {
    network lan {
        prefix [ 10.88.0.0/24 ];
    }
}
`
	if _, err := LoadConfig(kernelCfg, "test.conf", nil); err != nil {
		t.Fatalf("LoadConfig refused container network under kernel: %v", err)
	}
	kernelTree := mustParsePluginVerifyConfig(t, kernelCfg)
	if errs := VerifyPluginConfig(kernelTree); len(errs) != 0 {
		t.Fatalf("VerifyPluginConfig refused container network under kernel: %v", errs)
	}
	if diags := checkDataPlaneCompatibility(diagnostic.DoctorCheckContext{Tree: kernelTree}); len(diags) != 0 {
		t.Fatalf("doctor reported diagnostics for kernel container network: %+v", diags)
	}
}

// TestBackendWalkJudgesTreeAsWritten proves AC-10 (and U-13 moved twin of
// iface TestBackendGateVppRefusesTunnelTTL): the walk judges the candidate tree
// as written before schema defaults materialize, so a VPP tunnel without ttl
// commits at both doors while naming ttl (including 0) is refused by path, and
// a sit tunnel with ttl is refused at the tunnel kind rather than sit/ttl.
//
// VALIDATES: AC-10 -- tree judged as written before schema defaults materialize.
// PREVENTS: defaulted kernel-only leaves (tunnel ttl) refusing every VPP tunnel,
// or explicit ttl slipping past the walk onto VPP.
func TestBackendWalkJudgesTreeAsWritten(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

	kinds := []string{"gre", "gretap", "ipip"}
	makeTunnelCfg := func(backend, kind, ttlLine string) string {
		return "interface {\n  backend " + backend + ";\n  tunnel t0 {\n    encapsulation {\n      " + kind + " {\n        local {\n          ip 192.0.2.1;\n        }\n        remote {\n          ip 192.0.2.2;\n        }\n" + ttlLine + "      }\n    }\n  }\n}\n"
	}

	for _, kind := range kinds {
		t.Run("accept_unset_ttl_"+kind, func(t *testing.T) {
			cfg := makeTunnelCfg("vpp", kind, "")
			if _, err := LoadConfig(cfg, "test.conf", nil); err != nil {
				t.Fatalf("LoadConfig refused vpp %s tunnel without ttl: %v", kind, err)
			}
			tree := mustParsePluginVerifyConfig(t, cfg)
			if errs := VerifyPluginConfig(tree); len(errs) != 0 {
				t.Fatalf("VerifyPluginConfig refused vpp %s tunnel without ttl: %v", kind, errs)
			}
		})

		t.Run("refuse_explicit_ttl_64_"+kind, func(t *testing.T) {
			cfg := makeTunnelCfg("vpp", kind, "        ttl 64;\n")
			_, loadErr := LoadConfig(cfg, "test.conf", nil)
			if loadErr == nil {
				t.Fatalf("LoadConfig accepted vpp %s tunnel with explicit ttl 64", kind)
			}
			for _, want := range []string{"encapsulation/" + kind + "/ttl", `"vpp"`, "supported: kernel"} {
				if !strings.Contains(loadErr.Error(), want) {
					t.Errorf("LoadConfig refusal missing %q:\n%s", want, loadErr)
				}
			}
			tree := mustParsePluginVerifyConfig(t, cfg)
			verifyErrs := VerifyPluginConfig(tree)
			if len(verifyErrs) != 1 {
				t.Fatalf("VerifyPluginConfig errors = %v, want 1", verifyErrs)
			}
			if verifyErrs[0].Error() != loadErr.Error() {
				t.Errorf("VerifyPluginConfig vs LoadConfig differ:\nverify: %s\nload:   %s", verifyErrs[0], loadErr)
			}
		})

		t.Run("refuse_explicit_ttl_200_"+kind, func(t *testing.T) {
			cfg := makeTunnelCfg("vpp", kind, "        ttl 200;\n")
			_, loadErr := LoadConfig(cfg, "test.conf", nil)
			if loadErr == nil {
				t.Fatalf("LoadConfig accepted vpp %s tunnel with explicit ttl 200", kind)
			}
			for _, want := range []string{"encapsulation/" + kind + "/ttl", `"vpp"`, "kernel"} {
				if !strings.Contains(loadErr.Error(), want) {
					t.Errorf("LoadConfig refusal missing %q:\n%s", want, loadErr)
				}
			}
		})

		t.Run("refuse_explicit_inherit_"+kind, func(t *testing.T) {
			cfg := makeTunnelCfg("vpp", kind, "        ttl 0;\n")
			if _, err := LoadConfig(cfg, "test.conf", nil); err == nil {
				t.Fatalf("LoadConfig accepted vpp %s tunnel with explicit ttl 0", kind)
			}
			tree := mustParsePluginVerifyConfig(t, cfg)
			if errs := VerifyPluginConfig(tree); len(errs) == 0 {
				t.Fatalf("VerifyPluginConfig accepted vpp %s tunnel with explicit ttl 0", kind)
			}
		})

		t.Run("kernel_accepts_explicit_ttl_"+kind, func(t *testing.T) {
			cfg := makeTunnelCfg("kernel", kind, "        ttl 200;\n")
			if _, err := LoadConfig(cfg, "test.conf", nil); err != nil {
				t.Fatalf("LoadConfig refused kernel %s tunnel with ttl 200: %v", kind, err)
			}
			tree := mustParsePluginVerifyConfig(t, cfg)
			if errs := VerifyPluginConfig(tree); len(errs) != 0 {
				t.Fatalf("VerifyPluginConfig refused kernel %s tunnel with ttl 200: %v", kind, errs)
			}
		})
	}

	t.Run("sit_refused_by_kind_not_by_ttl", func(t *testing.T) {
		cfg := makeTunnelCfg("vpp", "sit", "        ttl 200;\n")
		_, err := LoadConfig(cfg, "test.conf", nil)
		if err == nil {
			t.Fatal("LoadConfig accepted vpp sit tunnel")
		}
		if strings.Contains(err.Error(), "sit/ttl") {
			t.Errorf("sit refusal must name the tunnel kind, not sit/ttl:\n%s", err)
		}
		if !strings.Contains(err.Error(), "/interface/tunnel/t0") {
			t.Errorf("sit refusal must name /interface/tunnel/t0:\n%s", err)
		}
	})
}

// TestBackendWalkIfaceVppTunnelKinds is the U-13 moved twin of
// iface.TestBackendGateVppTunnelKinds against the real ze-iface-conf.yang.
//
// VALIDATES: per-kind ze:backend widening accepts gre/gretap/ipip/vxlan on vpp
// and rejects sit/ip6tnl while keeping kernel accepted.
// PREVENTS: list-wide widening accepting unsupported tunnel kinds on vpp.
func TestBackendWalkIfaceVppTunnelKinds(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

	accept := []struct {
		name string
		body string
	}{
		{"gre", "gre { local { ip 192.0.2.1; } remote { ip 192.0.2.2; } }"},
		{"gretap", "gretap { local { ip 192.0.2.1; } remote { ip 192.0.2.2; } }"},
		{"ipip", "ipip { local { ip 192.0.2.1; } remote { ip 192.0.2.2; } }"},
		{"vxlan", "vxlan { local { ip 10.0.0.1; } remote { ip 10.0.0.2; } vni 100; }"},
	}
	for _, tc := range accept {
		t.Run("accept_"+tc.name, func(t *testing.T) {
			cfg := "interface { backend vpp; tunnel t0 { encapsulation { " + tc.body + " } } }\n"
			if _, err := LoadConfig(cfg, "test.conf", nil); err != nil {
				t.Errorf("vpp should accept %s tunnel: %v", tc.name, err)
			}
		})
	}

	reject := []struct {
		name string
		body string
	}{
		{"sit", "sit { local { ip 192.0.2.1; } remote { ip 192.0.2.2; } }"},
		{"ip6tnl", "ip6tnl { local { ip 2001:db8::1; } remote { ip 2001:db8::2; } }"},
	}
	for _, tc := range reject {
		t.Run("reject_"+tc.name, func(t *testing.T) {
			cfg := "interface { backend vpp; tunnel t0 { encapsulation { " + tc.body + " } } }\n"
			if _, err := LoadConfig(cfg, "test.conf", nil); err == nil {
				t.Errorf("vpp should reject %s tunnel (kernel-only)", tc.name)
			}
		})
	}

	t.Run("kernel_still_accepts_gre", func(t *testing.T) {
		cfg := "interface { backend kernel; tunnel t0 { encapsulation { gre { local { ip 192.0.2.1; } remote { ip 192.0.2.2; } } } } }\n"
		if _, err := LoadConfig(cfg, "test.conf", nil); err != nil {
			t.Errorf("kernel should still accept gre tunnel: %v", err)
		}
	})
}

// TestBackendWalkIfaceVppMirrorAndWireguard is the U-13 moved twin of
// iface.TestBackendGateVppMirror and iface.TestBackendGateVppWireguard.
//
// VALIDATES: mirror and wireguard ze:backend "kernel vpp" annotations accept
// both vpp and kernel in the engine walk.
// PREVENTS: mirror or wireguard being rejected by ze:backend under vpp or kernel.
func TestBackendWalkIfaceVppMirrorAndWireguard(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

	for _, backend := range []string{"vpp", "kernel"} {
		t.Run("mirror_"+backend, func(t *testing.T) {
			cfg := "interface { backend " + backend + "; ethernet xe1 { } ethernet xe0 { unit 0 { mirror { ingress xe1; } } } }\n"
			tree := mustParsePluginVerifyConfig(t, cfg)
			if errs := VerifyPluginConfig(tree); len(errs) != 0 {
				t.Errorf("%s should accept mirror in the ze:backend walk: %v", backend, errs)
			}
		})
		t.Run("wireguard_"+backend, func(t *testing.T) {
			cfg := "interface { backend " + backend + "; wireguard wg0 { private-key \"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\"; listen-port 51820; } }\n"
			if _, err := LoadConfig(cfg, "test.conf", nil); err != nil {
				t.Errorf("%s should accept wireguard in the ze:backend walk: %v", backend, err)
			}
			tree := mustParsePluginVerifyConfig(t, cfg)
			if errs := VerifyPluginConfig(tree); len(errs) != 0 {
				t.Errorf("%s should accept wireguard in VerifyPluginConfig: %v", backend, errs)
			}
		})
	}
}

// TestBackendWalkIfaceIPv6RPFCheck is the U-13 moved twin of
// iface.TestBackendGateIPv6RPFCheck.
//
// VALIDATES: ipv6 rpf-check is refused under vpp naming rpf-check and accepted
// under kernel.
// PREVENTS: VPP accepting ipv6 rpf-check where VPP uRPF breaks neighbor discovery.
func TestBackendWalkIfaceIPv6RPFCheck(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

	makeCfg := func(backend string) string {
		return "interface { backend " + backend + "; ethernet xe0 { unit 0 { ipv6 { rpf-check strict; } } } }\n"
	}
	_, err := LoadConfig(makeCfg("vpp"), "test.conf", nil)
	if err == nil {
		t.Fatal("vpp must refuse ipv6 rpf-check")
	}
	if !strings.Contains(err.Error(), "rpf-check") {
		t.Errorf("refusal must name rpf-check: %v", err)
	}
	if _, err := LoadConfig(makeCfg("kernel"), "test.conf", nil); err != nil {
		t.Errorf("kernel must accept ipv6 rpf-check: %v", err)
	}
}

// TestBackendWalkVRRPAndDoctor is the U-12 moved twin of
// vrrp.TestVerifyRejectsVPPBackend, vrrp.TestIfaceBackend, and
// vrrp.TestDoctorReportsVPPBackend.
//
// VALIDATES: AC-9, AC-10, U-12 -- a VRRP group under interface backend vpp is
// refused by the engine walk at both config doors and reported by ze doctor
// under doctor-dataplane-mismatch; vpp with zero VRRP groups and an omitted
// backend (schema default kernel) are accepted and stay silent in doctor.
// PREVENTS: deleting the VRRP hand backend check and doctor-vrrp-backend-unusable
// leaving VRRP under vpp accepted or unreported in ze doctor.
func TestBackendWalkVRRPAndDoctor(t *testing.T) {
	t.Cleanup(ResetDeclaredSchemasForTest())
	registerRealYANGModuleForTest(t, "ze-vrrp-conf.yang", "../../plugins/vrrp/yang/ze-vrrp-conf.yang")
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

	vppWithGroup := `interface {
    backend vpp;
    ethernet eth0 {
        unit 0 {
            ipv4 {
                vrrp {
                    group any {
                        vrid 10;
                        virtual-address [ 192.0.2.1 ];
                    }
                }
            }
        }
    }
}
`
	_, loadErr := LoadConfig(vppWithGroup, "test.conf", nil)
	if loadErr == nil {
		t.Fatal("vpp backend with a VRRP group must be rejected")
	}
	for _, want := range []string{"/interface/ethernet/eth0/unit/0/ipv4/vrrp", "backend", `"vpp"`, "supported: kernel", "set interface backend kernel"} {
		if !strings.Contains(loadErr.Error(), want) {
			t.Errorf("VRRP refusal missing %q:\n%s", want, loadErr)
		}
	}

	vppTree := mustParsePluginVerifyConfig(t, vppWithGroup)
	verifyErrs := VerifyPluginConfig(vppTree)
	if len(verifyErrs) != 1 {
		t.Fatalf("VerifyPluginConfig errors = %v, want 1", verifyErrs)
	}
	if verifyErrs[0].Error() != loadErr.Error() {
		t.Errorf("VerifyPluginConfig vs LoadConfig differ:\nverify: %s\nload:   %s", verifyErrs[0], loadErr)
	}

	diags := checkDataPlaneCompatibility(diagnostic.DoctorCheckContext{Tree: vppTree})
	if len(diags) != 1 {
		t.Fatalf("doctor diagnostics = %+v, want 1", diags)
	}
	if diags[0].Code != CodeDoctorDataPlaneMismatch {
		t.Errorf("doctor code = %q, want %q", diags[0].Code, CodeDoctorDataPlaneMismatch)
	}
	if !strings.Contains(diags[0].Message, "backend") {
		t.Errorf("doctor message must name backend: %q", diags[0].Message)
	}

	// vpp + zero VRRP groups -> accepted (idle).
	vppNoGroup := `interface {
    backend vpp;
    ethernet eth0 {
        unit 0 {
            ipv4 {
                address [ 192.0.2.2/24 ];
            }
        }
    }
}
`
	if _, err := LoadConfig(vppNoGroup, "test.conf", nil); err != nil {
		t.Fatalf("vpp backend with no VRRP groups must be accepted: %v", err)
	}
	if diags := checkDataPlaneCompatibility(diagnostic.DoctorCheckContext{Tree: mustParsePluginVerifyConfig(t, vppNoGroup)}); len(diags) != 0 {
		t.Fatalf("doctor diagnostics for vpp without VRRP groups = %+v, want none", diags)
	}

	// Absent backend leaf -> schema default kernel accepts VRRP group.
	defaultBackendWithGroup := `interface {
    ethernet eth0 {
        unit 0 {
            ipv4 {
                vrrp {
                    group any {
                        vrid 10;
                        virtual-address [ 192.0.2.1 ];
                    }
                }
            }
        }
    }
}
`
	if _, err := LoadConfig(defaultBackendWithGroup, "test.conf", nil); err != nil {
		t.Fatalf("default kernel backend with VRRP group must be accepted: %v", err)
	}
	if diags := checkDataPlaneCompatibility(diagnostic.DoctorCheckContext{Tree: mustParsePluginVerifyConfig(t, defaultBackendWithGroup)}); len(diags) != 0 {
		t.Fatalf("doctor diagnostics for default kernel VRRP = %+v, want none", diags)
	}
}

// TestBackendWalkFirewallGate is the U-13 moved twin of
// firewall.TestFirewallBackendGateRejects_EmptyBackend,
// TestFirewallBackendGateRejects_Synthetic, and
// TestFirewallBackendGateAccepts_NftBackend.
//
// VALIDATES: firewall root is walked against the firewall selector (empty
// backend HostVerdict, synthetic kernel-only list refusal under vpp, real
// ze-firewall-conf.yang acceptance under kernel and refusal under vpp).
// PREVENTS: deleting firewall.validateBackendGate breaking firewall ze:backend
// enforcement.
func TestBackendWalkFirewallGate(t *testing.T) {
	t.Run("empty_backend_host_verdict", func(t *testing.T) {
		schema := buildVerdictTestSchema("kernel", "kernel", "kernel")
		primary := &registeredSelector{
			sel:        &registry.DataPlaneSelector{LeafPath: "/interface/backend", Roots: []string{"interface"}},
			display:    "interface backend",
			value:      "kernel",
			defaultVal: "kernel",
			valid:      true,
		}
		fwSel := &registeredSelector{
			sel:     &registry.DataPlaneSelector{LeafPath: "/firewall/backend", Roots: []string{"firewall"}},
			display: "firewall backend",
			value:   "",
			valid:   true,
		}
		treeMap := map[string]any{
			"firewall": map[string]any{"table": map[string]any{"t1": map[string]any{"name": "t1"}}},
		}
		err := findingsToError(evaluateBackendFeatureWalk(treeMap, schema, primary, []*registeredSelector{fwSel}))
		if err == nil {
			t.Fatal("empty firewall backend must be refused")
		}
		if !diagnostic.IsHostVerdict(err) {
			t.Fatalf("empty firewall backend refusal lost HostVerdict classification: %v", err)
		}
		if !strings.Contains(err.Error(), "backend feature gate: no backend configured at /firewall/backend") {
			t.Errorf("refusal missing /firewall/backend: %v", err)
		}
	})

	t.Run("synthetic_kernel_only_rejected_under_vpp", func(t *testing.T) {
		list := List(TypeString, Field("name", Leaf(TypeString)))
		list.KeyName = "name"
		list.Backend = []string{"kernel"}

		synthetic := NewSchema()
		synthetic.Define("interface", Container(Field("backend", &LeafNode{Type: TypeString, Default: "kernel"})))
		synthetic.Define("firewall", Container(
			Field("backend", &LeafNode{Type: TypeString, Default: "kernel"}),
			Field("table", list),
		))

		isolateRegistry(t)
		registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, nil)

		tree := NewTree()
		tree.GetOrCreateContainer("interface").Set("backend", "vpp")
		fw := tree.GetOrCreateContainer("firewall")
		fw.Set("backend", "vpp")
		t1 := NewTree()
		t1.Set("name", "t1")
		fw.AddListEntry("table", "t1", t1)

		err := refuseDataPlaneMismatches(tree, synthetic, nil)
		if err == nil {
			t.Fatal("synthetic firewall schema must reject vpp on kernel-annotated list")
		}
		for _, want := range []string{"/firewall/table", `"vpp"`, "kernel", "set firewall backend kernel"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal missing %q:\n%s", want, err)
			}
		}
	})

	t.Run("real_schema_kernel_accepts_and_vpp_refuses_connection_mark", func(t *testing.T) {
		t.Cleanup(ResetDeclaredSchemasForTest())
		registerRealYANGModuleForTest(t, "ze-firewall-conf.yang", "../firewall/yang/ze-firewall-conf.yang")
		isolateRegistry(t)
		registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, nil)

		kernelCfg := `firewall {
    backend kernel;
    table t1 {
        family ip;
    }
}
`
		if _, err := LoadConfig(kernelCfg, "test.conf", nil); err != nil {
			t.Fatalf("kernel firewall must be accepted: %v", err)
		}

		vppCfg := `interface {
    backend vpp;
}
firewall {
    backend vpp;
    table t1 {
        family inet;
        chain input {
            type filter;
            hook input;
            priority 0;
            policy accept;
            term r1 {
                from {
                    connection-mark 1;
                }
                then {
                    accept;
                }
            }
        }
    }
}
`
		_, err := LoadConfig(vppCfg, "test.conf", nil)
		if err == nil {
			t.Fatal("vpp firewall must refuse kernel-only connection-mark")
		}
		for _, want := range []string{"/firewall/table/t1/chain/input/term/r1/from/connection-mark", `"vpp"`, "supported: kernel", "set firewall backend kernel"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal missing %q:\n%s", want, err)
			}
		}
	})
}

// TestBackendWalkTrafficGate is the U-13 moved twin of
// traffic.TestTrafficBackendGateRejects_EmptyBackend and
// TestTrafficBackendGateRejects_Synthetic.
//
// VALIDATES: nested selector root "traffic/control" is dispatched to the
// traffic selector (empty-backend HostVerdict and synthetic kernel-only
// refusal under vpp).
// PREVENTS: nested selector roots falling back to the primary selector instead
// of their declared selector.
func TestBackendWalkTrafficGate(t *testing.T) {
	t.Run("empty_backend_host_verdict", func(t *testing.T) {
		schema := buildVerdictTestSchema("kernel", "kernel", "kernel")
		primary := &registeredSelector{
			sel:        &registry.DataPlaneSelector{LeafPath: "/interface/backend", Roots: []string{"interface"}},
			display:    "interface backend",
			value:      "kernel",
			defaultVal: "kernel",
			valid:      true,
		}
		tcSel := &registeredSelector{
			sel:     &registry.DataPlaneSelector{LeafPath: "/traffic/control/backend", Roots: []string{"traffic/control"}},
			display: "traffic control backend",
			value:   "",
			valid:   true,
		}
		treeMap := map[string]any{
			"traffic": map[string]any{
				"control": map[string]any{
					"interface": map[string]any{"eth0": map[string]any{"name": "eth0"}},
				},
			},
		}
		err := findingsToError(evaluateBackendFeatureWalk(treeMap, schema, primary, []*registeredSelector{tcSel}))
		if err == nil {
			t.Fatal("empty traffic backend must be refused")
		}
		if !diagnostic.IsHostVerdict(err) {
			t.Fatalf("empty traffic backend refusal lost HostVerdict classification: %v", err)
		}
		if !strings.Contains(err.Error(), "backend feature gate: no backend configured at /traffic/control/backend") {
			t.Errorf("refusal missing /traffic/control/backend: %v", err)
		}
	})

	t.Run("synthetic_nested_root_rejected_under_vpp", func(t *testing.T) {
		list := List(TypeString, Field("name", Leaf(TypeString)))
		list.KeyName = "name"
		list.Backend = []string{"kernel"}

		synthetic := NewSchema()
		synthetic.Define("interface", Container(Field("backend", &LeafNode{Type: TypeString, Default: "kernel"})))
		synthetic.Define("traffic", Container(
			Field("control", Container(
				Field("backend", &LeafNode{Type: TypeString, Default: "kernel"}),
				Field("interface", list),
			)),
		))

		isolateRegistry(t)
		registerTestSelectors(t, []string{"kernel", "vpp"}, nil, []string{"kernel", "vpp"})

		tree := NewTree()
		tree.GetOrCreateContainer("interface").Set("backend", "vpp")
		tc := tree.GetOrCreateContainer("traffic").GetOrCreateContainer("control")
		tc.Set("backend", "vpp")
		eth0 := NewTree()
		eth0.Set("name", "eth0")
		tc.AddListEntry("interface", "eth0", eth0)

		err := refuseDataPlaneMismatches(tree, synthetic, nil)
		if err == nil {
			t.Fatal("synthetic traffic schema must reject vpp on kernel-annotated list")
		}
		for _, want := range []string{"/traffic/control/interface", `"vpp"`, "kernel", "set traffic control backend kernel"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal missing %q:\n%s", want, err)
			}
		}
	})
}

// TestBackendWalkDiagnosticCausesAndHostVerdict is the U-13 moved twin of
// iface/backend_gate_diagnostic_test.go (HostVerdict preservation, multi-cause
// Unwrap aggregation, supported acceptance, and absent/unrelated roots).
//
// VALIDATES: empty-backend walk findings retain diagnostic.HostVerdict,
// multi-node mismatches expose every ordered cause via Unwrap() []error, and
// supported or empty trees produce nil.
// PREVENTS: rendering refusals dropping HostVerdict or masking sibling causes.
func TestBackendWalkDiagnosticCausesAndHostVerdict(t *testing.T) {
	first := Leaf(TypeString)
	first.Backend = []string{"kernel"}
	second := Leaf(TypeString)
	second.Backend = []string{"kernel"}
	schema := NewSchema()
	schema.Define("interface", Container(
		Field("backend", &LeafNode{Type: TypeString, Default: "kernel"}),
		Field("first", first),
		Field("second", second),
	))

	t.Run("preserves_host_verdict", func(t *testing.T) {
		primary := &registeredSelector{
			sel:     &registry.DataPlaneSelector{LeafPath: "/interface/backend", Roots: []string{"interface"}},
			display: "interface backend",
			value:   "",
			valid:   true,
		}
		err := findingsToError(evaluateBackendFeatureWalk(
			map[string]any{"interface": map[string]any{}},
			schema,
			primary,
			nil,
		))
		if err == nil {
			t.Fatal("empty interface backend must be refused")
		}
		if !diagnostic.IsHostVerdict(err) {
			t.Fatalf("empty-backend refusal lost HostVerdict classification: %v", err)
		}
		if !strings.Contains(err.Error(), "backend feature gate: no backend configured at /interface/backend") {
			t.Errorf("refusal missing /interface/backend: %v", err)
		}
	})

	t.Run("preserves_every_cause_and_accepts_supported", func(t *testing.T) {
		isolateRegistry(t)
		registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

		tree := NewTree()
		iface := tree.GetOrCreateContainer("interface")
		iface.Set("backend", "vpp")
		iface.Set("first", "yes")
		iface.Set("second", "yes")

		err := refuseDataPlaneMismatches(tree, schema, nil)
		if err == nil {
			t.Fatal("two unsupported nodes were accepted")
		}
		if diagnostic.IsHostVerdict(err) {
			t.Fatalf("feature mismatches were classified as a host verdict: %v", err)
		}
		wantCauses := []string{
			`/interface/first: feature not supported by backend "vpp" (supported: kernel)`,
			`/interface/second: feature not supported by backend "vpp" (supported: kernel)`,
		}
		var aggregate interface{ Unwrap() []error }
		if !errors.As(err, &aggregate) {
			t.Fatalf("aggregate exposes no joined causes: %v", err)
		}
		causes := aggregate.Unwrap()
		if len(causes) != 2 {
			t.Fatalf("aggregate has %d causes, want 2: %v", len(causes), causes)
		}
		for i, cause := range causes {
			if !strings.Contains(cause.Error(), wantCauses[i]) {
				t.Errorf("cause %d = %q, want substring %q", i, cause.Error(), wantCauses[i])
			}
			if !errors.Is(err, cause) {
				t.Errorf("aggregate lost reachable cause %v", cause)
			}
		}

		// Supported backend accepts both nodes.
		iface.Set("backend", "kernel")
		if err := refuseDataPlaneMismatches(tree, schema, nil); err != nil {
			t.Fatalf("supported nodes were rejected: %v", err)
		}
	})

	t.Run("empty_and_unrelated_trees_accepted", func(t *testing.T) {
		isolateRegistry(t)
		registerTestSelectors(t, []string{"kernel", "vpp"}, nil, nil)

		if err := refuseDataPlaneMismatches(NewTree(), schema, nil); err != nil {
			t.Fatalf("empty tree was rejected: %v", err)
		}
		other := NewTree()
		other.GetOrCreateContainer("other-component").Set("x", "y")
		if err := refuseDataPlaneMismatches(other, schema, nil); err != nil {
			t.Fatalf("unrelated section was rejected: %v", err)
		}
	})
}
