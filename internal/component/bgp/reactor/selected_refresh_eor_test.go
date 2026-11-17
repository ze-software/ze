// Design: docs/architecture/update-building.md -- complete connection EOR evidence.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// Accepted partial bytes do not complete EOR, even with no external observer.
// Reusing a Session in a new connection epoch does not inherit its old EOR.
func TestSelectedRefreshEORRequiresCompleteConnectionFrame(t *testing.T) {
	fixture, _ := selectedRefreshFixture(t)
	session := fixture.peer.session
	session.onWireMessage = nil
	writer := session.wireWriter
	encoding := bgpctx.Registry.Get(session.sendCtxID)
	require.NotNil(t, encoding)
	frame := message.PackTo(message.BuildEOR(family.IPv4Unicast), encoding)
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	accepted, err := writer.Write(frame[:10])
	require.NoError(t, err)
	require.Equal(t, 10, accepted)
	require.False(t, writer.sentEOR[family.IPv4Unicast])
	accepted, err = writer.Write(frame[10:])
	require.NoError(t, err)
	require.Equal(t, len(frame)-10, accepted)
	require.True(t, writer.sentEOR[family.IPv4Unicast])
	replacement := &observedBGPWriter{session: session, writer: fixture.conn}
	replacement.context.Store(uint32(session.sendCtxID))
	require.False(t, replacement.sentEOR[family.IPv4Unicast])
}
