// Design: docs/architecture/ike/ipsec-14-responder.md -- the Configuration payload in IKE_AUTH
// Related: cp.go -- releaseVirtualIP and the lease slot these tests drive
// VALIDATES: a pool-bound IKE SA's lease is released on the real close paths, and the
// lease slot is read safely while the owner goroutine releases it.
// PREVENTS: a lease that outlives its IKE SA, and a `show` that races the release.

package engine

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// closableLeasedResponder brings a pool-bound responder SA up through the real IKE_AUTH with a
// CFG_REQUEST and returns it with its session and the registry state.
func closableLeasedResponder(t *testing.T) (*SA, *PeerSession, *virtualIPState) {
	t.Helper()
	vip := cpVIP(t, cpPool())
	ini, resp, ps := cpResponder(t, vip, "ze", true)
	cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{})))
	if got := resp.VirtualIPs(); len(got) != 1 {
		t.Fatalf("the responder holds %v, want one lease", got)
	}
	return resp, ps, vip
}

// TestLeaseReleasedWhenRunEstablishedExits drives the owner loop's exit: runEstablished
// is entered with its session already stopped, so its deferred close (forgetKeys on the
// owned SA) is the only thing that can release the lease.
func TestLeaseReleasedWhenRunEstablishedExits(t *testing.T) {
	resp, ps, vip := closableLeasedResponder(t)
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	close(ps.stopCh)

	done := make(chan error, 1)
	go func() {
		done <- ps.runEstablished(resp, ps.peerCfg, testIKEGroup(), NewSATable(), nil, nil, slogutil.DiscardLogger())
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runEstablished did not return after the session was stopped")
	}
	if n := vip.leases.Held("rw"); n != 0 {
		t.Fatalf("%d leases held after the owner loop exited, want 0", n)
	}
	if got := resp.VirtualIPs(); got != nil {
		t.Fatalf("the closed SA still reports %v", got)
	}
}

// TestLeaseReleasedOnResponderDeadBranch drives runResponder's StateDead branch: the
// published SA is dead, so the poll loop removes it and forgets its keys, which must
// release the lease before the session is stopped.
func TestLeaseReleasedOnResponderDeadBranch(t *testing.T) {
	resp, ps, vip := closableLeasedResponder(t)
	resp.State = StateDead
	ps.stopCh = make(chan struct{})
	ps.setSA(resp)

	done := make(chan error, 1)
	go func() {
		done <- ps.runResponder(ipsec.SiteToSitePeer{}, ipsec.IKEGroup{}, NewSATable(), nil, nil, slogutil.DiscardLogger())
	}()
	deadline := time.Now().Add(5 * time.Second)
	for vip.leases.Held("rw") != 0 {
		if time.Now().After(deadline) {
			close(ps.stopCh)
			t.Fatal("the StateDead branch did not release the lease within 5s")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(ps.stopCh)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runResponder did not return after the session was stopped")
	}
	if ps.getSA() != nil {
		t.Fatal("the dead SA is still published")
	}
	if got := resp.VirtualIPs(); got != nil {
		t.Fatalf("the dead SA still reports %v", got)
	}
}

// TestVirtualIPsSafeAgainstRelease reads the lease slot from a second goroutine, as
// `show vpn ipsec sa` does, while the owner releases it. Run under -race, an unguarded
// slot is reported; without -race, a read that met the slot half-cleared would
// dereference nil.
func TestVirtualIPsSafeAgainstRelease(t *testing.T) {
	resp, _, vip := closableLeasedResponder(t)
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 1000 {
			_ = resp.VirtualIPs()
		}
	})
	resp.releaseVirtualIP()
	wg.Wait()
	if n := vip.leases.Held("rw"); n != 0 {
		t.Fatalf("%d leases held after release, want 0", n)
	}
}
