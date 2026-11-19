// Design: docs/architecture/l2tp/subscriber-session-model.md -- registered clear by subscriber attribute
//go:build linux

package l2tp_test

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestClearSessionByUserOrAddressThroughTheDispatcher types each new form as
// an operator does, through the daemon's command dispatcher, against a real
// admitted session and kernel worker. A clear succeeds only once the
// original's native cleanup is confirmed, and a value the session no longer
// carries (its replacement has no user yet) selects nothing and mutates
// nothing.
func TestClearSessionByUserOrAddressThroughTheDispatcher(t *testing.T) {
	for _, line := range []string{
		"clear l2tp session user alice@example.net reason maintenance",
		"clear l2tp session address 192.0.2.10 actor noc cause 6",
	} {
		t.Run(strings.Fields(line)[3], func(t *testing.T) {
			subsystem, _, admit, observe := l2tp.SessionCaptureDispatchFixtureForTest(t)
			l2tp.SetSessionSubscriberForTest(t, subsystem, 7, "alice@example.net", netip.MustParseAddr("192.0.2.10"))
			dispatch := captureActionDispatcher(t, subsystem)

			answer := dispatch(line)
			require.Equal(t, plugin.StatusDone, answer.Status, answer.Error)
			resources, deletes, closes := observe()
			require.Zero(t, resources, "the clear must confirm its original native cleanup")
			require.Equal(t, 1, deletes)
			require.Equal(t, 3, closes)

			admit()
			answer = dispatch(line)
			require.Equal(t, plugin.StatusError, answer.Status, "the replacement does not carry the cleared subscriber's user or address")
			require.True(t, strings.HasPrefix(answer.Error, "No L2TP session matches "), answer.Error)
			resources, deletes, closes = observe()
			require.Equal(t, 1, resources)
			require.Equal(t, 1, deletes)
			require.Equal(t, 3, closes)
		})
	}
}

// TestClearSessionByUserOrAddressRefusesBadInputBeforeMutation checks two
// refusals through the daemon dispatcher: the address form refuses an IPv6
// address, because it takes only the IPv4 address IPCP assigned, and the user
// form refuses the tunnel-id qualifier, which narrows a session ID and this
// form names none. Each refusal comes before any selection, so the session's
// resource stays and nothing is deleted or closed.
func TestClearSessionByUserOrAddressRefusesBadInputBeforeMutation(t *testing.T) {
	subsystem, _, _, observe := l2tp.SessionCaptureDispatchFixtureForTest(t)
	l2tp.SetSessionSubscriberForTest(t, subsystem, 7, "alice@example.net", netip.MustParseAddr("192.0.2.10"))
	dispatch := captureActionDispatcher(t, subsystem)
	for _, line := range []string{
		"clear l2tp session address 2001:db8::10",
		"clear l2tp session user alice@example.net tunnel-id 100",
	} {
		answer := dispatch(line)
		require.Equal(t, plugin.StatusError, answer.Status, line)
		resources, deletes, closes := observe()
		require.Equal(t, 1, resources, line)
		require.Zero(t, deletes, line)
		require.Zero(t, closes, line)
	}
}
