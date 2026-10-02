// VALIDATES: RFC 6286 Section 2.1, the half of "unique within an AS" that is this
// speaker's own identifier: a speaker of the local AS presenting it is refused.
// PREVENTS: two speakers of one AS running with one BGP Identifier because the
// local identifier was never compared with an internal peer's.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestRFC6286LocalIdentifierUniqueWithinItsAS opens a session toward an internal
// peer (AS 65001, the local AS of openSentSessionAS) whose OPEN carries this
// speaker's own identifier 1.2.3.1, and checks the refusal on the wire.
func TestRFC6286LocalIdentifierUniqueWithinItsAS(t *testing.T) {
	const localID uint32 = 0x01020301

	// RFC requirement: RFC6286-2.1-2 positive -- this speaker's own BGP Identifier stays unique within its AS: an OPEN from a speaker of the same AS (internal peer, AS 65001) carrying identifier 0x01020301, equal to the local one, is refused with ErrBadBGPIdentifier and NOTIFICATION 2/3 (OPEN Message Error, Bad BGP Identifier), and the session does not reach OpenConfirm
	session, written := openSentSessionAS(t, 65001, localID)

	err := session.handleOpen(openBodyWithIdentifier(65001, localID))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBadBGPIdentifier)
	assert.NotEqual(t, fsm.StateOpenConfirm, session.State(), "a duplicate identifier within the AS must not advance the FSM")

	code, subcode, found := notificationFrom(t, written)
	require.True(t, found, "the refusal is sent to the peer")
	assert.Equal(t, uint8(message.NotifyOpenMessage), code, "OPEN Message Error")
	assert.Equal(t, message.NotifyOpenBadBGPID, subcode, "Bad BGP Identifier")
}
