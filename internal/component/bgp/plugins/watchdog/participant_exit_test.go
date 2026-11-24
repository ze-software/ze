// Design: docs/architecture/exabgp-bridge.md -- watchdog forgets given routes on bus:participant-exit.
// Related: given.go -- forgetOwner and handleForget.
// Related: register.go -- watchdogBusReceives declares bus:participant-exit.

package watchdog

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type sentWatchdogCommand struct {
	peer string
	cmd  string
}

// VALIDATES: receiving bus:participant-exit for endpoint "script-a" forgets and
// withdraws every announced watchdog route owned by "script-a" (with
// restore-configured), removes its held routes so later announce does not
// resurrect them, and leaves "script-b"'s given routes untouched.
// PREVENTS: exited participants leaking watchdog-held routes in s.given or
// withdrawing another participant's watchdog routes.
func TestWatchdogParticipantExitForgetsOnlyExitedEndpoint(t *testing.T) {
	var sent []sentWatchdogCommand
	srv := newWatchdogServer(func(peer, cmd string) error {
		sent = append(sent, sentWatchdogCommand{peer: peer, cmd: cmd})
		return nil
	}, nil)

	// script-a gives one announced route and one held (withdraw) route to watchdog "svc".
	status, _, err := srv.handleCommand(commandRequestWatchdogGive, strings.Fields(
		"svc owner script-a selector 192.0.2.1 announce 7 update text announce route 198.51.100.1/32 next-hop 192.0.2.254 withdrawal update text withdraw route 198.51.100.1/32",
	), "*")
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	status, _, err = srv.handleCommand(commandRequestWatchdogGive, strings.Fields(
		"svc owner script-a selector 192.0.2.1 withdraw announce 7 update text announce route 198.51.100.2/32 next-hop 192.0.2.254 withdrawal update text withdraw route 198.51.100.2/32",
	), "*")
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	// script-b gives one announced route to the same watchdog "svc".
	status, _, err = srv.handleCommand(commandRequestWatchdogGive, strings.Fields(
		"svc owner script-b selector 192.0.2.1 announce 7 update text announce route 203.0.113.1/32 next-hop 192.0.2.254 withdrawal update text withdraw route 203.0.113.1/32",
	), "*")
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	require.Len(t, sent, 2, "only the two announced routes should have been sent")
	sent = nil

	// Deliver bus:participant-exit for script-a.
	payload, err := json.Marshal(events.ParticipantExitEvent{
		Type:        events.MessageParticipantExit,
		Endpoint:    "script-a",
		Participant: "/bus/endpoint/script-a",
	})
	require.NoError(t, err)
	srv.handleBusEvent(string(payload))

	// Only script-a's announced route is withdrawn (with restore-configured).
	require.Len(t, sent, 1)
	assert.Equal(t, "192.0.2.1", sent[0].peer)
	assert.Equal(t, "update text restore-configured withdraw route 198.51.100.1/32", sent[0].cmd)
	sent = nil

	// script-a's held route was forgotten too: announcing watchdog "svc" does not
	// send script-a's 198.51.100.2/32.
	status, _, err = srv.handleCommand(commandRequestWatchdogAnnounce, []string{"svc"}, "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	assert.Empty(t, sent, "forgotten held route from script-a must not be announced")

	// script-b's route is still owned by watchdog "svc" and responds to withdraw.
	status, _, err = srv.handleCommand(commandRequestWatchdogWithdraw, []string{"svc"}, "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Len(t, sent, 1)
	assert.Equal(t, "192.0.2.1", sent[0].peer)
	assert.Equal(t, "update text withdraw route 203.0.113.1/32", sent[0].cmd)
}

// VALIDATES: watchdogBusReceives declares bus:participant-exit (R-5 parity for
// both compiled registry.Registration and Stage 1 sdk.Registration) and
// parseParticipantExitEvent accepts both bare and enveloped JSON payloads while
// ignoring unrelated JSON events.
// PREVENTS: watchdog failing to subscribe to participant exits or misparsing
// unrelated JSON bus messages as participant exits.
func TestWatchdogParticipantExitDeclarationAndParsing(t *testing.T) {
	receives := watchdogBusReceives()
	hasExit := slices.ContainsFunc(receives, func(d rpc.MessageReceiveDecl) bool {
		return d.Message == events.MessageParticipantExit
	})
	assert.True(t, hasExit, "watchdogBusReceives must declare %s", events.MessageParticipantExit)

	bare, err := json.Marshal(events.ParticipantExitEvent{
		Type:        events.MessageParticipantExit,
		Endpoint:    "script-x",
		Participant: "/bus/endpoint/script-x",
	})
	require.NoError(t, err)
	endpoint, ok := parseParticipantExitEvent(string(bare))
	assert.True(t, ok)
	assert.Equal(t, "script-x", endpoint)

	enveloped, err := json.Marshal(rpc.EventEnvelope{
		Namespace: events.NamespaceBus,
		Event:     events.EventParticipantExit,
		Payload:   bare,
	})
	require.NoError(t, err)
	endpoint, ok = parseParticipantExitEvent(string(enveloped))
	assert.True(t, ok)
	assert.Equal(t, "script-x", endpoint)

	// Unrelated JSON or BGP text state events must not match.
	_, ok = parseParticipantExitEvent(`{"type":"bgp:state","endpoint":"ignored"}`)
	assert.False(t, ok)
	_, ok = parseParticipantExitEvent("peer 192.0.2.1 remote as 65001 state up\n")
	assert.False(t, ok)
}
