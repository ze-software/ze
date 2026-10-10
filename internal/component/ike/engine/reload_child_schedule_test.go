// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the childless IKE SA
// Related: reconcile.go -- reconcilePeers and adoptESPGroup on an esp-group reload
// Related: create_child.go -- childCreateSchedule

package engine

import (
	"log/slog"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestReloadOfMissingESPGroupRestartsChildlessSession checks the espResolved guard of
// reconcilePeers.
//
// Goal: the childless exception keeps the IKE SA only for an esp-group the new
// configuration defines. A peer whose esp-group is gone from the reload resolves to the
// zero group, and a restarted daemon would refuse to start it, so the running session
// MUST stop rather than adopt an esp-group with no proposals. Method: a childless session
// runs its owner loop; reconcilePeers is handed the same peer and ike-group, with the
// peer's esp-group name absent from the esp-group map. The active map MUST NOT hold the
// session afterwards, and the session MUST NOT have adopted the zero group.
//
// PREVENTS: a childless IKE SA kept on an esp-group that no longer exists, which no
// CREATE_CHILD_SA can ever satisfy.
func TestReloadOfMissingESPGroupRestartsChildlessSession(t *testing.T) {
	link := errLink(t)
	ps := link.ps
	ps.peerName = "reload-missing-esp"
	clfMakeChildless(t, link.resp, ps)
	active := map[string]*PeerSession{ps.peerName: ps}
	rcxStopRestarted(t, active, ps)
	rcxRunLoop(t, link, &sepBus{})

	cfg := rcxConfig(ps, ps.ikeGroup, testESPGroup())
	cfg.ESPGroups = map[string]ipsec.ESPGroup{}
	reconcilePeers(cfg, active, NewSATable(), nil, nil, nil, slog.Default())
	if active[ps.peerName] == ps {
		t.Fatal("a reload whose esp-group no longer exists kept the childless session")
	}
	if got := ps.getESPGroup(); len(got.Proposals) == 0 {
		t.Error("the stopped session adopted the zero esp-group of the missing name")
	}
}

// TestESPGroupReloadRestartsTheCreationSchedule checks that an adopted esp-group starts
// the creation schedule over.
//
// Goal: an operator who fixes the esp-group after the peer refused the Child SA for some
// time is answered on the first wait, not the one the refusals of the old proposals
// doubled up to childCreateMax. Method: a childless initiator session whose schedule has
// backed off to childCreateMax adopts a reloaded esp-group on the owner loop
// (adoptESPGroup). The schedule MUST then hold childCreateFirst and be due no later than
// childCreateFirst from the reload. A responder session, whose schedule is inactive,
// MUST stay inactive: only the IKE SA initiator retries.
//
// PREVENTS: a reload that fixes the mismatch and then waits up to five minutes for the
// next attempt.
func TestESPGroupReloadRestartsTheCreationSchedule(t *testing.T) {
	t.Run("initiator backed off", func(t *testing.T) {
		link := errLink(t)
		ps := link.ps
		clfMakeChildless(t, link.resp, ps)
		start := time.Now()
		ps.childCreate.start(start)
		for range 8 {
			ps.childCreate.refused(start)
		}
		if ps.childCreate.wait != childCreateMax {
			t.Fatalf("setup: the wait is %v, want it backed off to %v", ps.childCreate.wait, childCreateMax)
		}

		reload := espReload{group: rcxReloadedESPGroup(), reply: make(chan bool, 1)}
		before := time.Now()
		ps.adoptESPGroup(reload, slogutil.DiscardLogger())
		if !<-reload.reply {
			t.Fatal("setup: the childless session refused the reloaded esp-group")
		}
		if ps.childCreate.wait != childCreateFirst {
			t.Errorf("the creation wait is %v after the reload, want %v", ps.childCreate.wait, childCreateFirst)
		}
		if ps.childCreate.next.After(time.Now().Add(childCreateFirst)) {
			t.Errorf("the next attempt is at %v, later than %v after the reload", ps.childCreate.next.Sub(before), childCreateFirst)
		}
		if !ps.childCreate.active {
			t.Error("the reload stopped the creation schedule")
		}
	})
	t.Run("responder inactive", func(t *testing.T) {
		link := errLink(t)
		ps := link.ps
		clfMakeChildless(t, link.resp, ps)
		ps.childCreate.stop()

		reload := espReload{group: rcxReloadedESPGroup(), reply: make(chan bool, 1)}
		ps.adoptESPGroup(reload, slogutil.DiscardLogger())
		if !<-reload.reply {
			t.Fatal("setup: the childless session refused the reloaded esp-group")
		}
		if ps.childCreate.active {
			t.Error("the reload started a creation schedule on a session that does not retry")
		}
	})
}
