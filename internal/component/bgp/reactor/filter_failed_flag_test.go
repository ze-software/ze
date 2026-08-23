// Tests for PolicyResponse.Failed, the flag that separates a filter's DECISION
// to reject a route from ze rejecting one because the filter never decided.
//
// The distinction is not cosmetic. Failed rides the chain result into
// egressStepResult.failed, and forwardUpdateCore leaves a failed step out of
// suppressedCount, so a fan-out that reached nobody reports a DROP rather than
// errAllDestinationsSuppressed. A plugin timing out under load and a plugin
// deliberately rejecting every route would otherwise read the same, and the
// stored-route replay would count a lost route as a complete replay.
//
// Related: forward_failure_verdict_test.go -- the verdict those two answers pick.

package reactor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// errFilterIPCForTest is what a stand-in transport returns when the test wants
// the filter call itself to fail. The text is the production wording so a log
// read from a failing run says the same thing either way.
var errFilterIPCForTest = errors.New("filter-update: plugin not connected")

// fakeFilterTransport answers the three calls the policy filter's IPC body
// makes. It is the filterTransport seam, which exists because the production
// implementation is a concrete *pluginserver.Server whose filter answer arrives
// off a live plugin socket.
type fakeFilterTransport struct {
	declared []string
	raw      bool
	onError  rpc.OnErrorPolicy
	out      *rpc.FilterUpdateOutput
	err      error
}

func (f *fakeFilterTransport) FilterInfo(_, _ string) ([]string, bool) { return f.declared, f.raw }

func (f *fakeFilterTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy { return f.onError }

func (f *fakeFilterTransport) CallFilterUpdate(_ context.Context, _ string, _ *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	return f.out, f.err
}

// liveFilterServer is the production filter transport: a real plugin server with
// no plugin processes, so CallFilterUpdate cannot find the named plugin and
// FilterOnError answers OnErrorReject for it. That is the daemon's own answer
// for a filter whose plugin died, not a stand-in for one.
func liveFilterServer(t *testing.T) *pluginserver.Server {
	t.Helper()
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, &reactorAPIAdapter{r: &Reactor{}})
	require.NoError(t, err)
	return srv
}

// TestPolicyFilterFailedFlagMarksANonDecision drives the two branches of the IPC
// body that reject a route no filter chose to reject.
//
// VALIDATES: spec-fixit-stored-route-relay-hardening AC-11 -- an IPC error under
// a fail-closed on-error policy, and ze overriding a PolicyModify that touched an
// undeclared attribute, both return PolicyReject WITH Failed set.
// PREVENTS: either flag being dropped in a refactor. Neither branch had a test:
// r.api is a concrete *pluginserver.Server, so both were reachable only from a
// live plugin socket, and a flag nothing can test is a flag nothing protects.
func TestPolicyFilterFailedFlagMarksANonDecision(t *testing.T) {
	const (
		plug   = "policy-plugin"
		filter = "export-policy"
	)

	t.Run("an IPC error under a fail-closed policy", func(t *testing.T) {
		r := &Reactor{api: liveFilterServer(t)}

		got := r.policyFilterFunc(nil)(plug, filter, directionExport, "10.0.0.2", 65002, "origin igp")

		assert.Equal(t, PolicyReject, got.Action, "fail-closed: the route is still rejected")
		assert.True(t, got.Failed, "no filter decided this; the call never reached one")
	})

	// The control for the branch above: the SAME error, with the operator's
	// on-error policy set to accept. The route is then forwarded, so a Failed
	// flag on the reject cannot be an artifact of every error path answering
	// the same way.
	t.Run("an IPC error the operator chose to accept", func(t *testing.T) {
		r := &Reactor{filterTransportSeam: &fakeFilterTransport{
			onError: rpc.OnErrorAccept,
			err:     errFilterIPCForTest,
		}}

		got := r.policyFilterFunc(nil)(plug, filter, directionExport, "10.0.0.2", 65002, "origin igp")

		assert.Equal(t, PolicyAccept, got.Action, "on-error accept forwards the route")
		assert.False(t, got.Failed, "an accepted route carries no rejection to classify")
	})

	t.Run("a modify of an undeclared attribute", func(t *testing.T) {
		r := &Reactor{filterTransportSeam: &fakeFilterTransport{
			declared: []string{"origin"},
			onError:  rpc.OnErrorReject,
			out:      &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Update: "local-preference 200"},
		}}

		got := r.policyFilterFunc(nil)(plug, filter, directionExport, "10.0.0.2", 65002, "origin igp")

		assert.Equal(t, PolicyReject, got.Action,
			"fail-closed: a modify ze will not apply must not forward the route unmodified")
		assert.True(t, got.Failed,
			"the filter decided MODIFY and ze overrode it, so no filter decided to drop this route")
	})

	// The control for the branch above: the same modify, over an attribute the
	// filter DID declare. It is applied, so the override is a property of the
	// declaration and not of every modify.
	t.Run("a modify of a declared attribute", func(t *testing.T) {
		r := &Reactor{filterTransportSeam: &fakeFilterTransport{
			declared: []string{"origin"},
			onError:  rpc.OnErrorReject,
			out:      &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Update: "origin incomplete"},
		}}

		got := r.policyFilterFunc(nil)(plug, filter, directionExport, "10.0.0.2", 65002, "origin igp")

		assert.Equal(t, PolicyModify, got.Action, "a declared attribute is the filter's to change")
		assert.False(t, got.Failed, "nothing was overridden")
		assert.Equal(t, "origin incomplete", got.Delta)
	})
}

// TestEgressChainCarriesTheFailedFlagToTheStep is the consequence half: the flag
// exists to reach forwardUpdateCore, and this is the hop that carries it.
//
// VALIDATES: AC-11 -- a filter that could not run leaves the egress step
// `failed`, while a filter that rejected the route on purpose does not.
// PREVENTS: the two branches above being correct and the chain flattening them
// into one answer, which would put the route's drop back into suppressedCount
// and report a lost route as a policy decision (forward_failure_verdict_test.go).
func TestEgressChainCarriesTheFailedFlagToTheStep(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	wire := wireu.NewWireUpdate(modBufTestPayload(), ctxID)
	refs := []filterapi.FilterRef{{Name: "policy-plugin:export-policy"}}

	t.Run("a filter that could not run", func(t *testing.T) {
		r := &Reactor{api: liveFilterServer(t)}

		got := r.runEgressPolicyChainASN4(refs, "10.0.0.2", 65002, 65000, wire, true)

		assert.False(t, got.accept, "fail-closed: the route is withheld")
		assert.True(t, got.failed, "the step must report that it could not decide")
	})

	t.Run("a filter that rejected the route", func(t *testing.T) {
		r := &Reactor{
			api: liveFilterServer(t),
			filterTransportSeam: &fakeFilterTransport{
				onError: rpc.OnErrorReject,
				out:     &rpc.FilterUpdateOutput{Action: rpc.FilterReject},
			},
		}

		got := r.runEgressPolicyChainASN4(refs, "10.0.0.2", 65002, 65000, wire, true)

		assert.False(t, got.accept, "the filter said no")
		assert.False(t, got.failed, "this IS a policy decision, and the caller may count it as one")
	})
}
