// Design: docs/guide/rpki.md -- the RTR client's ASPA record rules, driven through the session's PDU handler.
// Related: rtr_session.go -- handlePDU, readLoop; aspa_cache.go -- ApplyDelta; rtr_pdu.go -- parseASPAPDU
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

// newHandlerRTRSession builds a session that is fed PDUs through handlePDU only and never dials.
// Its teardown stops the data lease only when the lease lock is free: a panic raised inside
// handlePDU's End of Data branch leaves that lock held, and a teardown that waited on it would
// hang the test binary instead of reporting the panic. The lease timer left behind then is the
// 7200 second Expire Interval, so it never fires inside the test binary's life. A held lock in a
// test that has not already failed is a lock the product leaked, so the teardown fails the test
// rather than skipping the stop in silence.
func newHandlerRTRSession(t *testing.T, aspas *aSPACache) *RTRSession {
	t.Helper()
	s := newRTRSession("127.0.0.1", 3323, 100, "", newROACache(), aspas, make(chan struct{}))
	t.Cleanup(func() {
		if s.dataLease == nil {
			return
		}
		if !s.dataLease.mu.TryLock() {
			if !t.Failed() {
				t.Error("the data lease lock (dataLease.mu) is still held after the test: handlePDU leaked it")
			}
			return
		}
		s.dataLease.mu.Unlock()
		s.dataLease.stop()
	})
	return s
}

// syncASPAPDUs runs one complete synchronization through the session's PDU handler: a Cache
// Response, the given ASPA PDUs, and an End of Data that applies them.
func syncASPAPDUs(t *testing.T, s *RTRSession, pdus ...[]byte) {
	t.Helper()
	applyPDU(t, s, cacheResponsePDU(), false)
	for _, pdu := range pdus {
		applyPDU(t, s, pdu, false)
	}
	applyPDU(t, s, endOfDataPDU(), true)
}

// aspaWithdrawPDU is an ASPA PDU with the announce/withdraw flag at 0 and no provider list.
func aspaWithdrawPDU(customer uint32) []byte {
	pdu := aspaPDU(customer)
	pdu[2] = 0
	return pdu
}

// TestASPAAnnouncementReplacesTheSameCustomer sends a customer's ASPA in one sync, then a
// second announcement in a later sync, and reads which providers the router then holds.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 5.12, "The router MUST see at most one ASPA from
// a particular cache for a particular Customer Autonomous System Number active at any time.",
// and "Receipt of an ASPA PDU announcement with the announce/withdraw flag set to 1 when the
// router already has an ASPA PDU with the same Customer Autonomous System Number from that cache
// replaces the previous one."
// PREVENTS: a second announcement merged into the first, leaving two ASPAs' providers active for
// one customer, or an announcement for one customer disturbing another customer's record.
func TestASPAAnnouncementReplacesTheSameCustomer(t *testing.T) {
	t.Run("a later announcement for the same customer replaces its record", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-4 positive -- after one sync announces customer 64500 with providers 100 and 200, a later sync announcing 64500 with provider 300 leaves 300 the only provider for 64500, 100 and 200 no longer providers, and one customer in the cache.
		aspas := newASPACache()
		s := newHandlerRTRSession(t, aspas)
		syncASPAPDUs(t, s, aspaPDU(64500, 100, 200))
		require.Equal(t, HopProviderPlus, aspas.checkPair(100, 64500), "the first announcement was not applied")
		syncASPAPDUs(t, s, aspaPDU(64500, 300))
		assert.Equal(t, HopProviderPlus, aspas.checkPair(300, 64500))
		assert.Equal(t, HopNotProviderPlus, aspas.checkPair(100, 64500), "the replaced ASPA's provider 100 is still active")
		assert.Equal(t, HopNotProviderPlus, aspas.checkPair(200, 64500), "the replaced ASPA's provider 200 is still active")
		assert.Equal(t, 1, aspas.count())
	})

	t.Run("an announcement for another customer leaves the first record alone", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-4 negative -- after one sync announces customer 64500 with provider 100, a later sync announcing customer 64501 with provider 300 replaces nothing: 64500 keeps provider 100 and does not gain 300, 64501 holds 300, and the cache holds two customers.
		aspas := newASPACache()
		s := newHandlerRTRSession(t, aspas)
		syncASPAPDUs(t, s, aspaPDU(64500, 100))
		syncASPAPDUs(t, s, aspaPDU(64501, 300))
		assert.Equal(t, HopProviderPlus, aspas.checkPair(100, 64500), "another customer's announcement replaced 64500")
		assert.Equal(t, HopNotProviderPlus, aspas.checkPair(300, 64500))
		assert.Equal(t, HopProviderPlus, aspas.checkPair(300, 64501))
		assert.Equal(t, 2, aspas.count())
	})
}

// TestASPAWithdrawalRemovesTheWholeRecord withdraws a customer's ASPA in a later sync, and sends
// a withdrawal that carries a provider list.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 5.12, "If the announce/withdraw flag is set to 0
// in an ASPA PDU, the customer AS of the ASPA record MUST be provided, there MUST be no Provider
// list, and the PDU Length MUST be 12. A router receiving this type of ASPA PDU (i.e., a
// withdrawal) MUST remove the entire ASPA record from that cache for that Customer AS."
// PREVENTS: a withdrawal that removes only some providers, or another customer's record, and a
// withdrawal carrying providers accepted as a withdrawal.
func TestASPAWithdrawalRemovesTheWholeRecord(t *testing.T) {
	t.Run("a 12-octet withdrawal removes the customer's record", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-5 positive -- a 12-octet withdrawal for customer 64500, held with providers 100 and 200, leaves 64500 with no record, so neither provider is attested, while customer 64501 keeps provider 300.
		aspas := newASPACache()
		s := newHandlerRTRSession(t, aspas)
		syncASPAPDUs(t, s, aspaPDU(64500, 100, 200), aspaPDU(64501, 300))
		require.True(t, aspas.hasRecord(64500), "the announcement was not applied")
		withdraw := aspaWithdrawPDU(64500)
		require.Len(t, withdraw, 12)
		syncASPAPDUs(t, s, withdraw)
		assert.False(t, aspas.hasRecord(64500), "the withdrawal left a record for 64500")
		assert.Equal(t, HopNoAttestation, aspas.checkPair(100, 64500))
		assert.Equal(t, HopNoAttestation, aspas.checkPair(200, 64500))
		assert.Equal(t, HopProviderPlus, aspas.checkPair(300, 64501), "the withdrawal removed another customer's record")
	})

	t.Run("a withdrawal with a provider list is refused and removes nothing", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-5 negative -- a 16-octet withdrawal for customer 64500 that carries provider 100 draws an Error Report with Error Code 9 encapsulating it over TCP, and through the handler it fails with the provider-list error and the End of Data that follows leaves 64500's record and provider 100 in place.
		bad := aspaPDU(64500, 100)
		bad[2] = 0
		reported := make(chan []byte, 1)
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
				return
			}
			if _, err := readRTRQuery(conn); err != nil {
				return
			}
			if _, err := conn.Write(slices.Concat(cacheResponsePDU(), bad)); err != nil {
				return
			}
			report := make([]byte, 16+len(bad))
			if _, err := io.ReadFull(conn, report); err != nil {
				return
			}
			reported <- report
		})
		overTCP := newASPACache()
		overTCP.Set(64500, []uint32{100})
		client := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), overTCP, make(chan struct{}))
		require.ErrorIs(t, client.syncOnce(), errASPAProviderList)
		var report []byte
		select {
		case report = <-reported:
		case <-time.After(3 * time.Second):
			t.Fatal("the router returned no Error Report for a withdrawal carrying providers")
		}
		assert.Equal(t, pduErrorRpt, report[1])
		assert.Equal(t, uint16(9), binary.BigEndian.Uint16(report[2:4]))
		assert.Equal(t, bad, report[12:12+len(bad)])

		aspas := newASPACache()
		s := newHandlerRTRSession(t, aspas)
		syncASPAPDUs(t, s, aspaPDU(64500, 100))
		applyPDU(t, s, cacheResponsePDU(), false)
		hdr, err := parseHeader(bad[:pduHeaderLen])
		require.NoError(t, err)
		_, err = s.handlePDU(hdr, bad)
		require.ErrorIs(t, err, errASPAProviderList)
		applyPDU(t, s, endOfDataPDU(), true)
		assert.True(t, aspas.hasRecord(64500), "a refused withdrawal removed the record")
		assert.Equal(t, HopProviderPlus, aspas.checkPair(100, 64500))
	})
}
