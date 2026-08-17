package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goodParseCI is a .ci file ParsingTests.parseCIFile accepts: an inline config
// plus one command, which is the minimum for a runnable parse test.
const goodParseCI = `stdin=config:terminator=EOF_CONF
bgp {
}
EOF_CONF

cmd=foreground:seq=1:exec=ze config validate -:stdin=config
expect=exit:code=0
`

// badParseCI carries an unterminated character class, so the expect=stdout:regex=
// branch fails regexp.Compile and parseCIFile returns an error.
const badParseCI = `stdin=config:terminator=EOF_CONF
bgp {
}
EOF_CONF

cmd=foreground:seq=1:exec=ze config validate -:stdin=config
expect=stdout:regex=[unterminated
`

// TestParsingDiscoverRecordsUnparseableFileAsFailure verifies that one
// unparseable .ci file in a parse-suite directory neither aborts discovery of
// its siblings nor disappears quietly: it is recorded as a test that FAILS.
//
// ParsingTests.Discover used to `return fmt.Errorf("parse %s: %w", ...)`, which
// aborted the whole directory. That is the exact failure mode that hid the
// entire test/ui suite for months: one bad .ci made the suite discover and run
// ZERO tests, and a suite that runs nothing reads as green.
//
// VALIDATES: Discover returns nil with a bad file present; the good file is
// discovered and fully parsed; the bad file is present with ParseError set, and
// the runner turns that into a hard failure rather than a vacuous pass.
// PREVENTS: one malformed .ci silently hiding an entire parse suite again.
func TestParsingDiscoverRecordsUnparseableFileAsFailure(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "good.ci"), []byte(goodParseCI), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.ci"), []byte(badParseCI), 0o600))

	pt := NewParsingTests(tmpDir)
	require.NoError(t, pt.Discover(tmpDir), "discovery must not abort on one unparseable .ci file")

	require.Equal(t, 2, pt.Count(), "both files must be present after discovery")

	var good, bad *parsingTest
	for _, test := range pt.Registered() {
		switch test.Name {
		case "good":
			good = test
		case "bad":
			bad = test
		}
	}

	require.NotNil(t, good, "good.ci must still be discovered alongside the bad file")
	require.NoError(t, good.ParseError, "good.ci must not be marked as a parse failure")
	assert.Len(t, good.Commands, 1, "good.ci cmd= line must be parsed")
	assert.NotNil(t, good.InlineConfig, "good.ci stdin=config block must be parsed")

	require.NotNil(t, bad, "bad.ci must still be recorded so it fails the suite")
	require.Error(t, bad.ParseError, "bad.ci must carry the parse error")
	assert.Contains(t, bad.ParseError.Error(), "bad.ci", "parse error must name the offending file")

	// Drive the failure through the runner entry point, not the parser alone:
	// a recorded ParseError is only a guard if execution turns it into a red.
	r := NewParsingRunner(pt, tmpDir, filepath.Join(tmpDir, "ze-does-not-exist"))
	assert.False(t, r.runTest(context.Background(), bad), "an unparseable .ci must FAIL, never pass vacuously")
	require.Error(t, bad.Error, "the runner must surface the parse error as the test's error")
	assert.Contains(t, bad.Error.Error(), "bad.ci")
}

// TestParsingDiscoverRecordsBrokenLegacyFixtureAsFailure covers the third and
// last abort path in the parse discoverer: the legacy `valid/`/`invalid/` .conf
// format, where a missing, empty, or bad-regex .expect file used to return out
// of Discover and abandon the whole directory.
//
// This layout has no instances in the tree today. That is exactly why the shape
// mattered: an unreachable abort becomes reachable the moment someone adds the
// directory, and the shape is the bug.
//
// VALIDATES: all three .expect failure modes are recorded as failing tests, the
// healthy fixtures in the same directory are still discovered, and the runner
// turns each recorded ParseError into a red.
// PREVENTS: one broken negative fixture hiding an entire legacy parse directory.
func TestParsingDiscoverRecordsBrokenLegacyFixtureAsFailure(t *testing.T) {
	tmpDir := t.TempDir()
	validDir := filepath.Join(tmpDir, "valid")
	invalidDir := filepath.Join(tmpDir, "invalid")
	require.NoError(t, os.MkdirAll(validDir, 0o750))
	require.NoError(t, os.MkdirAll(invalidDir, 0o750))

	// A healthy positive fixture, and a healthy negative fixture with its .expect.
	require.NoError(t, os.WriteFile(filepath.Join(validDir, "ok.conf"), []byte(minimalConfig), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "healthy.conf"), []byte("bogus {\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "healthy.expect"), []byte("unknown top-level keyword"), 0o600))

	// The three broken shapes. `noexpect` gets no .expect file at all.
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "noexpect.conf"), []byte("bogus {\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "empty.conf"), []byte("bogus {\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "empty.expect"), []byte("   \n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "badregex.conf"), []byte("bogus {\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "badregex.expect"), []byte("regex:[unterminated"), 0o600))

	pt := NewParsingTests(tmpDir)
	require.NoError(t, pt.Discover(tmpDir), "discovery must not abort on a broken legacy fixture")

	byName := make(map[string]*parsingTest, pt.Count())
	for _, test := range pt.Registered() {
		byName[test.Name] = test
	}
	require.Equal(t, 5, pt.Count(), "one valid + four invalid fixtures must all be present: %v", byName)

	require.NotNil(t, byName["valid/ok.conf"], "the healthy positive fixture must survive a broken sibling")
	require.NoError(t, byName["valid/ok.conf"].ParseError)

	healthy := byName["invalid/healthy.conf"]
	require.NotNil(t, healthy, "the healthy negative fixture must survive its broken siblings")
	require.NoError(t, healthy.ParseError)
	assert.Equal(t, []string{"unknown top-level keyword"}, healthy.ExpectErrors)

	r := NewParsingRunner(pt, tmpDir, filepath.Join(tmpDir, "ze-does-not-exist"))
	for name, wantMsg := range map[string]string{
		"invalid/noexpect.conf": "requires .expect file",
		"invalid/empty.conf":    "empty .expect file",
		"invalid/badregex.conf": "invalid regex pattern",
	} {
		bad := byName[name]
		require.NotNil(t, bad, "%s must be recorded so it fails the suite", name)
		require.Error(t, bad.ParseError, "%s must carry the parse error", name)
		assert.Contains(t, bad.ParseError.Error(), wantMsg,
			"%s error must name what is wrong with the fixture", name)

		assert.False(t, r.runTest(context.Background(), bad), "%s must FAIL, never pass vacuously", name)
		require.Error(t, bad.Error, "the runner must surface %s's parse error", name)
	}
}

// TestSkipMarkedMalformedCIStillFailsThroughParallelRunner drives the real entry
// point with BOTH ParseFailed and SkipReason set on the same record.
//
// This is the case the first version of this change got wrong. ParallelRunner's
// per-test goroutine short-circuited on SkipReason and emitted passed=true
// BEFORE calling t.Run, so Runner.runTest's own ParseFailed check was
// unreachable for any skip-marked file -- and 158 .ci files carry
// needs-linux/skip-os, 12 of them in test/ui, the suite this work exists to
// protect. On a darwin host they never run, so a malformed one reported SKIP and
// rotted invisibly: the exact outage, reintroduced by the fix for it.
//
// Driving runTest directly (as the earlier tests do) cannot see this class of
// bug, because runTest is not the entry point -- ai/rules/evidence.md
// "Test corollary": a unit test on the guard helper proves the helper, not that
// the caller ever reaches it with the input that matters.
//
// VALIDATES: a record carrying both markers ends FAIL, not SKIP, and the runner
// reports overall failure; a record carrying ONLY SkipReason still skips.
// PREVENTS: the skip short-circuit swallowing an unparseable file again.
func TestSkipMarkedMalformedCIStillFailsThroughParallelRunner(t *testing.T) {
	parseErr := errors.New("line 3: expect:bgp missing hex=")

	// Mirror the production wiring at runner.go: the .ci Runner owns its Records
	// and hands the scheduler an explicit Display (addRecord does not lazy-init
	// one, unlike addTestWithoutNick).
	colors := NewColorsWithOverride(false)
	tests := NewTests()
	display := NewDisplay(tests, colors)
	display.SetQuiet(true)

	pr := NewParallelRunner[*Record](colors)
	pr.setDisplay(display)
	pr.SetLabel("entry-point")
	pr.SetQuiet(true)

	const skipMarker = "needs-linux (run via make ze-qemu-needs-linux-test; current GOOS=darwin)"

	// The malformed file: needs-linux was parsed off an early line, then a later
	// line failed. Both markers are set, exactly as EncodingTests.Discover leaves
	// it on a darwin host.
	bad := tests.Add("bad-skip-marked")
	bad.Active = true
	bad.ParseFailed = true
	bad.State = StateFail
	bad.FailureType = failParseError
	bad.Error = parseErr
	bad.SkipReason = skipMarker

	// A healthy skip-marked file, to prove the fix does not turn every skip into
	// a failure.
	skipOnly := tests.Add("healthy-skip-marked")
	skipOnly.Active = true
	skipOnly.SkipReason = skipMarker

	var ran []string
	var mu sync.Mutex
	for _, rec := range []*Record{bad, skipOnly} {
		pr.addRecord(rec, rec, func(_ context.Context, r *Record) (bool, error) {
			mu.Lock()
			ran = append(ran, r.Name)
			mu.Unlock()
			return true, nil
		})
	}

	ok := pr.Run(context.Background())

	assert.False(t, ok, "a suite containing an unparseable file must report overall FAILURE")
	assert.Equal(t, StateFail, bad.State,
		"an unparseable file must FAIL even when it carries a skip marker: the marker came from the same broken file")
	assert.Equal(t, StateSkip, skipOnly.State,
		"a healthy skip-marked file must still SKIP")

	mu.Lock()
	defer mu.Unlock()
	assert.NotContains(t, ran, "bad-skip-marked",
		"the unparseable file must be reported without being executed")
	assert.NotContains(t, ran, "healthy-skip-marked",
		"the skip-marked file must not be executed either")
}

// TestRunTestChecksParseFailedBeforeSkipReason pins the same ordering inside
// Runner.runTest.
//
// ParallelRunner short-circuits before this function is reached, so this
// ordering governs only a direct caller -- but leaving the two guards
// inconsistent is how the next reader "tidies" the reorder away and reopens the
// hole for whoever adds a caller.
//
// VALIDATES: a record with both markers returns failure and State=StateFail.
// PREVENTS: restoring the SkipReason-first order in runTest.
func TestRunTestChecksParseFailedBeforeSkipReason(t *testing.T) {
	rec := newRecord("both-markers")
	rec.ParseFailed = true
	rec.State = StateFail
	rec.Error = errors.New("line 3: expect:bgp missing hex=")
	rec.SkipReason = "needs-linux (current GOOS=darwin)"

	r := &Runner{}
	assert.False(t, r.runTest(context.Background(), rec, &RunOptions{}),
		"an unparseable record must FAIL even when it also carries a skip marker")
	assert.Equal(t, StateFail, rec.State, "State must stay fail, not be overwritten with skip")
}

// TestSkipMarkedMalformedParseTestFailsThroughParallelRunner is the parse-suite
// half of the same hole: parsingRunner.Run copied test.SkipReason onto the
// Record unconditionally, so ParallelRunner short-circuited to SKIP and
// parsingRunner.runTest never saw ParseError.
//
// VALIDATES: a parse test whose .ci both declares option=skip-os and fails to
// parse is reported FAIL, driven through parsingRunner.Run.
// PREVENTS: reintroducing the skip short-circuit on the parse suite.
func TestSkipMarkedMalformedParseTestFailsThroughParallelRunner(t *testing.T) {
	tmpDir := t.TempDir()

	// option=skip-os is parsed off line 1 and matches this host; the regex on the
	// last line then fails to compile. Both markers end up set.
	badCI := "option=skip-os:value=" + runtime.GOOS + `
stdin=config:terminator=EOF_CONF
bgp {
}
EOF_CONF

cmd=foreground:seq=1:exec=ze config validate -:stdin=config
expect=stdout:regex=[unterminated
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.ci"), []byte(badCI), 0o600))

	pt := NewParsingTests(tmpDir)
	require.NoError(t, pt.Discover(tmpDir))
	require.Equal(t, 1, pt.Count())

	bad := pt.Registered()[0]
	require.Error(t, bad.ParseError, "the fixture must actually fail to parse")
	require.NotEmpty(t, bad.SkipReason, "the fixture must also carry a skip marker, or it does not test this hole")
	bad.SetActive(true)

	r := NewParsingRunner(pt, tmpDir, filepath.Join(tmpDir, "ze-does-not-exist"))
	assert.False(t, r.Run(context.Background(), false, true),
		"an unparseable parse test must FAIL through the runner even when it declares option=skip-os")
}

// goodDecodeCI uses the legacy decode= form, which needs no ze binary to parse.
const goodDecodeCI = `decode=update:family=ipv4/unicast:hex=DEADBEEF
expect=json:json={"type":"bgp"}
`

// badDecodeCI has neither a hex payload nor an expect=json line, so parseCIFile
// returns errMissingHexPayloadUseStdinPayload.
const badDecodeCI = "# no payload, no expectation\n"

// badDecodeTest is a .test file with fewer than the three required lines, so
// parseTestFile returns errMissingHexLine.
const badDecodeTest = "update ipv4/unicast\n"

// TestDecodingDiscoverRecordsUnparseableFileAsFailure verifies that a malformed
// decode test file is reported as a failure instead of vanishing.
//
// DecodingTests.Discover used to `continue` on a parse error under a bare
// "// Skip malformed test files" comment: the file dropped out of the suite with
// no warning and no failure record, so its coverage disappeared silently. A
// guard that neither denies nor speaks does not exist
// (ai/rules/evidence.md).
//
// Both discovery paths are covered: .ci (parseCIFile) and .test (parseTestFile).
//
// VALIDATES: every malformed file is still discovered, carries ParseError, and
// fails when run; the good file is unaffected.
// PREVENTS: silent coverage loss when a decode test file stops parsing.
func TestDecodingDiscoverRecordsUnparseableFileAsFailure(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "good.ci"), []byte(goodDecodeCI), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.ci"), []byte(badDecodeCI), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "truncated.test"), []byte(badDecodeTest), 0o600))

	dt := NewDecodingTests(tmpDir)
	require.NoError(t, dt.Discover(tmpDir), "discovery must not abort on malformed files")

	require.Equal(t, 3, dt.Count(), "every file must be present after discovery, malformed included")

	byName := make(map[string]*decodingTest, dt.Count())
	for _, test := range dt.Registered() {
		byName[test.Name] = test
	}

	good := byName["good"]
	require.NotNil(t, good, "good.ci must still be discovered")
	require.NoError(t, good.ParseError, "good.ci must not be marked as a parse failure")
	assert.Equal(t, "DEADBEEF", good.HexPayload, "good.ci payload must be parsed")

	r := NewDecodingRunner(dt, tmpDir, filepath.Join(tmpDir, "ze-does-not-exist"))
	for _, name := range []string{"bad", "truncated"} {
		bad := byName[name]
		require.NotNil(t, bad, "%s must be recorded so it fails the suite, not skipped", name)
		require.Error(t, bad.ParseError, "%s must carry the parse error", name)
		assert.Contains(t, bad.ParseError.Error(), name, "parse error must name the offending file")

		// Drive the failure through the runner entry point: a recorded
		// ParseError is only a guard if execution turns it into a red.
		assert.False(t, r.runTest(context.Background(), bad), "%s must FAIL, never pass vacuously", name)
		require.Error(t, bad.Error, "the runner must surface %s's parse error", name)
	}
}
