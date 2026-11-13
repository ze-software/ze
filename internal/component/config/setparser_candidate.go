// Design: docs/architecture/config/change-file-structural-ops.md -- candidate member boundaries.
// Related: list_key.go -- the shared token codec records quoting.

package config

// candidateMembers answers decoded members when bare bracket delimiters are
// complete. A malformed delimiter run cannot defer the ordinary parser's
// refusal. Quoted brackets remain member data. The metadata walk passes a
// suffix of lineTokens, whose quote provenance parseLineWithMeta recorded.
func (p *SetParser) candidateMembers(rest []string) ([]string, bool) {
	if len(rest) == 0 {
		return rest, true
	}
	offset := 1 + len(p.lineTokens) - len(rest)
	start, end := 0, len(rest)
	if rest[0] == "[" {
		if !p.quotedTokens[offset] {
			end--
			if rest[end] != "]" {
				return nil, false
			}
			if p.quotedTokens[offset+end] {
				return nil, false
			}
			start++
		}
	}
	for i := start; i < end; i++ {
		if p.quotedTokens[offset+i] {
			continue
		}
		if rest[i] == "[" {
			return nil, false
		}
		if rest[i] == "]" {
			return nil, false
		}
	}
	return rest[start:end], true
}
