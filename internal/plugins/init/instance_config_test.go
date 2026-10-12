package init

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: a non-seed init names its discovered config after the instance
// name it stored, so a bare `ze start` on that store reads it.
// PREVENTS: init writing file/active/ze.conf while `ze start` reads
// file/active/edge.conf, which leaves init's config unread and bootstraps a
// second one for any instance name but "ze".
func TestZeInitActiveConfigFollowsInstanceName(t *testing.T) {
	dbPath := t.TempDir()
	creds := "admin\nsecret123\n0.0.0.0\n22\nedge\n"
	if code := runInit(strings.NewReader(creds), nil, dbPath, false, "", "", false, false); code != 0 {
		t.Fatalf("runInit exit = %d, want 0", code)
	}
	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	if _, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf")); err == nil {
		t.Fatal("init named \"edge\" wrote file/active/ze.conf; want file/active/edge.conf")
	}
	if _, err := store.ReadFile(zefs.KeyFileActive.Key(resolve.DefaultConfig(store))); err != nil {
		// Discovery always finds the loopback, so init always writes one.
		t.Fatalf("init named \"edge\" wrote no file/active/edge.conf: %v", err)
	}
}

// VALIDATES: a managed init writes no discovered config, so the first
// `ze start` of the managed client fetches its config from the hub.
// PREVENTS: init writing file/active/edge-01.conf from local discovery, which
// cmdStartManaged then reads as the hub's cached config and boots from,
// skipping the hub bootstrap (test/managed/client-first-boot.ci).
func TestZeInitManagedWritesNoDiscoveredConfig(t *testing.T) {
	dbPath := t.TempDir()
	creds := "admin\nsecret123\n127.0.0.1\n2222\nedge-01\n"
	if code := runInit(strings.NewReader(creds), nil, dbPath, true, "", "", false, false); code != 0 {
		t.Fatalf("runInit exit = %d, want 0", code)
	}
	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	configName := resolve.DefaultConfig(store)
	if store.Exists(configName) {
		t.Fatalf("managed init wrote %s; the hub supplies a managed client's first config", configName)
	}
}
