// Design: docs/architecture/api/update-syntax.md -- the End-of-RIB form and its replay-eor meta
// Related: update_text.go -- handleUpdateText, which picks the reactor call

package update

import (
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleUpdateTextEORPicksTheReactorCall pins which reactor entry an
// `update text ... eor` reaches.
//
// VALIDATES: a plain EOR (an operator or a script) reaches AnnounceEOR, which
// sends the marker each time; the same command carrying the replay-eor meta
// (the route server's and the route reflector's replay close) reaches
// AnnounceReplayEOR, which stands down once the session carried the marker.
// PREVENTS: a replay close repeating the initial-sync End-of-RIB (RFC 4724
// Section 2), and a script's explicit EOR being swallowed by that gate.
func TestHandleUpdateTextEORPicksTheReactorCall(t *testing.T) {
	tests := []struct {
		name       string
		meta       map[string]any
		wantEOR    int
		wantReplay int
	}{
		{name: "plain", meta: nil, wantEOR: 1},
		{name: "replay", meta: map[string]any{bgptypes.ReplayEORMeta: true}, wantReplay: 1},
		{name: "replay false", meta: map[string]any{bgptypes.ReplayEORMeta: false}, wantEOR: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactorBatch{}
			ctx := &pluginserver.CommandContext{
				Server: mustNewServer(&pluginserver.ServerConfig{}, reactor),
				Peer:   "10.0.0.2",
				Meta:   tt.meta,
			}

			resp, err := handleUpdateText(ctx, []string{"nlri", "eor"})

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.wantEOR, reactor.eorCalls, "AnnounceEOR calls")
			assert.Equal(t, tt.wantReplay, reactor.replayEORCalls, "AnnounceReplayEOR calls")
		})
	}
}
