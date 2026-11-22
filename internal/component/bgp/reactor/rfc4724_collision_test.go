// Design: docs/architecture/behavior/peer-lifecycle.md -- RFC 4724 Section 4.2 GR collision override
// Related: session.go -- detectCollision, peerAdvertisedGRFamilies
// Related: peer_connection.go -- finishCollisionHandoff
// RFC 4724 Sections 4.2 and 5 -- see rfc/short/rfc4724.md

package reactor

import (
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// setupEstablishedGRSession establishes a passive session whose peer OPEN
// carries the supplied capabilities (e.g. Graceful Restart with or without
// address-family tuples).
func setupEstablishedGRSession(t *testing.T, localID, remoteID uint32, peerCaps ...capability.Capability) (*Session, net.Conn, net.Conn) {
	t.Helper()
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, localID,
	)
	settings.Connection = ConnectionPassive
	session := NewSession(settings)
	if err := session.Start(); err != nil {
		t.Fatalf("session.Start: %v", err)
	}

	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	collisionAcceptWithReader(t, session, server, client)

	peerOpen := &message.Open{
		Version:       4,
		MyAS:          65002,
		HoldTime:      90,
		BGPIdentifier: remoteID,
	}
	if len(peerCaps) > 0 {
		optParams, extended, err := capability.PackOptionalParams([]capability.OptionalParam{
			capability.CapabilitiesParam(peerCaps),
		})
		if err != nil {
			t.Fatalf("PackOptionalParams: %v", err)
		}
		peerOpen.OptionalParams = optParams
		peerOpen.ExtendedParams = extended
	}
	openBytes := message.PackTo(peerOpen, nil)
	go func() {
		_, _ = client.Write(openBytes)
		buf := make([]byte, 4096)
		_, _ = client.Read(buf) // drain local OPEN / KEEPALIVE
	}()
	if err := session.ReadAndProcess(); err != nil {
		t.Fatalf("ReadAndProcess OPEN: %v", err)
	}

	keepaliveBytes := message.PackTo(message.NewKeepalive(), nil)
	go func() {
		_, _ = client.Write(keepaliveBytes)
	}()
	if err := session.ReadAndProcess(); err != nil {
		t.Fatalf("ReadAndProcess KEEPALIVE: %v", err)
	}
	if session.State() != fsm.StateEstablished {
		t.Fatalf("session state = %v, want Established", session.State())
	}
	return session, client, server
}

// TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification proves the
// RFC 4724 Section 4.2 & Section 5 collision override on Established sessions.
//
// VALIDATES: when an Established session previously received a Graceful Restart
// Capability with one or more AFI/SAFI tuples, a subsequent incoming connection
// is accepted and the old TCP session is closed WITHOUT sending a NOTIFICATION,
// leaving the new connection retained. When no GR capability (or an empty GR
// capability with zero families) was received, RFC 4271 Section 6.8 stands:
// the new connection is rejected with Cease/Connection Collision and the old
// Established session stays open.
// PREVENTS: rejecting a restarting peer's reconnect with Cease/Connection
// Collision while the half-open old TCP session is still Established, or
// sending a NOTIFICATION on the old session that would flush retained routes.
//
// RFC requirement: RFC4724-4.2-1 positive -- Established session with received GR capability (1+ AFI/SAFI) treats subsequent connection as old-session termination and accepts the new connection.
// RFC requirement: RFC4724-4.2-1 negative -- Established session without GR capability or with an empty GR capability (0 families) rejects the subsequent connection per RFC 4271 Section 6.8.
// RFC requirement: RFC4724-4.2-2 positive -- finishCollisionHandoff closes the previous TCP session without sending a NOTIFICATION and retains the new connection.
// RFC requirement: RFC4724-4.2-2 negative -- without GR families, finishCollisionHandoff rejects the new connection with Cease/Connection Collision and leaves the previous TCP session open.
func TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification(t *testing.T) {
	t.Run("GR with families closes old TCP silently and retains new", func(t *testing.T) {
		grCap := &capability.GracefulRestart{
			RestartTime: 120,
			Families: []capability.GracefulRestartFamily{
				{AFI: capability.AFI(1), SAFI: capability.SAFIUnicast, ForwardingState: true},
			},
		}
		// Local BGP ID (0x0A000001) > remote BGP ID (0x01020302): under plain
		// RFC 4271 Section 6.8 the incoming connection would lose even in OpenConfirm.
		oldSession, oldClient, _ := setupEstablishedGRSession(t, 0x0A000001, 0x01020302, grCap)

		accept, closeExisting := oldSession.detectCollision(0x01020302)
		if !accept || !closeExisting {
			t.Fatalf("detectCollision with GR families = (%v, %v), want (true, true)", accept, closeExisting)
		}

		// Drive finishCollisionHandoff with a live Peer holding the new connection.
		peer := newTestPeer()
		newClient, newServer := net.Pipe()
		defer func() {
			_ = newClient.Close()
			_ = newServer.Close()
		}()
		peer.mu.Lock()
		peer.session = oldSession
		peer.inbound = &incomingSocket{conn: newServer}
		peer.collisionHandoff = true
		peer.mu.Unlock()

		// Read from oldClient to observe whether any NOTIFICATION bytes arrive before close.
		oldReadDone := make(chan struct {
			n   int
			err error
		}, 1)
		go func() {
			buf := make([]byte, 128)
			_ = oldClient.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := oldClient.Read(buf)
			oldReadDone <- struct {
				n   int
				err error
			}{n, err}
		}()

		peer.finishCollisionHandoff(oldSession, newServer, 0x01020302)

		res := <-oldReadDone
		if res.n != 0 {
			t.Fatalf("old TCP session received %d bytes (expected 0 / no NOTIFICATION)", res.n)
		}
		if res.err != io.EOF {
			t.Fatalf("old TCP session read err = %v, want io.EOF (closed)", res.err)
		}

		// Verify the new connection is retained in peer.inbound and still open.
		peer.mu.RLock()
		retained := inboundConn(peer.inbound)
		peer.mu.RUnlock()
		if retained != newServer {
			t.Fatalf("new connection was not retained in peer.inbound")
		}
		// Writing 1 byte from newClient to newServer succeeds because newServer is open.
		writeDone := make(chan error, 1)
		go func() {
			_, err := newClient.Write([]byte{0xFF})
			writeDone <- err
		}()
		var probe [1]byte
		_ = newServer.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := newServer.Read(probe[:]); err != nil {
			t.Fatalf("retained new connection is not readable: %v", err)
		}
		if err := <-writeDone; err != nil {
			t.Fatalf("write to retained new connection failed: %v", err)
		}
	})

	t.Run("no GR capability rejects new connection and keeps old", func(t *testing.T) {
		oldSession, _, _ := setupEstablishedGRSession(t, 0x01020304, 0xFFFFFFFF)

		accept, closeExisting := oldSession.detectCollision(0xFFFFFFFF)
		if accept || closeExisting {
			t.Fatalf("detectCollision without GR = (%v, %v), want (false, false)", accept, closeExisting)
		}

		peer := newTestPeer()
		newClient, newServer := net.Pipe()
		defer func() {
			_ = newClient.Close()
			_ = newServer.Close()
		}()
		peer.mu.Lock()
		peer.session = oldSession
		peer.inbound = &incomingSocket{conn: newServer}
		peer.collisionHandoff = true
		peer.mu.Unlock()

		notifCh := make(chan []byte, 1)
		go func() {
			buf := make([]byte, 128)
			_ = newClient.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _ := newClient.Read(buf)
			notifCh <- buf[:n]
		}()

		peer.finishCollisionHandoff(oldSession, newServer, 0xFFFFFFFF)

		got := <-notifCh
		if len(got) < message.HeaderLen+2 {
			t.Fatalf("expected NOTIFICATION on rejected new connection, got %d bytes", len(got))
		}
		if got[message.HeaderLen] != byte(message.NotifyCease) || got[message.HeaderLen+1] != message.NotifyCeaseConnectionCollision {
			t.Fatalf("NOTIFICATION code/subcode = %d/%d, want %d/%d",
				got[message.HeaderLen], got[message.HeaderLen+1],
				message.NotifyCease, message.NotifyCeaseConnectionCollision)
		}
		if oldSession.Conn() == nil {
			t.Fatal("old Established session without GR was closed; want kept open")
		}
	})

	t.Run("empty GR capability without families follows RFC 4271", func(t *testing.T) {
		emptyGR := &capability.GracefulRestart{RestartTime: 120}
		oldSession, _, _ := setupEstablishedGRSession(t, 0x01020304, 0xFFFFFFFF, emptyGR)

		accept, closeExisting := oldSession.detectCollision(0xFFFFFFFF)
		if accept || closeExisting {
			t.Fatalf("detectCollision with 0-family GR = (%v, %v), want (false, false)", accept, closeExisting)
		}
	})
}
