package engine

import (
	"errors"
	"net"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// ipsecTree builds a config tree carrying vpn { ipsec { interface <name> } }.
// An empty name omits the leaf entirely.
func ipsecTree(name string) *config.Tree {
	ipsecRoot := config.NewTree()
	if name != "" {
		ipsecRoot.Set("interface", name)
	}
	vpnRoot := config.NewTree()
	vpnRoot.SetContainer("ipsec", ipsecRoot)
	root := config.NewTree()
	root.SetContainer("vpn", vpnRoot)
	return root
}

// withInterfaceOracle swaps the interface resolver for the duration of a test.
func withInterfaceOracle(t *testing.T, present map[string]bool) {
	t.Helper()
	original := interfaceByName
	interfaceByName = func(name string) (*net.Interface, error) {
		if present[name] {
			return &net.Interface{Name: name}, nil
		}
		return nil, errors.New("no such network interface")
	}
	t.Cleanup(func() { interfaceByName = original })
}

// VALIDATES: a vpn ipsec interface absent from the host produces a
// doctor-ipsec-iface error diagnostic naming the interface.
// PREVENTS: the silent failure this check exists for -- resolveInterfaceAddr
// returns "" for an unknown interface, so every peer without an explicit
// local-address quietly never establishes.
func TestIPsecInterfaceDoctorCheckReportsMissing(t *testing.T) {
	withInterfaceOracle(t, map[string]bool{"eth0": true})

	diags := checkIPsecInterface(registry.DoctorCheckContext{Tree: ipsecTree("eth9")})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(diags))
	}
	if diags[0].Code != "doctor-ipsec-iface" {
		t.Errorf("code = %q, want doctor-ipsec-iface", diags[0].Code)
	}
	if diags[0].Severity != "error" {
		t.Errorf("severity = %q, want error", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "eth9") {
		t.Errorf("message does not name the interface: %q", diags[0].Message)
	}
}

// VALIDATES: an interface that exists produces no diagnostic.
// PREVENTS: the check firing on every healthy deployment, which would train
// operators to ignore ze doctor output.
func TestIPsecInterfaceDoctorCheckAcceptsPresent(t *testing.T) {
	withInterfaceOracle(t, map[string]bool{"eth0": true})

	if diags := checkIPsecInterface(registry.DoctorCheckContext{Tree: ipsecTree("eth0")}); len(diags) != 0 {
		t.Fatalf("got %d diagnostics for a present interface, want 0: %+v", len(diags), diags)
	}
}

// VALIDATES: no vpn section, no ipsec interface leaf, and a nil/unusable tree
// each yield no diagnostic and no panic.
// PREVENTS: ze doctor failing or reporting a phantom IPsec problem on the
// overwhelming majority of configs, which do not use IPsec at all.
func TestIPsecInterfaceDoctorCheckQuietWithoutConfig(t *testing.T) {
	withInterfaceOracle(t, map[string]bool{"eth0": true})

	cases := []struct {
		name string
		ctx  registry.DoctorCheckContext
	}{
		{"no vpn section", registry.DoctorCheckContext{Tree: config.NewTree()}},
		{"no interface leaf", registry.DoctorCheckContext{Tree: ipsecTree("")}},
		{"nil tree", registry.DoctorCheckContext{Tree: (*config.Tree)(nil)}},
		{"tree of the wrong type", registry.DoctorCheckContext{Tree: "not a tree"}},
		{"no tree at all", registry.DoctorCheckContext{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diags := checkIPsecInterface(tc.ctx); len(diags) != 0 {
				t.Fatalf("got %d diagnostics, want 0: %+v", len(diags), diags)
			}
		})
	}
}

// VALIDATES: a malformed interface name is reported as malformed and never
// reaches the host resolver.
// PREVENTS: passing operator-controlled config text with path separators or a
// NUL straight into net.InterfaceByName.
func TestIPsecInterfaceDoctorCheckRejectsMalformedName(t *testing.T) {
	var asked []string
	original := interfaceByName
	interfaceByName = func(name string) (*net.Interface, error) {
		asked = append(asked, name)
		return nil, errors.New("no such network interface")
	}
	t.Cleanup(func() { interfaceByName = original })

	// "0123456789012345" is exactly IFNAMSIZ (16) and must be rejected; its
	// 15-char sibling is asserted ACCEPTED in the boundary test below. Without
	// both, `>=` could be mutated to `>` and no test would notice.
	for _, bad := range []string{"..", ".", "eth0/eth1", "eth\x000", "eth with space", "0123456789012345"} {
		t.Run(bad, func(t *testing.T) {
			diags := checkIPsecInterface(registry.DoctorCheckContext{Tree: ipsecTree(bad)})
			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics, want 1", len(diags))
			}
			if !strings.Contains(diags[0].Message, "invalid name") {
				t.Errorf("message = %q, want it to report an invalid name", diags[0].Message)
			}
		})
	}
	if len(asked) != 0 {
		t.Errorf("malformed names reached the host resolver: %q", asked)
	}
}

// VALIDATES: the IFNAMSIZ boundary -- 15 characters is the longest name the
// kernel can hold, and it must reach the resolver.
// PREVENTS: an off-by-one in the length guard. `len(name) >= ifNameSize`
// mutated to `>` survived every earlier test, because the only over-length
// fixture was 30 characters and failed under either comparison.
func TestIPsecInterfaceDoctorCheckAcceptsLongestLegalName(t *testing.T) {
	const longest = "012345678901234" // 15 chars: IFNAMSIZ-1
	if len(longest) != ifNameSize-1 {
		t.Fatalf("fixture is %d chars, want %d", len(longest), ifNameSize-1)
	}
	withInterfaceOracle(t, map[string]bool{longest: true})

	if diags := checkIPsecInterface(registry.DoctorCheckContext{Tree: ipsecTree(longest)}); len(diags) != 0 {
		t.Fatalf("longest legal interface name rejected: %+v", diags)
	}
}

// VALIDATES: a name the kernel CAN hold reaches the resolver, even when it
// contains a dot run.
// PREVENTS: the over-broad `strings.Contains(name, "..")` guard the first
// version used, which reported the legal interface name "br..0" as malformed
// and so never checked whether it existed. Found by review.
func TestIPsecInterfaceDoctorCheckAllowsLegalDottedName(t *testing.T) {
	withInterfaceOracle(t, map[string]bool{"br..0": true})

	if diags := checkIPsecInterface(registry.DoctorCheckContext{Tree: ipsecTree("br..0")}); len(diags) != 0 {
		t.Fatalf("legal interface name reported as a problem: %+v", diags)
	}
}

// VALIDATES: the check is declared on the ike plugin registration, so ze doctor
// actually runs it.
// PREVENTS: the whole check existing as dead code. ValidateInterfaceRef spent its
// life in exactly that state -- implemented, tested, and called by nothing
// (ai/rules/wiring-completeness.md).
func TestIPsecInterfaceDoctorCheckRegistered(t *testing.T) {
	for _, check := range registry.PluginDoctorChecks() {
		if check.PluginName != "ike" || check.Name != "ipsec-interface" {
			continue
		}
		if check.Check == nil {
			t.Fatal("ike ipsec-interface doctor check has a nil Check function")
		}
		found := false
		for _, code := range check.Codes {
			if code == "doctor-ipsec-iface" {
				found = true
			}
		}
		if !found {
			t.Errorf("declared codes %v do not include doctor-ipsec-iface", check.Codes)
		}
		return
	}
	t.Fatal("ike plugin declares no ipsec-interface doctor check")
}
