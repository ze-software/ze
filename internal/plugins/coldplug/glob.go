// Design: docs/architecture/appliance/kernel-profiles.md -- Loading driver modules on the device: alias matching
// Overview: coldplug.go -- the cold-plug pass that matches device aliases against these patterns

package coldplug

// aliasMatch reports whether a device's modalias matches one modules.alias
// pattern. The pattern is the shell glob kmod hands to fnmatch(3) with no
// flags, so '/' and a leading '.' are ordinary characters: an alias is not a
// path, and path.Match, which stops '*' at '/', would answer differently for
// an alias that holds one.
//
//   - '*' matches any run of bytes, the empty run included
//   - '?' matches one byte
//   - '[...]' matches one byte from the set: ranges (a-z), a leading '!' or
//     '^' negates it, a ']' first in the set is a member, and a '[' with no
//     closing ']' is an ordinary byte
//   - '\' makes the next byte ordinary
//
// The POSIX named classes ([[:digit:]]) are not read: the kernel's alias
// generator writes only the forms above.
//
// The match is iterative with one backtrack point, the last '*', so its cost
// is bounded by len(pattern) * len(modalias) whatever the pattern holds.
func aliasMatch(pattern, modalias string) bool {
	p, i := 0, 0
	starP, starI := -1, 0
	for i < len(modalias) {
		if p < len(pattern) && pattern[p] == '*' {
			starP, starI = p, i
			p++
			continue
		}
		if p < len(pattern) {
			if width, ok := matchOne(pattern[p:], modalias[i]); ok {
				p += width
				i++
				continue
			}
		}
		if starP < 0 {
			return false
		}
		// The byte did not match after the last '*': let that '*' swallow
		// one more byte and retry the rest of the pattern from there.
		starI++
		p, i = starP+1, starI
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}

// matchOne matches the byte c against the single-byte token that starts
// token, which is never '*' and never empty. It answers how many pattern
// bytes the token spans, and whether c matched it.
func matchOne(token string, c byte) (width int, ok bool) {
	switch token[0] {
	case '?':
		return 1, true
	case '[':
		if width, matched, closed := matchClass(token, c); closed {
			return width, matched
		}
		// An unterminated '[' is an ordinary byte, as fnmatch reads it.
		return 1, c == '['
	case '\\':
		if len(token) > 1 {
			return 2, token[1] == c
		}
		return 1, c == '\\'
	default:
		return 1, token[0] == c
	}
}

// matchClass matches c against the bracket set that starts token ('[' at
// index 0). It answers the set's width up to and including its ']', whether c
// is a member, and whether the set was closed at all: an unclosed set answers
// closed false and the caller reads the '[' as an ordinary byte.
func matchClass(token string, c byte) (width int, matched, closed bool) {
	j := 1
	negated := false
	if j < len(token) && (token[j] == '!' || token[j] == '^') {
		negated = true
		j++
	}
	member := false
	first := true
	for j < len(token) {
		if token[j] == ']' && !first {
			return j + 1, member != negated, true
		}
		first = false
		low, next := classByte(token, j)
		high := low
		// A '-' between two bytes is a range; a '-' before the closing ']'
		// is a member.
		if next+1 < len(token) && token[next] == '-' && token[next+1] != ']' {
			high, next = classByte(token, next+1)
		}
		if low <= c && c <= high {
			member = true
		}
		j = next
	}
	return 0, false, false
}

// classByte reads one byte of a bracket set at index j, honoring a '\'
// escape, and answers the byte and the index after it.
func classByte(token string, j int) (b byte, next int) {
	if token[j] == '\\' && j+1 < len(token) {
		return token[j+1], j + 2
	}
	return token[j], j + 1
}
