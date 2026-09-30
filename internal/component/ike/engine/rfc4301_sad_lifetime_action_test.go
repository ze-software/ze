// VALIDATES: RFC 4301 Section 4.4.2.1, "Lifetime of this SA: a time interval after
// which an SA must be replaced with a new SA (and new SPI) or terminated, plus an
// indication of which of these actions should occur." Ze's indication is the owner
// loop's mapping (maintainSA, established.go): the soft time of a Child SA's lifetime
// replaces it (startChildRekey proposes a new inbound SPI), the hard time terminates
// it (cleanupChild removes it and the loop ends). Each unit drives the real loop over
// one tick with the lifetime already past one of the two times.
// PREVENTS: the two actions swapped, so a soft expiry tears the SA down or a hard
// expiry keeps it alive behind a replacement exchange.

package engine

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// RFC requirement: RFC4301-4.4.2.1-11 positive -- a Child SA past its soft time and before its hard time is replaced: the owner loop sends a CREATE_CHILD_SA request proposing a new inbound SPI for that Child SA, and neither removes it nor ends.
func TestRFC4301SADLifetimeSoftTimeReplacesTheSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t, sa)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"

	dp := &rkyDP{}
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)

	done := make(chan error, 1)
	go func() {
		done <- ps.maintainSA(sa, nil, winSoftExpired(), nil,
			testIKEGroup(), NewSATable(), dp, myTr, nil, log)
	}()

	request := rtxRecv(t, peerTr)
	if request == nil {
		t.Fatal("a soft-expired Child SA produced no request")
	}
	// The tick that wrote the request runs to its end before the loop reads stopCh,
	// so a hard-expiry exit in that tick would already be the returned value.
	close(ps.stopCh)
	var loopErr error
	select {
	case loopErr = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the owner loop did not stop")
	}
	defer ps.pendingRekey.clear()

	if loopErr != nil {
		t.Fatalf("the owner loop ended with %v before it was stopped; a soft expiry must not terminate the SA", loopErr)
	}
	if hdr := parseMsg(t, request).Header; hdr.ExchangeType != wire.ExchangeCreateChildSA {
		t.Errorf("the request exchange = %d, want CREATE_CHILD_SA (%d)", hdr.ExchangeType, wire.ExchangeCreateChildSA)
	}
	pending := ps.pendingRekey
	if pending == nil {
		t.Fatal("no replacement exchange is outstanding after the soft time")
	}
	if pending.kind != rekeyChild {
		t.Errorf("the replacement is kind %d, want a Child SA rekey (%d)", pending.kind, rekeyChild)
	}
	if pending.oldChild != old {
		t.Error("the replacement does not name the soft-expired Child SA")
	}
	if pending.newInboundSPI == 0 || pending.newInboundSPI == old.InboundSPI {
		t.Errorf("the replacement proposes SPI %#x, want a new non-zero SPI (old %#x)", pending.newInboundSPI, old.InboundSPI)
	}
}

// RFC requirement: RFC4301-4.4.2.1-11 negative -- a Child SA past its hard time is terminated and not kept alive by a replacement: the owner loop removes its SAs from the dataplane, starts no rekey, sends no request, and ends with the timeout error.
func TestRFC4301SADLifetimeHardTimeTerminatesTheSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t, sa)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := sa.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the SA has no resolvable peer address")
	}

	dp := &rkyDP{}
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)

	// The soft time is far away, so only the hard branch can act on this tick.
	lifetime := &lifetimeState{
		softTime: time.Now().Add(time.Hour),
		hardTime: time.Now().Add(-time.Second),
	}
	done := make(chan error, 1)
	go func() {
		done <- ps.maintainSA(sa, nil, lifetime, nil,
			testIKEGroup(), NewSATable(), dp, myTr, nil, log)
	}()

	var loopErr error
	select {
	case loopErr = <-done:
	case <-time.After(5 * time.Second):
		close(ps.stopCh)
		<-done
		ps.pendingRekey.clear()
		t.Fatal("the owner loop kept a hard-expired Child SA alive")
	}

	if !errors.Is(loopErr, errTimeout) {
		t.Errorf("the owner loop ended with %v, want the timeout error of a hard expiry", loopErr)
	}
	if ps.pendingRekey != nil {
		ps.pendingRekey.clear()
		t.Error("a hard-expired Child SA started a replacement exchange")
	}
	if ps.getChildSA() != nil {
		t.Error("the hard-expired Child SA is still the session's Child SA")
	}
	if !slices.Contains(dp.removed, old.InboundSPI) {
		t.Errorf("the dataplane removals %#x do not include the inbound SPI %#x", dp.removed, old.InboundSPI)
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "a hard-expired Child SA")
}
