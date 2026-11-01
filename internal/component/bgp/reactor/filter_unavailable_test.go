package reactor

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// unavailableError is what the plugin server answers for a filter whose
// plugin's slot answers rpc.ErrBridgeClosed (server.CallFilterUpdate, proven
// by TestCallFilterUpdateUnavailable in plugin/server).
func unavailableError(plugin, filter string) error {
	return fmt.Errorf("filter %s: plugin %q is unavailable (stopped, crashed or respawning): %w",
		filter, plugin, rpc.ErrBridgeClosed)
}

// unavailableTransport is a plugin server whose one filter, "probe:f", has
// a stopped plugin: every call answers the server's "unavailable" error, and
// the member's on-error is onError.
type unavailableTransport struct {
	externalTransport
	onError rpc.OnErrorPolicy
	calls   int
}

func (u *unavailableTransport) FilterDeclaration(plugin, filter string) ([]string, []string, rpc.OnErrorPolicy, bool) {
	reads, sets, _, ok := u.externalTransport.FilterDeclaration(plugin, filter)
	return reads, sets, u.onError, ok
}

func (u *unavailableTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy { return u.onError }

func (u *unavailableTransport) CallFilterUpdate(_ context.Context, plugin string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	u.calls++
	return nil, unavailableError(plugin, route.Filter())
}

// TestFilterUnavailableAppliesOnError proves AC-33's engine half: a member
// whose plugin is unavailable (its slot answers rpc.ErrBridgeClosed, which
// the server reports as "unavailable") takes the member's on-error, never a
// verdict, and the log line names the plugin and says "unavailable". Method:
// a transport answering the server's unavailable error, judged by the text
// chain under both on-error policies (the log line asserted there) and by a
// mixed library chain under reject (policychain logs that member's failure
// with the same error).
func TestFilterUnavailableAppliesOnError(t *testing.T) {
	const (
		plug   = "probe"
		filter = "f"
	)

	t.Run("text chain, on-error reject", func(t *testing.T) {
		sink := captureReactorLog(t)
		transport := &unavailableTransport{onError: rpc.OnErrorReject}
		r := &Reactor{filterTransportSeam: transport}

		got := r.policyFilterFunc(nil, nil, policyview.Session{})(plug, filter, directionImport, "10.0.0.2", 65002, "origin igp")

		if got.Action != PolicyReject || !got.Failed {
			t.Fatalf("got %+v, want a failed reject: on-error reject applies, and no filter decided it", got)
		}
		if transport.calls != 1 {
			t.Fatalf("the filter was called %d times, want 1", transport.calls)
		}
		line := sink.String()
		if !strings.Contains(line, `plugin \"probe\" is unavailable`) {
			t.Fatalf("the log does not name the plugin as unavailable:\n%s", line)
		}
	})

	t.Run("text chain, on-error accept", func(t *testing.T) {
		r := &Reactor{filterTransportSeam: &unavailableTransport{onError: rpc.OnErrorAccept}}

		got := r.policyFilterFunc(nil, nil, policyview.Session{})(plug, filter, directionImport, "10.0.0.2", 65002, "origin igp")

		if got.Action != PolicyAccept || got.Failed {
			t.Fatalf("got %+v, want an accept: the operator chose on-error accept", got)
		}
	})

	t.Run("mixed library chain, on-error reject", func(t *testing.T) {
		transport := &unavailableTransport{onError: rpc.OnErrorReject,
			externalTransport: externalTransport{reads: []string{"local-preference"}}}
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
		received := libraryTestBody(100)
		addr := netip.MustParseAddr("10.0.0.4")
		peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
			ImportPolicy: mixedPolicy(t, libraryTypeLP200,
				policychain.Member{Name: "member"}, policychain.Member{Name: filter, Plugin: plug})})

		if in := r.runIngressPolicyChain(peer, addr, testWireUpdate(received), received); in.accept {
			t.Fatalf("import = %+v, want the route dropped under on-error reject", in)
		}
		if transport.calls != 1 {
			t.Fatalf("the plugin member was called %d times, want 1", transport.calls)
		}
	})
}
