// Design: docs/architecture/config/syntax.md — Tree data structure
// Related: tree.go — the member store and its scalar mirror

package config

import "github.com/ze-software/ze/internal/core/textbuf"

// JoinLeafListMembers spells leaf-list members as one value: each member
// quoted when it needs it (quoteIfNeeded), separated by single spaces. It is
// the spelling of the scalar mirror and of a bracket leaf-list's metadata
// value, and SplitLeafListMembers reads it back to the same members, so a
// member holding a space stays one member.
func JoinLeafListMembers(items []string) string {
	var b textbuf.Buffer
	for i, item := range items {
		if i > 0 {
			b.Byte(' ')
		}
		b.Str(quoteIfNeeded(item))
	}
	return b.String()
}

// SplitLeafListMembers reads a value spelled by JoinLeafListMembers back into
// its members. Surrounding `[ ]` tokens are dropped, a double-quoted run is one
// member with the escapes quoteIfNeeded writes undone, and anything else splits
// on spaces and tabs. An unterminated quote ends at the end of the value.
func SplitLeafListMembers(value string) []string {
	var members []string
	var current textbuf.Buffer
	inMember := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch {
		case ch == '"':
			i = readQuotedMember(value, i+1, &current)
			inMember = true
		case ch == ' ' || ch == '\t':
			if inMember {
				members = append(members, current.String())
				current.Reset()
				inMember = false
			}
		default:
			current.Byte(ch)
			inMember = true
		}
	}
	if inMember {
		members = append(members, current.String())
	}
	if len(members) >= 2 && members[0] == "[" && members[len(members)-1] == "]" {
		members = members[1 : len(members)-1]
	}
	return members
}

// readQuotedMember appends the quoted run starting at value[start] (just past
// the opening quote) to out and returns the index of the closing quote, or the
// last index when the quote is not closed.
func readQuotedMember(value string, start int, out *textbuf.Buffer) int {
	for i := start; i < len(value); i++ {
		ch := value[i]
		if ch == '"' {
			return i
		}
		if ch != '\\' || i+1 == len(value) {
			out.Byte(ch)
			continue
		}
		i++
		switch value[i] {
		case 'n':
			out.Byte('\n')
		case 't':
			out.Byte('\t')
		default:
			out.Byte(value[i])
		}
	}
	return len(value) - 1
}
