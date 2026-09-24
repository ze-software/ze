package rpki

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestRTRDataExpiryAndRenewal(t *testing.T) {
	// RFC requirement: RFC8210-6-2 positive -- the EOD lease expires both usable payload stores and notifies their consumers without another connection or PDU.
	// RFC requirement: RFC8210-6-2 negative -- a later completed EOD renews the lease; an incomplete response does not renew it or become usable on expiry.
	synctest.Test(t, func(t *testing.T) {
		s := newRTRSession("cache.invalid", 323, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		roaExpired := make(chan uint8, 1)
		aspaExpired := make(chan []uint32, 1)
		s.onROAChange = func() {
			if state := s.cache.Validate("192.0.2.0/24", 64500); state == ValidationNotFound {
				roaExpired <- state
			}
		}
		s.onASPAChange = func(changed []uint32) {
			if !s.aspaCache.isProvider(64500, 64501) {
				aspaExpired <- changed
			}
		}
		response, end := cacheResponsePDU(), endOfDataPDU()
		response[0], end[0] = 2, 2
		binary.BigEndian.PutUint32(end[8:12], 1)
		prefix := []byte{2, pduIPv4Prefix, 0, 0, 0, 0, 0, 20, 1, 24, 24, 0, 192, 0, 2, 0, 0, 0, 0xfb, 0xf4}
		s.startFullSync()
		applyPDU(t, s, response, false)
		applyPDU(t, s, prefix, false)
		applyPDU(t, s, aspaPDU(64500, 64501), false)
		applyPDU(t, s, end, true)
		defer s.dataLease.stop()
		assertLive := func() {
			t.Helper()
			if s.cache.Validate("192.0.2.0/24", 64500) != ValidationValid || !s.aspaCache.isProvider(64500, 64501) {
				t.Fatal("unexpired payloads were lost")
			}
		}
		time.Sleep(s.expireInterval - time.Second)
		assertLive()
		// A completed serial response with no changes still renews the data.
		applyPDU(t, s, response, false)
		applyPDU(t, s, end, true)
		time.Sleep(time.Second)
		assertLive()
		// A new response starts but never finishes before this lease expires.
		applyPDU(t, s, response, false)
		prefix[14] = 3
		applyPDU(t, s, prefix, false)
		time.Sleep(s.expireInterval - time.Second)
		synctest.Wait()
		if s.cache.Validate("192.0.2.0/24", 64500) != ValidationNotFound || s.aspaCache.isProvider(64500, 64501) {
			t.Fatal("expired payloads still authorize routes")
		}
		if s.cache.Validate("192.0.3.0/24", 64500) != ValidationNotFound || s.Snapshot().Synced {
			t.Fatal("an incomplete response renewed or published expired data")
		}
		select {
		case <-roaExpired:
		default:
			t.Error("VRP expiry did not notify route revalidation")
		}
		select {
		case customers := <-aspaExpired:
			if len(customers) != 1 || customers[0] != 64500 {
				t.Errorf("ASPA expiry notified customers %v, want the removed authorization for 64500", customers)
			}
		default:
			t.Error("ASPA expiry did not notify path revalidation")
		}
		// The late delta cannot resurrect just its additions without the base
		// that expired while the query was outstanding.
		hdr, err := parseHeader(end)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.handlePDU(hdr, end); !errors.Is(err, errRtrDataExpired) {
			t.Fatalf("late serial response error = %v", err)
		}
		if s.cache.Validate("192.0.3.0/24", 64500) != ValidationNotFound {
			t.Error("a delta republished data after its serial base expired")
		}
	})
}

func TestRTRSerialDeltaCannotRenewAnElapsedLease(t *testing.T) {
	// RFC requirement: RFC8210-6-2 negative -- a serial EOD after the lease deadline cannot publish its delta, even if the expiry callback has not run.
	synctest.Test(t, func(t *testing.T) {
		s := newRTRSession("cache.invalid", 323, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		response, end := cacheResponsePDU(), endOfDataPDU()
		response[0], end[0] = 2, 2
		binary.BigEndian.PutUint32(end[8:12], 1)
		prefix := []byte{2, pduIPv4Prefix, 0, 0, 0, 0, 0, 20, 1, 24, 24, 0, 192, 0, 2, 0, 0, 0, 0xfb, 0xf4}
		s.startFullSync()
		applyPDU(t, s, response, false)
		applyPDU(t, s, prefix, false)
		applyPDU(t, s, end, true)
		defer s.dataLease.stop()
		applyPDU(t, s, response, false)
		prefix[14] = 3
		applyPDU(t, s, prefix, false)
		// Delay timer delivery independently of the elapsed protocol clock.
		s.dataLease.timer.Stop()
		time.Sleep(s.expireInterval)
		binary.BigEndian.PutUint32(end[8:12], 2)
		hdr, err := parseHeader(end)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.handlePDU(hdr, end); !errors.Is(err, errRtrDataExpired) {
			t.Fatalf("elapsed-base serial response error = %v", err)
		}
		if s.cache.Validate("192.0.3.0/24", 64500) != ValidationNotFound || s.Snapshot().Synced {
			t.Fatal("a delayed expiry callback allowed the old lease to be renewed")
		}
	})
}

func TestRTRExpiredDataRequiresResetOnTheWire(t *testing.T) {
	for _, expiry := range []string{"callback completed", "callback pending"} {
		t.Run(expiry, func(t *testing.T) {
			fixture := newRTRTLSFixture(t)
			peer := startRTRTLSPeer(t, fixture.serverConfig(tls.VersionTLS13))
			s := parsedRTRTLSSession(t, "127.0.0.1", peer.port, rtrTLSConfigLeaves(), make(chan struct{}))
			s.pkiConfig = fixture.store
			for range 2 {
				if err := s.syncOnce(); err != nil {
					t.Fatal(err)
				}
				if s.cache.Validate("192.0.2.0/24", 64500) != ValidationValid {
					t.Fatal("cache did not supply the base for fast resynchronization")
				}
			}
			s.dataLease.mu.Lock()
			deadline := s.dataLease.deadline
			if expiry == "callback pending" {
				s.dataLease.timer.Stop()
				s.dataLease.deadline = time.Now().Add(-time.Second)
			}
			s.dataLease.mu.Unlock()
			if expiry == "callback completed" {
				s.dataLease.expire(deadline)
				if s.cache.Validate("192.0.2.0/24", 64500) != ValidationNotFound {
					t.Fatal("expiry did not remove the serial base")
				}
			}
			if err := s.syncOnce(); err != nil {
				t.Fatal(err)
			}
			observed := peer.stop()
			want := []uint8{pduResetQuery, pduSerialQuery, pduResetQuery}
			if len(observed) != len(want) {
				t.Fatalf("cache observed %d queries, want %d", len(observed), len(want))
			}
			for i, typ := range want {
				if observed[i].err != nil || observed[i].queryType != typ {
					t.Errorf("query %d: type=%d error=%v, want type=%d", i, observed[i].queryType, observed[i].err, typ)
				}
			}
			if s.cache.Validate("192.0.2.0/24", 64500) != ValidationValid || !s.aspaCache.isProvider(64500, 64501) {
				t.Error("the full reset did not restore the complete payload set")
			}
		})
	}
}
