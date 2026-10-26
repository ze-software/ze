package rfc

// VALIDATES: the `profile` value of `./le rfc tree` and `./le rfc next`
// completes to every rfc/profiles/ name, use cases and aggregates, and an
// unknown profile is refused with an example naming the verb the user ran.
// PREVENTS: a parameter whose valid values a reader must guess, a refusal
// that tells a `next` user to run `tree`, and a gap that prints a command
// le does not register.

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestProfileParameterCompletesEveryProfile holds the completion source to
// the directory scope reads. Method: the real checkout's rfc/profiles/, read
// directly, against the candidates both actions' `profile` parameter offers.
func TestProfileParameterCompletesEveryProfile(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	entries, err := os.ReadDir(treePath(root, profileRel))
	if err != nil {
		t.Fatalf("read %s: %v", profileRel, err)
	}
	var want []string
	for _, entry := range entries {
		if name, isProfile := strings.CutSuffix(entry.Name(), ".md"); isProfile && !entry.IsDir() {
			want = append(want, name)
		}
	}
	slices.Sort(want)
	if !slices.Contains(want, "route-server") {
		t.Fatalf("%s holds %v, without route-server", profileRel, want)
	}
	seen := map[string]bool{}
	for _, row := range actions.Actions().Actions {
		if row.Verb != "tree" && row.Verb != "next" {
			continue
		}
		for _, parameter := range row.Parameters {
			if parameter.Keyword != keyProfile {
				continue
			}
			if parameter.Source == nil {
				t.Fatalf("rfc %s: parameter %s has no completion source", row.Verb, keyProfile)
			}
			var got []string
			for _, candidate := range parameter.Source() {
				if candidate.Summary == "" {
					t.Errorf("rfc %s: candidate %s has no summary", row.Verb, candidate.Value)
				}
				got = append(got, candidate.Value)
			}
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("rfc %s profile completes %v, want %v", row.Verb, got, want)
			}
			seen[row.Verb] = true
		}
	}
	if !seen["tree"] || !seen["next"] {
		t.Errorf("profile parameter found on %v, want tree and next", seen)
	}
}

// TestUnknownProfileNamesTheVerbRun: the refusal's example is the action the
// user ran. Method: the priority fixture, asked for a profile it lacks.
func TestUnknownProfileNamesTheVerbRun(t *testing.T) {
	corpus := priorityFixture(t, nil)
	if _, err := corpus.next("nowhere"); err == nil || !strings.Contains(err.Error(), "./le rfc next profile ") {
		t.Errorf("next nowhere: %v, want an example naming ./le rfc next", err)
	}
	if _, err := corpus.tree("nowhere"); err == nil || !strings.Contains(err.Error(), "./le rfc tree profile ") {
		t.Errorf("tree nowhere: %v, want an example naming ./le rfc tree", err)
	}
}

// TestNotWalkedGapNamesARegisteredAction: the command a not-walked gap prints
// is an action the extraction sub-area registers, so renaming the verb turns
// this red rather than leaving `./le rfc next` printing a dead command.
// Method: one not-walked gap rendered, matched against ExtractionActions.
func TestNotWalkedGapNamesARegisteredAction(t *testing.T) {
	var tb textbuf.Buffer
	writeNextGap(&tb, &NextGap{Stem: "rfc1", Summary: true, Profiles: []string{"edge"}, Path: "edge: rfc1 (root)"})
	text := tb.String()
	list := ExtractionActions()
	for _, row := range list.Actions {
		if strings.Contains(text, "./le "+list.Area+" "+row.Verb+" "+keyStem+" rfc1") {
			return
		}
	}
	t.Errorf("not-walked gap prints no registered %s action:\n%s", list.Area, text)
}
