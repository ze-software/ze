// Design: docs/architecture/testing/ci-format.md -- editor expectations that judge order

package testing

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli"
)

// TestExpectCompletionOrder proves completion:order judges the relative order
// of the named items and not their membership alone. Method: one menu in
// alphabetical order passes, the same items in another order fail, a missing
// item fails, and a single item is refused because it states no order.
func TestExpectCompletionOrder(t *testing.T) {
	expectation := Expectation{Type: "completion", Values: map[string]string{"order": "peer,router-id,session"}}

	sorted := []cli.Completion{{Text: "as-notation"}, {Text: "peer"}, {Text: "rib"}, {Text: "router-id"}, {Text: "session"}}
	if err := checkExpectation(expectation, &MockState{completions: sorted}); err != nil {
		t.Fatalf("alphabetical menu refused: %v", err)
	}

	definition := []cli.Completion{{Text: "router-id"}, {Text: "session"}, {Text: "peer"}}
	err := checkExpectation(expectation, &MockState{completions: definition})
	if err == nil {
		t.Fatal("menu in definition order passed an alphabetical order expectation")
	}
	if !strings.Contains(err.Error(), "out of order") {
		t.Fatalf("refusal does not say the order is wrong: %v", err)
	}

	missing := []cli.Completion{{Text: "peer"}, {Text: "router-id"}}
	if err := checkExpectation(expectation, &MockState{completions: missing}); err == nil {
		t.Fatal("menu without session passed")
	}

	single := Expectation{Type: "completion", Values: map[string]string{"order": "peer"}}
	if err := checkExpectation(single, &MockState{completions: sorted}); err == nil {
		t.Fatal("a one-item order expectation was accepted, though it states no order")
	}
}

// TestKeyExpectationPattern proves key:pattern= matches the decoded value of
// one key, so a .et can state the line order of a committed file, and that it
// is refused beside glob, whose checker would skip it. Method: a stored value
// with router-id before peer matches a pattern in that order and fails the
// reverse one; the glob form is refused before any key is read.
func TestKeyExpectationPattern(t *testing.T) {
	state := &keyValueState{value: "set bgp router-id 1.2.3.4\nset bgp session asn local 65000\nset bgp peer p1 connection remote ip 192.0.2.1\n"}

	inOrder := Expectation{Type: "key", Values: map[string]string{"path": "file/active/test.conf", "pattern": `(?s)router-id .*session .*peer p1`}}
	if err := checkExpectation(inOrder, state); err != nil {
		t.Fatalf("value in the stated order refused: %v", err)
	}

	reversed := Expectation{Type: "key", Values: map[string]string{"path": "file/active/test.conf", "pattern": `(?s)peer p1.*router-id `}}
	if err := checkExpectation(reversed, state); err == nil {
		t.Fatal("value matched a pattern in the reverse order")
	}

	globbed := Expectation{Type: "key", Values: map[string]string{"glob": "file/*/test.conf", "pattern": "router-id"}}
	if err := checkExpectation(globbed, state); err == nil {
		t.Fatal("pattern accepted beside glob, whose checker ignores it")
	}
}

type keyValueState struct {
	MockState
	value string
}

func (state *keyValueState) ReadKey(string) ([]byte, error) { return []byte(state.value), nil }
func (state *keyValueState) ListKeys(string) ([]string, error) {
	return []string{"file/active/test.conf"}, nil
}
