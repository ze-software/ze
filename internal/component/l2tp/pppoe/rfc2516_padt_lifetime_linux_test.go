// VALIDATES: PADT tears down the AC-owned transport descriptor before further
// PPP use, while other sessions and unrelated discovery frames remain usable.
// RFC: rfc/short/rfc2516.md
//go:build linux

package pppoe

import (
	"errors"
	"log/slog"
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

// newPADTSession gives the table ownership of one real transport descriptor.
// The opposite socket remains open so Write fails only after the AC closes its
// own endpoint, rather than because the test peer disconnected.
func newPADTSession(t *testing.T, server *InterfaceServer, mac [EthALen]byte) (uint16, int) {
	t.Helper()
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	sid, err := server.sessions.AllocSID()
	if err != nil {
		closePPPoxFD(pair[0])
		closePPPoxFD(pair[1])
		t.Fatal(err)
	}
	err = server.sessions.Add(&Session{SID: sid, MAC: net.HardwareAddr(mac[:]), PppoxFD: pair[0], State: StateSession})
	if err != nil {
		closePPPoxFD(pair[0])
		closePPPoxFD(pair[1])
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closePPPoxFD(server.sessions.Remove(sid))
		closePPPoxFD(pair[1])
	})
	return sid, pair[0]
}

// RFC requirement: RFC2516-5.5-4 positive -- a matching received PADT closes the AC transport descriptor so a later PPP write fails.
// RFC requirement: RFC2516-5.5-4 negative -- wrong source, destination and session identifiers cannot close a live transport; another session remains writable after the matching PADT.
// MUTATION: omit closing the descriptor returned by Remove in handlePADT.
func TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport(t *testing.T) {
	local := [EthALen]byte{2, 0, 0, 0, 0, 1}
	peer := [EthALen]byte{2, 0, 0, 0, 0, 2}
	server := &InterfaceServer{hwAddr: local, sessions: newSessionTable("eth0", 10), logger: slog.Default()}
	sid, fd := newPADTSession(t, server, peer)
	otherSID, otherFD := newPADTSession(t, server, peer)
	packet := Packet{Code: CodePADT, SID: sid, SrcMAC: peer, DstMAC: local}
	wrongSource := packet
	wrongSource.SrcMAC[5]++
	wrongDestination := packet
	wrongDestination.DstMAC[5]++
	wrongSession := packet
	wrongSession.SID = otherSID + 1
	for _, invalid := range []Packet{wrongSource, wrongDestination, wrongSession} {
		server.HandleDiscovery(&invalid)
		if _, err := unix.Write(fd, []byte("PPP")); err != nil {
			t.Fatalf("irrelevant PADT closed transport: %v", err)
		}
	}
	server.HandleDiscovery(&packet)
	if _, err := unix.Write(fd, []byte("PPP")); !errors.Is(err, unix.EBADF) {
		t.Fatalf("PPP after PADT: %v, want closed descriptor", err)
	}
	if _, err := unix.Write(otherFD, []byte("PPP")); err != nil {
		t.Fatalf("PADT closed a different session: %v", err)
	}
	if server.sessions.Lookup(sid) != nil || server.sessions.Lookup(otherSID) == nil {
		t.Fatal("PADT removed the wrong session from discovery state")
	}
}

// RFC requirement: RFC2516-5.5-4 positive -- the AC closes its transport before the outbound PADT reaches the discovery writer.
// MUTATION: move transport close below sendFrame in handleSessionDown.
func TestRFC2516ACSentPADTClosesTransportBeforeSend(t *testing.T) {
	local := [EthALen]byte{2, 0, 0, 0, 0, 1}
	peer := [EthALen]byte{2, 0, 0, 0, 0, 2}
	server := &InterfaceServer{hwAddr: local, sessions: newSessionTable("eth0", 1), logger: slog.Default()}
	sid, fd := newPADTSession(t, server, peer)
	sent := 0
	server.sendFrameFn = func(frame []byte) {
		sent++
		packet, err := ParseDiscovery(frame)
		if err != nil {
			t.Fatal(err)
		}
		if packet.Code != CodePADT || packet.SID != sid || packet.SrcMAC != local || packet.DstMAC != peer {
			t.Fatalf("PADT names the wrong session: %+v", packet)
		}
		if _, err := unix.Write(fd, []byte("PPP")); !errors.Is(err, unix.EBADF) {
			t.Fatalf("transport still writable when PADT was sent: %v", err)
		}
		// Even a concurrent received PADT cannot release this teardown's
		// SID until the outstanding outbound PADT has been published.
		server.handlePADT(&Packet{SID: sid, SrcMAC: peer, DstMAC: local})
		if _, err := server.sessions.AllocSID(); !errors.Is(err, ErrMaxSessions) {
			t.Fatalf("replacement admitted before old PADT completed: %v", err)
		}
	}
	server.handleSessionDown(sid)
	if sent != 1 {
		t.Fatalf("sent %d PADTs, want one", sent)
	}
	if replacement, err := server.sessions.AllocSID(); err != nil {
		t.Fatalf("session capacity not released after PADT: %v", err)
	} else {
		server.sessions.freeSID(replacement)
	}
}
