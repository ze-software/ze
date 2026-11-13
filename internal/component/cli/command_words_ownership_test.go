// Design: docs/architecture/cli/error-surface.md -- parsed commands own their pipe positions.

package cli

import (
	"slices"
	"testing"
)

// TestCommandWordsOwnership keeps decoded-word edits outside the immutable
// parsed command and verifies that its transformations retain literal pipes.
func TestCommandWordsOwnership(t *testing.T) {
	words, err := ParseCommandWords(`top show | match "|" | head 1`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"top", "show", "|", "match", "|", "|", "head", "1"}
	callerWords := words.Tokens()
	callerWords[0], callerWords[2], callerWords[4] = "changed", "changed", "changed"
	if !slices.Equal(words.Tokens(), want) {
		t.Fatalf("caller changed parsed words: %q", words.Tokens())
	}
	tail := words.Tail()
	callerTail := tail.Tokens()
	callerTail[1], callerTail[3] = "changed", "changed"
	if !slices.Equal(tail.Tokens(), want[1:]) {
		t.Fatalf("caller changed tail words: %q", tail.Tokens())
	}
	if tail.FirstPipe() != 1 {
		t.Fatalf("tail first pipe = %d, want 1", tail.FirstPipe())
	}
	command, filters := tail.SplitPipe()
	if !slices.Equal(command.Tokens(), []string{"show"}) {
		t.Fatalf("command = %q, want show", command.Tokens())
	}
	callerFilters := filters.Tokens()
	callerFilters[1], callerFilters[2] = "changed", "changed"
	match, head := filters.SplitPipe()
	if !slices.Equal(match.Tokens(), []string{"match", "|"}) {
		t.Fatalf("literal match words = %q", match.Tokens())
	}
	if match.FirstPipe() != -1 {
		t.Fatalf("literal pipe became operator at %d", match.FirstPipe())
	}
	if !slices.Equal(head.Tokens(), []string{"head", "1"}) {
		t.Fatalf("following filter = %q", head.Tokens())
	}
	if !slices.Equal(words.Tokens(), want) {
		t.Fatalf("transformations changed original words: %q", words.Tokens())
	}
	if words.FirstPipe() != 2 {
		t.Fatalf("original first pipe = %d, want 2", words.FirstPipe())
	}
	unchanged, empty := match.SplitPipe()
	if !slices.Equal(unchanged.Tokens(), match.Tokens()) {
		t.Fatalf("pipe-free split changed words: %q", unchanged.Tokens())
	}
	if len(empty.Tokens()) != 0 {
		t.Fatalf("pipe-free split returned suffix: %q", empty.Tokens())
	}
}

// TestCommandWordsZeroContract proves every operation accepts the empty value.
func TestCommandWordsZeroContract(t *testing.T) {
	var empty CommandWords
	left, right := empty.SplitPipe()
	for _, words := range []CommandWords{empty, empty.Tail(), left, right} {
		if len(words.Tokens()) != 0 {
			t.Errorf("empty words returned tokens: %q", words.Tokens())
		}
		if words.FirstPipe() != -1 {
			t.Errorf("empty words returned pipe: %d", words.FirstPipe())
		}
		if len(ParsePipeFilters(words)) != 0 {
			t.Errorf("empty words returned filters: %v", ParsePipeFilters(words))
		}
	}
}
