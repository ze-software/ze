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
	// segments holds each command's own text, parallel to commands: what lies
	// between the operator before it and the operator that ends it, with its
	// quotes and redirections as typed, a comment cut off, continuation lines
	// joined and the ends trimmed. A guard that judges a command's words
	// together with its redirections (`2>&1` as one word) splits this with
	// shellWords.
	segments []string
	// expands holds, parallel to commands, whether the shell may change each
	// word before the command reads it: a `$` outside single quotes (a
	// variable, `$'...'`, an arithmetic expansion), or a glob outside quotes
	// (`*`, `?`, or a `[` closed later in the word). Such a word's value is
	// not the text the hook reads, and an unquoted one can become several
	// words. A `~` the shell may replace with a home directory counts too.
	// An escaped character expands nothing.
	expands [][]bool
	// redirects holds, parallel to commands, whether the command's text holds
	// an unquoted `>`, so writesFile need not read a quoted `->` in a grep
	// pattern as a redirection.
	redirects []bool
	// separators holds, parallel to commands, the operator that ends each
	// command: `;`, `&&`, `||`, `|`, `|&`, `&`, a newline, a group or
	// subshell bracket, `!`, or "" at the end of the line.
	separators []string
	// substitutes reports a backtick or a `$(` outside single quotes. Either
	// one runs a command, and inside double quotes that command is part of
	// one word, so commands does not show it.
	substitutes bool
}

// writesFile reports whether the command at index redirects its output into
// a file: `> f`, `>> f`, `>| f`, `&> f` or `2> f`. A redirection to
// /dev/null or onto another descriptor (`2>&1`, `>&2`) writes nothing, and an
// input redirection holds no `>` (writesOnlyDevNull, gobuild.go). Once a
// command holds an unquoted `>`, every word of its text is judged, so a
// quoted `>` beside the redirection refuses in the safe direction.
func (line shellLine) writesFile(index int) bool {
	if !line.redirects[index] {
		return false
	}
	return !writesOnlyDevNull(shellWords(line.segments[index]))
}

// readerLine answers the commands of a line on which nothing runs a string
// it was handed and nothing writes a file: every command is a declared reader
// its judge admits (readerCommand, readers.go) whose output goes to no file
// (writesFile). It answers false for a line outside the shell syntax the hook
// models (modeledSyntax, shellsyntax.go), for a line lineCommands cannot
// parse and for a line with a command substitution. The syntax check comes
// first because lineCommands splits only that syntax the way bash does: on
// any other line its commands need not be the ones bash runs, so nothing it
// answers may lift a verdict.
//
// Each guard's text judgment is its default verdict, unchanged from before
// the guards read commands, and readerLine is the only line on which a guard
// may lift it (the build guard's reader+build line aside, goBuildLine). That
// is the only direction the parse moves a verdict: a quoted `go build` in a
// grep pattern is text on a reader line, while a shell, an interpreter, a
// launcher or any command the table does not hold may run what its quotes
// carry, so its line keeps the text judgment.
func readerLine(command string) (shellLine, bool) {
	if !modeledSyntax(command) {
		return shellLine{}, false
	}
	line, parsed := lineCommands(command)
	if !parsed {
		return shellLine{}, false
	}
	if line.substitutes {
		return shellLine{}, false
	}
	for index, words := range line.commands {
		if !readerCommand(words, line.expands[index]) {
			return shellLine{}, false
		}
		if line.writesFile(index) {
			return shellLine{}, false
		}
	}
	return line, true
}

// lineCommands is the one splitter of a shell line: it yields each command
// with its words, quotes removed, and its text. A word ends at whitespace
// and at any of `; & | ( ) { } ! < >`. A command ends at a newline and at
// `; | ( ) { } !`, and at an `&` that is not part of a redirect (`>&`, `<&`,
// `&>`), so `a && b`, `a | b` and `$(a)` are separate commands while
// `a 2>&1` is one. A redirect does not end a command, so its target is a
// word of the command it belongs to, and the `|` of `>|` is part of its
// redirect. An unquoted `>` marks its command (shellLine.redirects). Shell keywords are not recognized:
// `do`, `then` and a leading assignment are the first word of their command,
// and a caller that judges command words judges them as such. A `#` that
// starts a word comments out the rest of its line. Inside double quotes a
// backslash escapes only `$`, a backtick, `"`, `\` and a newline, as in bash,
// and is kept before any other character. It returns false when the line
// ends inside a quote or after a lone backslash, because a caller that cannot
// see where a word ends cannot judge it. It models only part of the shell's
// syntax (here-documents, `$'...'` and brace expansion are not among it), so
// a caller that trusts its split first requires modeledSyntax.
func lineCommands(line string) (shellLine, bool) {
	var parsed shellLine
	command := make([]string, 0, 8)
	commandExpands := make([]bool, 0, 8)
	// redirecting marks a command whose text holds an unquoted `>`.
	redirecting := false
	var word textbuf.Buffer
	word.Reset()
	// quoted marks a word that opened with a quote, so `''#x` is a word and
	// not a comment.
	quoted := false
	// expanding marks a word the shell may change (shellLine.expands), and
	// bracket an unquoted `[` that a later unquoted `]` turns into a glob.
	expanding, bracket := false, false
	endWord := func() {
		if word.Len() != 0 {
			command = append(command, word.String())
			commandExpands = append(commandExpands, expanding)
		}
		quoted, expanding, bracket = false, false, false
	}
	// start is the byte offset where the current command's text begins.
	start := 0
	endCommand := func(end int, separator string) {
		endWord()
		if len(command) != 0 {
			parsed.commands = append(parsed.commands, command)
			parsed.expands = append(parsed.expands, commandExpands)
			parsed.redirects = append(parsed.redirects, redirecting)
			parsed.separators = append(parsed.separators, separator)
			parsed.segments = append(parsed.segments, strings.TrimSpace(strings.ReplaceAll(line[start:end], "\\\n", "")))
			command = make([]string, 0, 8)
			commandExpands = make([]bool, 0, 8)
		}
		redirecting = false
		start = end + 1
	}
	quote := rune(0)
	escaped := false
	comment := false
	// commentAt is the byte offset of the `#` that opened the comment, where
	// the command's text ends.
	commentAt := 0
	// previous is the character before this one when it was neither escaped
	// nor an escape, else 0. It tells `$(` from `(` and `>&` from `&`.
	previous := rune(0)
	// redirectOperator marks a character that is an unquoted, unescaped `>`
	// the splitter read as a redirection operator: only it may take the next
	// `|` into `>|`.
	redirectOperator := false
	for index, character := range line {
		before := previous
		previous = character
		afterOperator := redirectOperator
		redirectOperator = false
		if comment {
			if character == '\n' {
				comment = false
				endCommand(commentAt, "\n")
				start = index + 1
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
		if character == '\\' && quote == '"' && strings.IndexByte("$`\"\\\n", byteAt(line, index+1)) < 0 {
			// Inside double quotes bash keeps a backslash that precedes any
			// other character, so the word holds it, as bash passes it on.
			word.WriteRune(character)
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
		if quote == '"' && character == '$' {
			expanding = true
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
			commentAt = index
		case character == '&' && redirectAmpersand(before, line[index+1:]):
			endWord()
		case character == '|' && afterOperator:
			// The `|` of `>|`, which writes a file despite noclobber, belongs to
			// the redirection and ends no command.
		case character == '\n' || strings.ContainsRune(";&|(){}!", character):
			endCommand(index, separatorAt(line, index))
		case character == '>':
			redirecting, redirectOperator = true, true
			endWord()
		case unicode.IsSpace(character) || character == '<':
			endWord()
		default:
			expanding = expanding || expandsUnquoted(character, bracket)
			expanding = expanding || tildeExpands(character, before, word.Len() == 0 && !quoted)
			bracket = bracket || character == '['
			word.WriteRune(character)
		}
	}
	if quote != 0 {
		return shellLine{}, false
	}
	if escaped {
		return shellLine{}, false
	}
	end := len(line)
	if comment {
		end = commentAt
	}
	endCommand(end, "")
	return parsed, true
}

// separatorAt answers the operator that starts at index, where a command
// ended: `&&`, `||` and `|&` whole, any other one character as written. The
// second character of a pair ends an empty command, which records nothing.
func separatorAt(line string, index int) string {
	pair := line[index:min(index+2, len(line))]
	if oneOf(pair, "&&", "||", "|&") {
		return pair
	}
	return line[index : index+1]
}

// expandsUnquoted reports whether an unquoted character lets the shell change
// its word: a `$`, a `*` or a `?`, or a `]` after an unquoted `[` in the same
// word (bracket), which closes a glob. A lone `[`, as the test command, stays
// text.
func expandsUnquoted(character rune, bracket bool) bool {
	if strings.ContainsRune("$*?", character) {
		return true
	}
	return bracket && character == ']'
}

// tildeExpands reports whether an unquoted `~` may become a home directory:
// at the start of a word (wordStart), or after `=` or `:`, where bash expands
// it in an assignment. A `~` after `=` or `:` in a word that is no
// assignment (`--dir=~/x`) is marked too, in the safe direction.
func tildeExpands(character, before rune, wordStart bool) bool {
	if character != '~' {
		return false
	}
	if wordStart {
		return true
	}
	return before == '=' || before == ':'
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
