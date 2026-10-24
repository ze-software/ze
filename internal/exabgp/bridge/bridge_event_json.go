// Design: docs/architecture/exabgp-bridge.md -- the JSON a script reads
// Related: bridge_event.go -- exabgpJSON, the document this serializes

package bridge

import (
	"encoding/json"
	"fmt"
)

// marshalExabgpJSON serializes one exabgpJSON document in ExaBGP's own spacing:
// `{ "key": value, "key": value }` and `[ a, b ]`.
//
// The spacing is part of the contract a script reads. ExaBGP builds its JSON
// by hand with a space after each colon and comma and inside each bracket
// (src/exabgp/reactor/api/response/json.py, `_header`, `_neighbor`; nlri/inet.py
// writes `{ "nlri": "<prefix>" }`), and scripts match on substrings of it: the
// upstream api-rr script tests `'"nlri": "192.168.0.0/32"' in line`
// (etc/exabgp/run/api-rr.run at f9bfb853b). Go's compact form has no spaces, so
// that test failed on a line carrying the route.
//
// An empty object or array stays `{}` or `[]`.
func marshalExabgpJSON(document map[string]any) ([]byte, error) {
	compact, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("marshal ExaBGP JSON: %w", err)
	}
	return spaceExabgpJSON(compact), nil
}

// spaceExabgpJSON re-spaces compact JSON, leaving string contents untouched.
// The input is json.Marshal output, so it holds no whitespace outside strings
// and every escape inside a string starts with a backslash.
func spaceExabgpJSON(compact []byte) []byte {
	// A quarter more covers the spaces of a typical event; append grows it for
	// a document made of nothing but separators.
	out := make([]byte, 0, len(compact)+len(compact)/4)
	inString := false
	escaped := false
	for i, c := range compact {
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
			out = append(out, c)
		case '{', '[':
			out = append(out, c)
			if i+1 < len(compact) && compact[i+1] != '}' && compact[i+1] != ']' {
				out = append(out, ' ')
			}
		case '}', ']':
			if i > 0 && compact[i-1] != '{' && compact[i-1] != '[' {
				out = append(out, ' ')
			}
			out = append(out, c)
		case ':', ',':
			out = append(out, c, ' ')
		default:
			out = append(out, c)
		}
	}
	return out
}
