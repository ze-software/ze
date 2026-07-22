// Design: ai/rules/feature-gate-registration.md -- ze_bmp/ze_mrt symbol-drop + dependent-gate proof

package hub

// VALIDATES:
//  (1) a bare ze_core binary links zero bmp / mrt / internal-mrt / msgtype
//      symbols -- the msgtype leaf drops by DCE once neither the BGP engine nor
//      MRT references it (its drop-condition is ze_bgp || ze_mrt);
//  (2) the DEPENDENT gate holds -- a build requesting ze_bmp WITHOUT ze_bgp
//      links neither BMP nor the BGP engine, because all_ze_bmp.go is guarded
//      //go:build ze_bgp && ze_bmp, so ze_bmp alone cannot drag the engine back
//      in through BMP's blank import.
// PREVENTS: a regression where BMP or MRT (or the msgtype leaf they share) leaks
// into a hardened build, or where the dependent gate degrades so that requesting
// ze_bmp pulls in the whole internal/component/bgp subtree.

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildTag_Gate11_AbsentBinaryDropsSymbols(t *testing.T) {
	// test-relax: -short guard only; this test still runs in full (make ze-verify
	// passes no -short). It builds and links the ze binary, so opt-in -short runs
	// skip it for speed. No coverage is lost in the verify/CI suite.
	if testing.Short() {
		t.Skip("builds the ze binary (slow); skipped under -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	repoRoot := filepath.Join("..", "..", "..")

	build := func(tags, name string) string {
		bin := filepath.Join(t.TempDir(), name)
		cmd := exec.CommandContext(ctx, "go", "build", "-tags", tags, "-o", bin, "./cmd/ze")
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build -tags %q failed: %v\n%s", tags, err, out)
		}
		return bin
	}
	symbols := func(bin string) string {
		out, err := exec.CommandContext(ctx, "go", "tool", "nm", bin).CombinedOutput()
		if err != nil {
			t.Fatalf("go tool nm failed: %v\n%s", err, out)
		}
		return string(out)
	}
	assertAbsent := func(what, syms string, needles []string) {
		for line := range strings.SplitSeq(syms, "\n") {
			for _, needle := range needles {
				if strings.Contains(line, needle) {
					t.Fatalf("%s: binary retained symbol %q matching %q", what, line, needle)
				}
			}
		}
	}

	// (1) bare ze_core: bmp, mrt, the internal/mrt format library, and the shared
	// msgtype leaf are all absent (nothing references them).
	assertAbsent("bare ze_core", symbols(build("ze_core", "ze-core")), []string{
		"internal/component/bgp/plugins/bmp",
		"internal/plugins/mrt.", "internal/plugins/mrt/",
		"codeberg.org/thomas-mangin/ze/internal/mrt.",
		"internal/core/bgp/msgtype.",
	})

	// (2) dependent gate: ze_bmp WITHOUT ze_bgp links neither BMP nor the engine.
	assertAbsent("ze_bmp without ze_bgp", symbols(build("ze_core,ze_bmp", "ze-bmp-nobgp")), []string{
		"internal/component/bgp/plugins/bmp",
		"internal/component/bgp/reactor.",
		"internal/component/bgp/message.",
	})
}
