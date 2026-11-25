// Design: docs/contributing/navigating-the-code.md -- the symbol map of one Go file
// Overview: symbols.go -- List, the symbol map gopls would answer for the file
// Related: register.go -- le area registration
package gosymbols

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// area is the name this command is typed as.
const area = "go symbols"

// The exit codes of the action, as le's exit-code table defines them.
const (
	// exitProblem is a file that was read and does not parse.
	exitProblem = 1
	// exitUsage is a path that names no Go file.
	exitUsage = 2
	// exitCannotRun is a file that is absent or cannot be read.
	exitCannotRun = 3
)

// actions is the whole command surface. It stays light (Heavy: false): it
// reads one file, so a subagent runs it and it takes no job slot.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "list",
		Summary: "print the symbols of one Go file with their positions",
		Why: "map a file without reading it: every declaration with the position gopls definition, " +
			"references and call_hierarchy take, read from the file's syntax tree with no package loaded",
		Parameters: []leaction.Parameter{
			{
				Keyword:     "path",
				Kind:        leaction.KindPath,
				Value:       "file",
				Help:        "the Go file to map, absolute or from the current directory",
				Requirement: leaction.Required,
			},
		},
		AnswerArgs: runList,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le go symbols` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// Report is the symbol map of one file. Symbols is the one row set, so the
// row operators act on the symbols.
type Report struct {
	File    string   `json:"file"`
	Symbols []Symbol `json:"symbols"`
}

// Text renders the map exactly as `gopls symbols` prints it: one line per
// symbol, `Name Kind <line>:<col>-<line>:<col>`, each member on a line of its
// own under its type, indented by one tab.
func (r Report) Text() string {
	var tb textbuf.Buffer
	for _, symbol := range r.Symbols {
		writeSymbol(&tb, symbol)
		for _, member := range symbol.Members {
			tb.Byte('\t')
			writeSymbol(&tb, member)
		}
	}
	return tb.String()
}

// writeSymbol writes the line of one symbol.
func writeSymbol(tb *textbuf.Buffer, symbol Symbol) {
	tb.Str(symbol.Name).Byte(' ').Str(symbol.Kind.String()).Byte(' ').
		Int(int64(symbol.StartLine)).Byte(':').Int(int64(symbol.StartColumn)).Byte('-').
		Int(int64(symbol.EndLine)).Byte(':').Int(int64(symbol.EndColumn)).Byte('\n')
}

func runList(arguments leaction.Arguments) (any, int) {
	report, code, refused := load(arguments.One("path"))
	if refused != nil {
		leaction.ReportError(refused)
		return nil, code
	}
	return report, 0
}

// load answers the symbol map of the Go file at path, or the exit code and
// the operator error that name what is wrong with the path and the command
// that fixes it. The error is a result of its own, declared *opererr.Error,
// so the error inventory reads the message handed to ReportError as typed.
func load(path string) (Report, int, *opererr.Error) {
	directory := filepath.Dir(path)
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Report{}, exitCannotRun, opererr.New(
			"no file at "+path,
			"go symbols list maps one Go file that exists, and a relative path is read from the current directory",
			"ls "+filepath.Join(directory, "*.go"),
		)
	}
	if err != nil {
		return Report{}, exitCannotRun, opererr.Wrap(err,
			"cannot look at "+path,
			"go symbols list must read the file to map it",
			"ls -l "+path,
		)
	}
	if info.IsDir() {
		return Report{}, exitUsage, opererr.New(
			path+" is a directory, not a Go file",
			"go symbols list maps one file; name one of the Go files the directory holds",
			"ls "+filepath.Join(path, "*.go"),
		)
	}
	if filepath.Ext(path) != ".go" {
		return Report{}, exitUsage, opererr.New(
			path+" is not a Go file",
			"go symbols list parses Go source, so it reads only a file whose name ends in .go",
			"ls "+filepath.Join(directory, "*.go"),
		)
	}
	source, err := cliio.ReadFile(path)
	if err != nil {
		return Report{}, exitCannotRun, opererr.Wrap(err,
			"cannot read "+path,
			"go symbols list must read the file to map it",
			"ls -l "+path,
		)
	}
	symbols, err := List(path, source)
	if err != nil {
		return Report{}, exitProblem, opererr.Wrap(err,
			path+" does not parse as Go",
			"the symbol map is read from the file's syntax tree, which a syntax error leaves incomplete; "+
				"correct this error, then run the command again",
			"./le go symbols list path "+path,
		)
	}
	return Report{File: path, Symbols: symbols}, 0, nil
}
