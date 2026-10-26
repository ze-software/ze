// VALIDATES: le's closed verb vocabulary over every registered action
// (AC-26, AC-52, D-Q9, D-P4-vocab of spec-le-cli-usability): every action word
// is a vocabulary verb or a name declaring the vocabulary verb it performs, a
// vocabulary verb never declares another, no name holds a verb, a retired verb
// or `test` as a hyphen part, every vocabulary entry is used, no two entries
// share a meaning, and only `ai hooks` freezes a word.
// PREVENTS: a synonym verb (`validate` beside `check`), a compound word that
// hides a second verb (`condensed-update`), and a vocabulary that lists every
// word and so proves nothing (R-20).
//
// Phase 4 lands its renames in batches. Until a batch lands, its rows are
// findings this tree still holds, listed in phase4Owed: a finding outside the
// list fails, and an entry that is no longer a finding fails too, so the list
// only shrinks and the last batch deletes it.
package le

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/aihelp/reference"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// The rule a vocabulary finding breaks, one per test below.
const (
	ruleUndeclaredName = "undeclared-name" // a word outside the vocabulary declares no Does
	ruleNotAVerb       = "not-a-verb"      // Does names a word outside the vocabulary
	ruleVerbDeclares   = "verb-declares"   // a vocabulary verb declares a different Does
	ruleEmbedsVerb     = "embeds-verb"     // a name holds a verb, a retired verb or `test`
	ruleUnusedVerb     = "unused-verb"     // a vocabulary entry no action performs
	ruleFrozenOutside  = "frozen-outside"  // a word frozen outside `ai hooks`
)

// frozenArea is the one area whose words an outside caller spells (D-P4-frozen).
const frozenArea = "ai hooks"

// vocabularyFindings judges every registered action against the vocabulary
// and answers one key per finding: "<rule> <area> <word>", or
// "unused-verb <verb>". The keys are sorted, so a failure lists them stably.
func vocabularyFindings(t *testing.T) []string {
	t.Helper()
	findings := map[string]bool{}
	used := map[string]bool{}
	for _, command := range leroot.Commands() {
		list, tabled := leroot.ActionsOf(command.Name)
		if !tabled {
			continue
		}
		for _, row := range list.Actions {
			key := command.Name + " " + row.Verb
			for _, rule := range judgeAction(command.Name, row) {
				findings[rule+" "+key] = true
			}
			if _, verb := leaction.VocabularyVerb(row.Verb); verb {
				used[row.Verb] = true
			}
			if row.Does != "" {
				used[row.Does] = true
			}
		}
	}
	for _, verb := range leaction.Vocabulary() {
		if !used[verb.Word] {
			findings[ruleUnusedVerb+" "+verb.Word] = true
		}
	}
	return slices.Sorted(maps.Keys(findings))
}

// judgeAction answers the rules one action word breaks.
func judgeAction(area string, row leaction.Row) []string {
	var broken []string
	if row.Frozen {
		if area != frozenArea {
			broken = append(broken, ruleFrozenOutside)
		}
		return broken
	}
	if row.Does != "" {
		if _, ok := leaction.VocabularyVerb(row.Does); !ok {
			broken = append(broken, ruleNotAVerb)
		}
	}
	if _, verb := leaction.VocabularyVerb(row.Verb); verb {
		if row.Does != "" && row.Does != row.Verb {
			broken = append(broken, ruleVerbDeclares)
		}
		return broken
	}
	if row.Verb == leaction.SharedName {
		return broken
	}
	if row.Does == "" {
		broken = append(broken, ruleUndeclaredName)
	}
	for part := range strings.SplitSeq(row.Verb, "-") {
		if leaction.ForbiddenNamePart(part) {
			broken = append(broken, ruleEmbedsVerb)
			break
		}
	}
	return broken
}

// requireRule fails for every finding of rule that phase4Owed does not list.
func requireRule(t *testing.T, rule string) {
	t.Helper()
	for _, finding := range vocabularyFindings(t) {
		if !strings.HasPrefix(finding, rule+" ") {
			continue
		}
		if _, owed := phase4Owed[finding]; owed {
			continue
		}
		t.Errorf("%s: see \"Verb vocabulary\" in docs/contributing/running-commands.md", finding)
	}
}

func TestEveryRegisteredVerbIsInTheVocabulary(t *testing.T) {
	requireRule(t, ruleNotAVerb)
}

func TestEveryActionWordIsAVerbOrADeclaringName(t *testing.T) {
	requireRule(t, ruleUndeclaredName)
	requireRule(t, ruleVerbDeclares)
}

func TestNoNameEmbedsAVerbOrTest(t *testing.T) {
	requireRule(t, ruleEmbedsVerb)
}

func TestEveryVocabularyVerbIsUsed(t *testing.T) {
	requireRule(t, ruleUnusedVerb)
}

func TestOnlyTheHookAreaFreezesAWord(t *testing.T) {
	requireRule(t, ruleFrozenOutside)
}

// TestNoTwoVocabularyVerbsShareAMeaning holds the vocabulary itself to one
// word per meaning: no two entries share a word or a meaning, and every entry
// names a kind.
func TestNoTwoVocabularyVerbsShareAMeaning(t *testing.T) {
	words := map[string]bool{}
	meanings := map[string]string{}
	for _, verb := range leaction.Vocabulary() {
		if words[verb.Word] {
			t.Errorf("the vocabulary lists %q twice", verb.Word)
		}
		words[verb.Word] = true
		if other, shared := meanings[verb.Meaning]; shared {
			t.Errorf("%q and %q share the meaning %q", other, verb.Word, verb.Meaning)
		}
		meanings[verb.Meaning] = verb.Word
		if verb.Kind == leaction.VerbKindUnspecified {
			t.Errorf("%q names no kind", verb.Word)
		}
		if strings.Contains(verb.Word, "-") {
			t.Errorf("%q is a compound word; a verb is one word", verb.Word)
		}
	}
}

// TestPhase4OwedListOnlyShrinks fails for an owed entry the tree no longer
// holds, so a batch that lands its rename deletes its rows in the same commit.
func TestPhase4OwedListOnlyShrinks(t *testing.T) {
	current := map[string]bool{}
	for _, finding := range vocabularyFindings(t) {
		current[finding] = true
	}
	for _, owed := range slices.Sorted(maps.Keys(phase4Owed)) {
		if !current[owed] {
			t.Errorf("%q is no longer a finding: delete it from phase4Owed", owed)
		}
	}
}

// phase4OwedFile is the work Phase 4 has still to land, one finding per line,
// a tab, then the rename or the declaration that clears it ("Phase 4 rename
// map" and D-P4-minimal in spec-le-cli-usability). It is data a batch deletes
// lines from, never adds to: TestPhase4OwedListOnlyShrinks refuses a stale
// line, and a new finding is refused by the rule tests above.
const phase4OwedFile = "testdata/vocabulary-owed.txt"

// phase4Owed maps each owed finding to what clears it.
var phase4Owed = func() map[string]string {
	text, err := os.ReadFile(phase4OwedFile)
	if err != nil {
		panic("BUG: read " + phase4OwedFile + ": " + err.Error())
	}
	owed := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(text)), "\n") {
		finding, clears, found := strings.Cut(line, "\t")
		if !found {
			panic("BUG: " + phase4OwedFile + " line has no tab before what clears it: " + line)
		}
		owed[finding] = clears
	}
	return owed
}()

// TestHelpAiVerbsRendersTheVocabulary proves `le help ai verbs` is the
// vocabulary itself (AC-26, D-P4-vocab): it lists every vocabulary verb in its
// declared order with its kind and meaning, and under each verb every
// registered name whose Does declares it, as JSON and as text.
// METHOD: dispatch `help ai verbs | json` and `help ai verbs` through le's
// own dispatcher, and compare with leaction.Vocabulary and every action table.
// PREVENTS: a verbs section that counts spellings rather than rendering the
// closed set, and a name that declares a verb the page does not show.
func TestHelpAiVerbsRendersTheVocabulary(t *testing.T) {
	out, code := dispatchToStdout(t, "help", "ai", "verbs", "|", "json")
	if code != 0 {
		t.Fatalf("help ai verbs | json: exit %d\n%s", code, out)
	}
	var ref reference.Reference
	if err := json.Unmarshal([]byte(out), &ref); err != nil {
		t.Fatalf("help ai verbs | json is not the reference: %v\n%s", err, out)
	}
	vocabulary := leaction.Vocabulary()
	if len(ref.Verbs) != len(vocabulary) {
		t.Fatalf("help ai verbs lists %d verbs, the vocabulary holds %d", len(ref.Verbs), len(vocabulary))
	}
	listed := map[string][]string{}
	for index, verb := range vocabulary {
		got := ref.Verbs[index]
		if got.Name != verb.Word || got.Kind != verb.Kind.String() || got.Meaning != verb.Meaning {
			t.Errorf("help ai verbs row %d is %+v, the vocabulary declares %+v", index, got, verb)
		}
		listed[got.Name] = got.Names
	}
	text, code := dispatchToStdout(t, "help", "ai", "verbs")
	if code != 0 {
		t.Fatalf("help ai verbs: exit %d\n%s", code, text)
	}
	declarations := 0
	for _, command := range leroot.Commands() {
		list, tabled := leroot.ActionsOf(command.Name)
		if !tabled {
			continue
		}
		for _, row := range list.Actions {
			if row.Does == "" {
				continue
			}
			declarations++
			name := strings.TrimSpace(command.Name + " " + row.Verb)
			if !slices.Contains(listed[row.Does], name) {
				t.Errorf("%s declares Does %q, and help ai verbs does not list it under that verb", name, row.Does)
			}
			if !strings.Contains(text, name) {
				t.Errorf("help ai verbs (text) does not name %s, which declares %q", name, row.Does)
			}
		}
	}
	if declarations == 0 {
		t.Fatal("no registered action declares Does, so no name was checked")
	}
	for _, verb := range vocabulary {
		if !strings.Contains(text, verb.Meaning) {
			t.Errorf("help ai verbs (text) does not carry the meaning of %s: %q", verb.Word, verb.Meaning)
		}
	}
}
