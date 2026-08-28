package fixture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLeDiscoveryFixtureBuildsOutsideCheckout(t *testing.T) {
	checkout, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	workspace, binary, err := temporaryLEFixtureWorkspace("le-workspace-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workspace) })

	if filepath.Clean(workspace) == filepath.Clean(checkout) {
		t.Fatal("fixture workspace is the checkout root")
	}
	if filepath.Dir(binary) != workspace || filepath.Base(binary) != "le" {
		t.Fatalf("fixture binary = %q, want <workspace>/le", binary)
	}
	launcher := filepath.Clean(filepath.Join(checkout, "..", "..", "..", "le"))
	if _, err := os.Stat(launcher); err != nil {
		t.Fatalf("fixture workspace selection changed the checkout launcher: %v", err)
	}
}
