// Design: docs/architecture/chaos-web-dashboard.md -- orchestrator utility unit tests

package orchestrator

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocatePort(t *testing.T) {
	port, err := allocatePort(context.Background(), "127.0.0.1")
	require.NoError(t, err)
	assert.Greater(t, port, 0)
	assert.LessOrEqual(t, port, 65535)
}

func TestAllocatePortUnique(t *testing.T) {
	ports := make(map[int]bool)
	for range 5 {
		port, err := allocatePort(context.Background(), "127.0.0.1")
		require.NoError(t, err)
		assert.False(t, ports[port], "duplicate port %d", port)
		ports[port] = true
	}
}

func TestCheckPortFree(t *testing.T) {
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	addr := ln.Addr().String()

	err = checkPortFree(addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in use")
}

func TestCheckPortFreeAvailable(t *testing.T) {
	port, err := allocatePort(context.Background(), "127.0.0.1")
	require.NoError(t, err)

	err = checkPortFree(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	assert.NoError(t, err)
}

func TestWaitForZeTimeout(t *testing.T) {
	port, err := allocatePort(context.Background(), "127.0.0.1")
	require.NoError(t, err)

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	err = waitForZe(context.Background(), addr, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "did not start")
}

func TestWaitForZeSuccess(t *testing.T) {
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			if closeErr := conn.Close(); closeErr != nil {
				return
			}
		}
	}()

	err = waitForZe(context.Background(), ln.Addr().String(), false)
	assert.NoError(t, err)
}
