// RFC naming: untagged -- these tests run RTR version 1 or 2 sessions, so they prove shared router behavior that RFC 8210 rows already carry; ze speaks no RTR version 0, and every RFC 6810 row they once carried is a gap (owner ruling 2026-10-30).
package rpki

import (
	"encoding/binary"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC6810DefaultPollCadenceIsHourly checks the cadence a session uses
// before any cache has supplied timing parameters.
//
// VALIDATES: before any End of Data has set intervals, both waits a version 2
// session (rtrVersionMax) takes between queries (after a sync and after a failed
// query) start at or below one hour.
// PREVENTS: a default that lets an idle router go quiet for more than an hour.
func TestRFC6810DefaultPollCadenceIsHourly(t *testing.T) {
	s := newTestRTRSession(t, "192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), make(chan struct{}))

	assert.LessOrEqual(t, s.pollDelay(true), time.Hour, "the refresh wait exceeds one hour")
	assert.LessOrEqual(t, s.pollDelay(false), time.Hour, "the retry wait exceeds one hour")
}

// TestRFC6810ReservedPrefixOctetsIgnored drives a Prefix PDU whose reserved
// octets are all ones through the session into the cache used for validation.
//
// VALIDATES: in a version 1 session, a Prefix PDU whose header zero field,
// reserved flag bits and zero octet are all ones still authorizes its route, and
// the same PDU with only the announce bit cleared withdraws it.
// PREVENTS: a receiver that rejects or misreads a PDU because of reserved bits.
func TestRFC6810ReservedPrefixOctetsIgnored(t *testing.T) {
	cache := newROACache()
	s := newTestRTRSession(t, "127.0.0.1", 3323, 100, "", cache, newASPACache(), make(chan struct{}))
	s.version = rtrVersionMin
	end := endOfDataPDU()
	end[0] = rtrVersionMin

	pdu := dirtyBuf(20)
	pdu[0], pdu[1] = rtrVersionMin, pduIPv4Prefix
	binary.BigEndian.PutUint32(pdu[4:8], uint32(len(pdu)))
	pdu[9], pdu[10] = 24, 24
	copy(pdu[12:16], []byte{198, 51, 100, 0})
	binary.BigEndian.PutUint32(pdu[16:20], 65010)

	applyPDU(t, s, pdu, false)
	applyPDU(t, s, end, true)
	assert.Equal(t, ValidationValid, cache.Validate("198.51.100.0/24", 65010))

	pdu[8] = 0xfe
	applyPDU(t, s, pdu, false)
	applyPDU(t, s, end, true)
	assert.Equal(t, ValidationNotFound, cache.Validate("198.51.100.0/24", 65010))
}

// TestRFC6810NoDataAvailableRepeatsResetQuery runs one cache that answers
// twice with No Data Available and then supplies a load.
//
// VALIDATES: in version 1 sessions with one cache that reports No Data
// Available, the router sends a Reset Query on each of three consecutive polls,
// and once that cache supplies a usable load the next query is a Serial Query.
// PREVENTS: a router that gives up after No Data Available, or one that keeps
// resetting after the usable load arrived.
func TestRFC6810NoDataAvailableRepeatsResetQuery(t *testing.T) {
	queries := make(chan uint8, 4)
	attempt := 0
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return
		}
		query := make([]byte, pduHeaderLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			return
		}
		if query[1] == pduSerialQuery {
			if _, err := io.CopyN(io.Discard, conn, 4); err != nil {
				return
			}
		}
		queries <- query[1]
		attempt++
		if attempt <= 2 {
			noData := []byte{1, pduErrorRpt, 0, 2, 0, 0, 0, 16, 0, 0, 0, 0, 0, 0, 0, 0}
			if _, err := conn.Write(noData); err != nil {
				return
			}
			_, _ = conn.Read(query)
			return
		}
		response, end := cacheResponsePDU(), endOfDataPDU()
		response[0], end[0] = 1, 1
		binary.BigEndian.PutUint32(end[8:12], 91)
		prefix := []byte{1, pduIPv4Prefix, 0, 0, 0, 0, 0, 20, 1, 24, 24, 0, 198, 51, 100, 0, 0, 0, 0xfd, 0xf2}
		if _, err := conn.Write(slices.Concat(response, prefix, end)); err != nil {
			return
		}
	})
	stop := make(chan struct{})
	s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), stop)
	s.version = rtrVersionMin
	s.retryInterval = time.Second
	done := make(chan struct{})
	go func() { defer close(done); newCacheGroup([]*RTRSession{s}, stop).Run() }()
	t.Cleanup(func() {
		close(stop)
		s.close()
		<-done
	})

	for range 3 {
		select {
		case query := <-queries:
			assert.Equal(t, pduResetQuery, query)
		case <-time.After(4 * time.Second):
			t.Fatal("No Data Available did not lead to another Reset Query")
		}
	}
	require.Eventually(t, func() bool {
		return s.cache.Validate("198.51.100.0/24", 65010) == ValidationValid && s.Snapshot().State == sessionIdle
	}, 4*time.Second, time.Millisecond, "the usable load was not published")

	require.NoError(t, s.syncOnce())
	select {
	case query := <-queries:
		assert.Equal(t, pduSerialQuery, query)
	case <-time.After(4 * time.Second):
		t.Fatal("the usable load did not permit a Serial Query")
	}
}
