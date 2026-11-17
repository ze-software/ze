// Design: docs/architecture/appliance/on-device-installer.md -- download with retry and integrity check
// Related: fetch.go -- ToFile, ToDisk, CheckSHA256

// VALIDATES: AC-13 (partial transfer detected, retry, fail closed)
// PREVENTS: truncated image written to disk as if complete

package fetch

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// oneByteBody hands one byte, then blocks every Read until it is closed, as
// a response body whose server stopped sending does.
type oneByteBody struct {
	sent   bool
	closed chan struct{}
}

func (b *oneByteBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		p[0] = 'a'
		return 1, nil
	}
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *oneByteBody) Close() error {
	close(b.closed)
	return nil
}

// TestStallReaderExported proves the exported stall reader, which the
// container registry client wraps each response body in: a byte that arrives
// before the timeout is handed on, and a read that then waits past the
// timeout ends with ErrStalled rather than hanging. Method: a body that sends
// one byte and then nothing until the stall closes it.
func TestStallReaderExported(t *testing.T) {
	const timeout = 100 * time.Millisecond
	sr := NewStallReader(&oneByteBody{closed: make(chan struct{})}, timeout)
	defer sr.Close()

	buf := make([]byte, 8)
	n, err := sr.Read(buf)
	if err != nil || n != 1 || buf[0] != 'a' {
		t.Fatalf("first read = %d, %v; want the one byte sent before the timeout", n, err)
	}
	started := time.Now()
	_, err = sr.Read(buf)
	waited := time.Since(started)
	if !errors.Is(err, ErrStalled) {
		t.Fatalf("read with no byte sent = %v, want ErrStalled", err)
	}
	if waited < timeout {
		t.Errorf("the stall ended the read after %v, before the %v timeout", waited, timeout)
	}
	if waited > 5*time.Second {
		t.Errorf("the stall ended the read after %v, far past the %v timeout", waited, timeout)
	}
}

func TestDownloadToFileSuccess(t *testing.T) {
	body := []byte("complete-image-data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(body) //nolint:errcheck // test
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")

	if err := ToFile(srv.URL, dest, ""); err != nil {
		t.Fatalf("ToFile: %v", err)
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

	if err := ToFile(srv.URL, dest, ""); err != nil {
		t.Fatalf("ToFile: %v", err)
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

	if err := ToDisk(srv.URL, disk, expectedSHA); err != nil {
		t.Fatalf("ToDisk: %v", err)
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
	err := ToDisk(srv.URL, disk, wrongSHA)
	if err == nil {
		t.Fatal("ToDisk should reject SHA mismatch")
	}
}

func TestDownloadToDiskStallTimeout(t *testing.T) {
	// A server that sends initial bytes then stalls must trigger an error
	// within the stall timeout, not hang forever (ISSUE-1).
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("initial-bytes")) //nolint:errcheck // test
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-done
	}))
	t.Cleanup(func() {
		close(done)
		srv.CloseClientConnections()
		srv.Close()
	})

	dir := t.TempDir()
	disk := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(disk, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	origTimeout := stallTimeout
	stallTimeout = 500 * time.Millisecond
	defer func() { stallTimeout = origTimeout }()

	err := ToDisk(srv.URL, disk, "")
	if err == nil {
		t.Fatal("ToDisk should fail on a stalled transfer")
	}
}

func TestDownloadToDiskSlowTransferNotKilled(t *testing.T) {
	// A steady slow transfer must NOT be killed by the stall timeout.
	body := []byte("slow-but-steady-image-data-here!")
	h := sha256.Sum256(body)
	expectedSHA := fmt.Sprintf("%x", h)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for i := range body {
			w.Write(body[i : i+1]) //nolint:errcheck // test
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	disk := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(disk, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	origTimeout := stallTimeout
	stallTimeout = 2 * time.Second
	defer func() { stallTimeout = origTimeout }()

	if err := ToDisk(srv.URL, disk, expectedSHA); err != nil {
		t.Fatalf("ToDisk should succeed for slow-but-steady: %v", err)
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
	err := ToDisk(srv.URL, disk, goodSHA)
	if err == nil {
		t.Fatal("ToDisk should detect partial transfer via SHA mismatch")
	}
}
