// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- the command guards that judge these words
// Related: session.go -- the identity check that judges each command's first word
package hookruntime

import (
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// lineWords splits a whole shell line into words with their quotes removed, so
// `'rm'` and `"rm"` are the word rm, as the shell runs them. It does not split the line
// into commands: whitespace and the operators `; & | ( ) { } ! < >` all break a
// word, so `2>&1`, `>/dev/null` and `{ rm x; }` leave the paths as words of
// their own. A `#` that starts a word comments out the rest of its line. It
// returns false when the line ends inside a quote or after a lone backslash,
// because a caller that cannot see where a word ends cannot judge it. The
// words are those of lineCommands, in line order, with the command boundaries
// dropped.
func lineWords(line string) ([]string, bool) {
	parsed, ok := lineCommands(line)
	if !ok {
		return nil, false
	}
	words := make([]string, 0, 16)
	for _, command := range parsed.commands {
		words = append(words, command...)
	}
	return words, true
}

// shellLine is one shell line split into the commands it runs.
type shellLine struct {
	// commands holds the words of each command in line order, quotes removed.
	// No command is empty.
	commands [][]string
	// substitutes reports a backtick or a `$(` outside single quotes. Either
	// one runs a command, and inside double quotes that command is part of
	// one word, so commands does not show it.
	substitutes bool
}

// lineCommands is the one splitter of a shell line: it yields each command
// with its words, quotes removed. A word ends at whitespace and at any of
// `; & | ( ) { } ! < >`. A command ends at a newline and at `; | ( ) { } !`,
// and at an `&` that is not part of a redirect (`>&`, `<&`, `&>`), so
// `a && b`, `a | b` and `$(a)` are separate commands while `a 2>&1` is one.
// A redirect does not end a command, so its target is a word of the command
// it belongs to. Shell keywords are not recognized: `do`, `then` and a leading
// assignment are the first word of their command, and a caller that judges
// command words judges them as such. A `#` that starts a word comments out
// the rest of its line. It returns false when the line ends inside a quote or
// after a lone backslash, because a caller that cannot see where a word ends
// cannot judge it.
func lineCommands(line string) (shellLine, bool) {
	var parsed shellLine
	command := make([]string, 0, 8)
	var word textbuf.Buffer
	word.Reset()
	// quoted marks a word that opened with a quote, so `''#x` is a word and
	// not a comment.
	quoted := false
	endWord := func() {
		if word.Len() != 0 {
			command = append(command, word.String())
		}
		quoted = false
	}
	endCommand := func() {
		endWord()
		if len(command) != 0 {
			parsed.commands = append(parsed.commands, command)
			command = make([]string, 0, 8)
		}
	}
	quote := rune(0)
	escaped := false
	comment := false
	// previous is the character before this one when it was neither escaped
	// nor an escape, else 0. It tells `$(` from `(` and `>&` from `&`.
	previous := rune(0)
	for index, character := range line {
		before := previous
		previous = character
		if comment {
			if character == '\n' {
				comment = false
				endCommand()
			}
			continue
		}
		if escaped {
			// A backslash before a newline continues the line and adds nothing.
			if character != '\n' {
				word.WriteRune(character)
			}
			escaped = false
			previous = 0
			continue
		}
		if character == '\\' && quote != '\'' {
			escaped = true
			previous = 0
			continue
		}
		if quote != '\'' && substitutionStart(before, character) {
			parsed.substitutes = true
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			} else {
				word.WriteRune(character)
			}
			continue
		}
		switch {
		case character == '\'' || character == '"':
			quote = character
			quoted = true
		case character == '#' && word.Len() == 0 && !quoted:
			comment = true
		case character == '&' && redirectAmpersand(before, line[index+1:]):
			endWord()
		case character == '\n' || strings.ContainsRune(";&|(){}!", character):
			endCommand()
		case unicode.IsSpace(character) || strings.ContainsRune("<>", character):
			endWord()
		default:
			word.WriteRune(character)
		}
	}
	if quote != 0 {
		return shellLine{}, false
	}
	if escaped {
		return shellLine{}, false
	}
	endCommand()
	return parsed, true
}

// substitutionStart reports whether a character opens a command substitution:
// a backtick, or the `(` of `$(`.
func substitutionStart(before, character rune) bool {
	if character == '`' {
		return true
	}
	return before == '$' && character == '('
}

// redirectAmpersand reports whether an `&` belongs to a redirect rather than
// separating two commands: the `&` of `>&` and `<&`, which follows the
// operator, and the `&` of `&>`, which the rest of the line shows.
func redirectAmpersand(before rune, rest string) bool {
	if before == '>' {
		return true
	}
	if before == '<' {
		return true
	}
	return strings.HasPrefix(rest, ">")
}
