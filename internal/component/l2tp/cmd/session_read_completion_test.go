// Design: docs/guide/l2tp.md -- readable original-session completion.
package cmd

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// MUTATION: omit the registered read completion or filter a configured listener
// value as if it were an audit keyword. Names remain useful at value positions;
// audit-only keywords must remain absent from read-keyword completion.
func TestSessionReadCompletionKeepsListenerNamesMatchingAuditKeywords(t *testing.T) {
	publishFake(t, &fakeService{listeners: []l2tp.ListenerSnapshot{
		{Name: "edge", Addr: netip.MustParseAddrPort("127.0.0.1:1917")},
		{Name: "reason", Addr: netip.MustParseAddrPort("127.0.0.1:1918")},
		{Name: "actor", Addr: netip.MustParseAddrPort("127.0.0.1:1919")},
		{Name: "cause", Addr: netip.MustParseAddrPort("127.0.0.1:1920")},
		{Name: "listener", Addr: netip.MustParseAddrPort("127.0.0.1:1921")},
		{Name: "tunnel-id", Addr: netip.MustParseAddrPort("127.0.0.1:1922")},
		{Name: "expected-id", Addr: netip.MustParseAddrPort("127.0.0.1:1923")},
	}})
	for _, method := range []string{"ze-l2tp:session", "ze-l2tp:observer", "ze-l2tp:session-history", "ze-l2tp:session-traffic"} {
		t.Run(method, func(t *testing.T) {
			var registered *pluginserver.RPCRegistration
			registrations := pluginserver.AllBuiltinRPCs()
			for index := range registrations {
				if registrations[index].WireMethod == method {
					require.Nil(t, registered)
					registered = &registrations[index]
				}
			}
			require.NotNil(t, registered)
			require.NotNil(t, registered.Complete, "read completion must reach the real registered command")
			var names []string
			for _, candidate := range registered.Complete(nil, []string{"7", "listener"}) {
				names = append(names, candidate.Value)
			}
			require.ElementsMatch(t, []string{"edge", "reason", "actor", "cause", "listener", "tunnel-id", "expected-id"}, names)
			var keywords []string
			for _, candidate := range registered.Complete(nil, []string{"7"}) {
				keywords = append(keywords, candidate.Value)
			}
			require.ElementsMatch(t, []string{"expected-id", "tunnel-id", "listener"}, keywords)
			for _, name := range names {
				var remaining []string
				for _, candidate := range registered.Complete(nil, []string{"7", "listener", name}) {
					remaining = append(remaining, candidate.Value)
				}
				require.ElementsMatch(t, []string{"expected-id", "tunnel-id"}, remaining, "completion after listener value %q must use its position, not its text", name)
			}
		})
	}
}

// A listener name is a value at this position even when its text is an audit
// keyword. Completion must never offer a configured name that selection refuses.
// MUTATION: interpret the listener value as the next disconnect audit keyword.
func TestSessionReadListenerNamesCanMatchAuditKeywords(t *testing.T) {
	for _, name := range []string{"edge", "reason", "actor", "cause", "listener", "tunnel-id", "expected-id"} {
		t.Run(name, func(t *testing.T) {
			service := newSessionReadService(t)
			service.listeners[0].Name = name
			publishFake(t, service)
			response, err := handleSession(nil, commandtest.Args("7", "listener", name))
			require.NoError(t, err)
			require.Equal(t, []l2tp.SessionSelector{{SID: 7, Listener: service.listeners[0].Addr}}, service.selectors)
			require.Equal(t, plugin.StatusDone, response.Status, response.Error)
			require.Zero(t, service.unsafeReads)
		})
	}
}
