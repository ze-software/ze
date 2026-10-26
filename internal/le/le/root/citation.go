// Design: docs/architecture/core-design.md -- le's command registry and dispatch
// Related: dispatch.go -- resolve, nestedNamespace and members, which this judge reuses
// Related: launcher.go -- the words the ./le script consumes before the binary runs
// Related: ../../doc/check/retired.go -- the sweep that hands every cited le line here
//
// A cited le line is judged by the resolver dispatch runs, so a page and the
// binary cannot disagree about what le holds (spec-le-cli-usability, AC-25).

package leroot

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrCitationRetired marks a cited line that opens with a retired spelling.
// The retired-name sweep reports that line itself, naming the successor, so a
// caller that already holds that report skips it rather than reporting it
// twice.
var ErrCitationRetired = errors.New("the line names a retired spelling")

// JudgeCitation answers whether le holds what a cited line names: the area,
// or the namespace, and the first word after an area that has verbs. words
// are the line's words after the program, its launcher words included; a word
// that is not a command word (a placeholder, a value) ends the line before
// it reaches here. It answers nil when dispatch would reach a command, an
// area listing, a namespace page or a help page, and an error naming the word
// le does not hold otherwise. A keyword or a value after the verb is the
// action's grammar and is not judged here.
//
// It reads the shared registry, so it MUST be called after every area's init
// has run.
func JudgeCitation(words []string) error {
	words = withoutLauncherWords(words)
	if len(words) == 0 {
		return nil
	}
	if isHelpArg(words[0]) {
		words = words[1:]
		if len(words) == 0 {
			return nil
		}
		if words[0] == aiWord {
			return nil
		}
	}
	// A retired spelling is refused before anything resolves it, as dispatch
	// refuses it (retiredLine).
	if retiredSpelling("le", "", words) != nil {
		return ErrCitationRetired
	}
	name, handler, rest := resolve(words)
	if token, after, nested := nestedNamespace(name, rest); nested {
		return judgeNamespaceWords(token, after)
	}
	if handler == nil {
		if len(members(words[0])) != 0 {
			return judgeNamespaceWords(words[0], words[1:])
		}
		var tb textbuf.Buffer
		return errors.New(tb.Str("le holds no command ").Str(words[0]).String())
	}
	return judgeActionWord(name, rest)
}

// withoutLauncherWords answers words with the launcher words that open it
// removed: `--name <name>` and `--name=<name>`, which the ./le script reads
// before the binary runs (launcherWords).
func withoutLauncherWords(words []string) []string {
	for len(words) != 0 {
		consumed := launcherWordsAt(words)
		if consumed == 0 {
			return words
		}
		words = words[consumed:]
	}
	return words
}

// launcherWordsAt answers how many of the opening words one launcher word
// takes, its value included, and 0 when the line does not open with one.
func launcherWordsAt(words []string) int {
	for _, word := range launcherWords {
		if strings.HasSuffix(word.Word, "=") {
			if strings.HasPrefix(words[0], word.Word) {
				return 1
			}
			continue
		}
		if words[0] != word.Word {
			continue
		}
		return min(2, len(words))
	}
	return 0
}

// judgeNamespaceWords judges the words after a namespace token that resolved
// to no command: none, or a lone help word, asks for the namespace page, and
// anything else names a member the namespace does not hold (answerNamespace).
func judgeNamespaceWords(token string, rest []string) error {
	if len(rest) == 0 {
		return nil
	}
	if len(rest) == 1 && isHelpArg(rest[0]) {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str(token).Str(" is a namespace and holds no ").Str(rest[0]).
		Str("; it holds: ").Join(members(token), " | ").String())
}

// judgeActionWord judges the first word after a resolved area against the
// table the area registered. No word asks for the listing, a help word asks
// for the area's page, and an area that passes its words to another program,
// or whose bare line is its sole action (Verbless), reads that word as its
// grammar. Every area registers a table (requireEveryAreaHasATable), so one
// that did not is said rather than read as held.
func judgeActionWord(name string, rest []string) error {
	if len(rest) == 0 {
		return nil
	}
	if isHelpArg(rest[0]) {
		return nil
	}
	list, declared := ActionsOf(name)
	if !declared {
		var tb textbuf.Buffer
		return errors.New(tb.Str(name).Str(" registered no action table, so ").Str(rest[0]).
			Str(" cannot be judged").String())
	}
	if list.PassesWords {
		return nil
	}
	if list.Verbless() {
		return nil
	}
	verbs := make([]string, 0, len(list.Actions))
	for _, row := range list.Actions {
		if row.Verb == rest[0] {
			return nil
		}
		verbs = append(verbs, row.Verb)
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str(name).Str(" holds no action ").Str(rest[0]).
		Str("; it holds: ").Join(verbs, " | ").String())
}
