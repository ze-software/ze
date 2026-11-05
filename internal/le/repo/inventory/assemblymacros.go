// Design: docs/architecture/plugin/feature-gates.md -- macro namespace evidence without token expansion.
// Related: assemblysyntax.go -- whole logical directives and literal definition census.
package repoinventory

import (
	"fmt"
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

// assemblyMacro admits only definitions whose complete token stream cannot
// supply a definition opcode/directive. Actual-argument and prefix origins are
// checked separately by macroDefinitionOrigins; this function expands nothing.
func assemblyMacro(statement assemblyStatement, unit *assemblyNamespace) error {
	tokens := statement.tokens
	if len(tokens) < 3 {
		return fmt.Errorf("incomplete macro definition")
	}
	name, err := assemblySymbol(tokens[2], "")
	if err != nil {
		return fmt.Errorf("unsupported macro name: %w", err)
	}
	if assemblyDefinitionWord(name) {
		return fmt.Errorf("macro redefines assembly definition word %s", name)
	}
	for _, value := range tokens[3:] {
		if value == "#" {
			return fmt.Errorf("macro %s can emit a directive or token-pasting operator", name)
		}
		if value == "\\" {
			return fmt.Errorf("macro %s has unsupported literal escape tokens", name)
		}
		if assemblyDefinitionWord(normalizedAssemblyToken(value)) {
			return fmt.Errorf("macro %s contains a definition-word origin", name)
		}
	}
	if statement.functionMacro {
		if len(tokens) < 5 {
			return fmt.Errorf("macro %s has incomplete formals", name)
		}
		if tokens[3] != "(" {
			return fmt.Errorf("macro %s has unproved function-like adjacency", name)
		}
		formals := make(map[string]bool)
		accept := true
		closed := false
		for _, value := range tokens[4:] {
			if value == ")" {
				closed = true
				break
			}
			if value == "," {
				if accept {
					return fmt.Errorf("macro %s has malformed formals", name)
				}
				accept = true
				continue
			}
			if !accept {
				return fmt.Errorf("macro %s has malformed formals", name)
			}
			formal, err := assemblySymbol(value, "")
			if err != nil {
				return fmt.Errorf("macro %s has unsupported formal: %w", name, err)
			}
			if formals[formal] {
				return fmt.Errorf("macro %s has duplicate formals", name)
			}
			formals[formal] = true
			accept = false
		}
		if !closed {
			return fmt.Errorf("macro %s has unclosed formals", name)
		}
	}
	return unit.bind(name, true)
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
// alias or a locally expanded instruction in the entered body.
func macroFreeTokens(tokens []string, names map[string]bool) error {
	for _, value := range tokens {
		if names[normalizedAssemblyToken(value)] {
			return fmt.Errorf("local macro %s prevents literal header/body ownership", value)
		}
	}
	return nil
}
