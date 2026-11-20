// Design: docs/architecture/api/commands.md -- every reader of a piped line goes through the one partition.

package command

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// chainGrammar names the functions that read a pipe chain out of a line, the
// one grammar's readers. A function that calls one of them and is not one of
// them is a reader of the chain.
var chainGrammar = []string{
	"ParsePipe", "parsePipeChain", "parseExecutionPipeChain",
	"chainPipeOps", "parsePipeOps", "scanChain", "scanExecution",
}

// chainPartition names the functions that partition a chain for its command:
// the declared list, the residual chain and its metadata (partitionPipeChain).
var chainPartition = []string{"partitionPipeChain", "prepareDocumentPipes"}

// chainReadersWithoutPartition names the readers of a chain that hold no
// command whose declared operations could be taken, each with the reason. A
// name here that no longer reads a chain is stale and fails the test.
var chainReadersWithoutPartition = map[string]string{
	"ScanExecution":                 "answers the command part's words; the chain is read only to refuse an unfinished quote",
	"TypedLineTokens":               "the accounting line: every word typed, chain included, judged by AAA as typed",
	"checkAlias":                    "an alias expansion is a chain with no command",
	"ProcessStandalonePipesChecked": "`ze pipe` runs operators over stdin with no command",
	"PresentStreamSuffix":           "the client presents the catalog's presentation suffix alone; the daemon partitions (R-10)",
}

// TestEveryPipeSplitGoesThroughThePartition asks R-11's question of the
// command package and of every package that reads command lines: does a
// reader of a piped line split it only through the one partition?
//
// Method, inside the package: parse the non-test files and name every
// function that calls a chain reader of the grammar. It must call the
// partition too, or be one of the few readers that hold no command
// (chainReadersWithoutPartition). Outside the package: every non-test file of
// internal, cmd and pkg that imports this package reads command lines, so a
// strings call that cuts at a pipe literal there, or a call of ParsePipe,
// splits a chain on its own and fails the test.
//
// MUTATION: make HasFormatPipe call parsePipeChain without partitionPipeChain,
// or put `strings.Cut(line, "|")` back in cli subjectOf; this test fails.
func TestEveryPipeSplitGoesThroughThePartition(t *testing.T) {
	t.Run("inside the command package", everyPackageReaderPartitions)
	t.Run("in every package that imports it", noImporterSplitsAChain)
}

func everyPackageReaderPartitions(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := token.NewFileSet()
	readers := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(files, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			function, isFunction := decl.(*ast.FuncDecl)
			if !isFunction || function.Body == nil {
				continue
			}
			owner := function.Name.Name
			if slices.Contains(chainGrammar, owner) {
				continue
			}
			readsChain, partitions := false, false
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				callee, isIdent := call.Fun.(*ast.Ident)
				if !isIdent {
					return true
				}
				if slices.Contains(chainGrammar, callee.Name) {
					readsChain = true
				}
				if slices.Contains(chainPartition, callee.Name) {
					partitions = true
				}
				return true
			})
			if !readsChain {
				continue
			}
			readers[owner] = true
			if partitions {
				continue
			}
			if _, exempt := chainReadersWithoutPartition[owner]; exempt {
				continue
			}
			t.Errorf("%s: %s reads a pipe chain without the partition (partitionPipeChain)",
				files.Position(function.Pos()), owner)
		}
	}
	for name := range chainReadersWithoutPartition {
		if !readers[name] {
			t.Errorf("chainReadersWithoutPartition names %s, which reads no pipe chain: remove the entry", name)
		}
	}
	if len(readers) == 0 {
		t.Fatal("no reader of a pipe chain found: the scan read nothing")
	}
}

// commandImportPath is this package's import path, which marks a file that
// reads command lines.
const commandImportPath = "github.com/ze-software/ze/internal/component/command"

// pipeCuts names the strings functions that cut or search a line.
var pipeCuts = []string{
	"Cut", "Split", "SplitN", "SplitSeq", "SplitAfter", "SplitAfterN", "FieldsFunc",
	"Index", "IndexByte", "IndexRune", "IndexAny", "LastIndex", "LastIndexByte", "LastIndexAny",
	"Contains", "ContainsRune", "ContainsAny", "Count",
}

func noImporterSplitsAChain(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("the module root %s holds no go.mod: %v", root, err)
	}
	self := filepath.Join(root, "internal", "component", "command")
	files := token.NewFileSet()
	importers := 0
	for _, tree := range []string{"internal", "cmd", "pkg"} {
		walkErr := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				name := entry.Name()
				if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if filepath.Dir(path) == self {
				return nil
			}
			header, err := parser.ParseFile(files, path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			commandName, stringsName, imports := importedNames(header)
			if !imports {
				return nil
			}
			importers++
			file, err := parser.ParseFile(files, path, nil, 0)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				owner := "a package-level value"
				if function, isFunction := decl.(*ast.FuncDecl); isFunction {
					owner = function.Name.Name
				}
				if _, exempt := pipeCutsOfNoCommandLine[filepath.ToSlash(relative)+" "+owner]; exempt {
					continue
				}
				inspectImporterDecl(t, files, decl, commandName, stringsName)
			}
			return nil
		})
		if walkErr != nil {
			t.Fatal(walkErr)
		}
	}
	if importers == 0 {
		t.Fatal("no file imports the command package: the scan read nothing")
	}
}

// pipeCutsOfNoCommandLine names, by file and function, the cuts at a pipe in
// a file that imports this package whose text is not a command line, each
// with the reason.
var pipeCutsOfNoCommandLine = map[string]string{
	"internal/le/le/root/dispatch.go areaPage":                                          "splits the alternatives of a grammar summary, written ` | ` between them",
	"internal/le/doc/yangcontract/command_surfaces.go primaryMarkdownCommandIdentities": "reads the cells of a Markdown table row",
}

// inspectImporterDecl reports, in one declaration of a file that imports this
// package, each call of ParsePipe and each strings call that cuts at a pipe.
func inspectImporterDecl(t *testing.T, files *token.FileSet, decl ast.Decl, commandName, stringsName string) {
	t.Helper()
	ast.Inspect(decl, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		pkg, isIdent := selector.X.(*ast.Ident)
		if !isIdent {
			return true
		}
		if pkg.Name == commandName && selector.Sel.Name == "ParsePipe" {
			t.Errorf("%s: calls ParsePipe, which splits the chain without the partition; "+
				"read the command part with ReadCommandPart, or the line with the route's reader",
				files.Position(call.Pos()))
		}
		if pkg.Name == stringsName && slices.Contains(pipeCuts, selector.Sel.Name) && cutsAtPipe(call.Args) {
			t.Errorf("%s: strings.%s cuts a command line at a pipe, outside the one grammar; "+
				"use command.ReadCommandPart", files.Position(call.Pos()), selector.Sel.Name)
		}
		return true
	})
}

// importedNames answers the local names a file gives this package and the
// strings package, and whether it imports this package at all.
func importedNames(file *ast.File) (commandName, stringsName string, imports bool) {
	commandName, stringsName = "command", "strings"
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		switch path {
		case commandImportPath:
			imports = true
			if spec.Name != nil {
				commandName = spec.Name.Name
			}
		case "strings":
			if spec.Name != nil {
				stringsName = spec.Name.Name
			}
		}
	}
	return commandName, stringsName, imports
}

// cutsAtPipe reports whether an argument is a pipe literal, alone or between
// spaces, as a string or a rune.
func cutsAtPipe(args []ast.Expr) bool {
	for _, arg := range args {
		literal, isLiteral := arg.(*ast.BasicLit)
		if !isLiteral {
			continue
		}
		switch literal.Value {
		case `"|"`, `'|'`, `" | "`, `"| "`, `" |"`:
			return true
		}
	}
	return false
}

// TestReadCommandPartReadsTheOneGrammar asks whether the command part a caller
// reads is the one the routes run: a quoted pipe is part of a word, a bare or
// joined one starts the chain, and malformed input stays whole.
func TestReadCommandPartReadsTheOneGrammar(t *testing.T) {
	for _, tc := range []struct {
		input, want string
		piped       bool
	}{
		{"show x | count", "show x", true},
		{"show x|count", "show x", true},
		{"show x |", "show x", true},
		{`show x "a|b"`, `show x "a|b"`, false},
		{"  show x  ", "show x", false},
		{`show x "a | count`, `show x "a | count`, false},
	} {
		got, piped := ReadCommandPart(tc.input)
		if got != tc.want || piped != tc.piped {
			t.Errorf("ReadCommandPart(%q) = (%q, %v), want (%q, %v)", tc.input, got, piped, tc.want, tc.piped)
		}
	}
}

// TestReadLocalLineHandsDeclaredOperationsAndRendersTheResidual asks whether a
// route with a handler of its own (`le <tool>`) reads its line through the
// partition: the declared operation comes back as the list, and the residual
// renders the answer exactly as ProcessPipesDefaultFormatLocal does.
//
// MUTATION: make DeclaredOperations answer nil; the list comparison fails.
func TestReadLocalLineHandsDeclaredOperationsAndRendersTheResidual(t *testing.T) {
	const path = "show test local line declared"
	RegisterPipeFilters([]string{path}, PipeFilter{Name: "only", Description: "Keep the rows of one state", Argument: ArgText})
	line := path + " alpha | only up | count | json"

	read, problem := ReadLocalLine(line, "")
	if problem != "" {
		t.Fatalf("ReadLocalLine refused %q: %s", line, problem)
	}
	want := []PipeOperation{{Name: "only", Args: []string{"up"}}}
	if !slices.EqualFunc(want, read.DeclaredOperations(), sameLocalPipeOperation) {
		t.Errorf("declared list = %+v, want %+v", read.DeclaredOperations(), want)
	}

	document, err := json.Marshal(map[string]any{"rows": []any{map[string]any{"state": "up"}, map[string]any{"state": "up"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, format, problem := ProcessPipesDefaultFormatLocal(line, "")
	if problem != "" {
		t.Fatalf("ProcessPipesDefaultFormatLocal refused %q: %s", line, problem)
	}
	if got, wantText := read.Render(string(document)), format(string(document)); got != wantText {
		t.Errorf("Render = %q, want the local formatter's %q", got, wantText)
	}
	if rendered := read.Render(string(document)); !strings.Contains(rendered, `"count": 2`) {
		t.Errorf("Render = %q, want the residual count of two rows", rendered)
	}
}
