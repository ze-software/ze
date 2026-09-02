// Design: docs/architecture/core-design.md -- the file-split command's grammar
//
// The command has ONE action, moving declarations, so every word after it is a
// modifier and a keyword always comes before a value (ai/rules/cli.md). Three
// keywords carry the whole request:
//
//	le go-extract source peer.go dest peer_fsm.go symbol handleOpen
//	le go-extract source peer.go dest peer_fsm.go symbol handleOpen symbol handleKeepalive
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

	"github.com/ze-software/ze/internal/core/textbuf"
)

const usage = "usage: le go-extract source <file.go> dest <file.go> symbol <name> [symbol <name> ...]"

// Answer is the `le go-extract` command.
func Answer(args []string) (any, int) {
	req, err := parseRequest(args)
	if err != nil {
		reportError(err)
		fmt.Fprintln(os.Stderr, usage) //nolint:errcheck // CLI output
		return nil, 1
	}

	report, err := Move(req, Goimports)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	return report, 0
}

// parseRequest reads the operator's words.
//
// The bound is the argument count: each keyword consumes exactly one word
// beyond itself, and a keyword with nothing after it is a refusal rather than a
// silent default. A default here would be a file path this tool then writes.
func parseRequest(args []string) (Request, error) {
	var req Request

	for index := 0; index < len(args); index++ {
		word := args[index]
		if index+1 >= len(args) {
			var tb textbuf.Buffer
			return req, errors.New(tb.Str("the keyword ").Quoted(word).Str(" needs a value after it").String())
		}
		value := args[index+1]
		index++

		switch word {
		case "source":
			req.Source = value
		case "dest":
			req.Dest = value
		case "symbol":
			req.Symbols = append(req.Symbols, value)
		default:
			var tb textbuf.Buffer
			return req, errors.New(tb.Str("unknown keyword ").Quoted(word).
				Str("; say source, dest or symbol").String())
		}
	}

	switch {
	case req.Source == "":
		return req, errors.New("say which file the declarations come out of: source <file.go>")
	case req.Dest == "":
		return req, errors.New("say which file the declarations go into: dest <file.go>")
	case len(req.Symbols) == 0:
		return req, errors.New("say what moves: symbol <name>")
	}

	return req, nil
}

// reportError writes one failure line in the spelling every ported le tool uses.
func reportError(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
}
