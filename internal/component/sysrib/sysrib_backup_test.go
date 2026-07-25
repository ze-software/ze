// VALIDATES: AC-10 / R-4 -- a fast-reroute backup next-hop is forwarded on the
// dedicated Backup field, never folded into the ECMP sibling set (ECMP paths
// load-share; a backup is used only on primary failure).
// PREVENTS: the backup load-sharing onto the worse path in steady state.
package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

func TestBackupNotFoldedIntoECMP(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	payload := makePayload("ospf", family.IPv4Unicast, []incomingChange{{
		Action:             routeaction.Add,
		Prefix:             netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:            netip.MustParseAddr("192.168.1.1"),
		Priority:           110,
		BackupNextHop:      netip.MustParseAddr("192.168.9.9"),
		BackupRepairLabels: []uint32{16010},
	}})
	_, changes := s.processEvent(payload)
	require.Len(t, changes, 1)

	assert.Empty(t, changes[0].ECMPPaths, "backup must not appear as an ECMP sibling")
	require.Len(t, changes[0].Backup, 1, "backup must ride the dedicated Backup field")
	assert.Equal(t, netip.MustParseAddr("192.168.9.9"), changes[0].Backup[0].NextHop)
	assert.Equal(t, []uint32{16010}, changes[0].Backup[0].Labels)
}
