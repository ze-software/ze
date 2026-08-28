// Design: ai/rules/plugins.md -- spec-feature-gate-12 symbol-drop proof
//
//go:build ze_l2tp && ze_radius

package hub

// VALIDATES: a bare ze_core binary links zero symbols from any package gated by
// spec-feature-gate-12 (the remaining compile-out candidates), and the MIXED
// l2tp/radius lanes partition correctly (an l2tp-only build links zero radius
// symbols; a radius-only build links zero l2tp symbols). One consolidated test
// covers every gate added by that spec (cheaper than one build per feature).
// PREVENTS: a regression where one of these features leaks into a hardened
// build via an always-on import or a missed composition root, or where the
// generator's mixed-tag split (all_ze_radius.go vs all_ze_radius_ze_l2tp.go)
// regresses so an advertised mixed build ships broken.
//
// The file is constrained to ze_l2tp && ze_radius (both default-on) so its
// build+nm jobs run ONCE, in the full-feature unit pass, not again in the bare
// ze_core pass -- the binaries under test are built with explicit -tags, so
// the running lane's own tags are irrelevant (the gate11 cost optimization).
//
// Registration-level and config-rejection checks live in the per-tag
// build_tag_<x>_absent_test.go files; this test is only the binary-level proof.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildTag_Gate12_AbsentBinaryDropsSymbols(t *testing.T) {
	// -short guard only; this test still runs in full verification
	// (`./le verify current mode full` passes no -short). It builds and links the
	// ze binary, so opt-in -short runs skip it for speed.
	if testing.Short() {
		t.Skip("builds the ze binary (slow); skipped under -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	repoRoot := filepath.Join("..", "..", "..")

	bin := filepath.Join(t.TempDir(), "ze-core")
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", "ze_core", "-o", bin, "./cmd/ze")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags ze_core failed: %v\n%s", err, out)
	}
	out, err := exec.CommandContext(ctx, "go", "tool", "nm", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool nm failed: %v\n%s", err, out)
	}
	syms := string(out)

	// One needle per gated subtree; a prefix covers the package, its yang
	// sidecar, and any transitively-dropped sub-packages.
	needles := []string{
		// Phase 1 (ze_ntp)
		"internal/plugins/ntp",
		// Phase 2 (Group A)
		"internal/plugins/flowexport",
		"internal/plugins/ddos",
		"internal/plugins/anomaly",
		"internal/plugins/as112",
		"internal/plugins/geodns",
		"internal/plugins/dhcpserver",
		"internal/plugins/tftpserver",
		"internal/plugins/imageserver",
		"internal/plugins/trafficusage",
		"internal/plugins/policyroute",
		"internal/plugins/cos",
		"internal/plugins/copp",
		"internal/component/mpls",
		"internal/plugins/mpls-cmd",
		// Phase 3 (Group B). internal/exabgp is NOT a blanket needle: the
		// topics + migration library leaves stay always-on for `ze config
		// migrate`; only the bridge library and the plugin drop.
		"internal/component/tacacs",
		"internal/plugins/exabgp",
		"internal/exabgp/bridge",
		// Phase 4 (ze_bfd). Deliberately NOT the subtree prefix: bfd/api (the
		// nil-able client seam) and bfd/packet (its State/Diag source) stay
		// linked through always-on consumers (static, and ospf/bgp when on).
		// The "bfd." needle catches root-package symbols without matching
		// bfd/api or bfd/packet.
		"internal/component/bfd.",
		"internal/component/bfd/engine",
		"internal/component/bfd/session",
		"internal/component/bfd/transport",
		"internal/component/bfd/auth",
		"internal/component/bfd/cmd",
		// Phase 5 (ze_vpp): the connector component, every per-plugin backend,
		// and the vendored GoVPP library itself.
		"internal/component/vpp",
		"internal/plugins/fib/vpp",
		"internal/plugins/firewall/vpp",
		"internal/plugins/iface/vpp",
		"internal/plugins/traffic/vpp",
		"internal/plugins/static/vpp",
		"go.fd.io/govpp",
		// Phase 6 (ze_ike). Deliberately NOT the subtree prefix:
		// ike/dataplane is the shared XFRM seam OSPF programs through
		// (always-on); engine, ipsec, cmd, and the transitively-linked
		// crypto/eap/wire/transport all drop.
		"internal/component/ike/engine",
		"internal/component/ike/ipsec",
		"internal/component/ike/cmd",
		"internal/component/ike/crypto",
		"internal/component/ike/eap",
		"internal/component/ike/wire",
		"internal/component/ike/transport",
		// Phase 7 (ze_l2tp + ze_radius): the whole BNG subtree (including
		// the events contract and every BNG plugin) and the RADIUS client.
		"internal/component/l2tp",
		"internal/component/radius",
	}
	for line := range strings.SplitSeq(syms, "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				t.Fatalf("bare ze_core: binary retained symbol %q matching %q", line, needle)
			}
		}
	}

	// AC-5 link-level proof: a BGP build WITHOUT ze_bfd compiles and links the
	// reactor while the bfd engine stays absent -- BGP's only BFD coupling is
	// the always-on bfd/api nil seam, so a bfd-less BGP daemon is a valid
	// build, not a broken one.
	binBGP := filepath.Join(t.TempDir(), "ze-bgp-nobfd")
	cmdBGP := exec.CommandContext(ctx, "go", "build", "-tags", "ze_core,ze_bgp", "-o", binBGP, "./cmd/ze")
	cmdBGP.Dir = repoRoot
	cmdBGP.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmdBGP.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags ze_core,ze_bgp failed: %v\n%s", err, out)
	}
	outBGP, errBGP := exec.CommandContext(ctx, "go", "tool", "nm", binBGP).CombinedOutput()
	if errBGP != nil {
		t.Fatalf("go tool nm failed: %v\n%s", errBGP, outBGP)
	}
	symsBGP := string(outBGP)
	if !strings.Contains(symsBGP, "internal/component/bgp/reactor") {
		t.Fatal("ze_core,ze_bgp build: bgp reactor unexpectedly absent (sanity check failed)")
	}
	for _, needle := range []string{"internal/component/bfd/engine", "internal/component/bfd/session"} {
		if strings.Contains(symsBGP, needle) {
			t.Fatalf("ze_core,ze_bgp build without ze_bfd: binary retained bfd engine symbols matching %q", needle)
		}
	}

	// Mixed l2tp/radius lanes: the generator splits ze_radius into a plain
	// group (radius system auth) and a ze_l2tp && ze_radius dependent group
	// (authradius). Both advertised mixed builds must partition cleanly --
	// l2tp-with-local-auth links zero radius symbols, radius-without-BNG links
	// zero l2tp symbols. This is the only automated coverage of those lanes:
	// the per-tag registration tests carry compound constraints and compile in
	// neither mixed build.
	mixed := []struct {
		tags    string
		name    string
		present string
		absent  []string
	}{
		{"ze_core,ze_l2tp", "ze-l2tp-noradius", "internal/component/l2tp.", []string{"internal/component/radius", "authradius"}},
		{"ze_core,ze_radius", "ze-radius-nol2tp", "internal/component/radius.", []string{"internal/component/l2tp"}},
	}
	for _, m := range mixed {
		binM := filepath.Join(t.TempDir(), m.name)
		cmdM := exec.CommandContext(ctx, "go", "build", "-tags", m.tags, "-o", binM, "./cmd/ze")
		cmdM.Dir = repoRoot
		cmdM.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmdM.CombinedOutput(); err != nil {
			t.Fatalf("go build -tags %s failed: %v\n%s", m.tags, err, out)
		}
		outM, errM := exec.CommandContext(ctx, "go", "tool", "nm", binM).CombinedOutput()
		if errM != nil {
			t.Fatalf("go tool nm failed: %v\n%s", errM, outM)
		}
		symsM := string(outM)
		if !strings.Contains(symsM, m.present) {
			t.Fatalf("%s build: expected symbols matching %q absent (sanity check failed)", m.tags, m.present)
		}
		for _, needle := range m.absent {
			if strings.Contains(symsM, needle) {
				t.Fatalf("%s build: binary retained symbols matching %q", m.tags, needle)
			}
		}
	}
}
