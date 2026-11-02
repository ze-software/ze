package ssh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestARefusalReadsAsTheLocalRouteOverTheExecChannel drives two refusals a
// command wrote whole over a real SSH exec channel and reads the raw stderr,
// as `ssh <host> <command>` shows it.
//
// VALIDATES: the operator line is the local route's Diagnostic as it stands:
// a usage refusal reads "usage: ...", not "error: usage: ...", and an
// "error: " refusal carries its prefix once; any other error still reads
// "error: " and its text.
// PREVENTS: the daemon route printing a refusal in other bytes than `ze
// <command>` prints it.
func TestARefusalReadsAsTheLocalRouteOverTheExecChannel(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{err: plugin.NewRefusalError("usage: show config list\n"), want: "usage: show config list\n"},
		{err: plugin.NewRefusalError("error: no such key\n"), want: "error: no such key\n"},
		{err: assert.AnError, want: "error: " + assert.AnError.Error() + "\n"},
	}
	for _, tc := range cases {
		srv := answerServer(t, func(string) (*plugin.Response, error) {
			return &plugin.Response{Status: plugin.StatusError, Error: tc.err.Error()}, tc.err
		})
		_, stderr, _ := runRawSSHExec(t, srv, "show config list")
		assert.True(t, strings.HasPrefix(stderr, tc.want), "stderr %q, want it to open with %q", stderr, tc.want)
		assert.NotContains(t, stderr, "error: usage:", "a usage refusal gained a prefix the local route does not print")
		assert.NotContains(t, stderr, "error: error:", "the prefix was printed twice")
	}
}
