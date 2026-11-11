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
	file, err := parseBashLine(line)
	if err != nil {
		return commandParse{failure: err.Error()}
	}
	if file == nil {
		return commandParse{failure: "the bash parser answered no statement tree and no error"}
	}
	return commandParse{file: file}
}
