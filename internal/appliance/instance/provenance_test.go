package instance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// VALIDATES: AC-17. The GPLv2 notice for a built kernel names Linux, the version
// the build's provenance records, and the tarball URL and SHA-256 the worker
// downloaded and verified. The record is the bytes kernelbuilder's
// writeProvenance writes (TestWriteProvenanceBytes pins them), with the digest
// from kernelbuilder.SourceDigest, the declaration verifyKernelSource checks.
// PREVENTS: a notice typed by hand that names a kernel or a source the image
// does not carry.
func TestLinuxNoticeMatchesTheBuiltKernel(t *testing.T) {
	digest, tracked := kernelbuilder.SourceDigest("7.2.9")
	if !tracked {
		t.Fatal("7.2.9 has no tracked digest")
	}
	const sourceURL = "https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.2.9.tar.xz"
	path := filepath.Join(t.TempDir(), kernelbuilder.ProvenanceName)
	written := "version=7.2.9\ntarget=runtime\nprofile=runtime\narch=arm64\nmodules=yes\nbuilder=docker\n" +
		"source-url=" + sourceURL + "\nsource-sha256=" + digest + "\n"
	if err := os.WriteFile(path, []byte(written), 0o600); err != nil {
		t.Fatal(err)
	}
	record, err := readProvenance(path)
	if err != nil {
		t.Fatalf("read the provenance the build wrote: %v", err)
	}
	if record.Version != "7.2.9" {
		t.Errorf("provenance version = %q, want the built 7.2.9", record.Version)
	}
	notice := record.LinuxNotice()
	for _, want := range []string{
		"Linux kernel",
		"GNU General Public License version 2",
		"Version: 7.2.9\n",
		"Source: " + sourceURL + "\n",
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
	path := filepath.Join(t.TempDir(), kernelbuilder.ProvenanceName)
	old := "version=7.2.9\ntarget=runtime\nprofile=runtime\narch=arm64\nmodules=yes\nbuilder=docker\n"
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readProvenance(path)
	if err == nil {
		t.Fatal("a provenance with no source lines was accepted")
	}
	if !strings.Contains(err.Error(), "source-url") {
		t.Errorf("refusal %q does not name the missing source-url", err)
	}
}
