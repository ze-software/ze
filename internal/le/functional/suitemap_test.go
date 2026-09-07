// Related: suitemap.go, run.go -- the reader these tests drive and the run that consults it
//
// VALIDATES: spec-verify-scope-5-suite-coverage-map AC-4 and AC-6 (every route
// that cannot answer runs every suite) and AC-8 (an operator skip the map
// cannot add back).
// PREVENTS: a map that narrows on a reason it could not check, a widening a
// caller reads as "no suite runs", and a suite recording nothing read as a
// suite that covers nothing.

package functional

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/changed"
	"github.com/ze-software/ze/internal/le/job"
)

// writeSuiteMap puts one map body at the artifact path inside a throwaway
// checkout root, and answers that root.
func writeSuiteMap(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, suiteMapPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create the artifact directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the suite map: %v", err)
	}
	return root
}

// TestAbsentMapRunsEverySuite drives the state every fresh checkout and every
// CI shard is in: no map has ever been recorded. The run must be the full one,
// and the answer must say why in a sentence a reader can act on.
func TestAbsentMapRunsEverySuite(t *testing.T) {
	selection := selectSuites(t.TempDir())

	if selection.Verdict != verdictEverySuite {
		t.Fatalf("an absent map answered verdict %d, want verdictEverySuite", selection.Verdict)
	}
	for _, name := range GatingNames() {
		if !selection.runs(name) {
			t.Errorf("suite %s does not run under an absent map", name)
		}
	}
	if selection.Reason == "" {
		t.Error("the widening names no reason")
	}
}

// TestAnUnusableSuiteMapWidens walks every shape a map can arrive in that the
// reader cannot answer from. The file is under tmp/, which several sessions
// share, so each one must widen and none of them may narrow.
func TestAnUnusableSuiteMapWidens(t *testing.T) {
	cases := []struct {
		name string
		body string
		says string
	}{
		{name: "truncated", body: `{"head":"a1b2c3","reached":{"encode":["./inter`, says: "malformed"},
		{name: "not json at all", body: "encode ./internal/component/ssh\n", says: "malformed"},
		{name: "no recorded commit", body: `{"reached":{"encode":["./internal/component/ssh"]}}`, says: "records no commit"},
		{name: "blank recorded commit", body: `{"head":"  ","reached":{"encode":["./x"]}}`, says: "records no commit"},
		{name: "no suite", body: `{"head":"a1b2c3","reached":{}}`, says: "records no suite"},
		{name: "an unnamed package", body: `{"head":"a1b2c3","reached":{"encode":[""]}}`, says: "unnamed package"},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			selection := selectSuites(writeSuiteMap(t, one.body))

			if selection.Verdict != verdictEverySuite {
				t.Fatalf("verdict %d, want verdictEverySuite", selection.Verdict)
			}
			if !selection.runs("encode") {
				t.Error("the encode suite does not run")
			}
			if !strings.Contains(selection.Reason, one.says) {
				t.Errorf("the reason is %q, which does not name %q", selection.Reason, one.says)
			}
		})
	}
}

// TestAnUnreadableSuiteMapWidens uses a directory at the artifact path, which
// is what a session that crashed mid-write can leave behind. A read error is
// not a map that records nothing.
func TestAnUnreadableSuiteMapWidens(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, suiteMapPath), 0o750); err != nil {
		t.Fatalf("create a directory at the artifact path: %v", err)
	}

	selection := selectSuites(root)

	if selection.Verdict != verdictEverySuite {
		t.Fatalf("verdict %d, want verdictEverySuite", selection.Verdict)
	}
	if !strings.Contains(selection.Reason, "read the suite map") {
		t.Errorf("the reason is %q, which does not say the read failed", selection.Reason)
	}
}

// TestAnEmptySuiteSetIsRefusedByTheReader holds the boundary the spec draws:
// zero suites selected is a valid answer, and zero packages under a suite is
// not. Reading an empty recorded set as "this suite covers nothing" would skip
// that suite for ever.
func TestAnEmptySuiteSetIsRefusedByTheReader(t *testing.T) {
	root := writeSuiteMap(t, `{"head":"a1b2c3","reached":{"encode":["./internal/le"],"web":[]}}`)

	if _, err := readSuiteMap(root); err == nil {
		t.Fatal("a suite recording no package was accepted")
	} else if !strings.Contains(err.Error(), "the recording broke") {
		t.Errorf("the refusal is %q, which does not say the recording broke", err)
	}

	if selection := selectSuites(root); selection.Verdict != verdictEverySuite {
		t.Errorf("verdict %d, want verdictEverySuite", selection.Verdict)
	}
}

// TestAWideningIsNeverAnEmptySelection is the whole point of the verdict. A
// widening carries no suite names, and a caller that read the slice instead of
// asking runs would run nothing at all.
func TestAWideningIsNeverAnEmptySelection(t *testing.T) {
	// The zero value carries a name, and that name is neither answer. A
	// selection nobody filled in must not read as one of the two verdicts.
	if empty := (suiteSelection{}); empty.Verdict != verdictUnspecified {
		t.Fatalf("the zero value of a selection carries verdict %d, want verdictUnspecified", empty.Verdict)
	}

	for _, one := range []struct {
		name      string
		selection suiteSelection
	}{
		{name: "the zero value nobody filled in", selection: suiteSelection{}},
		{name: "a widening", selection: everySuite("nothing to go on")},
	} {
		t.Run(one.name, func(t *testing.T) {
			if len(one.selection.Suites) != 0 {
				t.Fatalf("this selection names %d suite(s), so it is not the case under test", len(one.selection.Suites))
			}
			for _, name := range GatingNames() {
				if !one.selection.runs(name) {
					t.Errorf("suite %s does not run", name)
				}
			}
		})
	}
}

// TestAValidSuiteMapIsReadBack proves the format round-trips, and pins the
// second half of the selection: a well-formed map narrows nothing on its own.
// The change set is the other input, and a tree that cannot answer for one
// widens however good the map is.
func TestAValidSuiteMapIsReadBack(t *testing.T) {
	root := writeSuiteMap(t,
		`{"head":"447ba80f16","reached":{"encode":["./internal/component/bgp/wire"],`+
			`"ui":["./internal/component/ssh","./internal/component/cli"]}}`)

	recorded, err := readSuiteMap(root)
	if err != nil {
		t.Fatalf("read a well-formed suite map: %v", err)
	}
	if recorded.Head != "447ba80f16" {
		t.Errorf("the recorded commit is %q, want 447ba80f16", recorded.Head)
	}
	if got := len(recorded.Reached["ui"]); got != 2 {
		t.Errorf("the ui suite records %d package(s), want 2", got)
	}
	if recorded.Reached["ui"][0] != "./internal/component/ssh" {
		t.Errorf("the first ui package is %q, want the selector's spelling", recorded.Reached["ui"][0])
	}

	// The temporary directory is not a checkout, so the change-set selector
	// refuses it and the selection widens on a map it read perfectly well.
	selection := selectSuites(root)
	if selection.Verdict != verdictEverySuite {
		t.Fatalf("verdict %d, want verdictEverySuite: the change set is unknown", selection.Verdict)
	}
	if !strings.Contains(selection.Reason, "selector refused this checkout") {
		t.Errorf("the reason is %q, which does not say the change set is what is missing", selection.Reason)
	}
}

// TestTheGatingRunListWidensAndNeverAddsBackASkip drives the decision the
// gating run makes before it builds anything: the denominator and the suites
// the loop starts are one answer, and ZE_SKIP_SUITES outranks the map.
func TestTheGatingRunListWidensAndNeverAddsBackASkip(t *testing.T) {
	suites, err := GatingSuites(Gating, Suites)
	if err != nil {
		t.Fatalf("resolve the gating suites: %v", err)
	}

	t.Run("a widening runs every gating suite", func(t *testing.T) {
		running, skipped := gatingRunList(suites, map[string]bool{}, everySuite("no map"))

		if len(running) != len(suites) {
			t.Errorf("%d suite(s) run, want all %d", len(running), len(suites))
		}
		if len(skipped) != 0 {
			t.Errorf("%d suite(s) skipped, want none", len(skipped))
		}
	})

	t.Run("an operator skip outranks a selection that named it", func(t *testing.T) {
		selection := suiteSelection{Verdict: verdictSelected, Suites: []string{suiteWeb, suiteEncode}}

		running, skipped := gatingRunList(suites, map[string]bool{suiteWeb: true}, selection)

		if len(running) != 1 || running[0].Name != suiteEncode {
			t.Errorf("the run list is %v, want encode alone", suiteNames(running))
		}
		if len(skipped) != 1 || skipped[0].Name != suiteWeb {
			t.Errorf("the skip list is %v, want web alone", suiteNames(skipped))
		}
	})
}

// fullRecording is what a whole instrumented gating run holds: one entry for
// every gating suite. The named suites record nothing, which is the state
// editor, web, runner and policy are in on every run.
func fullRecording(head string, recordNothing ...string) *suiteRecording {
	silent := map[string]bool{}
	for _, name := range recordNothing {
		silent[name] = true
	}
	recording := newSuiteRecording(head)
	for _, name := range Gating {
		if silent[name] {
			recording.record(name, nil)
			continue
		}
		recording.record(name, []string{"./internal/component/ssh"})
	}
	return recording
}

// TestEmptyRecordedSetIsARefusal holds the boundary from the writer's side.
// The reader refuses a suite recorded with an empty set, so the writer must
// never produce one: a suite that recorded nothing is left OUT of the map,
// which makes it unknown and therefore always run, and a run in which no suite
// recorded anything writes no map at all.
//
// VALIDATES: spec-verify-scope-5-suite-coverage-map AC-1.
func TestEmptyRecordedSetIsARefusal(t *testing.T) {
	t.Run("a suite that recorded nothing is omitted, never written empty", func(t *testing.T) {
		root := t.TempDir()

		omitted, err := fullRecording("447ba80f16", suiteEditor, suiteWeb, suiteRunner).publish(root)
		if err != nil {
			t.Fatalf("publish a run in which two suites recorded nothing: %v", err)
		}
		if want := []string{suiteEditor, suiteWeb, suiteRunner}; !slices.Equal(slices.Sorted(slices.Values(omitted)),
			slices.Sorted(slices.Values(want))) {
			t.Errorf("the omitted suites are %v, want %v", omitted, want)
		}

		recorded, err := readSuiteMap(root)
		if err != nil {
			t.Fatalf("the published map is one the reader refuses: %v", err)
		}
		for _, name := range []string{suiteEditor, suiteWeb, suiteRunner} {
			if packages, present := recorded.Reached[name]; present {
				t.Errorf("suite %s is in the map with %d package(s), and it recorded none", name, len(packages))
			}
		}
		if len(recorded.Reached) != len(Gating)-3 {
			t.Errorf("the map names %d suite(s), want the %d that recorded something",
				len(recorded.Reached), len(Gating)-3)
		}
	})

	t.Run("a run in which no suite recorded anything writes no map", func(t *testing.T) {
		root := t.TempDir()

		_, err := fullRecording("447ba80f16", Gating...).publish(root)

		if err == nil {
			t.Fatal("a run that recorded nothing at all published a map")
		}
		if !strings.Contains(err.Error(), "records no suite") {
			t.Errorf("the refusal is %q, which does not say the map names no suite", err)
		}
		if _, statErr := os.Stat(filepath.Join(root, suiteMapPath)); !os.IsNotExist(statErr) {
			t.Errorf("a map exists at the artifact path: %v", statErr)
		}
	})
}

// TestOnlyAWholeGatingRunPublishesAMap is the other half of the same rule. A
// suite the map does not name is unknown to the reader and always runs, so a
// map written by a run that ran ONE suite would declare every other suite
// unknown while the next run narrowed on the one it named. The refusal reads
// the gating list rather than trusting the caller.
func TestOnlyAWholeGatingRunPublishesAMap(t *testing.T) {
	t.Run("a single-suite run publishes nothing", func(t *testing.T) {
		root := t.TempDir()
		recording := newSuiteRecording("447ba80f16")
		recording.record(suiteEncode, []string{"./internal/component/bgp/wire"})

		_, err := recording.publish(root)

		if err == nil {
			t.Fatal("a run that ran one suite published a map")
		}
		if !strings.Contains(err.Error(), "only a full run") {
			t.Errorf("the refusal is %q, which does not say a full run is what publishes", err)
		}
		if _, statErr := os.Stat(filepath.Join(root, suiteMapPath)); !os.IsNotExist(statErr) {
			t.Errorf("a map exists at the artifact path: %v", statErr)
		}
	})

	t.Run("a run that cannot name its commit publishes nothing", func(t *testing.T) {
		for _, head := range []string{"", job.Unknown} {
			root := t.TempDir()

			_, err := fullRecording(head).publish(root)

			if err == nil {
				t.Fatalf("a run whose commit is %q published a map", head)
			}
			if !strings.Contains(err.Error(), "cannot name the commit") {
				t.Errorf("the refusal is %q, which does not say the commit is missing", err)
			}
			if _, statErr := os.Stat(filepath.Join(root, suiteMapPath)); !os.IsNotExist(statErr) {
				t.Errorf("a map exists at the artifact path: %v", statErr)
			}
		}
	})

	t.Run("a whole run publishes a map the reader accepts", func(t *testing.T) {
		root := t.TempDir()

		omitted, err := fullRecording("447ba80f16").publish(root)

		if err != nil {
			t.Fatalf("publish a whole gating run: %v", err)
		}
		if len(omitted) != 0 {
			t.Errorf("%d suite(s) were omitted, want none", len(omitted))
		}
		recorded, err := readSuiteMap(root)
		if err != nil {
			t.Fatalf("read the published map back: %v", err)
		}
		if recorded.Head != "447ba80f16" {
			t.Errorf("the map records commit %q, want the one the run read", recorded.Head)
		}
		if len(recorded.Reached) != len(Gating) {
			t.Errorf("the map names %d suite(s), want all %d", len(recorded.Reached), len(Gating))
		}
	})
}

// gitCheckout builds a checkout the selection can be driven over: a Git
// repository holding one Go file, and the commit it holds it at.
//
// The selection asks Git which files moved since the map was recorded, so a
// bare directory answers nothing and every case here would widen for the wrong
// reason.
func gitCheckout(t *testing.T) (root, head string) {
	t.Helper()
	root = t.TempDir()
	writeCheckoutFile(t, root, ".gitignore", "tmp/\n")
	writeCheckoutFile(t, root, "internal/component/ssh/ssh.go", "package ssh\n")
	writeCheckoutFile(t, root, "internal/component/cli/cli.go", "package cli\n")
	commitCheckout(t, root, "fixture")

	head = job.Head(root)
	if head == job.Unknown {
		t.Fatalf("the fixture checkout at %s names no commit", root)
	}
	return root, head
}

// writeCheckoutFile puts one file in the checkout, creating its directory.
func writeCheckoutFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create the directory for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// commitCheckout commits everything in the checkout under one message.
func commitCheckout(t *testing.T, root, message string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"config", "commit.gpgsign", "false"},
		{"add", "."},
		{"commit", "--quiet", "--no-gpg-sign", "-m", message},
	} {
		if args[0] == "init" && dirExists(filepath.Join(root, ".git")) {
			continue
		}
		command := exec.CommandContext(t.Context(), "git", args...)
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
}

// dirExists says whether the path is a directory that is already there.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// recordedMap writes the map one gating run would have published at head.
func recordedMap(t *testing.T, root, head string, reached map[string][]string) {
	t.Helper()
	body, err := json.Marshal(suiteMap{Head: head, Reached: reached})
	if err != nil {
		t.Fatalf("render the fixture map: %v", err)
	}
	path := filepath.Join(root, suiteMapPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create the artifact directory: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write the fixture map: %v", err)
	}
}

// changedPackage is the one package every case here changes. It is the package
// spec-verify-scope-5-suite-coverage-map AC-3 names, and phase 1b measured it
// as reached by the ui suite alone.
const changedPackage = "./internal/component/ssh"

// everySuiteReaching is the map a full run would publish when changedPackage is
// reached by the named suites and every other gating suite reached something
// else. The suites in silent are OMITTED, which is the state editor, web,
// runner and policy are in on every run.
func everySuiteReaching(reaching, silent []string) map[string][]string {
	reached := map[string][]string{}
	for _, name := range Gating {
		if slices.Contains(silent, name) {
			continue
		}
		if slices.Contains(reaching, name) {
			reached[name] = []string{changedPackage, "./internal/le"}
			continue
		}
		reached[name] = []string{"./internal/le"}
	}
	return reached
}

// changeSetIs names the change-set answer a verify run published, which is
// where the selection reads the packages it intersects with the map
// (publishChangeScope, internal/le/verify/engine/scope.go).
func changeSetIs(t *testing.T, packages ...string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scope-packages.txt")
	if err := os.WriteFile(path, []byte(strings.Join(packages, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write the change-set answer: %v", err)
	}
	nameForTest(t, changed.ScopeFileKey, path)
}

// nameForTest gives one environment key a value for this test alone.
func nameForTest(t *testing.T, key, value string) {
	t.Helper()
	previous := env.Get(key)
	if err := env.Set(key, value); err != nil {
		t.Fatalf("name %s: %v", key, err)
	}
	t.Cleanup(func() {
		if err := env.Set(key, previous); err != nil {
			t.Fatalf("restore %s: %v", key, err)
		}
	})
}

// TestSuiteSelectionSkipsOnlyUnreachedSuites is the narrowing itself: a change
// set of one package runs the suites recorded as reaching it, and the suites
// the map never named, and nothing else.
//
// VALIDATES: spec-verify-scope-5-suite-coverage-map AC-3 and AC-4.
func TestSuiteSelectionSkipsOnlyUnreachedSuites(t *testing.T) {
	silent := []string{suiteEditor, suiteWeb, suiteRunner, suitePolicy}
	root, head := gitCheckout(t)
	recordedMap(t, root, head, everySuiteReaching([]string{suiteUi, suiteParse}, silent))

	t.Run("a recorded package runs the suites that reached it, and the unknown suites", func(t *testing.T) {
		changeSetIs(t, changedPackage)

		selection := selectSuites(root)

		if selection.Verdict != verdictSelected {
			t.Fatalf("verdict %d with reason %q, want verdictSelected", selection.Verdict, selection.Reason)
		}
		// A suite the map does not name is UNKNOWN and always runs: the map
		// learned nothing about it, so it can rule nothing out for it.
		want := []string{suiteParse, suiteUi, suiteEditor, suitePolicy, suiteWeb, suiteRunner}
		for _, name := range GatingNames() {
			if got, expected := selection.runs(name), slices.Contains(want, name); got != expected {
				t.Errorf("suite %s runs=%v, want %v", name, got, expected)
			}
		}
		if !slices.Equal(selection.Suites, []string{suiteParse, suiteUi, suiteEditor, suitePolicy, suiteWeb, suiteRunner}) {
			t.Errorf("the run list is %v, want it in gating order", selection.Suites)
		}
	})

	t.Run("a package the map never recorded widens and is named", func(t *testing.T) {
		changeSetIs(t, changedPackage, "./internal/component/nowhere")

		selection := selectSuites(root)

		if selection.Verdict != verdictEverySuite {
			t.Fatalf("verdict %d, want verdictEverySuite", selection.Verdict)
		}
		if !strings.Contains(selection.Reason, "./internal/component/nowhere") {
			t.Errorf("the reason is %q, and it must name the package it could not answer for", selection.Reason)
		}
		for _, name := range GatingNames() {
			if !selection.runs(name) {
				t.Errorf("suite %s does not run under a package the map cannot answer for", name)
			}
		}
	})

	t.Run("a change set holding no package still runs the suites the map never named", func(t *testing.T) {
		changeSetIs(t)

		selection := selectSuites(root)

		if selection.Verdict != verdictSelected {
			t.Fatalf("verdict %d with reason %q, want verdictSelected", selection.Verdict, selection.Reason)
		}
		if !slices.Equal(selection.Suites, []string{suiteEditor, suitePolicy, suiteWeb, suiteRunner}) {
			t.Errorf("the run list is %v, want the four suites the map does not name", selection.Suites)
		}
	})

	t.Run("a recording run runs every suite, because only a full run publishes", func(t *testing.T) {
		changeSetIs(t, changedPackage)
		nameForTest(t, "ze.cover", "1")

		selection := selectSuites(root)

		if selection.Verdict != verdictEverySuite {
			t.Fatalf("verdict %d, want verdictEverySuite", selection.Verdict)
		}
		if !strings.Contains(selection.Reason, "records the suite map") {
			t.Errorf("the reason is %q, which does not say this run is the recording one", selection.Reason)
		}
	})
}

// TestStaleMapTreatsTouchedPackagesAsUnknown drives the map's staleness rule: a
// package the map records is answerable only while no commit since the
// recording has touched it. The control case is the point of the test, because
// a selection that widened on any commit at all would pass the first half.
//
// VALIDATES: spec-verify-scope-5-suite-coverage-map AC-5.
func TestStaleMapTreatsTouchedPackagesAsUnknown(t *testing.T) {
	silent := []string{suiteEditor, suiteWeb, suiteRunner, suitePolicy}

	t.Run("a commit touching the package widens and names it", func(t *testing.T) {
		root, head := gitCheckout(t)
		recordedMap(t, root, head, everySuiteReaching([]string{suiteUi}, silent))
		changeSetIs(t, changedPackage)

		writeCheckoutFile(t, root, "internal/component/ssh/ssh.go", "package ssh\n\nfunc Listen() {}\n")
		commitCheckout(t, root, "ssh moved under the map")

		selection := selectSuites(root)

		if selection.Verdict != verdictEverySuite {
			t.Fatalf("verdict %d, want verdictEverySuite", selection.Verdict)
		}
		if !strings.Contains(selection.Reason, changedPackage) || !strings.Contains(selection.Reason, head) {
			t.Errorf("the reason is %q, and it must name the package and the commit the map was recorded at",
				selection.Reason)
		}
	})

	t.Run("a commit touching another package leaves the answer narrow", func(t *testing.T) {
		root, head := gitCheckout(t)
		recordedMap(t, root, head, everySuiteReaching([]string{suiteUi}, silent))
		changeSetIs(t, changedPackage)

		writeCheckoutFile(t, root, "internal/component/cli/cli.go", "package cli\n\nfunc Prompt() {}\n")
		commitCheckout(t, root, "another package moved")

		selection := selectSuites(root)

		if selection.Verdict != verdictSelected {
			t.Fatalf("verdict %d with reason %q, want verdictSelected", selection.Verdict, selection.Reason)
		}
		if selection.runs(suiteEncode) {
			t.Error("the encode suite runs, and the map records it as reaching no changed package")
		}
	})

	t.Run("a commit the checkout no longer holds widens", func(t *testing.T) {
		root, _ := gitCheckout(t)
		recordedMap(t, root, "0000000000000000000000000000000000000000",
			everySuiteReaching([]string{suiteUi}, silent))
		changeSetIs(t, changedPackage)

		selection := selectSuites(root)

		if selection.Verdict != verdictEverySuite {
			t.Fatalf("verdict %d, want verdictEverySuite", selection.Verdict)
		}
		if !strings.Contains(selection.Reason, "cannot be read") {
			t.Errorf("the reason is %q, which does not say the commits could not be compared", selection.Reason)
		}
	})
}

// TestOperatorSkipStillWins drives the whole plan, which is what the gating run
// executes and what `le functional select` prints. An operator's skip outranks
// the map: a suite the map selected is still left out, and the map cannot add
// back a suite the operator removed.
//
// VALIDATES: spec-verify-scope-5-suite-coverage-map AC-8.
func TestOperatorSkipStillWins(t *testing.T) {
	root, head := gitCheckout(t)
	recordedMap(t, root, head,
		everySuiteReaching([]string{suiteUi, suiteParse},
			[]string{suiteEditor, suiteWeb, suiteRunner, suitePolicy}))
	changeSetIs(t, changedPackage)
	nameForTest(t, "ze.skip.suites", suiteUi)

	plan, err := planRun(root)
	if err != nil {
		t.Fatalf("plan the run: %v", err)
	}

	if slices.Contains(plan.Report.Running, suiteUi) {
		t.Errorf("the run list is %v, and the operator skipped %s", plan.Report.Running, suiteUi)
	}
	if !slices.Equal(plan.Report.Skipped, []string{suiteUi}) {
		t.Errorf("the operator skips are %v, want %s alone", plan.Report.Skipped, suiteUi)
	}
	if !slices.Contains(plan.Report.Running, suiteParse) {
		t.Errorf("the run list is %v, and the map records %s as reaching the changed package",
			plan.Report.Running, suiteParse)
	}
	if !plan.Report.Narrowed {
		t.Error("the report does not say the run narrowed")
	}
	// The three states are one population: every gating suite is in exactly one
	// of them, so a suite cannot vanish from the plan unnoticed.
	if total := len(plan.Report.Running) + len(plan.Report.Skipped) + len(plan.Report.RuledOut); total != len(Gating) {
		t.Errorf("the plan accounts for %d suite(s), want all %d", total, len(Gating))
	}
	if !slices.Contains(plan.Report.RuledOut, suiteEncode) {
		t.Errorf("the ruled-out suites are %v, and the map records %s as reaching no changed package",
			plan.Report.RuledOut, suiteEncode)
	}
	// The rendering is what the operator reads, and both absences are named.
	text := plan.Report.Text()
	for _, want := range []string{"the suite map rules out", "ZE_SKIP_SUITES leaves out " + strconv.Itoa(1)} {
		if !strings.Contains(text, want) {
			t.Errorf("the printed plan is %q, and it does not say %q", text, want)
		}
	}
}
