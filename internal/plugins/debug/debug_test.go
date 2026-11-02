// VALIDATES: set debug module <m> enables the subsystem (level debug), idempotent.
// VALIDATES: delete debug module <m> disables it.
// VALIDATES: set/delete debug module <m> flag <f> add/remove a flag entry.
// VALIDATES: set debug active name <n> loads and applies a saved profile.
// VALIDATES: invalid flag/level rejected with a YANG-derived error.
// VALIDATES: set debug timeout bounds (0..1440m).
// PREVENTS: set/delete not persisting, non-idempotent enable, timeout bounds violation.

package debug

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
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

	code := runSetModule(commandtest.Args("test.debug.enable")).Code
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

	if code := runSetModule(commandtest.Args("test.debug.idem")).Code; code != 0 {
		t.Fatalf("first enable returned %d, want 0", code)
	}
	if code := runSetModule(commandtest.Args("test.debug.idem")).Code; code != 0 {
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

	runSetModule(commandtest.Args("test.debug.disable"))
	code := runDeleteModule(commandtest.Args("test.debug.disable")).Code
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
	code := runDeleteModule(commandtest.Args("never.enabled")).Code
	if code != 0 {
		t.Errorf("delete of absent module returned %d, want 0", code)
	}
}

func TestDebugSetFlag(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.flag")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule(commandtest.Args("test.debug.flag", "flag", "update")).Code
	if code != 0 {
		t.Fatalf("set flag returned %d, want 0", code)
	}
}

func TestDebugDeleteFlag(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.delflag")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule(commandtest.Args("test.debug.delflag", "flag", "update"))
	code := runDeleteModule(commandtest.Args("test.debug.delflag", "flag", "update")).Code
	if code != 0 {
		t.Fatalf("delete flag returned %d, want 0", code)
	}
}

func TestDebugSetDirectionScope(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.dirscope")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule(commandtest.Args("test.debug.dirscope", "scope", "direction", "receive")).Code
	if code != 0 {
		t.Fatalf("set direction scope returned %d, want 0", code)
	}
}

func TestDebugSetNeighborScope(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.scope")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule(commandtest.Args("test.debug.scope", "scope", "neighbor", "192.0.2.1")).Code
	if code != 0 {
		t.Fatalf("set neighbor scope returned %d, want 0", code)
	}
}

func TestDebugShowProfileByName(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.show")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule(commandtest.Args("test.debug.show"))

	code := runShowProfile(commandtest.Args("name", "default")).Code
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

	runSetModule(commandtest.Args("test.debug.filt.a"))

	// `show debug profile name default module <prefix>` filters the subtree
	// (preserves the historical `debug show <module>` view).
	code := runShowProfile(commandtest.Args("name", "default", "module", "test.debug.filt")).Code
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
		if code := runShowProfile(commandtest.Args(args...)).Code; code != 1 {
			t.Errorf("runShowProfile(%v) = %d, want 1", args, code)
		}
	}
}

func TestDebugShowProfileList(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.list")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule(commandtest.Args("test.debug.list"))

	code := runShowProfile(commandtest.Args()).Code
	if code != 0 {
		t.Fatalf("show debug profile (list) returned %d, want 0", code)
	}
}

func TestDebugRestore(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.restore")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule(commandtest.Args("test.debug.restore"))

	slogutil.RestoreLevel("test.debug.restore")
	levels := slogutil.ListLevels()
	if levels["test.debug.restore"] == "debug" {
		t.Fatal("level should not be debug after RestoreLevel")
	}

	code := runRestoreProfile(commandtest.Args("default")).Code
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

	runSetModule(commandtest.Args("test.debug.profsave"))

	code := runSaveProfile(commandtest.Args("test-profile")).Code
	if code != 0 {
		t.Fatalf("set debug profile name returned %d, want 0", code)
	}

	code = runShowProfile(commandtest.Args()).Code
	if code != 0 {
		t.Fatalf("show debug profile list returned %d, want 0", code)
	}
}

func TestDebugProfileDelete(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.profdel")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule(commandtest.Args("test.debug.profdel"))
	runSaveProfile(commandtest.Args("to-delete"))

	code := runDeleteProfileName(commandtest.Args("to-delete")).Code
	if code != 0 {
		t.Fatalf("delete debug profile name returned %d, want 0", code)
	}
}

// TestDebugProfileDeleteMissingRefused proves deleting a profile the store
// does not hold is refused with the name and the command that lists the
// stored ones, rather than answered "deleted" or blamed on the store file.
func TestDebugProfileDeleteMissingRefused(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	answer := runDeleteProfileName(commandtest.Args("never-saved"))
	if answer.Code != 1 {
		t.Fatalf("delete of a missing profile exited %d, want 1", answer.Code)
	}
	if answer.Payload != nil {
		t.Errorf("delete of a missing profile answered %v, want no answer", answer.Payload)
	}
	const want = "error: no debug profile named \"never-saved\"; run 'show debug profile' to list the stored profiles\n"
	if answer.Diagnostic != want {
		t.Errorf("diagnostic %q, want %q", answer.Diagnostic, want)
	}
}

func TestDebugClear(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.clear")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule(commandtest.Args("test.debug.clear"))

	code := runClear(commandtest.Args()).Code
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

	code := runSetModule(commandtest.Args("any.module.name")).Code
	if code != 0 {
		t.Errorf("unregistered subsystem returned %d, want 0 (profile stores intent)", code)
	}
}

func TestDebugInvalidModuleName(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule(commandtest.Args("has/slash")).Code
	if code != 1 {
		t.Errorf("module with slash returned %d, want 1", code)
	}
}

func TestDebugSetModuleNoArgs(t *testing.T) {
	code := runSetModule(commandtest.Args()).Code
	if code != 1 {
		t.Errorf("set debug module with no args returned %d, want 1", code)
	}
}

func TestDebugTimeoutMinutes(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("30m")).Code
	if code != 0 {
		t.Fatalf("timeout 30m returned %d, want 0", code)
	}
}

func TestDebugTimeoutHours(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("1h")).Code
	if code != 0 {
		t.Fatalf("timeout 1h returned %d, want 0", code)
	}
}

func TestDebugTimeoutSeconds(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("90s")).Code
	if code != 0 {
		t.Fatalf("timeout 90s returned %d, want 0", code)
	}
}

func TestDebugTimeoutZero(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("0")).Code
	if code != 0 {
		t.Fatalf("timeout 0 returned %d, want 0", code)
	}
}

func TestDebugTimeoutLastValid(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("1440m")).Code
	if code != 0 {
		t.Fatalf("timeout 1440m returned %d, want 0", code)
	}
}

func TestDebugTimeout24h(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("24h")).Code
	if code != 0 {
		t.Fatalf("timeout 24h returned %d, want 0", code)
	}
}

func TestDebugTimeoutAboveMax(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("1441m")).Code
	if code != 1 {
		t.Errorf("timeout 1441m returned %d, want 1", code)
	}
}

func TestDebugTimeoutAboveMax25h(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("25h")).Code
	if code != 1 {
		t.Errorf("timeout 25h returned %d, want 1", code)
	}
}

func TestDebugTimeoutNoUnit(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("30")).Code
	if code != 1 {
		t.Errorf("timeout without unit returned %d, want 1", code)
	}
}

func TestDebugTimeoutBadFormat(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("abc")).Code
	if code != 1 {
		t.Errorf("timeout abc returned %d, want 1", code)
	}
}

func TestDebugTimeoutOverflow(t *testing.T) {
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetTimeout(commandtest.Args("99999999999999999999m")).Code
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

	code := runSetModule(commandtest.Args("test.debug.flagval", "flag", "nonexistent")).Code
	if code != 1 {
		t.Errorf("invalid flag returned %d, want 1", code)
	}

	code = runSetModule(commandtest.Args("test.debug.flagval", "flag", "update")).Code
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

	code := runSetModule(commandtest.Args("test.debug.noflag", "flag", "anything")).Code
	if code != 0 {
		t.Errorf("flag with no modules should be accepted (no validation), got %d", code)
	}
}

func TestDebugOverlappingModulesSpecificWins(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.overlap.child")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	runSetModule(commandtest.Args("test.debug.overlap", "level", "debug"))
	runSetModule(commandtest.Args("test.debug.overlap.child", "level", "info"))

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

	code := runSetModule(commandtest.Args("test.debug.badlevel", "level", "bogus")).Code
	if code != 1 {
		t.Errorf("invalid level returned %d, want 1", code)
	}
}

func TestDebugSetLevel(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.level")

	cleanup := setupTestDebugStore(t)
	defer cleanup()

	code := runSetModule(commandtest.Args("test.debug.level", "level", "info")).Code
	if code != 0 {
		t.Fatalf("set level returned %d, want 0", code)
	}

	levels := slogutil.ListLevels()
	if levels["test.debug.level"] != "info" {
		t.Errorf("level = %q, want info", levels["test.debug.level"])
	}
}

// captureDebugOutput runs a handler with os.Stdout and os.Stderr redirected,
// and answers its answer and everything it wrote to either stream.
func captureDebugOutput(t *testing.T, fn func() command.LocalAnswer) (command.LocalAnswer, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = writer, writer
	answer := fn()
	os.Stdout, os.Stderr = origOut, origErr
	writer.Close() //nolint:errcheck // test pipe
	out, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	return answer, string(out)
}

// TestDebugModuleSettingRefusesTrailingWords checks that set and delete refuse
// a word after the one option they apply, naming it, and change nothing.
// Method: `level debug flag x` used to apply the level and drop `flag x`. The
// refusal is the returned Diagnostic, and the handler writes nothing itself.
func TestDebugModuleSettingRefusesTrailingWords(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.trailing")
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	before := slogutil.ListLevels()["test.debug.trailing"]
	answer, written := captureDebugOutput(t, func() command.LocalAnswer {
		return runSetModule(commandtest.Args("test.debug.trailing", "level", "error", "flag", "x"))
	})
	if answer.Code != 1 || answer.Payload != nil {
		t.Fatalf("set with trailing words answered code %d payload %v, want a refusal with code 1", answer.Code, answer.Payload)
	}
	if !strings.Contains(answer.Diagnostic, `"flag"`) {
		t.Errorf("diagnostic %q does not name the unexpected word flag", answer.Diagnostic)
	}
	if written != "" {
		t.Errorf("handler wrote %q, want nothing", written)
	}
	if level := slogutil.ListLevels()["test.debug.trailing"]; level != before {
		t.Errorf("level changed from %q to %q although the command was refused", before, level)
	}

	if code := runSetModule(commandtest.Args("test.debug.trailing", "flag", "f")).Code; code != 0 {
		t.Fatalf("set flag returned %d, want 0", code)
	}
	answer, written = captureDebugOutput(t, func() command.LocalAnswer {
		return runDeleteModule(commandtest.Args("test.debug.trailing", "flag", "f", "extra"))
	})
	if answer.Code != 1 {
		t.Fatalf("delete with a trailing word returned %d, want 1", answer.Code)
	}
	if !strings.Contains(answer.Diagnostic, `"extra"`) {
		t.Errorf("diagnostic %q does not name the unexpected word extra", answer.Diagnostic)
	}
	if written != "" {
		t.Errorf("handler wrote %q, want nothing", written)
	}
}

// TestDebugAnswersAreDataAndDrawAsBefore checks every debug command answers a
// value, writes nothing, and that the local route draws that value as exactly
// the text the command printed when it wrote stdout itself.
// Method: run each handler with both streams captured, then render its payload
// through command.RenderLocalAnswer (the plain local route) and compare bytes.
func TestDebugAnswersAreDataAndDrawAsBefore(t *testing.T) {
	slogutil.ResetLevelRegistry()
	_ = slogutil.Logger("test.debug.draw")
	cleanup := setupTestDebugStore(t)
	defer cleanup()

	cases := []struct {
		name string
		run  func() command.LocalAnswer
		path string
		want string
	}{
		{"empty list", func() command.LocalAnswer { return runShowProfile(commandtest.Args()) },
			"show debug profile", "no saved profiles\n"},
		{"set module", func() command.LocalAnswer { return runSetModule(commandtest.Args("test.debug.draw", "flag", "f")) },
			"set debug module", "debug module test.debug.draw enabled\n"},
		{"timeout", func() command.LocalAnswer { return runSetTimeout(commandtest.Args("90s")) },
			"set debug timeout", "debug timeout set to 2m\n"},
		{"timeout off", func() command.LocalAnswer { return runSetTimeout(commandtest.Args("0")) },
			"set debug timeout", "debug timeout disabled\n"},
		{"save", func() command.LocalAnswer { return runSaveProfile(commandtest.Args("p1")) },
			"set debug profile name", "debug profile \"p1\" saved\n"},
		{"list", func() command.LocalAnswer { return runShowProfile(commandtest.Args()) },
			"show debug profile", "default\np1\n"},
		{"table", func() command.LocalAnswer { return runShowProfile(commandtest.Args("name", "default")) },
			"show debug profile",
			"MODULE           LEVEL  FLAGS  SCOPES\n------           -----  -----  ------\ntest.debug.draw  debug  f      \n"},
		{"restore", func() command.LocalAnswer { return runRestoreProfile(commandtest.Args("p1")) },
			"set debug active name", "debug profile \"p1\" applied\n"},
		{"delete flag", func() command.LocalAnswer { return runDeleteModule(commandtest.Args("test.debug.draw", "flag", "f")) },
			"delete debug module", "debug module test.debug.draw updated\n"},
		{"delete module", func() command.LocalAnswer { return runDeleteModule(commandtest.Args("test.debug.draw")) },
			"delete debug module", "debug module test.debug.draw disabled\n"},
		{"delete profile", func() command.LocalAnswer { return runDeleteProfileName(commandtest.Args("p1")) },
			"delete debug profile name", "debug profile \"p1\" deleted\n"},
		{"clear", func() command.LocalAnswer { return runClear(commandtest.Args()) },
			"clear debug", "debug state cleared\n"},
	}
	for _, tc := range cases {
		answer, written := captureDebugOutput(t, tc.run)
		if written != "" {
			t.Errorf("%s: handler wrote %q, want nothing", tc.name, written)
		}
		if answer.Code != 0 || answer.Payload == nil || answer.Diagnostic != "" {
			t.Fatalf("%s: answer %+v, want a payload with code 0", tc.name, answer)
		}
		var code int
		_, drawn := captureDebugOutput(t, func() command.LocalAnswer {
			code = command.RenderLocalAnswer(tc.path, answer.Payload)
			return command.LocalAnswer{}
		})
		if code != 0 || drawn != tc.want {
			t.Errorf("%s: local route printed %q (code %d), want %q", tc.name, drawn, code, tc.want)
		}
	}
}
