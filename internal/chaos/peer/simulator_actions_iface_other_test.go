//go:build !linux

package peer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/chaos/engine"
)

// TestIfaceFaultUnsupportedNonLinux verifies the non-Linux stubs never disrupt
// the session and only surface an error event when an iface was actually asked
// for (interface faults require Linux/netlink).
//
// VALIDATES: AC-6 -- iface actions degrade gracefully off Linux.
// PREVENTS: a build break or a spurious disconnect on non-Linux platforms.
func TestIfaceFaultUnsupportedNonLinux(t *testing.T) {
	var events []Event
	emit := func(e Event) { events = append(events, e) }

	res := executeIfaceLinkFlap(engine.ChaosAction{Type: engine.ActionIfaceLinkFlap}, emit)
	require.False(t, res.Disconnected, "no-op without iface")
	require.Empty(t, events, "no error without an iface param")

	res = executeIfaceLinkFlap(engine.ChaosAction{
		Type:   engine.ActionIfaceLinkFlap,
		Params: map[string]string{engine.ParamIface: "veth0"},
	}, emit)
	require.False(t, res.Disconnected, "stub never disconnects")
	require.Len(t, events, 1, "an error event is surfaced when an iface is requested off Linux")
}
