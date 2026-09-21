// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- RFC 2328 section 4.4 link
// indications: the transport passes interface up and down events to OSPF.

package transport

import "testing"

// RFC requirement: RFC2328-4.4-3 positive — a link-up on an OSPF-enabled interface opens the
// socket and passes the up indication (ifindex, name) to OSPF, and the link-down that follows
// closes it and passes the down indication with the same ifindex (HandleLinkUp, HandleLinkDown).
func TestOSPFTransportPassesLinkIndicationsToOSPF(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	var upIf, downIf int
	var upName, downName string
	tr.OnInterfaceUp(func(ifindex int, name string) { upIf, upName = ifindex, name })
	tr.OnInterfaceDown(func(ifindex int, name string) { downIf, downName = ifindex, name })
	tr.EnableInterface("eth0")

	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	h := fb.handles["eth0"]
	if h == nil {
		t.Fatal("link-up did not open eth0")
	}
	if upIf != h.ifindex || upName != "eth0" {
		t.Fatalf("up indication = (%d, %q), want (%d, %q)", upIf, upName, h.ifindex, "eth0")
	}
	if downName != "" {
		t.Fatalf("down indication passed before any link-down: %q", downName)
	}

	if err := tr.HandleLinkDown("eth0"); err != nil {
		t.Fatalf("HandleLinkDown: %v", err)
	}
	if downIf != h.ifindex || downName != "eth0" {
		t.Fatalf("down indication = (%d, %q), want (%d, %q)", downIf, downName, h.ifindex, "eth0")
	}
	if !h.closed {
		t.Fatal("link-down did not close the socket")
	}
}

// RFC requirement: RFC2328-4.4-3 negative — a link-up on an interface OSPF is not enabled on
// opens no socket and passes no up indication, and a link-down on an interface that is not open
// passes no down indication: only an OSPF interface's transitions reach the protocol
// (HandleLinkUp enabled gate, HandleLinkDown open gate).
func TestOSPFTransportPassesNoIndicationForNonOSPFInterface(t *testing.T) {
	fb := newFakeBackend()
	tr := New(fb)
	ups, downs := 0, 0
	tr.OnInterfaceUp(func(int, string) { ups++ })
	tr.OnInterfaceDown(func(int, string) { downs++ })

	if err := tr.HandleLinkUp("eth9"); err != nil {
		t.Fatalf("HandleLinkUp on a non-OSPF interface: %v", err)
	}
	if len(fb.opens) != 0 {
		t.Fatalf("opened %v for a non-OSPF interface, want none", fb.opens)
	}
	if err := tr.HandleLinkDown("eth9"); err != nil {
		t.Fatalf("HandleLinkDown on a closed interface: %v", err)
	}
	if ups != 0 || downs != 0 {
		t.Fatalf("indications = up %d, down %d, want none for a non-OSPF interface", ups, downs)
	}
}
