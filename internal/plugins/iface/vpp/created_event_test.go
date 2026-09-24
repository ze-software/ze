// Design: docs/architecture/config/apply-ordering.md -- settlement on (interface, created)
// Related: monitor.go -- emitCreated
// Related: ifacevpp.go -- CreateDummy, CreateVLAN
// Related: tunnel.go -- createGRETunnel, createIPIPTunnel
// Related: vxlan.go -- createVxlanTunnel
// Related: wireguard.go -- ensureWireguardInterface

package ifacevpp

import (
	"encoding/json"
	"testing"

	"go.fd.io/govpp/binapi/interface_types"

	"github.com/ze-software/ze/internal/component/iface"
)

// createdBackend returns a backend on a scripted channel whose monitor holds
// bus, so the emit path runs exactly as it does after StartMonitor.
func createdBackend(ch *progChannel, bus *recordingBus) *vppBackendImpl {
	b := newLCPBackend(ch)
	b.mon = &monitor{b: b, eventBus: bus}
	return b
}

// TestCreateEmitsCreatedEvent proves that every VPP create path announces the
// interface it made, in the payload shape ifacenetlink uses.
//
// VALIDATES: each create emits one (interface, created) naming the full ze
// interface name, the SwIfIndex VPP returned, and the ze interface type, once
// the create has succeeded.
// PREVENTS: a config reload that adds an interface under the VPP backend
// waiting for an event VPP never sends, timing out after 5s, and rolling the
// whole reload back (settlement rule iface-add-interface-settles-created).
func TestCreateEmitsCreatedEvent(t *testing.T) {
	const idx = interface_types.InterfaceIndex(17)
	tests := []struct {
		name     string
		create   func(b *vppBackendImpl) error
		wantName string
		wantType string
	}{
		{
			name:     "loopback",
			create:   func(b *vppBackendImpl) error { return b.CreateDummy("lo1") },
			wantName: "lo1",
			wantType: "dummy",
		},
		{
			name: "vlan",
			create: func(b *vppBackendImpl) error {
				b.names.Add("xe0", 3, "xe0")
				return b.CreateVLAN(iface.VLANSpec{Parent: "xe0", VLANID: 100})
			},
			wantName: "xe0.100",
			wantType: "vlan",
		},
		{
			name: "gre",
			create: func(b *vppBackendImpl) error {
				return b.CreateTunnel(iface.TunnelSpec{Name: "gre1", Kind: iface.TunnelKindGRE, LocalAddress: "192.0.2.1", RemoteAddress: "192.0.2.2"})
			},
			wantName: "gre1",
			wantType: "gre",
		},
		{
			name: "ipip",
			create: func(b *vppBackendImpl) error {
				return b.CreateTunnel(iface.TunnelSpec{Name: "ipip1", Kind: iface.TunnelKindIPIP, LocalAddress: "192.0.2.1", RemoteAddress: "192.0.2.2"})
			},
			wantName: "ipip1",
			wantType: "ipip",
		},
		{
			name: "vxlan",
			create: func(b *vppBackendImpl) error {
				return b.CreateTunnel(iface.TunnelSpec{Name: "vx1", Kind: iface.TunnelKindVxlan, LocalAddress: "192.0.2.1", RemoteAddress: "192.0.2.2", VNI: 42, VNISet: true})
			},
			wantName: "vx1",
			wantType: "vxlan",
		},
		{
			name: "wireguard",
			create: func(b *vppBackendImpl) error {
				return b.ConfigureWireguardDevice(iface.WireguardSpec{Name: "wg1", ListenPort: 51820})
			},
			wantName: "wg1",
			wantType: "wireguard",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &progChannel{swIfIndex: idx, loopbackIndex: idx}
			bus := &recordingBus{}
			b := createdBackend(ch, bus)

			if err := tt.create(b); err != nil {
				t.Fatalf("create: %v", err)
			}
			if bus.len() != 1 {
				t.Fatalf("events: got %d, want 1 (created)", bus.len())
			}
			ev := bus.at(0)
			if ev.Namespace != "interface" || ev.Type != "created" {
				t.Fatalf("event: got %s/%s, want interface/created", ev.Namespace, ev.Type)
			}
			var got linkEventPayload
			if err := json.Unmarshal([]byte(ev.Payload), &got); err != nil {
				t.Fatalf("payload %q: %v", ev.Payload, err)
			}
			want := linkEventPayload{Name: tt.wantName, Type: tt.wantType, Index: int(idx)}
			if got != want {
				t.Errorf("payload: got %+v, want %+v", got, want)
			}
		})
	}
}

// TestFailedCreateEmitsNoCreatedEvent proves a create VPP refuses announces
// nothing, so a settlement waiter never reads a refused create as settled.
//
// VALIDATES: a nonzero retval on create_loopback emits no event.
// PREVENTS: an (interface, created) for an interface that does not exist.
func TestFailedCreateEmitsNoCreatedEvent(t *testing.T) {
	ch := &progChannel{retval: -1}
	bus := &recordingBus{}
	b := createdBackend(ch, bus)

	if err := b.CreateDummy("lo1"); err == nil {
		t.Fatal("CreateDummy: want error for retval -1")
	}
	if bus.len() != 0 {
		t.Fatalf("events: got %d, want 0", bus.len())
	}
}
