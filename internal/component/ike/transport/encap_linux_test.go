//go:build linux

package transport

import (
	"log/slog"
	"testing"
)

// VALIDATES: the NAT-T socket carries UDP_ENCAP=UDP_ENCAP_ESPINUDP, so the kernel
// decapsulates ESP that arrives inside UDP on port 4500.
// PREVENTS: Ze holding port 4500 with a plain socket, which swallows every
// UDP-encapsulated ESP datagram in user space and starves the installed XFRM state.
//
// It is deliberately UNTAGGED. RFC 7296 Section 2.23's receive obligation has no row
// in rfc/short/rfc7296.md yet. Ze cannot meet it in full, and the classification is
// the owner's to make (ai/rules/rfc-compliance.md).
//
// The measured reason is TestEncapKernelBindsOneESPFormPerState in
// internal/component/ike/dataplane. On Linux XFRM one inbound state accepts bare ESP
// or UDP-encapsulated ESP, never both. The open question OR-WP8-4 is recorded in
// plan/learned/1313-rfcgate-1b-rfc7296-pilot.md.
//
// This test DOES prove one real defect closed. Before it, Ze held port 4500 with a
// plain socket. The kernel never decapsulated, and every encapsulated ESP datagram
// died in user space.
//
// RFC 7296 Section 2.23 (rfc/full/rfc7296.txt:3544-3548):
// "If Network Address Translation Traversal (NAT-T) is supported (that is, if
// NAT_DETECTION_*_IP payloads were exchanged during IKE_SA_INIT), all devices MUST
// be able to receive and process both UDP-encapsulated ESP and
// non-UDP-encapsulated ESP packets at any time."
//
// The same section scopes itself (rfc/full/rfc7296.txt:3553-3556): "In this section
// only, requirements listed as MUST apply only to implementations supporting NAT
// traversal." Ze supports it. It sends the NAT_DETECTION payloads
// (buildNATDetectionPayloads, engine/initiator.go), it binds port 4500
// (engine/register.go), and it programs UDP encapsulation (engine/child.go). The
// antecedent holds, so the MUST binds.
//
// The assertion reads the option back off the socket. A test that only checked for a
// nil error would pass for a helper that set nothing (ai/rules/testing.md).
//
// SCOPE: this proves the socket half on Linux. The VPP dataplane cannot express
// encapsulation at all, and that gap is owned by
// plan/spec-fixit-vpp-ipsec-inoperable.md.
func TestEncapOptionSetOnNATTSocket(t *testing.T) {
	tr, err := NewNATTTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	defer func() {
		if cerr := tr.Close(); cerr != nil {
			t.Errorf("close: %v", cerr)
		}
	}()

	before, err := ESPInUDPEnabled(tr.Conn())
	if err != nil {
		t.Fatalf("read UDP_ENCAP before: %v", err)
	}
	if before {
		t.Fatalf("a fresh socket already reports UDP_ENCAP set, so the assertion below proves nothing")
	}

	if err := EnableESPInUDP(tr.Conn()); err != nil {
		t.Fatalf("EnableESPInUDP: %v", err)
	}

	after, err := ESPInUDPEnabled(tr.Conn())
	if err != nil {
		t.Fatalf("read UDP_ENCAP after: %v", err)
	}
	if !after {
		t.Errorf("UDP_ENCAP is not UDP_ENCAP_ESPINUDP after EnableESPInUDP; encapsulated ESP would reach user space and be dropped")
	}
}

// VALIDATES: EnableESPInUDP reports a missing socket instead of reporting success.
// PREVENTS: a caller reading a nil error as proof the kernel will decapsulate.
func TestEncapEnableRejectsNilConn(t *testing.T) {
	if err := EnableESPInUDP(nil); err == nil {
		t.Errorf("EnableESPInUDP(nil) = nil; a guard that cannot run must say so")
	}
	if _, err := ESPInUDPEnabled(nil); err == nil {
		t.Errorf("ESPInUDPEnabled(nil) = nil error; a read that cannot run must say so")
	}
}
