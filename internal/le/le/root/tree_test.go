// Related: tree.go -- the derived command tree these tests pin

package leroot

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// treeProbe registers one area under a namespace, carrying every kind the
// mapping table names, and answers the area.
func treeProbe(t *testing.T) leaction.Area {
	t.Helper()
	area := leaction.New("treeprobe area", leaction.Action{
		Verb: "run", Why: "run the probe. It takes every kind.",
		Parameters: []leaction.Parameter{
			{Keyword: "dry", Kind: leaction.KindSwitch, Help: "change nothing"},
			{Keyword: "step", Kind: leaction.KindEnum, Value: "step", Values: []string{"build", "test"}, Requirement: leaction.Optional, Help: "the step to run"},
			{Keyword: "count", Kind: leaction.KindUint, Value: "count", Requirement: leaction.Optional, Help: "how many"},
			{Keyword: "file", Kind: leaction.KindPath, Value: "path", Requirement: leaction.Required, Help: "the file"},
			{Keyword: "old", Retired: "file"},
		},
		AnswerArgs: func(leaction.Arguments) (any, int) { return nil, 0 },
	}, leaction.Action{Verb: "show", Why: "show the probe", Answer: func() (any, int) { return nil, 0 }})
	Register(area.Name(), GroupReport, area.Answer, registry.Meta{
		ShortHelp: "a probe area under a namespace", Mode: "offline", Section: registry.SectionTest,
	})
	RegisterActions(area.Name(), area.Actions)
	return area
}

// VALIDATES: Phase 1 wiring -- the tree derived from the registry holds every
// registered area at the path its words spell, and every action of its table
// under it.
// PREVENTS: a second declaration of le's surface for completion, help, the menu
// or `le help ai`, which drifts from what the registry holds.
func TestDerivedTreeHoldsEveryRegisteredAction(t *testing.T) {
	treeProbe(t)
	root := Tree()

	for _, area := range Commands() {
		node := nodeAt(root, strings.Fields(area.Name))
		if node == nil {
			t.Errorf("the tree has no node for area %q", area.Name)
			continue
		}
		if node.ShortHelp != area.Meta.ShortHelp {
			t.Errorf("area %q: the node says %q, the registry %q", area.Name, node.ShortHelp, area.Meta.ShortHelp)
		}
		list, declared := ActionsOf(area.Name)
		if !declared {
			continue
		}
		for _, row := range list.Actions {
			if node.Children[row.Verb] == nil {
				t.Errorf("area %q: the tree lacks action %q", area.Name, row.Verb)
			}
		}
	}
	if nodeAt(root, []string{"treeprobe", "area", "show"}) == nil {
		t.Error("the probe's zero-argument action is absent from the tree")
	}
}

// VALIDATES: the D-Q3 mapping table -- switch to ArgFlag, enum to ArgEnum with
// its values, uint to ArgUint, every other kind to ArgString; requiredness
// and help carried; a retired keyword absent.
// PREVENTS: a completion that offers a value after a switch, or a retired
// keyword offered as live.
func TestDerivedTreeMapsKindsToArgDefs(t *testing.T) {
	treeProbe(t)
	node := nodeAt(Tree(), []string{"treeprobe", "area", "run"})
	if node == nil {
		t.Fatal("the probe action is absent from the tree")
	}
	want := map[string]command.ArgKind{
		"dry": command.ArgFlag, "step": command.ArgEnum, "count": command.ArgUint, "file": command.ArgString,
	}
	if len(node.ArgDefs) != len(want) {
		t.Errorf("the action carries %d argument definitions, want %d", len(node.ArgDefs), len(want))
	}
	for _, def := range node.ArgDefs {
		kind, known := want[def.Name()]
		if !known {
			t.Errorf("the tree carries %q, which is retired or undeclared", def.Name())
			continue
		}
		if def.Kind() != kind {
			t.Errorf("%s maps to kind %v, want %v", def.Name(), def.Kind(), kind)
		}
		if def.ShortHelp() == "" {
			t.Errorf("%s carries no help", def.Name())
		}
	}
	for _, def := range node.ArgDefs {
		if def.Name() == "step" && strings.Join(slices.Collect(def.EnumValues()), ",") != "build,test" {
			t.Errorf("step carries enum values %v", def.EnumValues())
		}
		if def.Name() == "file" && !def.Mandatory() {
			t.Error("the required keyword file is not mandatory in the tree")
		}
	}
}

// VALIDATES: R-10 -- every word the tree completes is one dispatch accepts:
// each command word walks to a registered area or action, and each action's
// keywords, given a value of their kind, pass that action's own parser.
// PREVENTS: completion offering a word dispatch refuses.
func TestEveryCompletedWordIsAcceptedByDispatch(t *testing.T) {
	treeProbe(t)
	root := Tree()
	completer := command.NewTreeCompleter(root)
	areas := make(map[string]bool)
	for _, area := range Commands() {
		areas[area.Name] = true
	}

	var walk func(path []string, node *command.Node)
	walk = func(path []string, node *command.Node) {
		for name, child := range node.Children {
			words := append(append([]string(nil), path...), name)
			offered := false
			for _, suggestion := range completer.Complete(strings.Join(path, " ") + " " + name[:1]) {
				if suggestion.Text == name {
					offered = true
				}
			}
			if len(path) == 0 {
				offered = offered || completedAtRoot(completer, name)
			}
			if !offered {
				t.Errorf("the completer does not offer %q at %q", name, strings.Join(path, " "))
			}
			if len(child.ArgDefs) > 0 || isActionNode(words, areas) {
				checkActionLine(t, words, child)
				continue
			}
			walk(words, child)
		}
	}
	walk(nil, root)
}

// completedAtRoot reports whether the completer offers name for its first
// letter typed alone, which is how a root word is completed.
func completedAtRoot(completer *command.TreeCompleter, name string) bool {
	for _, suggestion := range completer.Complete(name[:1]) {
		if suggestion.Text == name {
			return true
		}
	}
	return false
}

// isActionNode reports whether words end in a verb of a registered area.
func isActionNode(words []string, areas map[string]bool) bool {
	if len(words) < 2 {
		return false
	}
	area := strings.Join(words[:len(words)-1], " ")
	if !areas[area] {
		return false
	}
	list, declared := ActionsOf(area)
	if !declared {
		return false
	}
	_, held := list.ActionPage("le", words[len(words)-1])
	return held
}

// checkActionLine builds the line that types every keyword the action node
// carries, with a value its kind accepts, and asks the area's own parser.
func checkActionLine(t *testing.T, words []string, node *command.Node) {
	t.Helper()
	area := strings.Join(words[:len(words)-1], " ")
	verb := words[len(words)-1]
	list, declared := ActionsOf(area)
	if !declared {
		t.Errorf("%q is completed as an action of %q, which registers no table", verb, area)
		return
	}
	line := []string{}
	for index := range node.ArgDefs {
		def := &node.ArgDefs[index]
		line = append(line, def.Name())
		switch def.Kind() {
		case command.ArgFlag:
		case command.ArgEnum:
			line = append(line, slices.Collect(def.EnumValues())[0])
		case command.ArgUint:
			line = append(line, "1")
		case command.ArgString, command.ArgUnion:
			line = append(line, sampleValue(list, verb, def.Name()))
		}
		// A pair keyword takes a second value after the first (`hunks <path>
		// <patch-file>`), so the line a reader completes types both.
		if takesPair(list, verb, def.Name()) {
			line = append(line, "x")
		}
	}
	if err := list.Accepts(verb, line); err != nil {
		t.Errorf("le %s %s %s: the completed line is refused: %v", area, verb, strings.Join(line, " "), err)
	}
}

// takesPair reports whether the verb's keyword declares a second value.
func takesPair(list leaction.List, verb, keyword string) bool {
	for _, row := range list.Actions {
		if row.Verb != verb {
			continue
		}
		for _, parameter := range row.Parameters {
			if parameter.Keyword == keyword {
				return parameter.Pair != ""
			}
		}
	}
	return false
}

// sampleValue answers a value the keyword's declared kind accepts.
func sampleValue(list leaction.List, verb, keyword string) string {
	for _, row := range list.Actions {
		if row.Verb != verb {
			continue
		}
		for _, parameter := range row.Parameters {
			if parameter.Keyword != keyword {
				continue
			}
			for _, candidate := range []string{"1s", "HEAD", "x", "plan/x.md", "spec claim"} {
				if parameter.Kind.Check(candidate, parameter.Values) == nil {
					return candidate
				}
			}
			if len(parameter.Values) > 0 {
				return parameter.Values[0]
			}
		}
	}
	return "x"
}

// nodeAt walks words from root and answers the node, or nil.
func nodeAt(root *command.Node, words []string) *command.Node {
	node := root
	for _, word := range words {
		next := node.Children[word]
		if next == nil {
			return nil
		}
		node = next
	}
	return node
}
