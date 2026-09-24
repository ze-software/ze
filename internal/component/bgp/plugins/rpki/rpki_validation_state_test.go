// RFC: rfc/short/rfc8210.md — Section 10, the router holds the data it loaded across a loss
// Related: rpki.go — statusCommand, appendSummaryFields, validationEnabled
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

// TestValidationStateIsMeasuredNotAsserted drives the two RPKI status fields an operator
// reads to answer "are my routes being validated right now", over the four conditions the
// daemon can be in, through the command entry point the CLI calls.
//
// VALIDATES: `running` reads the active flag that gates every per-prefix validation, and
// `validation-enabled` reads whether a cache server has delivered a set to validate against.
// PREVENTS: the two fields going back to the literal true they carried until 2026-09-20,
// under which a daemon with no cache server configured, and a daemon whose only cache server
// refuses the connection, both answered `show bgp rpki summary` with validation enabled while
// no route reached an RPKI verdict.
func TestValidationStateIsMeasuredNotAsserted(t *testing.T) {
	t.Run("no cache server is configured", func(t *testing.T) {
		rp := newStatePlugin(t)
		rp.startSessions(&rpkiConfig{})

		status := commandJSON(t, rp, "show bgp rpki status")
		assert.Equal(t, false, status["running"],
			"nothing is running: the config names no cache server, so the adj-rib-in validation gate was never enabled")

		summary := commandJSON(t, rp, "show bgp rpki summary")
		assert.Equal(t, false, summary["validation-enabled"])
		assert.EqualValues(t, 0, summary["sessions-total"])
	})

	t.Run("a configured cache server that refuses the connection", func(t *testing.T) {
		rp := newStatePlugin(t)
		rp.startSessions(oneCache(refusedPort(t), 100))

		status := commandJSON(t, rp, "show bgp rpki status")
		assert.Equal(t, true, status["running"],
			"the config names a cache server, so the validation gate is on and every prefix is looked up")
		assert.Equal(t, false, status["synced"])

		summary := commandJSON(t, rp, "show bgp rpki summary")
		assert.Equal(t, false, summary["validation-enabled"],
			"no cache has delivered a set, so every prefix reads not-found for want of data rather than for want of a ROA")
		assert.EqualValues(t, 1, summary["sessions-total"],
			"the false above is the cache server's silence, not an empty server list")
		assert.EqualValues(t, 0, summary["sessions-synced"])
	})

	t.Run("a cache server that delivered an empty set", func(t *testing.T) {
		rp := newStatePlugin(t)
		port, accepts := serveRTR(t, replyEmptySync(t))
		rp.startSessions(oneCache(port, 100))

		waitAccept(t, accepts, "the group never opened a connection to the configured cache server")
		requireSynced(t, rp, 1)

		status := commandJSON(t, rp, "show bgp rpki status")
		assert.Equal(t, true, status["running"])
		assert.Equal(t, true, status["synced"])

		summary := commandJSON(t, rp, "show bgp rpki summary")
		assert.Equal(t, true, summary["validation-enabled"],
			"the cache answered, so every route reaches a verdict; that the set is empty is the verdict, not the absence of one")
		assert.EqualValues(t, 0, summary["vrp-count"])

		// The fifth condition an operator meets: the cache is gone and ze keeps the set it
		// loaded. RFC 8210 Section 10 has the router retain that data, and the session
		// carries exactly this state between polls, connection closed and set held.
		assert.Equal(t, sessionIdle, rp.snapshots()[0].State)
		assert.Equal(t, true, rp.validationEnabled(),
			"a cache lost after it delivered leaves every route still validated against the set it delivered")
	})
}

// Removing the last server must publish the new policy as well as stop the
// transport; otherwise status continues reporting the previous ASPA switch.
func TestInactiveConfigReplacesReportedASPAPolicy(t *testing.T) {
	rp := newStatePlugin(t)
	rp.startSessions(&rpkiConfig{ASPAValidation: true})
	assert.Equal(t, true, commandJSON(t, rp, "show bgp rpki status")["aspa-enabled"])
	rp.startSessions(&rpkiConfig{})
	status := commandJSON(t, rp, "show bgp rpki status")
	assert.Equal(t, false, status["running"])
	assert.Equal(t, false, status["aspa-enabled"])
}

// newStatePlugin builds a plugin whose trackers are real, because the End of Data of a
// completed sync calls back into them.
func newStatePlugin(t *testing.T) *rPKIPlugin {
	t.Helper()
	rp := newTestPlugin()
	rp.originTracker = newOriginTracker()
	rp.aspaTracker = newASPATracker()
	t.Cleanup(func() {
		rp.stopSessions()
		if rp.dataLease != nil {
			rp.dataLease.stop()
		}
	})
	return rp
}

// commandJSON drives the plugin's real command entry point, the one the CLI dispatcher
// calls, and returns the decoded answer.
func commandJSON(t *testing.T, rp *rPKIPlugin, command string) map[string]any {
	t.Helper()
	status, data, err := rp.handleCommand(command, nil)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	return parseJSON(t, data)
}

// oneCache is the parsed config of a single cache server on the loopback.
func oneCache(port uint16, preference uint8) *rpkiConfig {
	return &rpkiConfig{
		CacheServers: []cacheServerConfig{{Address: "127.0.0.1", Port: port, Preference: preference}},
	}
}

// refusedPort returns a port nothing listens on, so a connection to it is refused at once
// rather than waiting out the dial timeout.
func refusedPort(t *testing.T) uint16 {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address is a TCP address")
	require.NoError(t, ln.Close())

	return uint16(addr.Port) //nolint:gosec // listener port fits uint16
}

// replyEmptySync answers a query with a Cache Response and an End of Data carrying no
// prefixes: the cache server is reachable and its VRP set is empty. The refresh interval it
// sends is an hour, so the group polls once inside a test.
func replyEmptySync(t *testing.T) func(net.Conn) {
	t.Helper()
	return func(conn net.Conn) {
		query := make([]byte, pduResetQueryLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			return
		}

		resp := make([]byte, pduHeaderLen)
		resp[0], resp[1] = query[0], pduCacheResp
		binary.BigEndian.PutUint32(resp[4:8], pduHeaderLen)

		eod := make([]byte, pduEndOfDataLen)
		eod[0], eod[1] = query[0], pduEndOfData
		binary.BigEndian.PutUint32(eod[4:8], pduEndOfDataLen)
		binary.BigEndian.PutUint32(eod[12:16], 3600) // refresh interval, seconds
		binary.BigEndian.PutUint32(eod[16:20], 600)  // retry interval, seconds
		binary.BigEndian.PutUint32(eod[20:24], 7200) // expire interval, seconds

		if _, err := conn.Write(append(resp, eod...)); err != nil {
			t.Logf("cache write failed: %v", err)
		}
	}
}

// requireSynced waits until want cache servers have completed a sync.
func requireSynced(t *testing.T, rp *rPKIPlugin, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return syncedSessions(rp.snapshots()) == want
	}, 10*time.Second, 10*time.Millisecond, "%d cache server(s) never completed a sync", want)
}
