// Design: docs/architecture/api/update-syntax.md -- the replay-eor meta
// Related: reactor_api_forward.go -- AnnounceReplayEOR, announceEOR

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnnounceReplayEORSendsTheSessionsMarkerOnce pins the gate the replay
// form keeps: a plugin's replay close is the session's one initial-sync
// End-of-RIB for the family (RFC 4724 Section 2), so it goes out only when no
// marker for the family has gone out on this session.
//
// VALIDATES: the first replay close is written; a second replay close, and a
// replay close after an explicit marker, write nothing and answer no error.
// PREVENTS: a route server's or a route reflector's replay repeating the
// marker the reactor's own initial sync, or the replay before it, sent.
func TestAnnounceReplayEORSendsTheSessionsMarkerOnce(t *testing.T) {
	ipv4 := uint16(family.AFIIPv4)
	unicast := uint8(family.SAFIUnicast)
	eor := eorWire(family.IPv4Unicast)

	t.Run("replay then replay", func(t *testing.T) {
		adapter, _, conn := newEORGuardPeer(t)
		require.NoError(t, adapter.AnnounceReplayEOR(selector.All(), ipv4, unicast, plugin.OperatorSender()))
		require.NoError(t, adapter.AnnounceReplayEOR(selector.All(), ipv4, unicast, plugin.OperatorSender()))
		assert.Equal(t, eor, conn.written(), "one marker per family per session")
	})
	t.Run("explicit then replay", func(t *testing.T) {
		adapter, _, conn := newEORGuardPeer(t)
		require.NoError(t, adapter.AnnounceEOR(selector.All(), ipv4, unicast, plugin.OperatorSender()))
		require.NoError(t, adapter.AnnounceReplayEOR(selector.All(), ipv4, unicast, plugin.OperatorSender()))
		assert.Equal(t, eor, conn.written(), "the explicit marker claimed the family, so the replay stands down")
	})
}
