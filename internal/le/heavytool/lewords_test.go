package heavytool

import (
	"slices"
	"strings"
	"testing"
)

// TestLeWordsReadsEachSpellingOfLe proves LeWords finds the words of an le
// line under every spelling the Bash hook and `le exec` meet, and nothing
// else. Its method: one row per spelling, asserting the words and the verdict.
func TestLeWordsReadsEachSpellingOfLe(t *testing.T) {
	for _, row := range []struct {
		program string
		words   string
		typed   bool
	}{
		{"le test unit all", "test unit all", true},
		{"./le test unit all", "test unit all", true},
		{"/x/bin/le go lint run", "go lint run", true},
		{"./le", "", true},
		{"ze le test unit all", "test unit all", true},
		{"/x/ze-test le exec go test", "exec go test", true},
		{"ze le", "", false},
		{"ze version", "", false},
		{"go test ./x/", "", false},
		{"lex test", "", false},
		{"", "", false},
	} {
		words, typed := LeWords(strings.Fields(row.program))
		if typed != row.typed || !slices.Equal(words, strings.Fields(row.words)) {
			t.Errorf("LeWords(%q) = %q, %v; want %q, %v", row.program, words, typed, row.words, row.typed)
		}
	}
}
