package local

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitLocalOperationsRespectPackageOwnership(t *testing.T) {
	prefix := t.TempDir()
	marker := filepath.Join(prefix, "share", "ze", "install-method")
	binary := filepath.Join(prefix, "bin", "ze")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("deb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("package-owned"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := RunInstall([]string{"--prefix", prefix}); code == 0 {
		t.Fatal("explicit local install replaced a package binary")
	}
	if code := RunUninstall([]string{"--prefix", prefix, "--yes"}); code == 0 {
		t.Fatal("explicit local uninstall removed a package binary")
	}
	data, err := os.ReadFile(binary)
	if err != nil || string(data) != "package-owned" {
		t.Fatalf("package binary changed: %q %v", data, err)
	}
}
