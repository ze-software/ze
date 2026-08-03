package engine

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
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
// (ai/rules/completion.md).
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

// TestIPsecInterfaceDoctorCheckSpeaksWhenItCannotEvaluate covers the guard's own
// error path, which used to be silent.
//
// VALIDATES: an unparseable vpn/ipsec block produces a diagnostic instead of a
// pass, and an EMPTY interface name is distinguished from an absent one.
//
// PREVENTS: the measured fail-open. With `vpn { ipsec { interface ze-missing0;
// esp-group ESP-1 { } } }`, ze doctor reported "ready": true and exit 0 while
// BOTH the unparseable esp-group and the missing interface vanished, and
// ze config validate agreed. The check returned nil on a parse error, justified
// by a comment saying it was "the config system's error to report" -- and no
// layer reported it: ike's ipsec validation hangs off the SDK OnConfigVerify,
// which VerifyPluginConfig deliberately does not run for a live plugin, and the
// Registration declares no InProcessConfigVerifier. A guard that can neither
// evaluate nor speak does not exist (ai/rules/evidence.md).
func TestIPsecInterfaceDoctorCheckSpeaksWhenItCannotEvaluate(t *testing.T) {
	withInterfaceOracle(t, map[string]bool{"eth0": true})

	t.Run("unparseable ipsec config is reported, not skipped", func(t *testing.T) {
		// An esp-group whose proposal cannot be parsed is the one class of cause
		// that makes ParseIPsecConfig error (ipsec/config.go), so it is a real
		// config defect rather than a benign partial config -- hence "error".
		ipsecRoot := config.NewTree()
		ipsecRoot.Set("interface", "eth0")
		esp := config.NewTree()
		proposal := config.NewTree()
		proposal.Set("encryption", "not-a-cipher")
		esp.AddListEntry("proposal", "1", proposal)
		ipsecRoot.AddListEntry("esp-group", "ESP-1", esp)
		vpnRoot := config.NewTree()
		vpnRoot.SetContainer("ipsec", ipsecRoot)
		root := config.NewTree()
		root.SetContainer("vpn", vpnRoot)

		diags := checkIPsecInterface(registry.DoctorCheckContext{Tree: root})
		if len(diags) == 0 {
			t.Fatal("a config the check cannot parse must not read as a pass")
		}
		if diags[0].Severity != "error" {
			t.Fatalf("severity = %q, want error: a warning still leaves doctor reporting ready", diags[0].Severity)
		}
	})

	t.Run("empty interface name is an error, absent is silence", func(t *testing.T) {
		ipsecRoot := config.NewTree()
		ipsecRoot.Set("interface", "")
		vpnRoot := config.NewTree()
		vpnRoot.SetContainer("ipsec", ipsecRoot)
		root := config.NewTree()
		root.SetContainer("vpn", vpnRoot)

		diags := checkIPsecInterface(registry.DoctorCheckContext{Tree: root})
		if len(diags) != 1 {
			t.Fatalf("got %d diagnostics, want 1: an explicitly empty interface resolves to \"\" at runtime and never binds", len(diags))
		}

		// The absent case must stay silent, or every config without a vpn
		// section would start failing doctor.
		if d := checkIPsecInterface(registry.DoctorCheckContext{Tree: ipsecTree("")}); len(d) != 0 {
			t.Fatalf("an ABSENT interface leaf must stay silent, got %+v", d)
		}
	})
}
