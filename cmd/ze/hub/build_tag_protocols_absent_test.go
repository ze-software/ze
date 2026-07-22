// Design: ai/rules/feature-gate-registration.md -- protocols absent symbol-drop proof
//
//go:build !ze_isis && !ze_ldp && !ze_ospf && !ze_rsvpte && !ze_vrrp && !ze_bgp

package hub

// VALIDATES: a bare ze_core build (all gated plugins off) contains zero symbols
// for any of isis/ldp/ospf/rsvpte (routing protocols), vrrp (first-hop
// redundancy), or the whole internal/component/bgp subtree (engine + codec +
// every BGP plugin) -- the binary-level compile-out proof across BOTH composition
// roots (generated all.go and the dispatch_*.go companions; vrrp has no dispatch
// companion, it registers CLI via the plugin registry). One build covers all
// six gated features (cheaper than one build per feature).
// PREVENTS: a regression where a gated plugin leaks into a hardened build via an
// always-on import or a missed composition root (R-2).
//
// The BGP needles are deliberately the whole subtree prefix rather than a
// per-package list: ze_bgp gates ~58 packages, and an enumeration would silently
// stop covering a package added later.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTag_Protocols_AbsentBinaryDropsSymbols(t *testing.T) {
	// test-relax: -short guard only; this test still runs in full (make ze-test
	// passes no -short). It builds and spawns the ze binary, so opt-in -short
	// runs skip it for speed. No coverage is lost in the verify/CI suite.
	if testing.Short() {
		t.Skip("builds the ze binary (slow); skipped under -short")
	}
	repoRoot := filepath.Join("..", "..", "..")
	bin := filepath.Join(t.TempDir(), "ze-core")
	build := exec.Command("go", "build", "-tags", "ze_core", "-o", bin, "./cmd/ze")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ze_core failed: %v\n%s", err, out)
	}

	out, err := exec.Command("go", "tool", "nm", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool nm failed: %v\n%s", err, out)
	}
	needles := []string{
		"internal/plugins/isis.", "internal/plugins/isis/",
		"internal/plugins/ldp.", "internal/plugins/ldp/",
		"internal/plugins/ospf.", "internal/plugins/ospf/",
		"internal/plugins/rsvpte.", "internal/plugins/rsvpte/",
		"internal/plugins/vrrp.", "internal/plugins/vrrp/",
		"internal/component/bgp.", "internal/component/bgp/",
		"internal/plugins/flowspec-firewall.", "internal/plugins/flowspec-firewall/",
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				t.Fatalf("ze_core binary retained gated-plugin symbol %q matching %q", line, needle)
			}
		}
	}
}
