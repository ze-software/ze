// Design: docs/contributing/patching-the-standard-library.md -- actual stale-version refusal preserves the original vendor tree.
package setup

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestVendoringStaleStampPreservesOwnedPatch vendors a real, version-specific
// local dependency, so Go produces modules.txt and CheckStamp must name the
// actual stale/current-version mismatch before the original tree changes.
func TestVendoringStaleStampPreservesOwnedPatch(t *testing.T) {
	root := t.TempDir()
	upstream := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(upstream, "go.mod"), "module "+fixtureModule+"\n\ngo 1.27.0\n")
	write(filepath.Join(upstream, "upstream.go"), "package netlink\nconst Upstream = 42\n")
	writeVendorPatchSet(t, root, map[string]string{
		"owned.patch": "Ze-Reviewed-Against: " + fixtureModule + " v1.3.0\n",
	})
	write(filepath.Join(root, "go.mod"), "module vendorfixture\n\ngo 1.27.0\nrequire "+fixtureModule+" "+fixtureVersion+"\nreplace "+fixtureModule+" "+fixtureVersion+" => "+strconv.Quote(filepath.ToSlash(upstream))+"\n")
	write(filepath.Join(root, "main.go"), "package vendorfixture\nimport \""+fixtureModule+"\"\nconst Value = netlink.Upstream\n")
	patched := filepath.Join(root, "vendor", "x.go")
	const owned = "package patched\nconst OwnedPatch = true\n"
	write(patched, owned)
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	report := &Report{}
	setup := &Setup{Root: root, Shell: &Shell{Ctx: t.Context()}}
	if setup.vendorGoDeps(report) {
		t.Fatalf("stale stamp accepted: %s", report.Text())
	}
	for _, want := range []string{"owned.patch", fixtureModule, "v1.3.0", fixtureVersion, "was reviewed against"} {
		if !strings.Contains(report.Text(), want) {
			t.Errorf("stale-version refusal omitted %q: %s", want, report.Text())
		}
	}
	if strings.Contains(report.Text(), "FAIL read vendored module versions") {
		t.Fatalf("fixture failed before CheckStamp: %s", report.Text())
	}
	body, err := os.ReadFile(patched)
	if err != nil {
		t.Fatalf("stale stamp refusal destroyed original patch: %v", err)
	}
	if string(body) != owned {
		t.Fatalf("stale stamp refusal changed original patch: got %q want %q", body, owned)
	}
}
