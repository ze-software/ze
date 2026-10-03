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

// TestRFC8210V1CacheResponseIsNotTerminated sends a v1 Cache Response to a
// router whose query was v1.
//
// VALIDATES: RFC 8210 Section 7 -- only a version 0 reply makes the router
// "either downgrade to version 0 or terminate the connection". A reply at the
// router's own version is loaded without an Error Report.
// PREVENTS: a version check that refuses every Cache Response.
func TestRFC8210V1CacheResponseIsNotTerminated(t *testing.T) {
	// RFC requirement: RFC8210-7-3 negative -- a v1 Cache Response to a v1 query keeps version 1, publishes its VRP, and draws no Error Report before the router closes the transport.
	type trailer struct {
		bytes []byte
		err   error
	}
	trailing := make(chan trailer, 1)
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return
		}
		query := make([]byte, pduResetQueryLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			return
		}
		response, end := cacheResponsePDU(), endOfDataPDU()
		response[0], end[0] = rtrVersionMin, rtrVersionMin
		binary.BigEndian.PutUint32(end[8:12], 5)
		prefix := []byte{rtrVersionMin, pduIPv4Prefix, 0, 0, 0, 0, 0, 20, 1, 24, 24, 0, 203, 0, 113, 0, 0, 0, 0xfd, 0xf3}
		if _, err := conn.Write(slices.Concat(response, prefix, end)); err != nil {
			return
		}
		// The router has nothing to send after End of Data. Any byte before
		// its EOF would be an Error Report.
		after, err := io.ReadAll(conn)
		trailing <- trailer{bytes: after, err: err}
	})
	s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
	s.version = rtrVersionMin

	require.NoError(t, s.syncOnce())
	assert.Equal(t, rtrVersionMin, s.version, "a v1 reply changed the negotiated version")
	assert.Equal(t, uint32(5), s.serial)
	assert.Equal(t, ValidationValid, s.cache.Validate("203.0.113.0/24", 65011))
	got := <-trailing
	require.NoError(t, got.err)
	assert.Empty(t, got.bytes, "the router answered a v1 load with an Error Report")
}
