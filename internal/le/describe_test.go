// Design: docs/architecture/core-design.md -- le's command surface as one tree
// Related: le/root/describe.go -- the startup check this test runs over the real registry

package le

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestEveryWordLeOffersSaysWhatItDoes proves AC-54 (D-P7-describe-all) over
// the real registry: every node of the derived tree (namespace, area, action)
// carries text, every keyword's argument definition carries its Help, and
// every value an enum or a closed list declares carries its ValueHelp line.
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

// walkDescribed refuses every node under node that carries no text, and every
// argument definition with no help. The recursion is bounded by the depth of
// le's command tree: an area name of at most a few words, then one action.
func walkDescribed(t *testing.T, path string, node *command.Node) {
	t.Helper()
	for word, child := range node.Children {
		at := strings.TrimSpace(path + " " + word)
		if strings.TrimSpace(child.ShortHelp) == "" && strings.TrimSpace(child.Description) == "" {
			t.Errorf("le %s has no text", at)
		}
		for index := range child.ArgDefs {
			def := &child.ArgDefs[index]
			if strings.TrimSpace(def.ShortHelp()) == "" {
				t.Errorf("le %s: keyword %q has no help", at, def.Name())
			}
		}
		walkDescribed(t, at, child)
	}
}
