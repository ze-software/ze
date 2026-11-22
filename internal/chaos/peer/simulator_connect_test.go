package peer

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// connectionAnswer is what the fake Ze of these tests does with one accepted
// connection. It MUST close the connection before it returns.
type connectionAnswer func(conn net.Conn)

// closeAtOnce closes the connection before reading or sending anything: what
// Ze does with a connection that finds no peer.
func closeAtOnce(conn net.Conn) {
	_ = conn.Close()
}

// sendZeOpen reads the speaker's OPEN and answers with Ze's.
func sendZeOpen(conn net.Conn) error {
	if _, err := readBGPMessage(conn); err != nil {
		return fmt.Errorf("reading the speaker's OPEN: %w", err)
	}
	open, err := BuildOpen(SessionConfig{
		ASN:      65000,
		RouterID: netip.MustParseAddr("10.0.0.1"),
		HoldTime: 90,
	})
	if err != nil {
		return fmt.Errorf("building Ze's OPEN: %w", err)
	}
	if _, err := conn.Write(serializeMessage(open)); err != nil {
		return fmt.Errorf("sending Ze's OPEN: %w", err)
	}
	return nil
}

// completeHandshake does Ze's whole side of the handshake, then reads until
// the speaker ends the session.
func completeHandshake(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	if err := sendZeOpen(conn); err != nil {
		return
	}
	if _, err := readBGPMessage(conn); err != nil {
		return
	}
	if _, err := conn.Write(serializeMessage(message.NewKeepalive())); err != nil {
		return
	}
	for {
		if _, err := readBGPMessage(conn); err != nil {
			return
		}
	}
}

// openThenClose answers the speaker's OPEN with Ze's and closes: a session Ze
// accepted and then failed.
func openThenClose(conn net.Conn) {
	_ = sendZeOpen(conn)
	_ = conn.Close()
}

// startOfOpenThenClose sends the first bytes of a BGP header and closes: the
// connection ends inside Ze's OPEN, not before it.
func startOfOpenThenClose(conn net.Conn) {
	if _, err := readBGPMessage(conn); err == nil {
		_, _ = conn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	}
	_ = conn.Close()
}

// notificationThenClose refuses the session the way Ze refuses one it knows it
// cannot serve: a NOTIFICATION, then the close.
func notificationThenClose(conn net.Conn) {
	if _, err := readBGPMessage(conn); err == nil {
		_, _ = conn.Write(serializeMessage(BuildCeaseNotification()))
	}
	_ = conn.Close()
}

// openThenNotificationThenClose answers OPEN with OPEN and then refuses the
// colliding session with a NOTIFICATION instead of KEEPALIVE (RFC 4271 §8.2.2).
func openThenNotificationThenClose(conn net.Conn) {
	if err := sendZeOpen(conn); err == nil {
		_, _ = conn.Write(serializeMessage(BuildCeaseNotification()))
	}
	_ = conn.Close()
}

// fakeZe is a listener whose i-th connection gets the i-th answer; a
// connection past the last answer gets the last one again.
type fakeZe struct {
	addr     string
	accepted atomic.Int32
}

// startFakeZe listens on a loopback port and answers connections one at a
// time, until the test ends.
func startFakeZe(t *testing.T, answers ...connectionAnswer) *fakeZe {
	t.Helper()
	require.NotEmpty(t, answers)

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ze := &fakeZe{addr: ln.Addr().String()}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			index := min(int(ze.accepted.Add(1))-1, len(answers)-1)
			answers[index](conn)
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-done
	})
	return ze
}

// connectProfile is the speaker every test of this file runs.
func connectProfile() SimProfile {
	return SimProfile{
		Index:    0,
		ASN:      65001,
		RouterID: netip.MustParseAddr("10.255.0.1"),
		HoldTime: 90,
	}
}

// runToOutcome runs one simulator until it reports a session or an error, and
// answers the events up to and including that one. The bound is above the
// handshake deadline, so a run that ends on the deadline is seen.
func runToOutcome(t *testing.T, cfg SimulatorConfig) []Event {
	t.Helper()

	events := make(chan Event, 100)
	cfg.Events = events
	cfg.Quiet = true
	ctx, cancel := context.WithTimeout(context.Background(), handshakeDeadline+20*time.Second)
	defer cancel()

	go func() {
		RunSimulator(ctx, cfg)
		close(events)
	}()

	var seen []Event
	decided := false
	for ev := range events {
		if decided {
			continue
		}
		seen = append(seen, ev)
		if ev.Type == EventEstablished {
			decided = true
		}
		if ev.Type == EventError {
			decided = true
		}
		if decided {
			cancel()
		}
	}
	require.True(t, decided, "the simulator reported neither a session nor an error: %v", seen)
	return seen
}

// retriesOf answers the connection retries among events, in order.
func retriesOf(events []Event) []Event {
	var retries []Event
	for index := range events {
		if events[index].Type == EventReconnecting {
			retries = append(retries, events[index])
		}
	}
	return retries
}

// TestSimulatorRetriesAConnectionClosedBeforeTheOpen: Ze closes the first two
// connections before sending anything, then completes the handshake. The
// simulator establishes on the third connection and reports two retries, each
// with its attempt number and its cause.
//
// VALIDATES: a connection closed before the peer's OPEN is a failed attempt
// that is tried again, and each retry is visible as an event.
// PREVENTS: the simulator ending the run on "reading OPEN: reading header: EOF"
// when Ze replaces a peer while the speaker connects.
func TestSimulatorRetriesAConnectionClosedBeforeTheOpen(t *testing.T) {
	// The repeated answer is the scenario: two connections closed, then one kept.
	answers := []connectionAnswer{closeAtOnce, closeAtOnce, completeHandshake}
	ze := startFakeZe(t, answers...)

	events := runToOutcome(t, SimulatorConfig{Profile: connectProfile(), Seed: 42, Addr: ze.addr})

	last := events[len(events)-1]
	require.Equal(t, EventEstablished, last.Type, "outcome: %v", last.Err)
	retries := retriesOf(events)
	require.Len(t, retries, 2)
	for index, retry := range retries {
		assert.Equal(t, index+1, retry.Count, "the attempt that failed")
		assert.ErrorIs(t, retry.Err, errClosedBeforeOpen)
	}
	assert.Equal(t, int32(3), ze.accepted.Load(), "connections Ze accepted")
}

// TestSimulatorGivesUpWhenEveryConnectionIsClosed: Ze closes every connection
// before sending anything. The simulator opens connectAttemptsMax connections,
// reports a retry before each one after the first, then fails with one error
// naming the address, the number of attempts and the last cause.
//
// VALIDATES: the retry is bounded and its failure says what was tried.
// PREVENTS: a simulator that retries forever against a Ze that refuses it.
func TestSimulatorGivesUpWhenEveryConnectionIsClosed(t *testing.T) {
	ze := startFakeZe(t, closeAtOnce)

	events := runToOutcome(t, SimulatorConfig{Profile: connectProfile(), Seed: 42, Addr: ze.addr})

	last := events[len(events)-1]
	require.Equal(t, EventError, last.Type)
	require.ErrorIs(t, last.Err, errClosedBeforeOpen)
	text := last.Err.Error()
	assert.Contains(t, text, "connecting to "+ze.addr)
	assert.Contains(t, text, fmt.Sprintf("gave up after %d attempts", connectAttemptsMax))
	assert.NotContains(t, text, "\n", "the error is one line")
	assert.Len(t, retriesOf(events), connectAttemptsMax-1)
	assert.Equal(t, int32(connectAttemptsMax), ze.accepted.Load(), "connections Ze accepted")
}

// TestSimulatorGivesUpWhenEveryConnectionIsRefused: nothing listens on the
// address. A refused connection is retried within the same bound, and the
// error names the address and the attempts.
//
// VALIDATES: a refused connect counts as a failed attempt, like an early close.
// PREVENTS: a second, unbounded path for the refused case.
func TestSimulatorGivesUpWhenEveryConnectionIsRefused(t *testing.T) {
	// A port that was listening a moment ago and no longer is.
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	events := runToOutcome(t, SimulatorConfig{Profile: connectProfile(), Seed: 42, Addr: addr})

	last := events[len(events)-1]
	require.Equal(t, EventError, last.Type)
	require.ErrorIs(t, last.Err, errClosedBeforeOpen)
	assert.Contains(t, last.Err.Error(), "connecting to "+addr)
	assert.Contains(t, last.Err.Error(), fmt.Sprintf("gave up after %d attempts", connectAttemptsMax))
	assert.Len(t, retriesOf(events), connectAttemptsMax-1)
}

// TestSimulatorDoesNotRetryAPeerThatNeverAnswers: Ze reads the speaker's OPEN
// and sends nothing. The simulator fails when the handshake deadline passes,
// on its first connection. This test takes the whole handshake deadline.
//
// VALIDATES: a handshake timeout stays fatal.
// PREVENTS: the retry hiding a Ze that holds a connection and never answers,
// which is how the defect fixed in the reactor showed.
func TestSimulatorDoesNotRetryAPeerThatNeverAnswers(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	ze := startFakeZe(t, func(conn net.Conn) {
		_, _ = readBGPMessage(conn)
		<-release
		_ = conn.Close()
	})
	// Registered after startFakeZe, so it runs before the listener's cleanup
	// waits for this answer to return.
	t.Cleanup(func() { close(release) })

	events := runToOutcome(t, SimulatorConfig{Profile: connectProfile(), Seed: 42, Addr: ze.addr})

	last := events[len(events)-1]
	require.Equal(t, EventError, last.Type)
	require.ErrorIs(t, last.Err, os.ErrDeadlineExceeded)
	assert.NotErrorIs(t, last.Err, errClosedBeforeOpen)
	assert.Empty(t, retriesOf(events))
	assert.Equal(t, int32(1), ze.accepted.Load(), "connections Ze accepted")
}

// TestSimulatorDoesNotRetryOnceThePeerAnswered: three ways Ze ends a
// connection after it began to answer. Each one ends the run on the first
// connection.
//
// VALIDATES: only a close before the first byte of the peer's OPEN is retried.
// PREVENTS: the retry hiding a session Ze accepted and then failed, or one it
// refused with a NOTIFICATION.
func TestSimulatorDoesNotRetryOnceThePeerAnswered(t *testing.T) {
	tests := []struct {
		name   string
		answer connectionAnswer
	}{
		{"an OPEN then a close", openThenClose},
		{"the start of an OPEN then a close", startOfOpenThenClose},
		{"a NOTIFICATION then a close", notificationThenClose},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ze := startFakeZe(t, tt.answer)

			events := runToOutcome(t, SimulatorConfig{Profile: connectProfile(), Seed: 42, Addr: ze.addr})

			last := events[len(events)-1]
			require.Equal(t, EventError, last.Type)
			assert.NotErrorIs(t, last.Err, errClosedBeforeOpen)
			assert.Empty(t, retriesOf(events))
			assert.Equal(t, int32(1), ze.accepted.Load(), "connections Ze accepted")
		})
	}
}

// TestSimulatorRetryOfACallerConnection: a caller that hands the simulator a
// connection gets a retry only when it also names the address to dial.
//
// VALIDATES: with Conn and Addr, the retry dials Addr; with Conn alone, an
// early close ends the run, because there is nothing to dial.
// PREVENTS: a retry that dials an empty address, and a caller connection that
// silently gets no retry although it named the address.
func TestSimulatorRetryOfACallerConnection(t *testing.T) {
	dial := func(t *testing.T, addr string) net.Conn {
		t.Helper()
		conn, err := (&net.Dialer{}).DialContext(context.Background(), "tcp", addr)
		require.NoError(t, err)
		return conn
	}

	t.Run("with the address", func(t *testing.T) {
		ze := startFakeZe(t, closeAtOnce, completeHandshake)

		events := runToOutcome(t, SimulatorConfig{
			Profile: connectProfile(), Seed: 42, Conn: dial(t, ze.addr), Addr: ze.addr,
		})

		last := events[len(events)-1]
		require.Equal(t, EventEstablished, last.Type, "outcome: %v", last.Err)
		assert.Len(t, retriesOf(events), 1)
		assert.Equal(t, int32(2), ze.accepted.Load(), "connections Ze accepted")
	})

	t.Run("without the address", func(t *testing.T) {
		ze := startFakeZe(t, closeAtOnce, completeHandshake)

		events := runToOutcome(t, SimulatorConfig{
			Profile: connectProfile(), Seed: 42, Conn: dial(t, ze.addr),
		})

		last := events[len(events)-1]
		require.Equal(t, EventError, last.Type)
		assert.ErrorIs(t, last.Err, errClosedBeforeOpen)
		assert.Empty(t, retriesOf(events))
		assert.Equal(t, int32(1), ze.accepted.Load(), "connections Ze accepted")
	})
}

// TestSimulatorRefusesOpenThenNotification verifies that when a peer answers
// OPEN with OPEN and then rejects the session with a NOTIFICATION instead of a
// KEEPALIVE (e.g., RFC 4271 §6.8 / §8.2.2 collision resolution), the simulator
// treats the handshake as failed rather than emitting EventEstablished.
func TestSimulatorRefusesOpenThenNotification(t *testing.T) {
	ze := startFakeZe(t, openThenNotificationThenClose)

	events := runToOutcome(t, SimulatorConfig{Profile: connectProfile(), Seed: 42, Addr: ze.addr})

	last := events[len(events)-1]
	require.Equal(t, EventError, last.Type)
	assert.Contains(t, last.Err.Error(), "NOTIFICATION")
	assert.Empty(t, retriesOf(events))
	assert.Equal(t, int32(1), ze.accepted.Load(), "connections Ze accepted")
}
