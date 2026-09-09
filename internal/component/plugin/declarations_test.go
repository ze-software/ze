// Design: docs/architecture/api/commands.md -- where a command is served
// Overview: declarations.go -- the command these tests exercise
//
// Goal: prove that `show plugin declarations` never answers with a plugin
// missing and never answers zero for a question it could not ask. Method: call
// the row builder and the keyword parser directly, over the registry this test
// binary already carries and over a reader the test installs.
//
// A test that needs a plugin to answer configures a `run` string that produces
// the answer it is about, and the framed line those run strings write is built
// by rpc.WriteDeclaration, the writer the ze binary and the SDK both use. So
// these tests read the line a real plugin writes rather than a spelling of it
// kept here.

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// withConfiguredPluginReader installs a reader for one test and puts the
// previous one back afterwards. It writes the field rather than calling
// SetConfiguredPluginReader, because that setter refuses nil on purpose and a
// test of the guard needs the absent reader back.
func withConfiguredPluginReader(t *testing.T, read ConfiguredPluginReader) {
	t.Helper()

	configuredPluginsMu.Lock()
	previous := configuredPlugins
	configuredPlugins = read
	configuredPluginsMu.Unlock()

	t.Cleanup(func() {
		configuredPluginsMu.Lock()
		configuredPlugins = previous
		configuredPluginsMu.Unlock()
	})
}

func TestDeclarationConfigPathReadsTheKeyword(t *testing.T) {
	present := filepath.Join(t.TempDir(), "ze.conf")
	if err := os.WriteFile(present, []byte("plugin {}\n"), 0o600); err != nil {
		t.Fatalf("cannot write the config file the test reads: %v", err)
	}
	absent := filepath.Join(t.TempDir(), "absent.conf")

	cases := []struct {
		name    string
		args    []string
		path    string
		refused string
	}{
		{name: "a path that exists", args: []string{present}, path: present},
		{name: "stdin needs no file", args: []string{"-"}, path: "-"},
		{name: "no path is refused", args: nil, refused: "path of a config file"},
		{name: "a second path is refused", args: []string{present, "extra"}, refused: "extra"},
		{name: "an absent file is refused by name", args: []string{absent}, refused: absent},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path, err := declarationConfigPath(testCase.args)
			if testCase.refused == "" {
				if err != nil {
					t.Fatalf("declarationConfigPath(%q) refused: %v", testCase.args, err)
				}
				if path != testCase.path {
					t.Fatalf("declarationConfigPath(%q) = %q, want %q", testCase.args, path, testCase.path)
				}
				return
			}
			if err == nil {
				t.Fatalf("declarationConfigPath(%q) = %q, want a refusal", testCase.args, path)
			}
			if !strings.Contains(err.Error(), testCase.refused) {
				t.Fatalf("declarationConfigPath(%q) refused with %q, which does not name %q",
					testCase.args, err, testCase.refused)
			}
		})
	}
}

// TestDeclarationRowsCoverEveryRegisteredPlugin holds the never-omit rule: the
// answer carries one row for each plugin this binary holds a record of, and
// each row says what state it is in rather than leaving the reader to read an
// empty declaration as an absent plugin.
//
// The population is registry.SetupResults, the set `show plugin list` answers
// for (pluginRows, register.go). Two commands about one binary's plugins that
// answered for two sets would tell an operator the binary disagrees with
// itself about which plugins it has.
func TestDeclarationRowsCoverEveryRegisteredPlugin(t *testing.T) {
	rows, err := declarationRows("")
	if err != nil {
		t.Fatalf("declarationRows over the compiled-in registry failed: %v", err)
	}

	recorded := registry.SetupResults()
	if len(rows) != len(recorded) {
		t.Fatalf("declarationRows answered %d rows, and `show plugin list` answers for %d plugins",
			len(rows), len(recorded))
	}

	byName := rowsByName(rows)
	for _, result := range recorded {
		row, held := byName[result.Plugin]
		if !held {
			t.Fatalf("plugin %q is in the binary's own record and has no row", result.Plugin)
		}
		if row.State == "" {
			t.Fatalf("plugin %q has a row with no state", result.Plugin)
		}
		if row.Kind != kindInternal {
			t.Fatalf("plugin %q is compiled in and its row reads kind %q", result.Plugin, row.Kind)
		}
	}
}

// TestDeclarationRowsCoverAPluginThatNeverRegistered holds that same rule over
// the one plugin the two populations differ by: a plugin whose own init()
// recorded a setup outcome and whose Register call never completed. It keeps a
// row in `show plugin list`, and it keeps one here.
//
// An answer built from registry.All alone drops it, and its absence reads as
// "not built into this binary" rather than "built in, and its setup failed".
func TestDeclarationRowsCoverAPluginThatNeverRegistered(t *testing.T) {
	const name = "query-recorded-never-registered"
	const recordedReason = "the netlink socket was refused"

	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	registry.RecordSetup(name, registry.SetupFailedSoft, recordedReason)

	rows, err := declarationRows("")
	if err != nil {
		t.Fatalf("declarationRows over a binary holding a failed setup failed: %v", err)
	}

	row, held := rowsByName(rows)[name]
	if !held {
		t.Fatalf("plugin %q recorded a setup outcome, never registered, and has no row", name)
	}
	if row.State != stateUnstartable {
		t.Fatalf("plugin %q reads state %q, and it declared nothing this binary can read, so it is %q",
			name, row.State, stateUnstartable)
	}
	if !strings.Contains(row.Reason, recordedReason) {
		t.Errorf("the row must carry what the plugin recorded, and its reason says %q", row.Reason)
	}
}

// TestDeclarationRowsRefuseWithoutAConfigReader holds the guard: a binary that
// links no config parser says it cannot read the file. Answering the
// compiled-in rows alone would report a config that names no plugin, which is
// the silently wrong answer (`ai/rules/principles.md`).
func TestDeclarationRowsRefuseWithoutAConfigReader(t *testing.T) {
	withConfiguredPluginReader(t, nil)

	rows, err := declarationRows("ze.conf")
	if !errors.Is(err, errNoConfigReader) {
		t.Fatalf("declarationRows with no reader answered %d rows and error %v, want errNoConfigReader",
			len(rows), err)
	}
}

// TestSetConfiguredPluginReaderRefusesNil holds the other half of the guard: a
// caller cannot clear the seam, so a reader registered at init stays
// registered.
func TestSetConfiguredPluginReaderRefusesNil(t *testing.T) {
	installed := func(string) ([]PluginConfig, error) { return nil, nil }
	withConfiguredPluginReader(t, installed)

	SetConfiguredPluginReader(nil)

	configuredPluginsMu.RLock()
	held := configuredPlugins
	configuredPluginsMu.RUnlock()
	if held == nil {
		t.Fatal("SetConfiguredPluginReader(nil) cleared the reader that was registered")
	}
}

// TestDeclarationRowsCarryTheConfiguredPlugins proves the second population
// reaches the answer: a plugin only the config names gains a row of its own,
// and a plugin the config names AND the binary carries gains one row rather
// than two.
func TestDeclarationRowsCarryTheConfiguredPlugins(t *testing.T) {
	registered := registry.All()
	if len(registered) == 0 {
		t.Skip("this test binary registers no plugin, so there is nothing to merge against")
	}
	shared := registered[0].Name

	withConfiguredPluginReader(t, func(string) ([]PluginConfig, error) {
		return []PluginConfig{
			{Name: "reader-only", Run: "ze plugin mrt"},
			{Name: shared, Run: shared, Internal: true},
		}, nil
	})

	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows over a config failed: %v", err)
	}

	seen := 0
	var external declarationRow
	for _, row := range rows {
		if row.Name == shared {
			seen++
		}
		if row.Name == "reader-only" {
			external = row
		}
	}
	if seen != 1 {
		t.Fatalf("plugin %q is registered and configured, and has %d rows", shared, seen)
	}
	if external.Name == "" {
		t.Fatal("the plugin only the config names has no row")
	}
	if external.Kind != kindExternal {
		t.Fatalf("the configured external plugin reads kind %q, want %q", external.Kind, kindExternal)
	}
	if external.State == "" {
		t.Fatal("the configured external plugin has a row with no state")
	}
}

// withQueryBudget sets the answer budget for one test and puts the previous
// environment back afterwards. A test that starts a child needs a budget short
// enough to keep the package's tests fast and long enough that a loaded machine
// still starts a shell.
func withQueryBudget(t *testing.T, budget string) {
	t.Helper()

	if err := env.Set(envQueryTimeout, budget); err != nil {
		t.Fatalf("cannot set the query budget the test runs under: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Unsetenv(envQueryTimeout); err != nil {
			t.Fatalf("cannot clear the query budget the test set: %v", err)
		}
		env.ResetCache()
	})
}

// queryRunString builds a `run` string whose child writes noise, then the
// framed declaration line, then exits 0. That is what a plugin supporting query
// mode does, and the line is produced by the protocol's own writer, so these
// tests read the line the ze binary and the SDK really emit rather than a
// spelling of it written here.
func queryRunString(t *testing.T, noise string, declaration *rpc.DeclareRegistrationInput) string {
	t.Helper()

	var line bytes.Buffer
	if err := rpc.WriteDeclaration(&line, declaration); err != nil {
		t.Fatalf("cannot build the declaration the fake plugin writes: %v", err)
	}
	return "printf '%s' '" + noise + line.String() + "'"
}

// rowsByName indexes an answer by plugin name, so a test names the row it
// asserts about rather than counting positions.
func rowsByName(rows []declarationRow) map[string]declarationRow {
	byName := make(map[string]declarationRow, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}
	return byName
}

// TestDeclarationRowsCarryEveryState holds AC-4: the five states are
// distinguishable, one row each, and no configured plugin is omitted. Each
// plugin below is the run string that produces its state, so a reader that
// collapsed two states into one fails on the row that lost its own answer.
func TestDeclarationRowsCarryEveryState(t *testing.T) {
	withQueryBudget(t, "1s")

	declared := rpc.DeclareRegistrationInput{Commands: []rpc.CommandDecl{{Name: "show query answer"}}}
	var none rpc.DeclareRegistrationInput

	withConfiguredPluginReader(t, func(string) ([]PluginConfig, error) {
		return []PluginConfig{
			{Name: "query-declared", Run: queryRunString(t, "", &declared)},
			{Name: "query-declared-none", Run: queryRunString(t, "", &none)},
			{Name: "query-no-answer", Run: "exit 0"},
			{Name: "query-unstartable", Run: "/no/such/plugin/binary"},
			{Name: "query-timeout", Run: "sleep 30"},
		}, nil
	})

	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows over the five states failed: %v", err)
	}

	want := map[string]string{
		"query-declared":      stateDeclared,
		"query-declared-none": stateDeclaredNone,
		"query-no-answer":     stateNoAnswer,
		"query-unstartable":   stateUnstartable,
		"query-timeout":       stateTimeout,
	}
	byName := rowsByName(rows)
	for name, state := range want {
		row, held := byName[name]
		if !held {
			t.Fatalf("plugin %q is configured and has no row", name)
		}
		if row.State != state {
			t.Fatalf("plugin %q reads state %q with reason %q, want %q", name, row.State, row.Reason, state)
		}
	}

	answered := byName["query-declared"]
	if len(answered.Commands) != 1 || answered.Commands[0].Name != "show query answer" {
		t.Fatalf("the declared row carries commands %v, want the one the child wrote", answered.Commands)
	}
	if len(byName["query-declared-none"].Commands) != 0 {
		t.Fatal("the declared-none row carries a command the child never wrote")
	}
}

// TestDeclarationRowsAnswerInTreeFromTheRegistration holds AC-9: a plugin an
// INTERNAL config block names is answered from the compiled-in registration
// and no process is started for it.
//
// The block is internal because that is the block naming code this binary
// carries. The proof is the run string: it names a binary that cannot start, so
// a reader that started it would answer `unstartable` with no declaration. The
// row reads `declared` and carries the registration's own commands, which is
// only reachable without a process.
func TestDeclarationRowsAnswerInTreeFromTheRegistration(t *testing.T) {
	declaring := declaringRegistration(t)

	withQueryBudget(t, "1s")
	withConfiguredPluginReader(t, func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: declaring.Name, Run: "/no/such/plugin/binary", Internal: true}}, nil
	})

	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows over a configured in-tree plugin failed: %v", err)
	}

	row, held := rowsByName(rows)[declaring.Name]
	if !held {
		t.Fatalf("plugin %q is registered and has no row", declaring.Name)
	}
	if row.State != stateDeclared {
		t.Fatalf("plugin %q reads state %q with reason %q, want %q from the compiled-in registration",
			declaring.Name, row.State, row.Reason, stateDeclared)
	}
	if len(row.Commands) != len(declaring.Commands) {
		t.Fatalf("plugin %q answers %d commands, and its registration declares %d",
			declaring.Name, len(row.Commands), len(declaring.Commands))
	}
	if row.Commands[0].Name != declaring.Commands[0].Name {
		t.Fatalf("plugin %q answers command %q, and its registration declares %q",
			declaring.Name, row.Commands[0].Name, declaring.Commands[0].Name)
	}
}

// TestDeclarationRowsRefuseAnInternalBlockWithNoCodeHere holds the other half
// of AC-9, the guard beside the case above: an `internal` block naming a plugin
// this binary does not carry names code that is in no binary here. There is
// nothing to start and nothing to read, so the row refuses and says why.
//
// The run string would leave a file behind, and the absence of that file is the
// proof that no process was started. A reader that fell through to the query
// would start the operator's run string on the strength of a block that says
// the code is compiled in, which is the one thing an internal block rules out.
func TestDeclarationRowsRefuseAnInternalBlockWithNoCodeHere(t *testing.T) {
	withQueryBudget(t, "1s")

	marker := filepath.Join(t.TempDir(), "the-query-started-it")
	withConfiguredPluginReader(t, func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: "internal-with-no-code-here", Run: "touch " + marker, Internal: true}}, nil
	})

	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows over an internal block with no code here failed: %v", err)
	}

	row, held := rowsByName(rows)["internal-with-no-code-here"]
	if !held {
		t.Fatal("the plugin the config names has no row, and a configured plugin is never omitted")
	}
	if row.State != stateUnstartable {
		t.Fatalf("the internal block reads state %q with reason %q, want %q", row.State, row.Reason, stateUnstartable)
	}
	if row.Reason == "" {
		t.Fatal("a row that carries no declaration must say why, and this one gives no reason")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the reader started the run string an internal block carries, and an internal block is answered from the registration or not at all")
	}
}

// TestDeclarationIgnoresUnframedStdout holds R-3: a child that writes a banner
// before its answer is still read correctly.
//
// The banner opens with `row `, which is an answer line's kind word
// (rpc.AnswerKindRecord). A reader framing this stream with rpc.ScanAnswerLines
// would take that line by the width its fields state, refuse the byte where the
// arithmetic ends, and deliver NOTHING, the framed answer included.
func TestDeclarationIgnoresUnframedStdout(t *testing.T) {
	withQueryBudget(t, "1s")

	declared := rpc.DeclareRegistrationInput{Commands: []rpc.CommandDecl{{Name: "show banner survivor"}}}
	noise := "starting the plugin\nrow plugin banner\n#not-an-id\n"

	withConfiguredPluginReader(t, func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: "query-noisy", Run: queryRunString(t, noise, &declared)}}, nil
	})

	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows over a noisy child failed: %v", err)
	}

	row := rowsByName(rows)["query-noisy"]
	if row.State != stateDeclared {
		t.Fatalf("the noisy child reads state %q with reason %q, want %q", row.State, row.Reason, stateDeclared)
	}
	if len(row.Commands) != 1 || row.Commands[0].Name != "show banner survivor" {
		t.Fatalf("the noisy child answers commands %v, want the one it wrote after the banner", row.Commands)
	}
}

// TestDeclarationBudgetRefusesNonPositive holds the boundary: a budget of zero
// or less is refused rather than read as "wait no time", which would report
// every started plugin as a timeout and look like a fleet of broken plugins.
func TestDeclarationBudgetRefusesNonPositive(t *testing.T) {
	for _, budget := range []string{"0s", "-1s"} {
		t.Run(budget, func(t *testing.T) {
			withQueryBudget(t, budget)

			rows, err := declarationRows("")
			if err == nil {
				t.Fatalf("a budget of %s answered %d rows, want a refusal", budget, len(rows))
			}
			if !strings.Contains(err.Error(), envQueryTimeout) {
				t.Fatalf("the refusal %q does not name %s", err, envQueryTimeout)
			}
		})
	}
}

// liveStageOneBudget bounds the live half of TestDeclarationAnswerMatchesStageOne.
// Both ends of the handshake are in this process over a net.Pipe, so the
// request arrives immediately; the budget exists so a scheduling stall fails
// the test with a sentence rather than hanging until the go test deadline.
const liveStageOneBudget = 5 * time.Second

// stageOneDeclaration is the declaration both halves of
// TestDeclarationAnswerMatchesStageOne send. It is written once, here, because
// the test is about two producers of ONE value: a second literal would compare
// this file with itself and prove nothing about either producer.
//
// It states a field of every kind the message carries, a list of structs, a
// list of strings, a bool and an enum, so a writer that drops or rewrites one
// of them is caught rather than only a writer that disagrees about commands.
func stageOneDeclaration() sdk.Registration {
	return sdk.Registration{
		Families: []rpc.FamilyDecl{{Name: "ipv4/unicast", Mode: "both", AFI: 1, SAFI: 1}},
		Commands: []rpc.CommandDecl{{
			Name:            "show declaration witness",
			Description:     "What this plugin declares at Stage 1.",
			LongHelp:        "The command exists so the declaration carries every field a reader compares.",
			Args:            []string{"name"},
			Completable:     true,
			Hidden:          false,
			DeprecatedNames: []string{"show witness declaration"},
			Shape:           "tab",
			Columns:         []string{"name", "state"},
			AddressFields:   []string{"peer"},
		}},
		Pipes: []rpc.PipeDecl{{
			Command:     "show declaration witness",
			Name:        "declared",
			Description: "Only the rows whose state is declared.",
			Expansion:   "match state declared",
		}},
		Dependencies:        []string{"bgp"},
		WantsConfig:         []string{"plugin"},
		Claims:              []string{"declaration-witness"},
		FailurePolicy:       rpc.FailureRestart,
		SignalsSessionReady: true,
	}
}

// liveStageOneDeclaration answers the declaration a live daemon receives at
// Stage 1 for reg.
//
// The plugin end is a real sdk.Plugin over a net.Pipe, and the engine end is
// the connection type the daemon's own stage driver reads (runStartupHandshake,
// internal/component/plugin/server/startup_driver.go). Nothing answers the
// request, so Run stops inside Stage 1 when the context is canceled. Stage 1
// is already sent by then, and it is the whole of what a query answers.
//
// validateOpen registers an OPEN validation callback, which is the one input
// Run reads outside reg: it derives WantsValidateOpen from the callbacks a
// Plugin holds.
func liveStageOneDeclaration(t *testing.T, reg sdk.Registration, validateOpen bool) rpc.DeclareRegistrationInput {
	t.Helper()

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	witness := sdk.NewWithConn("declaration-witness", pluginEnd)
	t.Cleanup(func() {
		witness.Close() //nolint:errcheck // test cleanup
	})
	if validateOpen {
		witness.OnValidateOpen(func(*sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
			return &sdk.ValidateOpenOutput{Accept: true}
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), liveStageOneBudget)
	defer cancel()

	// One goroutine for one handshake. It ends when the context is canceled
	// below, and the test fails rather than waits if it does not.
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		_ = witness.Run(ctx, reg) //nolint:errcheck // Stage 2 never comes: the error ends the handshake and is not the fact under test
	}()

	engine := pluginipc.NewPluginConn(engineEnd, engineEnd)
	request, err := engine.ReadRequest(ctx)
	if err != nil {
		t.Fatalf("the live plugin sent no Stage 1 request: %v", err)
	}
	if request.Method != rpc.MethodDeclareRegistration {
		t.Fatalf("the live plugin opened with %q, want %q", request.Method, rpc.MethodDeclareRegistration)
	}

	var declaration rpc.DeclareRegistrationInput
	if err := json.Unmarshal(request.Params, &declaration); err != nil {
		t.Fatalf("the live Stage 1 params do not decode: %v", err)
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(liveStageOneBudget):
		t.Fatal("the live plugin did not stop after its context was canceled")
	}
	return declaration
}

// queriedDeclaration answers the declaration a plugin writes under query mode
// for reg.
//
// It runs RunOrDeclare, the entry point a third-party plugin's main calls, and
// reads the answer with declarationFromStdout, the reader `show plugin
// declarations` runs over a queried child's output. Both ends of this half are
// therefore the product's own.
func queriedDeclaration(t *testing.T, reg sdk.Registration) rpc.DeclareRegistrationInput {
	t.Helper()
	withQueryMode(t)

	stdout := captureStdout(t, func() {
		code := sdk.RunOrDeclare(reg, func() int {
			t.Error("the plugin activated under a declaration query")
			return 1
		})
		if code != 0 {
			t.Errorf("the declaration query exited %d, want 0", code)
		}
	})

	declaration, answered := declarationFromStdout(stdout)
	if !answered {
		t.Fatalf("the query wrote no declaration line, only %q", stdout)
	}
	return declaration
}

// withQueryMode asks for query mode for one test and clears the request
// afterwards. It sets the same variable the reader gives a forked child
// (queryChildEnv), so the test asks the way the product asks.
func withQueryMode(t *testing.T) {
	t.Helper()

	if err := env.Set(sdk.EnvPluginMode, sdk.ModeDeclare); err != nil {
		t.Fatalf("cannot ask for query mode: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Unsetenv(sdk.EnvPluginMode); err != nil {
			t.Fatalf("cannot clear the query mode this test asked for: %v", err)
		}
		env.ResetCache()
	})
}

// captureStdout answers what write printed to this process's stdout.
//
// The query entry point writes there because a forked plugin's answer reaches
// its reader down that pipe, so a test that wants the line reads the same
// place rather than a writer the product does not use.
func captureStdout(t *testing.T, write func()) []byte {
	t.Helper()

	type captured struct {
		answer []byte
		err    error
	}

	read, written, err := os.Pipe()
	if err != nil {
		t.Fatalf("cannot open the pipe this test reads stdout through: %v", err)
	}

	// One reader for one pipe. It ends when the write end is closed below,
	// which is the only writer there is.
	held := make(chan captured, 1)
	go func() {
		answer, readErr := io.ReadAll(read)
		held <- captured{answer: answer, err: readErr}
	}()

	previous := os.Stdout
	os.Stdout = written
	write()
	os.Stdout = previous

	if closeErr := written.Close(); closeErr != nil {
		t.Fatalf("cannot close the stdout pipe this test wrote through: %v", closeErr)
	}
	taken := <-held
	if closeErr := read.Close(); closeErr != nil {
		t.Fatalf("cannot close the stdout pipe this test read through: %v", closeErr)
	}
	if taken.err != nil {
		t.Fatalf("cannot read what the query wrote to stdout: %v", taken.err)
	}
	return taken.answer
}

// declarationJSON answers the declaration as the JSON a reader compares. Both
// sides go through one encoder, so the strings differ exactly when the values
// differ, and the difference is readable in the failure message.
func declarationJSON(t *testing.T, declaration *rpc.DeclareRegistrationInput) string {
	t.Helper()

	encoded, err := json.Marshal(declaration)
	if err != nil {
		t.Fatalf("cannot encode a declaration for comparison: %v", err)
	}
	return string(encoded)
}

// TestDeclarationAnswerMatchesStageOne holds AC-7 and validates A-1: the
// declaration a plugin sends at Stage 1 to a live daemon and the declaration
// the same plugin writes under query mode are one value.
//
// Method: one declaration, two producers. The live half runs the real SDK
// startup over a net.Pipe and reads Stage 1 with the engine's own reader. The
// query half runs the real query entry point and parses its stdout with this
// package's own reader. Neither half writes a declaration down: both take
// stageOneDeclaration().
//
// A divergence is a field one producer derives and the other cannot see. Run
// derives exactly one, WantsValidateOpen, from the callbacks a Plugin holds,
// and a query has no Plugin. The second case exercises that field and requires
// every OTHER field to stay equal, so a second derived field fails here rather
// than reaching an operator as a query answer a live daemon disagrees with.
func TestDeclarationAnswerMatchesStageOne(t *testing.T) {
	cases := []struct {
		name         string
		validateOpen bool
	}{
		{name: "nothing is derived from the plugin", validateOpen: false},
		{name: "the one field Run derives", validateOpen: true},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			live := liveStageOneDeclaration(t, stageOneDeclaration(), one.validateOpen)
			queried := queriedDeclaration(t, stageOneDeclaration())

			if live.WantsValidateOpen != one.validateOpen {
				t.Fatalf("the live Stage 1 declaration reads wants-validate-open %t, want %t",
					live.WantsValidateOpen, one.validateOpen)
			}
			if queried.WantsValidateOpen {
				t.Fatal("the query answer derived wants-validate-open, and a query has no Plugin to derive it from")
			}

			// The one derived field is asserted above and aligned here, so the
			// comparison below covers every other field of the message.
			queried.WantsValidateOpen = live.WantsValidateOpen

			liveJSON := declarationJSON(t, &live)
			queriedJSON := declarationJSON(t, &queried)
			if liveJSON != queriedJSON {
				t.Fatalf("the two declarations differ\n  live: %s\n query: %s", liveJSON, queriedJSON)
			}
		})
	}
}

// TestUnstartableOutcomeNamesTheAbsentShell proves the row reports the cause an
// operator can act on. A host with no shell fails every external plugin start
// for one reason, and a row that named the plugin would send the operator to
// the plugin's own code.
//
// The shell answer is a parameter of the function under test, so this test
// states it: no test can take /bin/sh away from the host it runs on.
func TestUnstartableOutcomeNamesTheAbsentShell(t *testing.T) {
	shellErr := errors.New("the shell /bin/sh that starts an external plugin is absent: stat /bin/sh: no such file or directory")

	outcome := unstartableOutcome(errors.New("fork/exec /bin/sh: no such file or directory"), shellErr)

	if outcome.state != stateUnstartable {
		t.Fatalf("state = %q, want %q", outcome.state, stateUnstartable)
	}
	if !strings.Contains(outcome.reason, "/bin/sh") {
		t.Errorf("the reason must name the absent shell, and it says %q", outcome.reason)
	}
	if strings.Contains(outcome.reason, "cannot start the plugin") {
		t.Errorf("the reason must report the shell rather than the plugin, and it says %q", outcome.reason)
	}
}

// TestUnstartableOutcomeReportsTheStartFailureWhenTheShellIsThere holds the
// other half: a start that failed for its own reason keeps that reason.
func TestUnstartableOutcomeReportsTheStartFailureWhenTheShellIsThere(t *testing.T) {
	outcome := unstartableOutcome(errors.New("permission denied"), nil)

	if outcome.state != stateUnstartable {
		t.Fatalf("state = %q, want %q", outcome.state, stateUnstartable)
	}
	if !strings.Contains(outcome.reason, "permission denied") {
		t.Errorf("the reason must carry the start error, and it says %q", outcome.reason)
	}
}

// declaringRegistration answers a plugin this test binary carries that declares
// at least one command, so a test comparing an answer against the compiled-in
// declaration has one to compare with.
func declaringRegistration(t *testing.T) *registry.Registration {
	t.Helper()

	for _, registration := range registry.All() {
		if len(registration.Commands) > 0 {
			return registration
		}
	}
	t.Skip("this test binary carries no plugin that declares a command")
	return nil
}

// TestDeclarationRowsQueryTheBinaryAnExternalBlockNames holds what an EXTERNAL
// block means: it names ANOTHER binary, so the declaration comes from that
// binary and never from a plugin this one happens to carry under the same name.
//
// `external mrt { run "/opt/vendor/my-mrt" }` is a vendor's program that took a
// name Ze also uses. Answering it with Ze's own mrt declaration under a row
// that reads `kind: external` is the silently wrong answer: it has the right
// shape, it names commands the vendor's program does not serve, and no field of
// the row says so.
func TestDeclarationRowsQueryTheBinaryAnExternalBlockNames(t *testing.T) {
	declaring := declaringRegistration(t)

	withQueryBudget(t, "1s")
	vendor := rpc.DeclareRegistrationInput{Commands: []rpc.CommandDecl{{Name: "show vendor answer"}}}
	withConfiguredPluginReader(t, func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: declaring.Name, Run: queryRunString(t, "", &vendor)}}, nil
	})

	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows over an external block naming an in-tree plugin failed: %v", err)
	}

	row, held := rowsByName(rows)[declaring.Name]
	if !held {
		t.Fatalf("the config names plugin %q and it has no row", declaring.Name)
	}
	if row.Kind != kindExternal {
		t.Fatalf("plugin %q is configured external and its row reads kind %q", declaring.Name, row.Kind)
	}
	if row.State != stateDeclared {
		t.Fatalf("plugin %q reads state %q with reason %q, and the binary the block names answered",
			declaring.Name, row.State, row.Reason)
	}
	if len(row.Commands) != 1 || row.Commands[0].Name != "show vendor answer" {
		t.Fatalf("plugin %q answers commands %v, and the binary the block names declares only %q",
			declaring.Name, row.Commands, "show vendor answer")
	}
}

// TestDeclarationQueryStopsThePluginTheShellStarted holds the boundary row: a
// plugin that writes no declaration inside the budget is STOPPED, and the
// plugin is what the shell started rather than the shell alone.
//
// Method: a run string carrying a shell operator, so the shell cannot replace
// itself with the one command it was given and stays alive with the plugin as
// its child. The plugin writes its pid and then sleeps well past the budget. A
// stop aimed at the direct child alone leaves that pid running, which is a
// plugin running its LIVE start past the budget: the one outcome query mode
// exists to prevent. So this test reads the pid the plugin wrote and requires
// the process to be gone.
func TestDeclarationQueryStopsThePluginTheShellStarted(t *testing.T) {
	withQueryBudget(t, "1s")

	pidPath := filepath.Join(t.TempDir(), "plugin.pid")
	run := "sh -c 'echo $$ > " + pidPath + "; exec sleep 30' & wait"

	withConfiguredPluginReader(t, func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: "query-slow-plugin", Run: run}}, nil
	})

	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows over a plugin that outlives the budget failed: %v", err)
	}
	row := rowsByName(rows)["query-slow-plugin"]
	if row.State != stateTimeout {
		t.Fatalf("the slow plugin reads state %q with reason %q, want %q", row.State, row.Reason, stateTimeout)
	}

	pid := pluginPid(t, pidPath)
	t.Cleanup(func() {
		// The test stops what the product failed to stop, so a failure here
		// leaves no process sleeping for another 30 seconds. The signal fails
		// when the product did stop it, which is the passing case.
		_ = syscall.Kill(pid, syscall.SIGKILL) //nolint:errcheck // the process is already gone when the product stopped it
	})

	if processAlive(pid, processStopBudget) {
		t.Fatalf("the plugin at pid %d is still running after the query: the stop reached the shell and not the plugin", pid)
	}
}

// processStopBudget bounds how long the test above waits for the stopped
// process to leave the process table. The signal is delivered before the query
// returns, so the wait covers the kernel's own reaping and nothing else.
const processStopBudget = 2 * time.Second

// pluginPid answers the pid the fake plugin wrote, and fails the test when
// nothing wrote one: a plugin that never started proves nothing about stopping
// it.
func pluginPid(t *testing.T, path string) int {
	t.Helper()

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the fake plugin wrote no pid, so it never started: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(written)))
	if err != nil {
		t.Fatalf("the fake plugin wrote %q as its pid: %v", written, err)
	}
	return pid
}

// processAlive answers whether pid is still in the process table, waiting up to
// budget for it to go. Signal 0 asks the kernel about the process and delivers
// nothing to it.
func processAlive(pid int, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for {
		if err := syscall.Kill(pid, 0); err != nil {
			return false
		}
		if time.Now().After(deadline) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestQueryChildEnvDropsEveryPluginVariableSpelling holds the credential
// guarantee: a queried child is given no variable in the ze.plugin. namespace
// but the mode, whichever spelling that variable arrived under.
//
// The spellings are not hypothetical. The daemon's own fork writes
// ZE_PLUGIN_HUB_TOKEN, env.Set writes the canonical ze.plugin.hub.token with
// os.Setenv, and env.Get reads dots and underscores as one separator, so a
// variable this process can read arrives under either. A drop that matched the
// OS spelling alone would hand a plugin the hub token under the other one.
func TestQueryChildEnvDropsEveryPluginVariableSpelling(t *testing.T) {
	const token = "the-hub-token-a-query-must-not-carry"
	t.Setenv("ze.plugin.hub.token", token)
	t.Setenv("ZE_PLUGIN_HUB_HOST", "10.0.0.1")
	t.Setenv("Ze_Plugin_Ca_Pem", "-----BEGIN CERTIFICATE-----")

	childEnv := queryChildEnv()

	modeName := strings.ToUpper(strings.ReplaceAll(sdk.EnvPluginMode, ".", "_"))
	for _, entry := range childEnv {
		name, value, _ := strings.Cut(entry, "=")
		if !env.InNamespace(name, childEnvPluginNamespace) {
			continue
		}
		if name != modeName {
			t.Errorf("the child is given %q, and the mode is the only plugin variable it may receive", name)
			continue
		}
		if value != sdk.ModeDeclare {
			t.Errorf("the child is given mode %q, want %q", value, sdk.ModeDeclare)
		}
	}

	for _, entry := range childEnv {
		if strings.Contains(entry, token) {
			t.Fatalf("the hub token reached the queried child, in %q", entry)
		}
	}
}
