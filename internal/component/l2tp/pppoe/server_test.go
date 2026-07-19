package pppoe

import (
	"log/slog"
	"net"
	"testing"
)

// RFC requirement: RFC2516-7-1 positive -- handlePADT tears a session down only when the PADT's SESSION_ID and source MAC both match the stored session; the session-data (0x8864) pairing is enforced by the kernel pppox socket bound to (sid, MAC) in pppoeCreate (kernel_linux.go:91).
// RFC requirement: RFC2516-7-1 negative -- a PADT whose source MAC does not match the session's MAC is rejected: the session is left intact.
func TestHandlePADTVerifiesMACAndSID(t *testing.T) {
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01}

	newServer := func() (*InterfaceServer, uint16) {
		st := NewSessionTable("eth0", 100)
		sid, err := st.AllocSID()
		if err != nil {
			t.Fatalf("AllocSID: %v", err)
		}
		if err := st.Add(&Session{SID: sid, MAC: mac, IfName: "eth0", PppoxFD: -1, State: StateSession}); err != nil {
			t.Fatalf("Add: %v", err)
		}
		return &InterfaceServer{ifName: "eth0", sessions: st, logger: slog.Default()}, sid
	}

	// Positive: matching SESSION_ID and source MAC tears the session down.
	s, sid := newServer()
	var src [EthALen]byte
	copy(src[:], mac)
	s.handlePADT(&Packet{Code: CodePADT, SID: sid, SrcMAC: src})
	if s.sessions.Lookup(sid) != nil {
		t.Error("PADT with matching SID and MAC should have removed the session")
	}

	// Negative: a mismatched source MAC leaves the session intact.
	s, sid = newServer()
	var wrong [EthALen]byte
	copy(wrong[:], net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x99})
	s.handlePADT(&Packet{Code: CodePADT, SID: sid, SrcMAC: wrong})
	if s.sessions.Lookup(sid) == nil {
		t.Error("PADT with a mismatched MAC must not remove the session")
	}
}
