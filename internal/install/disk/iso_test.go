// VALIDATES: AC-11 (ISO install path), AC-12 (checksum verification for local images)
// PREVENTS: corrupted or misidentified ISO image written to disk

package disk

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func TestLocalImageToDiskVerifiesChecksum(t *testing.T) {
	dir := t.TempDir()
	imageData := []byte("raw-disk-image-content-for-test")
	imagePath := filepath.Join(dir, "test.img")
	if err := os.WriteFile(imagePath, imageData, 0o644); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(imageData)
	shaHex := textbuf.StringHex(h[:])
	checksumPath := filepath.Join(dir, "test.img.sha256")
	if err := os.WriteFile(checksumPath, []byte(shaHex), 0o644); err != nil {
		t.Fatal(err)
	}

	diskPath := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(diskPath, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := localImageToDisk(imagePath, checksumPath, diskPath); err != nil {
		t.Fatalf("localImageToDisk: %v", err)
	}

	got, err := os.ReadFile(diskPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:len(imageData)], imageData) {
		t.Fatal("disk content does not match image")
	}
}

func TestLocalImageToDiskRejectsBadChecksum(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "test.img")
	if err := os.WriteFile(imagePath, []byte("image-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	checksumPath := filepath.Join(dir, "test.img.sha256")
	wrongSHA := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if err := os.WriteFile(checksumPath, []byte(wrongSHA), 0o644); err != nil {
		t.Fatal(err)
	}

	diskPath := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(diskPath, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	err := localImageToDisk(imagePath, checksumPath, diskPath)
	if err == nil {
		t.Fatal("localImageToDisk should reject checksum mismatch")
	}
}

func TestReadExpectedSHA(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hash.txt")

	validSHA := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if err := os.WriteFile(path, []byte(validSHA+"  ze.img\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readExpectedSHA(path)
	if err != nil {
		t.Fatalf("readExpectedSHA: %v", err)
	}
	if got != validSHA {
		t.Fatalf("got %q, want %q", got, validSHA)
	}
}

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256: %v", err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
