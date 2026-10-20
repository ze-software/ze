// Design: docs/architecture/exabgp-bridge.md -- what the bridge refuses
// Related: bridge_dispatch_answer.go -- terminatorResult
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// zeNoFamilyMessage is the message ze's update-text handler answers, status
// warning, when the selected peers are configured for ipv6 unicast and none
// has it negotiated (route.ErrNoPeersAcceptedFamily).
const zeNoFamilyMessage = "announce ipv6/unicast: no peers have family negotiated"

// TestStandaloneRunnerReleasesADispatchAtItsTerminator is X-23 (AC-29) at the
// standalone runner. The family refusal itself is ze's
// (route.ErrNoPeerConfiguredFamily, proven by the api-refusals-v4 case).
//
// VALIDATES: a record answer whose terminator states no message releases the
// waiter as success, the warning document included, as ExaBGP answers a route
// for a configured family it cannot send yet; a terminator message fails it.
// ze spells the warning's terminator `wrn` (rpc.AnswerKindWarning), and the
// bridge still answers it done.
// PREVENTS: a `warning` refused as if no peer were configured for the family,
// and a failure stated on the terminator answered done.
func TestStandaloneRunnerReleasesADispatchAtItsTerminator(t *testing.T) {
	warning := json.RawMessage(`{"message":"` + zeNoFamilyMessage + `"}`)
	answer := func(id uint64, document json.RawMessage, terminator string) string {
		lines := []string{
			string(rpc.AppendAnswerHead(nil, id, rpc.AnswerTypeDocument, "", nil)),
			string(rpc.AppendAnswerItem(nil, id, document)),
			terminator,
		}
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], "\n")
		}
		return strings.Join(lines, "\n") + "\n"
	}
	b := NewBridge([]string{"echo"})
	b.running = true
	pending := newPendingResponses()
	warned := pending.register(7)
	failed := pending.register(8)
	var zeBuf, pluginBuf strings.Builder
	wire := answer(7, warning, "#7 wrn 1 0 0:") +
		answer(8, json.RawMessage(`{}`), string(rpc.AppendAnswerTerminator(nil, 8, 1, 0, "peer is shutting down")))
	b.zebgpToPluginWithScanner(context.Background(), bufio.NewScanner(strings.NewReader(wire)),
		&pluginBuf, &syncWriter{w: &zeBuf}, pending)

	var result pendingResult
	select {
	case result = <-warned:
	case <-time.After(time.Second):
		t.Fatal("a warning terminator must release the dispatch waiter")
	}
	assert.True(t, result.ok, "a warning is answered done, as ExaBGP answers it")
	result = <-failed
	require.False(t, result.ok)
	assert.Equal(t, "peer is shutting down", result.errText)
	assert.Empty(t, zeBuf.String())
}
