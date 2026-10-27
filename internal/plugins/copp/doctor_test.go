package copp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestDoctorCheckCoppNilTree(t *testing.T) {
	ctx := registry.DoctorCheckContext{Tree: nil}
	diags := checkCoppInputChain(ctx)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for nil tree, got %d", len(diags))
	}
}

func TestDoctorCheckCoppNoConfig(t *testing.T) {
	tree := config.NewTree()
	ctx := registry.DoctorCheckContext{Tree: tree}
	diags := checkCoppInputChain(ctx)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics without copp config, got %d", len(diags))
	}
}

func TestDoctorCheckCoppWithConfig(t *testing.T) {
	tree := config.NewTree()
	cpp := tree.GetOrCreateContainer("control-plane-protection")
	bgp := cpp.GetOrCreateContainer("bgp")
	bgp.Set("rate", "100/second")

	ctx := registry.DoctorCheckContext{Tree: tree}
	diags := checkCoppInputChain(ctx)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-copp-missing" {
		t.Errorf("code = %q, want doctor-copp-missing", diags[0].Code)
	}
}

// TestCoppDoctorNamesMissingDropTerm proves the check compares the kernel with
// what the configuration asks for and names the part that is gone, with the
// command that installs it again.
//
// VALIDATES: AC-15 (CoPP half): a kernel table that lost the
// unknown-source-drop term is named by term; a kernel with no ze_copp table is
// named as the whole table; a kernel holding the expected table answers
// nothing. The expected table is translatePolicy's answer for the tree, so the
// stand-in kernel is built from the same producer the plugin installs.
// PREVENTS: R-6, a CoPP that fails open in silence.
func TestCoppDoctorNamesMissingDropTerm(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("control-plane-protection").GetOrCreateContainer("bgp").Set("rate", "100/second")
	tree.GetOrCreateContainer("bgp").AddListEntry("peer", "edge", peerTree("192.0.2.1"))

	want, err := expectedTable(tree)
	if err != nil {
		t.Fatalf("expected table: %v", err)
	}
	if len(want.Sets) == 0 {
		t.Fatalf("expected table holds no neighbor set; the bgp section did not reach it: %+v", want)
	}

	withoutDrop := want
	withoutDrop.Chains = append([]firewall.Chain(nil), want.Chains...)
	input := withoutDrop.Chains[0]
	input.Terms = input.Terms[:len(input.Terms)-1]
	withoutDrop.Chains[0] = input

	cases := []struct {
		name   string
		kernel []firewall.Table
		want   []string
	}{
		{"drop term gone", []firewall.Table{withoutDrop}, []string{"term unknown-source-drop in chain input", "ze signal restart"}},
		{"table gone", nil, []string{"the whole table (inet ze_copp)", "ze signal restart"}},
		{"table whole", []firewall.Table{want}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved := readKernelTables
			readKernelTables = func() ([]firewall.Table, error) { return tc.kernel, nil }
			t.Cleanup(func() { readKernelTables = saved })

			diags := checkCoppInputChain(registry.DoctorCheckContext{Tree: tree})
			if tc.want == nil {
				if len(diags) != 0 {
					t.Fatalf("diagnostics = %+v, want none", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("diagnostics = %+v, want 1", diags)
			}
			for _, fragment := range tc.want {
				if !strings.Contains(diags[0].Message, fragment) {
					t.Errorf("message %q does not name %q", diags[0].Message, fragment)
				}
			}
		})
	}
}

// peerTree is a static BGP peer whose remote address is ip.
func peerTree(ip string) *config.Tree {
	peer := config.NewTree()
	peer.GetOrCreateContainer("connection").GetOrCreateContainer("remote").Set("ip", ip)
	return peer
}
