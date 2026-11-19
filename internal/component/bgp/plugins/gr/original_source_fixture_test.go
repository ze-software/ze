// Design: docs/guide/graceful-restart.md -- original physical source fixtures.
package gr

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// grFixtureSource constructs exactly the actor and physical lifetime supplied by
// the question. No address lookup or implicit owner can create fixture authority.
func grFixtureSource(t testing.TB, address string, owner, connection uint64) grSource {
	t.Helper()
	source, err := newGRSource(address, owner, connection)
	require.NoError(t, err)
	return source
}

// grFixtureState stops every remaining timer of its explicit original owner
// when the question ends, preserving the actual state manager and callbacks.
func grFixtureState(t testing.TB, source grSource, expired func(grSource)) *grStateManager {
	t.Helper()
	manager := newGRStateManager(expired)
	t.Cleanup(func() { manager.removePeer(source.grPeer) })
	return manager
}
