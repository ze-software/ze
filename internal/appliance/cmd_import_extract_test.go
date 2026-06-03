package appliance

import (
	"archive/tar"
	"bytes"
	"testing"
)

// TestExtractTarRejectsSymlink ensures a crafted archive containing a symlink
// (a directory-escape vector) is rejected rather than silently skipped.
func TestExtractTarRejectsSymlink(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// A regular file is fine.
	if err := tw.WriteHeader(&tar.Header{Name: "lab/config.json", Typeflag: tar.TypeReg, Size: 2, Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	// A symlink must be rejected.
	if err := tw.WriteHeader(&tar.Header{Name: "lab/evil", Typeflag: tar.TypeSymlink, Linkname: "/etc", Mode: 0o777}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractTar(buf.Bytes(), t.TempDir()); err == nil {
		t.Fatal("extractTar accepted a symlink entry; expected rejection")
	}
}
