// Design: docs/guide/config-editor.md -- offline source and destination identity
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// A loose set remains useful without storage and tells the operator that it
// cannot record history. It must not create the store as a side effect.
func TestOfflineSetWithoutStore(t *testing.T) {
	path := writeTestConfig(t, strings.Replace(showTestConfig, "router-id 1.2.3.4", "router-id 192.0.2.1", 1))
	code, diagnostic := captureStderr(t, func() int {
		return Run([]string{"set", path, "bgp", "router-id", "192.0.2.2"})
	})
	if code != exitOK {
		t.Fatalf("set exit %d: %s", code, diagnostic)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "192.0.2.2") {
		t.Fatalf("file was not updated: %s", data)
	}
	if !strings.Contains(diagnostic, "no version recorded") {
		t.Fatalf("missing history notice: %s", diagnostic)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "database")); !os.IsNotExist(err) {
		t.Fatalf("set created a store: %v", err)
	}
}

// An offline writer cannot replace the loose file while the store has an owner.
func TestOfflineSetRefusesLiveOwner(t *testing.T) {
	path := writeTestConfig(t, strings.Replace(showTestConfig, "router-id 1.2.3.4", "router-id 192.0.2.1", 1))
	store, err := storage.Create(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Test cleanup.
	if code := Run([]string{"set", path, "bgp", "router-id", "192.0.2.2"}); code == exitOK {
		t.Fatal("offline writer bypassed live owner")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "192.0.2.1") {
		t.Fatalf("refused write changed file: %s", data)
	}
}

// Equal basenames in independent source folders must remain distinct diff operands.
func TestDiffDistinctExplicitPaths(t *testing.T) {
	first := writeTestConfig(t, strings.Replace(testConfigBase, "router-id 1.1.1.1", "router-id 192.0.2.1", 1))
	second := writeTestConfig(t, strings.Replace(testConfigBase, "router-id 1.1.1.1", "router-id 192.0.2.2", 1))
	diff, code := resolveDiff(nil, []string{first, second})
	if code != exitOK {
		t.Fatalf("diff exit %d", code)
	}
	pair, ok := diff.Changed["router-id"]
	if !ok || pair.Old != "192.0.2.1" || pair.New != "192.0.2.2" {
		t.Fatalf("same-basename source comparison lost router-id: %#v", diff)
	}
}

// Import validates the complete destination name set before writing its first key.
func TestImportRefusesBasenameCollision(t *testing.T) {
	first := writeTestConfig(t, "first")
	second := writeTestConfig(t, "second")
	store := newImportBlobStore(t)
	if code := cmdImportWithStorage(store, []string{first, second}); code == exitOK {
		t.Fatal("colliding imports succeeded")
	}
	if store.Exists(filepath.Base(first)) {
		t.Fatal("collision partially published an input")
	}
}
