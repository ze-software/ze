// Design: docs/architecture/api/commands.md -- plain local command path identity.

package command

import (
	"slices"
	"testing"
)

// TestLookupLocalRequiresExactPathSegments proves the standalone handler lookup
// preserves keyword and argument boundaries, with the same longest-path choice.
func TestLookupLocalRequiresExactPathSegments(t *testing.T) {
	ResetLocalForTest()
	t.Cleanup(ResetLocalForTest)
	const parent = "show test plain"
	const child = parent + " segments"
	MustRegisterLocal(parent, func(ValidatedArgs) int { return 7 })
	MustRegisterLocal(child, func(ValidatedArgs) int { return 9 })
	withArgDefSource(t, func(string) ([]ArgDef, error) {
		return []ArgDef{mustArgDef(NewStringArg("content", nil, nil, ArgOptions{Mandatory: true, KeywordOnly: true}))}, nil
	})
	for _, words := range [][]string{
		{"show", "test plain", "segments"},
		{"show test", "plain", "segments"},
		{"show", "test", "plain segments"},
	} {
		handler, rest, err := LookupLocal(words, notDeclared)
		if err != nil {
			t.Fatal(err)
		}
		if handler != nil {
			t.Errorf("merged path token %q matched a plain handler; rest=%q", words, rest)
		}
	}
	for _, test := range []struct {
		words []string
		path  string
		code  int
	}{
		{[]string{"show", "test", "plain", "segments", "content", "two words"}, child, 9},
		{[]string{"show", "test", "plain", "content", "two words"}, parent, 7},
	} {
		handler, rest, err := LookupLocal(test.words, notDeclared)
		if err != nil {
			t.Fatal(err)
		}
		if handler == nil {
			t.Fatalf("valid path %q lost its handler", test.words)
		}
		if !slices.Equal(rest, []string{"content", "two words"}) {
			t.Errorf("argument boundaries changed: %q", rest)
		}
		if code := InvokeLocal(test.path, handler, rest); code != test.code {
			t.Errorf("selected handler exited %d, want %d", code, test.code)
		}
	}
}
