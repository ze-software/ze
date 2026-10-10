package kernelbuilder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: AC-17. The GPLv2 notice for a built kernel names Linux, the version
// the build's own provenance records, and the tarball URL and SHA-256 the worker
// downloaded (kernelTarballURL, used by downloadKernelSource) and verified
// (SourceDigest, used by verifyKernelSource).
// PREVENTS: a notice typed by hand that names a kernel or a source the image
// does not carry.
func TestLinuxNoticeMatchesTheBuiltKernel(t *testing.T) {
	root := t.TempDir()
	req := Request{Root: root, Version: "7.2.9", Arch: "arm64", Profile: "runtime", Target: "runtime", Modules: "yes"}
	if err := writeProvenance("out", req, "docker"); err != nil {
		t.Fatal(err)
	}
	record, err := ReadProvenance(filepath.Join(root, "out", ProvenanceName))
	if err != nil {
		t.Fatalf("read the provenance the build wrote: %v", err)
	}
	if record.Version != req.Version {
		t.Errorf("provenance version = %q, want the built %q", record.Version, req.Version)
	}
	digest, tracked := SourceDigest(req.Version)
	if !tracked {
		t.Fatal("7.2.9 has no tracked digest")
	}
	notice := record.LinuxNotice()
	for _, want := range []string{
		"Linux kernel",
		"GNU General Public License version 2",
		"Version: 7.2.9\n",
		"Source: " + kernelTarballURL(req.Version) + "\n",
		"SHA-256: " + digest + "\n",
		"https://github.com/ze-software/ze",
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("notice does not carry %q:\n%s", want, notice)
		}
	}
}

// VALIDATES: a provenance written before the build recorded its source is
// refused, naming the field it lacks.
// PREVENTS: an image whose notice carries an empty URL or digest because its
// kernel came from an older cache entry.
func TestReadProvenanceRefusesARecordWithoutItsSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), ProvenanceName)
	old := "version=7.2.9\ntarget=runtime\nprofile=runtime\narch=arm64\nmodules=yes\nbuilder=docker\n"
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ReadProvenance(path)
	if err == nil {
		t.Fatal("a provenance with no source lines was accepted")
	}
	if !strings.Contains(err.Error(), "source-url") {
		t.Errorf("refusal %q does not name the missing source-url", err)
	}
}
