// Design: docs/architecture/bgp/structural-forwarding.md -- receive validation and cache changes
// Related: forward_validation.go -- validationSelection.section
// Related: reactor_api_relay.go -- buildRelayUpdate, buildRelayWithdrawal
//
// VALIDATES: a withdrawal of a route still pending receive validation goes
// through the validation engine: never announced, it sends nothing; announced,
// it is written after any revalidation announce queued before it.
// PREVENTS: a withdrawal sent to a destination never advertised the route, and
// a live withdrawal overtaken by a queued revalidation announce, which left the
// destination holding a route its source had withdrawn.
package reactor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// pendingWithdrawalRails holds the two rails a validated route reaches a
// destination on: the live forward of a received UPDATE (ForwardUpdate) and
// the stored-route relay a ValidationChange triggers (RelayStoredRoute). The
// eligibility lookup stands in for Adj-RIB-In: eligible is what its gate
// answers at the moment each rail asks.
type pendingWithdrawalRails struct {
	f        *aigpReplayFixture
	api      *reactorAPIAdapter
	route    rpc.StoredRoute
	prefix   []byte
	eligible *bool
	seen     int
}

func newPendingWithdrawalRails(t *testing.T) *pendingWithdrawalRails {
	t.Helper()
	eligible := new(bool)
	ribevents.RegisterValidationLookup(func(ribevents.ValidationRoute, uint64) bool { return *eligible }, nil)
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	route := storedIPv4Route(f.source.Settings().Address.String())
	route.NLRIHex = "180A1400"
	return &pendingWithdrawalRails{f: f, api: &reactorAPIAdapter{r: f.r}, route: route,
		prefix: []byte{24, 10, 20, 0}, eligible: eligible}
}

// announce is the live forward of the source's announcement.
func (p *pendingWithdrawalRails) announce(t *testing.T) {
	t.Helper()
	p.f.forward(t, p.f.receive(t, p.f.body(t, 100)))
}

// withdraw is the live forward of the source's withdrawal of the prefix. A
// withdrawal the receive gate leaves to the removal relay dispatches nothing,
// which ForwardUpdate reports as errAllDestinationsSuppressed.
func (p *pendingWithdrawalRails) withdraw(t *testing.T) {
	t.Helper()
	sel, err := selector.Parse(p.f.destination.Settings().Address.String())
	require.NoError(t, err)
	id := p.f.receive(t, makeUpdateBody(p.prefix, nil, nil))
	err = p.api.ForwardUpdate(sel, id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder"))
	if !errors.Is(err, errAllDestinationsSuppressed) {
		require.NoError(t, err)
	}
}

// relay is the ValidationChange relay: Adj-RIB-In's answer for the key is the
// stored route, or an explicit withdrawal once the key was removed.
func (p *pendingWithdrawalRails) relay(t *testing.T, removed bool) {
	t.Helper()
	route := p.route
	route.Withdraw = removed
	require.NoError(t, p.api.RelayStoredRoute(p.f.destination.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()))
}

func (p *pendingWithdrawalRails) counts(t *testing.T) (announced, withdrawn int) {
	t.Helper()
	return validationSocketCounts(t, p.f, p.prefix, &p.seen)
}

// TestPendingRouteWithdrawnBeforeVerdictSendsNothing proves that a withdrawal
// of a route still pending receive validation cancels it without output.
// Method: the route arrives pending and is withheld; the source withdraws it
// before any verdict; Adj-RIB-In's removal relays an explicit withdrawal; a
// later Valid verdict finds no route, so its relay answers the withdrawal again.
// The destination was never advertised the route, so it is sent nothing at all
// (RFC 4271 Section 3.1: only "a previously advertised route" is withdrawn).
func TestPendingRouteWithdrawnBeforeVerdictSendsNothing(t *testing.T) {
	p := newPendingWithdrawalRails(t)

	p.announce(t)
	p.withdraw(t)
	p.relay(t, true)
	// The Valid verdict for the canceled generation installs nothing, so the
	// key's ValidationChange relay still answers its withdrawal.
	p.relay(t, true)

	announced, withdrawn := p.counts(t)
	require.Zero(t, announced, "a route withdrawn while pending must never be announced")
	require.Zero(t, withdrawn, "a route never announced to the destination must not be withdrawn from it")

	// Positive fence: the zeros above are only evidence if this rail writes to
	// this destination at all. A stored route that Adj-RIB-In answers as
	// eligible, relayed on the same rail for the same key, is announced once.
	*p.eligible = true
	p.relay(t, false)
	announced, withdrawn = p.counts(t)
	require.Equal(t, 1, announced, "the relay rail announces an eligible stored route to this destination")
	require.Zero(t, withdrawn)
}

// TestAnnouncedRouteWithdrawalFollowsQueuedRevalidation proves that the
// withdrawal of an announced route cannot be overtaken by a revalidation
// announce queued before it. Method: the route is announced eligible. The
// source then withdraws it while Adj-RIB-In still holds the route as eligible,
// so the live forward must not carry that withdrawal: a revalidation relay
// that read the stored route before the removal is still to be written. After
// that relay, Adj-RIB-In's removal relays the explicit withdrawal. The
// destination ends withdrawn, withdrawn exactly once and after the announce.
func TestAnnouncedRouteWithdrawalFollowsQueuedRevalidation(t *testing.T) {
	p := newPendingWithdrawalRails(t)
	*p.eligible = true

	p.announce(t)
	announced, withdrawn := p.counts(t)
	require.Equal(t, 1, announced, "the eligible route is announced")
	require.Zero(t, withdrawn)

	// RFC 4271 Section 3.1: the source withdraws the route. Adj-RIB-In has
	// not yet removed it, so the withdrawal is the removal relay's to send.
	p.withdraw(t)
	_, withdrawn = p.counts(t)
	require.Zero(t, withdrawn, "the live forward must not send a withdrawal a queued revalidation announce can overtake")

	// The revalidation relay queued before the removal.
	p.relay(t, false)
	// Adj-RIB-In removes the route and its ValidationChange relays the
	// explicit withdrawal.
	*p.eligible = false
	p.relay(t, true)

	_, withdrawn = p.counts(t)
	require.Equal(t, 1, withdrawn, "the removal withdraws the route exactly once, after any queued announce")
	bodies := aigpSocketBodies(t, p.f.conn)
	last, err := message.UnpackUpdate(bodies[len(bodies)-1])
	require.NoError(t, err)
	require.Equal(t, p.prefix, last.WithdrawnRoutes, "the destination ends withdrawn")
	require.Empty(t, last.NLRI)
}
