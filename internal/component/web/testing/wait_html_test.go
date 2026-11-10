// Design: docs/architecture/testing/runner-architecture.md -- in-page wait grammar.
// Related: wait_html_integration_test.go -- the same action against a real page.

package webtesting

import (
	"context"
	"errors"
	"testing"
)

// TestWaitHTMLRefusesMalformedCondition proves `action=wait-html` refuses
// every key set other than exactly one non-empty contains= or not-contains=,
// before it runs a browser command. Method: each malformed set is parsed from
// a .wb line and executed against a browser whose context is already
// canceled, so reaching any command would answer the cancellation instead of
// the named refusal.
// MUTATION: dropping the len(values) check lets `contains=a:not-contains=b`
// wait on one key and ignore the other, and dropping the trailing refusal lets
// a misspelled key return nil, which waits for nothing.
func TestWaitHTMLRefusesMalformedCondition(t *testing.T) {
	cases := []string{
		"action=wait-html",
		"action=wait-html:contains=a:not-contains=b",
		"action=wait-html:not-contain=commit-bar visible",
		"action=wait-html:text=commit-bar visible",
		"action=wait-html:not-contains=",
		"action=wait-html:contains=",
		"action=wait-html:path=/config/diff:not-contains=x",
	}
	for _, line := range cases {
		t.Run(line, func(t *testing.T) {
			tc, err := ParseWBFile(line + "\n")
			if err != nil {
				t.Fatalf("parse %q: %v", line, err)
			}
			if len(tc.Actions) != 1 {
				t.Fatalf("parse %q: %d actions, want 1", line, len(tc.Actions))
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			browser := newBrowser("http://127.0.0.1:1")
			browser.ctx = ctx
			err = executeAction(browser, &tc.Actions[0])
			if !errors.Is(err, errWaitHTMLRequiresOneCondition) {
				t.Fatalf("%q answered %v, want the wait-html refusal", line, err)
			}
		})
	}
}
