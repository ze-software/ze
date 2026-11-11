// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: runtime.go -- the context that holds the one parse of a call
// Related: session.go -- commandOutsideReaders, the parse's first reader
package hookruntime

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// commandParse is the result of reading one command line as bash: the
// statement tree, or the reason the line did not parse. Exactly one of the two
// is set, and a caller asks parsed before it reads the tree.
type commandParse struct {
	// file is the statement tree, nil when the line did not parse.
	file *syntax.File
	// failure says why the line did not parse, in words a refusal carries.
	// It is set exactly when file is nil.
	failure string
}

// parsed reports whether the line parsed as bash. A line that did not is no
// reader line: nothing the hook could read from it may lift a verdict.
func (parse commandParse) parsed() bool {
	return parse.file != nil
}

// shellParse holds the one parse of a Bash call's command for the whole hook
// call. It lives in the call's shared state (callState, runtime.go): Run's
// checks each receive a copy of the context, and the copies share that
// state, so the first check that asks parses the line and every later one
// reads that result. It is not safe for concurrent use: Run calls its checks
// one after another.
type shellParse struct {
	done   bool
	result commandParse
}

// commandParse answers the parse of the call's command, parsing it on the
// first ask. A context built without newContext (a unit test's literal)
// holds no call state, so the line is parsed for that one ask and the result
// is the same.
func (ctx context) commandParse() commandParse {
	command := stringInput(ctx.input, "command")
	if ctx.call == nil {
		return parseCommandLine(command)
	}
	if !ctx.call.parse.done {
		ctx.call.parse.result = parseCommandLine(command)
		ctx.call.parse.done = true
	}
	return ctx.call.parse.result
}

// parseBashLine is the parser parseCommandLine calls. It is a variable only
// so a test can put a parser that panics in its place and prove the
// containment (TestParserPanicIsAnUnparsedLine).
var parseBashLine = parseBash

// parseBash reads a line with the bash grammar of mvdan.cc/sh, the parser
// behind shfmt. Every option is passed here rather than left to the library's
// default: the bash variant, because the Bash tool runs bash, and no comments
// in the tree, because a comment runs nothing.
func parseBash(line string) (*syntax.File, error) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash), syntax.KeepComments(false))
	return parser.Parse(strings.NewReader(line), "")
}

// parseCommandLine reads one command line as bash. The parser is third-party
// code on the path of every Bash call of every session, so a panic inside it
// is contained here and answered as a line that did not parse. Run's own
// recover would answer exit 0 for it, and a line the hook cannot read would
// then pass every check unjudged: a hook that cannot read a line never clears
// it.
func parseCommandLine(line string) (parse commandParse) {
	defer func() {
		if recovered := recover(); recovered != nil {
			parse = commandParse{failure: "the bash parser failed on it (" + fmt.Sprint(recovered) + ")"}
		}
	}()
	// The parser drops a carriage return that comes before a newline, and
	// bash keeps it in the word (TestWordValuesMatchBash: bash passes `x\r`
	// where the tree reads `x`). A tree that disagrees with bash proves
	// nothing, so such a line is one the hook cannot read.
	if strings.ContainsRune(line, '\r') {
		return commandParse{failure: "it holds a carriage return, which bash keeps in the word and the parser drops"}
	}
	file, err := parseBashLine(line)
	if err != nil {
		return commandParse{failure: err.Error()}
	}
	if file == nil {
		return commandParse{failure: "the bash parser answered no statement tree and no error"}
	}
	return commandParse{file: file}
}

// wordValue is one parsed word as bash hands it to the command that receives
// it, read by readWord.
type wordValue struct {
	// text is the word with its quoting removed. When expands is false it is
	// exactly the argument bash passes. When expands is true it holds only the
	// word's literal parts, and nothing may take it for the argument: bash
	// replaces the expansions, then may split the result or match it against
	// the files of the working directory.
	text string
	// expands reports that the shell changes the word before the command
	// receives it: a parameter expansion, a command or process substitution,
	// arithmetic, an unquoted glob character, a brace expansion, an extended
	// glob, or a tilde prefix.
	expands bool
}

// wordNotRead is readWord's answer for a word whose value it cannot give.
// Its message completes "the word ...", so a refusal can carry it as is.
type wordNotRead struct {
	reason string
}

func (err wordNotRead) Error() string {
	return "its value is not read: " + err.reason
}

// readWord reads one word of the parsed line into the value bash passes for
// it. This is the only hand-written reading of shell text the hook keeps, and
// it works inside one word the parser already delimited: the parser has
// already joined each backslash-newline outside single quotes (the lexer's
// escNewl never reaches a Lit's Value), and a Lit keeps every other backslash
// as written, so the quote removal below is bash's own:
//
//   - outside quotes, a backslash drops and keeps the next character;
//   - inside double quotes, a backslash drops only before `$`, a backtick,
//     `"`, `\` and a newline;
//   - a single-quoted part is taken whole;
//   - a `$` the parser leaves in a Lit (`"foo$"`, a lone `$`) is that
//     character.
//
// ANSI-C quoting (`$'...'`) and locale quoting (`$"..."`) answer wordNotRead:
// bash decodes their escapes or translates them, and the hook does neither.
// A part of a kind this function does not list answers wordNotRead too, so a
// word nobody taught it to read is never given a value. TestWordValuesMatchBash
// compares the reading with what a real bash passes.
func readWord(word *syntax.Word) (wordValue, error) {
	var reading wordReading
	for index, part := range word.Parts {
		switch part := part.(type) {
		case *syntax.Lit:
			reading.unquoted(part.Value, index == 0)
		case *syntax.SglQuoted:
			if part.Dollar {
				return wordValue{}, wordNotRead{reason: "it holds ANSI-C quoting ($'...'), whose escapes bash decodes and the hook does not"}
			}
			reading.text.WriteString(part.Value)
		case *syntax.DblQuoted:
			if part.Dollar {
				return wordValue{}, wordNotRead{reason: `it holds locale quoting ($"..."), which bash translates through the message catalog`}
			}
			if err := reading.doubleQuoted(part.Parts); err != nil {
				return wordValue{}, err
			}
		case *syntax.ParamExp, *syntax.CmdSubst, *syntax.ArithmExp, *syntax.ProcSubst, *syntax.ExtGlob, *syntax.BraceExp:
			reading.expands = true
		default:
			return wordValue{}, wordNotRead{reason: "it holds a part of a kind the hook does not read"}
		}
	}
	return wordValue{text: reading.text.String(), expands: reading.expands}, nil
}

// wordReading is the state of readWord across the parts of one word. The
// glob and brace marks span parts because bash matches a pattern or a brace
// expansion over the whole word: `[a"b"]` is still a bracket expression.
type wordReading struct {
	text    strings.Builder
	expands bool
	// assignment is set when the word starts with a name and `=`. Bash, out
	// of POSIX mode, expands a tilde after that `=` or after a `:` in such a
	// word even when it is a command's argument.
	assignment bool
	// bracketOpen is set by an unquoted `[`: a later unquoted `]` makes the
	// word a pattern. A lone `[` is a literal.
	bracketOpen bool
	// braceOpen and braceSeparator follow an unquoted `{` and an unquoted `,`
	// or `..` after it; a later unquoted `}` makes a brace expansion. The
	// test is wider than bash's (it ignores nesting and the shape of a
	// sequence), so a word it marks may be one bash leaves alone, which is
	// the safe side: an expanded word is never taken for its text.
	braceOpen      bool
	braceSeparator bool
}

// unquoted reads one unquoted literal. first reports that it opens the word,
// where a tilde prefix and an assignment's name can start.
func (reading *wordReading) unquoted(raw string, first bool) {
	if first && assignmentShaped(raw) {
		reading.assignment = true
	}
	for index := 0; index < len(raw); index++ {
		character := raw[index]
		if character == '\\' && index+1 < len(raw) {
			index++
			if raw[index] != '\n' {
				reading.text.WriteByte(raw[index])
			}
			continue
		}
		reading.markExpansion(raw, index, first)
		reading.text.WriteByte(character)
	}
}

// markExpansion sets expands when the unquoted, unescaped byte at raw[index]
// makes bash change the word. Every byte it tests is ASCII, so it never
// splits a UTF-8 sequence.
func (reading *wordReading) markExpansion(raw string, index int, first bool) {
	switch raw[index] {
	case '*', '?':
		reading.expands = true
	case '[':
		reading.bracketOpen = true
	case ']':
		if reading.bracketOpen {
			reading.expands = true
		}
	case '{':
		reading.braceOpen = true
	case ',':
		if reading.braceOpen {
			reading.braceSeparator = true
		}
	case '.':
		if reading.braceOpen && index+1 < len(raw) && raw[index+1] == '.' {
			reading.braceSeparator = true
		}
	case '}':
		if reading.braceOpen && reading.braceSeparator {
			reading.expands = true
		}
	case '~':
		if first && index == 0 {
			reading.expands = true
			return
		}
		if reading.assignment && index > 0 && strings.IndexByte("=:", raw[index-1]) >= 0 {
			reading.expands = true
		}
	}
}

// doubleQuoted reads the parts inside one pair of double quotes. Glob and
// brace characters are literal there, and a tilde is not a prefix.
func (reading *wordReading) doubleQuoted(parts []syntax.WordPart) error {
	for _, part := range parts {
		switch part := part.(type) {
		case *syntax.Lit:
			reading.doubleQuotedLiteral(part.Value)
		case *syntax.ParamExp, *syntax.CmdSubst, *syntax.ArithmExp:
			reading.expands = true
		default:
			return wordNotRead{reason: "it holds, inside double quotes, a part of a kind the hook does not read"}
		}
	}
	return nil
}

// doubleQuotedLiteral reads one literal inside double quotes, where a
// backslash drops only before `$`, a backtick, `"`, `\` and a newline, and a
// backslash-newline joins the lines.
func (reading *wordReading) doubleQuotedLiteral(raw string) {
	for index := 0; index < len(raw); index++ {
		character := raw[index]
		if character == '\\' && index+1 < len(raw) && strings.IndexByte("$`\"\\\n", raw[index+1]) >= 0 {
			index++
			if raw[index] != '\n' {
				reading.text.WriteByte(raw[index])
			}
			continue
		}
		reading.text.WriteByte(character)
	}
}

// assignmentShaped reports whether a word's opening literal is a shell name
// followed by `=`, the shape bash gives a tilde after `=` or `:` in.
func assignmentShaped(raw string) bool {
	equals := strings.IndexByte(raw, '=')
	if equals < 1 {
		return false
	}
	for index := range equals {
		character := raw[index]
		letter := character == '_' || (character|0x20 >= 'a' && character|0x20 <= 'z')
		if letter {
			continue
		}
		if index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}
