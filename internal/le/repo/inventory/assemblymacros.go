// Design: docs/architecture/plugin/feature-gates.md -- macro namespace evidence, and the definitions a leaf body expands.
// Related: assemblysyntax.go -- whole logical directives and literal definition census.
package repoinventory

import (
	"fmt"
	"slices"
	"strings"
)

func normalizedAssemblyToken(value string) string {
	value = strings.ReplaceAll(value, "·", ".")
	return strings.ReplaceAll(value, "∕", "/")
}

func assemblyDefinitionWord(value string) bool {
	switch value {
	case "TEXT", "GLOBL", "DATA":
		return true
	}
	return false
}

// assemblyMacro records one #define of the census in unit. cmd/asm refuses a
// redefinition (Input.defineMacro: "redefinition of macro"), so a name
// already bound is admitted again only after its #undef (undefine).
func assemblyMacro(statement assemblyStatement, unit *assemblyNamespace) error {
	name, macro, err := parseAssemblyMacro(statement)
	if err != nil {
		return err
	}
	if unit.local[name] && unit.current[name] == nil {
		unit.current[name] = macro
		return nil
	}
	if err := unit.bind(name, true); err != nil {
		return err
	}
	unit.local[name] = true
	unit.current[name] = macro
	return nil
}

// parseAssemblyMacro admits only definitions whose complete token stream
// cannot supply a definition opcode/directive. Actual-argument and prefix
// origins are checked separately by macroDefinitionOrigins. It returns the
// normalized name and the macro as leafExpansion expands it, with its body
// copied out of the lexer's reused token slice.
func parseAssemblyMacro(statement assemblyStatement) (string, *assemblyLocalMacro, error) {
	tokens := statement.tokens
	if len(tokens) < 3 {
		return "", nil, fmt.Errorf("incomplete macro definition")
	}
	name, err := assemblySymbol(tokens[2], "")
	if err != nil {
		return "", nil, fmt.Errorf("unsupported macro name: %w", err)
	}
	if assemblyDefinitionWord(name) {
		return "", nil, fmt.Errorf("macro redefines assembly definition word %s", name)
	}
	for _, word := range tokens[3:] {
		if word == "#" {
			return "", nil, fmt.Errorf("macro %s can emit a directive or token-pasting operator", name)
		}
		if word == "\\" {
			return "", nil, fmt.Errorf("macro %s has unsupported literal escape tokens", name)
		}
		if assemblyDefinitionWord(normalizedAssemblyToken(word)) {
			return "", nil, fmt.Errorf("macro %s contains a definition-word origin", name)
		}
	}
	if !statement.functionMacro {
		return name, &assemblyLocalMacro{body: slices.Clone(tokens[3:])}, nil
	}
	if len(tokens) < 5 {
		return "", nil, fmt.Errorf("macro %s has incomplete formals", name)
	}
	if tokens[3] != "(" {
		return "", nil, fmt.Errorf("macro %s has unproved function-like adjacency", name)
	}
	macro := &assemblyLocalMacro{function: true}
	accept := true
	closing := -1
	for offset := 4; offset < len(tokens); offset++ {
		value := tokens[offset]
		if value == ")" {
			closing = offset
			break
		}
		if value == "," {
			if accept {
				return "", nil, fmt.Errorf("macro %s has malformed formals", name)
			}
			accept = true
			continue
		}
		if !accept {
			return "", nil, fmt.Errorf("macro %s has malformed formals", name)
		}
		formal, err := assemblySymbol(value, "")
		if err != nil {
			return "", nil, fmt.Errorf("macro %s has unsupported formal: %w", name, err)
		}
		if slices.Contains(macro.formals, formal) {
			return "", nil, fmt.Errorf("macro %s has duplicate formals", name)
		}
		macro.formals = append(macro.formals, formal)
		accept = false
	}
	if closing < 0 {
		return "", nil, fmt.Errorf("macro %s has unclosed formals", name)
	}
	macro.body = slices.Clone(tokens[closing+1:])
	return name, macro, nil
}

// macroDefinitionOrigins rejects raw definition words outside the literal
// opcode slot, including words supplied to macro calls as actual arguments or
// exposed by empty/label-producing prefixes. Whole tokens are never joined.
func macroDefinitionOrigins(tokens []string) error {
	for offset, value := range tokens {
		if value == "#" {
			return fmt.Errorf("unclassified embedded assembly directive")
		}
		if offset == 0 {
			continue
		}
		if assemblyDefinitionWord(normalizedAssemblyToken(value)) {
			return fmt.Errorf("raw definition word is hidden outside the literal opcode position")
		}
	}
	return nil
}

// macroFreeTokens is applied only after the complete unit's local namespace is
// known, including later declarations. Namespace safety cannot certify a header
// alias or a locally expanded trampoline instruction; a leaf body applies it
// after expansion (leafExpansion), where only a go_asm.h name can still match.
func macroFreeTokens(tokens []string, names map[string]bool) error {
	for _, value := range tokens {
		if names[normalizedAssemblyToken(value)] {
			return fmt.Errorf("local macro %s prevents literal header/body ownership", value)
		}
	}
	return nil
}
