// VALIDATES: AC-27, the vendor review stamp: parsed from the leading text,
// compared with vendor/modules.txt, refused when stale, missing, unknown or
// unreadable.
// PREVENTS: a vendor patch applied over a module version nobody reviewed.

package vendorpatch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/go/stdpatch"
)

// vendorFixture writes a checkout holding vendor/modules.txt and one patch
// with the given leading text, and answers the root and the patch path.
func vendorFixture(t *testing.T, leading string) (root, relative string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "vendor"), 0o750); err != nil {
		t.Fatal(err)
	}
	modules := "# github.com/vishvananda/netlink v1.3.1\n## explicit\ngithub.com/vishvananda/netlink\n"
	if err := os.WriteFile(filepath.Join(root, "vendor", "modules.txt"), []byte(modules), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(Dir)), 0o750); err != nil {
		t.Fatal(err)
	}
	relative = Dir + "/fixture.patch"
	body := leading + "diff --git a/vendor/x.go b/vendor/x.go\n--- a/vendor/x.go\n+++ b/vendor/x.go\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, relative
}

// TestVendorStampIsComparedWithModulesTxt proves CheckStamp: equal passes;
// a stale version, a missing stamp, an unknown module and an unreadable
// patch each refuse, naming what they found and ending with the command line.
func TestVendorStampIsComparedWithModulesTxt(t *testing.T) {
	for name, tc := range map[string]struct {
		leading string
		want    []string
	}{
		"equal":   {leading: "Ze-Reviewed-Against: github.com/vishvananda/netlink v1.3.1\n"},
		"stale":   {leading: "Ze-Reviewed-Against: github.com/vishvananda/netlink v1.3.0\n", want: []string{"v1.3.0", "v1.3.1", "github.com/vishvananda/netlink"}},
		"missing": {leading: "no stamp here\n", want: []string{"Ze-Reviewed-Against"}},
		"unknown": {leading: "Ze-Reviewed-Against: example.com/absent v1.0.0\n", want: []string{"example.com/absent", "holds no"}},
	} {
		t.Run(name, func(t *testing.T) {
			root, relative := vendorFixture(t, tc.leading)
			vendored, err := Vendored(root)
			if err != nil {
				t.Fatal(err)
			}
			err = CheckStamp(root, relative, vendored)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("an equal stamp refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("the stamp was accepted")
			}
			for _, want := range append(tc.want, relative) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal does not name %q:\n%v", want, err)
				}
			}
			if !strings.HasSuffix(err.Error(), "Then run: "+repairCommand) {
				t.Errorf("refusal does not end with the command line:\n%v", err)
			}
		})
	}

	t.Run("unreadable", func(t *testing.T) {
		root, _ := vendorFixture(t, "Ze-Reviewed-Against: github.com/vishvananda/netlink v1.3.1\n")
		dangling := Dir + "/dangling.patch"
		if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, filepath.FromSlash(dangling))); err != nil {
			t.Fatal(err)
		}
		paths, err := Patches(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 2 {
			t.Fatalf("Patches answered %v, want the dangling patch in the set", paths)
		}
		err = CheckStamp(root, dangling, map[string]string{})
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("an unreadable patch answered %v, want a not-exist refusal", err)
		}
	})

	t.Run("no-stamp-sentinel", func(t *testing.T) {
		if _, _, err := Stamp([]byte("diff --git a/x b/x\n")); !errors.Is(err, stdpatch.ErrNoStamp) {
			t.Fatalf("Stamp answered %v, want ErrNoStamp", err)
		}
	})
}
