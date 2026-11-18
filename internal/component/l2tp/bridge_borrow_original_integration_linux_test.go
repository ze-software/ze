//go:build integration && linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- actual original capture refusal.
package l2tp

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/test/userns"
	"golang.org/x/sys/unix"
)

// The production capture is invoked inside the SAME original-file borrow. A
// genuine unattached /dev/ppp file reaches SIOCGSKNS and refuses ENOTTY before
// it can yield a channel grant. This is native refusal, not an injected errno.
// The old raw -1 wrapper expected EBADF; the typed source contract instead
// requires a real retained file and queries its socket namespace first.
func TestMaybeBridgePPPoEBorrowsForNativeIoctl(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, original.Close()) })
	borrows := 0
	lease := &bridgeLeaseForTest{original: func(use func(*os.File) error) error {
		borrows++
		err := use(original)
		require.ErrorIs(t, err, unix.ENOTTY, "actual native refusal must return while the SAME file is borrowed")
		return err
	}}
	owners := &kernelPPPOwners{}
	t.Cleanup(func() {
		for _, failure := range closeOriginalKernelPPP(owners, nil, (*os.File).Close) {
			require.NoError(t, failure)
		}
	})
	requested, err := maybeBridgePPPoE(kernelSetupEvent{channel: lease}, owners)
	require.True(t, requested)
	require.ErrorIs(t, err, unix.ENOTTY)
	require.Equal(t, 1, borrows)
	require.Nil(t, owners.channel)
	require.Nil(t, owners.bridge)
	_, err = original.Stat()
	require.NoError(t, err)
}
