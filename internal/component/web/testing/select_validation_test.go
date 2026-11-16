// Design: docs/architecture/testing/runner-architecture.md -- native select actions.

package webtesting

import (
	"strings"
	"testing"
)

// TestSelectRequiresTargetAndExplicitValue refuses incomplete actions before
// they can reach the browser or silently select a different control.
func TestSelectRequiresTargetAndExplicitValue(t *testing.T) {
	for _, values := range []map[string]string{
		{}, {"value": "standard"}, {"id": "", "value": "standard"}, {"id": "collection-new-send"},
	} {
		err := executeAction(nil, &WBAction{Kind: "select", Values: values})
		if err == nil {
			t.Fatalf("incomplete select %v accepted", values)
		}
		if !strings.Contains(err.Error(), "select requires") {
			t.Fatalf("incomplete select %v: %v", values, err)
		}
	}
}
