// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/sessionpath"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

var (
	errNoConfigContentOrCommandsFound        = errors.New("no config content or commands found")
	errExpectedFailureButValidationSucceeded = errors.New("expected failure but validation succeeded")
)

// ciCommand holds a parsed cmd= line and its associated expectations.
//
// Every assertion here is scoped to ONE command: the cmd= line that precedes it
// in the file, checked against that command's own stdout and stderr. This is
// the parse suite's own scope and it differs from the generic runner, where a
// stream assertion is file-level over one combined buffer
// (docs/architecture/testing/ci-format.md).
type ciCommand struct {
	Seq       int
	Exec      string
	StdinName string
	// Timeout is the authored `cmd=...:timeout=<duration>`. It was parsed and
	// then dropped, so a command that does not exit ran until the whole test's
	// budget expired and reported that instead of its own deadline.
	Timeout string

	ExpectExitCode int
	HasExitCode    bool
	ExpectStdout   []string
	ExpectStdoutRe []*regexp.Regexp
	ExpectStderr   []string
	ExpectStderrRe []*regexp.Regexp
	RejectStdout   []string
	RejectStdoutRe []*regexp.Regexp
	RejectStderrRe []*regexp.Regexp
}

// parsingTest holds a single parsing test case.
type parsingTest struct {
	BaseTest // Embeds Name, Nick, Active, Error
	File     string

	// For .ci files: inline config content (nil for .conf files)
	InlineConfig []byte

	// Parsed commands from cmd= lines (nil for legacy .conf files)
	Commands []*ciCommand

	// Tmpfs files to materialize in the working directory. The parsed value is
	// kept whole, rather than flattened to path->content, because a file's MODE
	// is part of what the block declares: `mode=755`, and the executable
	// default a `.sh` path gets from defaultModeForPath. Flattening dropped
	// both, so `exec=./script.sh` in a parse test died with "permission
	// denied" while the same block worked in every other suite.
	Tmpfs *tmpfs.Tmpfs

	// Stdin blocks for piping into commands
	StdinBlocks map[string][]byte

	// Negative test support: if non-empty, expect validation to fail.
	// Each entry is a substring that must appear in stderr.
	// .expect files produce a single entry (optionally regex-prefixed).
	ExpectErrors []string
	ExpectRegex  *regexp.Regexp // Compiled regex when single ExpectErrors entry starts with "regex:"
	IsRegexMatch bool           // True if using regex matching (single-entry .expect files only)

	// Environment variables to set when running commands.
	EnvVars []string

	// SkipReason: when non-empty, the runner reports SKIP without
	// running the test. Set by option=skip-os:value=<list>
	// when the current GOOS is in the list.
	SkipReason string

	// ParseError marks a .ci file that could not be parsed at discovery time.
	// Discover records the file as a permanent failure and continues, so one
	// unparseable file fails loudly without aborting discovery of the rest of
	// the suite. The runner short-circuits such tests without executing them.
	ParseError error

	// Results
	Output string
}

// ParsingTests manages parsing test discovery and execution.
type ParsingTests struct {
	*TestSet[*parsingTest]
	baseDir string
}

// NewParsingTests creates a new parsing test manager.
func NewParsingTests(baseDir string) *ParsingTests {
	return &ParsingTests{
		TestSet: NewTestSet[*parsingTest](),
		baseDir: baseDir,
	}
}

// Discover finds parsing tests in the directory.
// Supports two formats:
//   - Legacy: valid/*.conf (positive) and invalid/*.conf + .expect (negative)
//   - Unified: *.ci files with stdin=, cmd:, expect: lines
func (pt *ParsingTests) Discover(dir string) error {
	ResetNickCounter()

	// First, try to discover .ci files (unified format)
	ciPattern := filepath.Join(dir, "*.ci")
	ciFiles, _ := filepath.Glob(ciPattern)
	slices.Sort(ciFiles)

	for _, ciFile := range ciFiles {
		// Skip-and-warn on a parse error rather than aborting discovery: one
		// unparseable .ci file must not hide every other test in the directory.
		// Aborting here is exactly what hid the whole test/ui suite -- discovery
		// returned an error, zero tests ran, and the suite read as green. The bad
		// file is still added as a permanent failure so it fails the suite loudly
		// instead of silently taking its siblings with it
		// (ai/rules/evidence.md). Mirrors EncodingTests.Discover.
		test, err := pt.parseCIFile(ciFile)
		if err != nil {
			recordLogger().Warn("unparseable .ci file recorded as failure; continuing discovery",
				"file", filepath.Base(ciFile), "error", err)
			if test == nil {
				// parseCIFile failed before a test existed (tmpfs read error), so
				// build a placeholder to keep the file visible in the suite.
				name := strings.TrimSuffix(filepath.Base(ciFile), ".ci")
				test = &parsingTest{
					Name: name,
					Nick: GenerateNick(name),
					File: ciFile,
				}
			}
			test.ParseError = fmt.Errorf("parse %s: %w", ciFile, err)
		}
		pt.Add(test)
	}

	// If .ci files found, we're done
	if pt.Count() > 0 {
		return nil
	}

	// Fall back to legacy format: valid/*.conf and invalid/*.conf

	// Discover positive tests (expect success) in valid/ subdirectory
	validDir := filepath.Join(dir, "valid")
	if _, err := os.Stat(validDir); err == nil {
		pattern := filepath.Join(validDir, "*.conf")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}

		slices.Sort(files)

		var tb textbuf.Buffer
		for _, confFile := range files {
			name := filepath.Base(confFile)
			nick := GenerateNick(name)

			test := &parsingTest{
				Name: tb.Reset().Str("valid/").Str(name).String(),
				Nick: nick,
				File: confFile,
			}
			pt.Add(test)
		}
	}

	// Discover negative tests (expect failure) in invalid/ subdirectory
	invalidDir := filepath.Join(dir, "invalid")
	if _, err := os.Stat(invalidDir); err == nil {
		invalidPattern := filepath.Join(invalidDir, "*.conf")
		invalidFiles, err := filepath.Glob(invalidPattern)
		if err != nil {
			return err
		}

		slices.Sort(invalidFiles)

		for _, confFile := range invalidFiles {
			// Same skip-and-warn-and-record contract as the .ci loop above. A
			// missing, empty, or bad-regex .expect file used to abort discovery of
			// the whole directory, so one broken negative fixture would hide every
			// other legacy test exactly as one bad .ci hid the test/ui suite.
			test, err := parseLegacyInvalidTest(confFile)
			if err != nil {
				recordLogger().Warn("unparseable legacy .conf test recorded as failure; continuing discovery",
					"file", filepath.Base(confFile), "error", err)
				test.ParseError = fmt.Errorf("parse %s: %w", confFile, err)
			}
			pt.Add(test)
		}
	}

	// Error if no tests found
	if pt.Count() == 0 {
		return fmt.Errorf("no parsing tests found in %s (expected *.ci or valid/*.conf)", dir)
	}

	return nil
}

// parseLegacyInvalidTest builds the negative-test record for one legacy
// invalid/*.conf fixture and its companion .expect file.
//
// Like parseCIFile it returns the partially-built test alongside any error, so
// the caller records the file as a failure instead of dropping it. The test is
// never nil: the record exists before the first thing that can fail, which is
// what lets a broken fixture still appear in the suite and fail loudly.
func parseLegacyInvalidTest(confFile string) (*parsingTest, error) {
	name := filepath.Base(confFile)

	var tb textbuf.Buffer
	test := &parsingTest{
		Name: tb.Str("invalid/").Str(name).String(),
		Nick: GenerateNick(name),
		File: confFile,
	}

	var tbE textbuf.Buffer
	expectFile := tbE.Str(confFile[:len(confFile)-len(".conf")]).Str(".expect").String()

	expectBytes, err := os.ReadFile(expectFile) //nolint:gosec // Test runner, path from glob
	if err != nil {
		return test, fmt.Errorf("negative test %s requires .expect file: %w", name, err)
	}
	expectError := strings.TrimSpace(string(expectBytes))
	if expectError == "" {
		return test, fmt.Errorf("negative test %s has empty .expect file", name)
	}
	test.ExpectErrors = []string{expectError}

	// Check for regex prefix
	const regexPrefix = "regex:"
	if strings.HasPrefix(expectError, regexPrefix) {
		pattern := strings.TrimSpace(expectError[len(regexPrefix):])
		re, compErr := regexp.Compile(pattern)
		if compErr != nil {
			return test, fmt.Errorf("negative test %s has invalid regex pattern: %w", name, compErr)
		}
		test.ExpectRegex = re
		test.IsRegexMatch = true
	}

	return test, nil
}

// parseCIFile parses a .ci file for parsing tests.
// Uses tmpfs.ReadFrom to handle stdin= and tmpfs= blocks, then parses
// cmd=, expect=, reject=, and option= directives from remaining lines.
//
// On error it returns the partially-built test alongside the error so the
// caller can mark it failed without generating a second nick (nicks are a
// monotone counter, so discarding one shifts every later test's id). The test
// is nil only when parsing failed before it was created.
func (pt *ParsingTests) parseCIFile(filePath string) (*parsingTest, error) {
	v, err := tmpfs.ReadFrom(filePath)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(filePath), ".ci")
	nick := GenerateNick(name)

	test := &parsingTest{
		Name: name,
		Nick: nick,
		File: filePath,
	}

	if len(v.Files) > 0 {
		test.Tmpfs = v
	}

	if len(v.StdinBlocks) > 0 {
		test.StdinBlocks = v.StdinBlocks
		if cfg, ok := v.StdinBlocks["config"]; ok {
			test.InlineConfig = cfg
		}
	}

	p := ciFileParser{test: test}
	for _, line := range v.OtherLines {
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if parseErr := p.line(trimmed); parseErr != nil {
			return test, fmt.Errorf("%s: %w", filepath.Base(filePath), parseErr)
		}
	}

	if test.InlineConfig == nil && test.Tmpfs == nil && len(test.Commands) == 0 {
		return test, errNoConfigContentOrCommandsFound
	}

	return test, nil
}

// ciFileParser carries the state one .ci file's directive lines build up: the
// test under construction and the cmd= an assertion attaches to.
type ciFileParser struct {
	test *parsingTest
	// cur is the cmd= line most recently read. Every per-command directive
	// asserts against it, so a directive that arrives before the first cmd= has
	// nothing to assert against and is refused rather than dropped.
	cur *ciCommand
}

// ciDirective binds one .ci directive prefix to the parser that reads its value.
type ciDirective struct {
	prefix string
	// needsCommand marks a directive whose value asserts against the preceding
	// cmd=. The dispatcher refuses such a directive when no cmd= has been read.
	needsCommand bool
	parse        func(p *ciFileParser, value string) error
}

// ciDirectives is the whole dialect the parse suite reads, in first-match order.
// It is the ONE declaration of that dialect: line dispatches through it and the
// unknown-directive refusal lists it, so no second enumeration can drift from
// what the parser does (ai/rules/principles.md). No entry may be a prefix of an
// entry below it.
//
// The spellings are the generic parser's (record_parse.go). Two dialect-only
// forms were deleted rather than aliased (ai/rules/no-layering.md). Both are
// what two parsers over one corpus costs:
//
//   - `expect=stdout:regex=` became `expect=stdout:pattern=`. The generic parser
//     reads `pattern=` and now REFUSES `regex=`, so every gate that walks the
//     corpus with it (the accept-only ratchet) could not read three test/parse
//     files at all.
//   - `expect=stdout:not:contains=` became `reject=stdout:contains=`, which this
//     parser already read with the identical meaning. This one was the dangerous
//     half: the generic parser splits `not:contains=` at the `:contains=` key
//     boundary and drops the bare `not`, so one written line meant "must be
//     absent" to the suite that runs test/parse and "must be present" to every
//     gate that reads it.
var ciDirectives = []ciDirective{
	{prefix: "cmd=", parse: (*ciFileParser).parseCommand},
	{prefix: "expect=exit:code=", needsCommand: true, parse: (*ciFileParser).parseExitCode},
	{prefix: "expect=stdout:contains=", needsCommand: true, parse: (*ciFileParser).parseExpectStdoutContains},
	{prefix: "expect=stdout:pattern=", needsCommand: true, parse: (*ciFileParser).parseExpectStdoutPattern},
	{prefix: "expect=stderr:contains=", parse: (*ciFileParser).parseExpectStderrContains},
	{prefix: "expect=stderr:pattern=", needsCommand: true, parse: (*ciFileParser).parseExpectStderrPattern},
	{prefix: "reject=stdout:contains=", needsCommand: true, parse: (*ciFileParser).parseRejectStdoutContains},
	{prefix: "reject=stdout:pattern=", needsCommand: true, parse: (*ciFileParser).parseRejectStdoutPattern},
	{prefix: "reject=stderr:pattern=", needsCommand: true, parse: (*ciFileParser).parseRejectStderrPattern},
	{prefix: "option=skip-os:value=", parse: (*ciFileParser).parseSkipOS},
	{prefix: "option=env:", parse: (*ciFileParser).parseEnv},
}

// line reads one directive line and refuses one no entry of ciDirectives reads.
//
// The chain that stood here had no default arm, so an unrecognized directive
// was dropped and the file still parsed. Twenty lines across nine test/parse
// files asserted nothing that way, and one of them was the only proof its test
// made. A parser that meets a directive it cannot answer says so
// (ai/rules/principles.md).
//
// The refusal quotes the whole line rather than a line number: OtherLines is
// what tmpfs.ReadFrom left after it consumed the stdin= and tmpfs= blocks, so
// its index is not the file's line number and quoting one would send the author
// to the wrong place.
func (p *ciFileParser) line(trimmed string) error {
	for _, d := range ciDirectives {
		after, ok := strings.CutPrefix(trimmed, d.prefix)
		if !ok {
			continue
		}
		if d.needsCommand && p.cur == nil {
			return fmt.Errorf("%q has no cmd= line before it to assert against", trimmed)
		}
		return d.parse(p, after)
	}

	var b textbuf.Buffer
	b.Str("unknown directive ").Quoted(trimmed).Str(" (the parse suite reads: ")
	for i, d := range ciDirectives {
		if i > 0 {
			b.Str(", ")
		}
		b.Str(d.prefix)
	}
	b.Byte(')')
	return errors.New(b.String())
}

func (p *ciFileParser) parseCommand(value string) error {
	rc, err := parseCmdExec("foreground", value)
	if err != nil {
		return err
	}
	p.cur = &ciCommand{Seq: rc.Seq, Exec: rc.Exec, StdinName: rc.Stdin, Timeout: rc.Timeout}
	p.test.Commands = append(p.test.Commands, p.cur)
	return nil
}

func (p *ciFileParser) parseExitCode(value string) error {
	code, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid exit code %q", value)
	}
	p.cur.ExpectExitCode = code
	p.cur.HasExitCode = true
	return nil
}

func (p *ciFileParser) parseExpectStdoutContains(value string) error {
	p.cur.ExpectStdout = append(p.cur.ExpectStdout, value)
	return nil
}

func (p *ciFileParser) parseExpectStdoutPattern(value string) error {
	re, err := compileCIPattern("expect=stdout:pattern", value)
	if err != nil {
		return err
	}
	p.cur.ExpectStdoutRe = append(p.cur.ExpectStdoutRe, re)
	return nil
}

// parseExpectStderrContains is the one directive that does not need a preceding
// cmd=. A .ci file holding an inline config and no command at all is a legacy
// negative test, and ExpectErrors is what runLegacyTest checks the validator's
// output against.
func (p *ciFileParser) parseExpectStderrContains(value string) error {
	if p.cur != nil {
		p.cur.ExpectStderr = append(p.cur.ExpectStderr, value)
	}
	p.test.ExpectErrors = append(p.test.ExpectErrors, value)
	return nil
}

func (p *ciFileParser) parseExpectStderrPattern(value string) error {
	re, err := compileCIPattern("expect=stderr:pattern", value)
	if err != nil {
		return err
	}
	p.cur.ExpectStderrRe = append(p.cur.ExpectStderrRe, re)
	return nil
}

func (p *ciFileParser) parseRejectStdoutContains(value string) error {
	p.cur.RejectStdout = append(p.cur.RejectStdout, value)
	return nil
}

func (p *ciFileParser) parseRejectStdoutPattern(value string) error {
	re, err := compileCIPattern("reject=stdout:pattern", value)
	if err != nil {
		return err
	}
	p.cur.RejectStdoutRe = append(p.cur.RejectStdoutRe, re)
	return nil
}

func (p *ciFileParser) parseRejectStderrPattern(value string) error {
	re, err := compileCIPattern("reject=stderr:pattern", value)
	if err != nil {
		return err
	}
	p.cur.RejectStderrRe = append(p.cur.RejectStderrRe, re)
	return nil
}

func (p *ciFileParser) parseSkipOS(value string) error {
	for skipOS := range strings.SplitSeq(value, ",") {
		if strings.TrimSpace(skipOS) != runtime.GOOS {
			continue
		}
		var b textbuf.Buffer
		p.test.SkipReason = b.Str("skip-os=").Str(value).Str(" (current GOOS=").Str(runtime.GOOS).Byte(')').String()
		return nil
	}
	return nil
}

func (p *ciFileParser) parseEnv(value string) error {
	var envVar, envVal string
	for field := range strings.SplitSeq(value, ":") {
		if v, ok := strings.CutPrefix(field, "var="); ok {
			envVar = v
		}
		if v, ok := strings.CutPrefix(field, "value="); ok {
			envVal = v
		}
	}
	if envVar == "" {
		return fmt.Errorf("option=env: needs var=<name>, got %q", value)
	}
	p.test.EnvVars = append(p.test.EnvVars, envVar+"="+envVal)
	return nil
}

// compileCIPattern compiles a directive's regex and refuses an empty one, which
// matches everything and so asserts nothing. Mirrors the generic parser
// (record_parse.go), so one spelling means one thing in both suites.
func compileCIPattern(directive, value string) (*regexp.Regexp, error) {
	if value == "" {
		return nil, fmt.Errorf("%s= must not be empty (an empty regex matches everything)", directive)
	}
	re, err := regexp.Compile(value)
	if err != nil {
		return nil, fmt.Errorf("invalid %s= %q: %w", directive, value, err)
	}
	return re, nil
}

// List prints available tests with type-specific formatting.
func (pt *ParsingTests) List() {
	writeTestListHeader("Available parsing tests")
	registered := pt.Registered()
	total := len(registered)
	for i, t := range registered {
		switch {
		case t.IsRegexMatch:
			writeTestListLine(i+1, total, t.Nick, t.Name, " (expect failure, regex)")
		case len(t.ExpectErrors) > 0:
			writeTestListLine(i+1, total, t.Nick, t.Name, " (expect failure)")
		default:
			writeTestListLine(i+1, total, t.Nick, t.Name, "")
		}
	}
	writeTestListFooter()
}

// parsingRunner executes parsing tests.
type parsingRunner struct {
	tests   *ParsingTests
	baseDir string
	zePath  string
	colors  *Colors
	// timeoutFactor widens an authored `cmd=...:timeout=` when this run
	// executes tests concurrently. Set by Run; 1 until then, so a caller that
	// never went through Run gets the authored value unchanged.
	timeoutFactor int
}

// NewParsingRunner creates a parsing test runner.
func NewParsingRunner(tests *ParsingTests, baseDir, zePath string) *parsingRunner {
	return &parsingRunner{
		tests:         tests,
		baseDir:       baseDir,
		zePath:        zePath,
		colors:        NewColors(),
		timeoutFactor: 1,
	}
}

// Run executes selected tests in parallel with real-time progress display.
func (r *parsingRunner) Run(ctx context.Context, verbose, quiet bool) bool {
	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Fprintln(os.Stdout, "No tests selected") //nolint:errcheck // user output
		return true
	}

	// Create parallel runner with generic type for direct test access
	runner := NewParallelRunner[*parsingTest](r.colors)
	runner.SetQuiet(quiet)
	runner.SetVerbose(verbose)
	runner.SetLabel("parse")
	runner.setNoHeader(true) // header managed by caller
	runner.SetBaseDir(r.baseDir)

	// An authored `timeout=` is measured on an uncontended run, so a budget set
	// near the uncontended time flakes when tests share CPU. Widen it by the
	// same ParallelTimeoutHeadroom the generic runner applies, and only when
	// this run really is concurrent: a single selected test keeps the authored
	// value, so a real slowdown still surfaces quickly.
	r.timeoutFactor = 1
	if min(runner.concurrencyLimit(), len(selected)) > 1 {
		r.timeoutFactor = ParallelTimeoutHeadroom
	}

	for _, test := range selected {
		rec := runner.AddTestWithNick(test.Name, test.Nick, test, func(runCtx context.Context, t *parsingTest) (bool, error) {
			success := r.runTest(runCtx, t)
			if !success {
				return false, t.Error
			}
			return true, nil
		})
		// Feed the Record the two fields ParallelRunner.Run short-circuits on.
		//
		// A file that failed to parse marks the Record ParseFailed and does NOT
		// propagate its SkipReason: that marker was parsed from the same broken
		// file, so honoring it would let a malformed skip-marked test report SKIP
		// (and pass) instead of failing. See the argument in parallel.go.
		if test.ParseError != nil {
			rec.ParseFailed = true
			rec.State = StateFail
			rec.Error = test.ParseError
			continue
		}
		// Propagate per-test SkipReason (from option=skip-os) onto the
		// Record so ParallelRunner.Run honors it without running the test.
		rec.SkipReason = test.SkipReason
	}

	runner.SetOnFail(func(test *parsingTest, _ error) {
		fmt.Fprintf(os.Stdout, "\n%s %s: %v\n", r.colors.Red("✗"), test.Name, test.Error) //nolint:errcheck // user output
		if test.File != "" {
			fmt.Fprintf(os.Stdout, "  %s %s\n", r.colors.Gray("File:"), test.File) //nolint:errcheck // user output
		}
	})

	return runner.Run(ctx)
}

// runTest executes a single parsing test.
// For .ci files with cmd= lines: executes each command in sequence with full
// expectation checking. For legacy .conf files: runs ze config validate.
func (r *parsingRunner) runTest(ctx context.Context, test *parsingTest) bool {
	// A .ci file that failed to parse at discovery has no runnable commands.
	// Report the parse error as a hard failure without attempting execution, so
	// one bad file fails the suite loudly rather than aborting discovery of every
	// other test (see ParsingTests.Discover).
	if test.ParseError != nil {
		test.Error = test.ParseError
		return false
	}
	if len(test.Commands) > 0 {
		return r.runCITest(ctx, test)
	}
	return r.runLegacyTest(ctx, test)
}

// runCITest executes a .ci test with full cmd=, tmpfs, stdin, expect, reject support.
func (r *parsingRunner) runCITest(ctx context.Context, test *parsingTest) bool {
	workDir, setupErr := r.setupWorkDir(test)
	if setupErr != nil {
		test.Error = setupErr
		return false
	}
	defer os.RemoveAll(workDir) //nolint:errcheck // test cleanup

	sort.Slice(test.Commands, func(i, j int) bool {
		return test.Commands[i].Seq < test.Commands[j].Seq
	})

	var allOutput strings.Builder
	for _, ci := range test.Commands {
		if !r.runOneCommand(ctx, test, ci, workDir, &allOutput) {
			return false
		}
	}

	test.Output = allOutput.String()
	return true
}

func (r *parsingRunner) setupWorkDir(test *parsingTest) (string, error) {
	workDir, mkErr := os.MkdirTemp(sessionpath.DefaultScratchRoot(), "ze-parse-ci-*")
	if mkErr != nil {
		return "", fmt.Errorf("create work dir: %w", mkErr)
	}

	// Materialize through the package that parsed the blocks. The copy that
	// stood here wrote every file 0o644, so a `mode=755` script -- and the
	// executable default a `.sh` path carries -- reached disk unexecutable, and
	// `exec=./script.sh` died with "permission denied" before the test ran.
	if test.Tmpfs != nil {
		if wErr := test.Tmpfs.WriteTo(workDir); wErr != nil {
			os.RemoveAll(workDir) //nolint:errcheck // cleanup on error
			return "", fmt.Errorf("write tmpfs: %w", wErr)
		}
	}

	for name, content := range test.StdinBlocks {
		stdinPath := filepath.Join(workDir, "stdin-"+name+".conf")
		if wErr := os.WriteFile(stdinPath, content, 0o644); wErr != nil { //nolint:gosec // Test runner
			os.RemoveAll(workDir) //nolint:errcheck // cleanup on error
			return "", fmt.Errorf("write stdin %s: %w", name, wErr)
		}
	}

	return workDir, nil
}

// runOneCommand executes a single cmd= and checks its expectations.
func (r *parsingRunner) runOneCommand(ctx context.Context, test *parsingTest, ci *ciCommand, workDir string, allOutput *strings.Builder) bool {
	cmdLine := ci.Exec
	if strings.HasPrefix(cmdLine, "ze ") {
		cmdLine = r.zePath + cmdLine[2:]
	} else if cmdLine == binNameZe {
		cmdLine = r.zePath
	}

	parts, splitErr := splitCommand(cmdLine)
	if splitErr != nil {
		test.Error = fmt.Errorf("seq %d: parse command %q: %w", ci.Seq, ci.Exec, splitErr)
		return false
	}

	// Replace "-" with the stdin file path. After replacement, containsDash
	// returns false, so the fallback pipe-stdin branch below is skipped
	// (stdin was already provided as a file argument).
	//
	// A `-` that IS the daemon's config argument needs the `start` verb in
	// front of the path: keyword-first grammar put the config path behind
	// `start` (spec-fixit-config-file-positional-grammar), so `ze <path>` is not
	// a command. Substituting in place produced exactly that argv, and the
	// daemon answered "unknown command: <path>" with its usage and exit 1
	// before it read one line of config. `test/parse/tacacs-key-required.ci` is
	// what that cost: its `expect=exit:code=1` was satisfied by the usage error,
	// so the file read as proof of a security guard that never ran. The generic
	// runner reads the same zeDaemonConfigArgIndex to insert the verb
	// (runner_exec.go); this one never got it.
	//
	// A `-` that is a SUBCOMMAND value (`ze config validate -`) is unaffected:
	// zeDaemonConfigArgIndex answers on the first non-flag token, so it returns
	// -1 for every quick-exit verb and the path is substituted in place.
	daemonCfgIdx := -1
	if parts[0] == r.zePath {
		daemonCfgIdx = zeDaemonConfigArgIndex(parts[1:])
	}
	for i, p := range parts {
		if p != "-" || ci.StdinName == "" {
			continue
		}
		stdinPath := filepath.Join(workDir, "stdin-"+ci.StdinName+".conf")
		if i == daemonCfgIdx+1 {
			parts = slices.Concat(parts[:i], []string{zeVerbStart, stdinPath}, parts[i+1:])
			break
		}
		parts[i] = stdinPath
	}

	// Honor the authored per-command deadline. Without it a command that does
	// not exit runs until the whole test's budget expires and reports that
	// instead of its own timeout, and a daemon launch whose config is accepted
	// never exits at all.
	if ci.Timeout != "" {
		d, tErr := time.ParseDuration(ci.Timeout)
		if tErr != nil {
			test.Error = fmt.Errorf("seq %d: invalid timeout %q: %w", ci.Seq, ci.Timeout, tErr)
			return false
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d*time.Duration(r.timeoutFactor))
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...) //nolint:gosec // Test runner
	cmd.Dir = workDir
	// Always set: childEnv adds GOTRACEBACK=all so a daemon that dies on a
	// runtime fault names the goroutine that faulted instead of just "fault".
	cmd.Env = childEnv(test.EnvVars...)

	if ci.StdinName != "" && !containsDash(parts) {
		if block, ok := test.StdinBlocks[ci.StdinName]; ok {
			cmd.Stdin = strings.NewReader(string(block))
		}
	}

	var outBuf, errBuf textbuf.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()

	stdout := outBuf.String()
	stderr := errBuf.String()
	allOutput.WriteString(stdout)
	allOutput.WriteString(stderr)

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](runErr); ok {
			exitCode = exitErr.ExitCode()
		} else {
			test.Error = fmt.Errorf("seq %d: command failed: %w", ci.Seq, runErr)
			return false
		}
	}

	if ci.HasExitCode && exitCode != ci.ExpectExitCode {
		test.Error = fmt.Errorf("seq %d: expected exit code %d, got %d\nstdout: %s\nstderr: %s",
			ci.Seq, ci.ExpectExitCode, exitCode, stdout, stderr)
		return false
	}

	if msg := checkExpectations(ci, stdout, stderr); msg != "" {
		test.Error = fmt.Errorf("seq %d: %s", ci.Seq, msg)
		return false
	}

	return true
}

// checkExpectations reports the first unmet assertion of one command, or "".
// Every assertion is scoped to this command alone (see ciCommand).
func checkExpectations(ci *ciCommand, stdout, stderr string) string {
	for _, expect := range ci.ExpectStdout {
		if !strings.Contains(stdout, expect) {
			return ciAssertFailure("stdout missing ", expect, "stdout", stdout)
		}
	}
	for _, re := range ci.ExpectStdoutRe {
		if !re.MatchString(stdout) {
			return ciAssertFailure("stdout does not match regex ", re.String(), "stdout", stdout)
		}
	}
	for _, expect := range ci.ExpectStderr {
		if !strings.Contains(stderr, expect) {
			return ciAssertFailure("stderr missing ", expect, "stderr", stderr)
		}
	}
	for _, re := range ci.ExpectStderrRe {
		if !re.MatchString(stderr) {
			return ciAssertFailure("stderr does not match regex ", re.String(), "stderr", stderr)
		}
	}
	for _, reject := range ci.RejectStdout {
		if strings.Contains(stdout, reject) {
			return ciAssertFailure("stdout must not contain ", reject, "stdout", stdout)
		}
	}
	for _, re := range ci.RejectStdoutRe {
		if re.MatchString(stdout) {
			return ciAssertFailure("stdout matches forbidden pattern ", re.String(), "stdout", stdout)
		}
	}
	for _, re := range ci.RejectStderrRe {
		if re.MatchString(stderr) {
			return ciAssertFailure("stderr matches forbidden pattern ", re.String(), "stderr", stderr)
		}
	}
	return ""
}

// ciAssertFailure builds one assertion-failure message: what was expected, the
// needle, and the whole stream the runner read, so the author sees why.
func ciAssertFailure(what, needle, stream, output string) string {
	var b textbuf.Buffer
	return b.Str(what).Quoted(needle).Str("\n").Str(stream).Str(": ").Str(output).String()
}

// runLegacyTest handles .conf files (valid/ and invalid/ directories).
func (r *parsingRunner) runLegacyTest(ctx context.Context, test *parsingTest) bool {
	configPath := test.File

	if test.InlineConfig != nil {
		tmpFile, writeErr := os.CreateTemp("", "ze-parse-test-*.conf")
		if writeErr != nil {
			test.Error = fmt.Errorf("create temp file: %w", writeErr)
			return false
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath) //nolint:errcheck // test cleanup

		if _, writeErr = tmpFile.Write(test.InlineConfig); writeErr != nil {
			tmpFile.Close() //nolint:errcheck // already failing
			test.Error = fmt.Errorf("write temp file: %w", writeErr)
			return false
		}
		if writeErr = tmpFile.Close(); writeErr != nil {
			test.Error = fmt.Errorf("close temp file: %w", writeErr)
			return false
		}
		configPath = tmpPath
	}

	isNegative := len(test.ExpectErrors) > 0
	var cmd *exec.Cmd
	if isNegative {
		cmd = exec.CommandContext(ctx, r.zePath, "config", "validate", configPath) //nolint:gosec // Test runner
	} else {
		cmd = exec.CommandContext(ctx, r.zePath, "config", "validate", "-q", configPath) //nolint:gosec // Test runner
	}
	output, runErr := cmd.CombinedOutput()
	test.Output = string(output)

	if isNegative {
		if runErr == nil {
			test.Error = errExpectedFailureButValidationSucceeded
			return false
		}
		if test.IsRegexMatch {
			if !test.ExpectRegex.MatchString(test.Output) {
				test.Error = fmt.Errorf("expected error matching regex %q, got: %s", test.ExpectRegex.String(), test.Output)
				return false
			}
		} else {
			var missing []string
			for _, expect := range test.ExpectErrors {
				if !strings.Contains(test.Output, expect) {
					missing = append(missing, expect)
				}
			}
			if len(missing) > 0 {
				test.Error = fmt.Errorf("expected error containing %v, got: %s", missing, test.Output)
				return false
			}
		}
		return true
	}

	if runErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](runErr); ok {
			test.Error = fmt.Errorf("validation failed with exit code %d", exitErr.ExitCode())
		} else {
			test.Error = fmt.Errorf("command failed: %w", runErr)
		}
		return false
	}

	return true
}

func containsDash(parts []string) bool {
	return slices.Contains(parts, "-")
}

// Build compiles ze for parsing tests.
func (r *parsingRunner) Build(ctx context.Context) error {
	// Use the provided zePath - assume it's already built
	if _, err := os.Stat(r.zePath); err != nil {
		return fmt.Errorf("ze binary not found at %s: %w", r.zePath, err)
	}
	return nil
}
