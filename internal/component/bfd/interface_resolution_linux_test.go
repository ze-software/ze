//go:build integration && linux

// Design: docs/architecture/iface/logical-name-resolution.md -- consumers bind before kernel I/O
// VALIDATES: both BFD entry points consume the shared resolver's device answer,
// not the client's logical label. The backend seam supplies interface facts;
// session sharing, socket binding and UDP transmission are real consumers.
// PREVENTS: resolving only in Send, leaving pinned sockets or ingress keys on
// the logical label. Selector parsing and unresolved-selector refusal are
// covered by the iface resolver tests, not reimplemented by this fixture.
package bfd

import (
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	ifcomp "github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/userns"
)

const bfdLogicalLoopback = "bfd-logical-uplink"

// The metadata seam must describe the address userns.Enter's real lo carries.
var bfdBindingLocal = netip.MustParseAddr("127.0.0.1")

// bfdBindingBackend gives the resolver a logical name and a distinct OS name.
// Everything downstream must use the latter. Unused backend operations panic
// through the embedded interface, rather than silently succeeding.
type bfdBindingBackend struct {
	ifcomp.Backend
	link ifcomp.InterfaceInfo
}

func (b *bfdBindingBackend) GetInterface(name string) (*ifcomp.InterfaceInfo, error) {
	if name != bfdLogicalLoopback && name != b.link.OsName {
		return nil, fmt.Errorf("unknown fixture interface %q", name)
	}
	info := b.link
	return &info, nil
}

func (b *bfdBindingBackend) ListInterfaces() ([]ifcomp.InterfaceInfo, error) {
	return []ifcomp.InterfaceInfo{b.link}, nil
}

func (*bfdBindingBackend) Close() error { return nil }

func loadBFDBindingBackend(t *testing.T) {
	t.Helper()
	lo, err := net.InterfaceByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	backend := &bfdBindingBackend{link: ifcomp.InterfaceInfo{
		Name: "lo", OsName: "lo", Index: lo.Index, MTU: lo.MTU,
		Addresses: []ifcomp.AddrInfo{{Address: bfdBindingLocal.String(), PrefixLength: 8}},
	}}
	if err := ifcomp.RegisterBackend(t.Name(), func() (ifcomp.Backend, error) { return backend, nil }); err != nil {
		t.Fatal(err)
	}
	if err := ifcomp.LoadBackend(t.Name()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ifcomp.CloseBackend() })
}

// The pinned path creates the production socket itself. Binding to the logical
// label would fail because no such kernel device exists in this namespace.
func TestBFDPinnedAliasBindsKernelDeviceAndSharesSession(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	loadBFDBindingBackend(t)
	state := newRuntimeState()
	t.Cleanup(state.stopAll)
	entry, err := parseSingleHopSession(wireListener.String(), map[string]any{
		"local": wireSender.String(), "interface": bfdLogicalLoopback,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.applyPinned(&pluginConfig{enabled: true, sessions: []sessionConfig{entry}}); err != nil {
		t.Fatalf("apply pinned logical interface: %v", err)
	}
	lk := loopKey{vrf: api.DefaultVRF, mode: api.SingleHop}
	if got := state.loopDevices[lk]; got != "lo" {
		t.Fatalf("socket device = %q, want lo", got)
	}
	service := &pluginService{state: state}
	for _, name := range []string{bfdLogicalLoopback, "lo"} {
		if _, err := service.EnsureSession(api.SessionRequest{
			Peer: wireListener, Local: wireSender, Interface: name, Mode: api.SingleHop,
		}); err != nil {
			t.Fatalf("join via %s: %v", name, err)
		}
	}
	snap := state.loops[lk].Snapshot()
	if len(snap) != 1 || snap[0].Interface != "lo" || snap[0].Refcount != 3 {
		t.Fatalf("pinned and runtime clients must share one kernel-keyed session: %+v", snap)
	}
}

// A shared, unbound socket must pin packets to the resolved link. The peer
// reads a real Control datagram, so merely fixing snapshot output cannot pass.
func TestBFDRuntimeAliasSendsControlOnKernelLink(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	loadBFDBindingBackend(t)
	listener := wireListen(t, transport.UDPPortSingleHopControl)
	tr := &transport.UDP{
		Bind: netip.AddrPortFrom(bfdBindingLocal, transport.UDPPortSingleHopControl),
		Mode: api.SingleHop, VRF: api.DefaultVRF,
	}
	loop := engine.NewLoop(tr, clock.RealClock{})
	state := newRuntimeState()
	state.cfg = &pluginConfig{enabled: true}
	lk := loopKey{vrf: api.DefaultVRF, mode: api.SingleHop}
	state.loops[lk] = loop
	state.loopDevices[lk] = ""
	t.Cleanup(state.stopAll)
	handle, err := (&pluginService{state: state}).EnsureSession(api.SessionRequest{
		Peer: wireListener, Interface: bfdLogicalLoopback, Mode: api.SingleHop,
		DesiredMinTxInterval: 100_000, RequiredMinRxInterval: 100_000, DetectMult: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if key := handle.Key(); key.Interface != "lo" || key.Local != bfdBindingLocal {
		t.Fatalf("logical interface did not derive the kernel link's local address: %+v", key)
	}
	if err := loop.Start(); err != nil {
		t.Fatal(err)
	}
	if err := listener.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var buf [512]byte
	n, from, err := listener.ReadFromUDPAddrPort(buf[:])
	if err != nil {
		t.Fatalf("logical-interface session emitted no Control packet: %v", err)
	}
	if from.Addr() != bfdBindingLocal {
		t.Fatalf("Control source = %s, want %s", from, bfdBindingLocal)
	}
	if _, _, err := packet.ParseControl(buf[:n]); err != nil {
		t.Fatalf("received datagram is not a BFD Control packet: %v", err)
	}
	if err := ipv4.NewPacketConn(listener).SetTTL(255); err != nil {
		t.Fatal(err)
	}
	changes := handle.Subscribe()
	defer handle.Unsubscribe(changes)
	// YourDiscriminator remains zero: the first packet has to match the
	// kernel-sourced ingress name, not the client's logical label.
	reply := packet.Control{
		Version: packet.Version, State: packet.StateDown,
		DetectMult: 3, Length: packet.MandatoryLen, MyDiscriminator: 42,
		DesiredMinTxInterval: 100_000, RequiredMinRxInterval: 100_000,
	}
	var response [packet.MandatoryLen]byte
	reply.WriteTo(response[:], 0)
	if _, err := listener.WriteToUDPAddrPort(response[:], tr.Bind); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case change, ok := <-changes:
			if !ok {
				t.Fatal("session closed before accepting the peer's first packet")
			}
			if change.State == packet.StateInit {
				return
			}
		case <-deadline.C:
			t.Fatal("first packet on the resolved kernel link did not advance the session to Init")
		}
	}
}
