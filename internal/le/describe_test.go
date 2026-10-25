// Design: docs/architecture/core-design.md -- le's command surface as one tree
// Related: le/root/describe.go -- the startup check this test runs over the real registry

package le

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestEveryWordLeOffersSaysWhatItDoes proves AC-54 (D-P7-describe-all) over
// the real registry: every node of the derived tree (namespace, area, action)
// carries text, every keyword's argument definition carries its Help, and
// every value an enum or a closed list declares carries its ValueHelp line.
// AC-55 (D-P7-short-long) adds the two lengths: every short text (a node's
// summary, a keyword's Help, a value's line) is one line of at most 60
// characters, and every node but a pass-through area carries a long
// description for the header the menu prints above an entered level.
// Method: walk leroot.Tree(), whose first build runs the startup check, then
// read every declared value through the same ValueCandidates completion uses.
// PREVENTS: a word help, completion, the menu or `le help ai` prints blank.
func TestEveryWordLeOffersSaysWhatItDoes(t *testing.T) {
	walkDescribed(t, "", leroot.Tree())

	described := 0
	for _, area := range leroot.Owned() {
		list, _ := leroot.ActionsOf(area)
		for _, row := range list.Actions {
			for _, parameter := range row.Parameters {
				for _, candidate := range parameter.ValueCandidates() {
					if strings.TrimSpace(candidate.Summary) == "" {
						t.Errorf("le %s %s: value %q of %q has no description", area, row.Verb, candidate.Value, parameter.Keyword)
						continue
					}
					checkShort(t, "le "+area+" "+row.Verb+" value "+candidate.Value, candidate.Summary)
					described++
				}
			}
		}
	}
	if described == 0 {
		t.Fatal("no declared value was read, so the walk proved nothing about value descriptions")
	}
	t.Logf("%d declared values carry a description", described)
}

// walkDescribed refuses every node under node whose summary is blank, longer
// than one line or over shortLineMax characters, every node but a pass-through
// area with no long description, and every argument definition whose help
// fails the same three short-text checks. The recursion is bounded by the depth of
// le's command tree: an area name of at most a few words, then one action.
func walkDescribed(t *testing.T, path string, node *command.Node) {
	t.Helper()
	for word, child := range node.Children {
		at := strings.TrimSpace(path + " " + word)
		if strings.TrimSpace(child.ShortHelp) == "" {
			t.Errorf("le %s has no summary", at)
		}
		if strings.Contains(child.ShortHelp, "\n") {
			t.Errorf("le %s summary spans more than one line: %q", at, child.ShortHelp)
		}
		if count := utf8.RuneCountInString(child.ShortHelp); count > shortLineMax {
			t.Errorf("le %s summary is %d characters, over %d: %q", at, count, shortLineMax, child.ShortHelp)
		}
		// A pass-through area prints its program's own help, so it alone owes
		// no long text; every namespace, area and action does.
		list, declared := leroot.ActionsOf(at)
		passes := declared && list.PassesWords
		if !passes && strings.TrimSpace(child.Description) == "" {
			t.Errorf("le %s has no long description", at)
		}
		for index := range child.ArgDefs {
			def := &child.ArgDefs[index]
			help := def.ShortHelp()
			if strings.TrimSpace(help) == "" {
				t.Errorf("le %s: keyword %q has no help", at, def.Name())
			}
			if strings.Contains(help, "\n") {
				t.Errorf("le %s: keyword %q help spans more than one line: %q", at, def.Name(), help)
			}
			if count := utf8.RuneCountInString(help); count > shortLineMax {
				t.Errorf("le %s: keyword %q help is %d characters, over %d: %q", at, def.Name(), count, shortLineMax, help)
			}
		}
		walkDescribed(t, at, child)
	}
}

// checkShort refuses a value's short text that is blank, spans more than one line, or
// holds more than the 60 characters one menu line shows (AC-55).
func checkShort(t *testing.T, item, text string) {
	t.Helper()
	if strings.TrimSpace(text) == "" {
		t.Errorf("%s is blank", item)
		return
	}
	if strings.Contains(text, "\n") {
		t.Errorf("%s spans more than one line: %q", item, text)
	}
	if count := utf8.RuneCountInString(text); count > shortLineMax {
		t.Errorf("%s is %d characters, over %d: %q", item, count, shortLineMax, text)
	}
}

// shortLineMax is the menu's right column, which a short text fills whole.
const shortLineMax = 60
