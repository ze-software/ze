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
// Method: select the reader this package registers for the kernel data plane,
// the one a config naming 'interface { backend netlink; }' selects, and call
// the handler over it.
func TestShowMPLSForwardingOffLinuxIsAnError(t *testing.T) {
	reader, refusal := readerForDataPlane(kernelDataPlane)
	require.Nil(t, refusal)
	withForwardingReader(t, reader)
	resp, err := handleShowMPLSForwarding(nil, commandtest.Args())
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, runtime.GOOS)
	assert.Contains(t, resp.Error, "Linux")
}
