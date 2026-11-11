// Design: docs/contributing/patching-the-standard-library.md -- real patched vendor publication.
package setup

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/vendorpatch"
)

const (
	nativeVendorUpstream = "package netlink\nconst Upstream = 42\n"
	nativeVendorPatched  = "package netlink\nconst Upstream = 43\n"
)

// realVendorFixture creates only synthetic local modules and a real patch. Go
// vendors their actual files and git applies the hunk; Shell has no fake Exec.
func realVendorFixture(t *testing.T) (root, patch string) {
	t.Helper()
	for _, name := range []string{"go", "git"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("real vendor publication proof requires %s: %v", name, err)
		}
	}
	root, upstream := t.TempDir(), t.TempDir()
	writeRealVendorFile(t, filepath.Join(upstream, "go.mod"), "module "+fixtureModule+"\n\ngo 1.27.0\n", 0o600)
	writeRealVendorFile(t, filepath.Join(upstream, "upstream.go"), nativeVendorUpstream, 0o600)
	writeRealVendorFile(t, filepath.Join(root, "go.mod"), "module vendorfixture\n\ngo 1.27.0\nrequire "+fixtureModule+" "+fixtureVersion+"\nreplace "+fixtureModule+" "+fixtureVersion+" => "+strconv.Quote(filepath.ToSlash(upstream))+"\n", 0o600)
	writeRealVendorFile(t, filepath.Join(root, "main.go"), "package vendorfixture\nimport \""+fixtureModule+"\"\nconst Value = netlink.Upstream\n", 0o600)
	patch = vendorpatch.Dir + "/native-owned.patch"
	target := "vendor/" + fixtureModule + "/upstream.go"
	body := fixtureStamp + "diff --git a/" + target + " b/" + target + "\n--- a/" + target + "\n+++ b/" + target + "\n@@ -1,2 +1,2 @@\n package netlink\n-const Upstream = 42\n+const Upstream = 43\n"
	writeRealVendorFile(t, filepath.Join(root, filepath.FromSlash(patch)), body, 0o600)
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GIT_CEILING_DIRECTORIES", root)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	return root, patch
}

func writeRealVendorFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func assertRealVendorPatch(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "vendor", filepath.FromSlash(fixtureModule), "upstream.go")
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != nativeVendorPatched {
		t.Errorf("actual published/staged dependency bytes = %q, want the real applied patch %q", actual, nativeVendorPatched)
	}
	modules, err := vendorpatch.Vendored(root)
	if err != nil {
		t.Fatal(err)
	}
	if modules[fixtureModule] != fixtureVersion {
		t.Errorf("actual vendored module version = %q, want %s", modules[fixtureModule], fixtureVersion)
	}
}

// TestVendoringPublishesActualPatchedReplacement reaches the native supported
// vendor producer with real Go output and real git application. It checks final
// bytes rather than only command order, then proves the obsolete tree is gone.
// MUTATION: replace only the forward git apply with another --check invocation;
// Go's unpatched constant 42 is then published instead of the required 43.
func TestVendoringPublishesActualPatchedReplacement(t *testing.T) {
	root, patch := realVendorFixture(t)
	old := filepath.Join(root, "vendor", "owned-old.go")
	writeRealVendorFile(t, old, "package old\nconst Original = true\n", 0o600)
	report := &Report{}
	setup := &Setup{Root: root, Shell: &Shell{Ctx: t.Context()}}
	if !setup.vendorGoDeps(report) {
		t.Fatalf("native patched replacement refused:\n%s", report.Text())
	}
	assertRealVendorPatch(t, root)
	if _, err := os.Lstat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("successful publication retained obsolete original bytes: %v", err)
	}
	if !strings.Contains(report.Text(), "applied vendor patch: "+patch) {
		t.Errorf("native patch application is not reported:\n%s", report.Text())
	}
	stages, err := filepath.Glob(filepath.Join(root, ".le-vendor-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 0 {
		t.Errorf("successful publication left owned staging directories: %v", stages)
	}
}

// TestVendoringPublicationRefusalRetainsPatchedCandidate gets past real Go,
// stamp validation and git apply, then refuses a regular-file live vendor path.
// The old file and the fully patched candidate both remain observable.
func TestVendoringPublicationRefusalRetainsPatchedCandidate(t *testing.T) {
	root, patch := realVendorFixture(t)
	live := filepath.Join(root, "vendor")
	const original = "owned regular-file vendor sentinel\n"
	writeRealVendorFile(t, live, original, 0o600)
	report := &Report{}
	setup := &Setup{Root: root, Shell: &Shell{Ctx: t.Context()}}
	if setup.vendorGoDeps(report) {
		t.Fatalf("publication replaced a non-directory live vendor path:\n%s", report.Text())
	}
	for _, want := range []string{"applied vendor patch: " + patch, "FAIL publish patched vendor tree:", "not a directory; refusing replacement", "Preserved vendor publication evidence:"} {
		if !strings.Contains(report.Text(), want) {
			t.Errorf("publication refusal omitted %q:\n%s", want, report.Text())
		}
	}
	actual, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(live)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || string(actual) != original {
		t.Errorf("publication refusal changed the original file: mode=%v bytes=%q", info.Mode(), actual)
	}
	stages, err := filepath.Glob(filepath.Join(root, ".le-vendor-*"))
	if err != nil || len(stages) != 1 {
		t.Fatalf("publication refusal retained %v staging directories, want one: %v", stages, err)
	}
	assertRealVendorPatch(t, stages[0])
	if !strings.Contains(report.Text(), stages[0]) {
		t.Errorf("retained recovery path is absent from the actual report:\n%s", report.Text())
	}
	if _, err := os.Lstat(filepath.Join(stages[0], "previous")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("refused non-directory was moved into a previous tree: %v", err)
	}
	// The retained stage is entirely under t.TempDir: its useful patched bytes
	// have been checked, and test cleanup removes it without a source archive.
}

// TestPublishVendorRestoresOriginalAfterSecondRenameFailure reaches the real
// filesystem publisher with an original tree and a candidate removed just before
// publication. The first rename succeeds; the second gets ENOENT; rollback must
// restore the complete original tree and preserve the publication failure.
// MUTATION: omit the os.Rename(previous, live) rollback after replacement fails.
func TestPublishVendorRestoresOriginalAfterSecondRenameFailure(t *testing.T) {
	root := t.TempDir()
	stage, err := os.MkdirTemp(root, ".le-vendor-")
	if err != nil {
		t.Fatal(err)
	}
	files := []struct {
		name string
		body string
		mode os.FileMode
	}{
		{"original.go", "package original\nconst Preserved = true\n", 0o600},
		{"nested/original.txt", "second owned original\n", 0o400},
	}
	for _, file := range files {
		writeRealVendorFile(t, filepath.Join(root, "vendor", file.name), file.body, file.mode)
	}
	candidate := filepath.Join(stage, "vendor")
	writeRealVendorFile(t, filepath.Join(candidate, "candidate.go"), nativeVendorPatched, 0o600)
	if err := os.RemoveAll(candidate); err != nil {
		t.Fatal(err)
	}
	err = publishVendor(patchedVendor{root: root, stage: stage})
	if err == nil || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "publish patched vendor tree") {
		t.Fatalf("second rename did not retain its exact publication cause: %v", err)
	}
	if strings.Contains(err.Error(), "restore original vendor tree") {
		t.Errorf("the normal rollback also failed: %v", err)
	}
	for _, file := range files {
		path := filepath.Join(root, "vendor", file.name)
		actual, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("rollback did not restore %s: %v", file.name, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(actual) != file.body || info.Mode().Perm() != file.mode {
			t.Errorf("rollback changed %s: bytes=%q mode=%v", file.name, actual, info.Mode().Perm())
		}
	}
	if _, err := os.Lstat(filepath.Join(stage, "previous")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("successful rollback left the original under its temporary name: %v", err)
	}
}
