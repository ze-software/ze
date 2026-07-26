package appliance

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWritesManifest(t *testing.T) {
	dir := t.TempDir()
	m := &BuildManifest{
		Appliance:   "edge-01",
		Timestamp:   "20260427-143022",
		ZeVersion:   "0.8.3",
		Arch:        "amd64",
		ConfigHash:  "sha256:abc123",
		Image:       "ze-20260427-143022.img",
		ImageSHA256: "def456",
	}

	path := filepath.Join(dir, "build.json")
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var loaded BuildManifest
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if loaded.Appliance != "edge-01" {
		t.Errorf("appliance = %q", loaded.Appliance)
	}
	if loaded.Arch != "amd64" {
		t.Errorf("arch = %q", loaded.Arch)
	}
	if loaded.ConfigHash != "sha256:abc123" {
		t.Errorf("config-hash = %q", loaded.ConfigHash)
	}
}

func TestBuildWritesChecksum(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "ze-test.img")
	imgData := []byte("fake image content for checksum test")
	os.WriteFile(imgPath, imgData, 0o644) //nolint:errcheck,gosec // test

	checksumPath := imgPath + ".sha256"
	sum, err := WriteImageChecksum(imgPath, checksumPath)
	if err != nil {
		t.Fatalf("write checksum: %v", err)
	}

	expected := sha256.Sum256(imgData)
	expectedHex := fmt.Sprintf("%x", expected)
	if sum != expectedHex {
		t.Errorf("checksum = %q, want %q", sum, expectedHex)
	}

	content, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), expectedHex) {
		t.Error("checksum file should contain the hash")
	}
}

func TestConfigHash(t *testing.T) {
	config := "set environment log level info\n"
	hash := ConfigHash(config)
	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("hash = %q, want sha256: prefix", hash)
	}
	if len(hash) != len("sha256:")+64 {
		t.Errorf("hash length = %d, want %d", len(hash), len("sha256:")+64)
	}
}

func TestImageFileName(t *testing.T) {
	name := ImageFileName("20260427-143022")
	if name != "ze-20260427-143022.img" {
		t.Errorf("name = %q", name)
	}
}

// VALIDATES: checkPortAvailable reports a held port as unavailable and a
// released port as available.
// PREVENTS: the appliance run command starting over a port another process
// already owns, and the inverse -- refusing to start on a free port.
func TestRunDetectsPortConflict(t *testing.T) {
	// Test a real conflict, which is what the name promises and what
	// checkPortAvailable exists to detect: hold a listener, then ask about that
	// port. Previously this probed port 1 and relied on it being privileged, but
	// checkPortAvailable's only barrier is the bind itself (cmd_run.go:187-195)
	// and root binds port 1 happily -- so under the QEMU unit phase, which runs as
	// root, the call returned nil and the assertion failed. A held port is
	// unavailable to every uid.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address type = %T, want *net.TCPAddr", ln.Addr())
	}
	port := addr.Port

	if err := checkPortAvailable(port); err == nil {
		t.Errorf("port %d is held by this test's listener but checkPortAvailable reported it free", port)
	}

	// The positive half: a port nothing holds must report available, so the test
	// cannot pass by reporting every port busy.
	free, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	freeAddr, ok := free.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address type = %T, want *net.TCPAddr", free.Addr())
	}
	freePort := freeAddr.Port
	// Released on purpose so the port is free for the positive assertion.
	if cerr := free.Close(); cerr != nil {
		t.Fatalf("close free listener: %v", cerr)
	}
	if err := checkPortAvailable(freePort); err != nil {
		t.Errorf("port %d was released but checkPortAvailable reported it in use: %v", freePort, err)
	}
}
