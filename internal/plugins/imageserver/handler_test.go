// Design: plan/spec-install-3-image-server.md -- image server handler tests

package imageserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func closeBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if err := resp.Body.Close(); err != nil {
		t.Logf("close body: %v", err)
	}
}

func testGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func newTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	imageDir := t.TempDir()
	bootDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(imageDir, "gokrazy.img"), []byte("DISK_IMAGE_DATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bootDir, "vmlinuz"), []byte("KERNEL_DATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bootDir, "initrd"), []byte("INITRD_DATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bootDir, "ipxe.cfg"), []byte("#!ipxe\nkernel http://server/install/boot/vmlinuz\ninitrd http://server/install/boot/initrd\nboot\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		BootDirectory:  bootDir,
	}
	return newMux(cfg)
}

func TestServeImage(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/image/gokrazy.img")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "DISK_IMAGE_DATA" {
		t.Errorf("body = %q, want DISK_IMAGE_DATA", string(body))
	}
}

func TestServeImageRange(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/install/image/gokrazy.img", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-3")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "DISK" {
		t.Errorf("body = %q, want DISK", string(body))
	}
}

func TestServeImageNotFound(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/image/nonexistent.img")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestServeImagePathTraversal(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	paths := []string{
		"/install/image/../../etc/passwd",
		"/install/image/../../../etc/passwd",
		"/install/image/..%2f..%2fetc%2fpasswd",
	}

	for _, p := range paths {
		resp := testGet(t, ts.URL+p)
		closeBody(t, resp)
		if resp.StatusCode == http.StatusOK {
			t.Errorf("GET %s: status = 200, expected non-200", p)
		}
	}
}

func TestServeBootFile(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/vmlinuz")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "KERNEL_DATA" {
		t.Errorf("body = %q, want KERNEL_DATA", string(body))
	}
}

func TestServeBootInitrd(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/initrd")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "INITRD_DATA" {
		t.Errorf("body = %q, want INITRD_DATA", string(body))
	}
}

func TestServeIPXEConfig(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/ipxe.cfg")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Error("expected non-empty iPXE config")
	}
}

func TestServeBootNotFound(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/missing")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestServeBootPathTraversal(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/../../etc/passwd")
	defer closeBody(t, resp)

	if resp.StatusCode == http.StatusOK {
		t.Error("path traversal should not return 200")
	}
}

func TestServeImageEmptyName(t *testing.T) {
	t.Parallel()

	mux := newTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/image/")
	defer closeBody(t, resp)

	if resp.StatusCode == http.StatusOK {
		t.Error("empty name should not return 200")
	}
}
