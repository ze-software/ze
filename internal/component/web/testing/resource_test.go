// Design: docs/architecture/testing/runner-architecture.md -- shared web admission.
package webtesting

import (
	"strings"
	"testing"
)

// TestWBResourceGrammar exercises the shared declarations through the web parser,
// including invalid declarations hidden after an authored skip.
func TestWBResourceGrammar(t *testing.T) {
	for _, body := range []string{
		"option=needs-linux\noption=exclusive:group=web-resource-proof\naction=wait:ms=1\n",
		"option=needs-linux:caps=net-admin\naction=wait:ms=1\n",
	} {
		if _, err := ParseWBFile(body); err != nil {
			t.Fatalf("shared resource declaration refused: %v", err)
		}
	}
	for _, declaration := range []string{"needs-linux:caps=net-admn", "exclusive:group=", "exclusive:gruop=example"} {
		if _, err := ParseWBFile("option=skip:reason=example\noption=" + declaration + "\n"); err == nil {
			t.Errorf("skip hid invalid declaration %q", declaration)
		} else if !strings.Contains(err.Error(), "line 2") {
			t.Errorf("resource error lost authored location: %v", err)
		}
	}
}
