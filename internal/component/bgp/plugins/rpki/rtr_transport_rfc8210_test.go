// Design: docs/guide/rpki.md -- the unprotected TCP transport, selected by declared network trust.
// Related: rpki_config.go -- parseRPKIConfig; rpki.go -- startSessions
package rpki

import (
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRTRUnprotectedTCPFromConfig starts the plugin's sessions from a parsed cache-server entry
// and reads what reaches the cache.
//
// VALIDATES: RFC 8210 Section 9, "Caches and routers MUST implement unprotected transport over
// TCP using a port, rpki-rtr (323)", and Section 3, a router "MUST have a trust relationship with,
// and a trusted transport channel to, any cache(s) it uses": the unprotected transport is offered
// on port 323 by default, and only toward a cache the operator declared trusted-network.
// PREVENTS: a config path that never reaches plain TCP, or one that dials an undeclared cache.
func TestRTRUnprotectedTCPFromConfig(t *testing.T) {
	t.Run("a trusted-network cache is served over plain TCP", func(t *testing.T) {
		// RFC requirement: RFC8210-9-1 positive -- a cache-server declared trusted-network with no port parses to rpki-rtr port 323, and the session the plugin starts from that entry speaks RTR over plain TCP: the cache reads a clear-text version 2 Reset Query and the plugin syncs.
		// RFC requirement: RFC6810-7-1 positive -- the same config-driven session implements the unprotected TCP transport, with rpki-rtr port 323 as the parsed default.
		// RFC requirement: RFC8210-3-1 positive -- the unprotected TCP session starts toward a cache the operator declared trusted-network, the trusted channel that transport relies on, and the data it supplies is used.
		cfg, err := parseRPKIConfig(`{"rpki":{"cache-server":{"127.0.0.1":{"trusted-network":"true"}}}}`)
		require.NoError(t, err)
		require.Len(t, cfg.CacheServers, 1)
		assert.Equal(t, uint16(323), cfg.CacheServers[0].Port)

		queries := make(chan []byte, 1)
		port, accepts := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			query, err := readRTRQuery(conn)
			if err != nil {
				return
			}
			queries <- query
			if _, err := conn.Write(slices.Concat(cacheResponsePDU(),
				ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64500), endOfDataPDU())); err != nil {
				return
			}
			_, _ = conn.Read(query[:1])
		})
		// Port 323 is privileged, so the test moves the parsed entry to the listener and keeps
		// every other parsed setting.
		cfg.CacheServers[0].Port = port
		rp := newStatePlugin(t)
		rp.startSessions(cfg)

		waitAccept(t, accepts, "the trusted-network cache was never contacted")
		assert.Equal(t, []byte{rtrVersionMax, pduResetQuery}, (<-queries)[:2], "the cache did not read a clear-text Reset Query")
		requireSynced(t, rp, 1)
		assert.Equal(t, ValidationValid, rp.cache.Validate("192.0.2.0/24", 64500))
	})

	t.Run("a cache with neither TLS nor declared trust is refused", func(t *testing.T) {
		// RFC requirement: RFC8210-3-1 negative -- a cache-server that declares neither TLS nor a trusted network, or declares trusted-network false, is refused when the config is parsed, so no session over an untrusted channel can start.
		for _, server := range []string{`{}`, `{"trusted-network":"false"}`} {
			_, err := parseRPKIConfig(`{"rpki":{"cache-server":{"127.0.0.1":` + server + `}}}`)
			require.Error(t, err, "cache-server %s was accepted", server)
		}
	})
}
