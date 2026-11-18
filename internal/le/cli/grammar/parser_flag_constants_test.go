// Design: docs/architecture/cli/command-namespacing.md -- offline flag declarations.
// Related: flags.go -- parser and registry names share package constant resolution.

package cligrammar

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/grammar"
)

// TestAParserFlagNamedByAPackageConstantIsRead checks a parser whose flag name
// lives in another source file, with its independent registration in a third.
// Removing that registration must still report the parsed flag; an unknown
// parser identifier remains counted as unresolved in both cases.
func TestAParserFlagNamedByAPackageConstantIsRead(t *testing.T) {
	for _, registered := range []bool{true, false} {
		name := "registered"
		if !registered {
			name = "undeclared"
		}
		t.Run(name, func(t *testing.T) {
			files := cleanFixture(t)
			files["cmd/ze/roots.go"] = "package main\n\n" +
				"func wire() {\n" +
				"\tregistry.MustRegisterRootHandler(\"env\", nil, meta)\n" +
				"\tregistry.MustRegisterRootHandler(\"fixture\", nil, meta)\n" +
				"}\n"
			files["internal/fixture/flag_name.go"] = "package fixture\n\nconst flagBackup = \"backup\"\n"
			files["internal/fixture/main.go"] = "package fixture\n\n" +
				"func run(args []string) int {\n" +
				"\tfs := flag.NewFlagSet(\"fixture\", flag.ContinueOnError)\n" +
				"\tfs.String(flagBackup, \"\", \"backup artifact\")\n" +
				"\tfs.String(\"user\", \"\", \"account\")\n" +
				"\tfs.String(flagUnknown, \"\", \"unresolved name\")\n" +
				"\treturn use(fs.Parse(args))\n" +
				"}\n"
			backup := ""
			if registered {
				backup = "\t\t{Name: \"--backup\", Description: \"backup artifact\"},\n"
			}
			files["internal/fixture/register.go"] = "package fixture\n\n" +
				"func init() {\n" +
				"\tregistry.RegisterCommandFlags(\"fixture\", []registry.FlagSpec{\n" +
				"\t\t{Name: \"--user\", Description: \"account\"},\n" + backup +
				"\t})\n" +
				"}\n"

			result, err := Check(writeTree(t, files), Floor{}, leProbeRoots())
			if err != nil {
				t.Fatalf("the gate failed over the fixture: %v", err)
			}
			want := 1
			if registered {
				want = 0
			}
			if got := rulesFound(result)[grammar.RuleFlagUndeclared]; got != want {
				t.Errorf("F4 findings = %d, want %d:\n%s", got, want, result.Text())
			}
			for _, finding := range result.FlagFindings {
				if finding.Rule != grammar.RuleFlagUndeclared {
					continue
				}
				if finding.Command != "fixture" {
					t.Errorf("F4 command = %q, want fixture", finding.Command)
				}
				if finding.Flag != "--backup" {
					t.Errorf("F4 flag = %q, want --backup", finding.Flag)
				}
			}
			if result.FlagNamesUnresolved != 1 {
				t.Errorf("unresolved flag names = %d, want only flagUnknown", result.FlagNamesUnresolved)
			}
		})
	}
}

// TestParserFlagNamesUseTheirLexicalBinding keeps parameters and variables
// unresolved while reading a shadowing constant's own literal. An outer
// package constant with the same name must not supply another flag spelling.
func TestParserFlagNamesUseTheirLexicalBinding(t *testing.T) {
	for _, test := range []struct {
		name, parameter, declaration, registered string
		unresolved                               int
	}{
		{"parameter", ", flagBackup string", "", "", 2},
		{"local variable", "", "\tflagBackup := \"runtime-backup\"\n", "", 2},
		{"local constant", "", "\tconst flagBackup = \"local-backup\"\n", "--local-backup", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := cleanFixture(t)
			files["cmd/ze/roots.go"] = "package main\n\n" +
				"func wire() {\n" +
				"\tregistry.MustRegisterRootHandler(\"env\", nil, meta)\n" +
				"\tregistry.MustRegisterRootHandler(\"fixture\", nil, meta)\n" +
				"}\n"
			files["internal/fixture/flag_name.go"] = "package fixture\n\nconst flagBackup = \"backup\"\n"
			files["internal/fixture/main.go"] = "package fixture\n\n" +
				"func run(args []string" + test.parameter + ") int {\n" + test.declaration +
				"\tfs := flag.NewFlagSet(\"fixture\", flag.ContinueOnError)\n" +
				"\tfs.String(flagBackup, \"\", \"backup artifact\")\n" +
				"\tfs.String(\"user\", \"\", \"account\")\n" +
				"\tfs.String(flagUnknown, \"\", \"unresolved name\")\n" +
				"\treturn use(fs.Parse(args))\n" +
				"}\n"
			backup := ""
			if test.registered != "" {
				backup = "\t\t{Name: \"" + test.registered + "\", Description: \"backup artifact\"},\n"
			}
			files["internal/fixture/register.go"] = "package fixture\n\n" +
				"func init() {\n" +
				"\tregistry.RegisterCommandFlags(\"fixture\", []registry.FlagSpec{\n" +
				"\t\t{Name: \"--user\", Description: \"account\"},\n" + backup +
				"\t})\n" +
				"}\n"

			result, err := Check(writeTree(t, files), Floor{}, leProbeRoots())
			if err != nil {
				t.Fatalf("the gate failed over the fixture: %v", err)
			}
			if got := rulesFound(result)[grammar.RuleFlagUndeclared]; got != 0 {
				t.Errorf("F4 findings = %d, want no borrowed outer spelling:\n%s", got, result.Text())
			}
			if result.FlagNamesUnresolved != test.unresolved {
				t.Errorf("unresolved flag names = %d, want %d", result.FlagNamesUnresolved, test.unresolved)
			}
		})
	}
}
