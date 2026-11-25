// Design: docs/architecture/core-design.md -- unit tests for the VPP WireGuard plugin tree validator and shared doctor check
// Related: verify_wireguard.go -- checkWireguardPluginTree, verifyWireguardPluginTree
// Related: register_wireguard_validator.go -- registers iface-vpp-wireguard-plugin

package ifacevpp

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

const testWireguardPrivateKey = "aHZkc2ZqaHZra2hkZnZoamtkc2Zoa2RoaGRma2poZmg="

func wireguardCandidateTree(backend string, wgNames []string, pluginState string) *config.Tree {
	tree := config.NewTree()
	ifaceTree := tree.GetOrCreateContainer("interface")
	if backend != "" {
		ifaceTree.Set("backend", backend)
	}
	for _, name := range wgNames {
		wg := config.NewTree()
		wg.Set("name", name)
		wg.Set("listen-port", "51820")
		wg.Set("private-key", testWireguardPrivateKey)
		ifaceTree.AddListEntry("wireguard", name, wg)
	}
	if pluginState != "" {
		plugins := tree.GetOrCreateContainer("vpp").GetOrCreateContainer("plugins")
		plugins.Set("wireguard", pluginState)
	}
	return tree
}

// VALIDATES: AC-2 -- checkWireguardPluginTree and verifyWireguardPluginTree refuse
// WireGuard interfaces under interface backend vpp when vpp.plugins.wireguard is
// absent or false, name every such interface with fix `set vpp plugins wireguard true`,
// and accept kernel (explicit or schema default), enabled plugin, or no WireGuard
// interfaces. The doctor check calls the same function and runs in DoctorPhaseParsedConfig.
// PREVENTS: a WireGuard-under-VPP config committing without wireguard_plugin.so
// enabled, or doctor and the tree validator disagreeing on the refusal.
func TestVerifyWireguardNeedsPlugin(t *testing.T) {
	t.Run("vpp with plugin absent refuses single interface", func(t *testing.T) {
		tree := wireguardCandidateTree("vpp", []string{"wg0"}, "")
		err := verifyWireguardPluginTree(tree)
		if err == nil {
			t.Fatal("expected refusal when vpp.plugins.wireguard is absent")
		}
		oe, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Fatalf("expected *opererr.Error, got %T: %v", err, err)
		}
		if !strings.Contains(oe.Problem(), "wg0") {
			t.Errorf("problem = %q, want wg0 named", oe.Problem())
		}
		if oe.Fix() != fixEnableVPPWireguardPlugin {
			t.Errorf("fix = %q, want %q", oe.Fix(), fixEnableVPPWireguardPlugin)
		}
	})

	t.Run("vpp with plugin false refuses every interface in one refusal", func(t *testing.T) {
		tree := wireguardCandidateTree("vpp", []string{"wg1", "wg0"}, "false")
		err := verifyWireguardPluginTree(tree)
		if err == nil {
			t.Fatal("expected refusal when vpp.plugins.wireguard is false")
		}
		oe, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Fatalf("expected *opererr.Error, got %T: %v", err, err)
		}
		if !strings.Contains(oe.Problem(), "wg0") || !strings.Contains(oe.Problem(), "wg1") {
			t.Errorf("problem = %q, want both wg0 and wg1 named", oe.Problem())
		}
		if oe.Fix() != fixEnableVPPWireguardPlugin {
			t.Errorf("fix = %q, want %q", oe.Fix(), fixEnableVPPWireguardPlugin)
		}

		// Doctor check delegates to the exact same function and reports every interface.
		diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: tree})
		if len(diags) != 1 {
			t.Fatalf("doctor emitted %d diagnostics, want 1", len(diags))
		}
		if diags[0].Code != codeDoctorVPPWireguard {
			t.Errorf("doctor code = %q, want %q", diags[0].Code, codeDoctorVPPWireguard)
		}
		if diags[0].Fix != fixEnableVPPWireguardPlugin {
			t.Errorf("doctor fix = %q, want %q", diags[0].Fix, fixEnableVPPWireguardPlugin)
		}
		if !strings.Contains(diags[0].Message, "wg0") || !strings.Contains(diags[0].Message, "wg1") {
			t.Errorf("doctor message = %q, want both wg0 and wg1 named", diags[0].Message)
		}
	})

	t.Run("vpp with plugin true is accepted", func(t *testing.T) {
		tree := wireguardCandidateTree("vpp", []string{"wg0"}, "true")
		if err := verifyWireguardPluginTree(tree); err != nil {
			t.Fatalf("expected acceptance with plugin enabled, got %v", err)
		}
	})

	t.Run("kernel backend explicit is accepted", func(t *testing.T) {
		tree := wireguardCandidateTree("kernel", []string{"wg0"}, "")
		if err := verifyWireguardPluginTree(tree); err != nil {
			t.Fatalf("expected acceptance under kernel backend, got %v", err)
		}
	})

	t.Run("omitted backend resolves schema default kernel and is accepted", func(t *testing.T) {
		tree := wireguardCandidateTree("", []string{"wg0"}, "")
		if err := verifyWireguardPluginTree(tree); err != nil {
			t.Fatalf("expected acceptance under omitted backend (schema default kernel), got %v", err)
		}
	})

	t.Run("vpp without wireguard interfaces is accepted", func(t *testing.T) {
		tree := wireguardCandidateTree("vpp", nil, "false")
		if err := verifyWireguardPluginTree(tree); err != nil {
			t.Fatalf("expected acceptance without wireguard interfaces, got %v", err)
		}
	})

	t.Run("doctor check runs in DoctorPhaseParsedConfig so ReadConfig refusal cannot hide it", func(t *testing.T) {
		found := false
		for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseParsedConfig) {
			if check.Name == "vpp-wireguard-plugin" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("vpp-wireguard-plugin is not registered in DoctorPhaseParsedConfig")
		}
	})
}

// VALIDATES: AC-2 -- init() registers iface-vpp-wireguard-plugin as a config
// original-tree validator, and config.ValidateCustomSections reaches it over the
// whole candidate tree when only the vpp root turns wireguard off.
// PREVENTS: a check that only works via direct unit call or only triggers when
// the interface root changed.
func TestWireguardValidatorIsRegisteredAndReadsTheWholeTree(t *testing.T) {
	// Duplicate registration must fail, proving init() installed the validator.
	if err := config.RegisterTreeValidator(wireguardTreeValidatorName, func(*config.Tree) error { return nil }); err == nil {
		t.Fatalf("expected duplicate registration error for %q", wireguardTreeValidatorName)
	}

	// Candidate with unchanged interface (vpp + wg0) and vpp root turning wireguard false.
	refusedTree := wireguardCandidateTree("vpp", []string{"wg0"}, "false")
	_, err := config.ValidateCustomSections(refusedTree)
	if err == nil {
		t.Fatal("ValidateCustomSections accepted WireGuard under vpp with vpp.plugins.wireguard=false")
	}
	if !strings.Contains(err.Error(), "wg0") {
		t.Errorf("refusal %q does not name wg0", err.Error())
	}
	if !strings.Contains(err.Error(), fixEnableVPPWireguardPlugin) {
		t.Errorf("refusal %q does not carry fix %q", err.Error(), fixEnableVPPWireguardPlugin)
	}

	// Same interface root with vpp.plugins.wireguard=true must pass ValidateCustomSections.
	acceptedTree := wireguardCandidateTree("vpp", []string{"wg0"}, "true")
	failures, err := config.ValidateCustomSections(acceptedTree)
	if err != nil {
		t.Fatalf("ValidateCustomSections refused valid WireGuard+VPP tree: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("ValidateCustomSections returned unexpected failures: %+v", failures)
	}
}
