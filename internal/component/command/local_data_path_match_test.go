// Design: docs/architecture/api/commands.md -- decoded command tokens preserve boundaries.

package command

import (
	"slices"
	"strings"
	"testing"
)

// TestLookupLocalDataRequiresExactPathSegments proves the longest matching
// registration uses whole input words, without splitting an argument's spaces.
func TestLookupLocalDataRequiresExactPathSegments(t *testing.T) {
	const parent = "show test local"
	const child = parent + " segments"
	t.Cleanup(AddLocalDataForTest(parent, func(ValidatedArgs) LocalAnswer {
		return LocalAnswer{Payload: "parent"}
	}))
	t.Cleanup(AddLocalDataForTest(child, func(ValidatedArgs) LocalAnswer {
		return LocalAnswer{Payload: "child"}
	}))
	for _, test := range []struct {
		name  string
		words []string
	}{
		{"merged middle keywords", []string{"show", "test local", "segments"}},
		{"merged first keywords", []string{"show test", "local", "segments"}},
		{"merged final keywords", []string{"show", "test", "local segments"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, rest := LookupLocalData(test.words)
			if handler != nil {
				t.Fatalf("merged path token matched a registration; rest=%q", rest)
			}
		})
	}
	for _, test := range []struct {
		words []string
		kind  string
	}{
		{[]string{"show", "test", "local", "segments", "content", "two words"}, "child"},
		{[]string{"show", "test", "local", "content", "two words"}, "parent"},
	} {
		handler, rest := LookupLocalData(test.words)
		if handler == nil {
			t.Fatalf("valid path %q lost its handler", test.words)
		}
		if answer := handler(ValidatedArgs{}); answer.Payload != test.kind {
			t.Errorf("selected %v, want longest matching %s", answer.Payload, test.kind)
		}
		if !slices.Equal(rest, []string{"content", "two words"}) {
			t.Errorf("argument boundaries changed: %q", rest)
		}
	}
}

// TestServeLocalDoesNotMergeQuotedPathKeywords keeps local execution aligned
// with daemon token matching while preserving quoted single-word keywords and
// quoted argument values as the positive control.
func TestServeLocalDoesNotMergeQuotedPathKeywords(t *testing.T) {
	const path = "show test local segments"
	calls := 0
	t.Cleanup(AddLocalDataForTest(path, func(ValidatedArgs) LocalAnswer {
		calls++
		return LocalAnswer{Payload: map[string]bool{"called": true}}
	}))
	withArgDefSource(t, func(string) ([]ArgDef, error) {
		return []ArgDef{mustArgDef(NewStringArg("content", nil, nil, ArgOptions{Mandatory: true, KeywordOnly: true}))}, nil
	})
	answer, code, served := ServeLocal(`show "test" local segments content "two words" | json compact`, "")
	if !served {
		t.Fatal("quoted single-word keyword control was not served")
	}
	if code != 0 {
		t.Fatalf("valid quoted control exited %d: %s", code, answer)
	}
	if calls != 1 {
		t.Fatalf("valid control called producer %d times, want 1", calls)
	}
	for _, invocation := range []string{
		`show "test local" segments`,
		`"show test" local segments`,
		`show test "local segments"`,
	} {
		answer, code, served := ServeLocal(invocation+` content "two words" | json compact`, "")
		if served {
			t.Errorf("merged registered keywords were served: %s", invocation)
		}
		if code != 0 {
			t.Errorf("unserved input returned local exit %d", code)
		}
		if answer != "" {
			t.Errorf("unserved input returned a local answer: %q", answer)
		}
		if calls != 1 {
			t.Errorf("merged keywords called producer; calls=%d", calls)
		}
	}
}

// TestServeLocalQuotedPathRetainsDeclaredShape proves quoting a valid keyword
// cannot hide the command's declaration and let source work run before refusal.
func TestServeLocalQuotedPathRetainsDeclaredShape(t *testing.T) {
	const path = "show test local shaped"
	ResetShapesForTest()
	t.Cleanup(ResetShapesForTest)
	RegisterShape([]string{path}, ShapeDoc)
	calls := 0
	t.Cleanup(AddLocalDataForTest(path, func(ValidatedArgs) LocalAnswer {
		calls++
		return LocalAnswer{Payload: []map[string]string{{"value": "row-looking payload"}}}
	}))
	withArgDefSource(t, func(string) ([]ArgDef, error) { return nil, nil })
	for _, invocation := range []string{path, `show "test" local shaped`} {
		answer, code, served := ServeLocal(invocation+" | first 1 | json compact", "")
		if !served {
			t.Fatalf("registered command not served: %s", invocation)
		}
		if code != 1 {
			t.Errorf("declared document accepted row operator, code=%d", code)
		}
		if calls != 0 {
			t.Errorf("declared shape checked after producer, calls=%d", calls)
		}
		if !IsPipeError(answer) {
			t.Errorf("row operator did not get a pipe refusal: %q", answer)
		}
		if !strings.Contains(answer, "first cannot apply here") {
			t.Errorf("refusal lost the declared shape: %q", answer)
		}
	}
}
