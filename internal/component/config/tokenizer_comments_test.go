package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTokenizerCommentTrivia checks that every `#` comment reaches the token
// after it as trivia, with its line, whether it follows a token on that line,
// and whether a blank line precedes it.
//
// VALIDATES: own-line, same-line, after `{`, after `}`, after `[` and
// end-of-file comments become trivia; the text is the bytes after `#`; the
// token kinds and values (automatic semicolon insertion included) equal those
// of the same input with its comments removed.
// PREVENTS: comments discarded before the parser can attach them, the defect
// that made `ze config fmt -w` delete every comment.
func TestTokenizerCommentTrivia(t *testing.T) {
	commented := "# ze-schema: 26.10.07\n" +
		"#head\n" +
		"\n" +
		"a 1 # trail a\r\n" +
		"# lead b\n" +
		"b [ # bracket\n" +
		"x y ]\n" +
		"\n" +
		"# lead c\n" +
		"c { # open\n" +
		"  d 2\n" +
		"} # close\n" +
		"# eof"
	plain := "\n\n\na 1\r\n\nb [\nx y ]\n\n\nc {\n  d 2\n}\n"

	got := newTokenizer(commented).all()
	want := newTokenizer(plain).all()
	require.Len(t, got, len(want), "token count")
	for i := range want {
		require.Equal(t, want[i].kind, got[i].kind, "token %d kind", i)
		require.Equal(t, want[i].value, got[i].value, "token %d value", i)
	}

	trivia := map[string][]comment{}
	blank := map[string]bool{}
	for _, tok := range got {
		key := tok.kind.String() + ":" + tok.value
		if len(tok.comments) > 0 {
			trivia[key] = tok.comments
		}
		blank[key] = tok.blankBefore
	}

	require.Equal(t, []comment{
		{text: " ze-schema: 26.10.07", line: 1},
		{text: "head", line: 2},
	}, trivia["WORD:a"])
	require.True(t, blank["WORD:a"], "a blank line separates the header from a")

	require.Equal(t, []comment{
		{text: " trail a", line: 4, sameLine: true},
		{text: " lead b", line: 5},
	}, trivia["WORD:b"], "the CR of a CRLF line ending is not comment text")
	require.Equal(t, []comment{{text: " bracket", line: 6, sameLine: true}}, trivia["WORD:x"])
	require.Equal(t, []comment{{text: " lead c", line: 9, blankBefore: true}}, trivia["WORD:c"])
	require.False(t, blank["WORD:c"])
	require.Equal(t, []comment{{text: " open", line: 10, sameLine: true}}, trivia["WORD:d"])
	require.Equal(t, []comment{
		{text: " close", line: 12, sameLine: true},
		{text: " eof", line: 13},
	}, trivia["EOF:"])
	require.Len(t, trivia, 6, "no token other than a, b, x, c, d and EOF carries a comment")
}
