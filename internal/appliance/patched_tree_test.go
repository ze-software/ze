// VALIDATES: AC-28, SetGokGoEnv refuses unless the process runs under the
// patched GOROOT, before it sets GOMODCACHE or GOPROXY, and accepts one
// without setting GOTOOLCHAIN itself; the marker path is the file the patch
// adds.
// PREVENTS: a ze appliance build from ze-setup that compiles the EAP-TLS
// authenticator against a plain GOROOT, and a marker check left naming a
// file the patch no longer adds.

package appliance

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
)

// patchedTreeFixture writes a GOROOT holding a fake bin/go that prints
// goVersion for `go env GOVERSION` and `go version`, and, when marker is true,
// the file the patched crypto/tls adds. It answers the GOROOT.
func patchedTreeFixture(t *testing.T, marker bool) string {
	const goVersion = "go1.27.2"
	t.Helper()
	goroot := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo \"go version " + goVersion + " linux/amd64\"; else echo " + goVersion + "; fi\n"
	if err := os.MkdirAll(filepath.Join(goroot, "bin"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goroot, "bin", "go"), []byte(script), 0o700); err != nil { //nolint:gosec // an executable fixture
		t.Fatal(err)
	}
	if marker {
		path := filepath.Join(goroot, filepath.FromSlash(StdpatchMarker))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package tls\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return goroot
}

// underPatchedTree sets the environment le's activation sets, over goroot.
func underPatchedTree(t *testing.T, goroot string) {
	t.Helper()
	t.Setenv("GOROOT", goroot)
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("PATH", filepath.Join(goroot, "bin"))
}

// TestSetGokGoEnvRefusesWithoutThePatchedTree proves each of the four
// conditions refuses before GOMODCACHE or GOPROXY is set, naming what it
// found and ending with the command line, and that a patched tree, also one
// spelled with a trailing slash, is accepted without SetGokGoEnv setting
// GOTOOLCHAIN.
func TestSetGokGoEnvRefusesWithoutThePatchedTree(t *testing.T) {
	refusing := map[string]struct {
		setup func(t *testing.T)
		want  string
	}{
		"no-goroot": {setup: func(t *testing.T) {
			underPatchedTree(t, patchedTreeFixture(t, true))
			t.Setenv("GOROOT", "")
		}, want: "GOROOT is not set"},
		"toolchain-not-local": {setup: func(t *testing.T) {
			underPatchedTree(t, patchedTreeFixture(t, true))
			t.Setenv("GOTOOLCHAIN", "go1.27.0")
		}, want: `GOTOOLCHAIN is "go1.27.0"`},
		"go-outside-goroot": {setup: func(t *testing.T) {
			underPatchedTree(t, patchedTreeFixture(t, true))
			t.Setenv("PATH", filepath.Join(patchedTreeFixture(t, true), "bin"))
		}, want: "not $GOROOT/bin/go"},
		"no-marker": {setup: func(t *testing.T) {
			underPatchedTree(t, patchedTreeFixture(t, false))
		}, want: "holds no " + StdpatchMarker},
	}
	for name, tc := range refusing {
		t.Run(name, func(t *testing.T) {
			tc.setup(t)
			t.Setenv("GOMODCACHE", "")
			t.Setenv("GOPROXY", "")
			err := SetGokGoEnv(filepath.Join(t.TempDir(), "modcache"))
			if err == nil {
				t.Fatal("SetGokGoEnv accepted an environment that is not the patched tree")
			}
			for _, want := range []string{tc.want, "docs/contributing/patching-the-standard-library.md", "./le build gokrazy"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal does not name %q:\n%v", want, err)
				}
			}
			if !strings.HasSuffix(err.Error(), patchedTreeCommand) {
				t.Errorf("refusal does not end with the command line:\n%v", err)
			}
			if os.Getenv("GOMODCACHE") != "" || os.Getenv("GOPROXY") != "" {
				t.Errorf("the refusal came after GOMODCACHE=%q GOPROXY=%q were set", os.Getenv("GOMODCACHE"), os.Getenv("GOPROXY"))
			}
		})
	}

	for name, spell := range map[string]func(string) string{
		"accepts":                     func(goroot string) string { return goroot },
		"accepts-trailing-slash-root": func(goroot string) string { return goroot + "/" },
	} {
		t.Run(name, func(t *testing.T) {
			goroot := patchedTreeFixture(t, true)
			underPatchedTree(t, goroot)
			t.Setenv("GOROOT", spell(goroot))
			t.Setenv("GOTOOLCHAIN", "local")
			t.Setenv("GOMODCACHE", "")
			t.Setenv("GOFLAGS", "")
			t.Setenv("GOPROXY", "")
			modcache := filepath.Join(t.TempDir(), "modcache")
			if err := SetGokGoEnv(modcache); err != nil {
				t.Fatalf("SetGokGoEnv refused the patched tree: %v", err)
			}
			if got := os.Getenv("GOMODCACHE"); got != modcache {
				t.Errorf("GOMODCACHE is %q, want %q", got, modcache)
			}
		})
	}
}

// TestApplianceMarkerPathMatchesThePatch proves StdpatchMarker is a file the
// crypto/tls patch adds, read through stdpatch (a test-only import: the
// shipped ze-setup links no le package).
func TestApplianceMarkerPathMatchesThePatch(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	patches, err := gen.Patches(root)
	if err != nil {
		t.Fatal(err)
	}
	var added []string
	for _, patch := range patches {
		added = append(added, gen.AddedFiles(patch)...)
	}
	if !slices.Contains(added, StdpatchMarker) {
		t.Fatalf("StdpatchMarker is %s, and the patches add %v", StdpatchMarker, added)
	}
}
