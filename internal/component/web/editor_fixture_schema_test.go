// Design: docs/architecture/web-interface.md -- per-user editor fixtures.
// Related: editor_test.go -- the shared manager seed must use the current schema.

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebEditorFixtureStartsWithParsedTree opens the shared seed through the
// real manager and checks its promised BGP values, so a parser's empty-tree
// fallback cannot make unrelated manager tests appear to cover a valid seed.
func TestWebEditorFixtureStartsWithParsedTree(t *testing.T) {
	manager := newTestEditorManager(t)
	_, err := manager.GetOrCreate("fixture-reader")
	require.NoError(t, err)
	tree := manager.snapshotTree("fixture-reader")
	require.NotNil(t, tree)
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	routerID, found := bgp.Get("router-id")
	require.True(t, found)
	assert.Equal(t, "1.2.3.4", routerID)
	session := bgp.GetContainer("session")
	require.NotNil(t, session)
	asn := session.GetContainer("asn")
	require.NotNil(t, asn)
	localASN, found := asn.Get("local")
	require.True(t, found)
	assert.Equal(t, "65000", localASN)
	assert.Zero(t, manager.ChangeCount("fixture-reader"))
}
