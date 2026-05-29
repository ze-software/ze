package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportRestoresAppliance(t *testing.T) {
	srcDir := t.TempDir()
	setupExportTestAppliance(t, srcDir, "lab")

	outDir := t.TempDir()
	passphrase := []byte("test-passphrase")

	archivePath, err := exportAppliance(srcDir, "lab", passphrase, outDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	dstDir := t.TempDir()
	imported, importErr := importArchive(archivePath, passphrase, dstDir, true)
	if importErr != nil {
		t.Fatalf("import: %v", importErr)
	}

	if len(imported) != 1 || imported[0] != "lab" {
		t.Errorf("imported = %v, want [lab]", imported)
	}

	cfgPath := filepath.Join(dstDir, "lab", configFileName)
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("appliance.json not restored: %v", err)
	}

	secretsPath := filepath.Join(dstDir, "lab", "secrets", "password.hash")
	if _, err := os.Stat(secretsPath); err != nil {
		t.Errorf("secrets/password.hash not restored: %v", err)
	}

	tlsPath := filepath.Join(dstDir, "lab", "secrets", "tls", "cert.pem")
	if _, err := os.Stat(tlsPath); err != nil {
		t.Errorf("secrets/tls/cert.pem not restored: %v", err)
	}

	// Excluded files should not be in the import
	imgPath := filepath.Join(dstDir, "lab", "ze-20260501-120000.img")
	if _, err := os.Stat(imgPath); err == nil {
		t.Error("image file should not be in archive")
	}

	// Validate restored config is loadable
	if _, err := LoadConfig(cfgPath); err != nil {
		t.Errorf("restored config not loadable: %v", err)
	}
}

func TestImportWrongPassphraseFails(t *testing.T) {
	srcDir := t.TempDir()
	setupExportTestAppliance(t, srcDir, "lab")

	outDir := t.TempDir()
	passphrase := []byte("correct-passphrase")

	archivePath, err := exportAppliance(srcDir, "lab", passphrase, outDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	dstDir := t.TempDir()
	wrongPassphrase := []byte("wrong-passphrase")
	_, importErr := importArchive(archivePath, wrongPassphrase, dstDir, true)
	if importErr == nil {
		t.Fatal("expected error for wrong passphrase")
	}
	if !strings.Contains(importErr.Error(), "decryption failed") {
		t.Errorf("error = %q, want 'decryption failed'", importErr.Error())
	}
}

func TestImportPromptsBeforeOverwrite(t *testing.T) {
	srcDir := t.TempDir()
	setupExportTestAppliance(t, srcDir, "lab")

	outDir := t.TempDir()
	passphrase := []byte("test-passphrase")

	archivePath, err := exportAppliance(srcDir, "lab", passphrase, outDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	dstDir := t.TempDir()
	// Create existing appliance dir
	if err := os.MkdirAll(filepath.Join(dstDir, "lab"), 0o755); err != nil {
		t.Fatal(err)
	}

	// force=false should error when target exists
	_, importErr := importArchive(archivePath, passphrase, dstDir, false)
	if importErr == nil {
		t.Fatal("expected error when target exists and force=false")
	}
	if !strings.Contains(importErr.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists' message", importErr.Error())
	}

	// force=true should succeed
	_, importErr = importArchive(archivePath, passphrase, dstDir, true)
	if importErr != nil {
		t.Fatalf("import with force: %v", importErr)
	}
}

func TestImportToNewDir(t *testing.T) {
	srcDir := t.TempDir()
	setupExportTestAppliance(t, srcDir, "lab")

	outDir := t.TempDir()
	passphrase := []byte("test-passphrase")

	archivePath, err := exportAppliance(srcDir, "lab", passphrase, outDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	newBastion := filepath.Join(t.TempDir(), "new-bastion")
	if err := os.MkdirAll(newBastion, 0o755); err != nil {
		t.Fatal(err)
	}

	imported, importErr := importArchive(archivePath, passphrase, newBastion, false)
	if importErr != nil {
		t.Fatalf("import to new dir: %v", importErr)
	}

	if len(imported) != 1 || imported[0] != "lab" {
		t.Errorf("imported = %v, want [lab]", imported)
	}

	cfgPath := filepath.Join(newBastion, "lab", configFileName)
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("appliance.json not at new bastion: %v", err)
	}
}
