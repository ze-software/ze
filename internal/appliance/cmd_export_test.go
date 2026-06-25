package appliance

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupExportTestAppliance(t *testing.T, dir, name string) {
	t.Helper()
	appDir := filepath.Join(dir, name)
	secretsDir := filepath.Join(appDir, "secrets", "tls")
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig(name)
	if err := saveConfig(filepath.Join(appDir, configFileName), &cfg); err != nil {
		t.Fatal(err)
	}

	for _, f := range []struct{ path, content string }{
		{filepath.Join(appDir, "ze.conf"), "router bgp 65000\n"},
		{filepath.Join(appDir, "build.json"), `{"appliance":"` + name + `"}`},
		{filepath.Join(appDir, "secrets", "password.hash"), "fakehash"},
		{filepath.Join(appDir, "secrets", "update.token"), "faketoken"},
		{filepath.Join(appDir, "secrets", ".encrypted"), ""},
		{filepath.Join(appDir, "secrets", "tls", "cert.pem"), "fakecert"},
		{filepath.Join(appDir, "secrets", "tls", "key.pem"), "fakekey"},
	} {
		if err := os.WriteFile(f.path, []byte(f.content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Create files that should be excluded
	if err := os.WriteFile(filepath.Join(appDir, "ze-20260501-120000.img"), []byte("bigimage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "ze-20260501-120000.img.sha256"), []byte("deadbeef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "database.zefs"), []byte("zefsdata"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExportCreatesArchive(t *testing.T) {
	dir := t.TempDir()
	setupExportTestAppliance(t, dir, "lab")

	outDir := t.TempDir()
	passphrase := []byte("test-passphrase")

	archivePath, err := exportAppliance(dir, "lab", passphrase, outDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if filepath.Base(archivePath) != "lab.ze.enc" {
		t.Errorf("archive name = %q, want lab.ze.enc", filepath.Base(archivePath))
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("archive is empty")
	}

	// Verify it's encrypted (decryptable with correct passphrase)
	_, err = Decrypt(data, passphrase)
	if err != nil {
		t.Fatalf("archive not decryptable: %v", err)
	}
}

func TestExportAllCreatesArchive(t *testing.T) {
	dir := t.TempDir()
	setupExportTestAppliance(t, dir, "edge-01")
	setupExportTestAppliance(t, dir, "edge-02")

	// Create a _shared dir and dotdir that should be skipped
	if err := os.MkdirAll(filepath.Join(dir, "_shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	passphrase := []byte("test-passphrase")

	archivePath, err := exportAll(dir, passphrase, outDir)
	if err != nil {
		t.Fatalf("export --all: %v", err)
	}

	base := filepath.Base(archivePath)
	if !strings.HasPrefix(base, "appliances-") || !strings.HasSuffix(base, ".ze.enc") {
		t.Errorf("archive name = %q, want appliances-<timestamp>.ze.enc", base)
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}

	tarBytes, err := Decrypt(data, passphrase)
	if err != nil {
		t.Fatalf("archive not decryptable: %v", err)
	}

	// Verify both appliances are in the tar
	entries := tarEntryNames(t, tarBytes)
	hasEdge01, hasEdge02 := false, false
	for _, e := range entries {
		if strings.HasPrefix(e, "edge-01/") {
			hasEdge01 = true
		}
		if strings.HasPrefix(e, "edge-02/") {
			hasEdge02 = true
		}
	}
	if !hasEdge01 {
		t.Error("archive missing edge-01/")
	}
	if !hasEdge02 {
		t.Error("archive missing edge-02/")
	}
}

func tarEntryNames(t *testing.T, tarBytes []byte) []string {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		names = append(names, hdr.Name)
	}
	return names
}

func TestExportImportRoundtrip(t *testing.T) {
	srcDir := t.TempDir()
	setupExportTestAppliance(t, srcDir, "lab")

	outDir := t.TempDir()
	passphrase := []byte("roundtrip-pass")

	archivePath, err := exportAppliance(srcDir, "lab", passphrase, outDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	dstDir := t.TempDir()
	_, importErr := importArchive(archivePath, passphrase, dstDir, false)
	if importErr != nil {
		t.Fatalf("import: %v", importErr)
	}

	// Compare every exported file
	exportedFiles := []string{
		configFileName,
		"ze.conf",
		"build.json",
		filepath.Join("secrets", ".encrypted"),
		filepath.Join("secrets", "password.hash"),
		filepath.Join("secrets", "update.token"),
		filepath.Join("secrets", "tls", "cert.pem"),
		filepath.Join("secrets", "tls", "key.pem"),
	}

	for _, rel := range exportedFiles {
		srcPath := filepath.Join(srcDir, "lab", rel)
		dstPath := filepath.Join(dstDir, "lab", rel)

		srcData, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			t.Errorf("read source %s: %v", rel, readErr)
			continue
		}
		dstData, readErr := os.ReadFile(dstPath)
		if readErr != nil {
			t.Errorf("read restored %s: %v", rel, readErr)
			continue
		}

		if !bytes.Equal(srcData, dstData) {
			t.Errorf("%s content mismatch: src=%d bytes, dst=%d bytes", rel, len(srcData), len(dstData))
		}
	}

	// Verify excluded files are absent
	for _, excluded := range []string{"ze-20260501-120000.img", "ze-20260501-120000.img.sha256", "database.zefs"} {
		if _, err := os.Stat(filepath.Join(dstDir, "lab", excluded)); err == nil {
			t.Errorf("excluded file %s present in import", excluded)
		}
	}
}

func TestExportRequiresPassphrase(t *testing.T) {
	dir := t.TempDir()
	setupExportTestAppliance(t, dir, "lab")

	outDir := t.TempDir()

	_, err := exportAppliance(dir, "lab", nil, outDir)
	if err == nil {
		t.Fatal("expected error for nil passphrase")
	}
	if !strings.Contains(err.Error(), "passphrase") {
		t.Errorf("error = %q, want passphrase-related message", err.Error())
	}
}
