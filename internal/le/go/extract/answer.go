// Design: docs/architecture/core-design.md -- the file-split command's grammar
//
// The command has ONE action, moving declarations, so every word after it is a
// modifier and a keyword always comes before a value (ai/rules/cli.md). Three
// keywords carry the whole request:
//
//	le go extract source peer.go dest peer_fsm.go symbol handleOpen
//	le go extract source peer.go dest peer_fsm.go symbol handleOpen symbol handleKeepalive
//
// Every value here is a NAME the operator chose -- two file paths and a list of
// Go identifiers -- so none of them can be read as a keyword and none of them
// can be left in an untyped slot. A file called `dest.go` is a value of source
// when source is what precedes it, and a symbol called `symbol` is a value of
// the keyword before it. The script this replaced took three bare positionals,
// where `go_extract.go source dest` and `go_extract.go dest source` differ only
// in the developer's memory.

package goextract

import (
	"errors"
	"fmt"
	"os"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// moveVerb is the one action: moving declarations.
const moveVerb = "move"

// The three keywords that carry the whole request.
const (
	sourceKeyword = "source"
	destKeyword   = "dest"
	symbolKeyword = "symbol"
)

// actions is the whole command surface, declared once, so the listing, the
// usage, the help pages and the parse read one grammar (internal/le/le/action).
var actions = leaction.New("go extract", leaction.Action{
	Verb:   moveVerb,
	Why:    "move named declarations from one Go file to another, comments and formatting intact",
	Writes: true,
	Parameters: []leaction.Parameter{
		{Keyword: sourceKeyword, Kind: leaction.KindPath, Value: "file.go", Requirement: leaction.Required,
			Help: "the file the declarations come out of"},
		{Keyword: destKeyword, Kind: leaction.KindPath, Value: "file.go", Requirement: leaction.Required,
			Help: "the file the declarations go into"},
		{Keyword: symbolKeyword, Kind: leaction.KindString, Value: "name", Requirement: leaction.Required, Repeat: true,
			Help: "a declaration that moves; give it again for each one"},
	},
	AnswerArgs: answerMove,
})

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le go extract` command. A line that does not open with the
// verb moves, as callers have always typed it (D-P3-bare in
// spec-le-cli-usability), so the verb is supplied here and the table reads the
// rest of the line. A refusal the table's parser makes answers 2
// (D-P3-stray); one the body makes keeps the 1 it answered before.
func Answer(args []string) (any, int) {
	if len(args) > 0 && actions.Holds(args[0]) {
		return actions.Answer(args)
	}
	return actions.Answer(append([]string{moveVerb}, args...))
}

// answerMove moves the declarations one parsed line names.
func answerMove(args leaction.Arguments) (any, int) {
	req, err := requestOf(args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	report, err := Move(req, Goimports)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	return report, 0
}

// parseRequest reads the operator's words through the table and turns them
// into the request. It moves nothing.
func parseRequest(args []string) (Request, error) {
	parsed, err := actions.Parse(moveVerb, args)
	if err != nil {
		return Request{}, err
	}
	return requestOf(parsed)
}

// requestOf turns the parsed keywords into the request. The parser does not
// yet refuse a missing required keyword (D-P2-required in
// spec-le-cli-usability), so this does: a default here would be a file path
// this tool then writes.
func requestOf(args leaction.Arguments) (Request, error) {
	req := Request{
		Source:  args.One(sourceKeyword),
		Dest:    args.One(destKeyword),
		Symbols: args.Values(symbolKeyword),
	}
	if req.Source == "" {
		return req, errors.New("say which file the declarations come out of: source <file.go>")
	}
	if req.Dest == "" {
		return req, errors.New("say which file the declarations go into: dest <file.go>")
	}
	if len(req.Symbols) == 0 {
		return req, errors.New("say what moves: symbol <name>")
	}
	return req, nil
}

// reportError writes one failure line in the spelling every ported le tool uses.
func reportError(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
}
