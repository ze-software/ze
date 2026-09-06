// Design: docs/architecture/api/commands.md -- the withdrawal answer this test reads
// Related: update_text.go -- handleUpdateText, which turns the reactor's reason into a warning
//
// RFC 4271 Section 4.3 identifies a withdrawn route "in the context of the BGP
// speaker - BGP speaker connection to which it has been previously advertised",
// so the API rail writes no withdrawal to a connection that has advertised
// nothing (reactor.withdrawBatchFromPeers). The reactor reports that as
// route.ErrWithdrawWithheld.
//
// This file pins what the COMMAND does with it. The reactor test proves the
// bytes stay off the wire; nothing proved the operator still reads `done` and
// still learns which peer went unwritten, and an ExaBGP script blocked on that
// answer is the caller who pays for a wrong one.

package update

import (
	"fmt"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleUpdateTextWithheldWithdrawalIsDoneAndNamed drives the answer an
// ExaBGP script reads for the first withdrawal of a session.
//
// The command DID what it asked for: the route the peer never held is not on
// it, and no UPDATE was owed. So the answer is `done` and the script is
// released. A zero UPDATE count with a bare `done` and no reason would be the
// silent no-op this rail must never produce (ai/rules/principles.md), so the
// reason rides on the answer as a warning that names the peer.
//
// VALIDATES: route.ErrWithdrawWithheld from the reactor answers `done`, counts
// the withdrawal, and carries a warning naming the family and the peer (AC-7).
// PREVENTS: the reason being answered as an `error`, which makes an ExaBGP
// script read a failure for a command that did what it asked; and the reason
// being dropped, which is the silent no-op.
func TestHandleUpdateTextWithheldWithdrawalIsDoneAndNamed(t *testing.T) {
	reactor := &mockReactorBatch{
		withdrawError: fmt.Errorf("%w: ipv4/unicast, peers 10.0.0.2", route.ErrWithdrawWithheld),
	}
	ctx := &pluginserver.CommandContext{
		Server: mustNewServer(&pluginserver.ServerConfig{}, reactor),
		Peer:   "10.0.0.2",
	}

	resp, err := handleUpdateText(ctx, []string{"nlri", "ipv4/unicast", "del", "1.1.0.0/24"})

	require.NoError(t, err, "a withheld withdrawal is not a command failure")
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status, "the script is released, as it is for any other command")

	result, ok := resp.Data.(*plugin.RouteResult)
	require.True(t, ok, "the answer carries the route result the warning rides on")
	assert.Equal(t, uint32(1), result.Withdrawn, "the command took the route back")
	require.Len(t, result.Warnings, 1, "the reason reaches the operator")
	assert.Contains(t, result.Warnings[0], "10.0.0.2", "and it names the peer nothing was written to")
	assert.Contains(t, result.Warnings[0], "withdrawal withheld")
}
