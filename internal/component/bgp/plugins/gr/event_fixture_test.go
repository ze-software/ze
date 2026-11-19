// Design: docs/guide/graceful-restart.md -- original physical GR event fixtures.
package gr

import "testing"

// grEventFixture binds an original sent OPEN and owns its state-manager timers.
// Callers MUST supply the captured owner and connection used by their events;
// cleanup MUST retire this manager's same original owner after the test.
func grEventFixture(t testing.TB, address string, owner, connection uint64) *grPlugin {
	t.Helper()
	source := grFixtureSource(t, address, owner, connection)
	gp := &grPlugin{
		peerCaps:     make(map[uint64]*grPeerCap),
		peerLLGRCaps: make(map[uint64]*llgrPeerCap),
		state:        grFixtureState(t, source, nil),
	}
	gp.recordSentLLGR(source, nil)
	return gp
}
