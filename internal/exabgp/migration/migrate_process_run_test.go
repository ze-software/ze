// Design: docs/architecture/exabgp-bridge.md -- multi-word run argument preservation through bus endpoint migration
// Related: migrate.go -- migrateProcesses
// Related: migrate_bus.go -- buildWrapperRunCommand
//
// VALIDATES: an ExaBGP `run` line with arguments containing spaces and
// apostrophes migrates into a `bus > endpoint <name> > external > run` line
// whose wrapper command splits back into the exact argv ExaBGP executes.
// PREVENTS: `ze exabgp migrate` refusing or mangling quoted arguments when
// embedding them after `ze exabgp plugin` inside a double-quoted Ze leaf.

package migration

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// TestMigrateRunCarriesEveryArgument migrates a multi-word `run` and checks
// both the migrated command line and the argv the shell/bridge splits it into.
//
// VALIDATES: Arguments with spaces and apostrophes survive JoinCommandLine ->
// SerializeTree -> Ze parser -> SplitCommandLine unchanged.
// PREVENTS: Unescaped quotes or backslashes corrupting argv on the wrapper run line.
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

	// The process manager reads the run leaf from the ze config the operator
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
	ep := reparsed.GetContainer("bus").GetList("endpoint")["inject"]
	if ep == nil {
		t.Fatalf("migrated config has no bus endpoint inject:\n%s", output)
	}
	run, ok := ep.GetContainer("external").Get("run")
	if !ok {
		t.Fatalf("bus endpoint inject has no external > run:\n%s", output)
	}
	argv, err := bridge.SplitCommandLine(run)
	if err != nil {
		t.Fatalf("split %q: %v", run, err)
	}
	argvWant := []string{
		"ze", "exabgp", "plugin", "--encoder", "json",
		"python3", "/opt/scripts/inject.py", "--label", "two words", "--quote", "it's", "plain",
	}
	if !slices.Equal(argv, argvWant) {
		t.Errorf("argv = %q, want %q", argv, argvWant)
	}
}
