// Design: docs/architecture/l2tp/subscriber-session-model.md -- selecting an original by subscriber attributes
//go:build linux

package l2tp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// SetSessionSubscriberForTest gives the dispatch fixture's live session the
// user name and IPv4 address the reactor records once PPP authenticates and
// IPCP opens (reactor_kernel.go, the session IP assignment), so the external
// command test can select that original by them.
func SetSessionSubscriberForTest(t *testing.T, subsystem *Subsystem, sid uint16, username string, address netip.Addr) {
	t.Helper()
	found := false
	for _, reactor := range subsystem.reactors {
		reactor.tunnelsMu.Lock()
		// Bounded by the fixture's tunnels.
		for _, tunnel := range reactor.tunnelsByLocalID {
			session := tunnel.sessions[sid]
			if session == nil {
				continue
			}
			session.username = username
			session.assignedAddr = address
			found = true
		}
		reactor.tunnelsMu.Unlock()
	}
	require.True(t, found, "the fixture holds no live session %d", sid)
}
