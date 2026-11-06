// Design: docs/architecture/plugin/feature-gates.md -- a leaf body is judged on the statements the Go assembler reads once its file's own macros expand.
// Related: assemblyleaf.go -- leafBody walks the expanded statements; assemblymacros.go -- parseAssemblyMacro reads each definition.
package repoinventory

import (
	"fmt"
	"maps"
	"slices"
)

// leafExpansionTokensMax bounds the tokens the macro expansion of one entered
// body may push, counting one more for each invocation. The xxhash bodies
// push a few hundred. Sixteen definitions that each double the one before
// reach 65536, and every further one doubles it again, so the bound refuses
// such a file instead of letting it grow without end.
const leafExpansionTokensMax = 1 << 16

// assemblyLocalMacro is one #define of the assembly file itself, as
// parseAssemblyMacro read it.
type assemblyLocalMacro struct {
	function bool     // the name is followed directly by its formals
	formals  []string // the normalized formal names, in order; function-like only
	body     []string // the replacement tokens as written; "\n" and ";" end a statement
}

// leafToken is one token waiting to be read during an expansion.
type leafToken struct {
	text  string
	chain []string // the macros whose replacement produced this token, outermost first
}

// leafExpansion is the macro state of one entered body while leafBody reads
// it. It is not safe for concurrent use.
type leafExpansion struct {
	defined map[string]*assemblyLocalMacro // the file's local macros defined at this point of the body
	local   map[string]bool                // every name the file defines locally, anywhere in it
	budget  int                            // the tokens the body may still push
}

// newLeafExpansion starts from the local macros defined where the body
// starts. It copies them, because a #define or #undef inside the body changes
// only this read.
func newLeafExpansion(definition *assemblyDefinition) *leafExpansion {
	defined := make(map[string]*assemblyLocalMacro, len(definition.locals))
	maps.Copy(defined, definition.locals)
	return &leafExpansion{defined: defined, local: definition.localNames, budget: leafExpansionTokensMax}
}

// directive applies a #define or #undef the body holds to the statements
// after it. The census (assemblyDefinitions) already refused a redefinition,
// an #undef of a name that is no local macro, and either one inside a
// conditional. Any other directive is an #include the census admitted, which
// binds no local macro.
func (expansion *leafExpansion) directive(statement assemblyStatement) error {
	if assemblyDefine(statement.tokens) {
		name, macro, err := parseAssemblyMacro(statement)
		if err != nil {
			return err
		}
		if expansion.defined[name] != nil {
			return fmt.Errorf("assembly macro %s is redefined", name)
		}
		expansion.defined[name] = macro
		return nil
	}
	if !assemblyUndef(statement.tokens) {
		return nil
	}
	name, err := assemblySymbol(statement.tokens[2], "")
	if err != nil {
		return fmt.Errorf("unsupported #undef name: %w", err)
	}
	delete(expansion.defined, name)
	return nil
}

// statements expands the tokens of one source statement and returns the
// statements the assembler reads from them, so every leaf rule judges the
// expanded body and never the text a macro hides it behind.
//
// The expansion is cmd/asm's (lex.Input.Next and invokeMacro). An identifier
// naming a defined macro is replaced: an object-like macro by its body, a
// function-like one by its body with each formal replaced by the raw tokens of
// its argument (leafActuals). The result is read again, so a macro in a body
// or an argument expands in turn, and a newline or `;` a body holds ends a
// statement. The refusals cover what the assembler refuses, and what it reads
// as plain text where a leaf rule would then judge a name instead of its
// replacement:
//   - a local macro name where it is not defined, before its #define or after
//     its #undef: the assembler reads it as a plain identifier;
//   - a macro whose expansion reaches itself: the assembler fails with
//     "recursive macro invocation" or never ends;
//   - an expansion pushing more than leafExpansionTokensMax tokens.
//
// A body never holds a definition word or a directive: parseAssemblyMacro
// refuses one in the definition, and macroDefinitionOrigins one passed as an
// argument, so no expansion yields a TEXT, GLOBL or DATA line.
func (expansion *leafExpansion) statements(tokens []string) ([][]string, error) {
	pending := make([]leafToken, 0, len(tokens))
	for offset := len(tokens) - 1; offset >= 0; offset-- {
		pending = append(pending, leafToken{text: tokens[offset]})
	}
	var statements [][]string
	var current []string
	for len(pending) > 0 {
		head := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		name := normalizedAssemblyToken(head.text)
		if !expansion.local[name] {
			current, statements = leafEmit(head.text, current, statements)
			continue
		}
		macro := expansion.defined[name]
		if macro == nil {
			return nil, fmt.Errorf("assembly macro %s is used where it is not defined: before its #define or after its #undef", head.text)
		}
		if slices.Contains(head.chain, name) {
			return nil, fmt.Errorf("assembly macro %s expands to itself", head.text)
		}
		var actuals [][]leafToken
		if macro.function {
			var err error
			actuals, pending, err = leafActuals(head.text, macro, pending)
			if err != nil {
				return nil, err
			}
		}
		replacement := leafReplacement(macro, actuals, append(slices.Clip(head.chain), name))
		expansion.budget -= len(replacement) + 1
		if expansion.budget < 0 {
			return nil, fmt.Errorf("assembly macro expansion of one body passes %d tokens", leafExpansionTokensMax)
		}
		for offset := len(replacement) - 1; offset >= 0; offset-- {
			pending = append(pending, replacement[offset])
		}
	}
	if len(current) != 0 {
		statements = append(statements, current)
	}
	return statements, nil
}

// leafStatementEnd reports the tokens that end a statement inside a macro body.
func leafStatementEnd(text string) bool {
	return text == "\n" || text == ";"
}

// leafEmit adds one expanded token to the statement being built, or ends that
// statement at a newline or `;`.
func leafEmit(text string, current []string, statements [][]string) ([]string, [][]string) {
	if !leafStatementEnd(text) {
		return append(current, text), statements
	}
	if len(current) == 0 {
		return nil, statements
	}
	return nil, append(statements, current)
}

// leafActuals reads a function-like macro's argument list from the top of
// pending, as cmd/asm's Input.argsFor and Input.collectArgument do: one
// argument per comma outside nested parentheses, up to the closing
// parenthesis. It returns the arguments and what stays pending. The
// assembler reads a name with no argument list as plain text, and refuses an
// argument list that a statement end interrupts ("unterminated arg list") or
// whose count differs from the formals ("wrong arg count for macro"); each of
// those refuses here.
func leafActuals(name string, macro *assemblyLocalMacro, pending []leafToken) ([][]leafToken, []leafToken, error) {
	if len(pending) == 0 {
		return nil, nil, fmt.Errorf("function-like assembly macro %s is used without its argument list", name)
	}
	if pending[len(pending)-1].text != "(" {
		return nil, nil, fmt.Errorf("function-like assembly macro %s is used without its argument list", name)
	}
	pending = pending[:len(pending)-1]
	var actuals [][]leafToken
	var actual []leafToken
	depth := 0
	// Each pass consumes one pending token, so the loop ends with pending.
	for {
		if len(pending) == 0 {
			return nil, nil, fmt.Errorf("assembly macro %s has an unterminated argument list", name)
		}
		token := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if leafStatementEnd(token.text) {
			return nil, nil, fmt.Errorf("assembly macro %s has an unterminated argument list", name)
		}
		if depth == 0 && token.text == ")" {
			actuals = append(actuals, actual)
			break
		}
		if depth == 0 && token.text == "," {
			actuals = append(actuals, actual)
			actual = nil
			continue
		}
		if token.text == "(" {
			depth++
		}
		if token.text == ")" {
			depth--
		}
		actual = append(actual, token)
	}
	// Input.argsFor reads `name()` as no argument for a macro with no formals.
	if len(macro.formals) == 0 && len(actuals) == 1 && len(actuals[0]) == 0 {
		actuals = nil
	}
	if len(actuals) != len(macro.formals) {
		return nil, nil, fmt.Errorf("assembly macro %s is called with %d arguments, but its definition names %d", name, len(actuals), len(macro.formals))
	}
	return actuals, pending, nil
}

// leafReplacement substitutes each formal in a macro's body with its
// argument, as Input.invokeMacro does, and gives every other token chain. An
// argument keeps the chain of the place it was written, so a macro passed as
// an argument expands as it would there: round(round(a, b), c) is not
// recursion, while a body that names its own macro is.
func leafReplacement(macro *assemblyLocalMacro, actuals [][]leafToken, chain []string) []leafToken {
	replacement := make([]leafToken, 0, len(macro.body))
	for _, text := range macro.body {
		if formal := slices.Index(macro.formals, normalizedAssemblyToken(text)); formal >= 0 {
			replacement = append(replacement, actuals[formal]...)
			continue
		}
		replacement = append(replacement, leafToken{text: text, chain: chain})
	}
	return replacement
}
