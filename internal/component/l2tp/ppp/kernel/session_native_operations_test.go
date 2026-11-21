// Design: docs/research/l2tpv2-ze-integration.md -- session exact mutation intent

package kernel

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"
)

type sessionNativeMutationModel struct {
	sessionTestMutationOperations
	setMTU     func(context.Context, int) error
	addPair    func(context.Context, netip.Prefix, netip.Prefix) error
	removePair func(context.Context, netip.Prefix, netip.Prefix) error
}

func (model *sessionNativeMutationModel) SetMTU(ctx context.Context, mtu int) error {
	return model.setMTU(ctx, mtu)
}
func (model *sessionNativeMutationModel) AddAddressP2P(ctx context.Context, local, peer netip.Prefix) error {
	return model.addPair(ctx, local, peer)
}
func (model *sessionNativeMutationModel) RemoveAddressP2P(ctx context.Context, local, peer netip.Prefix) error {
	return model.removePair(ctx, local, peer)
}

func TestSessionNativeMutationRequiresSeparateAuthority(t *testing.T) {
	_, handle, backend, _ := newSessionTest(t)
	owner, ok := handle.(*session)
	if !ok {
		t.Fatalf("session handle = %T, want *session", handle)
	}
	owner.mutations = nil
	if err := owner.SetMTU(1492); err == nil {
		t.Fatal("unbound handle mutated original backend by name")
	}
	if len(backend.calls) != 0 {
		t.Fatalf("unbound mutation reached old backend: %v", backend.calls)
	}
}

func TestSessionNativePartialPairRetainsFullWithdrawalIntent(t *testing.T) {
	_, handle, backend, _ := newSessionTest(t)
	owner, ok := handle.(*session)
	if !ok {
		t.Fatalf("session handle = %T, want *session", handle)
	}
	local := netip.MustParsePrefix("192.0.2.1/32")
	peer := netip.MustParsePrefix("192.0.2.2/30")
	cause := errors.New("settled partial address add")
	removed := false
	owner.mutations = &sessionNativeMutationModel{
		addPair: func(_ context.Context, gotLocal, gotPeer netip.Prefix) error {
			if gotLocal != local || gotPeer != peer {
				t.Errorf("changed exact add pair: %v %v", gotLocal, gotPeer)
			}
			return cause
		},
		removePair: func(_ context.Context, gotLocal, gotPeer netip.Prefix) error {
			removed = true
			if gotLocal != local || gotPeer != peer {
				t.Errorf("changed exact withdrawal pair: %v %v", gotLocal, gotPeer)
			}
			return nil
		},
	}
	if err := owner.AddAddressP2P(local, peer); !errors.Is(err, cause) {
		t.Fatalf("partial cause lost: %v", err)
	}
	if owned, ok := owner.addresses[local]; !ok || owned.peer != peer || owned.state != sessionAddressRetiring {
		t.Fatalf("partial intent lost: %+v", owner.addresses)
	}
	if err := owner.RemoveAddress(local); err != nil {
		t.Fatal(err)
	}
	if !removed || len(owner.addresses) != 0 {
		t.Fatalf("exact withdrawal incomplete: removed=%v intents=%v", removed, owner.addresses)
	}
	if len(backend.calls) != 0 {
		t.Fatalf("owned mutation fell back to backend labels: %v", backend.calls)
	}
}

func TestSessionNativeACKCannotBlockPhysicalClose(t *testing.T) {
	_, handle, _, _ := newSessionTest(t)
	owner, ok := handle.(*session)
	if !ok {
		t.Fatalf("session handle = %T, want *session", handle)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	owner.mutations = &sessionNativeMutationModel{setMTU: func(ctx context.Context, _ int) error {
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	mutation := make(chan error, 1)
	go func() { mutation <- owner.SetMTU(1492) }()
	<-entered
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	joined := false
	select {
	case err := <-closed:
		joined = true
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("physical session Close blocked behind native ACK")
	}
	close(release)
	<-mutation
	if !joined {
		<-closed
	}
}
