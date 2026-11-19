// Design: docs/architecture/testing/runner-architecture.md -- the attribute expectation.
package webtesting

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestAttributeExpectationReadsTheNamedAttribute proves the attribute
// expectation asks the browser for exactly the attribute and the element it
// names, and judges the answer: the whole value for value=, one substring for
// contains=. The fake browser answers the root's theme and style, and refuses
// any other query, so an expectation that read something else fails.
// MUTATION: comparing value= as a substring accepts "lig"; ignoring the
// answer accepts the dark and missing cases.
func TestAttributeExpectationReadsTheNamedAttribute(t *testing.T) {
	for _, test := range []struct {
		name   string
		values map[string]string
		query  string
		refuse bool
	}{
		{name: "exact", values: map[string]string{"selector": "html", "name": "data-theme", "value": "light"}, query: "get attr html data-theme"},
		{name: "exact-other", values: map[string]string{"selector": "html", "name": "data-theme", "value": "dark"}, query: "get attr html data-theme", refuse: true},
		{name: "exact-is-whole", values: map[string]string{"selector": "html", "name": "data-theme", "value": "lig"}, query: "get attr html data-theme", refuse: true},
		{name: "contains", values: map[string]string{"selector": "html", "name": "style", "contains": "--fleet-sidebar-width: 28rem"}, query: "get attr html style"},
		{name: "contains-absent", values: map[string]string{"selector": "html", "name": "style", "contains": "--fleet-sidebar-width: 12rem"}, query: "get attr html style", refuse: true},
		{name: "unreadable", values: map[string]string{"selector": "#absent", "name": "data-theme", "value": "light"}, query: "get attr #absent data-theme", refuse: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			log := installAttributeAgentBrowser(t)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			browser := newBrowser("")
			browser.ctx = ctx
			err := checkAttribute(browser, &WBExpectation{Kind: "attribute", Values: test.values})
			if test.refuse && err == nil {
				t.Errorf("attribute expectation accepted %v", test.values)
			}
			if !test.refuse && err != nil {
				t.Errorf("attribute expectation refused %v: %v", test.values, err)
			}
			if commands := readAgentLog(t, log); !slices.Contains(commands, test.query) {
				t.Errorf("expectation did not ask %q: %v", test.query, commands)
			}
		})
	}
}

// TestAttributeExpectationRefusesAMalformedShape proves a shape that would
// judge nothing is refused before the browser is asked: a missing selector or
// name, neither or both conditions, an empty condition, an unknown key.
func TestAttributeExpectationRefusesAMalformedShape(t *testing.T) {
	for _, values := range []map[string]string{
		{"name": "data-theme", "value": "light"},
		{"selector": "html", "value": "light"},
		{"selector": "html", "name": "data-theme"},
		{"selector": "html", "name": "data-theme", "value": "light", "contains": "li"},
		{"selector": "html", "name": "data-theme", "value": ""},
		{"selector": "html", "name": "data-theme", "vlaue": "light"},
	} {
		log := installAttributeAgentBrowser(t)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		browser := newBrowser("")
		browser.ctx = ctx
		if err := checkAttribute(browser, &WBExpectation{Kind: "attribute", Values: values}); err == nil {
			t.Errorf("attribute expectation accepted the malformed shape %v", values)
		}
		cancel()
		if commands := readAgentLog(t, log); len(commands) != 0 {
			t.Errorf("malformed shape %v asked the browser: %v", values, commands)
		}
	}
}

// TestAttributeIsAnExpectationKind proves a .wb step reaches checkAttribute
// through the dispatch, rather than failing as an unknown kind.
func TestAttributeIsAnExpectationKind(t *testing.T) {
	installAttributeAgentBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	browser := newBrowser("")
	browser.ctx = ctx
	tc, err := ParseWBFile("expect=attribute:selector=html:name=data-theme:value=light\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkExpectation(browser, &tc.Expects[0]); err != nil {
		t.Errorf("the parsed attribute step did not pass: %v", err)
	}
}

func installAttributeAgentBrowser(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "agent-browser.log")
	if err := os.WriteFile(log, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$AGENT_BROWSER_TEST_SCOPE_LOG"
case "$*" in
  'get attr html data-theme') printf '%s\n' 'light' ;;
  'get attr html style') printf '%s\n' '--fleet-sidebar-width: 28rem;' ;;
  *) printf '%s' 'no such element' >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, agentBrowserBin), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_BROWSER_TEST_SCOPE_LOG", log)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}
