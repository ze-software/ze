// Design: docs/architecture/iface/logical-name-resolution.md -- consumers resolve through iface
// Related: resolve.go -- InterfaceBinding, the one resolution an access interface keeps
//
// Goal: prove the access concentrator connects each session socket to the
// kernel device its access interface resolved to, not to the configured
// (logical) name, while show and events keep the configured name.
// Method: replace resolveInterfaceFn with a mapping in which logical "wan"
// names kernel device "eth7" (what `ethernet wan { os-name eth7 }` or a mac
// match selector gives), build the server through newInterfaceServer as boot
// and reload do, and read the device name the session socket seam receives.

package pppoe

import (
	"errors"
	"log/slog"
	"testing"
)

var errInjectedCreate = errors.New("pppoe test: session socket refused")

// TestInterfaceServerSessionSocketUsesResolvedDevice fails if the configured
// name rather than the resolved kernel device reaches the AF_PPPOX connect.
func TestInterfaceServerSessionSocketUsesResolvedDevice(t *testing.T) {
	restore := resolveInterfaceFn
	resolveInterfaceFn = func(name string) (InterfaceBinding, error) {
		if name != "wan" {
			return InterfaceBinding{}, errors.New("no such interface")
		}
		return InterfaceBinding{Device: "eth7", Ifindex: 9, HWAddr: [EthALen]byte{0x02, 0, 0, 0, 0, 0x09}, MTU: 1500}, nil
	}
	t.Cleanup(func() { resolveInterfaceFn = restore })

	s := &Subsystem{logger: slog.Default()}
	srv, err := s.newInterfaceServer(&InterfaceConfig{Name: "wan", MaxSessions: 1}, &Parameters{Enabled: true})
	if err != nil {
		t.Fatalf("newInterfaceServer: %v", err)
	}

	var device string
	srv.pppoeCreateFn = func(dev string, _ uint16, _ [EthALen]byte) (int, error) {
		device = dev
		return -1, errInjectedCreate
	}
	if _, err := srv.createTransport(0x1234, [EthALen]byte{0x02, 0, 0, 0, 0, 0x01}); !errors.Is(err, errInjectedCreate) {
		t.Fatalf("createTransport error = %v, want the injected refusal", err)
	}
	if device != "eth7" {
		t.Fatalf("session socket device = %q, want the resolved kernel device %q", device, "eth7")
	}
	if srv.ifIndex != 9 {
		t.Fatalf("discovery ifindex = %d, want 9 from the same binding", srv.ifIndex)
	}
	if srv.ifName != "wan" {
		t.Fatalf("reported interface = %q, want the configured name %q", srv.ifName, "wan")
	}
}
