package update

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEVPNTextIMETRequiresRouteTargetOnlyOnAnnouncement(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "multicast",
		"rd", "65000:7", "ip", "192.0.2.1",
	})
	require.Error(t, err)

	withdrawal, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "del", "multicast",
		"rd", "65000:7", "ip", "192.0.2.1",
	})
	require.NoError(t, err)
	require.Len(t, withdrawal.Groups, 1)
	require.Empty(t, withdrawal.Groups[0].Announce)
	require.Len(t, withdrawal.Groups[0].Withdraw, 1)
	require.Equal(t, byte(3), withdrawal.Groups[0].Withdraw[0].Bytes()[0])
}
