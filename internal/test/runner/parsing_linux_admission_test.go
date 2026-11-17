// Design: docs/architecture/testing/ci-format.md -- host-default acceptance in the parse suite
// Related: parsing.go -- discovery and command assertions
// Related: record_parse_option.go -- shared Linux admission

package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const linuxHostDefaultConfig = "interface { ethernet access0 {} }\npppoe { interface access0 {} }\n"

const linuxHostDefaultCI = "option=needs-linux\nstdin=config:terminator=EOF_CONF\n" +
	linuxHostDefaultConfig + "EOF_CONF\n" +
	"cmd=foreground:seq=1:exec=ze config validate -:stdin=config\n" +
	"expect=exit:code=0\nexpect=stdout:contains=configuration valid\n"

// discoverLinuxHostDefaultCI gives the shared guest-route reader a real module
// root and draft path. It tests the path passed by discovery, not a placeholder.
func discoverLinuxHostDefaultCI(t *testing.T) (*ParsingTests, *parsingTest) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "test", "draft", "parse")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "host-default.ci")
	if err := os.WriteFile(file, []byte(linuxHostDefaultCI), 0o600); err != nil {
		t.Fatal(err)
	}
	pt := NewParsingTests(root)
	if err := pt.Discover(dir); err != nil {
		t.Fatal(err)
	}
	if pt.Count() != 1 {
		t.Fatalf("discovered %d tests, want one", pt.Count())
	}
	return pt, pt.Registered()[0]
}

// TestParseCINeedsLinuxRetainsCommandAssertions drives discovery with a Linux
// host-default question. Admission must retain its stdin, command and outcomes.
// MAIN must observe the existing parser refuse the marker before implementing it.
func TestParseCINeedsLinuxRetainsCommandAssertions(t *testing.T) {
	_, test := discoverLinuxHostDefaultCI(t)
	if test.ParseError != nil {
		t.Fatalf("Linux host-default question did not parse: %v", test.ParseError)
	}
	if string(test.InlineConfig) != linuxHostDefaultConfig {
		t.Fatalf("stdin changed: %q", test.InlineConfig)
	}
	if len(test.Commands) != 1 {
		t.Fatalf("parsed %d commands, want one", len(test.Commands))
	}
	command := test.Commands[0]
	if command.Exec != "ze config validate -" {
		t.Errorf("command changed: %q", command.Exec)
	}
	if command.StdinName != "config" {
		t.Errorf("stdin name changed: %q", command.StdinName)
	}
	if !command.HasExitCode {
		t.Fatal("the exit assertion was dropped")
	}
	if command.ExpectExitCode != 0 {
		t.Errorf("exit assertion changed: %d", command.ExpectExitCode)
	}
	if len(command.ExpectStdout) != 1 {
		t.Fatalf("stdout assertions: %v", command.ExpectStdout)
	}
	if command.ExpectStdout[0] != "configuration valid" {
		t.Errorf("stdout assertion changed: %q", command.ExpectStdout[0])
	}
}

// TestParseCINeedsLinuxUsesExactGuestRoute compares admission with its shared
// producer. On a non-Linux host, the reason must name this exact draft in QEMU.
func TestParseCINeedsLinuxUsesExactGuestRoute(t *testing.T) {
	_, test := discoverLinuxHostDefaultCI(t)
	if test.ParseError != nil {
		t.Fatal(test.ParseError)
	}
	shared := &Record{}
	if err := parseOptionNeedsLinux(shared, test.File, nil); err != nil {
		t.Fatal(err)
	}
	if test.SkipReason != shared.SkipReason {
		t.Fatalf("admission differs from the shared reader: got %q, want %q", test.SkipReason, shared.SkipReason)
	}
	if runtime.GOOS == "linux" {
		if test.SkipReason != "" {
			t.Fatalf("Linux question skipped on Linux: %s", test.SkipReason)
		}
		return
	}
	if !strings.Contains(test.SkipReason, "./le test qemu ci test test/draft/parse/host-default.ci") {
		t.Fatalf("skip reason lost the actual guest route: %s", test.SkipReason)
	}
	if strings.Contains(test.SkipReason, "<dir>") {
		t.Fatalf("skip reason contains a placeholder: %s", test.SkipReason)
	}
}

// TestParseCINeedsLinuxRefusesMalformedEvenWhenSkipped puts a matching skip
// first. Unknown keys, malformed boundaries and invalid capabilities still fail.
func TestParseCINeedsLinuxRefusesMalformedEvenWhenSkipped(t *testing.T) {
	for _, scenario := range []struct {
		name, directive, want string
	}{
		{"suffix", "option=needs-linuxish", "needs-linuxish"},
		{"unknown key", "option=needs-linux:bogus=true", "bogus"},
		{"empty capabilities", "option=needs-linux:caps=", "caps="},
		{"unknown capability", "option=needs-linux:caps=not-a-capability", "not-a-capability"},
		{"duplicate key", "option=needs-linux:caps=net-admin:caps=net-raw", "caps"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			content := "option=skip-os:value=" + runtime.GOOS + "\n" + scenario.directive + "\n" + dialectBaseCI
			test := discoverParseCI(t, content)
			if test.SkipReason == "" {
				t.Fatal("the control marker did not skip this host")
			}
			if test.ParseError == nil {
				t.Fatalf("malformed declaration was accepted: %s", scenario.directive)
			}
			if !strings.Contains(test.ParseError.Error(), scenario.want) {
				t.Fatalf("refusal lost the invalid input: %v", test.ParseError)
			}
		})
	}
}

// TestParseCINeedsLinuxParseErrorWinsThroughRunner proves a later parse error
// cannot turn into a skipped success. No executable is needed for this failure.
func TestParseCINeedsLinuxParseErrorWinsThroughRunner(t *testing.T) {
	pt, test := discoverLinuxHostDefaultCI(t)
	content := "option=skip-os:value=" + runtime.GOOS + "\n" + linuxHostDefaultCI +
		"expect=stdout:pattern=[unterminated\n"
	if err := os.WriteFile(test.File, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	pt = NewParsingTests(pt.baseDir)
	if err := pt.Discover(filepath.Dir(test.File)); err != nil {
		t.Fatal(err)
	}
	if pt.Count() != 1 {
		t.Fatalf("discovered %d tests, want one", pt.Count())
	}
	broken := pt.Registered()[0]
	if broken.SkipReason == "" {
		t.Fatal("the control marker did not skip this host")
	}
	if broken.ParseError == nil {
		t.Fatal("the malformed file did not fail discovery")
	}
	if !strings.Contains(broken.ParseError.Error(), "invalid expect=stdout:pattern=") {
		t.Fatalf("the file failed before its malformed assertion: %v", broken.ParseError)
	}
	broken.SetActive(true)
	runner := NewParsingRunner(pt, pt.baseDir, filepath.Join(pt.baseDir, "ze-must-not-run"))
	if runner.Run(context.Background(), false, true) {
		t.Fatal("the malformed Linux question passed through the skip branch")
	}
}
