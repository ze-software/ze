// Design: docs/architecture/plugin/rib-storage-design.md -- initial replay carries the source revision.
package update

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// initialReplayTestMeta is the metadata bgp-rib's replayRoutesWithCursor
// attaches to a forwarded initial-replay group.
func initialReplayTestMeta() map[string]any {
	return map[string]any{
		bgptypes.SentOwnerMessageMeta:     "91",
		bgptypes.InitialReplayMeta:        "5",
		bgptypes.InitialSourcePeerMeta:    "192.0.2.10",
		bgptypes.InitialSourceIDMeta:      "17",
		bgptypes.InitialSourceOwnerMeta:   "101",
		bgptypes.InitialSourceMessageMeta: "81",
	}
}

// TestInitialReplayCarriesSourceMessage proves the received message ID of a
// forwarded initial replay reaches the reactor's batch. Method: dispatch an
// `update cursor` announcement with the metadata bgp-rib attaches, and read
// the batch the mock reactor received. A forwarded replay with the key
// missing, zero or malformed is refused, as one missing
// initial-source-owner is, and a local replay that carries it is refused.
//
// VALIDATES: the initial-source-message chain (Data Flow 3a, round 11, I-2).
// PREVENTS: an initial-replay entry at the destination with message ID 0,
// which no later election of the same revision can match.
func TestInitialReplayCarriesSourceMessage(t *testing.T) {
	args := strings.Fields("origin igp as-path [65010] next-hop 192.0.2.10 nlri add 10.0.0.0/24")
	t.Run("forwarded-carries-message", func(t *testing.T) {
		ctx := cursorTestCtx(t)
		ctx.Meta = initialReplayTestMeta()
		_, err := handleUpdateCursor(ctx, args)
		require.NoError(t, err)
		reactor := cursorTestReactor(t, ctx)
		require.Len(t, reactor.announceCalls, 1)
		require.Equal(t, uint64(81), reactor.announceCalls[0].InitialSourceMessage)
		require.Equal(t, uint64(101), reactor.announceCalls[0].InitialSourceOwner)
	})
	for _, tc := range []struct {
		name  string
		value any // nil: the key is absent
		want  string
	}{
		{"absent", nil, "initial source message must be a decimal string"},
		{"zero", "0", "initial source message must be nonzero"},
		{"negative", "-1", "initial source message must be a decimal uint64"},
		{"not-a-string", float64(81), "initial source message must be a decimal string"},
		{"past-uint64", "18446744073709551616", "initial source message must be a decimal uint64"},
	} {
		t.Run("forwarded-refused-"+tc.name, func(t *testing.T) {
			meta := initialReplayTestMeta()
			if tc.value == nil {
				delete(meta, bgptypes.InitialSourceMessageMeta)
			} else {
				meta[bgptypes.InitialSourceMessageMeta] = tc.value
			}
			_, err := initialReplayFromMeta(meta, 91)
			require.EqualError(t, err, tc.want, "initial-source-message %v", tc.value)
		})
	}
	t.Run("local-refuses-message", func(t *testing.T) {
		meta := map[string]any{
			bgptypes.InitialReplayMeta:        "5",
			bgptypes.InitialLocalMeta:         true,
			bgptypes.InitialSourceMessageMeta: "81",
		}
		_, err := initialReplayFromMeta(meta, 91)
		require.EqualError(t, err, "initial local replay cannot carry a forwarding source")
	})
	t.Run("no-receipt-refuses-message", func(t *testing.T) {
		_, err := initialReplayFromMeta(map[string]any{bgptypes.InitialSourceMessageMeta: "81"}, 91)
		require.EqualError(t, err, "initial replay origin lacks session receipt")
	})
}
