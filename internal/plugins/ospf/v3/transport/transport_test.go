// VALIDATES: spec-ospfv3-3-ipv6-transport -- the transport orchestrator opens on
// link-up, joins ff02::5, finalizes the IPv6 checksum from the egress link-local
// source on send (binding cm.Src to that source), demuxes the Instance ID,
// carries dst/hop-limit up, drops short datagrams, toggles ff02::6 on DR/BDR, and
// retries a pending open (IPv6 DAD). PREVENTS a wrong checksum source, a missing
// Instance ID filter, or a goroutine leak on teardown.

package transport

import (
	"bytes"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// ospfv3Packet builds a minimal valid OSPFv3 common header (version 3, the given
// type and Instance ID, Length = 16, checksum field zero) for transport tests.
// The transport never decodes the body, so the 16-byte header is sufficient.
func ospfv3Packet(ptype, instanceID byte) []byte {
	p := make([]byte, packet.CommonHeaderLen)
	p[0] = 3           // Version
	p[1] = ptype       // Type
	p[2], p[3] = 0, 16 // Packet Length = 16
	p[14] = instanceID // Instance ID (byte 14)
	return p
}

type fakeSend struct {
	dst     netip.Addr
	src     netip.Addr
	payload []byte
}

type routedSendRecord struct {
	dst, src netip.Addr
	payload  []byte
	hopLimit int
}

type fakeHandle struct {
	ifindex int
	src     netip.Addr
	recv    chan RawPacket
	closed  bool
	joins   []netip.Addr
	leaves  []netip.Addr
	sends   []fakeSend
	routed  []routedSendRecord
}

func (h *fakeHandle) IfIndex() int                { return h.ifindex }
func (h *fakeHandle) LinkLocalSource() netip.Addr { return h.src }
func (h *fakeHandle) Recv() <-chan RawPacket      { return h.recv }
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
func (h *fakeHandle) Send(dst, src netip.Addr, payload []byte) error {
	cp := append([]byte(nil), payload...)
	h.sends = append(h.sends, fakeSend{dst: dst, src: src, payload: cp})
	return nil
}

// SendRouted records a routed send (global source + hop limit > 1), satisfying the
// routedSenderV6 capability so SendPacketRouted takes the routed path.
func (h *fakeHandle) SendRouted(dst, src netip.Addr, payload []byte, hopLimit int) error {
	cp := append([]byte(nil), payload...)
	h.routed = append(h.routed, routedSendRecord{dst: dst, src: src, payload: cp, hopLimit: hopLimit})
	return nil
}
func (h *fakeHandle) Close() error {
	if !h.closed {
		h.closed = true
		close(h.recv)
	}
	return nil
}

type fakeBackend struct {
	handles map[string]*fakeHandle
	opens   []string
}

func newFakeBackend() *fakeBackend { return &fakeBackend{handles: make(map[string]*fakeHandle)} }

func (b *fakeBackend) OpenInterface(name string, _ DropRecorder) (InterfaceHandle, error) {
	h := &fakeHandle{
		ifindex: len(b.opens) + 10,
		src:     netip.MustParseAddr("fe80::1"),
		recv:    make(chan RawPacket, 4),
	}
	b.handles[name] = h
	b.opens = append(b.opens, name)
	return h, nil
}

// pendingBackend fails the first open (link-local not yet ready, IPv6 DAD) and
// succeeds on the second (rescan retry).
type pendingBackend struct {
	attempts int
	handle   *fakeHandle
}

func (b *pendingBackend) OpenInterface(string, DropRecorder) (InterfaceHandle, error) {
	b.attempts++
	if b.attempts == 1 {
		return nil, ErrNoLinkLocal
	}
	b.handle = &fakeHandle{ifindex: 30, src: netip.MustParseAddr("fe80::1"), recv: make(chan RawPacket, 4)}
	return b.handle, nil
}

type blockingBackend struct {
	opened  chan struct{}
	release chan struct{}
	handle  *fakeHandle
}

func (b *blockingBackend) OpenInterface(string, DropRecorder) (InterfaceHandle, error) {
	close(b.opened)
	<-b.release
	b.handle = &fakeHandle{ifindex: 20, src: netip.MustParseAddr("fe80::1"), recv: make(chan RawPacket, 4)}
	return b.handle, nil
}

func mustUp(t *testing.T, tr *Transport, id types.InstanceID) {
	t.Helper()
	const name = "eth0"
	tr.enableInterfaceInstance(name, id)
	if err := tr.HandleLinkUp(name); err != nil {
		t.Fatalf("HandleLinkUp(%s): %v", name, err)
	}
}

func TestPacketTypeLabel(t *testing.T) {
	cases := []struct {
		payload []byte
		want    string
	}{
		{[]byte{3, 1}, "hello"},
		{[]byte{3, 2}, "dbdesc"},
		{[]byte{3, 3}, "lsreq"},
		{[]byte{3, 4}, "lsupdate"},
		{[]byte{3, 5}, "lsack"},
		{[]byte{3, 99}, "unknown"},
		{[]byte{3}, "short"},
	}
	for _, tc := range cases {
		if got := packetTypeLabel(tc.payload); got != tc.want {
			t.Fatalf("packetTypeLabel(% x) = %s, want %s", tc.payload, got, tc.want)
		}
	}
}

func TestOSPFv3TransportOpenOnLinkUp(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	mustUp(t, tr, 0)
	if !tr.InterfaceOpen("eth0") || tr.OpenInterfaceCount() != 1 {
		t.Fatalf("interface not open")
	}
	if j := fb.handles["eth0"].joins; len(j) != 1 || j[0] != AllSPFRouters {
		t.Fatalf("AllSPFRouters not joined: %+v", j)
	}
}

// VALIDATES: spec-ospf-ext-7 AC-7 / A-8 / R-1 / R-2 -- a routed OSPFv3 virtual-link send
// uses the local GLOBAL source (not the interface link-local fe80::1) and a hop limit > 1,
// distinct from the link-local hop-limit-1 path.
func TestRoutedSendUsesGlobalSourceAndHopLimit(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	mustUp(t, tr, 0)
	global := netip.MustParseAddr("2001:db8::1")
	dst := netip.MustParseAddr("2001:db8:cafe::2")
	pkt := ospfv3Packet(1, 0)
	if err := tr.SendPacketRouted("eth0", dst, global, pkt); err != nil {
		t.Fatalf("SendPacketRouted: %v", err)
	}
	h := fb.handles["eth0"]
	if len(h.routed) != 1 {
		t.Fatalf("routed send not used: %+v", h.routed)
	}
	rs := h.routed[0]
	if rs.src != global {
		t.Fatalf("routed src = %v, want the GLOBAL source %v", rs.src, global)
	}
	if rs.src == h.src {
		t.Fatalf("routed send used the link-local source, not the global one")
	}
	if rs.hopLimit <= 1 {
		t.Fatalf("routed hop limit = %d, want > 1", rs.hopLimit)
	}
	if len(h.sends) != 0 {
		t.Fatalf("routed packet leaked onto the link-local hop-limit-1 path: %+v", h.sends)
	}
	// A missing global source is rejected (the checksum pseudo-header would not match).
	if err := tr.SendPacketRouted("eth0", dst, netip.Addr{}, pkt); !errors.Is(err, ErrInvalidDestination) {
		t.Fatalf("routed send without a global source err = %v, want ErrInvalidDestination", err)
	}
}

func TestOSPFv3TransportFinalizesChecksumOnSend(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	mustUp(t, tr, 0)

	pkt := ospfv3Packet(1, 0) // checksum field zero
	dst := AllSPFRouters
	if err := tr.SendPacket("eth0", dst, pkt); err != nil {
		t.Fatalf("SendPacket: %v", err)
	}
	got := fb.handles["eth0"].sends[0]
	src := netip.MustParseAddr("fe80::1")

	// The egress source is bound to the interface link-local and equals the
	// pseudo-header source.
	if got.src != src {
		t.Fatalf("send src = %v, want %v (bound to link-local)", got.src, src)
	}
	if got.dst != dst {
		t.Fatalf("send dst = %v, want %v", got.dst, dst)
	}
	// The checksum was finalized and verifies against the same src/dst.
	if !packet.VerifyPacketChecksum(src, dst, got.payload) {
		t.Fatalf("finalized packet fails VerifyPacketChecksum: % x", got.payload)
	}
	if got.payload[12] == 0 && got.payload[13] == 0 {
		t.Fatalf("checksum field still zero after finalize")
	}
}

func TestOSPFv3TransportSendDoesNotAlterBody(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	mustUp(t, tr, 0)

	pkt := ospfv3Packet(1, 7)
	orig := append([]byte(nil), pkt...)
	if err := tr.SendPacket("eth0", AllSPFRouters, pkt); err != nil {
		t.Fatalf("SendPacket: %v", err)
	}
	got := fb.handles["eth0"].sends[0].payload
	// Only the 2 checksum bytes (12,13) may differ; every other byte is unchanged.
	for i := range orig {
		if i == 12 || i == 13 {
			continue
		}
		if got[i] != orig[i] {
			t.Fatalf("byte %d changed: got %#02x want %#02x", i, got[i], orig[i])
		}
	}
}

func TestOSPFv3TransportSignerSkipsChecksum(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	tr.SetSigner(func(_ string, payload []byte) []byte { return payload }) // identity signer
	mustUp(t, tr, 0)

	pkt := ospfv3Packet(1, 0)
	if err := tr.SendPacket("eth0", AllSPFRouters, pkt); err != nil {
		t.Fatalf("SendPacket: %v", err)
	}
	got := fb.handles["eth0"].sends[0].payload
	// RFC 7166 §2.2: with a signer present the checksum stays zero (the trailer
	// covers integrity); the transport must NOT finalize it.
	if got[12] != 0 || got[13] != 0 {
		t.Fatalf("checksum finalized despite signer: %#02x%02x", got[12], got[13])
	}
}

func TestOSPFv3TransportRejectsWrongInstance(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	mustUp(t, tr, 5) // interface configured for Instance ID 5
	h := fb.handles["eth0"]

	// A wrong-Instance-ID datagram is dropped; the following matching one is
	// delivered, proving the wrong one was filtered (RFC 5340 §4.2.1).
	h.recv <- RawPacket{IfIndex: h.ifindex, Payload: ospfv3Packet(1, 9)}
	h.recv <- RawPacket{IfIndex: h.ifindex, Payload: ospfv3Packet(1, 5)}

	select {
	case got := <-tr.Receive():
		if id, _ := packet.PeekInstanceID(got.Payload); id != 5 {
			t.Fatalf("delivered Instance ID %d, want 5 (wrong one not filtered)", id)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out; matching-instance packet not delivered")
	}
}

func TestOSPFv3TransportDropsShortDatagram(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	mustUp(t, tr, 0)
	h := fb.handles["eth0"]

	h.recv <- RawPacket{IfIndex: h.ifindex, Payload: []byte{3, 1}} // 2 bytes < 16
	h.recv <- RawPacket{IfIndex: h.ifindex, Payload: ospfv3Packet(1, 0)}

	select {
	case got := <-tr.Receive():
		if len(got.Payload) != packet.CommonHeaderLen {
			t.Fatalf("delivered a %d-byte datagram; short one not dropped", len(got.Payload))
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out; valid datagram not delivered")
	}
}

func TestOSPFv3TransportReceiveCarriesDstHopLimit(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	mustUp(t, tr, 0)
	h := fb.handles["eth0"]

	want := RawPacket{
		IfIndex:  h.ifindex,
		Src:      netip.MustParseAddr("fe80::2"),
		Dst:      AllSPFRouters,
		HopLimit: 1,
		Payload:  ospfv3Packet(1, 0),
	}
	h.recv <- want
	select {
	case got := <-tr.Receive():
		if got.IfIndex != want.IfIndex || got.Src != want.Src || got.Dst != want.Dst || got.HopLimit != want.HopLimit {
			t.Fatalf("receive = %+v, want ifindex/src/dst/hop %d/%v/%v/%d", got, want.IfIndex, want.Src, want.Dst, want.HopLimit)
		}
		if !bytes.Equal(got.Payload, want.Payload) {
			t.Fatalf("payload mismatch")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for receive")
	}
}

func TestOSPFv3TransportJoinLeaveAllDRouters(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	mustUp(t, tr, 0)
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

func TestOSPFv3TransportOpenPendingLinkLocal(t *testing.T) {
	backend := &pendingBackend{}
	tr := New(backend)
	tr.enableInterfaceInstance("eth0", 0)

	// First open fails because the link-local source is still tentative (DAD).
	if err := tr.HandleLinkUp("eth0"); !errors.Is(err, ErrNoLinkLocal) {
		t.Fatalf("first HandleLinkUp err = %v, want ErrNoLinkLocal", err)
	}
	if tr.InterfaceOpen("eth0") {
		t.Fatal("interface opened despite no link-local source")
	}
	// The rescan retries and succeeds once the link-local is ready.
	tr.RescanInterfaces()
	if !tr.InterfaceOpen("eth0") {
		t.Fatal("rescan did not open the interface after the link-local appeared")
	}
}

func TestOSPFv3TransportCloseOnLinkDown(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	var downIf int
	tr.OnInterfaceDown(func(ifindex int, _ string) { downIf = ifindex })
	mustUp(t, tr, 0)
	h := fb.handles["eth0"]
	if err := tr.HandleLinkDown("eth0"); err != nil {
		t.Fatalf("HandleLinkDown: %v", err)
	}
	if tr.InterfaceOpen("eth0") || !h.closed || downIf != h.ifindex {
		t.Fatalf("down did not close: open=%v closed=%v downIf=%d", tr.InterfaceOpen("eth0"), h.closed, downIf)
	}
}

func TestOSPFv3TransportDisabledAfterSlowOpenNotPublished(t *testing.T) {
	backend := &blockingBackend{opened: make(chan struct{}), release: make(chan struct{})}
	tr := New(backend)
	tr.enableInterfaceInstance("eth0", 0)
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

func TestOSPFv3TransportErrors(t *testing.T) {
	tr := New(newFakeBackend())
	if err := tr.SendPacket("missing", AllSPFRouters, ospfv3Packet(1, 0)); !errors.Is(err, ErrInterfaceNotOpen) {
		t.Fatalf("SendPacket unopened err = %v, want ErrInterfaceNotOpen", err)
	}
	if err := tr.SendPacket("missing", netip.MustParseAddr("192.0.2.1"), ospfv3Packet(1, 0)); !errors.Is(err, ErrInvalidDestination) {
		t.Fatalf("SendPacket IPv4 dst err = %v, want ErrInvalidDestination", err)
	}
}
