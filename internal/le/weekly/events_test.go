// Goal: prove `le weekly report from <a> to <b>` reaches every registered
// source through the action table and answers one section per source, in name
// order, through the shared renderer.
// Method: a fixture repository with three commits stands in for the checkout,
// both endpoints are exported for real (export.Commit) into a temporary scratch
// directory, and two stub sources registered out of name order record what
// they were handed and read the exported trees.
//
// VALIDATES: AC-1 of spec-weekly-event-report: both full commits, the
//            first-parent commit list, one section per source in a stable
//            order, each endpoint's own tree, and `| json`, `| yaml`, `| table`
//            all answering it.
// PREVENTS: a source registered and never asked, a section order that follows
//           import order, and a report only the bare line can render.

package weekly

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func TestWeeklyReportAnswersEverySection(t *testing.T) {
	repo, commits := reportFixtureRepo(t)
	from, to := commits[0], commits[2]

	scratch := withReportScratch(t)
	seen := map[string]seenRange{}
	withEventSources(t,
		recordingSource("zeta", seen),
		recordingSource("alpha", seen),
	)
	reportCheckout = func() (string, error) { return repo, nil }

	var out, errOut strings.Builder
	code := leroot.Run("weekly", leroot.LookupCommand("weekly"),
		[]string{"report", "from", from, "to", to, "|", "json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("le weekly report | json exited %d: %s", code, errOut.String())
	}

	var decoded struct {
		From     Endpoint `json:"from"`
		To       Endpoint `json:"to"`
		Commits  int      `json:"commits"`
		RunDir   string   `json:"run-dir"`
		Sections []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Facts  []Fact `json:"facts"`
		} `json:"sections"`
	}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("the answer is not JSON: %v\n%s", err, out.String())
	}
	if decoded.From.Commit != from || decoded.To.Commit != to {
		t.Errorf("endpoints = %s..%s, want %s..%s", decoded.From.Commit, decoded.To.Commit, from, to)
	}
	if decoded.Commits != 2 {
		t.Errorf("commits = %d, want the 2 first-parent commits after from", decoded.Commits)
	}
	names := make([]string, 0, len(decoded.Sections))
	for _, section := range decoded.Sections {
		names = append(names, section.Name)
		if section.Status != "answered" {
			t.Errorf("section %s status = %q, want answered", section.Name, section.Status)
		}
		if len(section.Facts) != 1 || section.Facts[0].Value != section.Name {
			t.Errorf("section %s facts = %+v, want the one fact its source answered", section.Name, section.Facts)
		}
	}
	if !slices.Equal(names, []string{"alpha", "zeta"}) {
		t.Errorf("sections = %v, want [alpha zeta]: name order, not registration order", names)
	}

	for _, name := range []string{"alpha", "zeta"} {
		seenBy, asked := seen[name]
		if !asked {
			t.Errorf("source %s was never asked", name)
			continue
		}
		got := seenBy.events
		if got.Repo != repo || got.FromCommit != from || got.ToCommit != to {
			t.Errorf("source %s got repo %s range %s..%s", name, got.Repo, got.FromCommit, got.ToCommit)
		}
		if got.FromDir != filepath.Join(decoded.RunDir, "from") || got.ToDir != filepath.Join(decoded.RunDir, "to") {
			t.Errorf("source %s got trees %s and %s, want from and to under the run directory %s",
				name, got.FromDir, got.ToDir, decoded.RunDir)
		}
		if got.WorkDir != decoded.RunDir {
			t.Errorf("source %s got work directory %s, want the run directory %s", name, got.WorkDir, decoded.RunDir)
		}
		if seenBy.fromText != "one\n" || seenBy.toText != "three\n" {
			t.Errorf("source %s read week.txt %q at from and %q at to, want each endpoint's own tree",
				name, seenBy.fromText, seenBy.toText)
		}
		if !slices.Equal(got.Commits, commits[1:]) {
			t.Errorf("source %s got commits %v, want %v oldest first", name, got.Commits, commits[1:])
		}
	}
	if filepath.Dir(decoded.RunDir) != scratch {
		t.Errorf("run directory %s is not under the report scratch %s", decoded.RunDir, scratch)
	}
	assertRunsRemoved(t, scratch)

	for _, pipe := range []string{"yaml", "table"} {
		out.Reset()
		errOut.Reset()
		code := leroot.Run("weekly", leroot.LookupCommand("weekly"),
			[]string{"report", "from", from, "to", to, "|", pipe}, &out, &errOut)
		if code != 0 {
			t.Errorf("le weekly report | %s exited %d: %s", pipe, code, errOut.String())
			continue
		}
		if !strings.Contains(out.String(), "alpha") || !strings.Contains(out.String(), "zeta") {
			t.Errorf("le weekly report | %s lost a section:\n%s", pipe, out.String())
		}
	}
}

// seenRange is what one recording source was handed, and the fixture file it
// read from each exported tree while the run was live.
type seenRange struct {
	events   EventRange
	fromText string
	toText   string
}

// recordingSource answers one fact named after its section and records the
// range it was handed, with week.txt as each exported tree holds it.
func recordingSource(section string, seen map[string]seenRange) EventSource {
	return EventSource{
		Section: section,
		Answer: func(_ context.Context, events EventRange) (SectionBody, error) {
			fromText, err := os.ReadFile(filepath.Join(events.FromDir, "week.txt"))
			if err != nil {
				return SectionBody{}, err
			}
			toText, err := os.ReadFile(filepath.Join(events.ToDir, "week.txt"))
			if err != nil {
				return SectionBody{}, err
			}
			seen[section] = seenRange{events: events, fromText: string(fromText), toText: string(toText)}
			return SectionBody{Facts: []Fact{{Name: "stub", Value: section}}}, nil
		},
	}
}

// withEventSources replaces the registry with sources for one test, and puts
// the registered sources and the checkout seam back after it.
func withEventSources(t *testing.T, sources ...EventSource) {
	t.Helper()
	registered := eventSources
	checkout := reportCheckout
	eventSources = nil
	t.Cleanup(func() {
		eventSources = registered
		reportCheckout = checkout
	})
	for _, source := range sources {
		RegisterEventSource(source)
	}
}

// withReportScratch points the report's scratch at a temporary directory for
// one test, so the run does not resolve a session inside the fixture, and
// answers it. The export itself stays the production one.
func withReportScratch(t *testing.T) string {
	t.Helper()
	scratch := t.TempDir()
	previous := reportScratch
	reportScratch = func(string) (string, error) { return scratch, nil }
	t.Cleanup(func() { reportScratch = previous })
	return scratch
}

// assertRunsRemoved fails the test when the report scratch still holds a run
// directory.
func assertRunsRemoved(t *testing.T, scratch string) {
	t.Helper()
	entries, err := os.ReadDir(scratch)
	if err != nil {
		t.Fatalf("read the report scratch: %v", err)
	}
	for _, entry := range entries {
		t.Errorf("the run left %s behind in the report scratch", entry.Name())
	}
}

// reportFixtureRepo builds a repository of three linear commits and answers
// it with their full IDs, oldest first.
func reportFixtureRepo(t *testing.T) (string, []string) {
	t.Helper()
	repo := t.TempDir()
	reportGit(t, repo, "", "init", "-q", "--initial-branch=main")
	commits := make([]string, 0, 3)
	for _, body := range []string{"one\n", "two\n", "three\n"} {
		blob := reportGit(t, repo, body, "hash-object", "-w", "--stdin")
		tree := reportGit(t, repo, "100644 blob "+blob+"\tweek.txt\n", "mktree")
		args := []string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree}
		if len(commits) != 0 {
			args = append(args, "-p", commits[len(commits)-1])
		}
		commits = append(commits, reportGit(t, repo, body, args...))
	}
	reportGit(t, repo, "", "update-ref", "refs/heads/main", commits[2])
	return repo, commits
}

// reportGit runs git in the fixture with no user or system configuration.
func reportGit(t *testing.T, repo, input string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", repo}, args...)...)
	command.Stdin = strings.NewReader(input)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// TestWeeklyReportRefusesUnresolvableRevision proves a revision that names no
// commit is refused before anything is exported, and that the refusal names
// the revision and the fix.
// Method: each case runs the action table over the fixture with one bad
// endpoint, then the run itself for the reason text; a source records whether
// it was asked and the report scratch must stay empty.
//
// VALIDATES: AC-7 of spec-weekly-event-report.
// PREVENTS: an export of `from` started before `to` was resolved, and a
// refusal that leaves the operator guessing which revision to pass.
func TestWeeklyReportRefusesUnresolvableRevision(t *testing.T) {
	repo, commits := reportFixtureRepo(t)
	scratch := withReportScratch(t)
	asked := false
	withEventSources(t, EventSource{Section: "alpha", Answer: func(context.Context, EventRange) (SectionBody, error) {
		asked = true
		return SectionBody{Facts: []Fact{{Name: "stub", Value: "alpha"}}}, nil
	}})
	reportCheckout = func() (string, error) { return repo, nil }

	cases := []struct {
		name, from, to, bad, keyword string
	}{
		{name: "from", from: "no-such-revision", to: commits[2], bad: "no-such-revision", keyword: "from revision"},
		{name: "to", from: commits[0], to: "deadbeef00", bad: "deadbeef00", keyword: "to revision"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut strings.Builder
			code := leroot.Run("weekly", leroot.LookupCommand("weekly"),
				[]string{"report", "from", tc.from, "to", tc.to, "|", "json"}, &out, &errOut)
			if code != 1 {
				t.Errorf("exit = %d, want 1 for a refused revision; stdout %q", code, out.String())
			}

			run := reportRun{repo: repo, export: exportEndpoints, sources: registeredSources(), diagnostics: io.Discard}
			_, err := run.answer(t.Context(), tc.from, tc.to)
			if err == nil {
				t.Fatal("the run answered for an unresolvable revision")
			}
			for _, want := range []string{tc.keyword, `"` + tc.bad + `"`, "full sha", "release-from", "release-to"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not carry %q", err.Error(), want)
				}
			}
			if asked {
				t.Error("a source was asked for an unresolvable revision")
			}
			assertRunsRemoved(t, scratch)
		})
	}
}

// TestWeeklyReportFailedSourceIsNamed proves a failing source answers its own
// section failed with the reason while every other section still answers, and
// that the exit code says the report is incomplete.
// Method: four sources over the fixture through the action table, `| json`:
// one returns an error, one answers no facts, one panics, one answers.
//
// VALIDATES: AC-8 and R-3 of spec-weekly-event-report.
// PREVENTS: a failure read as a quiet week, a panicking source taking the
// other sections with it, and an incomplete report that exits 0.
func TestWeeklyReportFailedSourceIsNamed(t *testing.T) {
	repo, commits := reportFixtureRepo(t)
	scratch := withReportScratch(t)
	seen := map[string]seenRange{}
	withEventSources(t,
		EventSource{Section: "broken", Answer: func(context.Context, EventRange) (SectionBody, error) {
			return SectionBody{}, errors.New("the records could not be read")
		}},
		EventSource{Section: "empty", Answer: func(context.Context, EventRange) (SectionBody, error) {
			return SectionBody{}, nil
		}},
		EventSource{Section: "panicking", Answer: func(context.Context, EventRange) (SectionBody, error) {
			panic("BUG: a source defect")
		}},
		recordingSource("working", seen),
	)
	reportCheckout = func() (string, error) { return repo, nil }

	var out, errOut strings.Builder
	code := leroot.Run("weekly", leroot.LookupCommand("weekly"),
		[]string{"report", "from", commits[0], "to", commits[2], "|", "json"}, &out, &errOut)
	if code != 1 {
		t.Errorf("exit = %d, want 1: a report with a failed section is incomplete", code)
	}

	var decoded struct {
		Sections []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Reason string `json:"reason"`
			Facts  []Fact `json:"facts"`
		} `json:"sections"`
	}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("an incomplete report must still answer its sections: %v\n%s", err, out.String())
	}
	want := map[string]struct{ status, reason string }{
		"broken":    {"failed", "the records could not be read"},
		"empty":     {"failed", "the source answered no facts"},
		"panicking": {"failed", "BUG: a source defect"},
		"working":   {"answered", ""},
	}
	if len(decoded.Sections) != len(want) {
		t.Fatalf("sections = %+v, want one per source", decoded.Sections)
	}
	for _, section := range decoded.Sections {
		expected, known := want[section.Name]
		if !known {
			t.Errorf("unexpected section %s", section.Name)
			continue
		}
		if section.Status != expected.status {
			t.Errorf("section %s status = %q, want %q", section.Name, section.Status, expected.status)
		}
		if expected.status == "failed" {
			if !strings.Contains(section.Reason, expected.reason) {
				t.Errorf("section %s reason = %q, want it to carry %q", section.Name, section.Reason, expected.reason)
			}
			if len(section.Facts) != 0 {
				t.Errorf("failed section %s carries facts %+v", section.Name, section.Facts)
			}
			continue
		}
		if len(section.Facts) != 1 {
			t.Errorf("section %s facts = %+v, want the one its source answered", section.Name, section.Facts)
		}
	}
	if _, asked := seen["working"]; !asked {
		t.Error("the source after the failing ones was never asked")
	}
	assertRunsRemoved(t, scratch)
}

// TestWeeklyReportRemovesExports proves the run directory and both exports in
// it are gone on every exit path, and that the answer names the directory.
// Method: the fixture through the run, once answering, once with a source that
// fails after checking the exports exist, once with a panicking source, and
// once with an export that fails on its second endpoint.
//
// VALIDATES: AC-9 and R-2 of spec-weekly-event-report.
// PREVENTS: a failed or panicking run leaving two trees of the checkout behind
// in scratch, where repeated weekly runs fill the disk.
func TestWeeklyReportRemovesExports(t *testing.T) {
	repo, commits := reportFixtureRepo(t)
	from, to := commits[0], commits[2]

	failMidway := func(ctx context.Context, events EventRange) (SectionBody, error) {
		for _, dir := range []string{events.FromDir, events.ToDir} {
			if _, err := os.Stat(filepath.Join(dir, "week.txt")); err != nil {
				t.Errorf("the export %s is missing while a source runs: %v", dir, err)
			}
		}
		return SectionBody{}, errors.New("failed midway")
	}
	cases := []struct {
		name   string
		answer func(context.Context, EventRange) (SectionBody, error)
	}{
		{name: "answered", answer: func(context.Context, EventRange) (SectionBody, error) {
			return SectionBody{Facts: []Fact{{Name: "stub", Value: "1"}}}, nil
		}},
		{name: "source fails", answer: failMidway},
		{name: "source panics", answer: func(context.Context, EventRange) (SectionBody, error) {
			panic("BUG: a source defect")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scratch := withReportScratch(t)
			withEventSources(t, EventSource{Section: "alpha", Answer: tc.answer})
			run := reportRun{repo: repo, export: exportEndpoints, sources: registeredSources(), diagnostics: io.Discard}
			report, err := run.answer(t.Context(), from, to)
			if err != nil {
				t.Fatalf("the run refused: %v", err)
			}
			if filepath.Dir(report.RunDir) != scratch {
				t.Errorf("the answer names run directory %q, want one under %s", report.RunDir, scratch)
			}
			if _, statErr := os.Stat(report.RunDir); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("the run directory %s is still there: %v", report.RunDir, statErr)
			}
			assertRunsRemoved(t, scratch)
		})
	}

	t.Run("export fails", func(t *testing.T) {
		scratch := withReportScratch(t)
		missing := strings.Repeat("0", len(from))
		if _, err := exportEndpoints(t.Context(), repo, from, missing); err == nil {
			t.Fatal("the export answered for a commit the repository does not hold")
		}
		assertRunsRemoved(t, scratch)
	})
}
