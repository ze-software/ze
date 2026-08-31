// Design: docs/architecture/hub-architecture.md -- hub CLI entry point
// Related: startup_gate.go -- the refusal these tests drive
//
// startup_gate_test.go proves three things about the plugin setup gate: a hard
// failure stops the daemon before it does anything irreversible, a soft
// failure does not stop it at all, and no CLI verb can reach the gate.

package hub

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// gatePlugin is the plugin these tests record against. A name no real plugin
// uses keeps the record readable when the assertion fails.
const gatePlugin = "startup-gate-probe"

// isolateRegistry empties the plugin registry and the setup record for one
// test, and puts both back. Every test in this binary shares one registry.
func isolateRegistry(t *testing.T) {
	t.Helper()
	saved := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(saved) })
	registry.Reset()
}

// registerGatePlugin puts one plugin in the registry so the setup record has a
// registered plugin to answer for.
func registerGatePlugin(t *testing.T, name string) {
	t.Helper()
	err := registry.Register(registry.Registration{
		Name:        name,
		Description: "Probe plugin for the startup gate tests",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	})
	if err != nil {
		t.Fatalf("register %q: %v", name, err)
	}
}

// pinConfigDir points ze.config.dir at a fresh directory and returns it. The
// daemon opens {dir}/database.zefs there once it is past the gate, so an empty
// directory is evidence that it never got that far.
func pinConfigDir(t *testing.T) string {
	t.Helper()
	previous := env.Get("ze.config.dir")
	dir := t.TempDir()
	if err := env.Set("ze.config.dir", dir); err != nil {
		t.Fatalf("set ze.config.dir: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previous); err != nil {
			t.Fatalf("restore ze.config.dir: %v", err)
		}
	})
	return dir
}

// stderrDuringRun runs fn with os.Stderr redirected and returns the exit code
// fn produced and what it wrote.
//
// The pipe is read only after fn returns, so fn MUST write less than the pipe
// buffer holds (64 KiB on Linux). Every caller here writes one refusal line.
func stderrDuringRun(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stderr = writer
	defer func() { os.Stderr = original }()

	code := fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	written, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return code, string(written)
}

// TestRunRefusesOnHardSetupFailure proves AC-3.
//
// VALIDATES: a recorded hard setup failure stops run with exit code 1, a
// stderr line naming the plugin and its reason, and no state store created.
//
// PREVENTS: a daemon that starts without a plugin it cannot run without, and
// a refusal that lands after the first irreversible act. The empty config
// directory is what proves the ordering: openStateOnlyStore creates
// database.zefs there the moment run gets past this gate.
func TestRunRefusesOnHardSetupFailure(t *testing.T) {
	isolateRegistry(t)
	registerGatePlugin(t, gatePlugin)
	registry.RecordSetup(gatePlugin, registry.SetupFailedHard, "the kernel does not support it")
	configDir := pinConfigDir(t)

	code, written := stderrDuringRun(t, func() int {
		return run(storage.NewFilesystem(), filepath.Join(configDir, "hub.conf"), nil,
			0, -1, false, "", false, "", "", false, nil)
	})

	if code != 1 {
		t.Errorf("run exit code = %d, want 1", code)
	}
	for _, want := range []string{"plugin setup", gatePlugin, "the kernel does not support it"} {
		if !strings.Contains(written, want) {
			t.Errorf("the refusal does not carry %q: %s", want, written)
		}
	}
	if strings.Contains(written, "read config") {
		t.Errorf("run reached the config read before refusing: %s", written)
	}

	if _, err := os.Stat(filepath.Join(configDir, stateStoreName)); err == nil {
		t.Errorf("run created %s, so the refusal came after the first irreversible act", stateStoreName)
	}
}

// TestTheRefusalReachesTheLogAndNotOnlyStderr proves the clause of AC-3 the
// test above cannot: `logStartupFailure` records the same failure, so the
// refusal survives in the log ring `show log` reads rather than only on a
// stderr an appliance console may have scrolled past.
//
// The two producers are told apart deliberately. `fmt.Fprintln` writes
// `error: plugin setup: ...`; the slog text handler writes `stage="plugin
// setup"`, which nothing else in this path produces. Asserting the stderr
// string "plugin setup" alone would pass with logStartupFailure deleted,
// because the Fprintln carries those same two words.
//
// VALIDATES: the hub log ring carries the startup failure, and the slog
// rendering names the stage.
//
// PREVENTS: a daemon that refuses to start and leaves nothing in the log an
// operator can read afterwards. Seven of the startup failures surveyed for
// this spec were invisible in `show log` for exactly this reason: they were
// written with fmt.Fprintf to stderr and never reached the ring.
func TestTheRefusalReachesTheLogAndNotOnlyStderr(t *testing.T) {
	isolateRegistry(t)
	registerGatePlugin(t, gatePlugin)
	registry.RecordSetup(gatePlugin, registry.SetupFailedHard, "the kernel does not support it")
	configDir := pinConfigDir(t)

	before := len(slogutil.GlobalLogRing().Snapshot(0, "", "hub"))

	_, written := stderrDuringRun(t, func() int {
		return run(storage.NewFilesystem(), filepath.Join(configDir, "hub.conf"), nil,
			0, -1, false, "", false, "", "", false, nil)
	})

	if !strings.Contains(written, `stage="plugin setup"`) {
		t.Errorf(`the slog record does not name the stage, so logStartupFailure did not run: %s`, written)
	}

	entries := slogutil.GlobalLogRing().Snapshot(0, "", "hub")
	if len(entries) <= before {
		t.Fatalf("the hub log ring gained no entry: %d before, %d after", before, len(entries))
	}
	logged := false
	for _, entry := range entries[before:] {
		if entry.Message == "startup failed" && entry.Level == "ERROR" {
			logged = true
			break
		}
	}
	if !logged {
		t.Errorf("no ERROR 'startup failed' entry reached the ring: %+v", entries[before:])
	}
}

// TestRunRefusalNamesEveryHardFailure proves AC-8 at the entry point.
//
// VALIDATES: two plugins that recorded a hard failure are both named in the
// one refusal.
//
// PREVENTS: an operator repairing the first fault, restarting, and meeting the
// second. Each fault after the first would cost a whole boot.
func TestRunRefusalNamesEveryHardFailure(t *testing.T) {
	isolateRegistry(t)
	registerGatePlugin(t, "beta-plugin")
	registry.RecordSetup("beta-plugin", registry.SetupFailedHard, "beta reason")
	registerGatePlugin(t, "alpha-plugin")
	registry.RecordSetup("alpha-plugin", registry.SetupFailedHard, "alpha reason")
	configDir := pinConfigDir(t)

	code, written := stderrDuringRun(t, func() int {
		return run(storage.NewFilesystem(), filepath.Join(configDir, "hub.conf"), nil,
			0, -1, false, "", false, "", "", false, nil)
	})

	if code != 1 {
		t.Errorf("run exit code = %d, want 1", code)
	}
	for _, want := range []string{"alpha-plugin", "alpha reason", "beta-plugin", "beta reason"} {
		if !strings.Contains(written, want) {
			t.Errorf("the refusal does not carry %q: %s", want, written)
		}
	}
	if strings.Index(written, "alpha-plugin") > strings.Index(written, "beta-plugin") {
		t.Errorf("the refusal is not in plugin name order: %s", written)
	}
}

// TestRunProceedsOnSoftFailure proves AC-2.
//
// VALIDATES: a recorded soft failure does not stop run. The daemon goes past
// the gate and fails on the config it was actually given.
//
// PREVENTS: a daemon refusing to start over a feature it runs correctly
// without, which is worse than the silence this registry removes.
func TestRunProceedsOnSoftFailure(t *testing.T) {
	isolateRegistry(t)
	registerGatePlugin(t, gatePlugin)
	registry.RecordSetup(gatePlugin, registry.SetupFailedSoft, "the feature is absent")

	previous := env.Get("ze.config.dir")
	if err := env.Set("ze.config.dir", ""); err != nil {
		t.Fatalf("clear ze.config.dir: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previous); err != nil {
			t.Fatalf("restore ze.config.dir: %v", err)
		}
	})

	missing := filepath.Join(t.TempDir(), "absent.conf")
	code, written := stderrDuringRun(t, func() int {
		return run(storage.NewFilesystem(), missing, nil, 0, -1, false, "", false, "", "", false, nil)
	})

	if code != 1 {
		t.Errorf("run over a missing config exited %d, want 1", code)
	}
	if strings.Contains(written, "plugin setup") {
		t.Errorf("a soft failure refused the start: %s", written)
	}
	if !strings.Contains(written, "read config") {
		t.Errorf("run did not reach the config read, so the gate is not the only thing that stopped it: %s", written)
	}
}

// TestCLIVerbUnaffectedByHardSetupFailure proves AC-5.
//
// VALIDATES: a CLI verb answers normally while a plugin holds a recorded hard
// failure, and the answer carries that failure.
//
// PREVENTS: the fix becoming worse than the fault. A registry that refuses
// every ze invocation takes away the one command an operator would use to find
// out what is wrong.
func TestCLIVerbUnaffectedByHardSetupFailure(t *testing.T) {
	isolateRegistry(t)
	registerGatePlugin(t, gatePlugin)
	registry.RecordSetup(gatePlugin, registry.SetupFailedHard, "the kernel does not support it")

	answer, code, served := command.ServeLocal("show plugins | json", "")
	if !served {
		t.Fatal("show plugins was not served in this process")
	}
	if code != 0 {
		t.Fatalf("a CLI verb exited %d while a hard failure was recorded: %s", code, answer)
	}
	for _, want := range []string{gatePlugin, "hard-failure", "the kernel does not support it"} {
		if !strings.Contains(answer, want) {
			t.Errorf("the CLI answer does not carry %q: %s", want, answer)
		}
	}
}

// TestTheSetupGateHasOneCallerAndItIsRun is the mechanical half of AC-5.
//
// VALIDATES: hardSetupFailure is called from run and from nowhere else in the
// hub package.
//
// PREVENTS: a second call site added later on a path a CLI verb reaches. The
// behavioral test above proves today's CLI verb is unaffected; only this one
// keeps it true for a caller nobody has written yet.
func TestTheSetupGateHasOneCallerAndItIsRun(t *testing.T) {
	callers := gateCallers(t)
	if len(callers) != 1 {
		t.Fatalf("hardSetupFailure is called from %v, want run alone", callers)
	}
	if callers[0] != "run" {
		t.Errorf("hardSetupFailure is called from %q, want run", callers[0])
	}
}

// gateCallers returns the name of every non-test function in this package that
// calls hardSetupFailure.
func gateCallers(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the hub package directory: %v", err)
	}

	var callers []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		for _, declaration := range file.Decls {
			function, isFunction := declaration.(*ast.FuncDecl)
			if !isFunction || function.Body == nil {
				continue
			}
			if function.Name.Name == "hardSetupFailure" {
				continue
			}
			if callsTheGate(function.Body) {
				callers = append(callers, function.Name.Name)
			}
		}
	}
	return callers
}

// callsTheGate reports whether a function body calls hardSetupFailure.
func callsTheGate(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		name, isIdent := call.Fun.(*ast.Ident)
		if isIdent && name.Name == "hardSetupFailure" {
			found = true
		}
		return !found
	})
	return found
}
