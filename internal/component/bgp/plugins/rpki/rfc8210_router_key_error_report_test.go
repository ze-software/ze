// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 8210 Router Key duplicate detection and generic Error Report conformance tests
// Related: rtr_router_key.go -- parseRouterKeyPDU, applyStagedRouterKeyLocked
// Related: panic_guard.go -- reportInternalError
// Related: rtr_session.go -- RTRSession.handlePDU, RTRSession.readLoop
// RFC 8210 Sections 5.10 and 5.11 -- see rfc/short/rfc8210.md

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

func buildRouterKeyPDU(version uint8, announce bool, ski [20]byte, asn uint32, spki []byte) []byte {
	totalLen := 32 + len(spki)
	pdu := make([]byte, totalLen)
	pdu[0] = version
	pdu[1] = pduRouterKey
	if announce {
		pdu[2] = 1
	}
	binary.BigEndian.PutUint32(pdu[4:8], uint32(totalLen))
	copy(pdu[8:28], ski[:])
	binary.BigEndian.PutUint32(pdu[28:32], asn)
	copy(pdu[32:], spki)
	return pdu
}

// TestRFC8210RouterKeyDuplicateComparesSPKIAndSKI proves RFC 8210 Section 5.10
// over real TCP RTR sessions: implementations MUST compare Subject Public Key
// values as well as SKIs when detecting duplicate Router Key PDUs.
//
// RFC requirement: RFC8210-5.10-2 positive -- receiving two Router Key announcements with the SAME SKI and SAME ASN but DIFFERENT Subject Public Key Info values is NOT a duplicate (both keys are retained and sync completes with no Error Report); withdrawing one of them removes only its matching SPKI while keeping the colliding-SKI sibling; likewise, two keys with DIFFERENT SKIs and identical SPKI/ASN are NOT duplicates.
// RFC requirement: RFC8210-5.10-2 negative -- receiving a duplicate Router Key announcement with identical {SKI, ASN, Subject Public Key} (either within one sync or across a subsequent incremental Serial Query sync) raises Error Report PDU code 7 ("Duplicate Announcement Received") encapsulating the duplicate PDU and drops the session.
func TestRFC8210RouterKeyDuplicateComparesSPKIAndSKI(t *testing.T) {
	skiA := [20]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14}
	skiB := [20]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22, 0x33, 0x44,
		0x55, 0x66, 0x77, 0x88, 0x99, 0x00, 0x12, 0x34, 0x56, 0x78}
	spki1 := []byte{0x30, 0x59, 0x30, 0x13, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, 0x01}
	spki2 := []byte{0x30, 0x59, 0x30, 0x13, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, 0x02}

	t.Run("same SKI with different SPKI, different SKI with same SPKI, and different ASN with same SKI and SPKI are not duplicates (v1 and v2)", func(t *testing.T) {
		for _, ver := range []uint8{1, rtrVersionMax} {
			keySameSKISPKI1 := buildRouterKeyPDU(ver, true, skiA, 64500, spki1)
			keySameSKISPKI2 := buildRouterKeyPDU(ver, true, skiA, 64500, spki2)
			keyDiffSKISPKI1 := buildRouterKeyPDU(ver, true, skiB, 64500, spki1)
			keyDiffASN := buildRouterKeyPDU(ver, true, skiA, 64501, spki1)
			withdrawSKISPKI1 := buildRouterKeyPDU(ver, false, skiA, 64500, spki1)

			resp := cacheResponsePDU()
			resp[0] = ver
			eod1 := endOfDataTimed(10, 3600, 600)
			eod1[0] = ver
			eod2 := endOfDataTimed(11, 3600, 600)
			eod2[0] = ver

			answered := make(chan error, 2)
			attempt := 0
			port, _ := serveRTR(t, func(conn net.Conn) {
				if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					answered <- err
					return
				}
				if _, err := readRTRQuery(conn); err != nil {
					answered <- err
					return
				}
				attempt++
				var payload []byte
				if attempt == 1 {
					// First sync: colliding SKI with two distinct SPKIs + distinct SKI with same SPKI + distinct ASN with same SKI/SPKI.
					payload = slices.Concat(
						resp,
						keySameSKISPKI1,
						keySameSKISPKI2,
						keyDiffSKISPKI1,
						keyDiffASN,
						eod1,
					)
				} else {
					// Second incremental sync: withdraw ONLY (skiA, 64500, spki1); (skiA, 64500, spki2) MUST survive!
					payload = slices.Concat(
						resp,
						withdrawSKISPKI1,
						eod2,
					)
				}
				if _, err := conn.Write(payload); err != nil {
					answered <- err
					return
				}
				var b [1]byte
				_, err := io.ReadFull(conn, b[:])
				answered <- err
			})

			s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
			s.version = ver
			require.NoError(t, s.syncOnce(), "v%d: distinct SPKIs sharing an SKI must not trigger duplicate error", ver)
			require.ErrorIs(t, <-answered, io.EOF, "v%d: router must not send Error Report on distinct SPKIs", ver)

			s.mu.Lock()
			require.Len(t, s.activeRouterKeys, 4, "v%d: all four distinct {SKI, ASN, SPKI} keys must be active", ver)
			_, hasA1 := s.activeRouterKeys[routerKeyRecord{SKI: skiA, ASN: 64500, SPKI: string(spki1)}]
			_, hasA2 := s.activeRouterKeys[routerKeyRecord{SKI: skiA, ASN: 64500, SPKI: string(spki2)}]
			_, hasB1 := s.activeRouterKeys[routerKeyRecord{SKI: skiB, ASN: 64500, SPKI: string(spki1)}]
			_, hasDiffASN := s.activeRouterKeys[routerKeyRecord{SKI: skiA, ASN: 64501, SPKI: string(spki1)}]
			s.mu.Unlock()
			require.True(t, hasA1 && hasA2 && hasB1 && hasDiffASN)

			// Incremental withdrawal of (skiA, 64500, spki1) removes ONLY spki1 and keeps spki2.
			require.NoError(t, s.syncOnce())
			require.ErrorIs(t, <-answered, io.EOF)
			s.mu.Lock()
			require.Len(t, s.activeRouterKeys, 3)
			_, stillA1 := s.activeRouterKeys[routerKeyRecord{SKI: skiA, ASN: 64500, SPKI: string(spki1)}]
			_, stillA2 := s.activeRouterKeys[routerKeyRecord{SKI: skiA, ASN: 64500, SPKI: string(spki2)}]
			s.mu.Unlock()
			require.False(t, stillA1, "v%d: withdrawn SPKI must be removed", ver)
			require.True(t, stillA2, "v%d: colliding-SKI sibling with distinct SPKI must remain active", ver)
		}
	})

	t.Run("identical SKI ASN and SPKI triggers Error Report 7 within one sync", func(t *testing.T) {
		dupKey := buildRouterKeyPDU(rtrVersionMax, true, skiA, 64500, spki1)
		reported := make(chan []byte, 1)
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			if _, err := readRTRQuery(conn); err != nil {
				return
			}
			if _, err := conn.Write(slices.Concat(cacheResponsePDU(), dupKey, dupKey)); err != nil {
				return
			}
			got, _ := io.ReadAll(conn)
			reported <- got
		})

		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		require.ErrorIs(t, s.syncOnce(), errRtrDuplicateRouterKey)
		var report []byte
		select {
		case report = <-reported:
		case <-time.After(3 * time.Second):
			t.Fatal("router did not send Error Report 7 for duplicate Router Key")
		}
		require.Equal(t, 16+len(dupKey), len(report))
		assert.Equal(t, pduErrorRpt, report[1])
		assert.Equal(t, errDuplicateAnnouncement, binary.BigEndian.Uint16(report[2:4]))
		assert.Equal(t, uint32(len(dupKey)), binary.BigEndian.Uint32(report[8:12]))
		assert.Equal(t, dupKey, report[12:12+len(dupKey)])
	})

	t.Run("identical SKI ASN and SPKI across incremental sync triggers Error Report 7", func(t *testing.T) {
		key1 := buildRouterKeyPDU(rtrVersionMax, true, skiA, 64500, spki1)
		reported := make(chan []byte, 1)
		attempt := 0
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			if _, err := readRTRQuery(conn); err != nil {
				return
			}
			attempt++
			if attempt == 1 {
				_, _ = conn.Write(slices.Concat(cacheResponsePDU(), key1, endOfDataTimed(20, 3600, 600)))
				var b [1]byte
				_, _ = conn.Read(b[:])
				return
			}
			// Second incremental sync repeats already-active {SKI, ASN, SPKI}.
			_, _ = conn.Write(slices.Concat(cacheResponsePDU(), key1))
			got, _ := io.ReadAll(conn)
			reported <- got
		})

		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		require.NoError(t, s.syncOnce())
		require.ErrorIs(t, s.syncOnce(), errRtrDuplicateRouterKey)
		var report []byte
		select {
		case report = <-reported:
		case <-time.After(3 * time.Second):
			t.Fatal("router did not send Error Report 7 on incremental duplicate Router Key")
		}
		require.Equal(t, 16+len(key1), len(report))
		assert.Equal(t, pduErrorRpt, report[1])
		assert.Equal(t, errDuplicateAnnouncement, binary.BigEndian.Uint16(report[2:4]))
		assert.Equal(t, key1, report[12:12+len(key1)])
	})
}

// TestRFC8210GenericErrorReportHasEmptyErroneousPDU proves RFC 8210 Section 5.11
// over real TCP RTR connections: when an error is generic (e.g., "Internal Error")
// and not associated with the PDU to which it is responding, the Erroneous PDU
// field MUST be empty and the Length of Encapsulated PDU field MUST be zero,
// whereas PDU-associated errors DO encapsulate the offending PDU.
//
// RFC requirement: RFC8210-5.11-2 positive -- when readLoop suffers a generic Internal Error (error code 1) over a live TCP session, the Error Report PDU emitted to the cache carries Length of Encapsulated PDU == 0, an empty Erroneous PDU field, Length of Error Text == 0, and exact total length 16 bytes.
// RFC requirement: RFC8210-5.11-2 negative -- when readLoop rejects a PDU-associated protocol error (duplicate Router Key drawing code 7, or unrecognized-version Cache Response drawing code 4), the Error Report PDU carries Length of Encapsulated PDU == len(badPDU) > 0 and the exact offending PDU bytes in Erroneous PDU.
func TestRFC8210GenericErrorReportHasEmptyErroneousPDU(t *testing.T) {
	t.Run("generic Internal Error sends empty Erroneous PDU and zero Length of Encapsulated PDU", func(t *testing.T) {
		_, _ = swapRPKIPanics(t)
		answers := make(chan cacheAnswer, 1)
		port, _ := serveRTR(t, syncingCache([4]byte{192, 0, 2, 0}, 65001, answers))

		s := newTestRTRSession(t, "127.0.0.1", port, 1, "", newROACache(), newASPACache(), make(chan struct{}))
		s.onROAChange = func() { panic("generic internal error") }

		require.Error(t, s.syncOnce())
		select {
		case ans := <-answers:
			require.NoError(t, ans.err)
			require.Len(t, ans.received, 16, "generic Error Report must have empty Erroneous PDU and zero text")
			assert.Equal(t, pduErrorRpt, ans.received[1])
			assert.Equal(t, errInternalError, binary.BigEndian.Uint16(ans.received[2:4]))
			assert.Equal(t, uint32(16), binary.BigEndian.Uint32(ans.received[4:8]))
			assert.Equal(t, uint32(0), binary.BigEndian.Uint32(ans.received[8:12]), "Length of Encapsulated PDU must be zero")
			assert.Equal(t, uint32(0), binary.BigEndian.Uint32(ans.received[12:16]), "Length of Error Text must be zero")
		case <-time.After(5 * time.Second):
			t.Fatal("cache never received generic Internal Error report")
		}
	})

	t.Run("PDU-associated error encapsulates the offending PDU with non-zero length", func(t *testing.T) {
		badResp := cacheResponsePDU()
		badResp[0] = 99 // unrecognized protocol version
		reported := make(chan []byte, 1)
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			if _, err := readRTRQuery(conn); err != nil {
				return
			}
			if _, err := conn.Write(badResp); err != nil {
				return
			}
			got, _ := io.ReadAll(conn)
			reported <- got
		})

		s := newTestRTRSession(t, "127.0.0.1", port, 1, "", newROACache(), newASPACache(), make(chan struct{}))
		require.ErrorIs(t, s.syncOnce(), errRtrUnsupportedProtocol)
		select {
		case report := <-reported:
			require.Equal(t, 16+len(badResp), len(report))
			assert.Equal(t, pduErrorRpt, report[1])
			assert.Equal(t, errUnsupportedVersion, binary.BigEndian.Uint16(report[2:4]))
			assert.Equal(t, uint32(len(badResp)), binary.BigEndian.Uint32(report[8:12]), "PDU-associated error must encapsulate offending PDU")
			assert.Equal(t, badResp, report[12:12+len(badResp)])
		case <-time.After(5 * time.Second):
			t.Fatal("cache never received Error Report for bad Cache Response")
		}
	})
}
