// VALIDATES: AC-26 and AC-27 through `le go patch-stamps check`: both patch
// families over fixture trees, red and green, and the checkout's own patches
// green.
// PREVENTS: a toolchain bump or a re-vendor that ./le verify passes while a
// patch nobody re-reviewed still applies.

package patchstamps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/go/stdpatch"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/vendorpatch"
)

// stampFixture writes a checkout pinning go 1.27.2 with one standard-library
// patch stamped stdStamp and one vendor patch stamped vendorStamp, over a
// vendor tree holding github.com/vishvananda/netlink v1.3.1.
func stampFixture(t *testing.T, stdStamp, vendorStamp string) string {
	t.Helper()
	root := t.TempDir()
	write := func(relative, body string) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.27.2\n")
	write(stdpatch.PatchDir+"/std.patch", "Ze-Reviewed-Against: "+stdStamp+"\n\n"+
		"diff --git a/src/crypto/tls/x.go b/src/crypto/tls/x.go\n--- a/src/crypto/tls/x.go\n+++ b/src/crypto/tls/x.go\n")
	write("vendor/modules.txt", "# github.com/vishvananda/netlink v1.3.1\n## explicit\ngithub.com/vishvananda/netlink\n")
	write(vendorpatch.Dir+"/vendor.patch", "Ze-Reviewed-Against: github.com/vishvananda/netlink "+vendorStamp+"\n\n"+
		"diff --git a/vendor/x.go b/vendor/x.go\n--- a/vendor/x.go\n+++ b/vendor/x.go\n")
	return root
}

// TestPatchStampsCheckRefusesAStaleToolchainStamp proves a standard-library
// stamp is compared with both go.mod's pin and the activated GOVERSION.
func TestPatchStampsCheckRefusesAStaleToolchainStamp(t *testing.T) {
	if err := checkPatchStamps(stampFixture(t, "go1.27.2", "v1.3.1"), "go1.27.2"); err != nil {
		t.Fatalf("equal stamps refused: %v", err)
	}
	for name, tc := range map[string]struct{ stamp, active string }{
		"stale-against-pin":    {stamp: "go1.27.1", active: "go1.27.2"},
		"stale-against-active": {stamp: "go1.27.2", active: "go1.27.3"},
	} {
		t.Run(name, func(t *testing.T) {
			err := checkPatchStamps(stampFixture(t, tc.stamp, "v1.3.1"), tc.active)
			if err == nil {
				t.Fatal("the stale stamp was accepted")
			}
			for _, want := range []string{stdpatch.PatchDir + "/std.patch", tc.stamp, tc.active, "src/crypto/tls/x.go", "Then run: ./le go stdpatch goroot"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal does not name %q:\n%v", want, err)
				}
			}
		})
	}
}

// TestPatchStampsCheckRefusesAStaleVendorStamp proves a vendor stamp is
// compared with vendor/modules.txt, and that the checkout's own patches pass.
func TestPatchStampsCheckRefusesAStaleVendorStamp(t *testing.T) {
	err := checkPatchStamps(stampFixture(t, "go1.27.2", "v1.3.0"), "go1.27.2")
	if err == nil {
		t.Fatal("the stale vendor stamp was accepted")
	}
	for _, want := range []string{vendorpatch.Dir + "/vendor.patch", "v1.3.0", "v1.3.1", "github.com/vishvananda/netlink", "Then run: ./le setup install"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q:\n%v", want, err)
		}
	}

	t.Run("checkout", func(t *testing.T) {
		root, err := lepath.Root()
		if err != nil {
			t.Fatal(err)
		}
		patches, err := stdpatch.Patches(root)
		if err != nil {
			t.Fatal(err)
		}
		cache, err := stdpatch.CacheDir(root)
		if err != nil {
			t.Fatal(err)
		}
		toolchain, err := stdpatch.Resolve(root, cache, patches)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkPatchStamps(root, toolchain.Version); err != nil {
			t.Fatalf("the checkout's own patches refused: %v", err)
		}
	})
}
