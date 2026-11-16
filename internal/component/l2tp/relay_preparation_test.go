// Design: docs/architecture/l2tp/followup-l2tp-call.md -- accepted Service-Name relay ownership
// Related: relay_preparation.go -- prepareAndSetupPPP; relay_sink.go -- Relay

package l2tp

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/callsink"
)

type relayPreparationForTest struct {
	prepare func() error
}

func (p *relayPreparationForTest) PrepareBridge() error { return p.prepare() }

// A matched Service-Name request carries its preparation through the actual
// sink, call/session construction and reactor event to the worker's attachment
// boundary. Acceptance alone must not release the original local owner.
func TestServiceRelayPreparesBeforeWorkerAttachment(t *testing.T) {
	listener, reactor, _, stop := buildLogReactor(t)
	defer stop()
	peer := newClient(t, listener)
	defer peer.Close()
	remote := clientAddrPort(t, peer)
	insertEstablishedTunnel(t, reactor, 100, 200, remote)
	subsystem := &Subsystem{
		params: Parameters{
			Remotes: []Remote{{Name: "lns1", Address: remote}},
			Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}},
		},
		reactors: []*l2tpReactor{reactor}, logger: slog.Default(),
	}
	var trace []string
	preparation := &relayPreparationForTest{prepare: func() error {
		trace = append(trace, "release-local")
		return nil
	}}
	_, accepted, err := (&relaySink{s: subsystem}).Relay(callsink.Request{
		Service: "wholesale", SubscriberMAC: "02:00:00:00:00:01",
		Channel: relayDescriptorForTest(9), BeforeBridge: preparation, Handoff: &fakeRealmHandoff{},
	})
	require.NoError(t, err)
	require.True(t, accepted)
	require.Empty(t, trace, "acceptance must retain the lease until worker attachment")

	reactor.tunnelsMu.Lock()
	tunnel := reactor.tunnelsByLocalID[100]
	for _, session := range tunnel.sessions {
		session.kernelSetupNeeded = true
	}
	events := reactor.kernelSetupsLocked(tunnel)
	reactor.tunnelsMu.Unlock()
	require.Len(t, events, 1)
	channel, unit, number, err := prepareAndSetupPPP(events[0], 11,
		func(fd int, name string) (int, int, int, error) {
			require.Equal(t, 11, fd)
			require.Empty(t, name)
			trace = append(trace, "attach-worker")
			return 21, 22, 23, nil
		})
	require.NoError(t, err)
	require.Equal(t, []string{"release-local", "attach-worker"}, trace)
	require.Equal(t, 21, channel)
	require.Equal(t, 22, unit)
	require.Equal(t, 23, number)
}

// A service with no relay binding retains its local owner for normal PPP.
// Drive the actual sink rather than an isolated predicate over the service.
func TestServiceRelayUnmatchedKeepsLocalOwner(t *testing.T) {
	called := false
	preparation := &relayPreparationForTest{prepare: func() error {
		called = true
		return nil
	}}
	subsystem := &Subsystem{logger: slog.Default()}
	_, accepted, err := (&relaySink{s: subsystem}).Relay(callsink.Request{
		Service: "retail", BeforeBridge: preparation,
	})
	require.NoError(t, err)
	require.False(t, accepted)
	require.False(t, called, "unmatched local termination must keep its owner")
}

// Failed local-owner release must prevent a new attachment to that transport.
// The setup callback is the exact boundary pppSetupReal uses for DevPPPSetup.
func TestServiceRelayPreparationRefusalPreventsAttachment(t *testing.T) {
	want := errors.New("local PPP lease close failed")
	calls := 0
	preparation := &relayPreparationForTest{prepare: func() error {
		calls++
		return want
	}}
	attached := false
	channel, unit, number, err := prepareAndSetupPPP(kernelSetupEvent{beforeBridge: preparation}, 11,
		func(int, string) (int, int, int, error) {
			attached = true
			return 21, 22, 23, nil
		})
	require.ErrorIs(t, err, want)
	require.Equal(t, 1, calls)
	require.False(t, attached, "failed preparation must not attach a new PPP channel")
	require.Equal(t, -1, channel)
	require.Equal(t, -1, unit)
	require.Equal(t, -1, number)
}

// A non-PPPoE call has no preparation contract and still creates its local PPP
// channel. Optional preparation must not become permission for all attachment.
func TestServiceRelayWithoutPreparationStillAttaches(t *testing.T) {
	attached := false
	_, _, _, err := prepareAndSetupPPP(kernelSetupEvent{}, 11,
		func(int, string) (int, int, int, error) {
			attached = true
			return 21, 22, 23, nil
		})
	require.NoError(t, err)
	require.True(t, attached)
}
