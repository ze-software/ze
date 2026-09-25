// Design: docs/architecture/testing/ci-format.md -- the harness is le's, and ze's roots stay ze's
//
//go:build ze_core && ze_distro && ze_bgp && ze_exabgp && ze_isis && ze_l2tp && ze_ospf && ze_tacacs && !ze_appliance && !ze_setup && !ze_le

package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// updateZeRoots rewrites the golden root list instead of comparing with it.
var updateZeRoots = flag.Bool("update-ze-roots", false, "rewrite testdata/ze-roots-distro.golden")

// zeRootsGolden is the root set of a ze_core ze_distro build, recorded before
// the harness became `le test <name>` (spec-le-subject-first-command-tree,
// D-8) and rewritten only with -update-ze-roots.
const zeRootsGolden = "testdata/ze-roots-distro.golden"

// TestZeRootsUnchangedByTheHarness proves AC-33: moving the harness commands
// out of the root registry and into le leaves the root set of a shipped ze
// build as it was. The method is a golden list: it was written from this test
// on the tree BEFORE Phase 1d, and this build of the same test compares the
// live registry against it.
//
// The golden was written with every gate of feature-gates.txt, and six gates
// add a root (bgp, exabgp, isis, l2tp, ospf, tacacs), so the build constraint
// names those six: a build without them would compare a different tag set,
// and the file does not compile into it.
func TestZeRootsUnchangedByTheHarness(t *testing.T) {
	ensureLocalCommandsRegistered()

	roots := registry.ListRoot()
	names := make([]string, 0, len(roots))
	for _, root := range roots {
		names = append(names, root.Name)
	}
	live := strings.Join(names, "\n") + "\n"

	if *updateZeRoots {
		if err := os.MkdirAll(filepath.Dir(zeRootsGolden), 0o755); err != nil {
			t.Fatalf("create testdata: %v", err)
		}
		if err := os.WriteFile(zeRootsGolden, []byte(live), 0o600); err != nil {
			t.Fatalf("write %s: %v", zeRootsGolden, err)
		}
		return
	}

	recorded, err := os.ReadFile(zeRootsGolden)
	if err != nil {
		t.Fatalf("read %s: %v", zeRootsGolden, err)
	}
	if string(recorded) != live {
		t.Errorf("ze roots differ from %s:\nrecorded:\n%s\nlive:\n%s", zeRootsGolden, recorded, live)
	}
}
