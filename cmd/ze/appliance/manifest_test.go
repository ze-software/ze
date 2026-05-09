package appliance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

func TestRunDetectsPortConflict(t *testing.T) {
	err := checkPortAvailable(1)
	if err == nil {
		t.Error("port 1 should not be available (privileged)")
	}
}
