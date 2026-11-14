// Design: docs/contributing/terminal-demos.md -- one authored fragment per topic, shared by its recording and the showcase
// Related: validate_showcase_topics.go -- showcaseTopicOrder, the runtime half of the same contract
// Related: register_topics.go -- registerLiveTopic, which binds one check to both validators

package siteterminaldemo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// topicFragmentPrefix is the folder, relative to the demo tree, that holds the
// fragments a topic recording and the showcase share.
const topicFragmentPrefix = "topics/"

// directTopicSources returns the topic fragments the tape at tapePath names in
// its own Source lines, in order, one entry per line so a repeat is visible.
func directTopicSources(t *testing.T, tapePath string) []string {
	t.Helper()
	data, err := os.ReadFile(tapePath)
	if err != nil {
		t.Fatalf("read %s: %v", tapePath, err)
	}
	var fragments []string
	for line := range strings.SplitSeq(string(data), "\n") {
		word, argument, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found {
			continue
		}
		if word != tapeSourceCommand {
			continue
		}
		name := filepath.ToSlash(filepath.Clean(strings.TrimSpace(argument)))
		if strings.HasPrefix(name, topicFragmentPrefix) {
			fragments = append(fragments, name)
		}
	}
	return fragments
}

// VALIDATES: spec-terminal-demo-showcase AC-7. Every tape fragment under
// demos/terminal/topics is sourced exactly once by one topic recording and
// exactly once by the showcase, and every recording that sources one resolves
// to a scenario registered as a live topic that holds a check and registers a
// fragment its own tape sources. The test does not observe either validator
// calling that check. It need not: registerLiveTopic is the only constructor
// of a liveTopic, and in one call it hands the same check to the topic's
// validator (validateLiveTopic) and to the live topic the showcase runs, so a
// registered live topic has no second check.
// PREVENTS: a topic's typed config or demonstration written a second time in a
// tape, a fragment no recording shows, and a chapter the showcase drops.
// Method: read the Source lines of every demos/terminal/*/demo.tape, count the
// tapes that name each fragment, then look each sourcing demo up in the
// scenario registry through its manifest `validate` id.
func TestTopicFragmentIsSourcedByItsTopicAndTheShowcase(t *testing.T) {
	const showcaseID = "showcase"
	fragmentPaths, err := filepath.Glob(filepath.Join(repoDemoRoot, "topics", "*", "*.tape"))
	if err != nil {
		t.Fatal(err)
	}
	// An empty glob means the path no longer reaches the demo tree, and a loop
	// over nothing would pass in silence.
	if len(fragmentPaths) == 0 {
		t.Fatalf("no topic fragment under %s", filepath.Join(repoDemoRoot, "topics"))
	}
	tapePaths, err := filepath.Glob(filepath.Join(repoDemoRoot, "*", "demo.tape"))
	if err != nil {
		t.Fatal(err)
	}

	sourcedBy := make(map[string][]string)
	sourcesOf := make(map[string][]string)
	for _, tapePath := range tapePaths {
		id := filepath.Base(filepath.Dir(tapePath))
		sourcesOf[id] = directTopicSources(t, tapePath)
		for _, fragment := range sourcesOf[id] {
			sourcedBy[fragment] = append(sourcedBy[fragment], id)
		}
	}

	for _, fragmentPath := range fragmentPaths {
		relative, err := filepath.Rel(repoDemoRoot, fragmentPath)
		if err != nil {
			t.Fatal(err)
		}
		fragment := filepath.ToSlash(relative)
		showcaseCount := 0
		var topics []string
		for _, id := range sourcedBy[fragment] {
			if id == showcaseID {
				showcaseCount++
				continue
			}
			topics = append(topics, id)
		}
		if showcaseCount != 1 {
			t.Errorf("%s: the showcase tape sources it %d times, want exactly once", fragment, showcaseCount)
		}
		if len(topics) != 1 {
			t.Errorf("%s: sourced by topic recordings %q, want exactly one recording sourcing it once", fragment, topics)
		}
	}

	data, err := os.ReadFile(filepath.Join(repoDemoRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	liveTopics := 0
	for _, demo := range manifest.Demos {
		if demo.ID == showcaseID {
			continue
		}
		fragments := sourcesOf[demo.ID]
		if len(fragments) == 0 {
			continue
		}
		sc, err := scenarios.lookup(demo.Validate)
		if err != nil {
			t.Errorf("demo %s: %v", demo.ID, err)
			continue
		}
		if sc.live == nil {
			t.Errorf("demo %s sources %q and its scenario %s is not a live topic, so the showcase cannot call its check", demo.ID, fragments, demo.Validate)
			continue
		}
		if sc.live.check == nil {
			t.Errorf("demo %s: live topic %s holds no check", demo.ID, demo.Validate)
		}
		liveTopics++
		registered := filepath.ToSlash(filepath.Clean(sc.live.fragment))
		held := false
		for _, fragment := range fragments {
			if fragment == registered {
				held = true
			}
		}
		if !held {
			t.Errorf("demo %s: its live topic registers fragment %s, which its own tape does not source (%q)", demo.ID, registered, fragments)
		}
	}
	// Every sourcing tape skipped would leave the second half proving nothing.
	if liveTopics == 0 {
		t.Fatal("no manifest demo sources a topic fragment")
	}
}
