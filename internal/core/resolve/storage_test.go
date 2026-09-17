// Design: docs/architecture/hub-architecture.md -- config-folder precedence
package resolve

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
)

// Explicit paths own their folder regardless of a pinned default. Empty and
// stdin inputs share the environment/default resolution and never auto-create.
func TestStorageForResolution(t *testing.T) {
	prior := env.Get("ze.config.dir")
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", prior); err != nil {
			t.Error(err)
		}
	})
	pinned, explicit := t.TempDir(), t.TempDir()
	if err := env.Set("ze.config.dir", pinned); err != nil {
		t.Fatal(err)
	}
	if got := StoreDir(filepath.Join(explicit, "router.conf")); got != explicit {
		t.Fatalf("explicit dir=%q", got)
	}
	for _, path := range []string{"", "-"} {
		if got := StoreDir(path); got != pinned {
			t.Fatalf("StoreDir(%q)=%q", path, got)
		}
		if store, err := StorageFor(path); !errors.Is(err, storage.ErrNoStore) || store != nil {
			t.Fatalf("absent open=%v,%v", store, err)
		}
	}
	created, err := storage.Create(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if err := created.WriteKey("meta/test/resolution", []byte("explicit")); err != nil {
		t.Fatal(err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := StorageFor(filepath.Join(explicit, "router.conf"))
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close() //nolint:errcheck // test cleanup
	value, err := opened.ReadKey("meta/test/resolution")
	if err != nil || string(value) != "explicit" {
		t.Fatalf("resolved value=%q,%v", value, err)
	}
	if err := env.Set("ze.config.dir", ""); err != nil {
		t.Fatal(err)
	}
	if got := StoreDir("-"); got != paths.DefaultConfigDir() {
		t.Fatalf("default dir=%q", got)
	}
}
