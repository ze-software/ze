// GOAL: an ExaBGP `run` line that names a program and its arguments migrates
// into a ze bridge `run` line the bridge splits into the argv ExaBGP would
// have executed.
//
// METHOD: migrate a process block whose `run` carries arguments, one quoted
// with a space and one quoted with an apostrophe, then split the migrated
// command line with the splitter the bridge runner uses and compare the argv
// with ExaBGP's.
//
// PREVENTS: `ze exabgp migrate` refusing `run python3 /opt/scripts/inject.py;`
// with "expected ';' after run value, got WORD", which it did because the
// ExaBGP schema declared `run` as a one-word leaf. ExaBGP takes every word
// left in the statement: 4.2.25 and 5.0.1 `run` (configuration/process/
// parser.py) return `[prg] + [_ for _ in tokeniser.generator]`, and the main
// pin's `Program.parse` (configuration/grammar/types/basic.py) returns
// `[program] + [token.word for token in words.rest()]`, the list
// `subprocess.Popen` executes with no shell.

package migration

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	// The config modules of the roots the migrated config writes, so ze's
	// parser can read it back: bgp (which imports hub), plugin, and the
	// bridge's exabgp root.
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/yang"
	_ "github.com/ze-software/ze/internal/plugins/exabgp/bridgeplugin/yang"
)

// TestMigrateRunCarriesEveryArgument migrates a multi-word `run` and checks
// both the migrated command line and the argv the bridge splits it into.
func TestMigrateRunCarriesEveryArgument(t *testing.T) {
	const source = `
process inject {
	run python3 /opt/scripts/inject.py --label "two words" --quote "it's" plain;
	encoder json;
}
neighbor 10.0.0.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ inject ];
	}
}
`
	tree, err := ParseExaBGPConfig(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(result.Processes) != 1 {
		t.Fatalf("processes = %d, want 1: %+v", len(result.Processes), result.Processes)
	}

	command := result.Processes[0].RunCmd
	const commandWant = `python3 /opt/scripts/inject.py --label 'two words' --quote 'it'\''s' plain`
	if command != commandWant {
		t.Errorf("run = %q, want %q", command, commandWant)
	}

	// The bridge runner reads the run leaf from the ze config the operator
	// loads, so the line goes through the serializer and ze's parser first.
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load ze schema: %v", err)
	}
	output := SerializeTree(result.Tree)
	reparsed, err := config.NewParser(schema).Parse(output)
	if err != nil {
		t.Fatalf("ze refuses the migrated config: %v\n%s", err, output)
	}
	process := reparsed.GetContainer("exabgp").GetContainer("bridge").GetList("process")["inject"]
	if process == nil {
		t.Fatalf("migrated config has no bridge process inject:\n%s", output)
	}
	run, ok := process.Get("run")
	if !ok {
		t.Fatalf("bridge process inject has no run:\n%s", output)
	}
	argv, err := bridge.SplitCommandLine(run)
	if err != nil {
		t.Fatalf("split %q: %v", run, err)
	}
	argvWant := []string{"python3", "/opt/scripts/inject.py", "--label", "two words", "--quote", "it's", "plain"}
	if !slices.Equal(argv, argvWant) {
		t.Errorf("argv = %q, want ExaBGP's %q", argv, argvWant)
	}
}
