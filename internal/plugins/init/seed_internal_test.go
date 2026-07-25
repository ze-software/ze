package init

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: `ze init --seed` does not bake this host's interface discovery
// into file/active/ze.conf, so an appliance seed DB does not shadow its
// file/template/ze.conf. AC-2 (init-level mechanism) of
// spec-fixit-appliance-evidence-config; the end-to-end proof that the template
// becomes the effective boot config is the L2TP evidence test (AC-3).
// PREVENTS: a build-time GOKRAZY_TEMPLATE never taking effect because init's
// active config (build-host NICs) shadows it, so web/l2tp never start.

// hasActiveConfig reports whether file/active/ze.conf exists in the database.
func hasActiveConfig(t *testing.T, dbPath string) bool {
	t.Helper()
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	_, readErr := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	return readErr == nil
}

// runInitSeed runs a non-interactive init into a fresh temp database with the
// given seed flag and returns the database path.
func runInitSeed(t *testing.T, seed bool) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "database.zefs")
	creds := "admin\nsecret123\n0.0.0.0\n22\nze\n"
	if code := runInit(strings.NewReader(creds), nil, dbPath, false, "", "", seed); code != 0 {
		t.Fatalf("runInit(seed=%v) exit = %d, want 0", seed, code)
	}
	return dbPath
}

func TestZeInitSeedSkipsActiveConfig(t *testing.T) {
	seedDB := runInitSeed(t, true)
	if hasActiveConfig(t, seedDB) {
		t.Fatal("`ze init --seed` wrote file/active/ze.conf; want absent (it would shadow the appliance template)")
	}

	// --seed only skips interface discovery; credentials must still be written.
	store, err := zefs.Open(seedDB)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	got, err := store.ReadFile(zefs.KeySSHDefault.Pattern)
	if err != nil {
		t.Fatalf("read ssh default: %v", err)
	}
	if string(got) != "0.0.0.0/22" {
		t.Fatalf("meta/ssh/default = %q, want %q", got, "0.0.0.0/22")
	}
}

// TestZeInitWithoutSeedWritesActiveConfig proves the flag actually gates the
// write: on a host where interface discovery succeeds, a non-seed init writes
// the active config that --seed suppresses. Skips where the environment exposes
// no discoverable interface (no contrast to draw).
func TestZeInitWithoutSeedWritesActiveConfig(t *testing.T) {
	if !hasActiveConfig(t, runInitSeed(t, false)) {
		t.Skip("no interfaces discovered in this environment; cannot contrast --seed")
	}
	if hasActiveConfig(t, runInitSeed(t, true)) {
		t.Fatal("`ze init --seed` wrote file/active/ze.conf though a non-seed init did; the flag did not gate discovery")
	}
}
