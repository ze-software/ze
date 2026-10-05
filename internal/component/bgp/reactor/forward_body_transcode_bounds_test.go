// Design: docs/architecture/bgp/structural-forwarding.md -- cross-context forwarding.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForwardTranscodeAttributeSectionBounds checks the real forwarding consumer:
// a malformed attribute section returns its precise cause and emits no UPDATE.
// This enters at forwarding, not socket ingress; it does not claim that receive
// validation admits this malformed body from a peer.
func TestForwardTranscodeAttributeSectionBounds(t *testing.T) {
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)
	body := []byte{0, 0, 0, 3, 0x40, 2, 6, 2, 1, 0, 0, 0, 1}

	// RFC 4271 Section 4.3: section extraction alone leaves attributes lazy.
	update, err := message.UnpackUpdate(body)
	require.NoError(t, err)
	// RFC 4271 Section 4.3; RFC 6793 Section 4.2.2.
	transcoded, handle, err := fwdUpdateForDestination(update, srcCtxID, destCtxID, 0)
	defer returnReadBuffer(handle)
	require.ErrorIs(t, err, wireu.ErrUpdateMalformed)
	assert.Nil(t, transcoded)
	assert.Nil(t, handle.Buf, "rejected transcode must return its borrowed buffer")

	// RFC 4271 Section 4.3; RFC 6793 Section 4.2.2.
	result, ok := buildFwdBody(wireu.NewWireUpdate(body, srcCtxID),
		message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.20"), &fwdParseCache{})
	defer returnReadBuffer(result.transcodeBuf)
	require.False(t, ok, "the destination must be suppressed on a malformed attribute section")
	assert.Empty(t, result.rawBodies)
	assert.Empty(t, result.updates)
	assert.Nil(t, result.transcodeBuf.Buf)
}

// TestForwardTranscodeSectionBoundaryPreservesNLRI prevents rejection of valid
// cross-context forwarding by checking the exact output attributes and NLRI.
func TestForwardTranscodeSectionBoundaryPreservesNLRI(t *testing.T) {
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)
	body := []byte{
		0, 0, 0, 20,
		0x40, 1, 1, 0,
		0x40, 2, 6, 2, 1, 0, 0, 0, 1,
		0x40, 3, 4, 192, 0, 2, 1,
		24, 198, 51, 100,
	}
	// RFC 4271 Section 4.3; RFC 6793 Section 4.2.2.
	result, ok := buildFwdBody(wireu.NewWireUpdate(body, srcCtxID),
		message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.20"), &fwdParseCache{})
	defer returnReadBuffer(result.transcodeBuf)
	require.True(t, ok)
	require.Len(t, result.updates, 1)
	assert.Empty(t, result.rawBodies)
	wantAttrs := []byte{0x40, 1, 1, 0, 0x40, 2, 4, 2, 1, 0, 1, 0x40, 3, 4, 192, 0, 2, 1}
	assert.Equal(t, wantAttrs, result.updates[0].PathAttributes)
	assert.Equal(t, []byte{24, 198, 51, 100}, result.updates[0].NLRI)
	assert.Empty(t, result.updates[0].WithdrawnRoutes)
}
