package runner

// VALIDATES: the parse suite's .ci dialect. A directive no arm of ciDirectives
//   reads fails the file at discovery and names itself, and every assertion the
//   dialect carries discriminates in BOTH polarities.
// PREVENTS: the defect this file's subject was written for -- a CutPrefix chain
//   with no default arm dropped an unrecognized directive in silence, so twenty
//   committed lines across nine test/parse files asserted nothing while reading
//   as proof. It also pins the vocabulary to the generic parser's, because two
//   parsers over one corpus is what let `expect=stdout:not:contains=` mean
//   "absent" here and "present" to every gate that walks the same tree.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// discoverParseCI writes one .ci into a temp directory and runs the real
// discovery entry point over it. Driving Discover rather than parseCIFile is
// deliberate: Discover is what a suite run calls, and it is where a parse error
// becomes a recorded failure instead of an abandoned directory
// (ai/rules/testing.md, drive the guard from its entry point).
func discoverParseCI(t *testing.T, content string) *parsingTest {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dialect.ci"), []byte(content), 0o644); err != nil {
		t.Fatalf("write .ci: %v", err)
	}
	ResetNickCounter()
	pt := NewParsingTests(dir)
	if err := pt.Discover(dir); err != nil {
		t.Fatalf("discover: %v", err)
	}
	registered := pt.Registered()
	if len(registered) != 1 {
		t.Fatalf("discovered %d tests, want 1", len(registered))
	}
	return registered[0]
}

// The smallest file the parse suite accepts: one command and one assertion.
const dialectBaseCI = "cmd=foreground:seq=1:exec=ze --version\n"

func TestParseCIRefusesUnknownDirective(t *testing.T) {
	cases := []struct {
		name      string
		directive string
	}{
		// The three spellings measured live in test/parse on 2026-09-06, each
		// dropped in silence by the chain this replaces.
		{"retired regex key", "expect=stdout:regex=ze [0-9]"},
		{"generic parser's stdout regex key, before it was read here", "expect=stdout:oops=ze"},
		{"engine directive that no parse-suite command can answer", "expect=output:contains=ze"},
		{"typo in the action", "exepct=stdout:contains=ze"},
		{"key that belongs to expect=file", "expect=stdout:not-contains=ze"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			test := discoverParseCI(t, dialectBaseCI+tc.directive+"\n")
			if test.ParseError == nil {
				t.Fatalf("directive %q parsed without error; it must be refused", tc.directive)
			}
			msg := test.ParseError.Error()
			if !strings.Contains(msg, tc.directive) {
				t.Errorf("error does not name the directive: %s", msg)
			}
			if !strings.Contains(msg, "dialect.ci") {
				t.Errorf("error does not name the file: %s", msg)
			}
			// The refusal lists what IS read, so the author can correct the
			// line without opening the parser.
			if !strings.Contains(msg, "expect=stdout:contains=") {
				t.Errorf("error does not list the accepted directives: %s", msg)
			}
		})
	}
}

// TestParseCIRefusesAssertionWithoutCommand covers the second silent drop in
// the same loop: an assertion that arrives before any cmd= had nothing to
// assert against and was dropped by an `if cur != nil` guard.
func TestParseCIRefusesAssertionWithoutCommand(t *testing.T) {
	test := discoverParseCI(t, "expect=stdout:contains=ze\n"+dialectBaseCI)
	if test.ParseError == nil {
		t.Fatal("an assertion before the first cmd= parsed without error")
	}
	if !strings.Contains(test.ParseError.Error(), "no cmd=") {
		t.Errorf("error does not say the assertion has no command: %v", test.ParseError)
	}
}

func TestParseCIAcceptsTheDialect(t *testing.T) {
	test := discoverParseCI(t, dialectBaseCI+strings.Join([]string{
		"expect=exit:code=0",
		"expect=stdout:contains=ze",
		"expect=stdout:pattern=ze [0-9]",
		"expect=stderr:contains=warning",
		"expect=stderr:pattern=warn.*",
		"reject=stdout:contains=panic",
		"reject=stdout:pattern=goroutine [0-9]+",
		"reject=stderr:pattern=fatal",
		"option=skip-os:value=plan9",
		"option=env:var=ZE_X:value=1",
	}, "\n")+"\n")
	if test.ParseError != nil {
		t.Fatalf("the dialect must parse: %v", test.ParseError)
	}
	if len(test.Commands) != 1 {
		t.Fatalf("parsed %d commands, want 1", len(test.Commands))
	}
	ci := test.Commands[0]
	// Each assertion reached its own slice. A directive that parsed into
	// nothing is the defect this file exists for, so count every one.
	if !ci.HasExitCode || ci.ExpectExitCode != 0 {
		t.Errorf("exit code not recorded: has=%v code=%d", ci.HasExitCode, ci.ExpectExitCode)
	}
	if len(ci.ExpectStdout) != 1 || len(ci.ExpectStdoutRe) != 1 {
		t.Errorf("stdout expectations: contains=%d pattern=%d, want 1 and 1", len(ci.ExpectStdout), len(ci.ExpectStdoutRe))
	}
	if len(ci.ExpectStderr) != 1 || len(ci.ExpectStderrRe) != 1 {
		t.Errorf("stderr expectations: contains=%d pattern=%d, want 1 and 1", len(ci.ExpectStderr), len(ci.ExpectStderrRe))
	}
	if len(ci.RejectStdout) != 1 || len(ci.RejectStdoutRe) != 1 || len(ci.RejectStderrRe) != 1 {
		t.Errorf("rejects: stdout=%d stdoutRe=%d stderrRe=%d, want 1 each",
			len(ci.RejectStdout), len(ci.RejectStdoutRe), len(ci.RejectStderrRe))
	}
	if len(test.EnvVars) != 1 || test.EnvVars[0] != "ZE_X=1" {
		t.Errorf("env vars %v, want [ZE_X=1]", test.EnvVars)
	}
}

// TestParseCIAssertionsDiscriminate runs each directive against output that
// satisfies it and output that does not. A directive that parses is not yet a
// directive that asserts: the migrated lines are only worth the migration if
// the checker they now reach can go red.
func TestParseCIAssertionsDiscriminate(t *testing.T) {
	cases := []struct {
		name      string
		directive string
		stdout    string
		stderr    string
		wantFail  bool
	}{
		{"stdout pattern matches", "expect=stdout:pattern=ze [0-9]+", "ze 12", "", false},
		{"stdout pattern does not match", "expect=stdout:pattern=ze [0-9]+", "ze dev", "", true},
		{"stdout contains", "expect=stdout:contains=valid", "configuration valid", "", false},
		{"stdout does not contain", "expect=stdout:contains=valid", "configuration invalid", "", false},
		{"stdout contains, absent", "expect=stdout:contains=valid", "nothing", "", true},
		{"stderr pattern matches", "expect=stderr:pattern=no shared secret", "", "error: no shared secret configured", false},
		{"stderr pattern does not match", "expect=stderr:pattern=no shared secret", "", "started", true},
		{"stderr pattern reads stderr alone", "expect=stderr:pattern=secret", "secret", "", true},
		{"reject stdout contains, absent", "reject=stdout:contains=$9$hash", "masked", "", false},
		{"reject stdout contains, present", "reject=stdout:contains=$9$hash", "password $9$hash", "", true},
		{"reject stdout pattern, absent", "reject=stdout:pattern=panic: .*", "clean", "", false},
		{"reject stdout pattern, present", "reject=stdout:pattern=panic: .*", "panic: nil map", "", true},
		{"reject stderr pattern, absent", "reject=stderr:pattern=fatal", "", "ok", false},
		{"reject stderr pattern, present", "reject=stderr:pattern=fatal", "", "fatal error", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			test := discoverParseCI(t, dialectBaseCI+tc.directive+"\n")
			if test.ParseError != nil {
				t.Fatalf("parse: %v", test.ParseError)
			}
			msg := checkExpectations(test.Commands[0], tc.stdout, tc.stderr)
			if tc.wantFail && msg == "" {
				t.Errorf("%q passed over stdout=%q stderr=%q; it asserts nothing", tc.directive, tc.stdout, tc.stderr)
			}
			if !tc.wantFail && msg != "" {
				t.Errorf("%q failed over stdout=%q stderr=%q: %s", tc.directive, tc.stdout, tc.stderr, msg)
			}
		})
	}
}

// TestParseCIRefusesEmptyPattern pins the generic parser's rule: an empty regex
// matches everything, so it is an assertion that cannot fail.
func TestParseCIRefusesEmptyPattern(t *testing.T) {
	for _, directive := range []string{
		"expect=stdout:pattern=",
		"expect=stderr:pattern=",
		"reject=stdout:pattern=",
		"reject=stderr:pattern=",
	} {
		t.Run(directive, func(t *testing.T) {
			test := discoverParseCI(t, dialectBaseCI+directive+"\n")
			if test.ParseError == nil {
				t.Fatalf("%s parsed; an empty regex asserts nothing", directive)
			}
		})
	}
}

// TestEveryParseCIFileParses reads the whole committed test/parse corpus with
// the parser that RUNS it. Without this, a dead directive is loud only when the
// suite is run, and the suite is not part of a `go test` pass.
func TestEveryParseCIFileParses(t *testing.T) {
	root := repoRootForTest(t)
	dir := filepath.Join(root, "test", "parse")

	ResetNickCounter()
	pt := NewParsingTests(root)
	if err := pt.Discover(dir); err != nil {
		t.Fatalf("discover %s: %v", dir, err)
	}
	registered := pt.Registered()
	if len(registered) == 0 {
		// Fatal, never a skip: a gate that vanishes with its input reads green
		// forever (ai/rules/evidence.md).
		t.Fatalf("no .ci files discovered under %s", dir)
	}
	for _, test := range registered {
		if test.ParseError != nil {
			t.Errorf("%s: %v", test.File, test.ParseError)
		}
	}
	t.Logf("parsed %d test/parse .ci files", len(registered))
}

// TestParseCICorpusReadsUnderTheGenericParser is the convergence gate. The
// parse suite has its own parser, and the accept-only ratchet walks the same
// files with the GENERIC one, so a spelling only one of them reads is a line
// whose meaning depends on who is reading. Keeping the dialect a subset of the
// generic vocabulary is what makes the two agree; this test is what keeps it
// one.
func TestParseCICorpusReadsUnderTheGenericParser(t *testing.T) {
	root := repoRootForTest(t)
	dir := filepath.Join(root, "test", "parse")

	entries, err := filepath.Glob(filepath.Join(dir, "*.ci"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("no .ci files under %s", dir)
	}
	for _, path := range entries {
		ResetNickCounter()
		if _, err := NewEncodingTests(dir).parseAndAdd(path); err != nil {
			t.Errorf("%s does not read under the generic parser: %v", filepath.Base(path), err)
		}
	}
	t.Logf("read %d test/parse .ci files under both parsers", len(entries))
}
