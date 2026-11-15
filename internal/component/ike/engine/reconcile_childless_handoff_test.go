// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- childless ESP-group reload
// Related: reconcile.go -- retargetESPGroup and the owner-loop adoption decision
// Related: established.go -- publication of the established SA's owner

package engine

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestESPGroupReloadWaitsForChildlessOwnerHandoff keeps an established childless IKE
// SA when a reload reaches it before runEstablished publishes owner-loop ownership.
// Method: the owner can publish ownership only after receiving the real reload request,
// then calls adoptESPGroup. The channel rendezvous fixes the ordering without a sleep.
// The adopted group must preserve the IKE SA and the responder's inactive retry schedule.
//
// MUTATION: reject a reload while ownedSA is nil, and the request never reaches the
// owner, so the reload wrongly requires a session restart.
func TestESPGroupReloadWaitsForChildlessOwnerHandoff(t *testing.T) {
	_, responder, ps := establishPSK(t)
	ps.setSA(responder)
	clfMakeChildless(t, responder, ps)
	ps.childCreate.stop()
	ps.espReloads = make(chan espReload)
	ps.done = make(chan struct{})
	if ps.ownedSA.Load() != nil {
		t.Fatal("setup: the childless IKE SA already has a published owner")
	}
	initiatorSPI := responder.InitiatorSPI
	responderSPI := responder.ResponderSPI

	ownerContext, stopOwner := context.WithCancel(context.Background())
	t.Cleanup(func() {
		stopOwner()
		<-ps.done
	})
	go func() {
		defer close(ps.done)
		defer ps.ownedSA.Store(nil)
		select {
		case reload := <-ps.espReloads:
			ps.ownedSA.Store(responder)
			ps.adoptESPGroup(reload, slogutil.DiscardLogger())
		case <-ownerContext.Done():
			return
		}
		<-ownerContext.Done()
	}()

	reloaded := rcxReloadedESPGroup()
	if !ps.retargetESPGroup(reloaded) {
		t.Fatal("the reload required a restart before the childless owner could adopt it")
	}
	if ps.ownedSA.Load() != responder {
		t.Error("the owner did not adopt the established childless IKE SA")
	}
	if ps.getSA() != responder {
		t.Error("the reload replaced the established IKE SA")
	}
	if responder.State != StateEstablished {
		t.Errorf("the reload left the IKE SA in %v, want established", responder.State)
	}
	if responder.InitiatorSPI != initiatorSPI {
		t.Error("the reload changed the IKE initiator SPI")
	}
	if responder.ResponderSPI != responderSPI {
		t.Error("the reload changed the IKE responder SPI")
	}
	if !ps.getESPGroup().Equal(reloaded) {
		t.Error("the owner did not adopt the reloaded ESP group")
	}
	if ps.getChildSA() != nil {
		t.Error("the reload created a Child SA before a peer request")
	}
	if ps.getPendingSA() != nil {
		t.Error("the reload started a parallel IKE handshake")
	}
	if ps.pendingRekey != nil {
		t.Error("the reload started a rekey exchange")
	}
	if ps.childCreate.active {
		t.Error("the reload activated the responder's creation schedule")
	}
}
