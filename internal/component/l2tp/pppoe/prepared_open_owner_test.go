// Design: docs/architecture/l2tp/bng-5-pppoe.md -- prepared access lifetime
// Related: subsystem_reload.go -- captured access session dispatch

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// TestInterfaceServerOpensWithPreparedOwner drives the actual server callback
// and requires its captured preparation owner to supply the sealed session.
// Registration carries only preparation: the captured owner's Open supplies
// this exact result. This is not native kernel proof.
func TestInterfaceServerOpensWithPreparedOwner(t *testing.T) {
	handle := &fixtureSessionHandle{identity: ppp.SessionIdentity{
		InterfaceName: "prepared-session", NativeIndex: 9,
	}}
	opened, err := dataplane.NewOpenedSession(&fixtureFrames{}, handle, fixtureEmptyTransport{})
	if err != nil {
		t.Fatal(err)
	}
	var access dataplane.Access
	// The caller MUST finish both session owners before closing their parent.
	// Keep each close independent so one failure cannot suppress another role.
	t.Cleanup(func() {
		if err := opened.Dataplane().Close(); err != nil {
			t.Error(err)
		}
		if err := opened.CloseTransport(); err != nil {
			t.Error(err)
		}
		if access != nil {
			if err := access.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	owner := &preparedDispatchOwner{result: opened}
	info := dataplane.AccessInfo{
		Device: "prepared-dp", ControlDevice: "prepared-cp", Namespace: "prepared-ns",
		DiscoveryIfIndex: 9, HWAddr: [6]byte{2, 0, 0, 0, 0, 9}, MTU: 1500,
	}
	access, err = dataplane.NewAccess(info, owner)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("prepared-owner-%d", accessConfigurationTestID.Add(1))
	err = dataplane.RegisterSessionDataplane(name,
		func(context.Context, string) (dataplane.Access, error) {
			return nil, errors.New("session dispatch must retain the original preparation")
		})
	if err != nil {
		t.Fatal(err)
	}
	backend, err := dataplane.LookupSessionDataplane(name)
	if err != nil {
		t.Fatal(err)
	}
	sub := NewSubsystem(Parameters{})
	sub.logger = slog.Default()
	server, err := sub.newInterfaceServer(&InterfaceConfig{Name: "uplink", MaxSessions: 1},
		&Parameters{}, backend, access)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), preparedDispatchContextKey{}, owner)
	request := dataplane.Request{
		AccessName: "uplink", Device: "caller-dp", ControlDevice: "caller-cp", Namespace: "caller-ns",
		SessionID: 42, PeerMAC: [6]byte{2, 0, 0, 0, 0, 42}, CaptureProxyLCP: true,
	}
	result, err := server.openSession(ctx, request)
	if err != nil {
		t.Fatalf("open through prepared access: %v", err)
	}
	if result != opened {
		t.Fatal("session dispatch changed the sealed provider result")
	}
	if result.Dataplane() != handle {
		t.Fatal("session dispatch changed the admitted data-plane owner")
	}
	if result.Frames() != opened.Frames() {
		t.Fatal("session dispatch changed the admitted frame channel")
	}
	if owner.opens != 1 {
		t.Fatalf("prepared owner opens=%d, want 1", owner.opens)
	}
	if owner.context != ctx {
		t.Fatal("prepared owner did not receive the original caller context")
	}
	request.Device = info.Device
	request.ControlDevice = info.ControlDevice
	request.Namespace = info.Namespace
	if owner.request != request {
		t.Fatalf("prepared owner request=%+v, want %+v", owner.request, request)
	}
	if owner.closes != 0 {
		t.Fatal("opening a session released its shared preparation owner")
	}
}

// preparedDispatchOwner lends an already constructed result to this test's
// server. The test MUST close both result roles before closing the access.
// Not safe for concurrent use; this fixture records one synchronous dispatch.
type preparedDispatchOwner struct {
	result  dataplane.OpenedSession
	context context.Context
	request dataplane.Request
	opens   int
	closes  int
}

func (owner *preparedDispatchOwner) Open(ctx context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
	owner.opens++
	owner.context = ctx
	owner.request = request
	return owner.result, nil
}

func (owner *preparedDispatchOwner) Close() error {
	owner.closes++
	return nil
}

type preparedDispatchContextKey struct{}
