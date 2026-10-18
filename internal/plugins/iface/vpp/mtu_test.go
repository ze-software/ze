// VPP backend SetMTU: which VPP message carries the MTU for each kind of
// interface, and the refusal an MTU above the link's frame limit meets.
package ifacevpp

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
)

// mtuChannel is an api.Channel fake that answers SwInterfaceDump with one
// interface's details and HwInterfaceSetMtu with hwErr, the way GoVPP reports
// a non-zero retval: as an api.VPPApiError from ReceiveReply. It records every
// request so a test can assert which messages reached VPP.
type mtuChannel struct {
	details  interfaces.SwInterfaceDetails
	hwErr    error
	requests []api.Message
}

var _ api.Channel = (*mtuChannel)(nil)

func (c *mtuChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.requests = append(c.requests, msg)
	return &mtuReqCtx{ch: c}
}

func (c *mtuChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.requests = append(c.requests, msg)
	return &mtuMultiCtx{details: c.details}
}

func (c *mtuChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *mtuChannel) SetReplyTimeout(time.Duration) {}

func (c *mtuChannel) CheckCompatiblity(...api.Message) error { return nil }

func (c *mtuChannel) Close() {}

type mtuReqCtx struct{ ch *mtuChannel }

func (r *mtuReqCtx) ReceiveReply(msg api.Message) error {
	if _, ok := msg.(*interfaces.HwInterfaceSetMtuReply); ok {
		return r.ch.hwErr
	}
	return nil
}

type mtuMultiCtx struct {
	details interfaces.SwInterfaceDetails
	sent    bool
}

func (m *mtuMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.sent {
		return true, nil
	}
	if dst, ok := msg.(*interfaces.SwInterfaceDetails); ok {
		*dst = m.details
	}
	m.sent = true
	return false, nil
}

// sent returns the requests of type T the backend sent, in order.
func sent[T api.Message](c *mtuChannel) []T {
	var out []T
	for _, m := range c.requests {
		if t, ok := m.(T); ok {
			out = append(out, t)
		}
	}
	return out
}

func newMTUBackend(details interfaces.SwInterfaceDetails, hwErr error) (*vppBackendImpl, *mtuChannel) {
	ch := &mtuChannel{details: details, hwErr: hwErr}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", uint32(details.SwIfIndex), "xe0")
	return b, ch
}

func hardwareDetails(linkMTU uint16) interfaces.SwInterfaceDetails {
	return interfaces.SwInterfaceDetails{SwIfIndex: 3, SupSwIfIndex: 3, Type: interface_types.IF_API_TYPE_HARDWARE, LinkMtu: linkMTU}
}

// TestVPPSetMTURaisesTheLinkOfAHardwareInterface proves the MTU of a NIC goes
// through HwInterfaceSetMtu, which programs the NIC's frame size and makes the
// driver judge it. Before the fix only SwInterfaceSetMtu was sent: VPP 25.10's
// handler stores those protocol MTUs and returns 0 without reading the link,
// so the NIC kept its frame size whatever the operator asked for. The protocol
// MTUs follow with 0 per protocol, so each tracks the L3 value that VPP's own
// hardware MTU walk keeps on every sub-interface of the NIC.
func TestVPPSetMTURaisesTheLinkOfAHardwareInterface(t *testing.T) {
	b, ch := newMTUBackend(hardwareDetails(1500), nil)
	if err := b.SetMTU("xe0", 9000); err != nil {
		t.Fatalf("SetMTU: %v", err)
	}
	hw := sent[*interfaces.HwInterfaceSetMtu](ch)
	if len(hw) != 1 || hw[0].Mtu != 9000 || hw[0].SwIfIndex != 3 {
		t.Fatalf("HwInterfaceSetMtu requests = %+v, want one for index 3 with mtu 9000", hw)
	}
	sw := sent[*interfaces.SwInterfaceSetMtu](ch)
	if len(sw) != 1 {
		t.Fatalf("SwInterfaceSetMtu requests = %d, want 1", len(sw))
	}
	want := []uint32{9000, 0, 0, 0}
	if fmt.Sprint(sw[0].Mtu) != fmt.Sprint(want) {
		t.Fatalf("SwInterfaceSetMtu mtu = %v, want %v", sw[0].Mtu, want)
	}
}

// TestVPPSetMTURefusesAboveTheNICLimit proves an MTU the NIC driver refuses,
// above the link MTU VPP reports, fails with an error naming the limit and the
// command that fixes it, and that no protocol MTU larger than the link is
// written. This is the journal row of 2026-10-18 (silent-fall-through.md):
// mtu 9000 on a NIC capped at 1500 used to be accepted in silence.
func TestVPPSetMTURefusesAboveTheNICLimit(t *testing.T) {
	b, ch := newMTUBackend(hardwareDetails(1500), api.INVALID_VALUE)
	err := b.SetMTU("xe0", 9000)
	if err == nil {
		t.Fatal("SetMTU 9000 on a 1500 link the driver will not raise: want an error")
	}
	for _, part := range []string{"xe0", "9000", "1500", "set interface ethernet xe0 mtu 1500"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not name %q", err, part)
		}
	}
	if got := sent[*interfaces.SwInterfaceSetMtu](ch); len(got) != 0 {
		t.Fatalf("a refused MTU still wrote protocol MTUs: %+v", got)
	}
}

// TestVPPSetMTUOnAFixedFrameInterface proves an interface whose frame size VPP
// cannot change (a loopback answers UNSUPPORTED, a GRE tunnel FEATURE_DISABLED)
// still takes an MTU the link carries, through the protocol MTU alone. A link
// MTU of 0 is a device that states no frame limit, as VPP's tunnels do.
func TestVPPSetMTUOnAFixedFrameInterface(t *testing.T) {
	tests := []struct {
		name    string
		linkMTU uint16
		hwErr   error
		mtu     int
	}{
		{"loopback below its link", 9000, api.UNSUPPORTED, 1500},
		{"tunnel with no stated link limit", 0, api.FEATURE_DISABLED, 1476},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, ch := newMTUBackend(hardwareDetails(tt.linkMTU), tt.hwErr)
			if err := b.SetMTU("xe0", tt.mtu); err != nil {
				t.Fatalf("SetMTU: %v", err)
			}
			sw := sent[*interfaces.SwInterfaceSetMtu](ch)
			if len(sw) != 1 || sw[0].Mtu[0] != uint32(tt.mtu) {
				t.Fatalf("SwInterfaceSetMtu requests = %+v, want one with L3 %d", sw, tt.mtu)
			}
		})
	}
}

// TestVPPSetMTUOnASubInterface proves a VLAN sub-interface never sends
// HwInterfaceSetMtu (VPP refuses it for anything but a hardware interface),
// takes an MTU its parent's link carries, and refuses a larger one naming the
// link MTU, which VPP reports from the parent NIC.
func TestVPPSetMTUOnASubInterface(t *testing.T) {
	sub := interfaces.SwInterfaceDetails{SwIfIndex: 7, SupSwIfIndex: 3, Type: interface_types.IF_API_TYPE_SUB, LinkMtu: 1500}

	b, ch := newMTUBackend(sub, nil)
	if err := b.SetMTU("xe0", 1500); err != nil {
		t.Fatalf("SetMTU 1500: %v", err)
	}
	if got := sent[*interfaces.HwInterfaceSetMtu](ch); len(got) != 0 {
		t.Fatalf("a sub-interface sent HwInterfaceSetMtu: %+v", got)
	}
	if got := sent[*interfaces.SwInterfaceSetMtu](ch); len(got) != 1 {
		t.Fatalf("SwInterfaceSetMtu requests = %d, want 1", len(got))
	}

	b, ch = newMTUBackend(sub, nil)
	err := b.SetMTU("xe0", 9000)
	if err == nil || !strings.Contains(err.Error(), "1500") {
		t.Fatalf("SetMTU 9000 over a 1500 parent link: err = %v, want a refusal naming 1500", err)
	}
	if got := sent[*interfaces.SwInterfaceSetMtu](ch); len(got) != 0 {
		t.Fatalf("a refused MTU still wrote protocol MTUs: %+v", got)
	}
}

// TestVPPSetMTUTransportError proves a failure to reach VPP is returned as it
// is, never read as the driver refusing the frame size.
func TestVPPSetMTUTransportError(t *testing.T) {
	lost := errors.New("connection lost")
	b, ch := newMTUBackend(hardwareDetails(9000), lost)
	err := b.SetMTU("xe0", 1500)
	if !errors.Is(err, lost) {
		t.Fatalf("SetMTU error = %v, want it to wrap %v", err, lost)
	}
	if got := sent[*interfaces.SwInterfaceSetMtu](ch); len(got) != 0 {
		t.Fatalf("protocol MTUs written after a transport error: %+v", got)
	}
}
