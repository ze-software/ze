package rpki

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunPollsOnRefreshAfterSuccessRetryAfterFailure verifies which interval the Run loop
// actually waits, driven end to end against a listener that speaks RTR.
//
// VALIDATES: RFC 8210 Section 6 -- the Refresh Interval times the next poll after an End of Data
// completed the sync, the Retry Interval times it after the query failed. The cache in the first
// case sends refresh 1 and retry 7200, so a router that re-polls at all inside the test window
// is using the refresh value. The second case inverts the pair.
// PREVENTS: The parsed Refresh Interval being stored and never used, which made ze poll a cache
// sending refresh 3600 and retry 600 once every 600 seconds, six times more often than asked.
func TestRunPollsOnRefreshAfterSuccessRetryAfterFailure(t *testing.T) {
	t.Run("a completed sync waits the refresh interval", func(t *testing.T) {
		// The cache answers every query with a Cache Response and an End of Data carrying
		// refresh 1 and retry 7200. Only the refresh value can produce a second query here.
		port, accepts := serveRTR(t, func(conn net.Conn) {
			query := make([]byte, pduResetQueryLen)
			if _, err := io.ReadFull(conn, query); err != nil {
				return
			}
			resp := make([]byte, pduHeaderLen)
			resp[1] = pduCacheResp
			binary.BigEndian.PutUint32(resp[4:8], pduHeaderLen)
			eod := make([]byte, pduEndOfDataLen)
			eod[1] = pduEndOfData
			binary.BigEndian.PutUint32(eod[4:8], pduEndOfDataLen)
			binary.BigEndian.PutUint32(eod[12:16], 1)    // refresh interval, seconds
			binary.BigEndian.PutUint32(eod[16:20], 7200) // retry interval, seconds
			binary.BigEndian.PutUint32(eod[20:24], 7200) // expire interval, seconds
			if _, err := conn.Write(append(resp, eod...)); err != nil {
				t.Logf("cache write failed: %v", err)
			}
		})

		stopCh := make(chan struct{})
		s := newRTRSession("127.0.0.1", port, 100, "", newROACache(), newASPACache(), stopCh)
		done := make(chan struct{})
		go func() { defer close(done); s.Run() }()

		first := waitAccept(t, accepts, "the session never opened its first connection")
		second := waitAccept(t, accepts,
			"no second query: the router waited the retry interval after a completed sync")
		assert.Less(t, second.Sub(first), 30*time.Second,
			"the second query follows the 1s refresh interval, not the 7200s retry interval")

		close(stopCh)
		<-done
		assert.Equal(t, time.Second, s.pollDelay(true), "the refresh interval times a completed sync")
		assert.Equal(t, 7200*time.Second, s.pollDelay(false), "the retry interval times a failure")
	})

	t.Run("a failed query waits the retry interval", func(t *testing.T) {
		// The cache closes every connection without answering, so every query fails. The
		// session's refresh interval is an hour, so only the retry value can re-poll here.
		port, accepts := serveRTR(t, func(net.Conn) {})

		stopCh := make(chan struct{})
		s := newRTRSession("127.0.0.1", port, 100, "", newROACache(), newASPACache(), stopCh)
		s.refreshInterval = time.Hour
		s.retryInterval = 200 * time.Millisecond
		done := make(chan struct{})
		go func() { defer close(done); s.Run() }()

		first := waitAccept(t, accepts, "the session never opened its first connection")
		second := waitAccept(t, accepts,
			"no second query: the router waited the refresh interval after a failed query")
		assert.Less(t, second.Sub(first), 30*time.Second,
			"the retry follows the 200ms retry interval, not the one-hour refresh interval")

		close(stopCh)
		<-done
	})
}

// serveRTR accepts connections on a throwaway listener, hands each one to reply, and reports
// the time of every accept. The accept times are what the caller measures: each one is a query
// the router decided to send.
func serveRTR(t *testing.T, reply func(net.Conn)) (uint16, <-chan time.Time) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := ln.Close(); err != nil {
			t.Logf("listener close failed: %v", err)
		}
	})

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address is a TCP address")

	accepts := make(chan time.Time, 8)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			select {
			case accepts <- time.Now():
			default:
			}
			reply(conn)
			if err := conn.Close(); err != nil {
				t.Logf("cache close failed: %v", err)
			}
		}
	}()
	return uint16(tcpAddr.Port), accepts //nolint:gosec // listener port fits uint16
}

// waitAccept waits for the next connection the router opens, or fails the test with why.
func waitAccept(t *testing.T, accepts <-chan time.Time, why string) time.Time {
	t.Helper()
	select {
	case at := <-accepts:
		return at
	case <-time.After(10 * time.Second):
		t.Fatal(why)
		return time.Time{}
	}
}
