// Design: docs/contributing/patching-the-standard-library.md -- discrimination inherits the patched GOROOT
package rfc

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/go/stdpatch"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// VALIDATES: AC-20 for discrimination: the environment a functional
// observation's binaries are built under, and an interop child's, select the
// patched GOROOT with GOTOOLCHAIN=local and carry exactly one -overlay, the
// observation's own.
// PREVENTS: a broken build compiled against the real GOROOT, where
// internal/core/eap does not build, so every red reads as the break's.
func TestDiscriminateEnvironmentCarriesThePatchedGOROOT(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := stdpatch.Activate(root)
	if err != nil {
		t.Fatal(err)
	}
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")

	check := func(t *testing.T, environ []string) {
		t.Helper()
		if got := effective(environ, "GOROOT"); got != tree {
			t.Errorf("GOROOT is %q, want the tree %q", got, tree)
		}
		if got := effective(environ, "GOTOOLCHAIN"); got != "local" {
			t.Errorf("GOTOOLCHAIN is %q, want local", got)
		}
		flags := effective(environ, "GOFLAGS")
		if n := strings.Count(flags, "-overlay"); n != 1 || !strings.Contains(flags, "-overlay="+overlay) {
			t.Errorf("GOFLAGS is %q, want exactly one -overlay naming %s", flags, overlay)
		}
	}

	t.Run("functional", func(t *testing.T) {
		restore, err := setGoFlagsOverlay(overlay)
		if err != nil {
			t.Fatal(err)
		}
		defer restore()
		check(t, testfunctional.BinarySet{Dir: t.TempDir()}.Environment(toolchain))
	})
	t.Run("interop", func(t *testing.T) {
		runner := &observationRunner{toolchain: toolchain, carrier: Carrier{Name: "interop", Kind: kindInterop}}
		check(t, runner.environment(overlay))
	})
}

// effective answers the value os/exec gives a child for key: the last entry.
func effective(environ []string, key string) string {
	value := ""
	for _, entry := range environ {
		if rest, found := strings.CutPrefix(entry, key+"="); found {
			value = rest
		}
	}
	return value
}
