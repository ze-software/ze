// Design: docs/architecture/testing/test-health.md -- the checker proves itself first
// Related: testweakened.go -- every fixture enters through the public Check function.
package testweakened

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const fixtureLedgerHeader = "| Test | Reason |\n|------|--------|\n"

// selfTestSession is the commit namespace the fixture repository writes its
// ledger shard under. Every Check call below names it, so the fixture proves
// the same path derivation the live gate uses rather than a fixed file name.
const selfTestSession = "5e1f7e57"

type detectorFixture struct {
	path   string
	old    string
	new    string
	detail string
	added  bool
}

// SelfTestReport is the structured proof population and every failed contract.
type SelfTestReport struct {
	Positive int      `json:"positive"`
	Negative int      `json:"negative"`
	Checks   int      `json:"checks"`
	Failures []string `json:"failures,omitempty"`
}

// ExitCode answers one when any fixture or fail-closed contract did not fire.
func (r SelfTestReport) ExitCode() int {
	if len(r.Failures) != 0 {
		return 1
	}
	return 0
}

// Text preserves the producer's selftest verdict while the structured form
// records the population that earned it.
func (r SelfTestReport) Text() string {
	var page textbuf.Buffer
	page.SetColor(slogutil.UseColor(os.Stdout))
	color := textbuf.C
	if len(r.Failures) == 0 {
		return page.Str("SELFTEST PASS\n").String()
	}
	page.Colored(color.BoldRed).Str("SELFTEST FAIL").Colored(color.Reset).Byte('\n')
	for _, failure := range r.Failures {
		page.Str("  - ").Str(failure).Byte('\n')
	}
	return page.String()
}

// SelfTest drives every detector verdict, clean negative fixtures, pairing,
// malformed and absent ledgers, and an unresolved baseline through Check.
func SelfTest() SelfTestReport {
	positive := positiveFixtures()
	negative := negativeFixtures()
	report := SelfTestReport{Positive: len(positive), Negative: len(negative)}
	var text textbuf.Buffer
	check := func(condition bool, failure string) {
		report.Checks++
		if !condition {
			report.Failures = append(report.Failures, failure)
		}
	}

	selfTestShard := ShardPath(WeakenedDir, selfTestSession)
	root, err := os.MkdirTemp("", "ze-weakened-selftest-")
	if err != nil {
		check(false, text.Str("create fixture repository: ").Err(err).String())
		return report
	}
	defer func() { _ = os.RemoveAll(root) }()
	check(runSelfTestGit(root, "init", "-q"), "initialize fixture repository")
	check(writeSelfTestFile(root, selfTestShard, fixtureLedgerHeader) == nil,
		"write the fixture ledger")
	for _, fixture := range appendCopyFixtures(positive, negative) {
		if fixture.added {
			continue
		}
		check(writeSelfTestFile(root, fixture.path, fixture.old) == nil,
			text.Reset().Str("write baseline ").Str(fixture.path).String())
	}
	check(runSelfTestGit(root, "add", "-A"), "stage fixture baseline")
	check(runSelfTestGit(root,
		"-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "baseline"),
		"commit fixture baseline")

	paths := make([]string, 0, len(positive)+len(negative))
	for _, fixture := range appendCopyFixtures(positive, negative) {
		check(writeSelfTestFile(root, fixture.path, fixture.new) == nil,
			text.Reset().Str("write worktree ").Str(fixture.path).String())
		paths = append(paths, fixture.path)
	}
	missing := Check(Request{Root: root, Session: selfTestSession, Paths: paths})
	check(missing.ExitCode() == 1, "unrecorded weakenings must be refused")
	for _, fixture := range positive {
		finding, found := findingForPath(missing.Findings, fixture.path)
		check(found, text.Reset().Str(fixture.path).Str(" must produce a weakening").String())
		check(found && containsDetail(finding.Details, fixture.detail),
			text.Reset().Str(fixture.path).Str(" must report ").Str(fixture.detail).String())
	}
	for _, fixture := range negative {
		_, found := findingForPath(missing.Findings, fixture.path)
		check(!found,
			text.Reset().Str(fixture.path).Str(" must stay outside the positive population").String())
	}
	check(len(missing.Findings) >= len(positive),
		"finding population must cover every positive fixture")

	text.Reset().Str(fixtureLedgerHeader)
	for _, finding := range missing.Findings {
		text.Str("| ").Str(finding.Name).Str(" | fixture accepts this weakening |\n")
	}
	check(writeSelfTestFile(root, selfTestShard, text.String()) == nil,
		"write accepted fixture ledger")
	accepted := Check(Request{Root: root, Session: selfTestSession, Paths: paths})
	check(accepted.ExitCode() == 0, "one matching row for every weakening must be accepted")
	check(len(accepted.Findings) == len(missing.Findings),
		"accepted check must judge the same finding population")

	check(writeSelfTestFile(root, selfTestShard,
		text.Reset().Str(fixtureLedgerHeader).Str("| TestGone | stale |\n").String()) == nil,
		"write stale fixture ledger")
	stale := Check(Request{Root: root, Session: selfTestSession, Paths: paths})
	check(stale.ExitCode() == 1 && containsProblem(stale.Problems, "does not weaken"),
		"a stale row must be refused")

	check(writeSelfTestFile(root, selfTestShard, "# Tests\n\nno table\n") == nil,
		"write malformed fixture ledger")
	malformed := Check(Request{Root: root, Session: selfTestSession})
	check(malformed.ExitCode() == 1 && containsProblem(malformed.Problems, "has no `| Test | Reason |`"),
		"a malformed ledger must fail closed")

	check(os.Remove(filepath.Join(root, filepath.FromSlash(selfTestShard))) == nil,
		"remove fixture ledger")
	// A session that holds no shard holds no rows, which is the state a
	// checkout is in between commits and is not a problem of its own. The
	// fail-closed contract sits where the absence decides something: a commit
	// that weakens a test and has no shard is refused, and the refusal carries
	// the header and the row to write.
	absent := Check(Request{Root: root, Session: selfTestSession})
	check(absent.ExitCode() == 0 && len(absent.Problems) == 0,
		"an absent shard must read as a session holding no rows")
	absentWithWeakening := Check(Request{Root: root, Session: selfTestSession, Paths: paths})
	check(absentWithWeakening.ExitCode() == 1 &&
		containsProblem(absentWithWeakening.Problems, "does not exist, and this commit weakens"),
		"an absent shard must refuse a commit that weakens a test")
	check(containsProblem(absentWithWeakening.Problems, "| Test | Reason |"),
		"the absent-shard refusal must carry the header an author writes first")

	cleanWithoutLedger := Check(Request{Root: root, Session: selfTestSession, Paths: []string{negative[0].path}})
	check(cleanWithoutLedger.ExitCode() == 0,
		"a change that weakens nothing must not read an absent ledger")

	invalidAnchor := Check(Request{
		Root: root, Paths: []string{positive[0].path}, Anchor: "NO-SUCH-ANCHOR",
	})
	check(invalidAnchor.ExitCode() == 2 &&
		containsProblem(invalidAnchor.Problems, "does not resolve to a commit"),
		"an unresolved baseline must report cannot-run")
	return report
}

func positiveFixtures() []detectorFixture {
	return []detectorFixture{
		{
			path:   "pkg/empty_test.go",
			old:    "package a\nfunc TestEmpty(t *testing.T) { t.Fatal(\"x\") }\n",
			new:    "",
			detail: "replacing test content with empty string",
		},
		{
			path:   "pkg/delete_test.go",
			old:    "package a\nfunc TestDeleted(t *testing.T) { t.Error(\"x\") }\n",
			new:    "package a\n",
			detail: "deleting Test/Fuzz/Benchmark function",
		},
		{
			path: "pkg/run_test.go",
			old: "package a\nfunc TestRuns(t *testing.T) {\n" +
				"\tt.Run(\"a\", func(t *testing.T) {})\n\tt.Run(\"b\", func(t *testing.T) {})\n}\n",
			new: "package a\nfunc TestRuns(t *testing.T) {\n" +
				"\tt.Run(\"a\", func(t *testing.T) {})\n}\n",
			detail: "removing t.Run cases",
		},
		{
			path: "pkg/table_test.go",
			old: "package a\nfunc TestTable(t *testing.T) {\n\tcases := []struct{Name string}{\n" +
				"\t\t{Name: \"a\"},\n\t\t{Name: \"b\"},\n\t}\n\t_ = cases\n}\n",
			new: "package a\nfunc TestTable(t *testing.T) {\n\tcases := []struct{Name string}{\n" +
				"\t\t{Name: \"a\"},\n\t}\n\t_ = cases\n}\n",
			detail: "removing table-driven cases",
		},
		{
			path: "pkg/assertion_test.go",
			old: "package a\nfunc TestAssertions(t *testing.T) {\n" +
				"\trequire.Equal(t, 1, got)\n\trequire.NoError(t, err)\n}\n",
			new: "package a\nfunc TestAssertions(t *testing.T) {\n" +
				"\trequire.Equal(t, 1, got)\n}\n",
			detail: "removing assertions",
		},
		{
			path:   "pkg/fatal_test.go",
			old:    "package a\nfunc TestFatal(t *testing.T) { require.Equal(t, 1, got) }\n",
			new:    "package a\nfunc TestFatal(t *testing.T) { assert.Equal(t, 1, got) }\n",
			detail: "downgrading fatal assertions to non-fatal",
		},
		{
			path: "pkg/skip_test.go",
			old:  "package a\nfunc TestSkip(t *testing.T) { require.Equal(t, 1, got) }\n",
			new: "package a\nfunc TestSkip(t *testing.T) {\n" +
				"\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n",
			detail: "adding t.Skip",
		},
		{
			path: "pkg/ignore_test.go",
			old:  "package a\nfunc TestIgnore(t *testing.T) { require.Equal(t, 1, got) }\n",
			new: "//go:build ignore\n\npackage a\n" +
				"func TestIgnore(t *testing.T) { require.Equal(t, 1, got) }\n",
			detail: "adding 'ignore' build tag",
		},
		{
			path:   "pkg/comment_test.go",
			old:    "package a\nfunc TestComment(t *testing.T) { require.Equal(t, 1, got) }\n",
			new:    "package a\nfunc TestComment(t *testing.T) { // require.Equal(t, 1, got)\n}\n",
			detail: "commenting out assertions",
		},
		{
			path:   "test/weakened-fixtures/expect.ci",
			old:    "cmd=ze show\nexpect=out:text=one\nexpect=out:text=two\n",
			new:    "cmd=ze show\nexpect=out:text=one\n",
			detail: "removing expectations",
		},
		{
			path:   "test/weakened-fixtures/reject.et",
			old:    "reject=out:text=error\n",
			new:    "expect=out:text=error\n",
			detail: "removing negative expectations",
		},
		{
			path:   "test/weakened-fixtures/needle.ci",
			old:    "expect=out:text=ready\n",
			new:    "expect=out:text=\n",
			detail: "emptying an expectation's needle",
		},
		{
			path:   "test/fixtures/delete_python_test.py",
			old:    "def test_delete():\n    assert value\n",
			new:    "helper = 1\n",
			detail: "deleting every def test_ function",
		},
		{
			path: "test/fixtures/skip_python_test.py",
			old:  "def test_skip():\n    assert value\n",
			new: "def test_skip():\n    pytest.skip(\"later\")\n" +
				"    assert value\n",
			detail: "adding a Python skip",
		},
		{
			path:   "test/fixtures/assert_python_test.py",
			old:    "def test_assertion():\n    assert first\n    assert second\n",
			new:    "def test_assertion():\n    assert first\n",
			detail: "removing assertions",
		},
		{
			path:   "pkg/tautology_test.go",
			old:    "package a\nfunc TestTautology(t *testing.T) { assert.True(t, condition) }\n",
			new:    "package a\nfunc TestTautology(t *testing.T) { assert.True(t, true) }\n",
			detail: "introducing an assertion that cannot fail",
		},
	}
}

func negativeFixtures() []detectorFixture {
	return []detectorFixture{
		{
			path: "test/fixtures/string_fixture_test.py",
			old:  "def test_fixture():\n    assert value\n",
			new: "def test_fixture():\n    fixture = \"pytest.skip('later')\\nassert True\"\n" +
				"    assert value\n",
		},
		{
			path: "pkg/comment_fixture_test.go",
			old:  "package a\nfunc TestCommentFixture(t *testing.T) { require.Equal(t, 1, got) }\n",
			new: "package a\nfunc TestCommentFixture(t *testing.T) {\n" +
				"\turl := \"https://example.test//require.NoError\"\n" +
				"\trequire.Equal(t, 1, got)\n\t_ = url\n}\n",
		},
		{
			path:  "pkg/added_test.go",
			new:   "package a\nfunc TestAdded(t *testing.T) { t.Fatal(\"new\") }\n",
			added: true,
		},
	}
}

func appendCopyFixtures(first, second []detectorFixture) []detectorFixture {
	return slices.Concat(first, second)
}

func writeSelfTestFile(root, path, content string) error {
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o600)
}

func runSelfTestGit(root string, args ...string) bool {
	_, _, code, started := gitCapture(root, args...)
	return started && code == 0
}

func findingForPath(findings []Finding, path string) (Finding, bool) {
	for _, finding := range findings {
		if finding.Path == path {
			return finding, true
		}
	}
	return Finding{}, false
}

func containsDetail(details []string, expected string) bool {
	for _, detail := range details {
		if strings.Contains(detail, expected) {
			return true
		}
	}
	return false
}

func containsProblem(problems []string, expected string) bool {
	for _, problem := range problems {
		if strings.Contains(problem, expected) {
			return true
		}
	}
	return false
}
