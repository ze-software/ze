//go:build linux

package peer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/chaos/engine"
)

// TestIfaceFaultNoOpWithoutIface verifies the netns-scoped interface fault
// actions are a graceful no-op when no interface is configured -- the common
// case on a plain loopback chaos run (no netns/veth). The real kernel effect is
// exercised by the integration test (simulator_actions_iface_integration_linux_test.go).
//
// VALIDATES: AC-6 -- iface actions never disrupt a non-netns run and never
// touch the kernel without an explicit iface param.
// PREVENTS: an iface chaos action flapping a real host interface when scheduled
// on a loopback session, and a nil-param panic.
func TestIfaceFaultNoOpWithoutIface(t *testing.T) {
	var events []Event
	emit := func(e Event) { events = append(events, e) }

	res := executeIfaceLinkFlap(engine.ChaosAction{Type: engine.ActionIfaceLinkFlap}, emit)
	require.False(t, res.Disconnected, "link-flap with no iface must be a no-op")

	res = executeIfaceAddrRemove(engine.ChaosAction{Type: engine.ActionIfaceAddrRemove, Params: map[string]string{}}, emit)
	require.False(t, res.Disconnected, "addr-remove with no iface must be a no-op")

	require.Empty(t, events, "no error events expected for the no-op path")
}
