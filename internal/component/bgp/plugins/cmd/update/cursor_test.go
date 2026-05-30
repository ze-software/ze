package update

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// cursorTestCtx creates a CommandContext with a mock reactor for peer 192.0.2.1.
// Process is nil so processName is "" for cursor key.
func cursorTestCtx(t *testing.T) *pluginserver.CommandContext {
	t.Helper()
	reactor := &mockReactorBatch{}
	ctx := &pluginserver.CommandContext{
		Server: mustNewServer(&pluginserver.ServerConfig{}, reactor),
		Peer:   "192.0.2.1",
	}
	t.Cleanup(func() {
		key := cursorKey("", ctx.PeerSelector())
		cursors.Delete(key)
	})
	return ctx
}

func cursorTestReactor(t *testing.T, ctx *pluginserver.CommandContext) *mockReactorBatch {
	t.Helper()
	r, ok := ctx.Reactor().(*mockReactorBatch)
	require.True(t, ok, "reactor must be *mockReactorBatch")
	return r
}

func loadCursorAttrs(t *testing.T, peer string) *parsedAttrs {
	t.Helper()
	key := cursorKey("", peer)
	stored, ok := cursors.Load(key)
	require.True(t, ok, "cursor state should exist for key %s", key)
	attrs, ok := stored.(*parsedAttrs)
	require.True(t, ok, "cursor state must be *parsedAttrs")
	return attrs
}

// TestCursorSetup verifies that the first update cursor command initializes cursor state.
func TestCursorSetup(t *testing.T) {
	ctx := cursorTestCtx(t)

	args := strings.Fields("origin igp as-path [65001 65002] med 100 next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	reactor := cursorTestReactor(t, ctx)
	require.Len(t, reactor.announceCalls, 1)
	assert.Len(t, reactor.announceCalls[0].NLRIs, 1)

	// Verify cursor state was stored
	attrs := loadCursorAttrs(t, ctx.PeerSelector())
	require.NotNil(t, attrs.Origin)
	assert.Equal(t, uint8(0), *attrs.Origin) // igp = 0
	assert.Equal(t, []uint32{65001, 65002}, attrs.ASPath)
	require.NotNil(t, attrs.MED)
	assert.Equal(t, uint32(100), *attrs.MED)
}

// TestCursorDelta verifies that subsequent commands apply only changed attributes.
func TestCursorDelta(t *testing.T) {
	ctx := cursorTestCtx(t)

	// First: establish full state
	args := strings.Fields("origin igp as-path [65001 65002] med 100 next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)

	// Second: only change as-path
	args = strings.Fields("as-path [65001 65003] nlri ipv4/unicast add 10.1.0.0/24")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)

	reactor := cursorTestReactor(t, ctx)
	require.Len(t, reactor.announceCalls, 2)
	assert.Len(t, reactor.announceCalls[1].NLRIs, 1)

	// Verify cursor updated AS_PATH but kept other attrs
	attrs := loadCursorAttrs(t, ctx.PeerSelector())
	assert.Equal(t, []uint32{65001, 65003}, attrs.ASPath)
	require.NotNil(t, attrs.MED, "MED should be inherited")
	assert.Equal(t, uint32(100), *attrs.MED)
}

// TestCursorDel verifies that del removes an attribute from cursor state.
func TestCursorDel(t *testing.T) {
	ctx := cursorTestCtx(t)

	// First: establish state with MED
	args := strings.Fields("origin igp med 100 next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)

	// Second: del med
	args = strings.Fields("del med nlri ipv4/unicast add 10.1.0.0/24")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)

	attrs := loadCursorAttrs(t, ctx.PeerSelector())
	assert.Nil(t, attrs.MED, "MED should be removed by del")
	require.NotNil(t, attrs.Origin, "Origin should be inherited")
}

// TestCursorDelAbsent verifies that del for absent attr is a silent no-op.
func TestCursorDelAbsent(t *testing.T) {
	ctx := cursorTestCtx(t)

	// Establish without MED
	args := strings.Fields("origin igp next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)

	// Del MED that does not exist: should succeed
	args = strings.Fields("del med nlri ipv4/unicast add 10.1.0.0/24")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestCursorMultipleDel verifies multiple del keywords in one command.
func TestCursorMultipleDel(t *testing.T) {
	ctx := cursorTestCtx(t)

	args := strings.Fields("origin igp med 100 community [65000:1] next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)

	args = strings.Fields("del med del community nlri ipv4/unicast add 10.1.0.0/24")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)

	attrs := loadCursorAttrs(t, ctx.PeerSelector())
	assert.Nil(t, attrs.MED)
	assert.Nil(t, attrs.Communities)
}

// TestCursorInherit verifies NLRIs-only command inherits all cursor attributes.
func TestCursorInherit(t *testing.T) {
	ctx := cursorTestCtx(t)

	// Establish full state
	args := strings.Fields("origin igp as-path [65001] med 100 next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)

	// NLRIs only, no attr changes
	args = strings.Fields("nlri ipv4/unicast add 10.1.0.0/24 10.2.0.0/24")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)

	reactor := cursorTestReactor(t, ctx)
	require.Len(t, reactor.announceCalls, 2)
	assert.Len(t, reactor.announceCalls[1].NLRIs, 2, "should have 2 NLRIs")

	// Cursor should not have changed
	attrs := loadCursorAttrs(t, ctx.PeerSelector())
	require.NotNil(t, attrs.MED)
	assert.Equal(t, uint32(100), *attrs.MED)
}

// TestCursorDone verifies done clears cursor state.
func TestCursorDone(t *testing.T) {
	ctx := cursorTestCtx(t)

	// Establish state
	args := strings.Fields("origin igp next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)

	key := cursorKey("", ctx.PeerSelector())
	_, ok := cursors.Load(key)
	require.True(t, ok, "cursor should exist before done")

	// Send done
	args = strings.Fields("done")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	_, ok = cursors.Load(key)
	assert.False(t, ok, "cursor should be cleared after done")
}

// TestCursorReplace verifies new init replaces stale cursor (AC-10: peer flap).
func TestCursorReplace(t *testing.T) {
	ctx := cursorTestCtx(t)

	// First init
	args := strings.Fields("origin igp med 100 next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)

	// New init without done (peer flap scenario): new attributes replace old
	args = strings.Fields("origin egp med 200 next-hop 10.0.0.2 nlri ipv4/unicast add 10.1.0.0/24")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)

	attrs := loadCursorAttrs(t, ctx.PeerSelector())
	require.NotNil(t, attrs.Origin)
	assert.Equal(t, uint8(1), *attrs.Origin) // egp = 1
	require.NotNil(t, attrs.MED)
	assert.Equal(t, uint32(200), *attrs.MED)
}

// TestCursorClearProcess verifies ClearProcessCursors frees all cursors for a process.
func TestCursorClearProcess(t *testing.T) {
	// Manually set up cursor state for a specific process
	processName := "test-plugin-clear"
	key1 := cursorKey(processName, "192.0.2.1")
	key2 := cursorKey(processName, "192.0.2.2")
	otherKey := cursorKey("other-plugin", "192.0.2.1")

	med100 := uint32(100)
	cursors.Store(key1, &parsedAttrs{MED: &med100})
	cursors.Store(key2, &parsedAttrs{MED: &med100})
	cursors.Store(otherKey, &parsedAttrs{MED: &med100})

	t.Cleanup(func() {
		cursors.Delete(key1)
		cursors.Delete(key2)
		cursors.Delete(otherKey)
	})

	ClearProcessCursors(processName)

	_, ok1 := cursors.Load(key1)
	_, ok2 := cursors.Load(key2)
	_, okOther := cursors.Load(otherKey)
	assert.False(t, ok1, "key1 should be cleared")
	assert.False(t, ok2, "key2 should be cleared")
	assert.True(t, okOther, "other process cursor should not be cleared")
}

// TestCursorNoCursorState verifies error when NLRIs sent without prior cursor init.
func TestCursorNoCursorState(t *testing.T) {
	ctx := cursorTestCtx(t)

	args := strings.Fields("nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.Error(t, err)
	assert.ErrorIs(t, err, errCursorNoCursorState)
}

// TestCursorMissingNLRI verifies error when command has no nlri section.
func TestCursorMissingNLRI(t *testing.T) {
	ctx := cursorTestCtx(t)

	args := strings.Fields("origin igp")
	_, err := handleUpdateCursor(ctx, args)
	require.Error(t, err)
	assert.ErrorIs(t, err, errCursorMissingNLRI)
}

// TestCursorDelMissingKeyword verifies error for del without keyword.
func TestCursorDelMissingKeyword(t *testing.T) {
	ctx := cursorTestCtx(t)

	// Establish cursor first
	args := strings.Fields("origin igp next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	_, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)

	args = []string{"del"}
	_, err = handleUpdateCursor(ctx, args)
	require.Error(t, err)
	assert.ErrorIs(t, err, errCursorDelMissingKW)
}

// TestCursorAliasResolution verifies short-form aliases work in cursor mode.
func TestCursorAliasResolution(t *testing.T) {
	ctx := cursorTestCtx(t)

	// Use aliases: "next" for "next-hop", "pref" for "local-preference", "path" for "as-path"
	args := strings.Fields("origin igp path [65001] pref 200 next 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
	resp, err := handleUpdateCursor(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)

	attrs := loadCursorAttrs(t, ctx.PeerSelector())
	assert.Equal(t, []uint32{65001}, attrs.ASPath)
	require.NotNil(t, attrs.LocalPreference)
	assert.Equal(t, uint32(200), *attrs.LocalPreference)
}

// TestHandleUpdateCursorViaSwitch verifies "cursor" is accepted by handleUpdate dispatch.
func TestHandleUpdateCursorViaSwitch(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &pluginserver.CommandContext{
		Server: mustNewServer(&pluginserver.ServerConfig{}, reactor),
		Peer:   "192.0.2.1",
	}
	t.Cleanup(func() {
		cursors.Delete(cursorKey("", ctx.PeerSelector()))
	})

	args := []string{"cursor", "origin", "igp", "next-hop", "10.0.0.1", "nlri", "ipv4/unicast", "add", "10.0.0.0/24"}
	resp, err := handleUpdate(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestHandleUpdateUsageIncludesCursor verifies the error message lists cursor.
func TestHandleUpdateUsageIncludesCursor(t *testing.T) {
	assert.Contains(t, errUsagePeerAddrUpdateTexthexb64.Error(), "cursor")
}
