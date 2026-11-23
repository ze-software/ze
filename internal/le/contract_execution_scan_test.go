// Design: docs/architecture/testing/ci-workflows.md -- compiled bootstrap regression fixtures.
// Related: contract_execution_test.go -- the execution/data boundary and its refusal controls.
package le

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/ze-software/ze/internal/le/heavytool"
)

// developmentToolGoRun reads process construction rather than all source text.
// It covers literal argv, file constants, exec.Cmd literals and literal shell
// bodies. Explicit run intent refuses even when its executable or target is
// unreadable. Opaque Go verbs, runtime dataflow and shell bodies supplied by
// another producer are unproven rather than certified safe.
func developmentToolGoRun(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("read development test syntax: %w", err)
	}
	constants := make(map[string]ast.Expr)
	imports := make(map[string]bool)
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return err
		}
		if path != "os/exec" {
			continue
		}
		name := "exec"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = true
	}
	ast.Inspect(file, func(node ast.Node) bool {
		declaration, ok := node.(*ast.GenDecl)
		if !ok {
			return true
		}
		if declaration.Tok != token.CONST {
			return true
		}
		for _, spec := range declaration.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range value.Names {
				if _, duplicate := constants[name.Name]; duplicate {
					constants[name.Name] = nil
					continue
				}
				if index < len(value.Values) {
					constants[name.Name] = value.Values[index]
				}
			}
		}
		return true
	})
	developmentInvalidateConstants(file, constants)
	constructors := developmentConstructorAliases(file, imports)
	ast.Inspect(file, func(node ast.Node) bool {
		if err != nil {
			return false
		}
		switch value := node.(type) {
		case *ast.CallExpr:
			words := developmentGoWords(value.Args, constants)
			var offsets []int
			if name, ok := value.Fun.(*ast.Ident); ok {
				offsets = constructors[name.Name]
			}
			if offset, constructor := developmentConstructor(value.Fun, imports); constructor {
				offsets = []int{offset}
			}
			if len(offsets) > 0 {
				for _, offset := range offsets {
					if offset < len(words) {
						err = developmentArgvGoRun(words[offset:], 0)
						if err != nil {
							break
						}
					}
				}
				return err == nil
			}
			// An unknown helper must receive explicit compiler/shell argv before
			// it is execution evidence: t.Run("go", callback) is ordinary data.
			for index := range len(words) - 1 {
				if !words[index].known {
					continue
				}
				tokens := developmentProgramWords(words[index:])
				tool, heavy := heavytool.Started(tokens)
				_, handed := heavytool.ShellCommand(tokens)
				if !handed {
					if !heavy {
						continue
					}
					if tool != "go run" {
						continue
					}
				}
				err = developmentArgvGoRun(words[index:], 0)
				if err != nil {
					break
				}
			}
		case *ast.CompositeLit:
			selector, ok := value.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if !imports[qualifier.Name] {
				return true
			}
			if selector.Sel.Name != "Cmd" {
				return true
			}
			var executable developmentWord
			var args []developmentWord
			for _, element := range value.Elts {
				field, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				name, ok := field.Key.(*ast.Ident)
				if !ok {
					continue
				}
				switch name.Name {
				case "Path":
					executable = developmentGoWord(field.Value, constants, 0)
				case "Args":
					args = developmentGoWords([]ast.Expr{field.Value}, constants)
				}
			}
			if len(args) > 0 {
				args = args[1:]
			}
			err = developmentArgvGoRun(append([]developmentWord{executable}, args...), 0)
		}
		return err == nil
	})
	return err
}

// The scan does not resolve lexical scopes. Any parameter or local declaration
// sharing a constant's name therefore invalidates that constant throughout the
// file. Such an ambiguous name cannot turn explicit compiler intent safe.
func developmentInvalidateConstants(file *ast.File, constants map[string]ast.Expr) {
	ast.Inspect(file, func(node ast.Node) bool {
		var names []*ast.Ident
		switch value := node.(type) {
		case *ast.FuncType:
			for _, fields := range []*ast.FieldList{value.Params, value.Results} {
				if fields != nil {
					for _, field := range fields.List {
						names = append(names, field.Names...)
					}
				}
			}
		case *ast.FuncDecl:
			if value.Recv != nil {
				for _, field := range value.Recv.List {
					names = append(names, field.Names...)
				}
			}
		case *ast.GenDecl:
			if value.Tok == token.VAR {
				for _, spec := range value.Specs {
					if binding, ok := spec.(*ast.ValueSpec); ok {
						names = append(names, binding.Names...)
					}
				}
			}
		case *ast.AssignStmt:
			if value.Tok == token.DEFINE {
				for _, expression := range value.Lhs {
					if name, ok := expression.(*ast.Ident); ok {
						names = append(names, name)
					}
				}
			}
		case *ast.RangeStmt:
			if value.Tok == token.DEFINE {
				for _, expression := range []ast.Expr{value.Key, value.Value} {
					if name, ok := expression.(*ast.Ident); ok {
						names = append(names, name)
					}
				}
			}
		}
		for _, name := range names {
			if _, declared := constants[name.Name]; declared {
				constants[name.Name] = nil
			}
		}
		return true
	})
}

// Direct constructor aliases remain possible constructors even if the name is
// reused elsewhere. This is a conservative declaration census, not Go dataflow.
func developmentConstructorAliases(file *ast.File, imports map[string]bool) map[string][]int {
	aliases := make(map[string][]int)
	ast.Inspect(file, func(node ast.Node) bool {
		var names []*ast.Ident
		var values []ast.Expr
		switch value := node.(type) {
		case *ast.ValueSpec:
			names, values = value.Names, value.Values
		case *ast.AssignStmt:
			values = value.Rhs
			for _, expression := range value.Lhs {
				name, ok := expression.(*ast.Ident)
				if !ok {
					name = nil
				}
				names = append(names, name)
			}
		}
		for index, name := range names {
			if name == nil {
				continue
			}
			if index < len(values) {
				if offset, constructor := developmentConstructor(values[index], imports); constructor {
					aliases[name.Name] = append(aliases[name.Name], offset)
				}
			}
		}
		return true
	})
	return aliases
}

func developmentConstructor(expression ast.Expr, imports map[string]bool) (int, bool) {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return 0, false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return 0, false
	}
	if !imports[qualifier.Name] {
		return 0, false
	}
	switch selector.Sel.Name {
	case "Command":
		return 0, true
	case "CommandContext":
		return 1, true
	}
	return 0, false
}

// A zero word is unreadable, never an empty literal. Literal fragments preserve
// opaque positions; they can prove compiler intent, never certify a value.
// A missing executable followed by run is refused.
type developmentWord struct {
	text  string
	known bool
}

// This marker preserves an unknown word or word part while parsing a partial
// shell body. Any parsed word containing it remains unreadable: the marker is
// never evidence of the value the runtime expression supplies.
const developmentOpaque = "__ZE_UNREADABLE_WORD_PART__"

func developmentGoWords(expressions []ast.Expr, constants map[string]ast.Expr) []developmentWord {
	var words []developmentWord
	for _, expression := range expressions {
		if literal, ok := expression.(*ast.CompositeLit); ok {
			for _, element := range literal.Elts {
				words = append(words, developmentGoWord(element, constants, 0))
			}
			continue
		}
		words = append(words, developmentGoWord(expression, constants, 0))
	}
	return words
}

// The depth limit bounds constant aliases and refuses cycles without type
// checking or interpreting arbitrary Go expressions.
func developmentGoWord(expression ast.Expr, constants map[string]ast.Expr, depth int) developmentWord {
	if depth >= 16 {
		return developmentWord{}
	}
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return developmentWord{}
		}
		text, err := strconv.Unquote(value.Value)
		return developmentWord{text: text, known: err == nil}
	case *ast.Ident:
		return developmentGoWord(constants[value.Name], constants, depth+1)
	case *ast.ParenExpr:
		return developmentGoWord(value.X, constants, depth+1)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return developmentWord{}
		}
		left := developmentGoWord(value.X, constants, depth+1)
		right := developmentGoWord(value.Y, constants, depth+1)
		return developmentWord{text: developmentFragment(left) + developmentFragment(right), known: left.known && right.known}
	}
	return developmentWord{}
}

func developmentFragment(word developmentWord) string {
	if word.known {
		return word.text
	}
	if word.text != "" {
		return word.text
	}
	return developmentOpaque
}

// Only the compiler's command position decides whether run is executed. A
// compiled fixture's later data argument named run is not a Go subcommand.
func developmentArgvGoRun(words []developmentWord, depth int) error {
	if len(words) < 2 {
		return nil
	}
	if depth >= 16 {
		return fmt.Errorf("compiler execution exceeds the static nesting bound")
	}
	tokens := developmentProgramWords(words)
	if tool, heavy := heavytool.Started(tokens); heavy {
		if tool == "go run" {
			return fmt.Errorf("development test invokes the Go run subcommand")
		}
	}
	if _, first, end, handed := heavytool.ShellCommandWords(tokens); handed {
		if first < end {
			partial := append([]string(nil), tokens...)
			literal := false
			for index := first; index < end; index++ {
				partial[index] = words[index].text
				if words[index].text != "" {
					literal = true
				}
			}
			if !literal {
				return nil // The body comes entirely from another producer.
			}
			line, read := heavytool.ShellCommand(partial)
			if read {
				return developmentShellGoRun(line, depth+1)
			}
		}
	}
	program := heavytool.Program(tokens)
	if len(program) < 2 {
		return nil
	}
	at := len(tokens) - len(program)
	if !words[at].known {
		if program[1] == "run" {
			return fmt.Errorf("unresolved executable receives the run subcommand")
		}
		return nil
	}
	return nil
}

// Wrapper options and Go's directory flag belong to heavytool. Normalize only
// executable paths before asking that producer; never normalize a handed line.
// Each pass consumes a path or stops, bounded by the number of argv words.
func developmentProgramWords(words []developmentWord) []string {
	tokens := make([]string, len(words))
	for index, word := range words {
		tokens[index] = word.text
		if !word.known {
			if word.text == "" {
				tokens[index] = "\x00"
			}
		}
	}
	for range tokens {
		if _, handed := heavytool.ShellCommand(tokens); handed {
			break
		}
		program := heavytool.Program(tokens)
		if len(program) == 0 {
			break
		}
		name := filepath.Base(program[0])
		if name == program[0] {
			break
		}
		tokens[len(tokens)-len(program)] = name
	}
	return tokens
}

func developmentShellGoRun(body string, depth int) error {
	if depth >= 16 {
		return fmt.Errorf("shell execution exceeds the static nesting bound")
	}
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(body), "fixture.sh")
	if err != nil {
		return fmt.Errorf("read development test shell: %w", err)
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		if err != nil {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		var words []developmentWord
		for _, argument := range call.Args {
			words = append(words, developmentShellWord(argument.Parts, 0))
		}
		tokens := developmentProgramWords(words)
		program := heavytool.Program(tokens)
		if len(program) > 0 {
			at := len(tokens) - len(program)
			if words[at].known {
				if program[0] == "eval" {
					var fragments []string
					for _, word := range words[at+1:] {
						fragments = append(fragments, word.text)
					}
					err = developmentShellGoRun(strings.Join(fragments, " "), depth+1)
					return err == nil
				}
			}
		}
		err = developmentArgvGoRun(words, depth+1)
		return err == nil
	})
	return err
}

// This reads only statically literal shell words from the parser's structure.
// Expansion and escape semantics remain unreadable rather than guessed; nested
// command substitutions are independently visited by syntax.Walk.
func developmentShellWord(parts []syntax.WordPart, depth int) developmentWord {
	if depth >= 16 {
		return developmentWord{}
	}
	var text strings.Builder
	known := true
	for _, part := range parts {
		switch value := part.(type) {
		case *syntax.Lit:
			if strings.ContainsAny(value.Value, "\\*?[") {
				known = false
			}
			if strings.Contains(value.Value, developmentOpaque) {
				known = false
			}
			text.WriteString(value.Value)
		case *syntax.SglQuoted:
			if value.Dollar {
				known = false
			}
			if strings.Contains(value.Value, developmentOpaque) {
				known = false
			}
			text.WriteString(value.Value)
		case *syntax.DblQuoted:
			if value.Dollar {
				known = false
			}
			word := developmentShellWord(value.Parts, depth+1)
			if !word.known {
				known = false
			}
			text.WriteString(word.text)
		default:
			// Shell word parts are an open parser input. An unhandled part
			// cannot establish a safe executable or compiler subcommand.
			known = false
			text.WriteString(developmentOpaque)
		}
	}
	return developmentWord{text: text.String(), known: known}
}
