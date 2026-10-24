// Design: ai/rules/plugins.md -- ze_bgp/ze_bmp symbol-drop + dependent-gate proof

//go:build ze_bgp && ze_bmp

package hub

// VALIDATES: the full AC-2 tag matrix of spec feature-gate-11 -- for each build
// in the matrix, `go tool nm` shows exactly the expected symbol groups:
//   msgtype   present iff ze_bgp (DCE on the shared leaf)
//   internal/mrt (format library) present iff ze_bgp
//   bmp       present iff ze_bgp && ze_bmp (the DEPENDENT gate)
//   mrt plugin present iff ze_bgp (MRT is a BGP plugin with no tag of its own)
//   bgp engine present iff ze_bgp (ze_bmp alone must NOT drag it in)
// The all-on row asserts EVERY individual needle matches, so a renamed package
// or a misspelled needle turns the test red instead of silently matching
// nothing -- including the second needle of a multi-needle group, whose
// absence assertions would otherwise defuse without failing any group-level
// presence check.
// PREVENTS: a regression where BMP or MRT (or the msgtype leaf they share)
// leaks into a hardened build; where the dependent gate degrades so that
// requesting ze_bmp pulls in the whole internal/component/bgp subtree; or
// where a needle typo defuses the absence assertions.
//
// The file is constrained to the full-tag unit pass (ze_bgp && ze_bmp via
// ZE_FEATURES) so the five `go build` link jobs run once per verify, not again
// in the bare-core pass. The binaries it builds get their tags from the table
// below, independent of this file's own constraint.
//
// TestBuildTag_Gate11_AbsentBinaryDropsSymbols is renamed to
// TestBuildTag_Gate11_SymbolMatrix and strictly strengthened: both of the old
// builds and all their absence needles are retained (rows "bare ze_core" and
// "ze_bmp without ze_bgp"), and builds plus presence assertions are added to
// cover the spec's full AC-2 matrix. When MRT moved under ze_bgp (spec
// plugin-optional-dependency-refusal), the "bgp-and-mrt" row became the
// "bgp-only" row, and the "mrt-only" row became "retired-mrt-tag": the old
// ze_mrt tag alone must link nothing of MRT or BGP.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Symbol needle groups. A group is PRESENT when any of its needles matches an
// nm line, ABSENT when none does.
var gate11SymbolGroups = map[string][]string{
	"msgtype":      {"internal/core/bgp/msgtype."},
	"internal-mrt": {"github.com/ze-software/ze/internal/mrt."},
	"bmp":          {"internal/component/bgp/plugins/bmp"},
	"mrt-plugin":   {"internal/plugins/mrt.", "internal/plugins/mrt/"},
	"bgp-engine":   {"internal/component/bgp/reactor.", "internal/component/bgp/message."},
}

func TestBuildTag_Gate11_SymbolMatrix(t *testing.T) {
	// t.Parallel() must precede the -short skip guard: tparallel does not
	// recognize a top-level t.Parallel() call that sits after an early
	// t.Skip(), and flags the parent as non-parallel while its subtests are.
	t.Parallel()
	// -short guard only; this test still runs in full verification
	// (`./le verify current` passes no -short). It builds and links five
	// ze binaries, so opt-in -short runs skip it for speed.
	if testing.Short() {
		t.Skip("builds five ze binaries (slow); skipped under -short")
	}
	// No meta-test guards the t-passing discipline below; the failure mode it
	// prevents (parent Fatalf from a subtest goroutine skipping later rows)
	// is a Go testing-framework interaction that a regression test would have
	// to reproduce by running this test under a broken build, which is
	// disproportionate. The explicit t parameters make the bug structural to
	// reintroduce.
	// t.Cleanup, not defer: the matrix rows run as parallel subtests, which
	// execute after this function body returns. A deferred cancel would kill
	// the shared context before any row builds; Cleanup runs after all
	// subtests (parallel included) complete.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancel)
	repoRoot := filepath.Join("..", "..", "..")

	// build and symbols take the SUBTEST's t explicitly. Closing over the
	// outer t would call Fatalf on the parent from a subtest goroutine, which
	// aborts the whole matrix loop (remaining rows silently never run) and
	// misattributes the build error to the parent test.
	build := func(t *testing.T, tags, name string) string {
		t.Helper()
		bin := filepath.Join(t.TempDir(), name)
		cmd := exec.CommandContext(ctx, "go", "build", "-tags", tags, "-o", bin, "./cmd/ze")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build -tags %q failed: %v\n%s", tags, err, out)
		}
		return bin
	}
	symbols := func(t *testing.T, bin string) string {
		t.Helper()
		out, err := exec.CommandContext(ctx, "go", "tool", "nm", bin).CombinedOutput()
		if err != nil {
			t.Fatalf("go tool nm failed: %v\n%s", err, out)
		}
		return string(out)
	}
	groupMatches := func(syms, group string) bool {
		for line := range strings.SplitSeq(syms, "\n") {
			for _, needle := range gate11SymbolGroups[group] {
				if strings.Contains(line, needle) {
					return true
				}
			}
		}
		return false
	}

	// The AC-2 matrix (spec feature-gate-11), plus the AC-4 dependent-gate row.
	// Row 1 uses the two gate tags directly rather than the full ZE_FEATURES
	// set: the symbol groups only depend on ze_bgp/ze_bmp.
	matrix := []struct {
		name        string
		tags        string
		present     []string
		absent      []string
		needleGuard bool
	}{
		{
			// needleGuard: every individual needle of every group must match
			// in this build (all five packages are linked), so a stale needle
			// in a multi-needle group cannot hide behind its sibling.
			name:        "all-on",
			tags:        "ze_core,ze_bgp,ze_bmp",
			present:     []string{"msgtype", "internal-mrt", "bmp", "mrt-plugin", "bgp-engine"},
			needleGuard: true,
		},
		{
			// MRT is a BGP plugin under ze_bgp: a BGP build carries it and
			// its format library, and still drops the dependent BMP gate.
			name:    "bgp-only",
			tags:    "ze_core,ze_bgp",
			present: []string{"msgtype", "internal-mrt", "mrt-plugin", "bgp-engine"},
			absent:  []string{"bmp"},
		},
		{
			// ze_mrt is no longer a gate: setting it without ze_bgp links
			// neither MRT nor the engine whose data MRT records.
			name:   "retired-mrt-tag",
			tags:   "ze_core,ze_mrt",
			absent: []string{"msgtype", "internal-mrt", "bmp", "mrt-plugin", "bgp-engine"},
		},
		{
			name:   "bare-core",
			tags:   "ze_core",
			absent: []string{"msgtype", "internal-mrt", "bmp", "mrt-plugin", "bgp-engine"},
		},
		{
			// AC-4: the dependent gate. ze_bmp WITHOUT ze_bgp links neither BMP
			// nor the engine -- all_ze_bmp.go is guarded //go:build ze_bgp && ze_bmp.
			name:   "bmp-without-bgp",
			tags:   "ze_core,ze_bmp",
			absent: []string{"bmp", "bgp-engine", "msgtype", "internal-mrt", "mrt-plugin"},
		},
	}

	for _, row := range matrix {
		t.Run(row.name, func(t *testing.T) {
			// The builds are independent; run them concurrently so the
			// wall-clock cost is the slowest single link job, not the sum.
			t.Parallel()
			syms := symbols(t, build(t, row.tags, "ze-"+row.name))
			if row.needleGuard {
				for group, needles := range gate11SymbolGroups {
					for _, needle := range needles {
						if !strings.Contains(syms, needle) {
							t.Errorf("build %q (-tags %s): needle %q of group %q matches nothing -- stale or misspelled",
								row.name, row.tags, needle, group)
						}
					}
				}
			}
			for _, group := range row.present {
				if !groupMatches(syms, group) {
					t.Errorf("build %q (-tags %s): symbol group %q expected PRESENT, no nm line matches %v",
						row.name, row.tags, group, gate11SymbolGroups[group])
				}
			}
			for _, group := range row.absent {
				if !groupMatches(syms, group) {
					continue
				}
				for line := range strings.SplitSeq(syms, "\n") {
					for _, needle := range gate11SymbolGroups[group] {
						if strings.Contains(line, needle) {
							t.Errorf("build %q (-tags %s): binary retained symbol %q matching %q",
								row.name, row.tags, line, needle)
						}
					}
				}
			}
		})
	}
}
