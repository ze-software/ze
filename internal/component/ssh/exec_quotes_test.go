package ssh

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestExecChannelKeepsTheQuotesOfTheLine: the exec channel hands the executor
// the line the client sent, with its double quotes, so a quoted value reaches
// the dispatcher as one word and a `|` inside quotes is not a pipe.
//
// Method: an executor that records the input it receives, driven by a real
// ssh client over execMiddleware. Two lines: a quoted value of two words (the
// documented `system command help name "<name>"` form), and a quoted value
// holding a `|`.
//
// VALIDATES: execMiddleware reads the raw line (sess.RawCommand), and the
// pipe split after it leaves a quoted `|` in the command.
// PREVENTS: the line split by a shell lexer (sess.Command) and joined again,
// which dropped every quote: `system command help name "show version"` then
// asked for a command named `show`, and `name "a | b"` became the pipe `b`.
// With sess.Command restored, the first case records the line without its
// quotes and the second never reaches the executor.
func TestExecChannelKeepsTheQuotesOfTheLine(t *testing.T) {
	cases := []struct {
		name, line string
	}{
		{"a quoted value of two words", `system command help name "show version"`},
		{"a quoted value holding a pipe", `show demo name "a | b"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var received []string
			srv := answerServer(t, func(input string) (*plugin.Response, error) {
				mu.Lock()
				received = append(received, input)
				mu.Unlock()
				return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"ok": "yes"}}, nil
			})
			creds := answerCredentials(t, srv)

			_, err := sshclient.ExecCommand(creds, tc.line)
			require.NoError(t, err)

			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, []string{tc.line}, received, "the executor did not receive the line as the client sent it")
		})
	}
}
