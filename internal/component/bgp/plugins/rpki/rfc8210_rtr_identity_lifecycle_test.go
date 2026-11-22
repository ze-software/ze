// Design: docs/guide/rpki.md -- authenticated RTR data must keep its session identity.
// RFC: rfc/short/rfc8210.md -- Sections 5.1 and 7, Session ID and Protocol Version identity
// Related: rtr_response_identity.go -- checkResponseIdentity, rtrResponseErrorCode, revokeResponseData
// Related: rtr_session.go -- Cache Response and End of Data processing
package rpki

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRTRTLSSerialResponseSessionIDMismatch uses two authenticated connections
// to one cache: a complete base, then a delta whose Cache Response changes ID.
//
// VALIDATES: RFC 8210 Section 5.1: "If, at any time after the protocol version
// has been negotiated (Section 7), either the router or the cache finds that
// the value of the Session ID is not the same as the other's, the party which
// detects the mismatch MUST immediately terminate the session with an Error
// Report PDU with code 0 ("Corrupt Data"), and the router MUST flush all data
// learned from that cache."
// PREVENTS: adopting a restarted cache's ID while merging its delta with the
// previous cache instance's serial base.
//
// RFC requirement: RFC8210-5.1-3 positive -- when Cache Response changes Session ID after negotiation, the router terminates the session with Error Report code 0 (Corrupt Data).
// RFC requirement: RFC8210-5.1-3 negative -- when Cache Response keeps the negotiated Session ID, the router completes the serial sync without sending an Error Report.
// RFC requirement: RFC8210-5.1-4 positive -- on Session ID mismatch, the router flushes previously learned VRPs from that cache (192.0.2.0/24 returns ValidationNotFound) and marks the session unsynced.
// RFC requirement: RFC8210-5.1-4 negative -- when Session ID matches, the router retains its previously learned VRPs and applies the delta.
func TestRTRTLSSerialResponseSessionIDMismatch(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "matching ID", true: "changed Cache Response ID"}[mismatch], func(t *testing.T) {
			response, end := rtrIdentityResponse(0x1234, 2)
			if mismatch {
				binary.BigEndian.PutUint16(response[2:4], 0x1235)
				binary.BigEndian.PutUint16(end[2:4], 0x1235)
			}
			session, observed := newRTRIdentitySession(t,
				slices.Concat(response, rtrIdentityPrefix(3), end))
			err := session.syncOnce()
			got := <-observed
			assertRTRIdentityQuery(t, got.query)
			require.NoError(t, got.err, "cache exchange did not finish")
			if !mismatch {
				require.NoError(t, err)
				assert.Empty(t, got.report)
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.2.0/24", 64500))
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.3.0/24", 64500))
				return
			}
			assert.Error(t, err, "a changed Session ID must fail the serial response")
			assertRTRIdentityCorruptReport(t, got.report)
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.2.0/24", 64500),
				"the mismatched cache's previously learned base was not flushed")
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.3.0/24", 64500),
				"the mismatched response published a delta")
			assert.False(t, session.Snapshot().Synced)
		})
	}
}

// TestRTRTLSEndOfDataSessionIDMismatch changes only the End of Data ID after
// a matching Cache Response, so a response-header check alone cannot pass it.
//
// VALIDATES: RFC 8210 Section 5.1's mismatch termination, Corrupt Data report
// and flush, with the mismatch in End of Data. Section 5.8 states: "The
// Session ID and Protocol Version MUST be the same as that of the
// corresponding Cache Response which began the (possibly null) sequence
// of payload PDUs."
// PREVENTS: committing staged payloads before checking which session ended.
func TestRTRTLSEndOfDataSessionIDMismatch(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "matching ID", true: "changed End of Data ID"}[mismatch], func(t *testing.T) {
			response, end := rtrIdentityResponse(0x1234, 2)
			if mismatch {
				binary.BigEndian.PutUint16(end[2:4], 0x1235)
			}
			session, observed := newRTRIdentitySession(t,
				slices.Concat(response, rtrIdentityPrefix(3), end))
			err := session.syncOnce()
			got := <-observed
			assertRTRIdentityQuery(t, got.query)
			require.NoError(t, got.err)
			if !mismatch {
				require.NoError(t, err)
				assert.Empty(t, got.report)
				assert.Equal(t, uint32(2), session.Snapshot().Serial)
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.3.0/24", 64500))
				return
			}
			assert.Error(t, err, "mismatched End of Data completed the response")
			assertRTRIdentityCorruptReport(t, got.report)
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.2.0/24", 64500))
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.3.0/24", 64500))
			assert.False(t, session.Snapshot().Synced)
		})
	}
}

// TestRTRTLSSupportedVersionCannotChangeWithinResponse changes a supported
// version after a v1 Cache Response, with unchanged Session ID and serial base.
//
// VALIDATES: RFC 8210 Section 7: "If either party receives a PDU for a
// different Protocol Version once the above negotiation completes, that party
// MUST drop the session; unless the PDU containing the unexpected Protocol
// Version was itself an Error Report PDU, the party dropping the session
// SHOULD send an Error Report with an error code of 8 ("Unexpected Protocol
// Version")." The assertions require the MUST; Error Report 8 is a SHOULD
// and is deliberately not a prerequisite of this test.
// PREVENTS: checking only whether a version is supported, then merging payloads
// of another version into a v1 serial base.
//
// RFC requirement: RFC8210-7-4 positive -- receiving a prefix or End of Data PDU with a different Protocol Version after Cache Response drops the session without publishing the payload or advancing the serial.
// RFC requirement: RFC8210-7-4 negative -- receiving PDUs at the negotiated Protocol Version completes the response and publishes the delta without error.
func TestRTRTLSSupportedVersionCannotChangeWithinResponse(t *testing.T) {
	for _, stage := range []string{"unchanged", "prefix", "end of data"} {
		t.Run(stage, func(t *testing.T) {
			response, end := rtrIdentityResponse(0x1234, 2)
			prefix := rtrIdentityPrefix(3)
			if stage == "prefix" {
				prefix[0] = rtrVersionMax
			}
			if stage == "end of data" {
				end[0] = rtrVersionMax
			}
			session, observed := newRTRIdentitySession(t, slices.Concat(response, prefix, end))
			err := session.syncOnce()
			got := <-observed
			assertRTRIdentityQuery(t, got.query)
			require.NoError(t, got.err)
			if stage == "unchanged" {
				require.NoError(t, err)
				assert.Empty(t, got.report)
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.3.0/24", 64500))
				assert.Equal(t, uint32(2), session.Snapshot().Serial)
				return
			}
			assert.Error(t, err, "a PDU changed the negotiated version during a response")
			assert.Equal(t, uint32(1), session.Snapshot().Serial,
				"the mixed-version delta advanced the previous session's serial")
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.3.0/24", 64500),
				"the mixed-version payload became usable")
		})
	}
}

// TestRFC8210DifferentVersionSameSessionIDIsSeparateSession sends a Cache Response
// carrying the same numeric Session ID (0x1234) as the prior v1 session, but under
// a different Protocol Version (v2).
//
// VALIDATES: RFC 8210 Section 5.1: "routers MUST treat sessions with different
// Protocol Version fields as separate sessions even if they do happen to have
// the same Session ID."
// PREVENTS: matching Session ID alone and merging a v2 delta into a v1 serial base.
//
// RFC requirement: RFC8210-5.1-5 positive -- a Cache Response with a different Protocol Version (v2) and the same Session ID (0x1234) as an existing v1 session is rejected as a separate session and never merges its delta into the v1 serial base.
// RFC requirement: RFC8210-5.1-5 negative -- a Cache Response with the same Protocol Version (v1) and same Session ID (0x1234) continues the existing session and applies its delta.
func TestRFC8210DifferentVersionSameSessionIDIsSeparateSession(t *testing.T) {
	for _, diffVersion := range []bool{false, true} {
		name := map[bool]string{false: "same version same Session ID", true: "different version same Session ID"}[diffVersion]
		t.Run(name, func(t *testing.T) {
			response, end := rtrIdentityResponse(0x1234, 2)
			prefix := rtrIdentityPrefix(3)
			if diffVersion {
				response[0], prefix[0], end[0] = rtrVersionMax, rtrVersionMax, rtrVersionMax
			}
			session, observed := newRTRIdentitySession(t, slices.Concat(response, prefix, end))
			err := session.syncOnce()
			got := <-observed
			assertRTRIdentityQuery(t, got.query)
			require.NoError(t, got.err)
			if !diffVersion {
				require.NoError(t, err)
				assert.Empty(t, got.report)
				assert.Equal(t, uint32(2), session.Snapshot().Serial)
				assert.Equal(t, ValidationValid, session.cache.Validate("192.0.3.0/24", 64500))
				return
			}
			assert.ErrorIs(t, err, errRtrUnexpectedVersion,
				"different Protocol Version with identical Session ID must not continue the v1 session")
			assert.Equal(t, uint32(1), session.Snapshot().Serial,
				"cross-version response must not advance the v1 serial")
			assert.Equal(t, ValidationNotFound, session.cache.Validate("192.0.3.0/24", 64500),
				"cross-version payload must not merge into the v1 session")
		})
	}
}

type rtrIdentityObservation struct {
	query  []byte
	report []byte
	err    error
}

// newRTRIdentitySession starts one joined TLS cache worker for two connections,
// loads the complete base through syncOnce, and returns the session before its
// serial query. The buffered observation arrives only after router EOF.
func newRTRIdentitySession(t *testing.T, delta []byte) (*RTRSession, <-chan rtrIdentityObservation) {
	t.Helper()
	fixture := newRTRTLSFixture(t)
	var listen net.ListenConfig
	listener, err := listen.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	observed := make(chan rtrIdentityObservation, 2)
	done := make(chan struct{})
	response, end := rtrIdentityResponse(0x1234, 1)
	base := slices.Concat(response, rtrIdentityPrefix(2), end)
	go func() {
		defer close(done)
		for _, reply := range [][]byte{base, delta} {
			conn, err := listener.Accept()
			if err != nil {
				observed <- rtrIdentityObservation{err: err}
				return
			}
			observed <- rtrIdentityExchange(conn, fixture.serverConfig(tls.VersionTLS13), reply)
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	session := parsedRTRTLSSession(t, "127.0.0.1", uint16(address.Port), //nolint:gosec // TCP listener port fits uint16.
		rtrTLSConfigLeaves(), make(chan struct{}))
	session.pkiConfig = fixture.store
	session.version = rtrVersionMin
	require.NoError(t, session.syncOnce(), "complete authenticated base failed")
	baseObserved := <-observed
	require.NoError(t, baseObserved.err)
	require.Empty(t, baseObserved.report)
	require.Len(t, baseObserved.query, pduResetQueryLen)
	require.Equal(t, pduResetQuery, baseObserved.query[1])
	require.Equal(t, ValidationValid, session.cache.Validate("192.0.2.0/24", 64500))
	require.Equal(t, uint16(0x1234), session.Snapshot().SessionID)
	require.Equal(t, uint32(1), session.Snapshot().Serial)
	return session, observed
}

func rtrIdentityExchange(conn net.Conn, config *tls.Config, reply []byte) rtrIdentityObservation {
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return rtrIdentityObservation{err: err}
	}
	secure := tls.Server(conn, config)
	if err := secure.HandshakeContext(context.Background()); err != nil {
		return rtrIdentityObservation{err: err}
	}
	query, err := readRTRQuery(secure)
	if err != nil {
		return rtrIdentityObservation{err: err}
	}
	if _, err := secure.Write(reply); err != nil {
		return rtrIdentityObservation{query: query, err: err}
	}
	report, err := io.ReadAll(secure)
	return rtrIdentityObservation{query: query, report: report, err: err}
}

func rtrIdentityResponse(id uint16, serial uint32) ([]byte, []byte) {
	response, end := cacheResponsePDU(), endOfDataPDU()
	response[0], end[0] = rtrVersionMin, rtrVersionMin
	binary.BigEndian.PutUint16(response[2:4], id)
	binary.BigEndian.PutUint16(end[2:4], id)
	binary.BigEndian.PutUint32(end[8:12], serial)
	return response, end
}

func rtrIdentityPrefix(octet byte) []byte {
	prefix := ipv4AnnouncePDU([4]byte{192, 0, octet, 0}, 64500)
	prefix[0] = rtrVersionMin
	return prefix
}

func assertRTRIdentityQuery(t *testing.T, query []byte) {
	t.Helper()
	require.Len(t, query, pduSerialQueryLen)
	assert.Equal(t, rtrVersionMin, query[0])
	assert.Equal(t, pduSerialQuery, query[1])
	assert.Equal(t, uint16(0x1234), binary.BigEndian.Uint16(query[2:4]))
	assert.Equal(t, uint32(1), binary.BigEndian.Uint32(query[8:12]))
}

func assertRTRIdentityCorruptReport(t *testing.T, report []byte) {
	t.Helper()
	if !assert.GreaterOrEqual(t, len(report), 16, "Session ID mismatch drew no Error Report") {
		return
	}
	assert.Equal(t, pduErrorRpt, report[1])
	assert.Equal(t, uint16(0), binary.BigEndian.Uint16(report[2:4]))
	assert.Equal(t, uint32(len(report)), binary.BigEndian.Uint32(report[4:8])) //nolint:gosec // bounded test response
}
