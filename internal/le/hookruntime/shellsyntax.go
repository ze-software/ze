// Design: docs/contributing/running-commands.md -- the shell syntax the hook models
// Related: shellline.go -- lineCommands, the splitter this allow-list protects
// Related: session.go -- commandOutsideReaders, which requires it before the reader exemption
// Related: gobuild.go -- goBuildLine, which requires it before it reads a build's words
package hookruntime

import "strings"

// The constructs unmodeledSyntax names in more than one place.
const (
	syntaxNonASCII   = "a byte outside printable ASCII (a carriage return, a control character or a non-ASCII character)"
	syntaxHash       = "a `#`, which can start a comment"
	syntaxBang       = "a `!`, which can negate a command or expand history"
	syntaxBacktick   = "a backtick, which runs a command"
	syntaxQuoteLines = "a quoted string that spans lines"
	syntaxQuoteOpen  = "a quote that is never closed"
)

// modeledSyntax reports whether a whole line lies inside the shell syntax
// lineCommands splits into the words bash runs (unmodeledSyntax). A guard lifts its
// text verdict, and the identity check exempts a line of readers, only on a
// line for which it answers true, so a construct the splitter does not model
// keeps the text verdict and needs the identity line, without the splitter
// having to model it.
func modeledSyntax(line string) bool {
	return unmodeledSyntax(line) == ""
}

// unmodeledSyntax answers the first construct of a line that lies outside
// the modeled syntax, in words a message can carry, and "" when there is
// none. The syntax is an allow-list, never a list of known dangers:
//
//   - bytes: printable ASCII, a space, a tab and a newline;
//   - words: unquoted text, a glob (`*`, `?`, `[`) and a `~`, which
//     lineCommands marks as expanding;
//   - a single-quoted string on one line, holding no `#`;
//   - a double-quoted string on one line, holding no `#`, `!`, backtick or
//     `$(`, whose `$` is a plain parameter, and whose backslash precedes a
//     character bash keeps it before (lineCommands keeps it too);
//   - `$NAME`, `$_`, `$?`, `$@`, `$*`, `$$`, `$-` and `$0` to `$9`, and
//     `${NAME}` inside double quotes, where lineCommands keeps its braces
//     in the word;
//   - the separators `;`, `&&`, `||`, `|` and a newline;
//   - the redirections `<`, `>`, `>>`, `>|`, `&>`, `N>` and `N>>` followed by
//     a word, `<<<` followed by a word, and `>&M` onto a descriptor, each
//     only after the last word of its command (syntaxCommand).
//
// Everything else answers a construct: a here-document, ANSI-C or locale
// quoting, a brace, `!`, `#`, a backslash outside quotes, a backtick, `(` and
// `)` (subshells, substitutions, process substitution, arithmetic, function
// definitions, extended globs), `[[`, a background `&`, `|&`, `;;`, a
// carriage return, any control or non-ASCII byte, a redirection before or
// between a command's words, and an empty quoted word (an empty pair of
// single or double quotes).
func unmodeledSyntax(line string) string {
	var command syntaxCommand
	for index := 0; index < len(line); {
		next, construct := syntaxStep(line, index)
		if construct != "" {
			return construct
		}
		if construct := command.token(line, index, next); construct != "" {
			return construct
		}
		index = next
	}
	return command.closeWord()
}

// The constructs syntaxCommand names.
const (
	syntaxRedirectFirst = "a redirection before the command's first word, which can make its target the command"
	syntaxRedirectWord  = "a word after a redirection, which can change which word is the command or an option's value"
	syntaxEmptyWord     = "an empty quoted word (`''` or `\"\"`), which bash passes as an argument and the splitter drops"
)

// syntaxCommand tracks where the words and redirections of the command being
// judged lie. lineCommands drops a redirection operator and keeps its target
// as a word, and drops an empty quoted word, while bash does neither: a
// redirection before or between the words, or an empty word, moves which
// word the splitter reads as the command or as an option's value
// (`<<<cat ./le x` reads as cat, and diff -L with an empty quoted value
// before -l hands -L the -l). So a redirection may only follow the command's
// last word, and no word may be empty; a line that breaks either lies
// outside the modeled syntax.
type syntaxCommand struct {
	// words counts the command's words, its redirections' targets and
	// descriptor numbers left out.
	words int
	// redirected marks a command that has met a redirection.
	redirected bool
	// target marks that the next word is a redirection's target.
	target bool
	// open marks a word in progress, and content one that holds a byte
	// other than an empty quote pair.
	open, content bool
}

// token judges the token syntaxStep read between index and next for its
// place in the command, and answers the construct it breaks, or "".
func (command *syntaxCommand) token(line string, index, next int) string {
	character := line[index]
	switch {
	case character == ' ' || character == '\t':
		return command.closeWord()
	case character == '\n' || character == ';' || character == '|' || (character == '&' && byteAt(line, index+1) == '&'):
		construct := command.closeWord()
		*command = syntaxCommand{}
		return construct
	case character == '>' || character == '<' || character == '&':
		// syntaxStep admits an `&` only in `&&` and `&>`, so this one opens
		// a redirection.
		if construct := command.closeWord(); construct != "" {
			return construct
		}
		if command.words == 0 {
			return syntaxRedirectFirst
		}
		command.redirected = true
		// `>&N` ends at its descriptor; every other redirection's target
		// word starts at next.
		command.target = !wordEnds(byteAt(line, next))
		return ""
	}
	return command.wordToken(line, index, next)
}

// wordToken judges a token that starts or continues a word.
func (command *syntaxCommand) wordToken(line string, index, next int) string {
	if !command.open {
		command.open, command.content = true, false
		switch {
		case command.target:
			command.target = false
		case descriptorPrefix(line, index):
			// The `2` of `2>f` belongs to the redirection that follows it.
		case command.redirected:
			return syntaxRedirectWord
		default:
			command.words++
		}
	}
	if !emptyQuote(line, index, next) {
		command.content = true
	}
	return ""
}

// closeWord ends the word in progress, and answers syntaxEmptyWord when it
// held nothing but empty quote pairs.
func (command *syntaxCommand) closeWord() string {
	empty := command.open && !command.content
	command.open = false
	if empty {
		return syntaxEmptyWord
	}
	return ""
}

// descriptorPrefix reports whether the word that starts at index is a
// descriptor number written against a redirection (`2>f`, `0<f`).
func descriptorPrefix(line string, index int) bool {
	end := index
	for '0' <= byteAt(line, end) && byteAt(line, end) <= '9' {
		end++
	}
	if end == index {
		return false
	}
	return byteAt(line, end) == '>' || byteAt(line, end) == '<'
}

// emptyQuote reports whether the token between index and next is an empty
// quote pair, single or double.
func emptyQuote(line string, index, next int) bool {
	if next != index+2 {
		return false
	}
	return line[index] == '\'' || line[index] == '"'
}

// syntaxStep judges the token that starts at index, outside any quote, and
// answers the offset after it, or the construct that lies outside the syntax.
func syntaxStep(line string, index int) (int, string) {
	character := line[index]
	if !syntaxByte(character) {
		return 0, syntaxNonASCII
	}
	switch character {
	case '\'':
		return singleQuoted(line, index+1)
	case '"':
		return doubleQuoted(line, index+1)
	case '$':
		return dollarStep(line, index, false)
	case '#':
		return 0, syntaxHash
	case '\\':
		return 0, "a backslash outside quotes"
	case '`':
		return 0, syntaxBacktick
	case '{', '}':
		return 0, "a brace, which can group commands or expand to several words"
	case '!':
		return 0, syntaxBang
	case '(', ')':
		return 0, "a parenthesis (a subshell, a substitution or an extended glob)"
	case '[':
		if byteAt(line, index+1) == '[' {
			return 0, "a `[[` conditional"
		}
	case ';':
		if byteAt(line, index+1) == ';' {
			return 0, "a `;;`"
		}
	case '&':
		return ampersandStep(line, index)
	case '|':
		if byteAt(line, index+1) == '|' {
			return index + 2, ""
		}
		if byteAt(line, index+1) == '&' {
			return 0, "a `|&`"
		}
	case '>':
		return outputStep(line, index+1)
	case '<':
		return inputStep(line, index+1)
	}
	return index + 1, ""
}

// syntaxByte reports whether a byte is printable ASCII, a space, a tab or a
// newline.
func syntaxByte(character byte) bool {
	if character == '\t' || character == '\n' {
		return true
	}
	return ' ' <= character && character <= '~'
}

// byteAt answers the byte at index, and 0 past the end of the line.
func byteAt(line string, index int) byte {
	if index < len(line) {
		return line[index]
	}
	return 0
}

// singleQuoted judges a single-quoted string whose text starts at index.
// Inside it bash and lineCommands both read every byte as text.
func singleQuoted(line string, index int) (int, string) {
	for ; index < len(line); index++ {
		character := line[index]
		if character == '\'' {
			return index + 1, ""
		}
		if character == '\n' {
			return 0, syntaxQuoteLines
		}
		if character == '#' {
			return 0, syntaxHash
		}
		if !syntaxByte(character) {
			return 0, syntaxNonASCII
		}
	}
	return 0, syntaxQuoteOpen
}

// doubleQuoted judges a double-quoted string whose text starts at index.
func doubleQuoted(line string, index int) (int, string) {
	for index < len(line) {
		character := line[index]
		switch {
		case character == '"':
			return index + 1, ""
		case character == '\n':
			return 0, syntaxQuoteLines
		case character == '#':
			return 0, syntaxHash
		case character == '!':
			return 0, syntaxBang
		case character == '`':
			return 0, syntaxBacktick
		case character == '\\':
			// Bash keeps a backslash in double quotes unless it precedes one of
			// these, and lineCommands keeps it the same way.
			if strings.IndexByte("$`\"\\\n", byteAt(line, index+1)) >= 0 {
				return 0, "a backslash that escapes a character inside double quotes"
			}
			index++
		case character == '$':
			next, construct := dollarStep(line, index, true)
			if construct != "" {
				return 0, construct
			}
			index = next
		case !syntaxByte(character):
			return 0, syntaxNonASCII
		default:
			index++
		}
	}
	return 0, syntaxQuoteOpen
}

// dollarStep judges the `$` at index: a plain parameter, or `${NAME}` inside
// double quotes (quoted). Outside quotes lineCommands ends a command at a
// brace, so an unquoted `${NAME}` would be split where bash does not split it.
func dollarStep(line string, index int, quoted bool) (int, string) {
	next := byteAt(line, index+1)
	switch {
	case nameStart(next):
		return nameEnd(line, index+2), ""
	case strings.IndexByte("_?@*$-0123456789", next) >= 0 && next != 0:
		return index + 2, ""
	case next == '{' && quoted:
		end := nameEnd(line, index+2)
		if end == index+2 || !nameStart(line[index+2]) || byteAt(line, end) != '}' {
			return 0, "a `${...}` expansion with an operator"
		}
		return end + 1, ""
	case next == '{':
		return 0, "a `${...}` expansion outside double quotes"
	case next == '\'':
		return 0, "`$'...'` ANSI-C quoting"
	case next == '"':
		return 0, "`$\"...\"` locale quoting"
	case next == '(':
		return 0, "a `$(` command substitution or arithmetic expansion"
	}
	return 0, "a `$` that starts no plain parameter"
}

// nameStart reports whether a byte can start a shell variable name.
func nameStart(character byte) bool {
	return character == '_' || ('A' <= character && character <= 'Z') || ('a' <= character && character <= 'z')
}

// nameEnd answers the offset after the variable-name bytes that start at
// index.
func nameEnd(line string, index int) int {
	for index < len(line) {
		character := line[index]
		if nameStart(character) || ('0' <= character && character <= '9') {
			index++
			continue
		}
		break
	}
	return index
}

// ampersandStep judges the `&` at index: `&&`, or `&>` followed by a word. A
// lone `&` runs its command in the background, which lineCommands does not
// model as a separator of its own.
func ampersandStep(line string, index int) (int, string) {
	next := byteAt(line, index+1)
	if next == '&' {
		return index + 2, ""
	}
	if next != '>' {
		return 0, "a background `&`"
	}
	if byteAt(line, index+2) == '>' {
		return 0, "a `&>>` redirection"
	}
	return redirectTarget(line, index+2)
}

// outputStep judges an output redirection whose text after its first `>`
// starts at index: `>`, `>>` and `>|` followed by a word, or `>&M` onto a
// descriptor. A digit before the `>` is a word byte syntaxStep has passed.
func outputStep(line string, index int) (int, string) {
	switch byteAt(line, index) {
	case '>':
		if strings.IndexByte(">&|", byteAt(line, index+1)) >= 0 && byteAt(line, index+1) != 0 {
			return 0, "a redirection outside `>`, `>>`, `>|` and `>&N`"
		}
		return redirectTarget(line, index+1)
	case '|':
		return redirectTarget(line, index+1)
	case '&':
		end := index + 1
		for '0' <= byteAt(line, end) && byteAt(line, end) <= '9' {
			end++
		}
		if end == index+1 || !wordEnds(byteAt(line, end)) {
			return 0, "a `>&` redirection onto anything but a descriptor number"
		}
		return end, ""
	}
	return redirectTarget(line, index)
}

// inputStep judges an input redirection whose text after its first `<`
// starts at index: `<` followed by a word, or the here-string `<<<` followed
// by a word. `<<` opens a here-document, whose body the shell reads as text
// and lineCommands would read as commands.
func inputStep(line string, index int) (int, string) {
	if byteAt(line, index) != '<' {
		return redirectTarget(line, index)
	}
	if byteAt(line, index+1) != '<' {
		return 0, "a here-document (`<<`)"
	}
	if byteAt(line, index+2) == '<' {
		return 0, "a redirection outside `<` and `<<<`"
	}
	return redirectTarget(line, index+2)
}

// redirectTarget requires the word a redirection names: after spaces and
// tabs, a byte that starts a word. It answers the offset of that word, which
// syntaxStep then judges.
func redirectTarget(line string, index int) (int, string) {
	for byteAt(line, index) == ' ' || byteAt(line, index) == '\t' {
		index++
	}
	if wordEnds(byteAt(line, index)) {
		return 0, "a redirection with no word after it"
	}
	return index, ""
}

// wordEnds reports whether a byte cannot continue a word: the end of the
// line, a space, a tab, a newline or an operator character.
func wordEnds(character byte) bool {
	return character == 0 || strings.IndexByte(" \t\n;&|<>", character) >= 0
}
