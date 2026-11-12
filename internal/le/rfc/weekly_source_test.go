package rfc

// VALIDATES: AC-2, AC-3, AC-4 and AC-8 of spec-weekly-event-report: the three
// RFC sections `le weekly report` answers.
// METHOD: the seam that readies an endpoint's own le is replaced by an
// in-process answer over a fixture tree (treeReport, discriminationStatusOf,
// marshaled as `| json` does), or by canned answers built from the
// production types; the sections are then asked as the report asks them.
// PREVENTS: a report figure that differs from the ledger's, a lost proof read
// as no event, an interop record dropped because it names no scenario, and a
// key an older le does not publish read as zero.

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/weekly"
)

// weeklyFixture answers a synthetic checkout holding rfc9999: unwalked with
// nothing proven, or walked with its one MUST proven by a verified record,
// built as TestFixtureProofWalksTheStemAndVerifiesItsRecords builds it.
func weeklyFixture(t *testing.T, proven bool) string {
	t.Helper()
	files := partialFiles()
	files[selftestWorkflowRel] = selftestWorkflow
	files[FixtureAggregateRel] = FixtureAggregateProfile
	root := t.TempDir()
	writeFixtureFiles(t, root, files)
	if !proven {
		return root
	}
	if err := FixtureProof(root, selftestStem, map[string]string{"2:1": selftestRIDSend}, fixtureProofUnsealed()...); err != nil {
		t.Fatal(err)
	}
	writeFixtureFiles(t, root, map[string]string{
		selftestSummaryRel: strings.Replace(files[selftestSummaryRel], " "+partialMarker, "", 1),
	})
	return root
}

// inProcessLe answers what an endpoint's le prints for the two commands the
// report sends, from today's producers over tree.
func inProcessLe(tree string) endpointLe {
	return func(_ context.Context, args ...string) ([]byte, error) {
		if slices.Equal(args, []string{"rfc", "tree"}) {
			report, err := treeReport(tree, "")
			if err != nil {
				return nil, err
			}
			return json.Marshal(report)
		}
		if len(args) == 4 && slices.Equal(args[:3], []string{"rfc", "discriminate", keyStem}) {
			stem := args[3]
			status, err := discriminationStatusOf(tree, stem, "", func(rid string) bool { return hasRIDStem(rid, stem) })
			if err != nil {
				return nil, err
			}
			return json.Marshal(status)
		}
		return nil, errors.New("the fixture le was sent " + strings.Join(args, " "))
	}
}

// withEndpointLe replaces the seam for one test.
func withEndpointLe(t *testing.T, answer func(keyword, tree string) endpointLe) {
	t.Helper()
	saved := endpointLeOf
	endpointLeOf = func(_ context.Context, keyword, tree, _ string) (endpointLe, error) {
		return answer(keyword, tree), nil
	}
	t.Cleanup(func() { endpointLeOf = saved })
}

// weeklyGit runs git in the fixture repository with no user or system
// configuration.
func weeklyGit(t *testing.T, repo, input string, args ...string) string {
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

// weeklyRepo answers a repository of two commits, from and to. With
// recordChanged, to adds rfc/discrimination/rfc9999/RFC9999-2-1.json, so the
// range changes that stem's record file.
func weeklyRepo(t *testing.T, recordChanged bool) (repo, from, to string) {
	t.Helper()
	repo = t.TempDir()
	weeklyGit(t, repo, "", "init", "-q", "--initial-branch=main")
	blob := weeklyGit(t, repo, "week\n", "hash-object", "-w", "--stdin")
	base := "100644 blob " + blob + "\tweek.txt\n"
	identity := []string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree"}
	from = weeklyGit(t, repo, "from", append(identity, weeklyGit(t, repo, base, "mktree"))...)
	listing := base
	if recordChanged {
		record := weeklyGit(t, repo, "[]\n", "hash-object", "-w", "--stdin")
		stem := weeklyGit(t, repo, "100644 blob "+record+"\tRFC9999-2-1.json\n", "mktree")
		discrimination := weeklyGit(t, repo, "040000 tree "+stem+"\trfc9999\n", "mktree")
		rfcDir := weeklyGit(t, repo, "040000 tree "+discrimination+"\tdiscrimination\n", "mktree")
		listing += "040000 tree " + rfcDir + "\trfc\n"
	}
	to = weeklyGit(t, repo, "to", append(identity, weeklyGit(t, repo, listing, "mktree"), "-p", from)...)
	return repo, from, to
}

// weeklyEvents answers the range a report hands a source.
func weeklyEvents(t *testing.T, recordChanged bool, fromDir, toDir string) weekly.EventRange {
	t.Helper()
	repo, from, to := weeklyRepo(t, recordChanged)
	return weekly.EventRange{Repo: repo, FromCommit: from, ToCommit: to, FromDir: fromDir, ToDir: toDir,
		WorkDir: t.TempDir(), Commits: []string{to}}
}

// weeklyFact answers the fact of body named name, failing the test without one.
func weeklyFact(t *testing.T, body weekly.SectionBody, name string) weekly.Fact {
	t.Helper()
	for _, fact := range body.Facts {
		if fact.Name == name {
			return fact
		}
	}
	t.Fatalf("the section carries no fact %q: %+v", name, body.Facts)
	return weekly.Fact{}
}

// TestReportProvenMatchesTreeReport proves the section's MUST figure at each
// endpoint is the ledger's: the sum rollupOf takes over the figures
// StemFiguresOf derives for that tree, read through the same treeReport
// answer the endpoint's le prints. The `to` fixture proves one MUST, so the
// comparison cannot pass on two zeros.
//
// VALIDATES: AC-2, A-1, R-1.
func TestReportProvenMatchesTreeReport(t *testing.T) {
	from, to := weeklyFixture(t, false), weeklyFixture(t, true)
	withEndpointLe(t, func(_, tree string) endpointLe { return inProcessLe(tree) })

	body, err := answerWeeklyProven(t.Context(), weeklyEvents(t, false, from, to))
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []struct{ name, tree string }{{"from", from}, {"to", to}} {
		corpus, err := loadPriorityCorpus(endpoint.tree)
		if err != nil {
			t.Fatal(err)
		}
		rollup := rollupOf(corpus.figures, slices.Sorted(maps.Keys(corpus.figures)))
		want := strconv.Itoa(rollup.Mandatory.Proven) + " of " + strconv.Itoa(rollup.Mandatory.Total)
		if got := weeklyFact(t, body, "MUSTs proven at "+endpoint.name).Value; got != want {
			t.Errorf("MUSTs proven at %s = %q, want the ledger's %q", endpoint.name, got, want)
		}
	}
	if got := weeklyFact(t, body, "MUSTs proven at to").Value; got != "1 of 1" {
		t.Errorf("the fixture's to figure = %q, want 1 of 1: the comparison above would hold on zeros", got)
	}
}

// TestReportListsNewlyProvenAndLost proves a requirement proven at `to` alone
// is listed as newly proven, one proven at `from` alone as lost, and that a
// stem whose counts did not move is still compared record by record when its
// record files changed, and not otherwise (the stated bound).
//
// VALIDATES: AC-2.
func TestReportListsNewlyProvenAndLost(t *testing.T) {
	tests := []struct {
		name                       string
		fromProven, toProven       bool
		recordChanged              bool
		gained, lost, comparedWant []string
	}{
		{"proven in the range", false, true, false, []string{selftestRIDSend}, nil, []string{selftestStem}},
		{"lost in the range", true, false, false, nil, []string{selftestRIDSend}, []string{selftestStem}},
		{"counts equal, record file changed", true, true, true, nil, nil, []string{selftestStem}},
		{"counts equal, no record file changed", true, true, false, nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to := weeklyFixture(t, tt.fromProven), weeklyFixture(t, tt.toProven)
			withEndpointLe(t, func(_, tree string) endpointLe { return inProcessLe(tree) })

			body, err := answerWeeklyProven(t.Context(), weeklyEvents(t, tt.recordChanged, from, to))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []struct {
				fact  string
				items []string
			}{
				{"requirements newly proven", tt.gained},
				{"requirements no longer proven", tt.lost},
				{"documents compared record by record", tt.comparedWant},
			} {
				fact := weeklyFact(t, body, want.fact)
				if !slices.Equal(fact.Items, want.items) && len(fact.Items)+len(want.items) != 0 {
					t.Errorf("%s = %q, want %q", want.fact, fact.Items, want.items)
				}
				if fact.Value != strconv.Itoa(len(want.items)) {
					t.Errorf("%s counts %s, want %d", want.fact, fact.Value, len(want.items))
				}
			}
			if !strings.Contains(body.Note, "is not listed") {
				t.Errorf("the note does not state the bound of the comparison: %q", body.Note)
			}
		})
	}
}

// interopChecker is the checker file the interop fixture holds: one checker
// naming a scenario the tree holds, one naming none.
const (
	interopCheckerRel    = "internal/le/interoplab/bgp/check_fixture.go"
	interopCheckerSource = "package bgp\n\nfunc checkFixture() {\n\tconst name = \"fixture-scenario\"\n}\n\n" +
		"func checkNameless() {\n}\n"
)

// cannedLe answers fixed `| json` answers, encoded from the production types,
// so the shape is the one today's le prints.
func cannedLe(t *testing.T, figure Figure, records []DiscriminationRecord) endpointLe {
	t.Helper()
	tree, err := json.Marshal(&TreeReport{Rows: []*TreeRow{{Stem: selftestStem, Summary: true,
		StemFigures: StemFigures{Stem: selftestStem, Walked: true, Mandatory: figure}}}})
	if err != nil {
		t.Fatal(err)
	}
	status, err := json.Marshal(discriminationStatus{Selector: selftestStem, Records: records})
	if err != nil {
		t.Fatal(err)
	}
	return func(_ context.Context, args ...string) ([]byte, error) {
		if args[1] == "tree" {
			return tree, nil
		}
		return status, nil
	}
}

// TestReportNamesInteropScenarios proves an interop checker that gained a
// proving record is named by its scenario, one that names no scenario is
// listed by its unit with the reason, and that a record proven at both
// endpoints, a unit test's record and an escape are not interop events.
//
// VALIDATES: AC-3.
func TestReportNamesInteropScenarios(t *testing.T) {
	to := t.TempDir()
	writeFixtureFiles(t, to, map[string]string{
		interopCheckerRel:   interopCheckerSource,
		selftestWorkflowRel: selftestWorkflow,
		"test/interop/scenarios/fixture-scenario/README.md": "fixture\n",
	})
	unit := func(name string) string { return interopCheckerRel + "::" + name }
	old := DiscriminationRecord{RID: "RFC9999-2-2", Polarity: PolarityPositive, Unit: unit("checkOld"), Route: RouteRevert}
	toRecords := []DiscriminationRecord{
		old,
		{RID: selftestRIDSend, Polarity: PolarityPositive, Unit: unit("checkFixture"), Route: RouteRevert},
		{RID: selftestRIDSend, Polarity: PolarityNegative, Unit: unit("checkNameless"), Route: RouteRevert},
		{RID: selftestRIDDrop, Polarity: PolarityPositive, Unit: unit("checkFixture"), Route: RouteNoBreak},
		{RID: selftestRIDSend, Polarity: PolarityPositive, Unit: selftestDiscriminationUnit, Route: RouteMutant},
	}
	withEndpointLe(t, func(keyword, _ string) endpointLe {
		if keyword == "from" {
			return cannedLe(t, Figure{Proven: 1, Total: 3}, []DiscriminationRecord{old})
		}
		return cannedLe(t, Figure{Proven: 2, Total: 3}, toRecords)
	})

	body, err := answerWeeklyInterop(t.Context(), weeklyEvents(t, false, t.TempDir(), to))
	if err != nil {
		t.Fatal(err)
	}
	scenarios := weeklyFact(t, body, "scenarios that gained a proving record")
	if !slices.Equal(scenarios.Items, []string{"bgp fixture-scenario"}) || scenarios.Value != "1" {
		t.Errorf("scenarios = %s %q, want 1 [bgp fixture-scenario]", scenarios.Value, scenarios.Items)
	}
	unmapped := weeklyFact(t, body, "interop records naming no scenario")
	if len(unmapped.Items) != 1 || unmapped.Value != "1" {
		t.Fatalf("unmapped = %s %q, want the one nameless checker", unmapped.Value, unmapped.Items)
	}
	if !strings.HasPrefix(unmapped.Items[0], unit("checkNameless")+": ") || !strings.Contains(unmapped.Items[0], "declares no") {
		t.Errorf("unmapped item %q does not name the unit and why no scenario was read", unmapped.Items[0])
	}
}

// TestReportListsNewSignOffs proves a document walked at `to` and not at
// `from` is listed as signed off in the range, and the reverse as no longer
// signed off.
//
// VALIDATES: AC-4.
func TestReportListsNewSignOffs(t *testing.T) {
	unwalked, walked := weeklyFixture(t, false), weeklyFixture(t, true)
	withEndpointLe(t, func(_, tree string) endpointLe { return inProcessLe(tree) })

	for _, tt := range []struct {
		name, from, to   string
		signed, unsigned string
	}{
		{"signed off", unwalked, walked, "1", "0"},
		{"no longer signed off", walked, unwalked, "0", "1"},
	} {
		body, err := answerWeeklyExtraction(t.Context(), weeklyEvents(t, false, tt.from, tt.to))
		if err != nil {
			t.Fatal(err)
		}
		signed := weeklyFact(t, body, "documents signed off in the range")
		unsigned := weeklyFact(t, body, "documents no longer signed off")
		if signed.Value != tt.signed || unsigned.Value != tt.unsigned {
			t.Fatalf("%s: signed %s %q, no longer %s %q", tt.name, signed.Value, signed.Items, unsigned.Value, unsigned.Items)
		}
		listed := slices.Concat(signed.Items, unsigned.Items)
		if !slices.Equal(listed, []string{selftestStem}) {
			t.Errorf("%s: listed %q, want [%s]", tt.name, listed, selftestStem)
		}
	}
}

// TestReportRefusesAnAnswerMissingAKey proves an endpoint's answer without a
// key the report reads fails every RFC section, naming the key, rather than
// reading it as zero, and that the le is asked once for the three.
//
// VALIDATES: AC-8.
func TestReportRefusesAnAnswerMissingAKey(t *testing.T) {
	asked := 0
	withEndpointLe(t, func(string, string) endpointLe {
		return func(context.Context, ...string) ([]byte, error) {
			asked++
			return []byte(`{"rows":[{"stem":"rfc9999","summary":true,"mandatory-proven":1,"mandatory-total":1,` +
				`"optional-proven":0,"optional-total":0}]}`), nil
		}
	})
	events := weeklyEvents(t, false, t.TempDir(), t.TempDir())
	for _, answer := range []func(context.Context, weekly.EventRange) (weekly.SectionBody, error){
		answerWeeklyProven, answerWeeklyInterop, answerWeeklyExtraction,
	} {
		_, err := answer(t.Context(), events)
		if err == nil {
			t.Fatal("a row with no `walked` key was answered, want a refusal")
		}
		if !strings.Contains(err.Error(), "`walked`") || !strings.Contains(err.Error(), "from") {
			t.Errorf("the refusal does not name the key and the endpoint: %v", err)
		}
	}
	if asked != 1 {
		t.Errorf("the endpoint's le was asked %d times, want once for the three sections", asked)
	}
}

// TestEndpointLeRunsInTheExport proves the production seam strips the
// launcher's build name and every spelling of the repository root, and names
// the export as the root, by running a shell script in place of le.
func TestEndpointLeRunsInTheExport(t *testing.T) {
	tree := t.TempDir()
	script := filepath.Join(t.TempDir(), "le")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s|%s|%s|%s' \"$*\" \"$ZE_REPO_ROOT\" \"$ZE_LE_BUILD_NAME\" \"$(pwd -P)\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZE_LE_BUILD_NAME", "other")
	t.Setenv("ze.repo.root", "/elsewhere")
	out, err := runEndpointLe(t.Context(), script, tree, []string{"rfc", "tree"})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(tree)
	if err != nil {
		t.Fatal(err)
	}
	want := "rfc tree | json|" + tree + "||" + resolved
	if string(out) != want {
		t.Errorf("the child saw %q, want %q", out, want)
	}
}
