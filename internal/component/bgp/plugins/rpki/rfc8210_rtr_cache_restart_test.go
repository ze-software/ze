package rpki

import (
	"context"
	"encoding/binary"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCacheGroupResyncsAfterSingleCacheRestart runs the cache group against one cache that
// goes away and comes back on the same address, the way the RPKI terminal demo stops and
// restarts its RTR cache. The cache advertises Refresh Interval 2 and Retry Interval 1.
//
// VALIDATES: RFC 8210 Section 6, the Retry Interval "tells the router how long to wait before
// retrying a failed Serial Query or Reset Query": once the cache listens again, the router's
// next query reaches it within about one Retry Interval and completes a sync.
// PREVENTS: a lone cache that went away and returned staying unsynced past its advertised Retry
// Interval.
func TestCacheGroupResyncsAfterSingleCacheRestart(t *testing.T) {
	cache := newRestartableRTRCache(t)
	roas, aspas := newROACache(), newASPACache()
	stop := make(chan struct{})
	session := newTestRTRSession(t, "127.0.0.1", cache.port, 100, "", roas, aspas, stop)
	group := newCacheGroup([]*RTRSession{session}, stop)
	done := make(chan struct{})
	go func() { defer close(done); group.Run() }()
	t.Cleanup(func() {
		close(stop)
		<-done
	})

	require.Eventually(t, func() bool { return session.Snapshot().Synced }, 5*time.Second,
		10*time.Millisecond, "the first sync did not complete")
	require.Equal(t, time.Second, session.Snapshot().RetryInterval)

	cache.stop()
	require.Eventually(t, func() bool { return !group.retryAt[0].IsZero() }, 5*time.Second,
		10*time.Millisecond, "no query failed while the cache was down")
	// Keep the cache down for a few Retry Intervals, as the demo does.
	time.Sleep(2500 * time.Millisecond)

	answered := cache.answers()
	restarted := time.Now()
	cache.start(t)
	require.Eventually(t, func() bool { return cache.answers() > answered }, 4*time.Second,
		10*time.Millisecond, "the router did not complete a sync with the restarted cache")
	require.Less(t, time.Since(restarted), 3*time.Second,
		"the resync came later than the one-second Retry Interval allows")
}

// restartableRTRCache is an RTR cache on a fixed loopback port that can stop listening and
// start again. It answers every query with a full load, the way `le mock rpki` does: PDUs of
// version 1, refresh 2, retry 1, and it keeps the connection open after End of Data.
type restartableRTRCache struct {
	port uint16

	mu       sync.Mutex
	ln       net.Listener
	answered int
}

func newRestartableRTRCache(t *testing.T) *restartableRTRCache {
	t.Helper()
	c := &restartableRTRCache{}
	c.start(t)
	t.Cleanup(c.stop)
	return c
}

func (c *restartableRTRCache) start(t *testing.T) {
	t.Helper()
	var lc net.ListenConfig
	address := "127.0.0.1:0"
	if c.port != 0 {
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(int(c.port)))
	}
	ln, err := lc.Listen(context.Background(), "tcp", address)
	require.NoError(t, err)
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address is a TCP address")
	c.mu.Lock()
	c.ln = ln
	c.port = uint16(tcpAddr.Port) //nolint:gosec // listener port fits uint16
	c.mu.Unlock()
	go c.accept(ln)
}

func (c *restartableRTRCache) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ln != nil {
		_ = c.ln.Close() //nolint:errcheck // test listener teardown
		c.ln = nil
	}
}

func (c *restartableRTRCache) answers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.answered
}

func (c *restartableRTRCache) accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		c.serve(conn)
	}
}

func (c *restartableRTRCache) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }() //nolint:errcheck // test connection teardown
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return
	}
	if _, err := readRTRQuery(conn); err != nil {
		return
	}
	load := slices.Concat(cacheResponsePDU(), ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64501),
		endOfDataTimed(1, 2, 1))
	for i := 0; i < len(load); {
		load[i] = rtrVersionMin
		i += int(binary.BigEndian.Uint32(load[i+4 : i+8]))
	}
	if _, err := conn.Write(load); err != nil {
		return
	}
	c.mu.Lock()
	c.answered++
	c.mu.Unlock()
	buf := make([]byte, 1)
	_, _ = conn.Read(buf) //nolint:errcheck // wait for the router to close
}
