// Design: docs/architecture/plugin/feature-gates.md -- finite retained assembly symbol grammar.
// Related: assembly.go -- complete target-body and linkage certificate.
package repoinventory

import (
	"bytes"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"
)

type assemblyDefinition struct {
	symbol    string
	path      string
	header    []string
	bodyStart int
	bodyEnd   int
	macros    map[string]bool
	// locals holds, for a TEXT definition, the file's local macros defined
	// where its body starts; localNames every name the file defines locally.
	// leafExpansion expands the body from them.
	locals     map[string]*assemblyLocalMacro
	localNames map[string]bool
}

type assemblyStatement struct {
	tokens        []string
	start         int
	end           int
	functionMacro bool
}

// assemblyStatements lexes at most the retained byte length. Comments and
// separators are processed before recognizing definitions; it expands no macros
// and interprets no instructions. The visitor MUST NOT retain the token slice.
func assemblyStatements(raw []byte, name string, visit func(assemblyStatement) error) error {
	var input scanner.Scanner
	input.Init(bytes.NewReader(raw))
	input.Filename = name
	input.Whitespace = 1<<' ' | 1<<'\t' | 1<<'\r'
	input.Mode = scanner.ScanIdents | scanner.ScanInts | scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanComments
	input.IsIdentRune = func(ch rune, index int) bool {
		if unicode.IsLetter(ch) {
			return true
		}
		if ch == '_' {
			return true
		}
		if ch == '·' {
			return true
		}
		if ch == '∕' {
			return true
		}
		return index > 0 && unicode.IsDigit(ch)
	}
	var lexicalError error
	input.Error = func(current *scanner.Scanner, message string) {
		if lexicalError == nil {
			lexicalError = fmt.Errorf("%s:%d: %s", name, current.Position.Line, message)
		}
	}
	next := func() rune {
		for {
			token := input.Scan()
			if token != scanner.Comment {
				return token
			}
		}
	}
	statement := assemblyStatement{}
	nameEndColumn := 0
	flush := func(end int) error {
		if len(statement.tokens) == 0 {
			return nil
		}
		statement.end = end
		err := visit(statement)
		statement.tokens = statement.tokens[:0]
		statement.functionMacro = false
		nameEndColumn = 0
		return err
	}
	for {
		token := next()
		if lexicalError != nil {
			return lexicalError
		}
		if token == scanner.EOF {
			if assemblyDefine(statement.tokens) && len(statement.tokens) > 3 {
				return fmt.Errorf("%s: missing newline in macro definition", name)
			}
			return flush(len(raw))
		}
		if token == '\\' && assemblyDefine(statement.tokens) {
			escaped := next()
			if lexicalError != nil {
				return lexicalError
			}
			if escaped == '\n' {
				statement.tokens = append(statement.tokens, "\n")
				continue
			}
			if escaped == '\\' {
				statement.tokens = append(statement.tokens, "\\")
				continue
			}
			return fmt.Errorf("%s:%d: unsupported macro escape", name, input.Position.Line)
		}
		if token == '\n' {
			if err := flush(input.Position.Offset + 1); err != nil {
				return err
			}
			continue
		}
		if token == ';' {
			if !assemblyDefine(statement.tokens) {
				if err := flush(input.Position.Offset + 1); err != nil {
					return err
				}
				continue
			}
		}
		if len(statement.tokens) == 0 {
			statement.start = input.Position.Offset
		}
		if assemblyDefine(statement.tokens) {
			if len(statement.tokens) == 2 {
				nameEndColumn = input.Pos().Column
			}
			if len(statement.tokens) == 3 && token == '(' {
				// Match Input.macroDefinition's column-based adjacency rule,
				// including intervening comments without statement separators.
				statement.functionMacro = input.Pos().Column == nameEndColumn+1
			}
		}
		statement.tokens = append(statement.tokens, input.TokenText())
	}
}

func assemblyDefine(tokens []string) bool {
	return len(tokens) >= 2 && tokens[0] == "#" && tokens[1] == "define"
}

// assemblyUndef reports a complete `#undef name` statement.
func assemblyUndef(tokens []string) bool {
	return len(tokens) == 3 && tokens[0] == "#" && tokens[1] == "undef"
}

func assemblySymbol(raw, prefix string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty assembly symbol")
	}
	for index, ch := range raw {
		if unicode.IsLetter(ch) {
			continue
		}
		if ch == '_' {
			continue
		}
		if ch == '·' {
			continue
		}
		if ch == '∕' {
			continue
		}
		if index > 0 && unicode.IsDigit(ch) {
			continue
		}
		return "", fmt.Errorf("unsupported assembly symbol %q", raw)
	}
	name := strings.ReplaceAll(raw, "·", ".")
	name = strings.ReplaceAll(name, "∕", "/")
	if strings.HasPrefix(name, ".") {
		name = prefix + name
	}
	return name, nil
}

// assemblyDefinitions retains headers and body byte ranges, not tokenized
// unrelated instructions. Unknown preprocessing cannot establish noninterference.
func assemblyDefinitions(raw []byte, name, prefix string, unit *assemblyNamespace) ([]assemblyDefinition, error) {
	var definitions []assemblyDefinition
	active := -1
	blocks := assemblyConditionals{}
	err := assemblyStatements(raw, name, func(statement assemblyStatement) error {
		tokens, start, end := statement.tokens, statement.start, statement.end
		if tokens[0] == "#" {
			handled, err := blocks.accept(tokens)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			if handled {
				return nil
			}
			if assemblyDefine(tokens) {
				if blocks.inside() {
					return fmt.Errorf("%s: conditional assembly changes namespace through define", name)
				}
				if err := assemblyMacro(statement, unit); err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
				return nil
			}
			// An #undef inside a conditional would change the namespace in one
			// arm only, so it stays unsupported preprocessing, refused below.
			if assemblyUndef(tokens) && !blocks.inside() {
				if err := unit.undefine(tokens[2]); err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
				return nil
			}
			if len(tokens) != 3 {
				return fmt.Errorf("%s: unsupported assembly preprocessing", name)
			}
			if tokens[1] != "include" {
				return fmt.Errorf("%s: unsupported assembly preprocessing %s", name, tokens[1])
			}
			if blocks.inside() {
				return fmt.Errorf("%s: conditional assembly changes namespace through include", name)
			}
			quoted := tokens[2]
			if len(quoted) < 2 {
				return fmt.Errorf("%s: unproved assembly include %s", name, quoted)
			}
			if quoted[0] != '"' {
				return fmt.Errorf("%s: unproved assembly include %s", name, quoted)
			}
			if quoted[len(quoted)-1] != '"' {
				return fmt.Errorf("%s: unproved assembly include %s", name, quoted)
			}
			if err := unit.include(quoted[1 : len(quoted)-1]); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return nil
		}
		// Parser.line consumes leading labels before classifying the opcode.
		// Keep start unchanged so the preceding body's byte range ends before
		// the complete labelled definition, not halfway through its statement.
		for len(tokens) >= 2 && tokens[1] == ":" {
			if _, err := assemblySymbol(tokens[0], ""); err != nil {
				return fmt.Errorf("%s: unsupported assembly label: %w", name, err)
			}
			tokens = tokens[2:]
		}
		if len(tokens) == 0 {
			return nil
		}
		if tokens[0] == "#" {
			return fmt.Errorf("%s: unsupported label-prefixed preprocessing", name)
		}
		if len(tokens) > 1 && tokens[1] == "=" {
			if blocks.inside() {
				return fmt.Errorf("%s: conditional assembly changes namespace through assignment", name)
			}
			return fmt.Errorf("%s: unsupported assembly symbol assignment", name)
		}
		if err := macroDefinitionOrigins(tokens); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		definition := assemblyDefinitionWord(tokens[0])
		if !definition {
			return nil
		}
		if blocks.inside() {
			return fmt.Errorf("%s: conditional assembly changes namespace through %s", name, tokens[0])
		}
		if tokens[0] == "TEXT" {
			if active >= 0 {
				definitions[active].bodyEnd = start
			}
			active = len(definitions)
		}
		definitionRecord := assemblyDefinition{path: name, header: append([]string(nil), tokens...), bodyStart: end, bodyEnd: len(raw)}
		if tokens[0] != "TEXT" {
			definitionRecord.bodyEnd = end
		}
		if tokens[0] == "TEXT" {
			definitionRecord.locals = maps.Clone(unit.current)
		}
		definitions = append(definitions, definitionRecord)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := blocks.finish(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for index := range definitions {
		if err := macroFreeTokens(definitions[index].header, unit.expandable); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		symbol, err := assemblyDefinitionSymbol(definitions[index].header, prefix)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		definitions[index].symbol = symbol
		definitions[index].macros = unit.expandable
		definitions[index].localNames = unit.local
	}
	return definitions, nil
}

// assemblyDefinitionSymbol proves the definition's base symbol even when its
// unrelated body is not a trampoline. Unknown address/qualifier grammar cannot
// be treated as absence of a potentially conflicting definition.
func assemblyDefinitionSymbol(tokens []string, prefix string) (string, error) {
	if len(tokens) < 2 {
		return "", fmt.Errorf("missing assembly definition symbol")
	}
	symbol, err := assemblySymbol(tokens[1], prefix)
	if err != nil {
		return "", err
	}
	position := 2
	if position < len(tokens) && tokens[position] == "<" {
		position++
		if position < len(tokens) && tokens[position] != ">" {
			position++
		}
		if position >= len(tokens) {
			return "", fmt.Errorf("unclosed assembly symbol attributes")
		}
		if tokens[position] != ">" {
			return "", fmt.Errorf("unsupported assembly symbol attributes")
		}
		position++
	}
	if position < len(tokens) && (tokens[position] == "+" || tokens[position] == "-") {
		if tokens[0] != "DATA" {
			return "", fmt.Errorf("unsupported assembly definition offset")
		}
		position++
		if position >= len(tokens) {
			return "", fmt.Errorf("missing assembly data offset")
		}
		if _, err := strconv.ParseUint(tokens[position], 0, 32); err != nil {
			return "", fmt.Errorf("unsupported assembly data offset: %w", err)
		}
		position++
	}
	if len(tokens)-position < 3 {
		return "", fmt.Errorf("incomplete assembly definition address")
	}
	if tokens[position] != "(" {
		return "", fmt.Errorf("unsupported assembly package/symbol grammar")
	}
	if tokens[position+1] != "SB" {
		return "", fmt.Errorf("unsupported assembly definition base")
	}
	if tokens[position+2] != ")" {
		return "", fmt.Errorf("unsupported assembly definition address")
	}
	return symbol, nil
}
