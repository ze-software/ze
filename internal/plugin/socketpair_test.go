package plugin

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewInternalSocketPairs verifies that two net.Pipe pairs are created
// for internal (goroutine) plugins.
//
// VALIDATES: Both socket pairs are functional bidirectional connections.
// PREVENTS: Nil connections or swapped engine/plugin sides.
func TestNewInternalSocketPairs(t *testing.T) {
	t.Parallel()

	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	// Verify all connections are non-nil
	assert.NotNil(t, pairs.Engine.EngineSide)
	assert.NotNil(t, pairs.Engine.PluginSide)
	assert.NotNil(t, pairs.Callback.EngineSide)
	assert.NotNil(t, pairs.Callback.PluginSide)
}

// TestInternalSocketPairBidirectional verifies data flows correctly on both socket pairs.
//
// VALIDATES: Engine to Plugin and Plugin to Engine data transfer on both sockets.
// PREVENTS: Crossed wires between socket A and socket B.
func TestInternalSocketPairBidirectional(t *testing.T) {
	t.Parallel()

	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	// Socket A: Plugin sends to Engine (plugin is client, engine is server)
	// net.Pipe has zero buffering, so write must be concurrent with read.
	msg := []byte("hello from plugin on socket A")
	go func() {
		_, writeErr := pairs.Engine.PluginSide.Write(msg)
		assert.NoError(t, writeErr)
	}()

	buf := make([]byte, 256)
	n, err := pairs.Engine.EngineSide.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, msg, buf[:n])

	// Socket B: Engine sends to Plugin (engine is client, plugin is server)
	msg2 := []byte("hello from engine on socket B")
	go func() {
		_, writeErr := pairs.Callback.EngineSide.Write(msg2)
		assert.NoError(t, writeErr)
	}()

	n, err = pairs.Callback.PluginSide.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, msg2, buf[:n])
}

// TestDualSocketPairClose verifies that Close() closes all connections.
//
// VALIDATES: All four connections are closed after Close().
// PREVENTS: Resource leak from unclosed connections.
func TestDualSocketPairClose(t *testing.T) {
	t.Parallel()

	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)

	pairs.Close()

	// After close, writes should fail
	_, err = pairs.Engine.PluginSide.Write([]byte("should fail"))
	assert.Error(t, err)

	_, err = pairs.Callback.EngineSide.Write([]byte("should fail"))
	assert.Error(t, err)
}

// TestNewExternalSocketPairs verifies that syscall.Socketpair creates two pairs.
//
// VALIDATES: Socket pairs created with correct FDs for external plugins.
// PREVENTS: FD leak or incorrect FD assignment.
func TestNewExternalSocketPairs(t *testing.T) {
	t.Parallel()

	pairs, err := NewExternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	// Verify all connections are non-nil
	assert.NotNil(t, pairs.Engine.EngineSide)
	assert.NotNil(t, pairs.Engine.PluginSide)
	assert.NotNil(t, pairs.Callback.EngineSide)
	assert.NotNil(t, pairs.Callback.PluginSide)
}

// TestExternalSocketPairBidirectional verifies data flows on external socket pairs.
//
// VALIDATES: Socket pairs created via socketpair() are functional.
// PREVENTS: FD conversion to net.Conn breaking the connection.
func TestExternalSocketPairBidirectional(t *testing.T) {
	t.Parallel()

	pairs, err := NewExternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	// Socket A: Plugin to Engine
	msg := []byte("external plugin to engine")
	_, err = pairs.Engine.PluginSide.Write(msg)
	require.NoError(t, err)

	buf := make([]byte, 256)
	n, err := pairs.Engine.EngineSide.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, msg, buf[:n])

	// Socket B: Engine to Plugin
	msg2 := []byte("external engine to plugin")
	_, err = pairs.Callback.EngineSide.Write(msg2)
	require.NoError(t, err)

	n, err = pairs.Callback.PluginSide.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, msg2, buf[:n])
}

// TestExternalSocketPairPluginFiles verifies PluginFiles returns correct os.File handles.
//
// VALIDATES: Plugin-side FDs are extractable for ExtraFiles inheritance.
// PREVENTS: External subprocess receiving wrong FDs.
func TestExternalSocketPairPluginFiles(t *testing.T) {
	t.Parallel()

	pairs, err := NewExternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	engineFile, callbackFile, err := pairs.PluginFiles()
	require.NoError(t, err)
	assert.NotNil(t, engineFile)
	assert.NotNil(t, callbackFile)

	// Verify the files are usable (non-nil was already checked)
	assert.NotEqual(t, ^uintptr(0), engineFile.Fd(), "engine file should have valid FD")
	assert.NotEqual(t, ^uintptr(0), callbackFile.Fd(), "callback file should have valid FD")

	require.NoError(t, engineFile.Close())
	require.NoError(t, callbackFile.Close())
}

// TestSocketPairType verifies the SocketPair struct fields map to correct roles.
//
// VALIDATES: SocketPair struct fields map to correct roles.
// PREVENTS: Confusion between engine and plugin sides.
func TestSocketPairType(t *testing.T) {
	t.Parallel()

	// Create a simple pair from net.Pipe
	a, b := net.Pipe()
	sp := SocketPair{
		EngineSide: a,
		PluginSide: b,
	}

	// Write from plugin, read from engine
	go func() {
		_, writeErr := sp.PluginSide.Write([]byte("test"))
		assert.NoError(t, writeErr)
	}()

	buf := make([]byte, 10)
	n, err := sp.EngineSide.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "test", string(buf[:n]))

	require.NoError(t, a.Close())
	require.NoError(t, b.Close())
}
