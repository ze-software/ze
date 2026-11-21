//go:build integration && linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- actual typed bridge consumer.
package l2tp

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stretchr/testify/require"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/callsink"
	"github.com/ze-software/ze/internal/test/userns"
)

type originalBridgeTransport struct {
	original *os.File
	borrows  int
	releases int
}

func (transport *originalBridgeTransport) RetainOriginalFile() (callsink.RelayFileLease, error) {
	return transport, nil
}
func (transport *originalBridgeTransport) Close() error { transport.releases++; return nil }
func (*originalBridgeTransport) WithDescriptor(func(int) error) error {
	panic("BUG: typed bridge requested raw transport")
}
func (transport *originalBridgeTransport) WithOriginalFile(use func(*os.File) error) error {
	transport.borrows++
	return use(transport.original)
}

// The actual worker bridge consumer captures its peer from the SAME genuine
// PPPoX file, consumes both new attached channels, and closes both through its
// retained exact bridge before either separate transport owner is released.
func TestLACChannelBridge(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	t.Run("unattached original refuses native capture", func(t *testing.T) {
		original, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, original.Close()) })
		partial, err := pppkernel.CreatePPPChannel(original)
		if partial != nil {
			t.Cleanup(func() { require.NoError(t, partial.Close()) })
		}
		require.ErrorIs(t, err, unix.ENOTTY, "unattached /dev/ppp MUST reach and fail the actual SIOCGSKNS capture boundary")
		_, err = original.Stat()
		require.NoError(t, err, "refused capture consumed the borrowed original")
	})
	local := kernelCounterTransport(t)
	peer := kernelCounterTransport(t)
	channel, err := pppkernel.CreatePPPChannel(local)
	require.NoError(t, err)
	owners := &kernelPPPOwners{channel: channel}
	transport := &originalBridgeTransport{original: peer}
	t.Cleanup(func() {
		failures := closeOriginalKernelPPP(owners, nil, (*os.File).Close)
		for _, failure := range failures {
			require.NoError(t, failure)
		}
	})
	requested, err := maybeBridgePPPoE(kernelSetupEvent{channel: transport}, owners)
	require.NoError(t, err)
	require.True(t, requested)
	require.Equal(t, 1, transport.borrows)
	require.Nil(t, owners.channel, "constructor-consumed owner must not remain independently closable")
	require.NotNil(t, owners.bridge)
	require.True(t, owners.bridge.ClosePending())
	require.NoError(t, unbridgeOriginalKernelPPP(owners))
	require.NoError(t, owners.bridge.Close())
	require.False(t, owners.bridge.ClosePending())
	failures := closeOriginalKernelPPP(owners, nil, (*os.File).Close)
	for _, failure := range failures {
		require.NoError(t, failure)
	}
	require.Equal(t, 1, transport.releases)
	_, err = local.Stat()
	require.NoError(t, err, "channel cleanup must not close original PPPoX transport")
	_, err = peer.Stat()
	require.NoError(t, err)
}
