// Design: docs/contributing/running-commands.md -- bare le at a terminal, and prompting for a missing value
// Related: complete.go -- the candidates a prompt offers are the completion callback's
// Related: dispatch.go -- where a line reaches the prompt before it runs
//
// At a terminal (stdin AND stdout), a line that names an action but leaves out
// a value the action declares Required is not refused: le asks for each
// missing value, one by one, then echoes the whole line to stderr and runs it,
// so the reader learns the command (D-Q5). Optional values are never asked.
// Off a terminal nothing is asked, because an agent's tool call reaches le on
// pipes and a prompt there would wait forever (A-11): the line runs and is
// refused with its one action's usage, as before.

package leroot

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/component/cli/menu"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// echoPrefix opens the line le echoes before it runs a line the reader built
// at the menu or the prompt, so it is never mistaken for the command's output.
const echoPrefix = "running: "

// interactive answers whether a person is at the terminal: stdin and stdout
// both. A test replaces it.
var interactive = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// askValue asks the reader for one value. A test replaces it.
var askValue = func(prompt menu.Prompt) (string, error) {
	return menu.Ask(prompt, os.Stdin, os.Stdout)
}

// filled is a line after the prompt: the words to run, and whether le asked
// for any of them.
type filled struct {
	args  []string
	asked bool
}

// fillLine asks for every Required value the line name toolArgs leaves out and
// answers the completed words, which are toolArgs unchanged when nothing is
// missing. When le asked, or when echo is set because the reader chose the line
// at the menu, the completed line is echoed to stderr. Canceling a prompt
// answers menu.ErrCanceled and nothing runs.
func fillLine(program, name string, toolArgs []string, echo bool) (filled, error) {
	own, _ := splitChain(toolArgs)
	tail := toolArgs[len(own):]
	list, declared := ActionsOf(name)
	if !declared {
		return filled{args: toolArgs}, nil
	}
	verb, rest, missing := missingIn(list, own)
	if len(missing) != 0 {
		line, err := askMissing(name, verb, rest, missing)
		if err != nil {
			return filled{}, err
		}
		toolArgs = append(append([]string{verb}, line...), tail...)
	}
	if echo || len(missing) != 0 {
		echoLine(program, name, toolArgs)
	}
	return filled{args: toolArgs, asked: len(missing) != 0}, nil
}

// missingIn answers the verb own names, the words after it, and the Required
// parameters those words leave out. A bare line names the area's only action,
// which is what its body runs. Nothing is missing on a pass-through area or a
// forwarding action, whose words are another program's; on a line naming no
// action of the area; or on a line the action's parser refuses, which then
// runs and is refused as it always was.
func missingIn(list leaction.List, own []string) (string, []string, []leaction.Parameter) {
	if list.PassesWords {
		return "", nil, nil
	}
	if len(own) == 0 {
		if len(list.Actions) != 1 {
			return "", nil, nil
		}
		own = []string{list.Actions[0].Verb}
	}
	row, held := rowOf(list, own[0])
	if !held {
		return "", nil, nil
	}
	if row.Forwards {
		return "", nil, nil
	}
	missing, err := list.Missing(own[0], own[1:])
	if err != nil {
		return "", nil, nil
	}
	return own[0], own[1:], missing
}

// askMissing asks for each missing parameter in declaration order and answers
// the words after the verb with the answers in place: the positional value in
// the first slot, each keyword with its value after the words already typed.
func askMissing(name, verb string, rest []string, missing []leaction.Parameter) ([]string, error) {
	line := slices.Clone(rest)
	for _, parameter := range missing {
		if parameter.Keyword == "" {
			words, err := askOne(parameter, parameter.Value, append(strings.Fields(name), verb))
			if err != nil {
				return nil, err
			}
			line = append(words, line...)
			continue
		}
		before := append(append(append(strings.Fields(name), verb), line...), parameter.Keyword)
		words, err := askOne(parameter, parameter.Value, before)
		if err != nil {
			return nil, err
		}
		line = append(append(line, parameter.Keyword), words...)
		if parameter.Pair == "" {
			continue
		}
		second, err := askOne(parameter, parameter.Pair, nil)
		if err != nil {
			return nil, err
		}
		line = append(line, second...)
	}
	return line, nil
}

// askOne asks for one value, named label, offering the completion callback's
// candidates after the words before (none when before is nil). A list of words
// (KindWords, KindArgv) is split where the reader put spaces; every other
// value is one word.
func askOne(parameter leaction.Parameter, label string, before []string) ([]string, error) {
	prompt := menu.Prompt{Label: label, Help: parameter.Help}
	if before != nil {
		prompt.Complete = func(partial string) []string {
			return candidatesFor(parameter.Kind, append(slices.Clone(before), partial))
		}
	}
	value, err := askValue(prompt)
	if err != nil {
		return nil, err
	}
	if parameter.Kind == leaction.KindWords || parameter.Kind == leaction.KindArgv {
		return strings.Fields(value), nil
	}
	return []string{strings.TrimSpace(value)}, nil
}

// candidatesFor answers what Tab offers for the last of words: the completion
// callback's candidates, and for a path or a spec file, the paths on disk that
// begin with it, which is what the shell offers for the same word.
func candidatesFor(kind leaction.Kind, words []string) []string {
	var out []string
	candidates, err := Complete(words)
	if err == nil {
		for _, candidate := range candidates {
			out = append(out, candidate.Value)
		}
	}
	if kind == leaction.KindPath || kind == leaction.KindSpec {
		out = append(out, pathsFrom(words[len(words)-1])...)
	}
	return out
}

// pathsFrom answers the entries of partial's directory whose path begins with
// partial, a directory with a trailing slash so the next Tab descends. A
// directory that cannot be read offers nothing.
func pathsFrom(partial string) []string {
	dir, base := filepath.Split(partial)
	read := dir
	if read == "" {
		read = "."
	}
	entries, err := os.ReadDir(read)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		path := dir + entry.Name()
		if entry.IsDir() {
			path += "/"
		}
		out = append(out, path)
	}
	return out
}

// echoLine writes the line about to run to stderr, after echoPrefix, quoting
// each word a shell would split.
func echoLine(program, name string, toolArgs []string) {
	var tb textbuf.Buffer
	tb.Str(echoPrefix).Str(program).Byte(' ').Str(name)
	for _, word := range toolArgs {
		tb.Byte(' ')
		if word != "" && !strings.ContainsAny(word, " \t\n'\"\\$`|&;<>()*?[]{}!#~") {
			tb.Str(word)
			continue
		}
		tb.Byte('\'').Str(strings.ReplaceAll(word, "'", `'\''`)).Byte('\'')
	}
	tb.Byte('\n').StdErr() //nolint:errcheck // CLI output
}

// canceled is the refusal of a line whose prompt did not answer: nothing ran.
// The reader leaving with Ctrl-C or Ctrl-D is a usage answer, 2; a terminal
// that failed under the prompt is an execution failure, 1.
func canceled(program string, err error) int {
	var tb textbuf.Buffer
	tb.Str(program).Str(": ").Str(err.Error())
	if errors.Is(err, menu.ErrCanceled) {
		tb.Byte('\n').StdErr() //nolint:errcheck // CLI output
		return 2
	}
	tb.Str("; nothing ran\n").StdErr() //nolint:errcheck // CLI output
	return 1
}

// openMenu opens the menu over the derived tree and runs the line the reader
// chooses, echoed first, with every missing Required value asked for. Quitting
// runs nothing and answers 0: the reader asked what le holds and left.
func openMenu(program string) int {
	tree := Tree()
	chosen, err := menu.Run(menu.New(program, menu.LevelOf(tree, nil), tree))
	if err != nil {
		return canceled(program, err)
	}
	if len(chosen) == 0 {
		return 0
	}
	return dispatchLine(program, chosen, true)
}
