// Design: docs/architecture/testing/runner-architecture.md -- result-scoped browser expectations.
package webtesting

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestHTMLExpectationScopesContentsToItsID keeps an echoed source elsewhere in
// the document from satisfying an assertion about the application's answer.
// The executable answers the native get-html query differently for the body,
// the present result and a missing result; both positive and negative checks
// must read the requested target, and an unreadable target proves no absence.
// MUTATION: ignoring id in checkHTML accepts the outside-only and missing cases.
func TestHTMLExpectationScopesContentsToItsID(t *testing.T) {
	for _, test := range []struct {
		name   string
		values map[string]string
		query  string
		refuse bool
	}{
		{name: "whole-page", values: map[string]string{"contains": "outside-only"}, query: "body"},
		{name: "inside-result", values: map[string]string{"id": "owned-result", "contains": "inside-only"}, query: "#owned-result"},
		{name: "outside-result", values: map[string]string{"id": "owned-result", "contains": "outside-only"}, query: "#owned-result", refuse: true},
		{name: "negative-result", values: map[string]string{"id": "owned-result", "not-contains": "outside-only"}, query: "#owned-result"},
		{name: "missing-positive", values: map[string]string{"id": "absent-result", "contains": "outside-only"}, query: "#absent-result", refuse: true},
		{name: "missing-negative", values: map[string]string{"id": "absent-result", "not-contains": "never-present"}, query: "#absent-result", refuse: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			log := installHTMLScopeAgentBrowser(t)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			browser := newBrowser("")
			browser.ctx = ctx
			err := checkHTML(browser, &WBExpectation{Kind: "html", Values: test.values})
			if test.refuse {
				if err == nil {
					t.Error("HTML expectation accepted text outside its requested result or an unreadable result")
				}
			} else if err != nil {
				t.Errorf("HTML expectation refused its readable target: %v", err)
			}
			commands := readAgentLog(t, log)
			if !slices.Contains(commands, "get html "+test.query) {
				t.Errorf("expectation did not read its requested target %s: %v", test.query, commands)
			}
			if test.query != "body" {
				if slices.Contains(commands, "get html body") {
					t.Errorf("result-scoped expectation read the whole page: %v", commands)
				}
			}
		})
	}
}

func installHTMLScopeAgentBrowser(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "agent-browser.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$AGENT_BROWSER_TEST_SCOPE_LOG"
case "$*" in
  'get html body') printf '%s' '<textarea>outside-only</textarea><pre id="owned-result">inside-only</pre>' ;;
  'get html #owned-result') printf '%s' 'inside-only' ;;
  'get html #absent-result') printf '%s' 'missing #absent-result' >&2; exit 1 ;;
  *) printf '%s' 'unexpected browser query' >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, agentBrowserBin), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_BROWSER_TEST_SCOPE_LOG", log)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}
