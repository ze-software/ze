// Design: docs/contributing/running-commands.md -- installing le's completion
// Related: tree.go -- the derived tree sharing the same command surface rules
// Related: shellcallback.go -- why the callback runs no hook
//
// Complete answers the candidates for the word a reader is typing. It resolves
// areas, namespaces and verbs directly along the typed prefix over Commands and
// ActionsOf (matching the derived tree without materializing unrelated areas),
// then reads the action's own table for what follows the verb, because the
// table, not the tree, knows repeat, pair and retired (D-Q3): a keyword already
// given is not offered again unless it repeats, a retired keyword is never
// offered, and an enum's values are offered only after their keyword.

package leroot

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// ErrUnknownWord is a typed word that is neither a command, a namespace nor a
// verb at its place, so le cannot say what follows it.
var ErrUnknownWord = errors.New("le has no such command")

// Complete answers the candidates for the last of words, which is the word
// being typed and may be empty; every word before it is complete. A typed word
// le does not know is ErrUnknownWord, naming it: the shell then offers nothing,
// and a caller can tell that from a place where nothing may follow. Typed
// words that open with a retired spelling answer a *RetiredError, the refusal
// dispatch gives for the same words. Every word
// after a pass-through area answers no candidate, because its program reads
// them.
//
// Complete walks only the prefix named by words directly over Commands and
// ActionsOf rather than building the full Tree for every registered area, so
// each Tab keystroke avoids allocating argument definitions for unrelated areas.
func Complete(words []string) ([]leaction.Candidate, error) {
	if len(words) == 0 {
		words = []string{""}
	}
	// A launcher word comes first and is the ./le script's, not the tree's: its
	// value is completed from its declaration, and the line after it is an
	// ordinary le line (launcher.go).
	values, atValue, rest := afterLauncher(words)
	if atValue {
		return withPrefix(values, words[len(words)-1]), nil
	}
	if len(rest) != len(words) {
		return Complete(rest)
	}
	typed, partial := words[:len(words)-1], words[len(words)-1]
	// A retired spelling typed whole is refused as dispatch refuses it, naming
	// the line that does the work now, so Tab teaches what a run would: after
	// a help word, and before a pipe chain, as dispatch reads them.
	if retired := retiredLine(leaction.Program(), typed); retired != nil {
		return nil, retired
	}
	commands := Commands()
	if len(typed) == 0 {
		return withPrefix(append(directChildren(commands, nil), launcherCandidates()...), partial), nil
	}
	var path []string
	for index, word := range typed {
		name := strings.Join(path, " ")
		if name != "" {
			if !Hidden(name) {
				if list, declared := ActionsOf(name); declared {
					if list.PassesWords {
						return nil, nil
					}
					// A verbless area's words are its sole action's grammar, from
					// the first one on (leaction.List.Verbless).
					if list.Verbless() {
						return afterVerb(list.Actions[0], typed[index:], partial)
					}
					// An area may also be the first words of other areas (`repo` and
					// `repo changed`, `setup` and `setup docker host`). Tree refuses a
					// verb spelled like an area under it, so a word that is no verb of
					// this area and is a child of its node names the area below.
					row, held := rowOf(list, word)
					if held {
						return afterVerb(row, typed[index+1:], partial)
					}
				}
			}
		}
		nextPath := append(path, word)
		if !pathHasVisibleNode(commands, nextPath) {
			return nil, unknownWord(path, word)
		}
		path = nextPath
	}
	name := strings.Join(path, " ")
	if name != "" {
		if !Hidden(name) {
			if list, declared := ActionsOf(name); declared {
				if list.PassesWords {
					return nil, nil
				}
				if list.Verbless() {
					return afterVerb(list.Actions[0], nil, partial)
				}
			}
		}
	}
	return withPrefix(directChildren(commands, path), partial), nil
}

// pathHasVisibleNode reports whether path names a non-hidden registered area
// or a namespace prefix of at least one non-hidden registered area.
func pathHasVisibleNode(commands []registry.RootCommand, path []string) bool {
	prefix := strings.Join(path, " ")
	prefixSpace := prefix + " "
	for _, area := range commands {
		if Hidden(area.Name) {
			continue
		}
		if area.Name == prefix {
			return true
		}
		if strings.HasPrefix(area.Name, prefixSpace) {
			return true
		}
	}
	return false
}

// directChildren answers the sorted direct child words of path (sub-areas,
// sub-namespaces, and non-verbless actions of the area at path), each with the
// same ShortHelp that Tree assigns to that node.
func directChildren(commands []registry.RootCommand, path []string) []leaction.Candidate {
	summaries := make(map[string]string)
	isArea := make(map[string]bool)
	prefix := strings.Join(path, " ")
	prefixSpace := ""
	if prefix != "" {
		prefixSpace = prefix + " "
	}
	for _, area := range commands {
		if Hidden(area.Name) {
			continue
		}
		rest := area.Name
		if prefix != "" {
			cut, ok := strings.CutPrefix(area.Name, prefixSpace)
			if !ok {
				continue
			}
			rest = cut
		}
		word, _, hasMore := strings.Cut(rest, " ")
		if !hasMore {
			summaries[word] = area.Meta.ShortHelp
			isArea[word] = true
			continue
		}
		if isArea[word] {
			continue
		}
		if _, exists := summaries[word]; exists {
			continue
		}
		namespace := word
		if prefix != "" {
			namespace = prefixSpace + word
		}
		summary, _ := NamespaceSummary(namespace)
		summaries[word] = summary
	}
	if prefix != "" {
		if !Hidden(prefix) {
			if list, declared := ActionsOf(prefix); declared {
				if !list.PassesWords {
					if !list.Verbless() {
						for _, row := range list.Actions {
							if _, taken := summaries[row.Verb]; !taken {
								summaries[row.Verb] = row.Summary
							}
						}
					}
				}
			}
		}
	}
	names := make([]string, 0, len(summaries))
	for name := range summaries {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]leaction.Candidate, 0, len(names))
	for _, name := range names {
		out = append(out, leaction.Candidate{Value: name, Summary: summaries[name]})
	}
	return out
}

// unknownWord is the refusal of word typed after path, naming the page that
// lists what may come there.
func unknownWord(path []string, word string) error {
	if len(path) == 0 {
		return fmt.Errorf("%w %q: ./le help lists every command", ErrUnknownWord, word)
	}
	place := strings.Join(path, " ")
	return fmt.Errorf("%w %q in %s: ./le %s --help lists what follows %s", ErrUnknownWord, word, place, place, place)
}

// rowOf answers the action spelled verb.
func rowOf(list leaction.List, verb string) (leaction.Row, bool) {
	for _, row := range list.Actions {
		if row.Verb == verb {
			return row, true
		}
	}
	return leaction.Row{}, false
}

// afterVerb reads the complete words after the verb against the action's
// table and answers what may come next: the values of the keyword waiting for
// one, else the positional's candidates in the first slot, and the keywords
// still open, each beginning with partial. A KindWords value is a whole le command
// line, so a word le does not know inside it is Complete's refusal.
func afterVerb(row leaction.Row, rest []string, partial string) ([]leaction.Candidate, error) {
	positional, hasPositional := positionalOf(row)
	if hasPositional && positional.Kind == leaction.KindArgv {
		return nil, nil // every word is the positional's own, typed as the reader means it
	}
	used := map[string]bool{}
	var waiting leaction.Parameter
	pending := 0 // values still owed to waiting
	for index, word := range rest {
		if pending > 0 {
			pending--
			continue
		}
		parameter, live := keywordOf(row, word)
		if !live {
			// The positional fills the first slot; any other unknown word is
			// the parser's to refuse, and the words after it still complete.
			continue
		}
		if parameter.Kind == leaction.KindWords {
			// The value is a whole le command line, completed as one.
			return Complete(append(append([]string(nil), rest[index+1:]...), partial))
		}
		used[parameter.Keyword] = true
		waiting, pending = parameter, valueCount(parameter)
	}
	if pending > 0 {
		if pending < valueCount(waiting) {
			return nil, nil // the second value of a pair names no candidate set
		}
		return withPrefix(valuesOf(waiting), partial), nil
	}
	var out []leaction.Candidate
	if len(rest) == 0 && hasPositional {
		out = append(out, valuesOf(positional)...)
	}
	for _, parameter := range row.Parameters {
		if parameter.Keyword == "" {
			continue // the positional, offered above
		}
		if parameter.Retired != "" {
			continue // the parser refuses it, naming its successor
		}
		if used[parameter.Keyword] && !parameter.Repeat {
			continue
		}
		out = append(out, leaction.Candidate{Value: parameter.Keyword, Summary: parameter.Help})
	}
	return withPrefix(out, partial), nil
}

// positionalOf answers the action's positional value, which has no keyword.
func positionalOf(row leaction.Row) (leaction.Parameter, bool) {
	for _, parameter := range row.Parameters {
		if parameter.Keyword == "" {
			return parameter, true
		}
	}
	return leaction.Parameter{}, false
}

// keywordOf answers the live keyword spelled word. A retired keyword is not
// live: the parser refuses it, naming its successor.
func keywordOf(row leaction.Row, word string) (leaction.Parameter, bool) {
	for _, parameter := range row.Parameters {
		if parameter.Keyword == word && parameter.Retired == "" {
			return parameter, true
		}
	}
	return leaction.Parameter{}, false
}

// valueCount answers how many words follow a keyword: none for a switch, two
// for a pair, one otherwise.
func valueCount(parameter leaction.Parameter) int {
	if parameter.Kind == leaction.KindSwitch {
		return 0
	}
	if parameter.Pair != "" {
		return 2
	}
	return 1
}

// valuesOf answers a value's candidates: its source's, then its declared
// values, each with the line that says what it does. A source lists
// candidates and never runs an action.
func valuesOf(parameter leaction.Parameter) []leaction.Candidate {
	var out []leaction.Candidate
	if parameter.Source != nil {
		out = append(out, parameter.Source()...)
	}
	return append(out, parameter.ValueCandidates()...)
}

// withPrefix keeps the candidates that begin with the word being typed.
func withPrefix(candidates []leaction.Candidate, partial string) []leaction.Candidate {
	out := candidates[:0:0]
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate.Value, partial) {
			out = append(out, candidate)
		}
	}
	return out
}
