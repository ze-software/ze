// Design: plan/learned/811-install-3-image-server.md -- image server handler tests

package imageserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
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
	return newMux(cfg, "", "127.0.0.1")
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

func TestServeZefsDB(t *testing.T) {
	t.Parallel()

	imageDir := t.TempDir()
	bootDir := t.TempDir()
	zefsDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(imageDir, "gokrazy.img"), []byte("IMG"), 0o644); err != nil {
		t.Fatal(err)
	}

	zefsPath, err := buildZefsDB(zefsDir, "admin", "$2a$10$examplehash")
	if err != nil {
		t.Fatalf("buildZefsDB: %v", err)
	}

	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		BootDirectory:  bootDir,
	}
	mux := newMux(cfg, zefsPath, "127.0.0.1")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/database.zefs")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", resp.Header.Get("Cache-Control"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty zefs database")
	}

	// Verify the served zefs contains the expected SSH credentials.
	tmpFile := filepath.Join(t.TempDir(), "verify.zefs")
	if err := os.WriteFile(tmpFile, body, 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := zefs.Open(tmpFile)
	if err != nil {
		t.Fatalf("zefs.Open: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("close store: %v", err)
		}
	}()

	username, err := store.ReadFile(zefs.KeySSHUsername.Key("127.0.0.1", "2222"))
	if err != nil {
		t.Fatalf("read remote username: %v", err)
	}
	if string(username) != "admin" {
		t.Errorf("remote username = %q, want admin", string(username))
	}

	password, err := store.ReadFile(zefs.KeySSHPassword.Key("127.0.0.1", "2222"))
	if err != nil {
		t.Fatalf("read remote password: %v", err)
	}
	if string(password) != "$2a$10$examplehash" {
		t.Errorf("remote password = %q, want $2a$10$examplehash", string(password))
	}

	localUsername, err := store.ReadFile(zefs.KeyLocalAdminUsername.Pattern)
	if err != nil {
		t.Fatalf("read local username: %v", err)
	}
	if string(localUsername) != "admin" {
		t.Errorf("local username = %q, want admin", string(localUsername))
	}

	localPassword, err := store.ReadFile(zefs.KeyLocalAdminPassword.Pattern)
	if err != nil {
		t.Fatalf("read local password: %v", err)
	}
	if string(localPassword) != "$2a$10$examplehash" {
		t.Errorf("local password = %q, want $2a$10$examplehash", string(localPassword))
	}

	def, err := store.ReadFile(zefs.KeySSHDefault.Pattern)
	if err != nil {
		t.Fatalf("read default: %v", err)
	}
	if string(def) != "127.0.0.1/2222" {
		t.Errorf("default = %q, want 127.0.0.1/2222", string(def))
	}
}

func TestServeZefsDBNoCreds(t *testing.T) {
	t.Parallel()

	imageDir := t.TempDir()
	bootDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(imageDir, "gokrazy.img"), []byte("IMG"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		BootDirectory:  bootDir,
	}
	mux := newMux(cfg, "", "127.0.0.1")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/database.zefs")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
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

func TestServeDynamicBootIPXE(t *testing.T) {
	t.Parallel()

	imageDir := t.TempDir()
	bootDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(imageDir, "ze-20260601-120000.img"), []byte("IMG1"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		BootDirectory:  bootDir,
		ListenPort:     80,
	}
	mux := newMux(cfg, "", "198.19.255.1")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	script := string(body)

	if !strings.HasPrefix(script, "#!ipxe\n") {
		t.Error("missing iPXE shebang")
	}
	if !strings.Contains(script, "ze.server=198.19.255.1") {
		t.Errorf("missing ze.server in script:\n%s", script)
	}
	if !strings.Contains(script, "ze.image=ze-20260601-120000.img") {
		t.Errorf("missing ze.image in script:\n%s", script)
	}
	if !strings.Contains(script, "ip=dhcp") {
		t.Errorf("missing ip=dhcp in script:\n%s", script)
	}
	// Console selection is arch-aware via iPXE ${buildarch}: x86 must not carry
	// the ARM-only ttyAMA0 (it never registers and can dead-end /dev/console),
	// while arm64 keeps the full set. Exactly one branch runs on the client.
	if !strings.Contains(script, "iseq ${buildarch} arm64 && set zeconsole console=tty0 console=ttyS0,115200n8 console=ttyAMA0,115200n8 || set zeconsole console=tty0 console=ttyS0,115200n8\n") {
		t.Errorf("missing arch-aware console selection in script:\n%s", script)
	}
	if !strings.Contains(script, "panic=-1 ${zeconsole}\n") {
		t.Errorf("kernel line should reference the arch-selected ${zeconsole}:\n%s", script)
	}
	// loglevel=8 + earlycon give the installer kernel the same verbose, early
	// console output as the runtime kernel so an operator can watch the install
	// on the target's screen/serial instead of a blank screen.
	if !strings.Contains(script, "loglevel=8 earlycon panic=-1 ${zeconsole}\n") {
		t.Errorf("kernel line should carry loglevel=8 earlycon for install visibility:\n%s", script)
	}
	if strings.Contains(script, "ze.port=") {
		t.Errorf("port 80 should not include ze.port in script:\n%s", script)
	}
	// The kernel must pin to the iPXE boot NIC (ze.mac) so a second NIC on a
	// foreign network cannot hijack the install (iPXE expands ${mac}).
	if !strings.Contains(script, "ze.mac=${mac}") {
		t.Errorf("missing ze.mac=${mac} in script:\n%s", script)
	}
	// No rescue-auth configured -> the gate arg must be absent (installer
	// then fails closed).
	if strings.Contains(script, "ze.rescue-auth=") {
		t.Errorf("no rescue-auth configured: ze.rescue-auth must be absent:\n%s", script)
	}
}

func TestServeDynamicBootIPXERescueAuth(t *testing.T) {
	t.Parallel()

	imageDir := t.TempDir()
	bootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(imageDir, "ze-20260601-120000.img"), []byte("IMG1"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash := "aabbccddeeff00112233445566778899:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		BootDirectory:  bootDir,
		ListenPort:     80,
		RescueAuth:     hash,
	}
	mux := newMux(cfg, "", "198.19.255.1")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
	defer closeBody(t, resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	script := string(body)
	if !strings.Contains(script, "ze.rescue-auth="+hash) {
		t.Errorf("missing ze.rescue-auth=<value> in script:\n%s", script)
	}
	if !strings.Contains(script, "ze.mac=${mac}") {
		t.Errorf("missing ze.mac in script:\n%s", script)
	}
}

func TestServeDynamicBootIPXENonDefaultPort(t *testing.T) {
	t.Parallel()

	imageDir := t.TempDir()
	bootDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(imageDir, "ze.img"), []byte("IMG"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		BootDirectory:  bootDir,
		ListenPort:     8080,
	}
	mux := newMux(cfg, "", "198.19.255.1")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	script := string(body)

	if !strings.Contains(script, "ze.port=8080") {
		t.Errorf("missing ze.port=8080 in script:\n%s", script)
	}
	if !strings.Contains(script, ":8080/install/boot/") {
		t.Errorf("missing port in URLs:\n%s", script)
	}
}

func TestServeDynamicBootIPXENoBootDir(t *testing.T) {
	t.Parallel()

	imageDir := t.TempDir()

	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		ListenPort:     80,
	}
	mux := newMux(cfg, "", "198.19.255.1")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when boot-directory not configured", resp.StatusCode)
	}
}

func TestServeDynamicBootIPXENoServerAddr(t *testing.T) {
	t.Parallel()

	imageDir := t.TempDir()
	bootDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(imageDir, "ze.img"), []byte("IMG"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		BootDirectory:  bootDir,
		ListenPort:     80,
	}
	mux := newMux(cfg, "", "")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when serverAddr is empty (no dynamic boot.ipxe)", resp.StatusCode)
	}
}

func TestServeDynamicBootIPXEStaticOverride(t *testing.T) {
	t.Parallel()

	imageDir := t.TempDir()
	bootDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(imageDir, "ze.img"), []byte("IMG"), 0o644); err != nil {
		t.Fatal(err)
	}
	staticScript := "#!ipxe\n# custom operator script\nboot\n"
	if err := os.WriteFile(filepath.Join(bootDir, "boot.ipxe"), []byte(staticScript), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDir,
		BootDirectory:  bootDir,
		ListenPort:     80,
	}
	mux := newMux(cfg, "", "198.19.255.1")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
	defer closeBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != staticScript {
		t.Errorf("expected static override, got:\n%s", string(body))
	}
}

func TestServeDynamicBootIPXEImageDetection(t *testing.T) {
	t.Parallel()

	t.Run("latest image selected", func(t *testing.T) {
		t.Parallel()

		imageDir := t.TempDir()
		bootDir := t.TempDir()

		for _, name := range []string{"ze-20260601-120000.img", "ze-20260604-090000.img", "ze-20260602-060000.img"} {
			if err := os.WriteFile(filepath.Join(imageDir, name), []byte("IMG"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		cfg := imageConfig{
			Enabled:        true,
			ImageDirectory: imageDir,
			BootDirectory:  bootDir,
			ListenPort:     80,
		}
		mux := newMux(cfg, "", "198.19.255.1")
		ts := httptest.NewServer(mux)
		defer ts.Close()

		resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
		defer closeBody(t, resp)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "ze.image=ze-20260604-090000.img") {
			t.Errorf("expected lexicographically last image, got:\n%s", string(body))
		}
	})

	t.Run("no images returns 503", func(t *testing.T) {
		t.Parallel()

		imageDir := t.TempDir()
		bootDir := t.TempDir()

		cfg := imageConfig{
			Enabled:        true,
			ImageDirectory: imageDir,
			BootDirectory:  bootDir,
			ListenPort:     80,
		}
		mux := newMux(cfg, "", "198.19.255.1")
		ts := httptest.NewServer(mux)
		defer ts.Close()

		resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
		defer closeBody(t, resp)

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503 when no images", resp.StatusCode)
		}
	})

	t.Run("hidden dotfile .img ignored", func(t *testing.T) {
		t.Parallel()

		imageDir := t.TempDir()
		bootDir := t.TempDir()

		if err := os.WriteFile(filepath.Join(imageDir, ".img"), []byte("hidden"), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg := imageConfig{
			Enabled:        true,
			ImageDirectory: imageDir,
			BootDirectory:  bootDir,
			ListenPort:     80,
		}
		mux := newMux(cfg, "", "198.19.255.1")
		ts := httptest.NewServer(mux)
		defer ts.Close()

		resp := testGet(t, ts.URL+"/install/boot/boot.ipxe")
		defer closeBody(t, resp)

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503 when only hidden .img file exists", resp.StatusCode)
		}
	})
}
