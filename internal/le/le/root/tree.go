// Design: docs/architecture/core-design.md -- le's command surface as one tree
// Related: leroot.go -- the registrations the tree is derived from
// Related: manifest.go -- the same surface as a payload
//
// The command tree completion, help, the menu, prompting and `le help ai` read.
// It is DERIVED from the registry and the action tables at the moment it is
// asked for, so it holds no fact of its own: dispatch and validation still
// parse only through leaction (plan/post-release/engineering/spec-le-cli-usability.md,
// D-Q3).
package leroot

import (
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// Tree answers le's command surface as a read-only command.Node tree: one node
// per word of every registered area, one child per action of its table, and
// the action's live parameters as argument definitions. Safe for concurrent
// use, because each call builds a new tree from the registry.
func Tree() *command.Node {
	checkRegistrations()
	return treeFrom(Commands())
}

// treeFrom builds the tree over one command set, which a test can pin.
func treeFrom(roots []registry.RootCommand) *command.Node {
	root := &command.Node{Children: map[string]*command.Node{}}
	for _, area := range roots {
		node := nodeFor(root, strings.Fields(area.Name))
		node.ShortHelp = area.Meta.ShortHelp
		list, declared := ActionsOf(area.Name)
		if !declared {
			continue
		}
		// A pass-through area has no action to offer, and every word after it
		// is its program's, so completion offers nothing below it and says why.
		if list.PassesWords {
			node.Description = list.PassesLine()
			continue
		}
		for _, row := range list.Actions {
			if _, taken := node.Children[row.Verb]; taken {
				panic("BUG: leroot.Tree: an action of " + area.Name + " is spelled like an area under it: " + row.Verb)
			}
			node.Children[row.Verb] = actionNode(row)
		}
	}
	return root
}

// nodeFor answers the node at words, creating each missing level. A level it
// creates is a namespace until an area of that name claims it, so it carries
// the summary the namespace registered (RegisterNamespace); treeFrom then
// overwrites it with the area's own summary when the name is also an area.
func nodeFor(root *command.Node, words []string) *command.Node {
	node := root
	for depth, word := range words {
		next, held := node.Children[word]
		if !held {
			next = &command.Node{Name: word, Children: map[string]*command.Node{}}
			if summary, named := NamespaceSummary(strings.Join(words[:depth+1], " ")); named {
				next.ShortHelp = summary
			}
			node.Children[word] = next
		}
		node = next
	}
	return node
}

// actionNode projects one action row. Each keyword becomes an argument
// definition of the kind its declaration maps to; a positional value has no
// keyword to offer, so its candidates become the node's value hints. Repeat,
// pair and retired are not projected: the completion callback applies them
// from the action table itself.
func actionNode(row leaction.Row) *command.Node {
	node := &command.Node{Name: row.Verb, ShortHelp: row.Why}
	for _, parameter := range row.Parameters {
		if parameter.Keyword == "" {
			node.ValueHints = hintsOf(parameter)
			continue
		}
		node.ArgDefs = append(node.ArgDefs, argDefOf(parameter))
	}
	return node
}

// argDefOf maps one keyword by its kind: switch to ArgFlag, enum to ArgEnum,
// uint to ArgUint, every other kind to ArgString. A constructor refuses only
// an empty name or an empty enum, which leaction.New already refused, so an
// error here is a Ze defect.
func argDefOf(parameter leaction.Parameter) command.ArgDef {
	options := command.ArgOptions{Mandatory: parameter.Requirement == leaction.Required, ShortHelp: parameter.Help}
	var def command.ArgDef
	var err error
	switch parameter.Kind {
	case leaction.KindSwitch:
		def, err = command.NewFlagArg(parameter.Keyword, options)
	case leaction.KindEnum:
		def, err = command.NewEnumArg(parameter.Keyword, parameter.Values, options)
	case leaction.KindUint:
		def, err = command.NewUintArg(parameter.Keyword, 64, nil, options)
	case leaction.KindString, leaction.KindList, leaction.KindDuration, leaction.KindPath,
		leaction.KindRevision, leaction.KindSpec, leaction.KindLabel, leaction.KindArea, leaction.KindWords,
		leaction.KindArgv:
		def, err = command.NewStringArg(parameter.Keyword, nil, nil, options)
	case leaction.KindUnspecified:
		panic("BUG: leroot.Tree: a live parameter reached the tree with no kind: " + parameter.Keyword)
	}
	if err != nil {
		panic("BUG: leroot.Tree: a declared parameter does not map to an argument definition: " + err.Error())
	}
	return def
}

// hintsOf answers the candidates of a positional value: its Source when it
// declares one, else its Values, else none.
func hintsOf(parameter leaction.Parameter) func() []command.Suggestion {
	return func() []command.Suggestion {
		var candidates []leaction.Candidate
		if parameter.Source != nil {
			candidates = parameter.Source()
		}
		for _, value := range parameter.Values {
			candidates = append(candidates, leaction.Candidate{Value: value})
		}
		out := make([]command.Suggestion, 0, len(candidates))
		for _, candidate := range candidates {
			out = append(out, command.Suggestion{Text: candidate.Value, Description: candidate.Summary, Type: command.SuggestionValue})
		}
		return out
	}
}
