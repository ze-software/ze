// VPP backend MTU against a parent: the MTU a new VLAN sub-interface takes
// from its parent, and a link MTU raise on a driver that changes the frame
// size of a stopped port only.
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

	"github.com/ze-software/ze/internal/component/iface"
)

// parentChannel is an api.Channel fake that holds VPP's details for several
// interfaces by index and models the answers this file's tests need:
// CreateVlanSubif creates index subIdx, SwInterfaceSetMtu answers swErr,
// DeleteLoopback refuses (the index is a sub-interface), and when busyWhileUp
// is set HwInterfaceSetMtu answers BUSY while the interface is admin up, the
// way VPP relays i40e's -EBUSY for a started port. SwInterfaceSetFlags moves
// the admin state and answers upErr when it sets the interface up. Every
// request is recorded so a test can assert the order VPP saw.
type parentChannel struct {
	details     map[interface_types.InterfaceIndex]interfaces.SwInterfaceDetails
	subIdx      interface_types.InterfaceIndex
	swErr       error
	busyWhileUp bool
	adminUp     bool
	upErr       error
	requests    []api.Message
}

var _ api.Channel = (*parentChannel)(nil)

func (c *parentChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.requests = append(c.requests, msg)
	return &parentReqCtx{ch: c, req: msg}
}

func (c *parentChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.requests = append(c.requests, msg)
	dump, ok := msg.(*interfaces.SwInterfaceDump)
	if !ok {
		return &mtuMultiCtx{sent: true}
	}
	details, known := c.details[dump.SwIfIndex]
	if !known {
		return &mtuMultiCtx{sent: true}
	}
	return &mtuMultiCtx{details: details}
}

func (c *parentChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *parentChannel) SetReplyTimeout(time.Duration) {}

func (c *parentChannel) CheckCompatiblity(...api.Message) error { return nil }

func (c *parentChannel) Close() {}

type parentReqCtx struct {
	ch  *parentChannel
	req api.Message
}

func (r *parentReqCtx) ReceiveReply(msg api.Message) error {
	switch reply := msg.(type) {
	case *interfaces.CreateVlanSubifReply:
		reply.SwIfIndex = r.ch.subIdx
	case *interfaces.SwInterfaceSetMtuReply:
		return r.ch.swErr
	case *interfaces.DeleteLoopbackReply:
		reply.Retval = int32(api.INVALID_SW_IF_INDEX)
	case *interfaces.HwInterfaceSetMtuReply:
		if r.ch.busyWhileUp && r.ch.adminUp {
			return api.BUSY
		}
	case *interfaces.SwInterfaceSetFlagsReply:
		flags, ok := r.req.(*interfaces.SwInterfaceSetFlags)
		if !ok {
			return fmt.Errorf("SwInterfaceSetFlagsReply for %T", r.req)
		}
		if flags.Flags&interface_types.IF_STATUS_API_FLAG_ADMIN_UP == 0 {
			r.ch.adminUp = false
			return nil
		}
		if r.ch.upErr != nil {
			return r.ch.upErr
		}
		r.ch.adminUp = true
	}
	return nil
}

// parentDetails is hardware interface xe0 at index 3 with the given L3 and
// link MTUs, admin up.
func parentDetails(l3MTU uint32, linkMTU uint16) interfaces.SwInterfaceDetails {
	return interfaces.SwInterfaceDetails{
		SwIfIndex:    3,
		SupSwIfIndex: 3,
		Type:         interface_types.IF_API_TYPE_HARDWARE,
		Flags:        interface_types.IF_STATUS_API_FLAG_ADMIN_UP,
		LinkMtu:      linkMTU,
		Mtu:          []uint32{l3MTU, 0, 0, 0},
	}
}

func newParentBackend(ch *parentChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 3, "xe0")
	return b
}

// TestVPPCreateVLANTakesItsParentMTU proves a VLAN sub-interface VPP creates
// is given its parent's MTU. Before the fix CreateVLAN sent no MTU: VPP builds
// the sub-interface from a zeroed template, its L3 MTU stays 0, and
// vnet_sw_interface_get_mtu reads 0 as 9000, so under a parent with no mtu
// leaf (which Ze never sets) the VLAN exceeded a 1500 parent. Method: create
// eth VLAN 100 under parents stating an L3 MTU, only a link MTU, and neither,
// and read the SwInterfaceSetMtu sent for the new index 7.
func TestVPPCreateVLANTakesItsParentMTU(t *testing.T) {
	tests := []struct {
		name   string
		parent interfaces.SwInterfaceDetails
		want   uint32
	}{
		{"parent carries an L3 MTU", parentDetails(1500, 9000), 1500},
		{"parent states only its link MTU", parentDetails(0, 1500), 1500},
		{"parent states no MTU", parentDetails(0, 0), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &parentChannel{
				details: map[interface_types.InterfaceIndex]interfaces.SwInterfaceDetails{3: tt.parent},
				subIdx:  7,
			}
			b := newParentBackend(ch)
			if err := b.CreateVLAN(iface.VLANSpec{Parent: "xe0", VLANID: 100}); err != nil {
				t.Fatalf("CreateVLAN: %v", err)
			}
			sw := sent[*interfaces.SwInterfaceSetMtu](&mtuChannel{requests: ch.requests})
			if tt.want == 0 {
				if len(sw) != 0 {
					t.Fatalf("a parent with no MTU still set the VLAN's: %+v", sw)
				}
				return
			}
			if len(sw) != 1 {
				t.Fatalf("SwInterfaceSetMtu requests = %+v, want one for the new VLAN", sw)
			}
			want := fmt.Sprint([]uint32{tt.want, 0, 0, 0})
			if sw[0].SwIfIndex != 7 || fmt.Sprint(sw[0].Mtu) != want {
				t.Fatalf("SwInterfaceSetMtu = index %d mtu %v, want index 7 mtu %s", sw[0].SwIfIndex, sw[0].Mtu, want)
			}
		})
	}
}

// TestVPPCreateVLANRemovesTheVLANWhenItsMTUFails proves a VLAN whose MTU VPP
// refuses is deleted and the create fails. config-apply reads a failed create
// whose interface exists as an interface that already existed, so a VLAN left
// behind would keep MTU 0 with no undo recorded to remove it.
func TestVPPCreateVLANRemovesTheVLANWhenItsMTUFails(t *testing.T) {
	ch := &parentChannel{
		details: map[interface_types.InterfaceIndex]interfaces.SwInterfaceDetails{3: parentDetails(1500, 1500)},
		subIdx:  7,
		swErr:   api.INVALID_VALUE,
	}
	b := newParentBackend(ch)
	err := b.CreateVLAN(iface.VLANSpec{Parent: "xe0", VLANID: 100})
	if err == nil || !strings.Contains(err.Error(), "xe0.100") {
		t.Fatalf("CreateVLAN with a refused MTU: err = %v, want an error naming xe0.100", err)
	}
	del := sent[*interfaces.DeleteSubif](&mtuChannel{requests: ch.requests})
	if len(del) != 1 || del[0].SwIfIndex != 7 {
		t.Fatalf("DeleteSubif requests = %+v, want one for index 7", del)
	}
	if _, known := b.names.lookupName(7); known {
		t.Fatal("the deleted VLAN is still in the name map")
	}
}

// TestVPPSetMTURaisesABusyLinkWithThePortStopped proves a link raise the
// driver refuses as busy (i40e answers -EBUSY for a started port, and VPP
// relays it as BUSY) is retried with the port stopped: down, raise, up, then
// the protocol MTU. Before the fix the commit failed with the driver's answer.
func TestVPPSetMTURaisesABusyLinkWithThePortStopped(t *testing.T) {
	ch := &parentChannel{
		details:     map[interface_types.InterfaceIndex]interfaces.SwInterfaceDetails{3: parentDetails(1500, 1500)},
		busyWhileUp: true,
		adminUp:     true,
	}
	b := newParentBackend(ch)
	if err := b.SetMTU("xe0", 9000); err != nil {
		t.Fatalf("SetMTU 9000 on a busy port: %v", err)
	}
	var got []string
	for _, m := range ch.requests {
		switch r := m.(type) {
		case *interfaces.HwInterfaceSetMtu:
			got = append(got, fmt.Sprint("hw ", r.Mtu))
		case *interfaces.SwInterfaceSetFlags:
			got = append(got, fmt.Sprint("flags ", r.Flags&interface_types.IF_STATUS_API_FLAG_ADMIN_UP != 0))
		case *interfaces.SwInterfaceSetMtu:
			got = append(got, fmt.Sprint("sw ", r.Mtu[0]))
		}
	}
	want := "[hw 9000 flags false hw 9000 flags true sw 9000]"
	if fmt.Sprint(got) != want {
		t.Fatalf("requests = %v, want %s", got, want)
	}
	if !ch.adminUp {
		t.Fatal("the port was left admin down")
	}
}

// TestVPPSetMTUStopsABusyPortOnlyWhenNeeded proves the stop is reserved for
// a raise on a running port: a lower MTU fits the link and takes the protocol
// MTU alone, and a port already down is not touched.
func TestVPPSetMTUStopsABusyPortOnlyWhenNeeded(t *testing.T) {
	ch := &parentChannel{
		details:     map[interface_types.InterfaceIndex]interfaces.SwInterfaceDetails{3: parentDetails(1500, 1500)},
		busyWhileUp: true,
		adminUp:     true,
	}
	b := newParentBackend(ch)
	if err := b.SetMTU("xe0", 1400); err != nil {
		t.Fatalf("SetMTU 1400 below the link: %v", err)
	}
	if got := sent[*interfaces.SwInterfaceSetFlags](&mtuChannel{requests: ch.requests}); len(got) != 0 {
		t.Fatalf("a lower MTU stopped the port: %+v", got)
	}
	if got := sent[*interfaces.SwInterfaceSetMtu](&mtuChannel{requests: ch.requests}); len(got) != 1 || got[0].Mtu[0] != 1400 {
		t.Fatalf("SwInterfaceSetMtu = %+v, want one with L3 1400", got)
	}

	down := parentDetails(1500, 1500)
	down.Flags = 0
	ch = &parentChannel{
		details: map[interface_types.InterfaceIndex]interfaces.SwInterfaceDetails{3: down},
	}
	ch.busyWhileUp = true
	ch.adminUp = true // VPP still answers busy: the stop would change nothing
	b = newParentBackend(ch)
	if err := b.SetMTU("xe0", 9000); err == nil {
		t.Fatal("SetMTU 9000 on a busy port VPP reports down: want the refusal")
	}
	if got := sent[*interfaces.SwInterfaceSetFlags](&mtuChannel{requests: ch.requests}); len(got) != 0 {
		t.Fatalf("a port reported down was set down and up: %+v", got)
	}
}

// TestVPPSetMTUReportsAPortLeftDown proves a port set down for the raise
// that VPP then refuses to set up again fails the commit with an error saying
// it is left down, rather than reporting the MTU as applied.
func TestVPPSetMTUReportsAPortLeftDown(t *testing.T) {
	ch := &parentChannel{
		details:     map[interface_types.InterfaceIndex]interfaces.SwInterfaceDetails{3: parentDetails(1500, 1500)},
		busyWhileUp: true,
		adminUp:     true,
		upErr:       errors.New("start failed"),
	}
	b := newParentBackend(ch)
	err := b.SetMTU("xe0", 9000)
	if err == nil || !strings.Contains(err.Error(), "left admin down") {
		t.Fatalf("SetMTU with a failed restart: err = %v, want it to say the port is left admin down", err)
	}
	if got := sent[*interfaces.SwInterfaceSetMtu](&mtuChannel{requests: ch.requests}); len(got) != 0 {
		t.Fatalf("protocol MTUs written over a port left down: %+v", got)
	}
}
