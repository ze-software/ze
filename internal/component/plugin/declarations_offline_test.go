// Design: docs/architecture/api/commands.md -- where a command is served
// Overview: declarations.go -- the command these tests exercise
// Related: declarations_test.go -- the same row builder as a running daemon answers it
//
// Goal: prove that `show plugin declarations config <path>` starts no external
// plugin's program in a command with no daemon, and says so in the row. The
// contract is the owner ruling: "plugin command requiring external plugins
// which can not be started from the code when no daemon is running to pass it
// to are not available, for command built-in then yes you should run them as
// we can look them up in the yang hardcoded in the binary".
//
// Method: each run string leaves a marker file when a shell starts it, so the
// absence of the marker is the proof that nothing was started. The same run
// string is then given to the row builder as a running daemon answers it, and
// the marker it leaves there is the control: without it, an absent marker
// would prove nothing.

package plugin

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// markingRunString answers a run string whose program leaves marker when it
// is started, in any mode, and then answers a declaration query with one
// command, as a conforming plugin does. The declaration line is the one the
// protocol's own writer produces, kept in a file the program prints, so the
// run string holds no quote and reads in a row's reason as an operator would
// type it.
func markingRunString(t *testing.T, marker string) string {
	t.Helper()

	declaration := rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show marking answer"}},
	}
	var line bytes.Buffer
	if err := rpc.WriteDeclaration(&line, &declaration); err != nil {
		t.Fatalf("cannot build the declaration the marking plugin writes: %v", err)
	}
	answer := filepath.Join(t.TempDir(), "declaration")
	if err := os.WriteFile(answer, line.Bytes(), 0o600); err != nil {
		t.Fatalf("cannot write the declaration the marking plugin prints: %v", err)
	}
	return "touch " + marker + "; cat " + answer
}

// started reports whether the program of a markingRunString left its marker.
func started(t *testing.T, marker string) bool {
	t.Helper()

	_, err := os.Stat(marker)
	if err == nil {
		return true
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cannot look for the marker %s: %v", marker, err)
	}
	return false
}

// TestDeclarationRowsStartNoExternalProgramWithNoDaemon holds the ruling: a
// command with no daemon starts no program an external block names, and the
// row says the declaration is not available and how to read it.
//
// VALIDATES: the row keeps the plugin as kind external in state not-available,
// carries no command, and its reason names the daemon form of this command,
// the session that reaches a running ze and the conformance checker with the
// block's own run string. The marker is absent after the answer.
// PREVENTS: inspecting a config someone else wrote running the commands it
// names; an external plugin dropped from the answer; a row that reads as a
// plugin with nothing to declare.
func TestDeclarationRowsStartNoExternalProgramWithNoDaemon(t *testing.T) {
	withQueryBudget(t, "1s")

	marker := filepath.Join(t.TempDir(), "started-with-no-daemon")
	run := markingRunString(t, marker)
	configured := func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: "acme-offline", Run: run}}, nil
	}

	withPluginReaderInProcess(t, ConfigProcessOffline, configured)
	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows with no daemon failed: %v", err)
	}
	if started(t, marker) {
		t.Fatal("a command with no daemon started the program an external block names")
	}

	row, held := rowsByName(rows)["acme-offline"]
	if !held {
		t.Fatal("the external plugin the config declares has no row, and a configured plugin is never omitted")
	}
	if row.Kind != kindExternal {
		t.Fatalf("the row reads kind %q, want %q", row.Kind, kindExternal)
	}
	if row.State != stateNotAvailable {
		t.Fatalf("the row reads state %q with reason %q, want %q", row.State, row.Reason, stateNotAvailable)
	}
	if len(row.Commands) != 0 || len(row.Pipes) != 0 {
		t.Fatalf("a row nobody read a declaration for carries commands %v and pipes %v", row.Commands, row.Pipes)
	}
	for _, want := range []string{
		"not available with no daemon running",
		"starts no external plugin",
		"`show plugin declarations config ze.conf`",
		"`ze cli`",
		`ze show plugin conformance run "` + run + `"`,
	} {
		if !strings.Contains(row.Reason, want) {
			t.Fatalf("the reason does not hold %q:\n%s", want, row.Reason)
		}
	}
	if strings.Contains(row.Reason, "ze cli -c") {
		t.Fatalf("the reason names `ze cli -c`, which answers in the caller's own process and starts nothing:\n%s", row.Reason)
	}

	// Control: a running daemon starts the same run string, the marker shows
	// it, and the row carries what the program declared.
	withPluginReaderInProcess(t, ConfigProcessDaemon, configured)
	rows, err = declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows inside a daemon failed: %v", err)
	}
	if !started(t, marker) {
		t.Fatal("the marker does not show a start inside a daemon, so its absence above proves nothing")
	}
	row = rowsByName(rows)["acme-offline"]
	if row.State == stateNotAvailable {
		t.Fatalf("a running daemon answered the row as not available: reason %q", row.Reason)
	}
	if len(row.Commands) != 1 || row.Commands[0].Name != "show marking answer" {
		t.Fatalf("a running daemon read commands %v in state %q with reason %q, want the one the program declared",
			row.Commands, row.State, row.Reason)
	}
}

// TestDeclarationRowsWithNoDaemonKeepTheCompiledInDeclarations holds the other
// half of the ruling: what the binary carries is still answered with no
// daemon, from the registration.
//
// VALIDATES: with an external block in the file and no daemon, a compiled-in
// plugin keeps its row as internal, declared, with its registration's
// commands.
// PREVENTS: the offline answer withholding a declaration the binary holds.
func TestDeclarationRowsWithNoDaemonKeepTheCompiledInDeclarations(t *testing.T) {
	declaring := declaringRegistration(t)
	withQueryBudget(t, "1s")

	withPluginReaderInProcess(t, ConfigProcessOffline, func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: "acme-offline", Run: "/no/such/plugin/binary"}}, nil
	})
	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows with no daemon failed: %v", err)
	}

	row, held := rowsByName(rows)[declaring.Name]
	if !held {
		t.Fatalf("plugin %q is compiled in and has no row with no daemon", declaring.Name)
	}
	if row.Kind != kindInternal || row.State != stateDeclared {
		t.Fatalf("plugin %q reads kind %q state %q with no daemon, want %q %q from the registration",
			declaring.Name, row.Kind, row.State, kindInternal, stateDeclared)
	}
	if len(row.Commands) != len(declaring.Commands) {
		t.Fatalf("plugin %q answers %d commands with no daemon, and its registration declares %d",
			declaring.Name, len(row.Commands), len(declaring.Commands))
	}
}

// TestDeclarationRowsWithNoDaemonNeverAnswerAnExternalBlockFromTheBinary holds
// what an external block means with no daemon: it names another program, so
// Ze's own declaration under the same name is not its answer, and with no
// daemon nobody asked that program.
//
// VALIDATES: an external block that takes a compiled-in plugin's name reads
// kind external, state not-available, with no command, and is not started.
// PREVENTS: the offline answer filling an external row from the compiled-in
// registration, which names commands the vendor's program does not serve.
func TestDeclarationRowsWithNoDaemonNeverAnswerAnExternalBlockFromTheBinary(t *testing.T) {
	declaring := declaringRegistration(t)
	withQueryBudget(t, "1s")

	marker := filepath.Join(t.TempDir(), "started-under-a-compiled-in-name")
	withPluginReaderInProcess(t, ConfigProcessOffline, func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: declaring.Name, Run: markingRunString(t, marker)}}, nil
	})
	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows with no daemon failed: %v", err)
	}

	row := rowsByName(rows)[declaring.Name]
	if row.Kind != kindExternal || row.State != stateNotAvailable {
		t.Fatalf("the external block named %q reads kind %q state %q, want %q %q",
			declaring.Name, row.Kind, row.State, kindExternal, stateNotAvailable)
	}
	if len(row.Commands) != 0 {
		t.Fatalf("the external block named %q carries commands %v, and nobody asked its program", declaring.Name, row.Commands)
	}
	if started(t, marker) {
		t.Fatal("a command with no daemon started the program an external block names")
	}
}

// TestDeclarationRowsWithNoDaemonKeepABlockWithNoProgram holds the rows that
// need no process: a block with nothing to start is answered the same with no
// daemon as inside one, because nothing about it waits on a program.
//
// VALIDATES: an internal block for a plugin this binary does not carry, and an
// external block with no run command, read unstartable with their own reason,
// never not-available.
// PREVENTS: the offline answer telling an operator to ask a running ze about a
// block no ze could start.
func TestDeclarationRowsWithNoDaemonKeepABlockWithNoProgram(t *testing.T) {
	withQueryBudget(t, "1s")

	marker := filepath.Join(t.TempDir(), "started-from-an-internal-block")
	withPluginReaderInProcess(t, ConfigProcessOffline, func(string) ([]PluginConfig, error) {
		return []PluginConfig{
			{Name: "internal-with-no-code-here", Run: "touch " + marker, Internal: true},
			{Name: "external-with-no-run"},
		}, nil
	})
	rows, err := declarationRows("ze.conf")
	if err != nil {
		t.Fatalf("declarationRows with no daemon failed: %v", err)
	}

	byName := rowsByName(rows)
	want := map[string]string{
		"internal-with-no-code-here": "this binary does not carry it",
		"external-with-no-run":       "names no run command",
	}
	for name, reason := range want {
		row, held := byName[name]
		if !held {
			t.Fatalf("plugin %q is configured and has no row", name)
		}
		if row.State != stateUnstartable {
			t.Fatalf("plugin %q reads state %q with reason %q, want %q", name, row.State, row.Reason, stateUnstartable)
		}
		if !strings.Contains(row.Reason, reason) {
			t.Fatalf("plugin %q gives reason %q, want one that holds %q", name, row.Reason, reason)
		}
	}
	if started(t, marker) {
		t.Fatal("the run string an internal block carries was started")
	}
}

// TestDeclarationRowsRefuseAReaderThatNamesNoProcess holds the guard on the
// seam: a reader that did not say whether a daemon runs here is refused, and
// nothing is started on the strength of the missing answer.
//
// VALIDATES: ConfigProcessUnspecified answers errNoConfigProcess and no row.
// PREVENTS: the zero value read as "a daemon runs here", which would start an
// operator's program in a command with no daemon.
func TestDeclarationRowsRefuseAReaderThatNamesNoProcess(t *testing.T) {
	withQueryBudget(t, "1s")

	marker := filepath.Join(t.TempDir(), "started-by-an-unnamed-process")
	withPluginReaderInProcess(t, ConfigProcessUnspecified, func(string) ([]PluginConfig, error) {
		return []PluginConfig{{Name: "acme-unnamed", Run: markingRunString(t, marker)}}, nil
	})

	rows, err := declarationRows("ze.conf")
	if !errors.Is(err, errNoConfigProcess) {
		t.Fatalf("declarationRows answered %d rows and error %v, want errNoConfigProcess", len(rows), err)
	}
	if started(t, marker) {
		t.Fatal("a reader that named no process had its external plugin started")
	}
}

// TestUnavailableDeclarationRowNamesCommandsAnOperatorCanType holds the two
// boundary spellings of the reason: a config read from standard input has no
// file name to give a running ze, and a run string holding a double quote
// cannot be pasted inside double quotes.
//
// VALIDATES: standard input is named `<config-file>`, never `-`; a quoted run
// string is named, never spelled as a command that would not parse.
// PREVENTS: a fix line that names the file `-` to a daemon that refuses
// standard input, and a pasted command the shell reads as something else.
func TestUnavailableDeclarationRowNamesCommandsAnOperatorCanType(t *testing.T) {
	fromStdin := unavailableDeclarationRow(PluginConfig{Name: "acme", Run: "./acme --flag"}, "-")
	if !strings.Contains(fromStdin.Reason, "`show plugin declarations config <config-file>`") {
		t.Fatalf("a config read from standard input is not named as a placeholder:\n%s", fromStdin.Reason)
	}
	if !strings.Contains(fromStdin.Reason, `ze show plugin conformance run "./acme --flag"`) {
		t.Fatalf("the reason does not spell the conformance command with the run string:\n%s", fromStdin.Reason)
	}

	quoted := unavailableDeclarationRow(PluginConfig{Name: "acme", Run: `./acme --name "a b"`}, "ze.conf")
	if strings.Contains(quoted.Reason, `--name "a b"`) {
		t.Fatalf("a run string holding a double quote is spelled inside double quotes:\n%s", quoted.Reason)
	}
	if !strings.Contains(quoted.Reason, "<the run string of this plugin's config block>") {
		t.Fatalf("a run string holding a double quote is not named:\n%s", quoted.Reason)
	}
}
