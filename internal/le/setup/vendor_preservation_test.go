// Design: docs/contributing/patching-the-standard-library.md -- unsafe re-vendoring refuses before replacing owned patches.
package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVendoringRefusesBeforeLosingOwnedPatch reaches native setup vendoring in
// an owned dependency-free module. Missing staged module metadata must refuse
// while the original patched bytes remain present. This is a generic prepatch
// refusal proof, not evidence that the stale-stamp branch executed.
func TestVendoringRefusesBeforeLosingOwnedPatch(t *testing.T) {
	root := t.TempDir()
	writeVendorPatchSet(t, root, map[string]string{
		"owned.patch": "Ze-Reviewed-Against: " + fixtureModule + " v0.0.0\n",
	})
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module vendorfixture\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package vendorfixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	patched := filepath.Join(root, "vendor", "x.go")
	const owned = "package patched\nconst OwnedPatch = true\n"
	if err := os.WriteFile(patched, []byte(owned), 0o600); err != nil {
		t.Fatal(err)
	}
	// No dependency may be downloaded, and workspace/ambient flags cannot move
	// this native operation out of its owned fixture.
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	report := &Report{}
	setup := &Setup{Root: root, Shell: &Shell{Ctx: t.Context()}}
	if setup.vendorGoDeps(report) {
		t.Fatalf("unsafe vendoring succeeded:\n%s", report.Text())
	}
	if !strings.Contains(report.Text(), "owned.patch") {
		t.Errorf("refusal does not identify the owned patch:\n%s", report.Text())
	}
	body, err := os.ReadFile(patched)
	if err != nil {
		t.Fatalf("vendoring destroyed the owned patch before refusing: %v\n%s", err, report.Text())
	}
	if string(body) != owned {
		t.Fatalf("vendoring changed owned patch before refusing: got %q, want %q", body, owned)
	}
}
