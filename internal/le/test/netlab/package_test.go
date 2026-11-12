package testnetlab

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: a downloaded lab contains its daemon definition and templates,
// and exporting it cannot overwrite an operator's existing working directory.
func TestPackageIsIndependentAndRefusesExistingDirectory(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	for _, name := range []string{"Dockerfile.tools", "README.md"} {
		writeFixtureFile(t, filepath.Join(fixture.root, "contrib/netlab", name), name+"\n", 0o644)
	}
	destination := filepath.Join(t.TempDir(), "lab")
	if err := fixture.checker.Package(destination); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"topology.yml", "topology-defaults.yml", "templates/ze/ze.j2", "Dockerfile.tools", "README.md"} {
		body, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil || len(body) == 0 {
			t.Errorf("package member %s: %q, %v", name, body, err)
		}
	}
	if len(fixture.process.commands) != 0 {
		t.Fatal("packaging unexpectedly requires an installed netlab")
	}
	marker := filepath.Join(destination, "operator-data")
	writeFixtureFile(t, marker, "keep me", 0o644)
	if err := fixture.checker.Package(destination); err == nil || !strings.Contains(err.Error(), "exists") {
		t.Fatalf("existing destination was not refused: %v", err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "keep me" {
		t.Fatalf("existing operator data changed: %q, %v", body, err)
	}
}
