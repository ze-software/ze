// Design: docs/architecture/behavior/fsm-open-sent.md -- every OPEN validation error sends NOTIFICATION Error Code 2.
// Related: rfc4271_open_framing_proof_test.go -- outer OPEN framing proof matrices.
// Related: session_handlers.go -- admitOpen and validateOpenLocked.
// RFC: rfc/short/rfc4271.md -- Section 6.2.
package reactor

import (
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type openErrorMatrixCase struct {
	name        string
	configure   func(*PeerSettings)
	body        []byte
	roleReject  bool
	wantSubcode uint8
	wantData    []byte
}

func openErrorMatrixCases() []openErrorMatrixCase {
	const (
		localAS     uint32 = 65001
		peerAS      uint32 = 65002
		localRouter uint32 = 0x0a000002
		peerRouter  uint32 = 0x0a000001
	)
	ipv4Cap := &capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
	ipv6Cap := &capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}
	rrCap := &capability.RouteRefresh{}
	grCap := &capability.GracefulRestart{RestartTime: 120}
	addPathV4Send := &capability.AddPath{
		Families: []capability.AddPathFamily{{
			AFI:  capability.AFIIPv4,
			SAFI: capability.SAFIUnicast,
			Mode: capability.AddPathSend,
		}},
	}
	addPathV4Recv := &capability.AddPath{
		Families: []capability.AddPathFamily{{
			AFI:  capability.AFIIPv4,
			SAFI: capability.SAFIUnicast,
			Mode: capability.AddPathReceive,
		}},
	}
	addPathV4Both := &capability.AddPath{
		Families: []capability.AddPathFamily{{
			AFI:  capability.AFIIPv4,
			SAFI: capability.SAFIUnicast,
			Mode: capability.AddPathBoth,
		}},
	}
	// RFC 9234 Section 4: Role capability (code 9, length 1, value 0 = Provider).
	roleOptParams := []byte{2, 3, 9, 1, 0}

	return []openErrorMatrixCase{
		{
			name:        "unsupported-version-3",
			body:        encodeOpenBody(&message.Open{Version: 3, MyAS: uint16(peerAS), HoldTime: 90, BGPIdentifier: peerRouter}),
			wantSubcode: message.NotifyOpenUnsupportedVersion,
			wantData:    []byte{0, 4},
		},
		{
			name:        "unsupported-version-5",
			body:        encodeOpenBody(&message.Open{Version: 5, MyAS: uint16(peerAS), HoldTime: 90, BGPIdentifier: peerRouter}),
			wantSubcode: message.NotifyOpenUnsupportedVersion,
			wantData:    []byte{0, 4},
		},
		{
			name:        "unacceptable-hold-time-1",
			body:        encodeOpenBody(&message.Open{Version: 4, MyAS: uint16(peerAS), HoldTime: 1, BGPIdentifier: peerRouter}),
			wantSubcode: message.NotifyOpenUnacceptableHoldTime,
			wantData:    []byte{0, 1},
		},
		{
			name:        "unacceptable-hold-time-2",
			body:        encodeOpenBody(&message.Open{Version: 4, MyAS: uint16(peerAS), HoldTime: 2, BGPIdentifier: peerRouter}),
			wantSubcode: message.NotifyOpenUnacceptableHoldTime,
			wantData:    []byte{0, 2},
		},
		{
			name:        "bad-peer-as-zero-2byte",
			body:        encodeOpenBody(&message.Open{Version: 4, MyAS: 0, HoldTime: 90, BGPIdentifier: peerRouter}),
			wantSubcode: message.NotifyOpenBadPeerAS,
		},
		{
			name: "bad-peer-as-zero-4byte",
			body: encodeOpenBodyWithCaps(4, message.AS_TRANS, 90, peerRouter,
				&capability.ASN4{ASN: 0}),
			wantSubcode: message.NotifyOpenBadPeerAS,
		},
		{
			name:        "bad-peer-as-mismatch",
			body:        encodeOpenBody(&message.Open{Version: 4, MyAS: 65099, HoldTime: 90, BGPIdentifier: peerRouter}),
			wantSubcode: message.NotifyOpenBadPeerAS,
		},
		{
			name: "unsupported-optional-parameter",
			body: encodeOpenBody(&message.Open{
				Version:        4,
				MyAS:           uint16(peerAS),
				HoldTime:       90,
				BGPIdentifier:  peerRouter,
				OptionalParams: []byte{99, 2, 0xaa, 0xbb},
			}),
			wantSubcode: message.NotifyOpenUnsupportedOptParam,
		},
		{
			name: "malformed-capability-tlv",
			body: encodeOpenBody(&message.Open{
				Version:        4,
				MyAS:           uint16(peerAS),
				HoldTime:       90,
				BGPIdentifier:  peerRouter,
				OptionalParams: []byte{2, 3, 65, 4, 0},
			}),
			wantSubcode: 0,
		},
		{
			name:        "bad-bgp-identifier-zero",
			body:        encodeOpenBody(&message.Open{Version: 4, MyAS: uint16(peerAS), HoldTime: 90, BGPIdentifier: 0}),
			wantSubcode: message.NotifyOpenBadBGPID,
		},
		{
			name: "bad-bgp-identifier-ibgp-self",
			configure: func(s *PeerSettings) {
				s.PeerAS = localAS
			},
			body:        encodeOpenBody(&message.Open{Version: 4, MyAS: uint16(localAS), HoldTime: 90, BGPIdentifier: localRouter}),
			wantSubcode: message.NotifyOpenBadBGPID,
		},
		{
			name:       "role-mismatch-validator",
			roleReject: true,
			body: encodeOpenBody(&message.Open{
				Version:        4,
				MyAS:           uint16(peerAS),
				HoldTime:       90,
				BGPIdentifier:  peerRouter,
				OptionalParams: roleOptParams,
			}),
			wantSubcode: message.NotifyOpenRoleMismatch,
		},
		{
			name: "missing-required-family",
			configure: func(s *PeerSettings) {
				s.Capabilities = []capability.Capability{ipv4Cap, ipv6Cap}
				s.RequiredFamilies = []capability.Family{{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}}
			},
			body:        encodeOpenBodyWithCaps(4, uint16(peerAS), 90, peerRouter, ipv4Cap),
			wantSubcode: message.NotifyOpenUnsupportedCapability,
			wantData:    []byte{byte(capability.CodeMultiprotocol), 4, 0, 2, 0, 1},
		},
		{
			name: "missing-required-capability",
			configure: func(s *PeerSettings) {
				s.Capabilities = []capability.Capability{ipv4Cap, rrCap}
				s.RequiredCapabilities = []capability.Code{capability.CodeRouteRefresh}
			},
			body:        encodeOpenBodyWithCaps(4, uint16(peerAS), 90, peerRouter, ipv4Cap),
			wantSubcode: message.NotifyOpenUnsupportedCapability,
			wantData:    []byte{byte(capability.CodeRouteRefresh), 0},
		},
		{
			name: "refused-capability-present",
			configure: func(s *PeerSettings) {
				s.Capabilities = []capability.Capability{ipv4Cap}
				s.RefusedCapabilities = []capability.Code{capability.CodeGracefulRestart}
			},
			body:        encodeOpenBodyWithCaps(4, uint16(peerAS), 90, peerRouter, ipv4Cap, grCap),
			wantSubcode: message.NotifyOpenUnsupportedCapability,
			wantData:    []byte{byte(capability.CodeGracefulRestart), 2, 0, 120},
		},
		{
			name: "missing-required-addpath-family",
			configure: func(s *PeerSettings) {
				s.Capabilities = []capability.Capability{ipv4Cap, addPathV4Recv}
				s.RequiredAddPathFamilies = []capability.Family{{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}}
			},
			body:        encodeOpenBodyWithCaps(4, uint16(peerAS), 90, peerRouter, ipv4Cap),
			wantSubcode: message.NotifyOpenUnsupportedCapability,
			wantData:    []byte{byte(capability.CodeAddPath), 4, 0, 1, 1, byte(capability.AddPathReceive)},
		},
		{
			name: "refused-addpath-family-present",
			configure: func(s *PeerSettings) {
				s.Capabilities = []capability.Capability{ipv4Cap, addPathV4Both}
				s.RefusedAddPathFamilies = []capability.Family{{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}}
			},
			body:        encodeOpenBodyWithCaps(4, uint16(peerAS), 90, peerRouter, ipv4Cap, addPathV4Send),
			wantSubcode: message.NotifyOpenUnsupportedCapability,
			wantData:    []byte{byte(capability.CodeAddPath), 4, 0, 1, 1, byte(capability.AddPathSend)},
		},
	}
}

// TestRFC4271AllOPENValidationClassesSendOpenMessageError proves that every
// OPEN error class detected by handleOpen sends NOTIFICATION Error Code 2 with
// its exact subcode and Data, closes the TCP connection, and moves to Idle.
//
// RFC 4271 Section 6.2: "All errors detected while processing the OPEN message
// MUST be indicated by sending the NOTIFICATION message with the Error Code
// OPEN Message Error."
// RFC requirement: RFC4271-6.2-3 positive -- every OPEN validation error class
// detected by handleOpen (unsupported version, unacceptable hold time, AS 0
// and peer AS mismatch, unsupported and malformed optional parameters, zero
// and internal-self BGP Identifier, role mismatch, missing required family,
// missing required capability, refused capability, missing required ADD-PATH,
// and refused ADD-PATH) sends NOTIFICATION Error Code 2 with exact subcode and
// Data, closes the connection, and transitions the ordinary session to Idle.
func TestRFC4271AllOPENValidationClassesSendOpenMessageError(t *testing.T) {
	for _, tc := range openErrorMatrixCases() {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x0a000002)
			settings.Connection = ConnectionPassive
			settings.ReceiveHoldTime = 90 * time.Second
			settings.Capabilities = []capability.Capability{
				&capability.ASN4{ASN: 65001},
				&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			}
			if tc.configure != nil {
				tc.configure(settings)
			}

			session := NewSession(settings)
			t.Cleanup(session.timers.StopAll)
			if tc.roleReject {
				session.setOpenValidator(func(_ string, _, _ *message.Open) error {
					return &bgpserver.OpenValidationError{
						NotifyCode:    uint8(message.NotifyOpenMessage),
						NotifySubcode: message.NotifyOpenRoleMismatch,
						Reason:        "open error matrix role mismatch",
					}
				})
			}
			require.NoError(t, session.Start())
			client, server := net.Pipe()
			t.Cleanup(func() {
				_ = client.Close()
				_ = server.Close()
			})
			_ = acceptWithReader(t, session, server, client)
			require.Equal(t, fsm.StateOpenSent, session.State())
			require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))

			type wireResult struct {
				reply []byte
				eof   error
				err   error
			}
			done := make(chan wireResult, 1)
			go func() {
				if _, err := client.Write(rfc4271ProofOPENPacket(tc.body)); err != nil {
					done <- wireResult{err: err}
					return
				}
				reply, err := core4271ReadMessage(client)
				if err != nil {
					done <- wireResult{err: err}
					return
				}
				var tail [1]byte
				_, eofErr := client.Read(tail[:])
				done <- wireResult{reply: reply, eof: eofErr}
			}()

			err := session.ReadAndProcess()
			require.Error(t, err)
			res := <-done
			require.NoError(t, res.err)
			require.Equal(t, byte(msgtype.TypeNOTIFICATION), res.reply[18])
			wantPayload := append([]byte{uint8(message.NotifyOpenMessage), tc.wantSubcode}, tc.wantData...)
			require.Equal(t, wantPayload, res.reply[message.HeaderLen:])
			require.ErrorIs(t, res.eof, io.EOF, "session must close the TCP connection after OPEN error NOTIFICATION")
			require.Equal(t, fsm.StateIdle, session.State())
		})
	}
}

// TestRFC4271PendingAllOPENValidationClassesSendOpenMessageError proves that
// every OPEN validation error class detected on a pending incoming connection
// sends NOTIFICATION Error Code 2 with exact subcode and Data on that socket,
// closes the pending socket, clears pending admission, and preserves the
// incumbent OpenConfirm session.
//
// RFC 4271 Section 6.2: "All errors detected while processing the OPEN message
// MUST be indicated by sending the NOTIFICATION message with the Error Code
// OPEN Message Error."
// RFC requirement: RFC4271-6.2-3 positive -- every OPEN validation error class
// on a pending incoming connection (unsupported version, unacceptable hold
// time, AS 0 and peer AS mismatch, unsupported and malformed optional
// parameters, zero and internal-self BGP Identifier, plugin role mismatch,
// missing required family, missing required capability, refused capability,
// missing required ADD-PATH, and refused ADD-PATH) sends NOTIFICATION Error
// Code 2 with exact subcode and Data on the pending socket, closes that
// connection, clears pending admission, and leaves the incumbent OpenConfirm
// session intact.
func TestRFC4271PendingAllOPENValidationClassesSendOpenMessageError(t *testing.T) {
	for _, tc := range openErrorMatrixCases() {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x0a000002)
			settings.Connection = ConnectionPassive
			settings.ReceiveHoldTime = 90 * time.Second
			settings.Capabilities = []capability.Capability{
				&capability.ASN4{ASN: 65001},
				&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			}
			if tc.configure != nil {
				tc.configure(settings)
			}

			// Build an incumbent OpenConfirm session matching the peer's config.
			incumbent := NewSession(settings)
			t.Cleanup(incumbent.timers.StopAll)
			require.NoError(t, incumbent.Start())
			incClient, incServer := net.Pipe()
			t.Cleanup(func() {
				_ = incClient.Close()
				_ = incServer.Close()
			})
			_ = acceptWithReader(t, incumbent, incServer, incClient)

			validIncumbentOpen := buildMatchingPeerOpen(settings, 0x01020302)
			go func() {
				_, _ = incClient.Write(message.PackTo(validIncumbentOpen, nil))
				buf := make([]byte, 4096)
				_, _ = incClient.Read(buf) // drain KEEPALIVE
			}()
			require.NoError(t, incumbent.ReadAndProcess())
			require.Equal(t, fsm.StateOpenConfirm, incumbent.State())

			r := New(&Config{})
			r.clock = clock.RealClock{}
			peer := NewPeer(settings)
			peer.reactor = r
			peer.session = incumbent
			if tc.roleReject {
				installRoleRejectPlugin(t, r)
			}

			client, pending := net.Pipe()
			require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
			require.NoError(t, peer.setPendingConnection(pending))

			finished := make(chan struct{})
			go func() {
				defer close(finished)
				r.handlePendingCollision(peer, pending)
			}()
			t.Cleanup(func() {
				_ = client.Close()
				_ = pending.Close()
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("pending OPEN reader did not stop")
				}
			})

			localOpen, err := core4271ReadMessage(client)
			require.NoError(t, err)
			require.Equal(t, byte(msgtype.TypeOPEN), localOpen[18], "pending connection must send local OPEN first")

			_, err = client.Write(rfc4271ProofOPENPacket(tc.body))
			require.NoError(t, err)

			reply, err := core4271ReadMessage(client)
			require.NoError(t, err)
			require.Equal(t, byte(msgtype.TypeNOTIFICATION), reply[18])
			wantPayload := append([]byte{uint8(message.NotifyOpenMessage), tc.wantSubcode}, tc.wantData...)
			require.Equal(t, wantPayload, reply[message.HeaderLen:])

			var tail [1]byte
			_, eofErr := client.Read(tail[:])
			require.ErrorIs(t, eofErr, io.EOF, "pending socket must close after OPEN error NOTIFICATION")

			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("pending handler did not finish after OPEN error")
			}
			require.False(t, peer.hasPendingConnection())
			require.Equal(t, fsm.StateOpenConfirm, incumbent.State())
		})
	}
}

// TestRFC4271ValidOPENControlsAdvanceWithoutOpenMessageError proves that valid
// OPEN messages satisfying all configured version, hold-time, peer-AS,
// BGP-Identifier, capability, ADD-PATH, and plugin-validator constraints advance
// cleanly to OpenConfirm with KEEPALIVE on the ordinary path and reach Cease
// 6/7 collision arbitration (not Error Code 2) on a pending connection.
//
// RFC 4271 Section 6.2: "All errors detected while processing the OPEN message
// MUST be indicated by sending the NOTIFICATION message with the Error Code
// OPEN Message Error."
// RFC requirement: RFC4271-6.2-3 negative -- valid OPEN messages across
// version 4, hold time 0 and 90, 2-octet and 4-octet peer AS, valid BGP
// Identifier (including eBGP shared identifier), required families/capabilities/
// ADD-PATH, and an accepting open validator advance the ordinary handshake to
// OpenConfirm with KEEPALIVE and resolve a pending collision with Cease 6/7
// rather than NOTIFICATION Error Code 2.
func TestRFC4271ValidOPENControlsAdvanceWithoutOpenMessageError(t *testing.T) {
	ipv4Cap := &capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
	ipv6Cap := &capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}
	rrCap := &capability.RouteRefresh{}
	addPathV4Recv := &capability.AddPath{
		Families: []capability.AddPathFamily{{
			AFI:  capability.AFIIPv4,
			SAFI: capability.SAFIUnicast,
			Mode: capability.AddPathReceive,
		}},
	}
	addPathV4Send := &capability.AddPath{
		Families: []capability.AddPathFamily{{
			AFI:  capability.AFIIPv4,
			SAFI: capability.SAFIUnicast,
			Mode: capability.AddPathSend,
		}},
	}

	controls := []struct {
		name      string
		configure func(*PeerSettings)
		body      []byte
	}{
		{
			name: "hold-time-zero-and-ebgp-same-id",
			body: encodeOpenBodyWithCaps(4, 65002, 0, 0x0a000002, ipv4Cap, &capability.ASN4{ASN: 65002}),
		},
		{
			name: "four-octet-peer-as",
			configure: func(s *PeerSettings) {
				s.PeerAS = 4200000001
			},
			body: encodeOpenBodyWithCaps(4, message.AS_TRANS, 90, 0x0a000001,
				ipv4Cap, &capability.ASN4{ASN: 4200000001}),
		},
		{
			name: "required-family-capability-and-addpath-satisfied",
			configure: func(s *PeerSettings) {
				s.Capabilities = []capability.Capability{ipv4Cap, ipv6Cap, rrCap, addPathV4Recv}
				s.RequiredFamilies = []capability.Family{{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}}
				s.RequiredCapabilities = []capability.Code{capability.CodeRouteRefresh}
				s.RefusedCapabilities = []capability.Code{capability.CodeGracefulRestart}
				s.RequiredAddPathFamilies = []capability.Family{{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}}
			},
			body: encodeOpenBodyWithCaps(4, 65002, 90, 0x0a000001,
				ipv4Cap, ipv6Cap, rrCap, addPathV4Send),
		},
	}

	for _, tc := range controls {
		t.Run("ordinary/"+tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x0a000002)
			settings.Connection = ConnectionPassive
			settings.ReceiveHoldTime = 90 * time.Second
			settings.Capabilities = []capability.Capability{
				&capability.ASN4{ASN: 65001},
				ipv4Cap,
			}
			if tc.configure != nil {
				tc.configure(settings)
			}

			session := NewSession(settings)
			t.Cleanup(session.timers.StopAll)
			session.setOpenValidator(func(_ string, _, _ *message.Open) error { return nil })
			require.NoError(t, session.Start())
			client, server := net.Pipe()
			t.Cleanup(func() {
				_ = client.Close()
				_ = server.Close()
			})
			_ = acceptWithReader(t, session, server, client)
			require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))

			type response struct {
				reply []byte
				err   error
			}
			done := make(chan response, 1)
			go func() {
				if _, err := client.Write(rfc4271ProofOPENPacket(tc.body)); err != nil {
					done <- response{err: err}
					return
				}
				reply, err := core4271ReadMessage(client)
				done <- response{reply: reply, err: err}
			}()

			require.NoError(t, session.ReadAndProcess())
			res := <-done
			require.NoError(t, res.err)
			require.Equal(t, byte(msgtype.TypeKEEPALIVE), res.reply[18])
			require.Len(t, res.reply, message.HeaderLen)
			require.Equal(t, fsm.StateOpenConfirm, session.State())
		})
	}

	t.Run("pending/valid-constrained-open-resolves-as-collision-cease", func(t *testing.T) {
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x0a000002)
		settings.Connection = ConnectionPassive
		settings.ReceiveHoldTime = 90 * time.Second
		settings.Capabilities = []capability.Capability{ipv4Cap, ipv6Cap, rrCap, addPathV4Recv}
		settings.RequiredFamilies = []capability.Family{{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}}
		settings.RequiredCapabilities = []capability.Code{capability.CodeRouteRefresh}
		settings.RefusedCapabilities = []capability.Code{capability.CodeGracefulRestart}
		settings.RequiredAddPathFamilies = []capability.Family{{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}}

		incumbent := NewSession(settings)
		t.Cleanup(incumbent.timers.StopAll)
		require.NoError(t, incumbent.Start())
		incClient, incServer := net.Pipe()
		t.Cleanup(func() {
			_ = incClient.Close()
			_ = incServer.Close()
		})
		_ = acceptWithReader(t, incumbent, incServer, incClient)

		validOpen := buildMatchingPeerOpen(settings, 0x0a000001)
		go func() {
			_, _ = incClient.Write(message.PackTo(validOpen, nil))
			buf := make([]byte, 4096)
			_, _ = incClient.Read(buf)
		}()
		require.NoError(t, incumbent.ReadAndProcess())
		require.Equal(t, fsm.StateOpenConfirm, incumbent.State())

		r := &Reactor{clock: clock.RealClock{}, config: &Config{}}
		peer := NewPeer(settings)
		peer.reactor = r
		peer.session = incumbent

		client, pending := net.Pipe()
		require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
		require.NoError(t, peer.setPendingConnection(pending))
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			r.handlePendingCollision(peer, pending)
		}()
		t.Cleanup(func() {
			_ = client.Close()
			_ = pending.Close()
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Error("pending collision reader did not stop")
			}
		})

		localOpen, err := core4271ReadMessage(client)
		require.NoError(t, err)
		require.Equal(t, byte(msgtype.TypeOPEN), localOpen[18])

		validBody := encodeOpenBodyWithCaps(4, 65002, 90, 0x0a000001, ipv4Cap, ipv6Cap, rrCap, addPathV4Send)
		_, err = client.Write(rfc4271ProofOPENPacket(validBody))
		require.NoError(t, err)

		reply, err := core4271ReadMessage(client)
		require.NoError(t, err)
		require.Equal(t, byte(msgtype.TypeNOTIFICATION), reply[18])
		require.Equal(t, []byte{uint8(message.NotifyCease), message.NotifyCeaseConnectionCollision}, reply[message.HeaderLen:])
		require.Equal(t, fsm.StateOpenConfirm, incumbent.State())
	})
}

func encodeOpenBody(open *message.Open) []byte {
	body := make([]byte, 10+len(open.OptionalParams))
	body[0] = open.Version
	binary.BigEndian.PutUint16(body[1:3], open.MyAS)
	binary.BigEndian.PutUint16(body[3:5], open.HoldTime)
	binary.BigEndian.PutUint32(body[5:9], open.BGPIdentifier)
	body[9] = uint8(len(open.OptionalParams)) //nolint:gosec // bounded fixture params
	copy(body[10:], open.OptionalParams)
	return body
}

func encodeCapsAsOptionalParams(caps ...capability.Capability) []byte {
	if len(caps) == 0 {
		return nil
	}
	total := 0
	for _, c := range caps {
		total += c.Len()
	}
	out := make([]byte, 2+total)
	out[0] = 2            // Parameter Type 2: Capabilities (RFC 5492).
	out[1] = uint8(total) //nolint:gosec // bounded fixture capability list
	off := 2
	for _, c := range caps {
		off += c.WriteTo(out, off)
	}
	return out
}

func encodeOpenBodyWithCaps(version uint8, myAS, holdTime uint16, bgpID uint32, caps ...capability.Capability) []byte {
	return encodeOpenBody(&message.Open{
		Version:        version,
		MyAS:           myAS,
		HoldTime:       holdTime,
		BGPIdentifier:  bgpID,
		OptionalParams: encodeCapsAsOptionalParams(caps...),
	})
}

func buildMatchingPeerOpen(settings *PeerSettings, routerID uint32) *message.Open {
	var caps []capability.Capability
	caps = append(caps, &capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast})
	for _, fam := range settings.RequiredFamilies {
		caps = append(caps, &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
	}
	for _, code := range settings.RequiredCapabilities {
		if code == capability.CodeRouteRefresh {
			caps = append(caps, &capability.RouteRefresh{})
		}
	}
	if len(settings.RequiredAddPathFamilies) > 0 {
		var families []capability.AddPathFamily
		for _, fam := range settings.RequiredAddPathFamilies {
			families = append(families, capability.AddPathFamily{
				AFI:  fam.AFI,
				SAFI: fam.SAFI,
				Mode: capability.AddPathSend,
			})
		}
		caps = append(caps, &capability.AddPath{Families: families})
	}
	return &message.Open{
		Version:        4,
		MyAS:           uint16(settings.PeerAS), //nolint:gosec // bounded test AS
		HoldTime:       90,
		BGPIdentifier:  routerID,
		OptionalParams: encodeCapsAsOptionalParams(caps...),
	}
}

func installRoleRejectPlugin(t *testing.T, r *Reactor) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "rfc4271-open-role-reject-test"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "RFC 4271 Section 6.2 role mismatch test validator",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			defer p.Close() //nolint:errcheck // test cleanup
			p.OnValidateOpen(func(*sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
				return &sdk.ValidateOpenOutput{
					NotifyCode:    uint8(message.NotifyOpenMessage),
					NotifySubcode: message.NotifyOpenRoleMismatch,
					Reason:        "pending role mismatch",
				}
			})
			if err := p.Run(t.Context(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	srv := flowForwardPluginServer(t, r, plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
	r.SetPluginServer(srv)
	r.eventDispatcher = bgpserver.NewEventDispatcher(srv)
}
