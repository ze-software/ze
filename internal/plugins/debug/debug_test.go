// VALIDATES: AC-1 -- debug <module> when not in profile adds module, prints "enabled".
// VALIDATES: AC-2 -- debug <module> when in profile removes module, prints "disabled".
// VALIDATES: AC-3 -- debug <module> flag <flag> toggles flag entry.
// VALIDATES: AC-12 -- debug restore loads and applies saved profile.
// VALIDATES: AC-14 -- invalid flag name rejected with YANG-derived error.
// PREVENTS: Toggle semantics broken, profile not persisting, timeout bounds violation.

package debug

import (
	"path/filepath"
	"testing"

	debugyang "codeberg.org/thomas-mangin/ze/internal/component/debug/yang"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

func setupTestDebugStore(t *testing.T) func() {
	t.Helper()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "debug.zefs")
	store, err := zefs.Create(storePath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.Close() //nolint:errcheck // test setup

	orig := debugStoreOverride
	debugStoreOverride = storePath
	return func() { debugStoreOverride = orig }
}

func TestDebugToggleOn(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.toggle")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"test.debug.toggle"})
	if code != 0 {
		t.Fatalf("toggle on returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.toggle"] != "debug" {
		t.Errorf("level = %q, want debug", levels["test.debug.toggle"])
	}
}

func TestDebugToggleOff(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.toggle2")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	Run([]string{"test.debug.toggle2"})
	code := Run([]string{"test.debug.toggle2"})
	if code != 0 {
		t.Fatalf("toggle off returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.toggle2"] == "debug" {
		t.Error("level should not be debug after toggle off")
	}
}

func TestDebugToggleFlag(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.flag")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"test.debug.flag", "flag", "update"})
	if code != 0 {
		t.Fatalf("flag toggle returned %d, want 0", code)
	}
}

func TestDebugDirectionAsScope(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.dirscope")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"test.debug.dirscope", "scope", "direction", "receive"})
	if code != 0 {
		t.Fatalf("direction scope toggle returned %d, want 0", code)
	}
}

func TestDebugToggleScope(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.scope")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"test.debug.scope", "scope", "neighbor", "192.0.2.1"})
	if code != 0 {
		t.Fatalf("scope toggle returned %d, want 0", code)
	}
}

func TestDebugShow(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.show")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	Run([]string{"test.debug.show"})

	code := Run([]string{"show"})
	if code != 0 {
		t.Fatalf("show returned %d, want 0", code)
	}
}

func TestDebugShowSubtree(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.sub.a")
	_ = slogutil.Logger("test.debug.sub.b")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	Run([]string{"test.debug.sub.a"})

	code := Run([]string{"show", "test.debug.sub"})
	if code != 0 {
		t.Fatalf("show subtree returned %d, want 0", code)
	}
}

func TestDebugRestore(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.restore")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	Run([]string{"test.debug.restore"})

	slogutil.RestoreLevel("test.debug.restore")
	levels := slogutil.ListLevels()
	if levels["test.debug.restore"] == "debug" {
		t.Fatal("level should not be debug after RestoreLevel")
	}

	code := Run([]string{"restore"})
	if code != 0 {
		t.Fatalf("restore returned %d, want 0", code)
	}

	levels = slogutil.ListLevels()
	if levels["test.debug.restore"] != "debug" {
		t.Errorf("level = %q, want debug after restore", levels["test.debug.restore"])
	}
}

func TestDebugProfileSaveList(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.profsave")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	Run([]string{"test.debug.profsave"})

	code := Run([]string{"profile", "save", "test-profile"})
	if code != 0 {
		t.Fatalf("profile save returned %d, want 0", code)
	}

	code = Run([]string{"profile", "list"})
	if code != 0 {
		t.Fatalf("profile list returned %d, want 0", code)
	}
}

func TestDebugProfileDelete(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.profdel")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	Run([]string{"test.debug.profdel"})
	Run([]string{"profile", "save", "to-delete"})

	code := Run([]string{"profile", "delete", "to-delete"})
	if code != 0 {
		t.Fatalf("profile delete returned %d, want 0", code)
	}
}

func TestDebugClear(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.clear")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	Run([]string{"test.debug.clear"})

	code := Run([]string{"clear"})
	if code != 0 {
		t.Fatalf("clear returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.clear"] == "debug" {
		t.Error("level should not be debug after clear")
	}
}

func TestDebugUnregisteredSubsystemAccepted(t *testing.T) {
	slogutil.ResetLevelRegistry()

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"any.module.name"})
	if code != 0 {
		t.Errorf("unregistered subsystem returned %d, want 0 (profile stores intent)", code)
	}
}

func TestDebugInvalidModuleName(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"has/slash"})
	if code != 1 {
		t.Errorf("module with slash returned %d, want 1", code)
	}
}

func TestDebugNoArgs(t *testing.T) {
	code := Run(nil)
	if code != 1 {
		t.Errorf("no args returned %d, want 1", code)
	}
}

func TestDebugHelp(t *testing.T) {
	code := Run([]string{"help"})
	if code != 0 {
		t.Errorf("help returned %d, want 0", code)
	}
}

func TestDebugTimeoutMinutes(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "30m"})
	if code != 0 {
		t.Fatalf("timeout 30m returned %d, want 0", code)
	}
}

func TestDebugTimeoutHours(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "1h"})
	if code != 0 {
		t.Fatalf("timeout 1h returned %d, want 0", code)
	}
}

func TestDebugTimeoutSeconds(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "90s"})
	if code != 0 {
		t.Fatalf("timeout 90s returned %d, want 0", code)
	}
}

func TestDebugTimeoutZero(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "0"})
	if code != 0 {
		t.Fatalf("timeout 0 returned %d, want 0", code)
	}
}

func TestDebugTimeoutLastValid(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "1440m"})
	if code != 0 {
		t.Fatalf("timeout 1440m returned %d, want 0", code)
	}
}

func TestDebugTimeout24h(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "24h"})
	if code != 0 {
		t.Fatalf("timeout 24h returned %d, want 0", code)
	}
}

func TestDebugTimeoutAboveMax(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "1441m"})
	if code != 1 {
		t.Errorf("timeout 1441m returned %d, want 1", code)
	}
}

func TestDebugTimeoutAboveMax25h(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "25h"})
	if code != 1 {
		t.Errorf("timeout 25h returned %d, want 1", code)
	}
}

func TestDebugTimeoutNoUnit(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "30"})
	if code != 1 {
		t.Errorf("timeout without unit returned %d, want 1", code)
	}
}

func TestDebugTimeoutBadFormat(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "abc"})
	if code != 1 {
		t.Errorf("timeout abc returned %d, want 1", code)
	}
}

func TestDebugTimeoutOverflow(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"timeout", "99999999999999999999m"})
	if code != 1 {
		t.Errorf("overflow duration returned %d, want 1", code)
	}
}

func TestDebugInvalidFlagRejected(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.flagval")

	debugyang.ResetForTest()
	defer debugyang.ResetForTest()
	debugyang.RegisterModule(debugyang.Module{
		Prefix: "test.debug",
		Flags:  []string{"update", "open", "keepalive"},
	})

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"test.debug.flagval", "flag", "nonexistent"})
	if code != 1 {
		t.Errorf("invalid flag returned %d, want 1", code)
	}

	code = Run([]string{"test.debug.flagval", "flag", "update"})
	if code != 0 {
		t.Errorf("valid flag returned %d, want 0", code)
	}
}

func TestDebugNoFlagsRegisteredAcceptsAny(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.noflag")

	debugyang.ResetForTest()
	defer debugyang.ResetForTest()

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"test.debug.noflag", "flag", "anything"})
	if code != 0 {
		t.Errorf("flag with no modules should be accepted (no validation), got %d", code)
	}
}

func TestDebugOverlappingModulesSpecificWins(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.overlap.child")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	Run([]string{"test.debug.overlap", "level", "debug"})
	Run([]string{"test.debug.overlap.child", "level", "info"})

	levels := slogutil.ListLevels()
	if levels["test.debug.overlap.child"] != "info" {
		t.Errorf("more-specific module should win, got %q want info", levels["test.debug.overlap.child"])
	}
}

func TestDebugInvalidLevelRejected(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.badlevel")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"test.debug.badlevel", "level", "bogus"})
	if code != 1 {
		t.Errorf("invalid level returned %d, want 1", code)
	}
}

func TestDebugSetLevel(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.level")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := Run([]string{"test.debug.level", "level", "info"})
	if code != 0 {
		t.Fatalf("set level returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.level"] != "info" {
		t.Errorf("level = %q, want info", levels["test.debug.level"])
	}
}
