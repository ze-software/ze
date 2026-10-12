//go:build !linux

// Design: docs/architecture/mpls/mpls-kernel.md -- non-Linux `show mpls forwarding` refusal

package mpls

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestShowMPLSForwardingOffLinuxIsAnError proves `show mpls forwarding` on a
// host that is not Linux says the table cannot be read, naming the OS, rather
// than answering an empty table a caller would read as "no MPLS routes".
//
// VALIDATES: the handler, over the real platform reader, answers StatusError
// with a message naming runtime.GOOS.
// PREVENTS: dumpMPLSRoutes returning nil, nil off Linux.
//
// Method: call the handler with no seam swapped.
func TestShowMPLSForwardingOffLinuxIsAnError(t *testing.T) {
	resp, err := handleShowMPLSForwarding(nil, commandtest.Args())
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, runtime.GOOS)
	assert.Contains(t, resp.Error, "Linux")
}
