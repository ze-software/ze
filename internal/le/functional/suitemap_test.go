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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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

// TestAValidSuiteMapIsReadBack proves the format round-trips, and records what
// this phase deliberately does not do: a well-formed map still runs every
// suite, because no change set is compared against it yet.
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

	selection := selectSuites(root)
	if selection.Verdict != verdictEverySuite {
		t.Fatalf("verdict %d, want verdictEverySuite: nothing narrows yet", selection.Verdict)
	}
	if !strings.Contains(selection.Reason, "447ba80f16") {
		t.Errorf("the reason is %q, which does not name the commit it read", selection.Reason)
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

// suiteNames renders a suite list for a failure message.
func suiteNames(suites []Suite) []string {
	names := make([]string, 0, len(suites))
	for _, suite := range suites {
		names = append(names, suite.Name)
	}
	return names
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
