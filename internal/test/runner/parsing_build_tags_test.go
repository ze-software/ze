// Design: docs/architecture/testing/ci-format.md -- conditional questions about the actual DUT build
// Related: parsing.go -- discovery and native parse execution

package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const buildTagParseBody = "stdin=config:terminator=EOF_CONF\n" +
	"pppoe { ac-name proof; }\nEOF_CONF\n" +
	"cmd=foreground:seq=1:exec=ze config validate -:stdin=config\n" +
	"expect=exit:code=0\nexpect=stdout:contains=configuration valid\n"

// discoverBuildTagParseCI provides the same manifest and module ancestry that
// native discovery has. Neither a copied tag list nor this process's tags can
// answer the later question about the binary the runner actually selected.
func discoverBuildTagParseCI(t *testing.T, options string) (*ParsingTests, *parsingTest) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := "ze_l2tp internal/component/l2tp\nze_vpp internal/component/vpp\n"
	if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "test", "parse")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "build-condition.ci")
	if err := os.WriteFile(path, []byte(options+buildTagParseBody), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := NewParsingTests(root)
	if err := tests.Discover(dir); err != nil {
		t.Fatal(err)
	}
	if tests.Count() != 1 {
		t.Fatalf("discovered %d tests, want one", tests.Count())
	}
	return tests, tests.Registered()[0]
}

// TestParseCIBuildTagDeclarationRetainsAssertions proves the real discoverer
// accepts a present/absent conjunction without changing its command question.
// Current discovery must first fail this semantic assertion, not compilation.
func TestParseCIBuildTagDeclarationRetainsAssertions(t *testing.T) {
	_, test := discoverBuildTagParseCI(t,
		"option=needs-build-tag:value=ze_l2tp:state=present\n"+
			"option=needs-build-tag:value=ze_vpp:state=absent\n")
	if test.ParseError != nil {
		t.Fatalf("valid DUT build conditions failed discovery: %v", test.ParseError)
	}
	if len(test.Commands) != 1 {
		t.Fatalf("retained %d commands, want one", len(test.Commands))
	}
	command := test.Commands[0]
	if command.Exec != "ze config validate -" {
		t.Fatalf("command changed: %q", command.Exec)
	}
	if command.StdinName != "config" {
		t.Fatalf("stdin binding changed: %q", command.StdinName)
	}
	if !command.HasExitCode {
		t.Fatal("exit assertion was lost")
	}
	if command.ExpectExitCode != 0 {
		t.Fatalf("exit assertion changed: %d", command.ExpectExitCode)
	}
	if len(command.ExpectStdout) != 1 {
		t.Fatalf("stdout assertions changed: %v", command.ExpectStdout)
	}
	if command.ExpectStdout[0] != "configuration valid" {
		t.Fatalf("stdout assertion changed: %q", command.ExpectStdout[0])
	}
}

// TestParseCIBuildTagBadStateWinsOverHostSkip proves a valid host skip cannot
// hide a malformed condition. The refusal must identify the state grammar.
func TestParseCIBuildTagBadStateWinsOverHostSkip(t *testing.T) {
	_, test := discoverBuildTagParseCI(t,
		"option=skip-os:value="+runtime.GOOS+"\n"+
			"option=needs-build-tag:value=ze_vpp:state=unknown\n")
	if test.SkipReason == "" {
		t.Fatal("the host-skip control did not apply")
	}
	if test.ParseError == nil {
		t.Fatal("an unknown build condition state was accepted")
	}
	if !strings.Contains(test.ParseError.Error(), "state must be present or absent") {
		t.Fatalf("failure did not identify the invalid state: %v", test.ParseError)
	}
}

// TestParseCIBuildTagUnknownArtifactFails proves an unreadable selected DUT is
// not interpreted as a build without the requested tag. No child may execute.
func TestParseCIBuildTagUnknownArtifactFails(t *testing.T) {
	tests, test := discoverBuildTagParseCI(t,
		"option=needs-build-tag:value=ze_l2tp:state=present\n"+
			"option=needs-build-tag:value=ze_vpp:state=absent\n")
	if test.ParseError != nil {
		t.Fatalf("valid DUT build conditions failed discovery: %v", test.ParseError)
	}
	test.SetActive(true)
	binary := filepath.Join(tests.baseDir, "missing-dut")
	runner := NewParsingRunner(tests, tests.baseDir, binary)
	if runner.Run(t.Context(), false, true) {
		t.Fatal("unknown DUT metadata was accepted or skipped")
	}
	if test.Error == nil {
		t.Fatal("unknown DUT metadata lost its cause")
	}
	if !strings.Contains(test.Error.Error(), "DUT build tags") {
		t.Fatalf("failure did not identify the unreadable build metadata: %v", test.Error)
	}
	if !strings.Contains(test.Error.Error(), binary) {
		t.Fatalf("failure did not name the selected artifact: %v", test.Error)
	}
	if test.SkipReason != "" {
		t.Fatalf("unknown DUT metadata became a skip: %s", test.SkipReason)
	}
}
