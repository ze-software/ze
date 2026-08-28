// Design: docs/architecture/core-design.md -- the completeness gate over the migration
// Detail: completeness_record_test.go -- the ported and retired record this reads
//
// VALIDATES: AC-6 and AC-9. Every first-party producer of the retired Make
// build has one reachable native le action, or a recorded reason why it has
// none.
//
// PREVENTS: a completeness claim that reads one package's action table. The
// test that carried this name before pinned four verbs of internal/le/sourcerewrite
// and could not go red for a producer missing anywhere else in the repository.
//
// Both populations are DERIVED, and neither is written down here. The producer
// population is parsed from the Makefile and mk/ fragments as git holds them at
// the revision below, because the working tree has deleted them. The action
// population is read from the live command registry, through the same
// leroot.Commands() that answers `le` and the same Meta.ResolveSubs() that
// answers `le <area>`. This file joins the two and reports the difference.
//
// The gate FAILS CLOSED. A Make text it cannot read, a fragment count under the
// floor, a producer population under the floor, and an empty or short registry
// each fail the run. A completeness gate that reports a clean population over a
// tree it never read is the failure it exists to prevent, turned on itself.

package le

import (
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leroot"
)

// historicalMakeRevision is the last commit that holds the Makefile and mk/.
// The working tree deleted both, so the producer population can only be read
// from history. The revision is pinned rather than resolved from HEAD: HEAD
// moves past the deletion, and a population that silently empties as soon as
// the deletion lands would make this gate green by forgetting what it judges.
const historicalMakeRevision = "70ac6766944fdd3241706305e27197cdfe90811e"

// The floors. Each states a count measured on 2026-08-28 against the pinned
// revision and the live registry. They are floors rather than equalities
// because the registry grows; the Make text cannot, since it is history.
const (
	makeFragmentFloor = 19  // mk/*.mk files at the pinned revision
	producerFloor     = 290 // named targets derived from that text
	areaFloor         = 80  // le areas in the live registry
	actionWordFloor   = 200 // action words those areas declare
)

// targetPattern matches a Make rule line and captures its target. A variable
// assignment (`NAME := value`) is excluded by requiring that the character
// after the colon is not an equals sign, and a double-colon rule ends the name
// at the first colon either way.
var targetPattern = regexp.MustCompile(`^([^\s:=#]+)[ \t]*:($|[^=])`)

// verbPattern matches an action word in a Subs hint: a bare lower-case token.
var verbPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// gitShow reads one path at the pinned revision. A failure is fatal: it means
// this gate cannot see the population it judges.
func gitShow(t *testing.T, root, path string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", "show", historicalMakeRevision+":"+path)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("read %s at %s: %v; the producer population cannot be derived without it", path, historicalMakeRevision, err)
	}
	if len(out) == 0 {
		t.Fatalf("%s at %s is empty", path, historicalMakeRevision)
	}
	return string(out)
}

// historicalMakeText answers the Makefile and every mk/ fragment at the pinned
// revision, as one string per file.
func historicalMakeText(t *testing.T, root string) []string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", "ls-tree", "-r", "--name-only", historicalMakeRevision, "mk/")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list mk/ at %s: %v", historicalMakeRevision, err)
	}

	fragments := make([]string, 0, makeFragmentFloor)
	for path := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if strings.HasSuffix(path, ".mk") {
			fragments = append(fragments, path)
		}
	}
	if len(fragments) < makeFragmentFloor {
		t.Fatalf("mk/ holds %d fragments at %s, want at least %d", len(fragments), historicalMakeRevision, makeFragmentFloor)
	}

	texts := make([]string, 0, len(fragments)+1)
	texts = append(texts, gitShow(t, root, "Makefile"))
	for _, path := range fragments {
		texts = append(texts, gitShow(t, root, path))
	}
	return texts
}

// derivedProducers answers every named job the retired build declared.
//
// Three kinds of rule are excluded, each by a derived property rather than by
// name. A target starting with a dot is a Make directive (.PHONY). A target
// holding a slash or a variable reference is a FILE rule, whose job is reached
// through the named target that depends on it. A target starting with an
// underscore is the inner half of an admission pair, where the outer target
// takes the job slot and re-enters Make: it is the same job, and the caller
// verifies that its outer half is in the population rather than assuming so.
func derivedProducers(t *testing.T, texts []string) []string {
	t.Helper()

	seen := make(map[string]bool, producerFloor)
	inner := make(map[string]bool)
	for _, text := range texts {
		for line := range strings.SplitSeq(text, "\n") {
			match := targetPattern.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			target := match[1]
			switch {
			case strings.HasPrefix(target, "."):
				continue
			case strings.ContainsAny(target, "/$()%"):
				continue
			case strings.HasPrefix(target, "_"):
				inner[target] = true
			default:
				seen[target] = true
			}
		}
	}

	// An inner half whose outer target is absent would be a job this gate
	// never sees, so the exclusion is checked rather than trusted.
	for target := range inner {
		outer := strings.TrimSuffix(strings.TrimPrefix(target, "_"), "-impl")
		if !seen[outer] {
			t.Errorf("admission inner half %s has no outer target %s in the population", target, outer)
		}
	}

	producers := make([]string, 0, len(seen))
	for target := range seen {
		producers = append(producers, target)
	}
	slices.Sort(producers)
	return producers
}

// areaVerbs is one live area: the words a developer can type after it, and
// whether that list is a complete table or a shape hint.
//
// An area backed by leaction publishes every verb in its Subs line, so the
// list is authoritative and a record naming an absent verb is drift. An area
// whose grammar is keywords and values (`le job run label <label> command
// <argv...>`) publishes a usage line instead, and its words are not a verb
// table. The distinction is derived from the text: a table is a bar-separated
// list of bare words, each optionally marked as writing.
type areaVerbs struct {
	verbs []string
	table bool
}

// liveActions answers every le area and its action words, read from the same
// registry entries that `le` and `le <area>` print.
func liveActions(t *testing.T) map[string]areaVerbs {
	t.Helper()

	commands := leroot.Commands()
	if len(commands) < areaFloor {
		t.Fatalf("the live registry holds %d le areas, want at least %d", len(commands), areaFloor)
	}

	words := 0
	actions := make(map[string]areaVerbs, len(commands))
	for _, command := range commands {
		subs := strings.TrimSpace(command.Meta.ResolveSubs())
		if subs == "" {
			actions[command.Name] = areaVerbs{}
			continue
		}

		parts := strings.Split(subs, "|")
		entry := areaVerbs{verbs: make([]string, 0, len(parts)), table: true}
		for _, part := range parts {
			verb := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(part), "(writes)"))
			if !verbPattern.MatchString(verb) {
				entry.table = false
				verb, _, _ = strings.Cut(verb, " ")
			}
			if verbPattern.MatchString(verb) {
				entry.verbs = append(entry.verbs, verb)
			}
		}
		words += len(entry.verbs)
		actions[command.Name] = entry
	}

	if words < actionWordFloor {
		t.Fatalf("the live registry declares %d action words, want at least %d", words, actionWordFloor)
	}
	return actions
}

// TestEveryMakeProducerHasAReachableNativeAction is the completeness gate for
// AC-6 and AC-9.
func TestEveryMakeProducerHasAReachableNativeAction(t *testing.T) {
	root := checkoutRoot(t)

	producers := derivedProducers(t, historicalMakeText(t, root))
	if len(producers) < producerFloor {
		t.Fatalf("derived %d producers from the Make text at %s, want at least %d", len(producers), historicalMakeRevision, producerFloor)
	}

	actions := liveActions(t)

	accounted := make(map[string]bool, len(producers))
	for _, ported := range portedProducers {
		if accounted[ported.Target] {
			t.Errorf("%s is recorded twice", ported.Target)
		}
		accounted[ported.Target] = true

		area, registered := actions[ported.Area]
		if !registered {
			t.Errorf("%s is recorded at le %s, which the registry does not hold", ported.Target, ported.Area)
			continue
		}
		switch {
		case len(area.verbs) == 0 && ported.Verb != "":
			t.Errorf("%s is recorded at le %s %s, and that area declares no action word", ported.Target, ported.Area, ported.Verb)
		case area.table && ported.Verb == "":
			t.Errorf("%s is recorded at the bare le %s, and that area is a table of actions: name one", ported.Target, ported.Area)
		case area.table && !slices.Contains(area.verbs, ported.Verb):
			t.Errorf("%s is recorded at le %s %s, and that area declares %v", ported.Target, ported.Area, ported.Verb, area.verbs)
		}
	}
	for _, retired := range retiredProducers {
		if accounted[retired.Target] {
			t.Errorf("%s is recorded twice", retired.Target)
		}
		accounted[retired.Target] = true
		if len(retired.Reason) < 40 {
			t.Errorf("%s is retired with no reason a reader can judge: %q", retired.Target, retired.Reason)
		}
	}

	inPopulation := make(map[string]bool, len(producers))
	for _, producer := range producers {
		inPopulation[producer] = true
	}
	for target := range accounted {
		if !inPopulation[target] {
			t.Errorf("the record names %s, which is not a target of the retired build", target)
		}
	}

	var missing []string
	for _, producer := range producers {
		if !accounted[producer] {
			missing = append(missing, producer)
		}
	}
	if len(missing) != 0 {
		t.Errorf("%d of %d first-party producers have no native action and no recorded retirement:\n%s",
			len(missing), len(producers), strings.Join(missing, "\n"))
	}
}
