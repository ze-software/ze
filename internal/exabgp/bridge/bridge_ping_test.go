package bridge

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uuid4RE is the shape Python's str(uuid.uuid4()) prints: lowercase hex in
// 8-4-4-4-12 groups, version nibble 4, variant bits 10.
var uuid4RE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestBridgeAnswersPingAsExaBGPDoes holds the ExaBGP 6.0 health check.
//
// ExaBGP's `ping` (v4 spelling, `session ping` in v6) answers the daemon's UUID
// and whether the client is active, as JSON unless the line carries `text`,
// then done (src/exabgp/reactor/api/command/reactor.py, ping, at the pinned
// commit f9bfb853b). The bridge refused the line, so api-health stopped at its
// first command.
//
// VALIDATES: every accepted spelling translates to the bridge's own answer,
// and the answer is ExaBGP's bytes: json.dumps's separators for the JSON form,
// `pong <uuid> active=true` for the text form, the UUID one per process.
// PREVENTS: a ping refused, or answered in a shape a 6.0 script cannot parse.
func TestBridgeAnswersPingAsExaBGPDoes(t *testing.T) {
	for _, line := range []string{"ping", "ping 0f1e2d3c 1700000000.25", "session ping", "PING"} {
		translation, err := TranslateLine(line)
		require.NoError(t, err, line)
		assert.Equal(t, LocalPing, translation.Local, line)
		assert.Empty(t, translation.Commands, "%s: the bridge answers ping itself", line)
	}
	for _, line := range []string{"ping text", "session ping text", "ping TEXT"} {
		translation, err := TranslateLine(line)
		require.NoError(t, err, line)
		assert.Equal(t, LocalPingText, translation.Local, line)
	}

	require.Regexp(t, uuid4RE, daemonUUID, "the UUID is a version 4 UUID as Python prints it")

	var out strings.Builder
	mode := AckMode{enabled: true}
	mode.AnswerLocal(&out, LocalPing)
	assert.Equal(t, `{"pong": "`+daemonUUID+`", "active": true}`+"\ndone\n", out.String())

	out.Reset()
	mode.AnswerLocal(&out, LocalPingText)
	assert.Equal(t, "pong "+daemonUUID+" active=true\ndone\n", out.String())

	// ExaBGP writes the pong with processes.write, which the ack setting does
	// not silence, and only the done with answer_done_sync, which it does.
	out.Reset()
	silent := AckMode{enabled: false}
	silent.AnswerLocal(&out, LocalPingText)
	assert.Equal(t, "pong "+daemonUUID+" active=true\n", out.String())
}
