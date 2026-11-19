// Design: docs/architecture/l2tp/bng-5-pppoe.md -- discoverable VPP provider
// Related: register.go -- composition-root side effect

package pppoevpp

import (
	"context"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	vppdataplane "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp"
)

// Importing the composition wrapper must install the exact VPP backend.
// Preparation and prepared session admission both refuse an absent context.
func TestVPPProviderRegisters(t *testing.T) {
	backend, err := dataplane.LookupSessionDataplane(vppdataplane.Name)
	if err != nil {
		t.Fatal(err)
	}
	if backend.Name() != vppdataplane.Name {
		t.Fatalf("selected provider=%q", backend.Name())
	}
	access, err := dataplane.NewAccess(dataplane.AccessInfo{
		Device: "Ethernet0", ControlDevice: "cp0", DiscoveryIfIndex: 1,
		HWAddr: [6]byte{2, 0, 0, 0, 0, 1}, MTU: 1500,
	}, &registrationAccessOwner{test: t})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	if opened, err := access.Open(nil, dataplane.Request{}); err == nil || opened != nil { //nolint:staticcheck // SA1012: intentionally nil to prove refusal before the session owner is invoked.
		t.Fatalf("session construction bypassed prepared access admission: %v, %v", opened, err)
	}
	if access, err := backend.Prepare(nil, "wan"); err == nil || access != nil { //nolint:staticcheck // SA1012: intentionally nil to prove refusal before the provider's preparation is invoked.
		t.Fatalf("access preparation bypassed registry admission: %v, %v", access, err)
	}
}

type registrationAccessOwner struct{ test *testing.T }

func (owner *registrationAccessOwner) Open(context.Context, dataplane.Request) (dataplane.OpenedSession, error) {
	owner.test.Fatal("nil context reached the session owner")
	return nil, errors.New("unreachable session owner")
}

func (*registrationAccessOwner) Close() error { return nil }
