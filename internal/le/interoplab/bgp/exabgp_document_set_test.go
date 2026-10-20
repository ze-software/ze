//go:build ze_bgp

package bgp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSortExaBGPCommunitySetsIgnoresOrderOnly proves the document check reads
// the community lists as sets and nothing more.
//
// VALIDATES: two documents whose extended-community lists hold the same members
// in another order compare equal after sorting; a changed member still differs,
// and every other list keeps its order.
// PREVENTS: api-flow-merge failing on the order ExaBGP 6.0 writes (value order)
// against the order ze sends (the command's), and the sort hiding a wrong value.
func TestSortExaBGPCommunitySetsIgnoresOrderOnly(t *testing.T) {
	first := `{"string":"interface-set:input:3405770241:1","transitive":false},{"string":"interface-set:output:254:254","transitive":true}`
	swapped := `{"string":"interface-set:output:254:254","transitive":true},{"string":"interface-set:input:3405770241:1","transitive":false}`
	changed := `{"string":"interface-set:output:254:253","transitive":true},{"string":"interface-set:input:3405770241:1","transitive":false}`

	sorted := func(members string) string {
		t.Helper()
		var parsed map[string]any
		text := []byte(`{"neighbor":{"message":{"update":{"attribute":{"extended-community":[` + members + `],"as-path":[2,1]}}}}}`)
		if err := json.Unmarshal(text, &parsed); err != nil {
			t.Fatalf("fixture does not parse: %v", err)
		}
		sortExaBGPCommunitySets(parsed)
		out, err := json.Marshal(parsed)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(out)
	}

	if a, b := sorted(first), sorted(swapped); a != b {
		t.Errorf("same set in another order differs:\n  %s\n  %s", a, b)
	}
	if a, b := sorted(first), sorted(changed); a == b {
		t.Errorf("a changed member compares equal: %s", a)
	}
	if got := sorted(first); !strings.Contains(got, `"as-path":[2,1]`) {
		t.Errorf("a list that is not a community set was reordered: %s", got)
	}
}
