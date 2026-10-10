package kernelbuilder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: AC-8 of spec-appliance-ships-ze-kernel. A kernel tarball whose
// SHA-256 differs from the digest the request carries is refused before
// extraction, and the error names both digests; the matching tarball is used.
// Method: a pre-downloaded tarball in WorkDir, checked once with its own digest
// and once with another, through downloadKernelSource, which runs the same
// verifyKernelSource a fresh download runs before it publishes the file.
func TestDownloadKernelSourceVerifiesDigest(t *testing.T) {
	work := t.TempDir()
	content := []byte("not really a kernel")
	if err := os.WriteFile(filepath.Join(work, kernelTarballName("7.2.9")), content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	good := hex.EncodeToString(sum[:])
	bad := strings.Repeat("0", len(good))

	req := WorkerRequest{Version: "7.2.9", WorkDir: work, SourceSHA256: good, Stdout: io.Discard}
	if _, err := downloadKernelSource(context.Background(), req); err != nil {
		t.Fatalf("tarball with the pinned digest refused: %v", err)
	}

	req.SourceSHA256 = bad
	_, err := downloadKernelSource(context.Background(), req)
	if err == nil {
		t.Fatal("tarball with another digest accepted")
	}
	for _, digest := range []string{good, bad} {
		if !strings.Contains(err.Error(), digest) {
			t.Errorf("error %q does not name digest %s", err, digest)
		}
	}
}

// VALIDATES: a version with no tracked digest is refused before any download,
// so a kernel.version bump that forgets the digest cannot build from whatever
// the network serves.
func TestRunWorkerRefusesUntrackedVersion(t *testing.T) {
	var out bytes.Buffer
	err := RunWorker(context.Background(), WorkerRequest{Version: "7.0.1", Arch: "arm64", Profile: "runtime", Modules: modulesYes, WorkDir: t.TempDir(), Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "no tracked SHA-256") {
		t.Fatalf("untracked version error = %v", err)
	}
}
