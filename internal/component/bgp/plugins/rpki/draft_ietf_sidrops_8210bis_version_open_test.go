// Design: docs/guide/rpki.md -- the RTR client's version negotiation, driven over a real transport.
// Related: rtr_session.go -- newRTRSession, connectAndSync, handlePDU; rtr_pdu.go -- rtrVersionMax
package rpki

import (
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveRTRVersions runs a cache that records the Protocol Version and PDU type of every query it
// reads, one connection at a time, and answers the n-th connection with replies[n]. A connection
// past the last reply is closed unanswered.
func serveRTRVersions(t *testing.T, replies ...[]byte) (uint16, <-chan [2]byte) {
	t.Helper()
	queries := make(chan [2]byte, len(replies)+4)
	attempt := 0
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return
		}
		query, err := readRTRQuery(conn)
		if err != nil {
			return
		}
		queries <- [2]byte{query[0], query[1]}
		if attempt >= len(replies) {
			return
		}
		reply := replies[attempt]
		attempt++
		if _, err := conn.Write(reply); err != nil {
			return
		}
		_, _ = conn.Read(query[:1])
	})
	return port, queries
}

// takeQueryVersions reads count recorded queries, failing the test when the router sent fewer.
func takeQueryVersions(t *testing.T, queries <-chan [2]byte, count int) [][2]byte {
	t.Helper()
	got := make([][2]byte, 0, count)
	for range count {
		select {
		case query := <-queries:
			got = append(got, query)
		case <-time.After(6 * time.Second):
			t.Fatalf("the router sent %d queries, want %d: %v", len(got), count, got)
		}
	}
	return got
}

// errorReportPDU is an Error Report of the given Protocol Version and Error Code, encapsulating
// no PDU and carrying no text.
func errorReportPDU(version uint8, code uint16) []byte {
	report := make([]byte, 16)
	n := writeErrorReport(report, version, code, nil)
	return report[:n]
}

// TestRTRSessionOpensAtTheHighestVersion reads the Protocol Version of the queries a router sends
// to a cache, on a new session and after the cache answered with an Error Report.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 7, "Once a router has established a transport
// connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a
// Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this
// protocol the router implements in the Protocol Version field."
// PREVENTS: a session opened below version 2, and a version lowered by an Error Report that does
// not report an unsupported version, or below the version the cache's report names.
func TestRTRSessionOpensAtTheHighestVersion(t *testing.T) {
	t.Run("a new session opens at version 2 with either query", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-7-4 positive -- the highest version ze implements is 2, and a new session's first query carries Protocol Version 2 both as a Reset Query, with no serial held, and as a Serial Query, with serial 42 held.
		require.Equal(t, uint8(2), rtrVersionMax)
		sync := slices.Concat(cacheResponsePDU(), endOfDataPDU())

		port, queries := serveRTRVersions(t, sync)
		reset := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		require.NoError(t, reset.syncOnce())
		assert.Equal(t, [][2]byte{{2, pduResetQuery}}, takeQueryVersions(t, queries, 1))

		port, queries = serveRTRVersions(t, sync)
		serial := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		serial.serial = 42
		require.NoError(t, serial.syncOnce())
		assert.Equal(t, [][2]byte{{2, pduSerialQuery}}, takeQueryVersions(t, queries, 1))
	})

	t.Run("only an Unsupported Protocol Version report lowers the version, to the one it names", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-7-4 negative -- after a No Data Available Error Report the next query is still Protocol Version 2, and only an Unsupported Protocol Version Error Report of version 1 makes the router query again at version 1, so the three queries carry versions 2, 2 and 1.
		v1Sync := slices.Concat(cacheResponsePDU(), endOfDataPDU())
		v1Sync[0], v1Sync[pduHeaderLen] = 1, 1
		port, queries := serveRTRVersions(t,
			errorReportPDU(2, errNoDataAvail),
			errorReportPDU(1, errUnsupportedVersion),
			v1Sync)
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		require.ErrorIs(t, s.syncOnce(), errRtrNoDataAvailable)
		require.NoError(t, s.syncOnce())
		got := takeQueryVersions(t, queries, 3)
		assert.Equal(t, []byte{2, 2, 1}, []byte{got[0][0], got[1][0], got[2][0]})
	})
}
