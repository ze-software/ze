//go:build linux

// Design: docs/architecture/testing/interop.md -- the native MOBIKE lab's strongSwan settings.
// Related: mobike_netns_linux.go -- writeSwanConfig.
package ipsec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: the native MOBIKE lab composes charon's settings the way the Docker
// lab mounts them: the lab-wide drop-in always, a scenario's own strongswan.conf
// only when the scenario has one (prepareScenario, ipsec.go).
// PREVENTS: the native lab refusing a scenario whose only strongSwan settings are
// the lab's, which the Docker lab runs.
func TestMOBIKENativeSwanConfigComposesLabAndOptionalScenario(t *testing.T) {
	root := t.TempDir()
	labDir := filepath.Join(root, "test", "interop-ipsec")
	if err := os.MkdirAll(labDir, 0o700); err != nil {
		t.Fatal(err)
	}
	const labMarker = "lab-wide-marker"
	if err := os.WriteFile(filepath.Join(labDir, swanLabConfig), []byte("# "+labMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("no scenario file", func(t *testing.T) {
		lab := &mobikeNativeLab{directory: t.TempDir()}
		if err := lab.writeSwanConfig(root, t.TempDir()); err != nil {
			t.Fatalf("a scenario with no strongswan.conf was refused: %v", err)
		}
		written := readFile(t, filepath.Join(lab.directory, "strongswan.conf"))
		if !strings.Contains(written, labMarker) {
			t.Fatalf("the lab-wide drop-in is missing from the composed settings:\n%s", written)
		}
	})

	t.Run("scenario file", func(t *testing.T) {
		source := t.TempDir()
		const scenarioMarker = "scenario-marker"
		if err := os.WriteFile(filepath.Join(source, "strongswan.conf"), []byte("# "+scenarioMarker+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		lab := &mobikeNativeLab{directory: t.TempDir()}
		if err := lab.writeSwanConfig(root, source); err != nil {
			t.Fatal(err)
		}
		written := readFile(t, filepath.Join(lab.directory, "strongswan.conf"))
		if strings.Index(written, labMarker) < 0 || strings.Index(written, scenarioMarker) < strings.Index(written, labMarker) {
			t.Fatalf("want the lab drop-in, then the scenario's settings:\n%s", written)
		}
	})

	t.Run("lab file missing", func(t *testing.T) {
		lab := &mobikeNativeLab{directory: t.TempDir()}
		if err := lab.writeSwanConfig(t.TempDir(), t.TempDir()); err == nil {
			t.Fatal("a checkout without the lab-wide drop-in was accepted")
		}
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path) //nolint:gosec // a path under the test's own TempDir
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
