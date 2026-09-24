package adj_rib_in

import (
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func retainedReceive(t *testing.T, r *AdjRIBInManager, msgID uint64, body []byte) {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true,
		map[family.Family]bool{family.IPv4Unicast: true}))
	require.NoError(t, err)
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType: rpc.EventKindUpdate, PeerAddress: "192.0.2.1",
		RawMessage: &bgptypes.RawMessage{
			Type: msgtype.TypeUPDATE, MessageID: msgID, WireUpdate: wu, AttrsWire: attrs,
		},
	})
}

func retainedDecision(t *testing.T, r *AdjRIBInManager, action string, pathID, msgID uint64) {
	t.Helper()
	_, _, err := r.handleCommand("request bgp adj-rib-in batch-validate", []string{
		action, "192.0.2.1", "ipv4/unicast", "203.0.113.0/24",
		strconv.FormatUint(pathID, 10), "1", strconv.FormatUint(msgID, 10),
	}, "")
	require.NoError(t, err)
}

func retainedRelay(t *testing.T, r *AdjRIBInManager, command string, args ...string) ([]rpc.StoredRoute, map[string]any) {
	t.Helper()
	var relayed []rpc.StoredRoute
	r.routeRelayer = func(_ string, routes []rpc.StoredRoute) error {
		relayed = append(relayed, routes...)
		return nil
	}
	_, result, err := r.handleCommand(command, args, "")
	require.NoError(t, err)
	return relayed, result.(map[string]any)
}

// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1 positive — retained
// Invalid paths keep all received bytes for re-evaluation, independently of an
// eligible ADD-PATH sibling, and recovery is visible after an existing cursor.
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1 negative — timeout
// cannot release a retained Invalid path, and withdrawal cannot replay it again.
func TestASPARetainedPathReplayRecovery(t *testing.T) {
	r := newTestManager(t)
	_, _, err := r.handleCommand("request bgp adj-rib-in enable-validation", nil, "")
	require.NoError(t, err)
	body := rfc4271Announce(1,
		0, 0, 0, 0, 24, 203, 0, 113,
		0, 0, 0, 1, 24, 203, 0, 113)
	retainedReceive(t, r, 40, body)
	retainedDecision(t, r, "i", 0, 40)
	retainedDecision(t, r, "a", 1, 40)

	// Run the real expiry path with every original receive deadline elapsed.
	r.mu.Lock()
	r.validationTimeout = time.Nanosecond
	r.sweepExpiredPending()
	r.unlockValidation()
	routes, firstReplay := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Len(t, routes, 1)
	require.Equal(t, uint32(1), routes[0].PathID, "the rejected sibling is not replayable")

	retained, _ := retainedRelay(t, r, "request bgp adj-rib-in replay-path",
		"192.0.2.99", "192.0.2.1", "ipv4/unicast", "203.0.113.0/24", "0")
	require.Equal(t, []rpc.StoredRoute{{
		SourcePeer: "192.0.2.1", Family: "ipv4/unicast", MsgID: 40,
		AttrHex: hex.EncodeToString(body[4:18]), NextHopHex: "0a000001",
		NLRIHex: "18cb0071", NLRIFraming: rpc.NLRIFramingPrefixOnly,
	}}, retained, "targeted reconciliation retains original bytes for the reactor gate")

	retainedDecision(t, r, "a", 0, 40)
	lastIndex, ok := firstReplay["last-index"].(uint64)
	require.True(t, ok, "replay last-index is %T, want uint64", firstReplay["last-index"])
	delta, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99",
		strconv.FormatUint(lastIndex, 10))
	require.Equal(t, retained, delta, "acceptance rediscovers only the recovered path without an UPDATE")

	retainedReceive(t, r, 41, []byte{0, 8, 0, 0, 0, 0, 24, 203, 0, 113, 0, 0})
	gone, _ := retainedRelay(t, r, "request bgp adj-rib-in replay-path",
		"192.0.2.99", "192.0.2.1", "ipv4/unicast", "203.0.113.0/24", "0")
	require.Equal(t, []rpc.StoredRoute{{
		SourcePeer: "192.0.2.1", Family: "ipv4/unicast", Withdraw: true,
		NLRIHex: "18cb0071", NLRIFraming: rpc.NLRIFramingPrefixOnly,
	}}, gone, "an absent path is an explicit withdrawal, not an empty announcement")
	routes, _ = retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Len(t, routes, 1)
	require.Equal(t, uint32(1), routes[0].PathID, "withdrawing path zero must not remove its sibling")
}

// VALIDATES: an early replacement decision supersedes the prior pending route.
// PREVENTS: the prior pending timeout overwriting a retained, newer generation.
func TestASPARetainedEarlyReplacementSurvivesTimeout(t *testing.T) {
	r := newTestManager(t)
	_, _, err := r.handleCommand("request bgp adj-rib-in enable-validation", nil, "")
	require.NoError(t, err)
	retainedReceive(t, r, 50, rfc4271Announce(1, 0, 0, 0, 0, 24, 203, 0, 113))
	retainedDecision(t, r, "i", 0, 51)
	r.mu.Lock()
	for _, decision := range r.earlyDecisions {
		decision.receivedAt = time.Now().Add(-2 * earlyDecisionTimeout)
	}
	for _, pending := range r.pending {
		pending.receivedAt = time.Now().Add(-2 * defaultValidationTimeout)
	}
	r.sweepExpiredEarlyDecisions()
	r.sweepExpiredPending()
	r.unlockValidation()
	retainedDecision(t, r, "a", 0, 50)
	stale, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Empty(t, stale, "a late accept cannot revive the predecessor of a known replacement")
	body := rfc4271Announce(2, 0, 0, 0, 0, 24, 203, 0, 113)
	retainedReceive(t, r, 51, body)

	r.mu.Lock()
	r.validationTimeout = time.Nanosecond
	r.sweepExpiredPending()
	r.unlockValidation()
	routes, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Empty(t, routes, "the older pending deadline cannot expose either generation")

	retainedDecision(t, r, "a", 0, 51)
	routes, _ = retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Equal(t, []rpc.StoredRoute{{
		SourcePeer: "192.0.2.1", Family: "ipv4/unicast", MsgID: 51,
		AttrHex: hex.EncodeToString(body[4:18]), NextHopHex: "0a000002",
		NLRIHex: "18cb0071", NLRIFraming: rpc.NLRIFramingPrefixOnly,
	}}, routes, "recovery must use the replacement's bytes, never the expired predecessor")
}
