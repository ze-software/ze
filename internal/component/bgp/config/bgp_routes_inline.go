// Design: docs/architecture/config/syntax.md — inline key-value tokenizer for VPLS/MUP config
// Overview: bgp_routes.go — route extraction orchestrator
// Related: bgp_routes_vpls.go — VPLS route parsing uses tokenizer
// Related: bgp_routes_mup.go — MUP route parsing uses tokenizer

package bgpconfig

import "strings"

// parseInlineKeyValues parses an inline "key value key value ..." string into a map.
// Handles arrays like "[ a b c ]" and parenthesized content like "( ... )".
func parseInlineKeyValues(inline string) map[string]string {
	tokens := tokenizeInline(inline)
	return parseKeyValuesFromTokens(tokens, 0)
}

// parseKeyValuesFromTokens parses "key value key value ..." from a token slice.
// Handles arrays like "[ a b c ]" and parenthesized content like "( ... )".
// Start specifies the index to begin parsing from.
func parseKeyValuesFromTokens(tokens []string, start int) map[string]string {
	result := make(map[string]string)
	i := start
	for i < len(tokens) {
		key := tokens[i]
		i++
		if i >= len(tokens) {
			break
		}

		// Handle array values: [ a b c ]
		if tokens[i] == "[" {
			var arr []string
			i++ // skip [
			for i < len(tokens) && tokens[i] != "]" {
				arr = append(arr, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++ // skip ]
			}
			result[key] = "[" + strings.Join(arr, " ") + "]"
			continue
		}

		// Handle parenthesized values: ( ... )
		if tokens[i] == "(" {
			depth := 1
			var paren []string
			i++ // skip (
		parenLoop:
			for i < len(tokens) && depth > 0 {
				switch tokens[i] {
				case "(":
					depth++
				case ")":
					depth--
					if depth == 0 {
						break parenLoop
					}
				}
				paren = append(paren, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++ // skip )
			}
			result[key] = "(" + strings.Join(paren, " ") + ")"
			continue
		}

		// Simple key-value pair
		result[key] = tokens[i]
		i++
	}

	return result
}

// tokenizeInline splits an inline string into tokens, preserving brackets and parens.
func tokenizeInline(s string) []string {
	var tokens []string
	var current strings.Builder

	for i := range len(s) {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		if c == '[' || c == ']' || c == '(' || c == ')' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(c))
			continue
		}
		if c == '\\' {
			// Skip backslash continuations - artifacts from multiline parsing
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteByte(c)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
