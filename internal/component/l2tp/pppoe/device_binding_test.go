// Design: docs/architecture/iface/logical-name-resolution.md -- consumers resolve through iface
// Related: resolve.go -- InterfaceBinding, the one resolution an access interface keeps
//
// Goal: prove the access concentrator connects each session socket to the
// kernel device its access interface resolved to, not to the configured
// (logical) name, while show and events keep the configured name.
// Method: replace resolveInterfaceFn with a mapping in which logical "wan"
// names kernel device "eth7" (what `ethernet wan { os-name eth7 }` or a mac
// match selector gives), build the server through newInterfaceServer as boot
// and reload do, and drive a valid PADR through the server. The selected data-plane factory
// records its binding request and refuses before PADS or ownership publication.

package pppoe

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

var errInjectedCreate = errors.New("pppoe test: session socket refused")

// TestInterfaceServerSessionSocketUsesResolvedDevice fails if the configured
// name rather than the resolved kernel device reaches the AF_PPPOX connect.
func TestInterfaceServerSessionSocketUsesResolvedDevice(t *testing.T) {
	fixtureSelectBackend(t)
	restore := resolveInterfaceFn
	resolveInterfaceFn = func(name string) (InterfaceBinding, error) {
		if name != "wan" {
			return InterfaceBinding{}, errors.New("no such interface")
		}
		return InterfaceBinding{Device: "eth7", Ifindex: 9, HWAddr: [EthALen]byte{0x02, 0, 0, 0, 0, 0x09}, MTU: 1500}, nil
	}
	t.Cleanup(func() { resolveInterfaceFn = restore })

	s := &Subsystem{logger: slog.Default()}
	ic := InterfaceConfig{Name: "wan", MaxSessions: 1, MaxSessionsPerMAC: 1}
	backend, accesses := fixturePrepareConfiguration(t, []InterfaceConfig{ic})
	srv, err := s.newInterfaceServer(&ic, &Parameters{Enabled: true, CookieTimeout: DefaultCookieTimeout}, backend, accesses.accesses[ic.Name])
	if err != nil {
		t.Fatalf("newInterfaceServer: %v", err)
	}

	var device string
	var sent int
	var called bool
	srv.sendFrameFn = func([]byte) { sent++ }
	srv.openSession = func(_ context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
		called = true
		device = request.Device
		return nil, errInjectedCreate
	}
	peer := [EthALen]byte{0x02, 0, 0, 0, 0, 0x01}
	srv.handlePADR(&Packet{Code: CodePADR, SrcMAC: peer, DstMAC: srv.hwAddr, Tags: []Tag{
		{Type: TagACCookie, Value: GenerateCookie(srv.cookieKey, srv.hwAddr[:], peer[:], nil)},
		{Type: TagServiceName},
	}})
	if !called {
		t.Fatal("PADR did not reach the selected data-plane factory")
	}
	if sent != 0 || srv.sessions.Count() != 0 {
		t.Fatalf("factory refusal published %d frames and retained %d sessions, want neither", sent, srv.sessions.Count())
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
