package bgp

import (
	"fmt"
	"strings"
	"testing"
)

// TestEmptyASPathVerdictFailsClosed proves the empty-AS_PATH check accepts
// only FRR's empty form on every path and refuses every answer that does not
// show an empty path. Method: FRR 10.7.1's empty form accepted, then a path
// listing 65001, a non-empty path without 65001, a second non-empty path, and
// each malformed answer (missing aspath, segments, length or string, no
// paths, not JSON), each of which must be refused with its reason.
func TestEmptyASPathVerdictFailsClosed(t *testing.T) {
	const prefix = "10.77.5.0/24"
	accepted := fmt.Sprintf(frrEmptyASPathAnswer, prefix)
	if err := emptyASPathVerdict(accepted, prefix); err != nil {
		t.Fatalf("empty AS_PATH refused: %v", err)
	}
	const empty = `{"aspath":{"string":"Local","segments":[],"length":0}}`
	for name, case_ := range map[string]struct {
		output string
		reason string
	}{
		"path listing 65001":    {fmt.Sprintf(frrNoASRoute, prefix, "65001", "65001"), "path 0 AS_PATH is not empty: segment count 1"},
		"path without 65001":    {fmt.Sprintf(frrNoASRoute, prefix, "65000 65010", "65000,\n              65010"), "not empty"},
		"second path not empty": {`{"paths":[` + empty + `,{"aspath":{"string":"65001","segments":[{"type":"as-sequence","list":[65001]}],"length":1}}]}`, "path 1 AS_PATH is not empty"},
		"length not zero":       {`{"paths":[{"aspath":{"string":"Local","segments":[],"length":1}}]}`, "length 1"},
		"string not Local":      {`{"paths":[{"aspath":{"string":"","segments":[],"length":0}}]}`, `want "Local"`},
		"missing aspath":        {`{"paths":[{"valid":true}]}`, "no aspath object"},
		"missing segments":      {`{"paths":[{"aspath":{"string":"Local","length":0}}]}`, "no segments field"},
		"null segments":         {`{"paths":[{"aspath":{"string":"Local","segments":null,"length":0}}]}`, "no segments field"},
		"missing length":        {`{"paths":[{"aspath":{"string":"Local","segments":[]}}]}`, "no length field"},
		"missing string":        {`{"paths":[{"aspath":{"segments":[],"length":0}}]}`, "no string field"},
		"no paths":              {`{"prefix":"10.77.5.0/24","paths":[]}`, "no paths"},
		"route absent":          {`{}`, "no paths"},
		"not json":              {prefix + " Local", "not FRR route JSON"},
		"segments not an array": {`{"paths":[{"aspath":{"string":"Local","segments":{},"length":0}}]}`, "not FRR route JSON"},
	} {
		t.Run(name, func(t *testing.T) {
			err := emptyASPathVerdict(case_.output, prefix)
			if err == nil {
				t.Fatalf("answer accepted:\n%s", case_.output)
			}
			if !strings.Contains(err.Error(), case_.reason) {
				t.Fatalf("refusal %q does not say %q", err, case_.reason)
			}
		})
	}
}
