// VALIDATES: set debug module <m> enables the subsystem (level debug), idempotent.
// VALIDATES: delete debug module <m> disables it.
// VALIDATES: set/delete debug module <m> flag <f> add/remove a flag entry.
// VALIDATES: set debug active name <n> loads and applies a saved profile.
// VALIDATES: invalid flag/level rejected with a YANG-derived error.
// VALIDATES: set debug timeout bounds (0..1440m).
// PREVENTS: set/delete not persisting, non-idempotent enable, timeout bounds violation.

package debug

import (
	"path/filepath"
	"testing"

	debugyang "github.com/ze-software/ze/internal/component/debug/yang"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/zefs"
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

func TestDebugEnableModule(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.enable")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule([]string{"test.debug.enable"})
	if code != 0 {
		t.Fatalf("set debug module returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.enable"] != "debug" {
		t.Errorf("level = %q, want debug", levels["test.debug.enable"])
	}
}

func TestDebugEnableIdempotent(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.idem")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	if code := runSetModule([]string{"test.debug.idem"}); code != 0 {
		t.Fatalf("first enable returned %d, want 0", code)
	}
	if code := runSetModule([]string{"test.debug.idem"}); code != 0 {
		t.Fatalf("second enable returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.idem"] != "debug" {
		t.Errorf("level = %q, want debug (enable must be idempotent)", levels["test.debug.idem"])
	}
}

func TestDebugDisableModule(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.disable")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.disable"})
	code := runDeleteModule([]string{"test.debug.disable"})
	if code != 0 {
		t.Fatalf("delete debug module returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.disable"] == "debug" {
		t.Error("level should not be debug after delete")
	}
}

func TestDebugDisableIdempotent(t *testing.T) {
	slogutil.ResetLevelRegistry()
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	// Deleting an already-absent module is a no-op success.
	code := runDeleteModule([]string{"never.enabled"})
	if code != 0 {
		t.Errorf("delete of absent module returned %d, want 0", code)
	}
}

func TestDebugSetFlag(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.flag")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule([]string{"test.debug.flag", "flag", "update"})
	if code != 0 {
		t.Fatalf("set flag returned %d, want 0", code)
	}
}

func TestDebugDeleteFlag(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.delflag")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.delflag", "flag", "update"})
	code := runDeleteModule([]string{"test.debug.delflag", "flag", "update"})
	if code != 0 {
		t.Fatalf("delete flag returned %d, want 0", code)
	}
}

func TestDebugSetDirectionScope(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.dirscope")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule([]string{"test.debug.dirscope", "scope", "direction", "receive"})
	if code != 0 {
		t.Fatalf("set direction scope returned %d, want 0", code)
	}
}

func TestDebugSetNeighborScope(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.scope")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule([]string{"test.debug.scope", "scope", "neighbor", "192.0.2.1"})
	if code != 0 {
		t.Fatalf("set neighbor scope returned %d, want 0", code)
	}
}

func TestDebugShowProfileByName(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.show")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.show"})

	code := runShowProfile([]string{"name", "default"})
	if code != 0 {
		t.Fatalf("show debug profile name default returned %d, want 0", code)
	}
}

func TestDebugShowProfileModuleFilter(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.filt.a")
	_ = slogutil.Logger("test.debug.filt.b")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.filt.a"})

	// `show debug profile name default module <prefix>` filters the subtree
	// (preserves the historical `debug show <module>` view).
	code := runShowProfile([]string{"name", "default", "module", "test.debug.filt"})
	if code != 0 {
		t.Fatalf("show debug profile name default module <prefix> returned %d, want 0", code)
	}
}

func TestDebugShowProfileMalformedArgsRejected(t *testing.T) {
	slogutil.ResetLevelRegistry()
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	// Trailing tokens that are not exactly `module <prefix>` are rejected,
	// not silently ignored.
	cases := [][]string{
		{"name", "default", "module"},                 // module keyword, no value
		{"name", "default", "junk"},                   // unknown trailing token
		{"name", "default", "xxx", "bgp"},             // wrong keyword
		{"name", "default", "module", "bgp", "extra"}, // extra token after filter
	}
	for _, args := range cases {
		if code := runShowProfile(args); code != 1 {
			t.Errorf("runShowProfile(%v) = %d, want 1", args, code)
		}
	}
}

func TestDebugShowProfileList(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.list")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.list"})

	code := runShowProfile(nil)
	if code != 0 {
		t.Fatalf("show debug profile (list) returned %d, want 0", code)
	}
}

func TestDebugRestore(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.restore")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.restore"})

	slogutil.RestoreLevel("test.debug.restore")
	levels := slogutil.ListLevels()
	if levels["test.debug.restore"] == "debug" {
		t.Fatal("level should not be debug after RestoreLevel")
	}

	code := runRestoreProfile([]string{"default"})
	if code != 0 {
		t.Fatalf("set debug active name default returned %d, want 0", code)
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

	runSetModule([]string{"test.debug.profsave"})

	code := runSaveProfile([]string{"test-profile"})
	if code != 0 {
		t.Fatalf("set debug profile name returned %d, want 0", code)
	}

	code = runShowProfile(nil)
	if code != 0 {
		t.Fatalf("show debug profile list returned %d, want 0", code)
	}
}

func TestDebugProfileDelete(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.profdel")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.profdel"})
	runSaveProfile([]string{"to-delete"})

	code := runDeleteProfileName([]string{"to-delete"})
	if code != 0 {
		t.Fatalf("delete debug profile name returned %d, want 0", code)
	}
}

func TestDebugClear(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.clear")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.clear"})

	code := cmdClear()
	if code != 0 {
		t.Fatalf("clear debug returned %d, want 0", code)
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

	code := runSetModule([]string{"any.module.name"})
	if code != 0 {
		t.Errorf("unregistered subsystem returned %d, want 0 (profile stores intent)", code)
	}
}

func TestDebugInvalidModuleName(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule([]string{"has/slash"})
	if code != 1 {
		t.Errorf("module with slash returned %d, want 1", code)
	}
}

func TestDebugSetModuleNoArgs(t *testing.T) {
	code := runSetModule(nil)
	if code != 1 {
		t.Errorf("set debug module with no args returned %d, want 1", code)
	}
}

func TestDebugTimeoutMinutes(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"30m"})
	if code != 0 {
		t.Fatalf("timeout 30m returned %d, want 0", code)
	}
}

func TestDebugTimeoutHours(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"1h"})
	if code != 0 {
		t.Fatalf("timeout 1h returned %d, want 0", code)
	}
}

func TestDebugTimeoutSeconds(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"90s"})
	if code != 0 {
		t.Fatalf("timeout 90s returned %d, want 0", code)
	}
}

func TestDebugTimeoutZero(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"0"})
	if code != 0 {
		t.Fatalf("timeout 0 returned %d, want 0", code)
	}
}

func TestDebugTimeoutLastValid(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"1440m"})
	if code != 0 {
		t.Fatalf("timeout 1440m returned %d, want 0", code)
	}
}

func TestDebugTimeout24h(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"24h"})
	if code != 0 {
		t.Fatalf("timeout 24h returned %d, want 0", code)
	}
}

func TestDebugTimeoutAboveMax(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"1441m"})
	if code != 1 {
		t.Errorf("timeout 1441m returned %d, want 1", code)
	}
}

func TestDebugTimeoutAboveMax25h(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"25h"})
	if code != 1 {
		t.Errorf("timeout 25h returned %d, want 1", code)
	}
}

func TestDebugTimeoutNoUnit(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"30"})
	if code != 1 {
		t.Errorf("timeout without unit returned %d, want 1", code)
	}
}

func TestDebugTimeoutBadFormat(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"abc"})
	if code != 1 {
		t.Errorf("timeout abc returned %d, want 1", code)
	}
}

func TestDebugTimeoutOverflow(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout([]string{"99999999999999999999m"})
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

	code := runSetModule([]string{"test.debug.flagval", "flag", "nonexistent"})
	if code != 1 {
		t.Errorf("invalid flag returned %d, want 1", code)
	}

	code = runSetModule([]string{"test.debug.flagval", "flag", "update"})
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

	code := runSetModule([]string{"test.debug.noflag", "flag", "anything"})
	if code != 0 {
		t.Errorf("flag with no modules should be accepted (no validation), got %d", code)
	}
}

func TestDebugOverlappingModulesSpecificWins(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.overlap.child")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule([]string{"test.debug.overlap", "level", "debug"})
	runSetModule([]string{"test.debug.overlap.child", "level", "info"})

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

	code := runSetModule([]string{"test.debug.badlevel", "level", "bogus"})
	if code != 1 {
		t.Errorf("invalid level returned %d, want 1", code)
	}
}

func TestDebugSetLevel(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.level")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule([]string{"test.debug.level", "level", "info"})
	if code != 0 {
		t.Fatalf("set level returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.level"] != "info" {
		t.Errorf("level = %q, want info", levels["test.debug.level"])
	}
}
