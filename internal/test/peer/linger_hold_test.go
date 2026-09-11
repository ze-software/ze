// Design: docs/architecture/testing/ci-format.md -- option=linger
// Related: reject.go -- endSequence and holdSession, the code under test

package peer

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// newHoldTestPeer builds a check-mode peer with no expectations, which is all
// endSequence needs: the sequences are already consumed when it is called.
func newHoldTestPeer(t *testing.T, linger bool, connMap string) *Peer {
	t.Helper()
	peer, err := New(&Config{
		Mode:    ModeCheck,
		Linger:  linger,
		ConnMap: connMap,
		Output:  new(bytes.Buffer),
	})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	return peer
}

// TestEndSequenceHoldsUntilTheRemoteCloses proves the goal of option=linger
// between connections: the peer answers KEEPALIVEs on a connection whose own
// expectations are met, and returns only when the REMOTE closes it.
//
// The method is a socket pair. The far side sends a KEEPALIVE, reads the reply
// that proves the session is being kept alive rather than merely blocked, then
// closes.
//
// VALIDATES: a lingering check peer never ends a session the daemon under test
//
//	is the one expected to end.
//
// PREVENTS: the peer closing first, which ze reads as session-down and answers
//
//	with a dial on its retry timer. The next connection then proves
//	that ze retries and says nothing about the stop and start the
//	apply order owes (test/reload/config-apply-ordering-address-swap.ci).
func TestEndSequenceHoldsUntilTheRemoteCloses(t *testing.T) {
	local, remote := net.Pipe()
	defer local.Close()  //nolint:errcheck // test socket
	defer remote.Close() //nolint:errcheck // test socket

	peer := newHoldTestPeer(t, true, "")
	answered := make(chan error, 1)
	go func() {
		if err := remote.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			answered <- err
			return
		}
		if _, err := remote.Write(KeepaliveMsg()); err != nil {
			answered <- err
			return
		}
		header, _, err := ReadMessage(remote)
		if err != nil {
			answered <- err
			return
		}
		if header[18] != MsgKEEPALIVE {
			answered <- errors.New("the held session answered something other than a KEEPALIVE")
			return
		}
		answered <- remote.Close()
	}()

	result := peer.endSequence(t.Context(), local)
	if err := <-answered; err != nil {
		t.Fatalf("remote side: %v", err)
	}
	if !result.Success {
		t.Fatalf("a session the remote closed must succeed, got %v", result.Error)
	}
}

// TestEndSequenceFailsWhenHeldToTeardown proves the peer reports the shortfall
// rather than reporting success it did not earn: the remote never closed, so
// the connections the file still owed never happened.
//
// VALIDATES: a peer torn down while holding answers what it actually validated.
// PREVENTS: a vacuous pass on a build that never stops the session, which is
//
//	the exact break the swap test's discrimination walk drives.
func TestEndSequenceFailsWhenHeldToTeardown(t *testing.T) {
	local, remote := net.Pipe()
	defer local.Close()  //nolint:errcheck // test socket
	defer remote.Close() //nolint:errcheck // test socket

	peer := newHoldTestPeer(t, true, "")
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	result := peer.endSequence(ctx, local)
	if result.Success {
		t.Fatal("a peer still owed a connection must not report success at teardown")
	}
	if !errors.Is(result.Error, errSessionHeldToTeardown) {
		t.Fatalf("error must name the held session, got %v", result.Error)
	}
}

// TestEndSequenceReturnsAtOnceWithoutLinger proves the default is untouched:
// without option=linger the peer returns and its caller closes the connection,
// which is what every multi-connection test relied on before the hold existed.
//
// VALIDATES: option=linger is what selects the hold.
// PREVENTS: a silent behavior change to the 100-odd .ci files that declare
//
//	several connections and no linger.
func TestEndSequenceReturnsAtOnceWithoutLinger(t *testing.T) {
	cases := map[string]struct {
		linger  bool
		connMap string
	}{
		"no linger":                    {linger: false, connMap: ""},
		"linger on a conn_map batch":   {linger: true, connMap: connMapRemoteIP},
		"no linger on a conn_map peer": {linger: false, connMap: connMapRouterID},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			local, remote := net.Pipe()
			defer local.Close()  //nolint:errcheck // test socket
			defer remote.Close() //nolint:errcheck // test socket

			peer := newHoldTestPeer(t, tc.linger, tc.connMap)
			done := make(chan Result, 1)
			go func() { done <- peer.endSequence(t.Context(), local) }()
			select {
			case result := <-done:
				if !result.Success {
					t.Fatalf("returning at once must succeed, got %v", result.Error)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("endSequence held a connection it must have handed back at once")
			}
		})
	}
}
