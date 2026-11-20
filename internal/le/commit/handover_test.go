// VALIDATES: handoverRefusal refuses a commit that removes a tracked handover
// tied to a spec set DeriveSets answers live, stalled or cannot tell, or named
// for the whole stem of any open spec, a member of an idea set included, and
// names the set's state, its open children,
// what settles a cannot tell and the keyword to drop. It removes freely the
// handover of an idea set (a `-0-umbrella` one or a number-less parent), of a
// set whose children all closed, of a closed spec, of a one-token spec it only
// shares a topic word with, and of the last open child the same commit closes.
// The claims come from an injected reader, so no case reads the host's
// process table, and each claim verdict is asserted by the state it prints.
// PREVENTS: 935398f8df again: three closed children made the agent-economy set
// look finished, and a cleanup removed the one handover naming its next child.
package commit

import (
	"context"
	"errors"
	"strings"
	"testing"

	lesession "github.com/ze-software/ze/internal/le/session"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

const handoverSetPrefix = "agent-economy"

// handoverFixtureSpec is a spec carrying status, as specstatus.Parse reads one.
func handoverFixtureSpec(status string) string {
	return "# Spec: fixture\n\n| Field | Value |\n|---|---|\n| Status | " + status + " |\n"
}

// handoverSpecPath is the engineering-bucket path of the spec named stem.
func handoverSpecPath(stem string) string {
	return "plan/post-release/engineering/spec-" + stem + ".md"
}

// handoverBaseline is a committed checkout holding the handover at handover
// and each spec of specs (stem to Status).
func handoverBaseline(t *testing.T, handover string, specs map[string]string) string {
	t.Helper()
	root := newCommitRepository(t)
	writeCommitFixture(t, root, handover, "# Handover\n\nNext: claim the next child.\n")
	runCommitGit(t, root, "add", "--", handover)
	for stem, status := range specs {
		path := handoverSpecPath(stem)
		writeCommitFixture(t, root, path, handoverFixtureSpec(status))
		runCommitGit(t, root, "add", "--", path)
	}
	handoverFixtureCommit(t, root, "handover baseline")
	return root
}

// handoverFixtureCommit commits what the fixture index holds.
func handoverFixtureCommit(t *testing.T, root, subject string) {
	t.Helper()
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", subject)
}

// useHandoverClaims makes the gate read claims from claims, and the record of
// closed specs from the fixture history, for the rest of the test.
func useHandoverClaims(t *testing.T, claims func(string) ([]lesession.Claim, error)) {
	t.Helper()
	saved := liveHandoverReaders
	t.Cleanup(func() { liveHandoverReaders = saved })
	liveHandoverReaders = handoverReaders{closed: specstatus.LoadClosedSpecs, claims: claims}
}

// noHandoverClaims finds no claim marker.
func noHandoverClaims(string) ([]lesession.Claim, error) { return nil, nil }

// handoverClaimOn answers a reader holding one claim on spec, with verdict.
func handoverClaimOn(spec string, verdict lesession.Liveness) func(string) ([]lesession.Claim, error) {
	return func(string) ([]lesession.Claim, error) {
		return []lesession.Claim{{Marker: "tmp/session/fixture-spec", Spec: spec, Session: "fixture",
			Liveness: verdict, Reason: "fixture verdict " + verdict.String()}}, nil
	}
}

// TestAHandoverOfALiveSetCannotBeRemoved removes the handover of a started set
// with open children under each claim verdict, then the handover named for an
// open spec outside any set: each is refused, naming the state DeriveSets
// answered, what is still open and the keyword to drop.
func TestAHandoverOfALiveSetCannotBeRemoved(t *testing.T) {
	handover := "plan/handover/2026-11-14-" + handoverSetPrefix + ".md"
	open := handoverSetPrefix + "-4-supervision"
	cases := []struct {
		name   string
		claims func(string) ([]lesession.Claim, error)
		want   []string
	}{
		{"live", handoverClaimOn("spec-"+open+".md", lesession.LivenessLive), []string{"set " + handoverSetPrefix + " (live), open children:"}},
		{"stalled", noHandoverClaims, []string{"set " + handoverSetPrefix + " (stalled), open children:"}},
		{"dead holder", handoverClaimOn("spec-"+open+".md", lesession.LivenessDead), []string{"set " + handoverSetPrefix + " (stalled), open children:"}},
		{"claims unread", func(string) ([]lesession.Claim, error) { return nil, errors.New("fixture: no process table") },
			[]string{"set " + handoverSetPrefix + " (cannot tell): ", "fixture: no process table",
				"to tell its state: repair what the error names, then run the same ./le commit create line again"}},
		{"holder cannot be told", handoverClaimOn("spec-"+open+".md", lesession.LivenessCannotTell),
			[]string{"set " + handoverSetPrefix + " (cannot tell): ", "to tell its state: wait for the holder"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useHandoverClaims(t, tc.claims)
			root := handoverBaseline(t, handover, map[string]string{
				handoverSetPrefix + "-0-umbrella": "in-progress",
				open:                              "in-progress",
				handoverSetPrefix + "-5-limits":   "skeleton",
			})
			err := handoverRefusal(root, nil, []string{handover}, nil, "HEAD")
			if err == nil {
				t.Fatal("a commit removing the handover of a set with open children was admitted")
			}
			for _, want := range append([]string{
				handover,
				handoverSpecPath(open) + " (in-progress)",
				handoverSpecPath(handoverSetPrefix+"-5-limits") + " (skeleton)",
				"./le commit create",
			}, tc.want...) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal lacks %q:\n%v", want, err)
				}
			}
			lines := strings.Split(strings.TrimRight(err.Error(), "\n"), "\n")
			if last := strings.TrimSpace(lines[len(lines)-1]); last != "remove "+handover {
				t.Errorf("last line = %q, want the keyword to drop, alone: %q", last, "remove "+handover)
			}
			if strings.Contains(err.Error(), handoverSpecPath(handoverSetPrefix+"-0-umbrella")) {
				t.Errorf("refusal lists the umbrella as an open child:\n%v", err)
			}
		})
	}

	useHandoverClaims(t, noHandoverClaims)
	lone := "plan/handover/53-ppp-ncp-independent-lifetime.md"
	root := handoverBaseline(t, lone, map[string]string{"ppp-ncp-independent-lifetime": "ready"})
	err := handoverRefusal(root, nil, []string{lone}, nil, "HEAD")
	if err == nil {
		t.Fatal("a commit removing the handover of an open spec was admitted")
	}
	if !strings.Contains(err.Error(), "spec "+handoverSpecPath("ppp-ncp-independent-lifetime")+" (ready)") {
		t.Errorf("refusal does not name the open spec:\n%v", err)
	}
}

// TestAHandoverNamedForAnOpenChildOfAnIdeaSetIsKept removes, from an idea set
// none of whose children started, first a handover named for the full stem of
// one open child, then the set's own prefix-only handover.
//
// VALIDATES: AC-18 "the stem of an open spec": the first is refused, naming
// that child, though the set's idea verdict lets its prefix go; the second
// passes, under a `-0-umbrella` and under a number-less parent whose stem is
// the prefix itself.
// PREVENTS: a set member judged by its set's state alone, which let the
// handover of an open child of an idea set be removed.
func TestAHandoverNamedForAnOpenChildOfAnIdeaSetIsKept(t *testing.T) {
	useHandoverClaims(t, noHandoverClaims)
	child := handoverSetPrefix + "-2-second"
	for _, umbrella := range []string{handoverSetPrefix + "-0-umbrella", handoverSetPrefix} {
		t.Run(umbrella, func(t *testing.T) {
			specs := map[string]string{
				umbrella:                       "skeleton",
				handoverSetPrefix + "-1-first": "ready",
				child:                          "skeleton",
			}
			named := "plan/handover/2026-11-14-" + child + ".md"
			root := handoverBaseline(t, named, specs)
			err := handoverRefusal(root, nil, []string{named}, nil, "HEAD")
			if err == nil {
				t.Fatal("the handover named for an open child of an idea set was removed")
			}
			if !strings.Contains(err.Error(), "spec "+handoverSpecPath(child)+" (skeleton)") {
				t.Errorf("refusal does not name the open child:\n%v", err)
			}
			if strings.Contains(err.Error(), "set "+handoverSetPrefix+" (") {
				t.Errorf("refusal names the idea set as holding its handover:\n%v", err)
			}
			prefixOnly := "plan/handover/2026-11-14-" + handoverSetPrefix + ".md"
			root = handoverBaseline(t, prefixOnly, specs)
			if err := handoverRefusal(root, nil, []string{prefixOnly}, nil, "HEAD"); err != nil {
				t.Fatalf("the idea set's own prefix-only handover was refused:\n%v", err)
			}
		})
	}
}

// TestARecordThatCannotBeExtendedIsNeverReadAsTheStaleFile computes the
// record of closed specs, then closes the set's only child in a later commit,
// and makes the walk that would extend the record fail.
//
// VALIDATES: the gate reads the record as not computed, so the set is cannot
// tell and the refusal names `./le spec status set list`, the command that
// computes it.
// PREVENTS: the record as the file holds it, Known at the older commit,
// answering a set whose child closed after it as an idea (that child is in
// neither its open nor its closed children), and the handover being removed.
func TestARecordThatCannotBeExtendedIsNeverReadAsTheStaleFile(t *testing.T) {
	handover := "plan/handover/2026-11-14-" + handoverSetPrefix + ".md"
	child := handoverSetPrefix + "-1-first"
	root := handoverBaseline(t, handover, map[string]string{
		handoverSetPrefix + "-0-umbrella": "in-progress",
		child:                             "in-progress",
	})
	if _, err := specstatus.LoadClosedSpecs(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	runCommitGit(t, root, "rm", "-q", "--", handoverSpecPath(child))
	handoverFixtureCommit(t, root, "close the only child")
	if stale := specstatus.ReadClosedSpecs(root); !stale.Known() {
		t.Fatalf("the fixture holds no stale record, so the case proves nothing: %s", stale.Unknown())
	}
	saved := liveHandoverReaders
	t.Cleanup(func() { liveHandoverReaders = saved })
	liveHandoverReaders = handoverReaders{
		closed: func(context.Context, string) (specstatus.ClosedSpecs, error) {
			return specstatus.ClosedSpecs{}, errors.New("fixture: the history walk failed")
		},
		claims: noHandoverClaims,
	}
	err := handoverRefusal(root, nil, []string{handover}, nil, "HEAD")
	if err == nil {
		t.Fatal("the handover of a set whose closed children cannot be told was removed")
	}
	for _, want := range []string{
		"set " + handoverSetPrefix + " (cannot tell): ",
		"not computed: fixture: the history walk failed",
		"to tell its state: run `./le spec status set list`",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q:\n%v", want, err)
		}
	}
}

// TestCreateRefusesTheRemovalOfAHandoverOfALiveSet drives the gate through
// `./le commit create`'s own entry point: the handover of a set with an open
// child is refused, and once that child is closed in the same commit the
// removal is prepared.
func TestCreateRefusesTheRemovalOfAHandoverOfALiveSet(t *testing.T) {
	useHandoverClaims(t, noHandoverClaims)
	handover := "plan/handover/2026-11-14-" + handoverSetPrefix + ".md"
	open := handoverSetPrefix + "-4-supervision"
	root := handoverBaseline(t, handover, map[string]string{
		handoverSetPrefix + "-0-umbrella": "in-progress",
		open:                              "in-progress",
	})
	t.Setenv("CLAUDE_CODE_SESSION_ID", "handover-removal-fixture")
	_, err := Create(root, &Options{Subject: "plan: remove the agent-economy handover", Remove: []string{handover}})
	if err == nil {
		t.Fatal("Create prepared the removal of the handover of a set with an open child")
	}
	if !strings.Contains(err.Error(), "the handover of work that is still open") {
		t.Fatalf("Create refused for another reason than the open set:\n%v", err)
	}
	if !strings.Contains(err.Error(), "set "+handoverSetPrefix+" (stalled)") {
		t.Fatalf("Create's refusal does not name the set as stalled:\n%v", err)
	}
	if !strings.Contains(err.Error(), handoverSpecPath(open)) {
		t.Fatalf("Create's refusal does not name the open child:\n%v", err)
	}
	if err := handoverRefusal(root, nil, []string{handoverSpecPath(open), handover}, nil, "HEAD"); err != nil {
		t.Fatalf("closing the last open child with the handover was refused:\n%v", err)
	}
}

// TestAHandoverOfASetWhoseStateCannotBeToldIsKept holds the refusal to the
// state DeriveSets answers: cannot tell refuses with its reason and its fix,
// never a pass, while idea and children closed pass.
func TestAHandoverOfASetWhoseStateCannotBeToldIsKept(t *testing.T) {
	handover := "plan/handover/2026-11-14-" + handoverSetPrefix + ".md"
	why := "whether anyone holds it cannot be told: the claims could not be read"
	fix := "repair what the error names, then run the same ./le commit create line again"
	set := specstatus.SetView{Prefix: handoverSetPrefix, State: specstatus.SetCannotTell, Why: why, Fix: fix,
		Handovers: []string{handover},
		Open:      []specstatus.SetOpenChild{{Number: 4, Path: handoverSpecPath(handoverSetPrefix + "-4-supervision"), Status: "in-progress"}}}
	text := handoverProblem(handover, nil, []specstatus.SetView{set})
	for _, want := range []string{"set " + handoverSetPrefix + " (cannot tell): " + why, "to tell its state: " + fix,
		"settle it as its line says, then run the same ./le commit create line again"} {
		if !strings.Contains(text, want) {
			t.Fatalf("cannot tell did not refuse with %q:\n%s", want, text)
		}
	}
	set.Why, set.Fix = "", ""
	for _, state := range []specstatus.SetState{specstatus.SetIdea, specstatus.SetChildrenClosed} {
		set.State = state
		if text := handoverProblem(handover, nil, []specstatus.SetView{set}); text != "" {
			t.Errorf("state %s refused the removal:\n%s", state, text)
		}
	}
	for _, state := range []specstatus.SetState{specstatus.SetLive, specstatus.SetStalled} {
		set.State = state
		text := handoverProblem(handover, nil, []specstatus.SetView{set})
		if !strings.Contains(text, "("+state.String()+"), open children:") {
			t.Errorf("state %s admitted the removal:\n%s", state, text)
		}
		if strings.Contains(text, "settle it") {
			t.Errorf("state %s printed the cannot tell fix:\n%s", state, text)
		}
	}
}

// TestAHandoverOfAClosedSetIsRemoved removes the handover of a set whose
// children all closed (its umbrella alone stays), of an idea set with a
// `-0-umbrella` and of one with a number-less parent, of a closed spec, of a
// one-token spec whose name is only a topic word of the handover, of the set
// whose last open child the same commit closes, and of a name that only
// shares part of a token with an open spec: each passes.
func TestAHandoverOfAClosedSetIsRemoved(t *testing.T) {
	useHandoverClaims(t, noHandoverClaims)
	handover := "plan/handover/2026-11-14-" + handoverSetPrefix + ".md"
	root := handoverBaseline(t, handover, map[string]string{
		handoverSetPrefix + "-0-umbrella": "in-progress",
		"agent-econ":                      "ready",
	})
	if err := handoverRefusal(root, nil, []string{handover}, nil, "HEAD"); err != nil {
		t.Fatalf("the handover of a set whose children all closed was refused:\n%v", err)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "handover-closed-set-fixture")
	if _, err := Create(root, &Options{Subject: "plan: remove the agent-economy handover", Remove: []string{handover}}); err != nil {
		t.Fatalf("Create refused the removal of the handover of a set whose children all closed:\n%v", err)
	}

	idea := handoverBaseline(t, handover, map[string]string{
		handoverSetPrefix + "-0-umbrella": "skeleton",
		handoverSetPrefix + "-1-first":    "ready",
		handoverSetPrefix + "-2-second":   "skeleton",
	})
	if err := handoverRefusal(idea, nil, []string{handover}, nil, "HEAD"); err != nil {
		t.Fatalf("the handover of an idea set, none of whose children started, was refused:\n%v", err)
	}

	// The number-less parent `spec-agent-economy.md` is the set's umbrella,
	// and its name is the prefix itself: the set's idea verdict decides, not
	// the spec loop.
	parent := handoverBaseline(t, handover, map[string]string{
		handoverSetPrefix:               "design",
		handoverSetPrefix + "-1-first":  "ready",
		handoverSetPrefix + "-2-second": "skeleton",
	})
	if err := handoverRefusal(parent, nil, []string{handover}, nil, "HEAD"); err != nil {
		t.Fatalf("the handover of an idea set under a number-less parent was refused:\n%v", err)
	}

	closed := "plan/handover/40-fetched-data-survives-restart.md"
	root = handoverBaseline(t, closed, map[string]string{"unrelated-work": "ready"})
	if err := handoverRefusal(root, nil, []string{closed}, nil, "HEAD"); err != nil {
		t.Fatalf("the handover of a closed spec was refused:\n%v", err)
	}

	topic := "plan/handover/2026-11-20-vrf-leak.md"
	root = handoverBaseline(t, topic, map[string]string{"vrf": "in-progress"})
	if err := handoverRefusal(root, nil, []string{topic}, nil, "HEAD"); err != nil {
		t.Fatalf("a one-token spec tied a handover that only shares its topic word:\n%v", err)
	}

	last := handoverSetPrefix + "-4-supervision"
	root = handoverBaseline(t, handover, map[string]string{
		handoverSetPrefix + "-0-umbrella": "in-progress",
		last:                              "in-progress",
	})
	if err := handoverRefusal(root, nil, []string{handoverSpecPath(last), handover}, nil, "HEAD"); err != nil {
		t.Fatalf("the closure of the last open child with its set's handover was refused:\n%v", err)
	}
	if err := handoverRefusal(root, nil, []string{handoverSpecPath(last)}, nil, "HEAD"); err != nil {
		t.Fatalf("a commit removing no handover was refused:\n%v", err)
	}
}
