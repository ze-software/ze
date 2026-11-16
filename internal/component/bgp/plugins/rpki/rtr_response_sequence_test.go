// Design: docs/architecture/plugin/rib-storage-design.md -- RTR response identity
// Related: rtr_session.go -- readLoop must require Cache Response before payloads
package rpki

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRTRTLSResponseRequiresCacheResponse checks the response sequence over two
// authenticated connections: a complete base, then a delta with or without its
// opening Cache Response. The valid control must keep the base and add the delta.
//
// VALIDATES: RFC 8210 Section 5.8: "The Session ID and Protocol Version MUST be the same as
// that of the corresponding Cache Response which began the (possibly null) sequence of
// payload PDUs." Sections 8.1 and 8.2 place Cache Response before every payload sequence.
// PREVENTS: a response with no identity bypassing the checks and publishing its delta.
func TestRTRTLSResponseRequiresCacheResponse(t *testing.T) {
	for _, withResponse := range []bool{true, false} {
		name := map[bool]string{true: "complete response", false: "missing Cache Response"}[withResponse]
		t.Run(name, func(t *testing.T) {
			response, end := rtrIdentityResponse(0x1234, 2)
			delta := slices.Concat(rtrIdentityPrefix(3), end)
			if withResponse {
				delta = slices.Concat(response, delta)
			}
			// RFC 8210 Sections 8.1 and 8.2: retain a completed base before
			// driving the next Serial Query through the production TLS client.
			session, observed := newRTRIdentitySession(t, delta)
			err := session.syncOnce()
			got := <-observed
			assertRTRIdentityQuery(t, got.query)
			require.NoError(t, got.err, "cache exchange did not finish")
			assert.Equal(t, ValidationValid, session.cache.Validate("192.0.2.0/24", 64500),
				"the unexpired base must survive a malformed response")
			if withResponse {
				require.NoError(t, err)
				assert.Empty(t, got.report)
				assert.Equal(t, uint32(2), session.Snapshot().Serial)
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.3.0/24", 64500))
				return
			}
			assert.Error(t, err, "payloads without Cache Response completed synchronization")
			assert.Equal(t, uint32(1), session.Snapshot().Serial,
				"a response with no identity advanced the previous serial")
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.3.0/24", 64500),
				"a response with no identity published its payload")
			// RFC 8210 Sections 5.11 and 12: report the corrupt response and
			// close the transport; parse the actual report received by the cache.
			header, parseErr := parseHeader(got.report)
			require.NoError(t, parseErr, "missing Cache Response drew no valid Error Report")
			assert.Equal(t, pduErrorRpt, header.Type)
			assert.Zero(t, header.SessionID, "the sequence violation must report Corrupt Data")
			assert.Equal(t, uint32(len(got.report)), header.Length) //nolint:gosec // bounded test response
		})
	}
}

// TestRTRTLSUnknownInitialResponseVersionKeepsSerialBase distinguishes a new
// transport's version negotiation from a version change after Cache Response.
// Both cases first publish a complete authenticated version 2 base.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 12 defines "4: Unsupported Protocol
// Version (non-fatal):  The Protocol Version is not known by the receiver of the
// PDU." It states: "A session is not [re-]established, but data previously learned
// need not be flushed." Section 7 makes the v2 code explicit: "it MUST either
// downgrade to a known version or terminate the connection, with an
// Error Report PDU with Error Code 4 ("Unsupported Protocol Version")
// unless the received PDU is itself an Error Report PDU."
// PREVENTS: a remembered serial making an initial unknown version look like a
// version change within an already negotiated response, and drawing code 8.
func TestRTRTLSUnknownInitialResponseVersionKeepsSerialBase(t *testing.T) {
	for _, version := range []uint8{2, 99} {
		name := map[uint8]string{2: "recognized version 2", 99: "unrecognized version 99"}[version]
		t.Run(name, func(t *testing.T) {
			fixture := newRTRTLSFixture(t)
			var listen net.ListenConfig
			listener, err := listen.Listen(context.Background(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			address, ok := listener.Addr().(*net.TCPAddr)
			require.True(t, ok)

			baseResponse, baseEnd := rtrIdentityResponse(0x1234, 1)
			basePrefix := rtrIdentityPrefix(2)
			baseResponse[0], basePrefix[0], baseEnd[0] = 2, 2, 2
			response, end := rtrIdentityResponse(0x1234, 2)
			prefix := rtrIdentityPrefix(3)
			response[0], prefix[0], end[0] = version, 2, 2
			observed := make(chan rtrIdentityObservation, 2)
			done := make(chan struct{})
			serverConfig := fixture.serverConfig(tls.VersionTLS13)
			go func() {
				defer close(done)
				// One complete base and one serial reply bound the cache worker.
				for _, reply := range [][]byte{
					slices.Concat(baseResponse, basePrefix, aspaPDU(64500, 64501), baseEnd),
					slices.Concat(response, prefix, end),
				} {
					conn, err := listener.Accept()
					if err != nil {
						observed <- rtrIdentityObservation{err: err}
						return
					}
					observed <- rtrIdentityExchange(conn, serverConfig, reply)
					conn.Close() //nolint:errcheck // The exchange has recorded its result; this closes the test-owned socket.
				}
			}()
			t.Cleanup(func() {
				listener.Close() //nolint:errcheck // Closing the owned listener wakes Accept before joining its worker.
				<-done
			})
			session := parsedRTRTLSSession(t, "127.0.0.1", uint16(address.Port), //nolint:gosec // TCP listener port fits uint16.
				rtrTLSConfigLeaves(), make(chan struct{}))
			session.pkiConfig = fixture.store
			session.version = 2
			// RFC 8210 Sections 8.1 and 8.2: load a complete base before the
			// new transport begins negotiation with a remembered serial.
			require.NoError(t, session.syncOnce(), "complete authenticated v2 base failed")
			base := <-observed
			require.NoError(t, base.err)
			require.Empty(t, base.report)
			require.Len(t, base.query, pduResetQueryLen)
			require.Equal(t, []byte{2, pduResetQuery}, base.query[:2])
			require.Equal(t, ValidationValid, session.cache.Validate("192.0.2.0/24", 64500))
			require.True(t, session.aspaCache.isProvider(64500, 64501))
			require.Equal(t, uint32(1), session.Snapshot().Serial)

			err = session.syncOnce()
			got := <-observed
			require.NoError(t, got.err, "cache exchange did not finish")
			require.Len(t, got.query, pduSerialQueryLen)
			assert.Equal(t, []byte{2, pduSerialQuery}, got.query[:2])
			assert.Equal(t, uint16(0x1234), binary.BigEndian.Uint16(got.query[2:4]))
			assert.Equal(t, uint32(1), binary.BigEndian.Uint32(got.query[8:12]))
			assert.Equal(t, ValidationValid, session.cache.Validate("192.0.2.0/24", 64500))
			assert.True(t, session.aspaCache.isProvider(64500, 64501), "non-fatal negotiation must retain ASPA data")
			if version == 2 {
				require.NoError(t, err)
				assert.Empty(t, got.report)
				assert.Equal(t, uint32(2), session.Snapshot().Serial)
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.3.0/24", 64500))
				return
			}
			assert.ErrorIs(t, err, errRtrUnsupportedProtocol)
			assert.Equal(t, uint32(1), session.Snapshot().Serial)
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.3.0/24", 64500))
			// RFC 8210 Sections 7 and 12: this first Cache Response has not
			// negotiated a version, so its unknown value requires code 4.
			header, parseErr := parseHeader(got.report)
			require.NoError(t, parseErr, "unknown initial version drew no Error Report")
			assert.Equal(t, pduErrorRpt, header.Type)
			assert.Equal(t, errUnsupportedVersion, header.SessionID)
			require.Len(t, got.report, 16+len(response))
			assert.Equal(t, response, got.report[12:12+len(response)])
		})
	}
}

// TestRTRTLSV2FatalErrorsFlushPublishedBase checks fatal error handling after
// a successful v2 load. The valid response is the control for every refusal.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 12: "Errors which are considered fatal MUST
// cause the session to be dropped, and the router MUST flush all data learned from that cache."
// Section 5.12 requires Error Code 9 when an announcement has no provider.
// PREVENTS: retaining a fatal v2 cache's VRPs or ASPAs, using the received PDU's
// version to choose cleanup, or answering an Error Report to perform cleanup.
func TestRTRTLSV2FatalErrorsFlushPublishedBase(t *testing.T) {
	response, end := rtrIdentityResponse(0x1234, 2)
	prefix := rtrIdentityPrefix(3)
	response[0], prefix[0], end[0] = 2, 2, 2
	changedVersion := slices.Clone(prefix)
	changedVersion[0] = 1
	cacheError := make([]byte, 16)
	cacheError = cacheError[:writeErrorReport(cacheError, 2, 0, nil)]
	for _, scenario := range []struct {
		name          string
		reply         []byte
		fatal         bool
		wantReport    bool
		wantErrorCode uint16
	}{
		{name: "complete valid response", reply: slices.Concat(response, prefix, end)},
		{name: "valid payload before Cache Response", reply: slices.Concat(prefix, end), fatal: true, wantReport: true, wantErrorCode: 0},
		{name: "version changes within response", reply: slices.Concat(response, changedVersion, end), fatal: true, wantReport: true, wantErrorCode: 8},
		{name: "empty ASPA before Cache Response", reply: aspaPDU(64510), fatal: true, wantReport: true, wantErrorCode: 9},
		{name: "empty ASPA after Cache Response", reply: slices.Concat(response, aspaPDU(64510)), fatal: true, wantReport: true, wantErrorCode: 9},
		{name: "fatal Error Report is not answered", reply: cacheError, fatal: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			session, observed := newRTRV2SequenceSession(t,
				rtrV2SequenceLoad(2, 64500, 64501), scenario.reply)
			// Draft-ietf-sidrops-8210bis Section 8.1: establish usable data
			// before exposing the same cache to a second response.
			require.NoError(t, session.syncOnce(), "complete v2 base failed")
			base := <-observed
			require.NoError(t, base.err)
			require.Empty(t, base.report)
			require.Equal(t, ValidationValid, session.cache.Validate("192.0.2.0/24", 64500))
			require.True(t, session.aspaCache.isProvider(64500, 64501))
			require.Equal(t, uint32(1), session.Snapshot().Serial)

			err := session.syncOnce()
			got := <-observed
			require.NoError(t, got.err, "cache exchange did not finish")
			require.Len(t, got.query, pduSerialQueryLen)
			assert.Equal(t, []byte{2, pduSerialQuery}, got.query[:2])
			assert.Equal(t, uint16(0x1234), binary.BigEndian.Uint16(got.query[2:4]))
			assert.Equal(t, uint32(1), binary.BigEndian.Uint32(got.query[8:12]))
			if !scenario.fatal {
				require.NoError(t, err)
				assert.Empty(t, got.report)
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.2.0/24", 64500))
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.3.0/24", 64500))
				assert.True(t, session.aspaCache.isProvider(64500, 64501))
				assert.Equal(t, uint32(2), session.Snapshot().Serial)
				return
			}
			assert.Error(t, err, "a fatal v2 error completed synchronization")
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.2.0/24", 64500),
				"fatal v2 response retained its previously published base")
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.3.0/24", 64500))
			assert.False(t, session.aspaCache.isProvider(64500, 64501), "fatal v2 response retained its ASPA")
			assert.False(t, session.Snapshot().Synced)
			if !scenario.wantReport {
				assert.Empty(t, got.report, "an Error Report must never receive another Error Report")
				return
			}
			// Draft-ietf-sidrops-8210bis Sections 5.11 and 12: classify the
			// actual report independently from the cache-data assertions.
			header, parseErr := parseHeader(got.report)
			require.NoError(t, parseErr)
			assert.Equal(t, pduErrorRpt, header.Type)
			assert.Equal(t, uint8(2), header.Version)
			assert.Equal(t, scenario.wantErrorCode, header.SessionID)
		})
	}
}

// TestRTRTLSV2StaleFatalResponsePreservesReplacement checks the other side of
// fatal cleanup: a second cache has already completed its replacement load.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 12: "the router MUST flush all data learned
// from that cache." Data already replaced by another cache does not belong to the failed one.
// PREVENTS: an obsolete session clearing the currently published VRP and ASPA generation.
func TestRTRTLSV2StaleFatalResponsePreservesReplacement(t *testing.T) {
	prefix := rtrIdentityPrefix(3)
	prefix[0] = 2
	old, oldObserved := newRTRV2SequenceSession(t,
		rtrV2SequenceLoad(2, 64500, 64501), prefix)
	require.NoError(t, old.syncOnce())
	base := <-oldObserved
	require.NoError(t, base.err)
	require.Equal(t, ValidationValid, old.cache.Validate("192.0.2.0/24", 64500))
	require.True(t, old.aspaCache.isProvider(64500, 64501))

	replacement, replacementObserved := newRTRV2SequenceSession(t, rtrV2SequenceLoad(4, 64510, 64511))
	replacement.cache, replacement.aspaCache = old.cache, old.aspaCache
	replacement.dataLease = old.dataLease
	// Draft-ietf-sidrops-8210bis Sections 8.1 and 10: the second cache's
	// completed Reset Query replaces both stores before the old cache fails.
	require.NoError(t, replacement.syncOnce())
	loaded := <-replacementObserved
	require.NoError(t, loaded.err)
	require.Empty(t, loaded.report)
	require.NotEqual(t, old.dataGeneration, replacement.dataGeneration)
	require.Equal(t, ValidationNotFound, old.cache.Validate("192.0.2.0/24", 64500))
	require.Equal(t, ValidationValid, old.cache.Validate("192.0.4.0/24", 64500))
	require.False(t, old.aspaCache.isProvider(64500, 64501))
	require.True(t, old.aspaCache.isProvider(64510, 64511))

	err := old.syncOnce()
	failed := <-oldObserved
	require.NoError(t, failed.err)
	assert.Error(t, err)
	require.Len(t, failed.query, pduResetQueryLen, "an obsolete generation requires a Reset Query")
	assert.Equal(t, []byte{2, pduResetQuery}, failed.query[:2])
	header, parseErr := parseHeader(failed.report)
	require.NoError(t, parseErr)
	assert.Equal(t, pduErrorRpt, header.Type)
	assert.Zero(t, header.SessionID)
	assert.Equal(t, ValidationValid, old.cache.Validate("192.0.4.0/24", 64500),
		"the failed cache cleared a completed replacement")
	assert.True(t, old.aspaCache.isProvider(64510, 64511), "the failed cache cleared the replacement ASPA")
	assert.True(t, replacement.Snapshot().Synced)
}

// TestRTRV2FatalCleanupSurvivesErrorReportWriteFailure uses a completed TLS
// base and a deterministic write-failure seam at the next stream boundary.
// An actual peer close races TCP buffering and cannot guarantee a failed write.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 12: "Errors which are considered fatal MUST
// cause the session to be dropped, and the router MUST flush all data learned from that cache."
// PREVENTS: returning a failed Error Report write before revoking the fatal cache's data.
func TestRTRV2FatalCleanupSurvivesErrorReportWriteFailure(t *testing.T) {
	session, observed := newRTRV2SequenceSession(t, rtrV2SequenceLoad(2, 64500, 64501))
	require.NoError(t, session.syncOnce())
	base := <-observed
	require.NoError(t, base.err)
	require.Equal(t, ValidationValid, session.cache.Validate("192.0.2.0/24", 64500))
	require.True(t, session.aspaCache.isProvider(64500, 64501))

	client, cache := net.Pipe()
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, cache.SetDeadline(time.Now().Add(5*time.Second)))
	finished := make(chan struct{})
	peerResult := make(chan error, 1)
	prefix := rtrIdentityPrefix(3)
	prefix[0] = 2
	go func() {
		defer close(finished)
		if _, err := cache.Write(prefix); err != nil {
			peerResult <- err
			return
		}
		var reply [1]byte
		_, err := cache.Read(reply[:])
		peerResult <- err
	}()
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // Closing both owned pipe ends releases the joined peer on failure.
		cache.Close()  //nolint:errcheck // Closing both owned pipe ends releases the joined peer on failure.
		<-finished
	})
	writeFailure := errors.New("injected RTR Error Report write failure")
	err := session.readLoop(rtrReportWriteFailure{Conn: client, failure: writeFailure})
	require.ErrorIs(t, err, writeFailure)
	require.NoError(t, client.Close())
	require.ErrorIs(t, <-peerResult, io.EOF)
	assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.2.0/24", 64500))
	assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.3.0/24", 64500))
	assert.False(t, session.aspaCache.isProvider(64500, 64501))
	assert.False(t, session.Snapshot().Synced)
}

// rtrReportWriteFailure retains the real stream reads and deadlines while
// making the report write fail independently of peer-close timing.
type rtrReportWriteFailure struct {
	net.Conn
	failure error
}

func (c rtrReportWriteFailure) Write([]byte) (int, error) {
	return 0, c.failure
}

// newRTRV2SequenceSession starts one joined TLS cache worker for the finite
// reply script. The caller drives every exchange through syncOnce; cleanup
// MUST close the listener and join the worker, including after a test failure.
func newRTRV2SequenceSession(t *testing.T, replies ...[]byte) (*RTRSession, <-chan rtrIdentityObservation) {
	t.Helper()
	fixture := newRTRTLSFixture(t)
	var listen net.ListenConfig
	listener, err := listen.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	observed := make(chan rtrIdentityObservation, len(replies))
	done := make(chan struct{})
	serverConfig := fixture.serverConfig(tls.VersionTLS13)
	go func() {
		defer close(done)
		for _, reply := range replies {
			conn, err := listener.Accept()
			if err != nil {
				observed <- rtrIdentityObservation{err: err}
				return
			}
			observed <- rtrIdentityExchange(conn, serverConfig, reply)
			conn.Close() //nolint:errcheck // The exchange has recorded its result; this closes the test-owned socket.
		}
	}()
	t.Cleanup(func() {
		listener.Close() //nolint:errcheck // Closing the owned listener wakes Accept before joining its worker.
		<-done
	})
	session := parsedRTRTLSSession(t, "127.0.0.1", uint16(address.Port), //nolint:gosec // TCP listener port fits uint16.
		rtrTLSConfigLeaves(), make(chan struct{}))
	session.pkiConfig = fixture.store
	session.version = 2
	return session, observed
}

// rtrV2SequenceLoad is a complete v2 cache load with one VRP and one ASPA.
// Draft-ietf-sidrops-8210bis Section 8.1: Cache Response starts the payload
// sequence and End of Data completes the Reset Query.
func rtrV2SequenceLoad(octet byte, customer, provider uint32) []byte {
	response, end := rtrIdentityResponse(0x1234, 1)
	prefix := rtrIdentityPrefix(octet)
	response[0], prefix[0], end[0] = 2, 2, 2
	return slices.Concat(response, prefix, aspaPDU(customer, provider), end)
}
