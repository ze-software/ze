// VALIDATES: AC-13 (partial transfer detected, retry, fail closed)
// PREVENTS: truncated image written to disk as if complete

package disk

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadToFileSuccess(t *testing.T) {
	body := []byte("complete-image-data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(body) //nolint:errcheck // test
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")

	if err := downloadToFile(srv.URL, dest); err != nil {
		t.Fatalf("downloadToFile: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("got %q, want %q", got, body)
	}
}

func TestDownloadToFileRetriesOnError(t *testing.T) {
	attempts := 0
	body := []byte("eventual-success")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(body) //nolint:errcheck // test
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")

	if err := downloadToFile(srv.URL, dest); err != nil {
		t.Fatalf("downloadToFile: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestDownloadToDiskWithSHA256(t *testing.T) {
	body := []byte("image-for-sha-check")
	h := sha256.Sum256(body)
	expectedSHA := fmt.Sprintf("%x", h)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(body) //nolint:errcheck // test
	}))
	defer srv.Close()

	dir := t.TempDir()
	disk := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(disk, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := downloadToDisk(srv.URL, disk, expectedSHA); err != nil {
		t.Fatalf("downloadToDisk: %v", err)
	}

	got, err := os.ReadFile(disk)
	if err != nil {
		t.Fatalf("read disk: %v", err)
	}
	if !bytes.Equal(got[:len(body)], body) {
		t.Fatalf("disk content mismatch")
	}
}

func TestDownloadToDiskRejectsBadSHA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("wrong-data")) //nolint:errcheck // test
	}))
	defer srv.Close()

	dir := t.TempDir()
	disk := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(disk, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	wrongSHA := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	err := downloadToDisk(srv.URL, disk, wrongSHA)
	if err == nil {
		t.Fatal("downloadToDisk should reject SHA mismatch")
	}
}

func TestDownloadToDiskPartialTransferFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte("short")) //nolint:errcheck // test
	}))
	defer srv.Close()

	dir := t.TempDir()
	disk := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(disk, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	goodSHA := fmt.Sprintf("%x", sha256.Sum256(make([]byte, 1000)))
	err := downloadToDisk(srv.URL, disk, goodSHA)
	if err == nil {
		t.Fatal("downloadToDisk should detect partial transfer via SHA mismatch")
	}
}
