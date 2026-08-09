// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- transport orchestrator tests

package transport

import (
	"bytes"
	"errors"
	"net/netip"
	"testing"
	"time"
)

type fakeBackend struct {
	handles map[string]*fakeHandle
	opens   []string
}

func newFakeBackend() *fakeBackend { return &fakeBackend{handles: make(map[string]*fakeHandle)} }

func (b *fakeBackend) OpenInterface(name string, _ dropRecorder) (InterfaceHandle, error) {
	h := &fakeHandle{ifindex: len(b.opens) + 10, recv: make(chan RawPacket, 4)}
	b.handles[name] = h
	b.opens = append(b.opens, name)
	return h, nil
}

type blockingBackend struct {
	opened  chan struct{}
	release chan struct{}
	handle  *fakeHandle
}

func (b *blockingBackend) OpenInterface(name string, _ dropRecorder) (InterfaceHandle, error) {
	close(b.opened)
	<-b.release
	b.handle = &fakeHandle{ifindex: 20, recv: make(chan RawPacket, 4)}
	return b.handle, nil
}

type fakeHandle struct {
	ifindex int
	recv    chan RawPacket
	closed  bool
	joins   []netip.Addr
	leaves  []netip.Addr
	sends   []fakeSend
	routed  []fakeSend
}

type fakeSend struct {
	dst     netip.Addr
	payload []byte
}

func (h *fakeHandle) IfIndex() int           { return h.ifindex }
func (h *fakeHandle) Recv() <-chan RawPacket { return h.recv }
func (h *fakeHandle) JoinAllSPFRouters() error {
	h.joins = append(h.joins, AllSPFRouters)
	return nil
}
func (h *fakeHandle) JoinAllDRouters() error {
	h.joins = append(h.joins, AllDRouters)
	return nil
}
func (h *fakeHandle) LeaveAllDRouters() error {
	h.leaves = append(h.leaves, AllDRouters)
	return nil
}
func (h *fakeHandle) Send(dst netip.Addr, payload []byte) error {
	cp := append([]byte(nil), payload...)
	h.sends = append(h.sends, fakeSend{dst: dst, payload: cp})
	return nil
}

// SendRouted records a routed (TTL > 1) send, satisfying the routedSender capability so
// SendPacketRouted takes the routed path rather than the TTL-1 link-local Send.
func (h *fakeHandle) SendRouted(dst netip.Addr, payload []byte) error {
	cp := append([]byte(nil), payload...)
	h.routed = append(h.routed, fakeSend{dst: dst, payload: cp})
	return nil
}
func (h *fakeHandle) Close() error {
	if !h.closed {
		h.closed = true
		close(h.recv)
	}
	return nil
}

func TestStripIPv4Header(t *testing.T) {
	packet := []byte{
		0x45, 0, 0, 24, 0, 0, 0, 0, 1, Protocol, 0, 0,
		192, 0, 2, 1, 224, 0, 0, 5,
		0xde, 0xad, 0xbe, 0xef,
	}
	payload, src, ok := StripIPv4Header(packet)
	if !ok || src != netip.MustParseAddr("192.0.2.1") || !bytes.Equal(payload, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Fatalf("StripIPv4Header = payload % x src %v ok %v", payload, src, ok)
	}
	if _, _, ok := StripIPv4Header(packet[:10]); ok {
		t.Fatalf("short datagram accepted")
	}
	badIHL := append([]byte(nil), packet...)
	badIHL[0] = 0x44
	if _, _, ok := StripIPv4Header(badIHL); ok {
		t.Fatalf("IHL < 20 accepted")
	}
	badIHL[0] = 0x4f
	if _, _, ok := StripIPv4Header(badIHL); ok {
		t.Fatalf("IHL overrun accepted")
	}
}

func TestMulticastGroupConstants(t *testing.T) {
	if Protocol != 89 || AllSPFRouters.String() != "224.0.0.5" || AllDRouters.String() != "224.0.0.6" {
		t.Fatalf("wrong OSPF constants: proto=%d all=%s dr=%s", Protocol, AllSPFRouters, AllDRouters)
	}
}

func TestPacketTypeLabel(t *testing.T) {
	cases := []struct {
		payload []byte
		want    string
	}{
		{[]byte{2, 1}, "hello"},
		{[]byte{2, 2}, "dbdesc"},
		{[]byte{2, 3}, "lsreq"},
		{[]byte{2, 4}, "lsupdate"},
		{[]byte{2, 5}, "lsack"},
		{[]byte{2, 99}, "unknown"},
		{[]byte{2}, "short"},
	}
	for _, tc := range cases {
		if got := packetTypeLabel(tc.payload); got != tc.want {
			t.Fatalf("packetTypeLabel(% x) = %s, want %s", tc.payload, got, tc.want)
		}
	}
}

func TestOSPFTransportOpenOnLinkUp(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	if !tr.InterfaceOpen("eth0") || tr.OpenInterfaceCount() != 1 {
		t.Fatalf("interface not open")
	}
	if len(fb.handles["eth0"].joins) != 1 || fb.handles["eth0"].joins[0] != AllSPFRouters {
		t.Fatalf("AllSPFRouters not joined: %+v", fb.handles["eth0"].joins)
	}
}

func TestOSPFTransportDoesNotPublishDisabledInterfaceAfterSlowOpen(t *testing.T) {
	backend := &blockingBackend{opened: make(chan struct{}), release: make(chan struct{})}
	tr := New(backend)
	tr.EnableInterface("eth0")
	errCh := make(chan error, 1)
	go func() { errCh <- tr.HandleLinkUp("eth0") }()
	<-backend.opened
	tr.DisableInterface("eth0")
	close(backend.release)
	if err := <-errCh; err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	if tr.InterfaceOpen("eth0") {
		t.Fatal("disabled interface was published after slow open")
	}
	if backend.handle == nil || !backend.handle.closed {
		t.Fatalf("disabled interface handle not closed: %#v", backend.handle)
	}
}

func TestOSPFTransportSendMulticastDoesNotAlterPayload(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	payload := []byte{1, 2, 3, 4}
	if err := tr.SendPacket("eth0", AllSPFRouters, payload); err != nil {
		t.Fatalf("SendPacket: %v", err)
	}
	got := fb.handles["eth0"].sends[0]
	if got.dst != AllSPFRouters || !bytes.Equal(got.payload, payload) {
		t.Fatalf("send = dst %v payload % x", got.dst, got.payload)
	}
	payload[0] = 9
	if got.payload[0] != 1 {
		t.Fatalf("fake should capture original payload copy")
	}
}

func TestOSPFTransportSendUnicastDoesNotAlterPayload(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	payload := []byte{2, 2, 0, 24}
	dst := netip.MustParseAddr("192.0.2.2")
	if err := tr.SendPacket("eth0", dst, payload); err != nil {
		t.Fatalf("SendPacket: %v", err)
	}
	got := fb.handles["eth0"].sends[0]
	if got.dst != dst || !bytes.Equal(got.payload, payload) {
		t.Fatalf("send = dst %v payload % x", got.dst, got.payload)
	}
}

// VALIDATES: spec-ospf-ext-7 AC-6 / A-7 / R-1 -- a virtual-link send takes the routed path
// (SendRouted, TTL > 1), distinct from the TTL-1 link-local Send used for normal OSPF.
func TestVirtualLinkSendUsesRoutedTTL(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	dst := netip.MustParseAddr("172.16.0.2")
	payload := []byte{1, 2, 3, 4}
	if err := tr.SendPacketRouted("eth0", dst, netip.Addr{}, payload); err != nil {
		t.Fatalf("SendPacketRouted: %v", err)
	}
	h := fb.handles["eth0"]
	if len(h.routed) != 1 || h.routed[0].dst != dst || !bytes.Equal(h.routed[0].payload, payload) {
		t.Fatalf("routed send not used: routed=%+v", h.routed)
	}
	if len(h.sends) != 0 {
		t.Fatalf("virtual-link packet leaked onto the TTL-1 link-local path: %+v", h.sends)
	}
	// An IPv6 destination is rejected by the IPv4 routed path.
	if err := tr.SendPacketRouted("eth0", netip.MustParseAddr("2001:db8::1"), netip.Addr{}, payload); !errors.Is(err, ErrInvalidDestination) {
		t.Fatalf("IPv6 dst on IPv4 routed send err = %v, want ErrInvalidDestination", err)
	}
}

func TestOSPFTransportJoinLeaveAllDRouters(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	if err := tr.JoinAllDRouters("eth0"); err != nil {
		t.Fatalf("JoinAllDRouters: %v", err)
	}
	if err := tr.LeaveAllDRouters("eth0"); err != nil {
		t.Fatalf("LeaveAllDRouters: %v", err)
	}
	h := fb.handles["eth0"]
	if h.joins[len(h.joins)-1] != AllDRouters || h.leaves[len(h.leaves)-1] != AllDRouters {
		t.Fatalf("DR group join/leave wrong: joins=%v leaves=%v", h.joins, h.leaves)
	}
}

func TestOSPFTransportReceiveDispatchByIfindex(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	h := fb.handles["eth0"]
	want := RawPacket{IfIndex: h.ifindex, Src: netip.MustParseAddr("192.0.2.1"), Payload: []byte{2, 1}}
	h.recv <- want
	select {
	case got := <-tr.Receive():
		if got.IfIndex != want.IfIndex || got.Src != want.Src || !bytes.Equal(got.Payload, want.Payload) {
			t.Fatalf("receive = %+v, want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for receive")
	}
}

func TestOSPFTransportCloseOnLinkDown(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	tr.EnableInterface("eth0")
	var downIf int
	tr.OnInterfaceDown(func(ifindex int, name string) { downIf = ifindex })
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	h := fb.handles["eth0"]
	if err := tr.HandleLinkDown("eth0"); err != nil {
		t.Fatalf("HandleLinkDown: %v", err)
	}
	if tr.InterfaceOpen("eth0") || !h.closed || downIf != h.ifindex {
		t.Fatalf("down did not close: open=%v closed=%v downIf=%d", tr.InterfaceOpen("eth0"), h.closed, downIf)
	}
}

func TestOSPFTransportErrors(t *testing.T) {
	tr := New(newFakeBackend())
	if err := tr.SendPacket("missing", AllSPFRouters, []byte{1}); !errors.Is(err, ErrInterfaceNotOpen) {
		t.Fatalf("SendPacket unopened err = %v", err)
	}
	if err := tr.SendPacket("missing", netip.MustParseAddr("2001:db8::1"), []byte{1}); !errors.Is(err, ErrInvalidDestination) {
		t.Fatalf("SendPacket IPv6 err = %v", err)
	}
}
