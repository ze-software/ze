// Design: docs/architecture/exabgp-bridge.md -- the commands a script writes
// Related: bridge_command.go -- Translator.Line, which reads a route line through resolveLineEscapes

package bridge

import (
	"errors"
	"strconv"
	"strings"
)

// errBadEscape refuses a route line whose `\u` escape is not four hexadecimal
// digits, as ExaBGP's unescape refuses it (configuration/grammar/lexer.py).
// It names the problem, why a backslash matters at all, and both fixes.
var errBadEscape = errors.New("the route command has a \\u escape without four hexadecimal digits " +
	"after it: ExaBGP reads a backslash in a route command as the start of an escape, " +
	"and \\u names a character by four hexadecimal digits; write \\u and four digits " +
	"(\\u0041 for A), or write \\\\ where you meant a backslash")

// resolveLineEscapes answers the line with its backslash escapes resolved, as
// every ExaBGP release reads a route command before splitting it into words,
// and only a route command: a neighbor control, refresh or operational line
// keeps its backslashes (Translator.Line). 4.2 and 5.0 resolve them in
// tokens (`line = unescape(letters)`, configuration/core/format.py), the pin
// in its lexer ("Resolve the backslash escapes of a line, as the legacy
// tokeniser does before splitting it", unescape, configuration/grammar/lexer.py
// at f9bfb853b). `\b`, `\f`, `\n`, `\r` and `\t` are those control characters,
// `\uXXXX` is the character the four hexadecimal digits name, and any other
// escaped character is itself. A backslash ending the line escapes the space
// ExaBGP adds before the terminator it gives an API command (`text + ' ;'`,
// read_command, configuration/grammar/read.py at f9bfb853b), so it ends the
// last word and is no word of its own: 5.0 sent `announce route 10.0.0.0/8
// next-hop self \` as the route without it (qa/old/commands-5.0.json).
// A `\u` without four hexadecimal digits refuses the line, as the pin does.
func resolveLineEscapes(line string) (string, error) {
	if !strings.Contains(line, `\`) {
		return line, nil
	}
	var resolved strings.Builder
	resolved.Grow(len(line))
	for position := 0; position < len(line); position++ {
		char := line[position]
		if char != '\\' {
			resolved.WriteByte(char)
			continue
		}
		if position+1 == len(line) {
			resolved.WriteByte(' ')
			break
		}
		position++
		escape := line[position]
		if escape != 'u' {
			resolved.WriteByte(lineEscape(escape))
			continue
		}
		if position+5 > len(line) {
			return "", errBadEscape
		}
		code, err := strconv.ParseUint(line[position+1:position+5], 16, 16)
		if err != nil {
			return "", errBadEscape
		}
		resolved.WriteRune(rune(code))
		position += 4
	}
	return resolved.String(), nil
}

// lineEscape is the character a one-character escape stands for: ExaBGP's
// _ESCAPES, and the escaped character itself for every other one.
func lineEscape(escape byte) byte {
	switch escape {
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	}
	return escape
}
