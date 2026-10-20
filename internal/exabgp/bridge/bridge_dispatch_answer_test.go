// Design: docs/architecture/exabgp-bridge.md -- what the bridge refuses
// Related: bridge_dispatch_answer.go -- RefuseUnsentRoute and answerDocuments
//
// ze's update-text handler answers `warning`, not an error, when no selected
// peer has the route's family negotiated, and the record wire drops that
// status. ExaBGP answers `error` (X-23). These tests feed both answer shapes,
// in-process and over the standalone runner's wire, and read the refusal.
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// zeNoFamilyMessage is the message ze's update-text handler answers when no
// selected peer has ipv6 unicast negotiated (route.ErrNoPeersAcceptedFamily).
const zeNoFamilyMessage = "announce ipv6/unicast: no peers have family negotiated"

// TestFamilyNoSelectedPeerCarriesAnswersError is X-23 (AC-29) at the bridge.
// The real engine's answer is proven by the api-refusals-v6 .ci case.
//
// VALIDATES: the in-process answer (status warning) and the wire answer
// (status done, the same document) both refuse with ze's reason; a sent route
// (RouteResult, even with zero counts) and a command that is not update text
// pass; the standalone runner's Dispatch reads the document off the answer's
// record and refuses, and a terminator message fails it.
// PREVENTS: `done` for a route that reached no peer.
func TestFamilyNoSelectedPeerCarriesAnswersError(t *testing.T) {
	const command = "send bgp * update text nhop 2001:db8::1 nlri ipv6/unicast add 2001:db8::/32"
	warning := json.RawMessage(`{"message":"` + zeNoFamilyMessage + `"}`)

	err := RefuseUnsentRoute(command, "warning", warning)
	require.ErrorIs(t, err, ErrRouteSentNothing)
	assert.Contains(t, err.Error(), zeNoFamilyMessage)

	err = RefuseUnsentRoute(command, rpc.StatusDone, warning)
	require.ErrorIs(t, err, ErrRouteSentNothing)

	require.NoError(t, RefuseUnsentRoute(command, rpc.StatusDone, json.RawMessage(`{"announced":1,"withdrawn":0}`)))
	require.NoError(t, RefuseUnsentRoute(command, rpc.StatusDone, json.RawMessage(`{"announced":0,"withdrawn":0}`)))
	require.NoError(t, RefuseUnsentRoute("bgp peer * clear soft", rpc.StatusDone, warning))

	t.Run("standalone runner", func(t *testing.T) {
		answer := func(id uint64, document json.RawMessage, message string) string {
			lines := []string{
				string(rpc.AppendAnswerHead(nil, id, rpc.AnswerTypeDocument, "", nil)),
				string(rpc.AppendAnswerItem(nil, id, document)),
				string(rpc.AppendAnswerTerminator(nil, id, 1, 0, message)),
			}
			for i := range lines {
				lines[i] = strings.TrimRight(lines[i], "\n")
			}
			return strings.Join(lines, "\n") + "\n"
		}
		b := NewBridge([]string{"echo"})
		b.running = true
		pending := newPendingResponses()
		unsent := pending.register(7)
		failed := pending.register(8)
		var zeBuf, pluginBuf strings.Builder
		wire := answer(7, warning, "") + answer(8, json.RawMessage(`{}`), "peer is shutting down")
		b.zebgpToPluginWithScanner(context.Background(), bufio.NewScanner(strings.NewReader(wire)),
			&pluginBuf, &syncWriter{w: &zeBuf}, pending)

		result := <-unsent
		require.True(t, result.ok)
		err := RefuseUnsentRoute(command, rpc.StatusDone, result.document)
		require.ErrorIs(t, err, ErrRouteSentNothing)
		assert.Contains(t, err.Error(), zeNoFamilyMessage)

		result = <-failed
		assert.False(t, result.ok)
		assert.Equal(t, "peer is shutting down", result.errText)
		assert.Empty(t, zeBuf.String())
	})
}
