// VALIDATES: ai/rules/cli.md "`--flag` or Keyword: Which Register a Token
// Belongs To" -- F1 a flag that is a command name, F2 a flag a client sends to
// the daemon, F3 a flag that repeats a pipe operator, F4 a flag the registry and
// the parser disagree about.
// PREVENTS: a feeder that cannot fail. Each rule is checked over a fixture that
// violates it AND over one that obeys it, so a check that answered nothing
// (an empty population, a predicate inverted) fails here rather than passing a
// tree it never judged.

package grammar

import (
	"strings"
	"testing"
)

// VALIDATES: the flag shape the daemon refuses by, which this package and
// (*Dispatcher).Dispatch now share.
// PREVENTS: a gate that judged shape differently from the daemon, which would
// pass a command string the daemon rejects. "-", "--" and a negative number are
// values, not flags.
func TestFlagShapeMatchesWhatTheDaemonRefuses(t *testing.T) {
	for _, token := range []string{"-u", "--user", "-V", "--dry-run"} {
		if !FlagShaped(token) {
			t.Errorf("%q is not read as a flag", token)
		}
	}
	for _, token := range []string{"-", "--", "-5", "", "user", "10.0.0.1/24"} {
		if FlagShaped(token) {
			t.Errorf("%q is read as a flag", token)
		}
	}
}

// VALIDATES: F1 -- a registered root spelled as a flag is a command name in
// disguise, and the four universal flags are the stated exception.
// PREVENTS: the hole CheckRootNamespace leaves on purpose. It skips a
// leading-hyphen root because R9 asks about namespaces, so before this check
// nothing judged the shape at all.
func TestARootSpelledAsAFlagIsAFinding(t *testing.T) {
	findings := CheckRootFlagForm([]string{"--plugins", "env", "--version", "-V", "--help", "-h"})
	if len(findings) != 1 {
		t.Fatalf("CheckRootFlagForm drew %d findings, want the one --plugins row: %+v", len(findings), findings)
	}
	if findings[0].Command != "--plugins" || findings[0].Rule != RuleFlagIsACommand {
		t.Errorf("the finding is %+v, want F1 on --plugins", findings[0])
	}
	if !strings.Contains(findings[0].Message, "plugins") {
		t.Errorf("the message does not name the keyword to register instead: %q", findings[0].Message)
	}
}

// VALIDATES: F2 -- a command string naming a daemon path and carrying a flag.
// PREVENTS: the `ze interface migrate` defect, where the client built
// "interface migrate --from ..." and the daemon refused it before any handler
// ran, on every invocation, while both halves read as finished code.
func TestAClientCommandStringCarryingAFlagIsAFinding(t *testing.T) {
	paths := map[string]bool{"request interface migrate": true, "interface migrate": true}

	path, flag, found := DaemonCommandFlag("interface migrate --from %s.%d --to %s.%d", paths)
	if !found {
		t.Fatal("a command string built with --from was not found")
	}
	if path != "interface migrate" || flag != "--from" {
		t.Errorf("the finding names %q / %q, want the path and its first flag", path, flag)
	}

	longest, _, found := DaemonCommandFlag("request interface migrate --from a", paths)
	if !found || longest != "request interface migrate" {
		t.Errorf("the verb-carrying spelling matched %q, want the longest path", longest)
	}
	if finding := ClientFlagFinding(path, flag); finding.Rule != RuleFlagToTheDaemon || finding.Flag != "--from" {
		t.Errorf("the rendered finding is %+v, want F2 carrying the flag", finding)
	}
}

// VALIDATES: F2 does not fire on prose, on a usage line, or on a flag that
// precedes any command path.
// PREVENTS: a feeder so noisy it gets switched off. A usage string opens with
// the binary name, and a format string can start with a flag.
func TestF2LeavesProseAndUsageLinesAlone(t *testing.T) {
	paths := map[string]bool{"request interface migrate": true, "interface migrate": true}
	for _, text := range []string{
		"ze interface migrate [--user <name>] from <name>.<unit>",
		"--from is required",
		"interface migrate from eth0.0 to lo1.0",
		"some sentence with --dashes but no command",
	} {
		if _, _, found := DaemonCommandFlag(text, paths); found {
			t.Errorf("%q was read as a client command string", text)
		}
	}
}

// VALIDATES: F3 -- a flag that renders an answer the pipe layer already
// renders, on a command served with data.
// PREVENTS: `--json` and `| json` living side by side, where an operator learns
// one job twice and only one of the two composes.
func TestAFlagThatRepeatsAPipeOperatorIsAFinding(t *testing.T) {
	findings := CheckPipeFlags("config dump", []string{"json", "strip-private"})
	if len(findings) != 1 {
		t.Fatalf("CheckPipeFlags drew %d findings, want the one --json row: %+v", len(findings), findings)
	}
	if findings[0].Rule != RuleFlagIsAPipe || findings[0].Flag != "--json" {
		t.Errorf("the finding is %+v, want F3 on --json", findings[0])
	}
	if len(CheckPipeFlags("config dump", []string{"strip-private", "user"})) != 0 {
		t.Error("a flag that renders nothing was read as a pipe operator")
	}
}

// VALIDATES: F4 -- the flag registry and the parser agree, in both directions.
// PREVENTS: a flag the handler reads that completion never offers, and a flag
// the help promises that the handler drops.
func TestTheRegistryAndTheParserMustAgree(t *testing.T) {
	undeclared := CheckFlagDeclarations("config dump", []string{"json", "strip-private"}, []string{"--json"})
	if len(undeclared) != 1 || undeclared[0].Flag != "--strip-private" {
		t.Fatalf("the parsed-but-undeclared direction drew %+v, want the one --strip-private row", undeclared)
	}

	unparsed := CheckFlagDeclarations("config dump", []string{"json"}, []string{"--json", "--pretty"})
	if len(unparsed) != 1 || unparsed[0].Flag != "--pretty" {
		t.Fatalf("the declared-but-unparsed direction drew %+v, want the one --pretty row", unparsed)
	}

	if findings := CheckFlagDeclarations("l2tp show", []string{"user", "u"}, []string{"--user", "-u"}); len(findings) != 0 {
		t.Errorf("a path whose registry matches its parser drew %+v", findings)
	}
}

// VALIDATES: a short name and its typed token are one flag on both sides of the
// comparison.
// PREVENTS: "-u" and "u" counted as two flags, which would report every short
// alias as undeclared drift.
func TestAFlagNameAndItsTokenAreOneFlag(t *testing.T) {
	if FlagToken("u") != "-u" || FlagToken("user") != "--user" || FlagToken("--user") != "--user" {
		t.Errorf("FlagToken renders %q / %q / %q", FlagToken("u"), FlagToken("user"), FlagToken("--user"))
	}
	if FlagName("--json") != "json" || FlagName("json") != "json" {
		t.Errorf("FlagName answers %q / %q", FlagName("--json"), FlagName("json"))
	}
}
