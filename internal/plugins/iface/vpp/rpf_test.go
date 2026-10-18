// VPP backend reverse-path checks: the urpf_update messages an IPv4 set turns
// into, the stale checks it turns off, and the refusals.
package ifacevpp

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/urpf"

	"github.com/ze-software/ze/internal/component/iface"
)

// urpfChannel is an api.Channel fake for the urpf plugin. It answers
// UrpfInterfaceDump with installed, UrpfUpdate with retval, and
// CheckCompatiblity with compatErr, the way GoVPP reports messages VPP does
// not know. It records every request so a test can assert what reached VPP.
type urpfChannel struct {
	installed []urpf.UrpfInterfaceDetails
	retval    int32
	compatErr error
	requests  []api.Message
}

var _ api.Channel = (*urpfChannel)(nil)

func (c *urpfChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.requests = append(c.requests, msg)
	return &urpfReqCtx{retval: c.retval}
}

func (c *urpfChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.requests = append(c.requests, msg)
	return &urpfMultiCtx{rows: c.installed}
}

func (c *urpfChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *urpfChannel) SetReplyTimeout(time.Duration) {}

func (c *urpfChannel) CheckCompatiblity(...api.Message) error { return c.compatErr }

func (c *urpfChannel) Close() {}

type urpfReqCtx struct{ retval int32 }

func (r *urpfReqCtx) ReceiveReply(msg api.Message) error {
	if reply, ok := msg.(*urpf.UrpfUpdateReply); ok {
		reply.Retval = r.retval
	}
	return nil
}

type urpfMultiCtx struct{ rows []urpf.UrpfInterfaceDetails }

func (m *urpfMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if len(m.rows) == 0 {
		return true, nil
	}
	if dst, ok := msg.(*urpf.UrpfInterfaceDetails); ok {
		*dst = m.rows[0]
	}
	m.rows = m.rows[1:]
	return false, nil
}

func newURPFBackend(ch *urpfChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 1, "xe0")
	b.names.Add("xe1", 2, "xe1")
	b.names.Add("xe1.100", 3, "xe1.100")
	return b
}

func urpfUpdates(ch *urpfChannel) []urpf.UrpfUpdate {
	var out []urpf.UrpfUpdate
	for _, m := range ch.requests {
		if u, ok := m.(*urpf.UrpfUpdate); ok {
			out = append(out, *u)
		}
	}
	return out
}

func rx4(idx uint32, mode urpf.UrpfMode) urpf.UrpfUpdate {
	return urpf.UrpfUpdate{IsInput: true, Mode: mode, Af: ip_types.ADDRESS_IP4, SwIfIndex: interface_types.InterfaceIndex(idx)}
}

// VALIDATES: an IPv4 set becomes one urpf_update per interface, receive
// direction, IPv4 family, with VPP's own mode for strict and loose.
// PREVENTS: the defect this backend had, where an IPv4 rpf-check reached only
// the Linux Control Plane tap's rp_filter and VPP forwarded transit traffic
// with no source check at all.
func TestVPPReplaceIPv4RPFInstallsEachCheck(t *testing.T) {
	ch := &urpfChannel{}
	b := newURPFBackend(ch)

	err := b.ReplaceIPv4RPF([]iface.RPFCheck{
		{Interface: "xe0", Mode: iface.RPFStrict},
		{Interface: "xe1.100", Mode: iface.RPFLoose},
	})
	if err != nil {
		t.Fatalf("ReplaceIPv4RPF: %v", err)
	}
	got := urpfUpdates(ch)
	want := []urpf.UrpfUpdate{rx4(1, urpf.URPF_API_MODE_STRICT), rx4(3, urpf.URPF_API_MODE_LOOSE)}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("urpf_update requests = %+v, want %+v", got, want)
	}
}

// VALIDATES: the set is whole. A receive IPv4 check VPP holds and the set no
// longer names is turned off, a changed mode is rewritten, an unchanged one is
// left alone, and checks of another family or direction are not touched.
// PREVENTS: a check that outlives its leaf or its interface, the stale state
// an incremental update leaves after a removal or a restart.
func TestVPPReplaceIPv4RPFTurnsOffWhatTheSetDropped(t *testing.T) {
	ch := &urpfChannel{installed: []urpf.UrpfInterfaceDetails{
		{SwIfIndex: 1, IsInput: true, Mode: urpf.URPF_API_MODE_LOOSE, Af: ip_types.ADDRESS_IP4},
		{SwIfIndex: 2, IsInput: true, Mode: urpf.URPF_API_MODE_STRICT, Af: ip_types.ADDRESS_IP4},
		{SwIfIndex: 3, IsInput: true, Mode: urpf.URPF_API_MODE_STRICT, Af: ip_types.ADDRESS_IP4},
		{SwIfIndex: 2, IsInput: true, Mode: urpf.URPF_API_MODE_STRICT, Af: ip_types.ADDRESS_IP6},
		{SwIfIndex: 2, IsInput: false, Mode: urpf.URPF_API_MODE_STRICT, Af: ip_types.ADDRESS_IP4},
	}}
	b := newURPFBackend(ch)

	err := b.ReplaceIPv4RPF([]iface.RPFCheck{
		{Interface: "xe0", Mode: iface.RPFStrict},
		{Interface: "xe1.100", Mode: iface.RPFStrict},
	})
	if err != nil {
		t.Fatalf("ReplaceIPv4RPF: %v", err)
	}
	got := urpfUpdates(ch)
	want := []urpf.UrpfUpdate{rx4(2, urpf.URPF_API_MODE_OFF), rx4(1, urpf.URPF_API_MODE_STRICT)}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("urpf_update requests = %+v, want %+v", got, want)
	}

	ch.requests = nil
	if err := b.ReplaceIPv4RPF(nil); err != nil {
		t.Fatalf("ReplaceIPv4RPF(empty): %v", err)
	}
	got = urpfUpdates(ch)
	want = []urpf.UrpfUpdate{rx4(1, urpf.URPF_API_MODE_OFF), rx4(2, urpf.URPF_API_MODE_OFF), rx4(3, urpf.URPF_API_MODE_OFF)}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("empty set: urpf_update requests = %+v, want %+v", got, want)
	}
}

// VALIDATES: a VPP without the urpf plugin cannot hold a check, so the empty
// set every config without an IPv4 rpf-check asks for succeeds there without
// a message, while a real set is refused naming the plugin to load.
func TestVPPReplaceIPv4RPFWithoutThePlugin(t *testing.T) {
	ch := &urpfChannel{compatErr: errors.New("message urpf_update not found")}
	b := newURPFBackend(ch)

	if err := b.ReplaceIPv4RPF(nil); err != nil {
		t.Fatalf("empty set without the plugin: %v", err)
	}
	for _, m := range ch.requests {
		switch m.(type) {
		case *urpf.UrpfUpdate, *urpf.UrpfInterfaceDump:
			t.Fatalf("urpf request %T sent to a VPP without the plugin", m)
		}
	}
	err := b.ReplaceIPv4RPF([]iface.RPFCheck{{Interface: "xe0", Mode: iface.RPFStrict}})
	if err == nil {
		t.Fatal("a check without the urpf plugin must be refused")
	}
	if !strings.Contains(err.Error(), "urpf_plugin.so") {
		t.Fatalf("refusal %q does not name urpf_plugin.so", err)
	}
}

// VALIDATES: an interface VPP does not know, and a urpf_update VPP refuses,
// fail the apply rather than leave the check uninstalled.
func TestVPPReplaceIPv4RPFRefusals(t *testing.T) {
	b := newURPFBackend(&urpfChannel{})
	if err := b.ReplaceIPv4RPF([]iface.RPFCheck{{Interface: "nope0", Mode: iface.RPFStrict}}); err == nil {
		t.Fatal("an unknown interface must be refused")
	}
	b = newURPFBackend(&urpfChannel{retval: -1})
	err := b.ReplaceIPv4RPF([]iface.RPFCheck{{Interface: "xe0", Mode: iface.RPFLoose}})
	if err == nil {
		t.Fatal("a urpf_update VPP refuses must fail the apply")
	}
	if !strings.Contains(err.Error(), "xe0") {
		t.Fatalf("refusal %q does not name the interface", err)
	}
}

// VALIDATES: an IPv6 check is still refused. VPP judges a link-local source
// against fe80::/10, whose uRPF list is empty, so both modes would drop every
// unicast packet from a link-local address: neighbor advertisements, DHCPv6
// and link-local BGP.
func TestVPPReplaceIPv6RPFRefusesAndAcceptsEmpty(t *testing.T) {
	b := newURPFBackend(&urpfChannel{})
	if err := b.ReplaceIPv6RPF(nil); err != nil {
		t.Fatalf("empty set: %v", err)
	}
	err := b.ReplaceIPv6RPF([]iface.RPFCheck{{Interface: "xe0", Mode: iface.RPFLoose}})
	if err == nil {
		t.Fatal("an ipv6 check must be refused")
	}
	if !strings.Contains(err.Error(), "link-local") {
		t.Fatalf("refusal %q does not say why", err)
	}
}
