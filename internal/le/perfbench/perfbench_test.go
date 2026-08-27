// The nudge is advisory, so the failure it can have is SILENCE: a real
// data-plane change that produces no suggestion. Every case here is about that,
// which is why the git failure paths are driven rather than assumed.

package perfbench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeGit returns canned output for named queries and fails every unnamed query.
// A new query therefore fails a case instead of becoming a silent "nothing changed" result.
type fakeGit struct {
	answers map[string]string
	calls   []string
}

func (f *fakeGit) run(args ...string) (string, bool) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, key)
	out, known := f.answers[key]
	return out, known
}

// runnerWith answers a runner over a temporary root, driven by canned git.
func runnerWith(t *testing.T, answers map[string]string) (*Runner, *fakeGit) {
	t.Helper()
	git := &fakeGit{answers: answers}
	return &Runner{Root: t.TempDir(), Git: git.run}, git
}

func TestIsHotAcceptsOnlyMeasuredGoFiles(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"internal/component/bgp/reactor/peer.go", true},
		{"internal/component/bgp/plugins/rs/server_forward.go", true},
		{"internal/core/bgp/attribute/aspath.go", true},
		{"internal/perf/cli/run.go", true},
		{"internal/component/bgp/reactor/peer_test.go", true},
		{"internal/component/bgp/reactor/README.md", false},
		{"internal/component/cli/model.go", false},
		{"internal/component/bgp/plugins/rpki/rpki.go", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsHot(tc.path); got != tc.want {
			t.Errorf("IsHot(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestBaselinePrefersTheRecordedRunOverTheMergeBase verifies the priority order.
// The marker proves that the performance run covered its commit.
// The merge base only identifies a branch that has not had a performance test.
// Reversing them would measure work that the recorded run already covered.
func TestBaselinePrefersTheRecordedRunOverTheMergeBase(t *testing.T) {
	const recorded = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const merged = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	run, _ := runnerWith(t, map[string]string{
		"cat-file -e " + recorded + "^{commit}": "",
		"cat-file -e " + merged + "^{commit}":   "",
		"rev-parse --abbrev-ref @{upstream}":    "origin/main\n",
		"merge-base HEAD origin/main":           merged + "\n",
	})
	writeMarker(t, run, recorded)

	sha, origin := run.Baseline()
	if sha != recorded || origin != OriginLastRun {
		t.Errorf("Baseline answered %q from %q, want the recorded run", sha, origin)
	}
}

// TestBaselineFallsBackToTheMergeBase is the case the fallback exists for: a
// hot change went silent the moment it was committed, before any perf run.
func TestBaselineFallsBackToTheMergeBase(t *testing.T) {
	const merged = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	run, _ := runnerWith(t, map[string]string{
		"cat-file -e " + merged + "^{commit}": "",
		"rev-parse --abbrev-ref @{upstream}":  "origin/main\n",
		"merge-base HEAD origin/main":         merged + "\n",
	})

	sha, origin := run.Baseline()
	if sha != merged || origin != OriginMergeBase {
		t.Errorf("Baseline answered %q from %q, want the merge-base", sha, origin)
	}
}

// TestBaselineIgnoresAMarkerNoCommitAnswers is the stale-marker case. A SHA the
// checkout no longer holds would make every diff against it fail, so it is
// dropped rather than trusted.
func TestBaselineIgnoresAMarkerNoCommitAnswers(t *testing.T) {
	run, _ := runnerWith(t, map[string]string{
		"rev-parse --abbrev-ref @{upstream}": "",
	})
	writeMarker(t, run, "cccccccccccccccccccccccccccccccccccccccc")

	sha, origin := run.Baseline()
	if sha != "" || origin != OriginWorkingTree {
		t.Errorf("Baseline answered %q from %q, want no trusted point", sha, origin)
	}
}

// TestBaselineIgnoresAnUnknownMarker covers the value written when record cannot read HEAD.
// That value marks a run with an unreadable SHA, not a commit suitable for a diff.
func TestBaselineIgnoresAnUnknownMarker(t *testing.T) {
	// cat-file ANSWERS for the sentinel here, so the case turns on the
	// sentinel being rejected by name rather than on git failing to resolve
	// it. A reachability check alone would let "unknown" through the day a
	// branch or a tag happened to carry that name.
	run, _ := runnerWith(t, map[string]string{
		"cat-file -e " + unknownSHA + "^{commit}": "",
		"rev-parse --abbrev-ref @{upstream}":      "",
	})
	writeMarker(t, run, unknownSHA)

	if sha, origin := run.Baseline(); sha != "" || origin != OriginWorkingTree {
		t.Errorf("Baseline answered %q from %q on an %q marker", sha, origin, unknownSHA)
	}
}

// TestChangedHotReadsAllThreeWorkingTreeQueriesAndTheCommittedRange verifies the query count.
// Three queries return no files, so omitting one would not change the result.
// Only the call count exposes that omission.
func TestChangedHotReadsAllThreeWorkingTreeQueriesAndTheCommittedRange(t *testing.T) {
	const base = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	run, git := runnerWith(t, map[string]string{
		"diff --name-only":                     "internal/component/bgp/fsm/fsm.go\n",
		"diff --cached --name-only":            "internal/component/bgp/store/store.go\n",
		"ls-files --others --exclude-standard": "internal/perf/harness.go\n",
		"diff --name-only " + base + "..HEAD":  "internal/core/bgp/wire.go\ndocs/guide.md\n",
	})

	hot, err := run.ChangedHot(base)
	if err != nil {
		t.Fatalf("ChangedHot: %v", err)
	}
	want := []string{
		"internal/component/bgp/fsm/fsm.go",
		"internal/component/bgp/store/store.go",
		"internal/core/bgp/wire.go",
		"internal/perf/harness.go",
	}
	if !slices.Equal(hot, want) {
		t.Errorf("ChangedHot answered %v, want %v", hot, want)
	}
	if len(git.calls) != 4 {
		t.Errorf("ChangedHot made %d git calls: %v, want the three working-tree queries and the range", len(git.calls), git.calls)
	}
}

// TestChangedHotReportsAGitItCouldNotRun covers the former fail-open behavior.
//
// The script returned "" for both a failed command and an empty result.
// Missing git or a non-repository directory therefore silenced the advisory.
// The tool documentation identifies that silence as the critical advisory failure.
func TestChangedHotReportsAGitItCouldNotRun(t *testing.T) {
	run, _ := runnerWith(t, map[string]string{})

	hot, err := run.ChangedHot("")
	if err == nil {
		t.Fatalf("a git that answered nothing was read as a clean tree: %v", hot)
	}
	if !strings.Contains(err.Error(), "could not read") {
		t.Errorf("the error does not say what went wrong: %v", err)
	}

	report, code := run.Suggest()
	if code != 0 {
		t.Errorf("the nudge exited %d; it is advisory and may never block a build", code)
	}
	if report.Error == "" {
		t.Error("the report carries no error, so the operator is told nothing")
	}
	if text := report.Text(); !strings.Contains(text, "perf-suggest:") {
		t.Errorf("the rendering says nothing about the failure: %q", text)
	}
}

// TestRecordWritesTheHeadAndClearsTheSuggestion is the side effect this area
// has, read back from disk.
func TestRecordWritesTheHeadAndClearsTheSuggestion(t *testing.T) {
	const head = "dddddddddddddddddddddddddddddddddddddddd"
	run, _ := runnerWith(t, map[string]string{"rev-parse HEAD": head + "\n"})

	report, code := run.Record()
	if code != 0 {
		t.Fatalf("record exited %d", code)
	}
	if report.Recorded != head {
		t.Errorf("record answered %q, want the HEAD it wrote", report.Recorded)
	}
	body, err := os.ReadFile(filepath.Join(run.Root, filepath.FromSlash(MarkerPath)))
	if err != nil {
		t.Fatalf("reading the marker: %v", err)
	}
	if string(body) != head+"\n" {
		t.Errorf("the marker holds %q, want the SHA and a newline", body)
	}
}

// TestRecordWritesUnknownWhenHeadCannotBeRead keeps the two facts apart: perf
// ran here and the SHA was unreadable is not the same as perf never ran.
func TestRecordWritesUnknownWhenHeadCannotBeRead(t *testing.T) {
	run, _ := runnerWith(t, map[string]string{})

	report, code := run.Record()
	if code != 0 || report.Recorded != unknownSHA {
		t.Errorf("record answered %q with code %d, want %q", report.Recorded, code, unknownSHA)
	}
}

// TestTheNudgeNamesTwelveFilesAndCountsTheRest is the boundary the message has.
// One below names every file, one above starts counting.
func TestTheNudgeNamesTwelveFilesAndCountsTheRest(t *testing.T) {
	report := Report{Origin: OriginWorkingTree, Uncovered: hotPaths(namedFiles)}
	text := report.Text()
	if strings.Contains(text, "more") {
		t.Errorf("%d files were summarized rather than named:\n%s", namedFiles, text)
	}
	if got := strings.Count(text, "internal/perf/hot"); got != namedFiles {
		t.Errorf("the message names %d files, want %d", got, namedFiles)
	}

	report.Uncovered = hotPaths(namedFiles + 1)
	text = report.Text()
	if !strings.Contains(text, "... and 1 more") {
		t.Errorf("%d files did not produce a count of the rest:\n%s", namedFiles+1, text)
	}
	if got := strings.Count(text, "internal/perf/hot"); got != namedFiles {
		t.Errorf("the message names %d files past the bound, want %d", got, namedFiles)
	}
}

// TestTheNudgeIsSilentWithNothingUncovered is the quiet case, which is the one
// the tool sits in for most of a day.
func TestTheNudgeIsSilentWithNothingUncovered(t *testing.T) {
	if text := (Report{Origin: OriginWorkingTree}).Text(); text != "" {
		t.Errorf("nothing is uncovered and the nudge printed %q", text)
	}
}

// TestTheMessageNamesWhereTheBaselineCameFrom pins the three wordings. The same
// SHA means "perf ran here" from a marker and "never perf-tested" from a
// merge-base, and a reader acts differently on each.
func TestTheMessageNamesWhereTheBaselineCameFrom(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	cases := []struct {
		origin Origin
		want   string
	}{
		{OriginLastRun, "last perf run 0123456789ab"},
		{OriginMergeBase, "branch merge-base 0123456789ab (perf never recorded here)"},
		{OriginWorkingTree, "working tree (perf never recorded here)"},
	}
	for _, tc := range cases {
		report := Report{Baseline: sha, Origin: tc.origin, Uncovered: hotPaths(1)}
		if got := report.Text(); !strings.Contains(got, tc.want) {
			t.Errorf("%s: the message does not say %q:\n%s", tc.origin, tc.want, got)
		}
	}
}

// TestReportIsStructuredDataWithKebabCaseKeys is AC-7 for this area: the answer
// is data a pipe operator renders, never text the tool formatted itself.
func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(Report{
		Baseline: "abc", Origin: OriginLastRun, Uncovered: []string{"internal/perf/x.go"},
		Recorded: "abc", Error: "broken",
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, key := range []string{`"baseline"`, `"origin"`, `"uncovered"`, `"recorded"`, `"error"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}

// hotPaths answers n distinct hot-path file names.
func hotPaths(n int) []string {
	paths := make([]string, 0, n)
	for i := range n {
		paths = append(paths, "internal/perf/hot"+string(rune('a'+i))+".go")
	}
	return paths
}

// writeMarker records a SHA the way the record verb does.
func writeMarker(t *testing.T, run *Runner, sha string) {
	t.Helper()
	path := filepath.Join(run.Root, filepath.FromSlash(MarkerPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("marker directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(sha+"\n"), 0o600); err != nil {
		t.Fatalf("marker: %v", err)
	}
}
