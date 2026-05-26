package debug

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

func setupTestStore(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test.zefs")
	store, err := zefs.Create(storePath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.Close() //nolint:errcheck // test setup
	return storePath, func() { _ = os.RemoveAll(dir) }
}

func TestDebugEnableSubsystem(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.en")

	storePath, cleanup := setupTestStore(t)
	defer cleanup()

	origBlobPath := blobPathOverride
	blobPathOverride = storePath
	defer func() { blobPathOverride = origBlobPath }()

	code := Run([]string{"enable", "test.debug.en"})
	if code != 0 {
		t.Fatalf("enable returned %d, want 0", code)
	}

	store, err := zefs.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	data, err := store.ReadFile("state/debug/test.debug.en")
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if string(data) != "on" {
		t.Errorf("key value = %q, want %q", string(data), "on")
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.en"] != "debug" {
		t.Errorf("level = %q, want debug", levels["test.debug.en"])
	}
}

func TestDebugDisableSubsystem(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.dis")

	storePath, cleanup := setupTestStore(t)
	defer cleanup()

	origBlobPath := blobPathOverride
	blobPathOverride = storePath
	defer func() { blobPathOverride = origBlobPath }()

	Run([]string{"enable", "test.debug.dis"})
	code := Run([]string{"disable", "test.debug.dis"})
	if code != 0 {
		t.Fatalf("disable returned %d, want 0", code)
	}

	store, err := zefs.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	data, err := store.ReadFile("state/debug/test.debug.dis")
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if string(data) != "off" {
		t.Errorf("key value = %q, want %q", string(data), "off")
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.dis"] == "debug" {
		t.Error("level should not be debug after disable")
	}
}

func TestDebugEnableAll(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.all1")
	_ = slogutil.Logger("test.debug.all2")

	storePath, cleanup := setupTestStore(t)
	defer cleanup()

	origBlobPath := blobPathOverride
	blobPathOverride = storePath
	defer func() { blobPathOverride = origBlobPath }()

	code := Run([]string{"enable", nameAll})
	if code != 0 {
		t.Fatalf("enable all returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	for _, name := range []string{"test.debug.all1", "test.debug.all2"} {
		if levels[name] != "debug" {
			t.Errorf("%s level = %q, want debug", name, levels[name])
		}
	}
}

func TestDebugDisableAll(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.dall")

	storePath, cleanup := setupTestStore(t)
	defer cleanup()

	origBlobPath := blobPathOverride
	blobPathOverride = storePath
	defer func() { blobPathOverride = origBlobPath }()

	Run([]string{"enable", nameAll})
	code := Run([]string{"disable", nameAll})
	if code != 0 {
		t.Fatalf("disable all returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.dall"] == "debug" {
		t.Error("level should not be debug after disable all")
	}
}

func TestDebugDisableAllPreservesPerSubsystem(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.kept")
	_ = slogutil.Logger("test.debug.cleared")

	storePath, cleanup := setupTestStore(t)
	defer cleanup()

	origBlobPath := blobPathOverride
	blobPathOverride = storePath
	defer func() { blobPathOverride = origBlobPath }()

	// Enable per-subsystem for "kept", then enable all, then disable all.
	// "kept" should remain at debug because its per-subsystem key is "on".
	Run([]string{"enable", "test.debug.kept"})
	Run([]string{"enable", nameAll})
	code := Run([]string{"disable", nameAll})
	if code != 0 {
		t.Fatalf("disable all returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.kept"] != "debug" {
		t.Errorf("test.debug.kept: expected debug (per-subsystem on), got %q", levels["test.debug.kept"])
	}
	if levels["test.debug.cleared"] == "debug" {
		t.Error("test.debug.cleared: should not be debug after disable all")
	}
}

func TestDebugShow(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.show")

	storePath, cleanup := setupTestStore(t)
	defer cleanup()

	origBlobPath := blobPathOverride
	blobPathOverride = storePath
	defer func() { blobPathOverride = origBlobPath }()

	code := Run([]string{"show"})
	if code != 0 {
		t.Fatalf("show returned %d, want 0", code)
	}
}

func TestDebugInvalidSubsystem(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.valid")

	storePath, cleanup := setupTestStore(t)
	defer cleanup()

	origBlobPath := blobPathOverride
	blobPathOverride = storePath
	defer func() { blobPathOverride = origBlobPath }()

	code := Run([]string{"enable", "nonexistent.subsystem"})
	if code != 1 {
		t.Errorf("enable nonexistent returned %d, want 1", code)
	}
}

func TestDebugNoArgs(t *testing.T) {
	code := Run(nil)
	if code != 1 {
		t.Errorf("no args returned %d, want 1", code)
	}
}

func TestDebugUnknownSubcommand(t *testing.T) {
	code := Run([]string{"bogus"})
	if code != 1 {
		t.Errorf("unknown subcommand returned %d, want 1", code)
	}
}

func TestZefsKeyRegistration(t *testing.T) {
	allKey := zefs.KeyDebugAll.Key()
	if allKey != "state/debug/all" {
		t.Errorf("KeyDebugAll = %q, want %q", allKey, "state/debug/all")
	}

	subKey := zefs.KeyDebugSubsystem.Key("bgp")
	if subKey != "state/debug/bgp" {
		t.Errorf("KeyDebugSubsystem(bgp) = %q, want %q", subKey, "state/debug/bgp")
	}

	if !zefs.IsRegistered("state/debug/all") {
		t.Error("state/debug/all not registered")
	}
	if !zefs.IsRegistered("state/debug/bgp.reactor") {
		t.Error("state/debug/bgp.reactor not registered (template match)")
	}
}
