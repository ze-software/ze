// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- encrypted MOBIKE exchanges.
// Related: responder_test.go, rfc7296_retransmit_test.go -- PSK and UDP fixtures.
package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The dispatcher queues only Packet, while maintainSA retains the original IKE
// transport. Replies, cached replies and the graceful-stop drain must select the
// NAT-T socket from the authenticated packet rather than that original transport.
// RFC requirement: RFC4555-3.7-1 positive -- COOKIE2 survives the actual owner handoff.
// MUTATION: replace sendRaw's reply socket with its tr argument.
func TestMobikeOwnerDispatchRepliesFromArrivalSocket(t *testing.T) {
	for _, goodbye := range []bool{false, true} {
		t.Run("goodbye="+strconv.FormatBool(goodbye), func(t *testing.T) {
			ini, resp, _ := establishPSK(t)
			f := mbOwner(t, resp, ini, true)
			original := mbTransport(t, "127.0.0.1:0", false)
			f.local.bindSockets(original, f.myTr)
			f.ps.stopCh = make(chan struct{})
			f.ps.inbound = make(chan transport.Packet, inboundQueueDepth)
			f.ps.ownedSA.Store(f.local)
			setActivePeers(map[string]*PeerSession{f.local.PeerName: f.ps})
			t.Cleanup(func() { setActivePeers(nil) })
			log := slogutil.DiscardLogger()
			table := NewSATable()
			msgID := f.local.ExpectedMsgID
			// Keep our request window occupied independently of the peer's
			// request window, including while a graceful stop drains input.
			sendDPD(f.local, original, winDueDPD(), log)
			probe := mbReceive(t, f.peerTr)
			probeID := parseMsg(t, probe.Data).Header.MessageID
			if goodbye {
				f.ps.graceful.Store(true)
				close(f.ps.stopCh)
			}
			done := make(chan struct{})
			var loopErr error
			go func() {
				loopErr = f.ps.maintainSA(f.local, nil, nil, nil,
					testIKEGroup(), table, f.dp, original, nil, log)
				close(done)
			}()
			t.Cleanup(func() {
				select {
				case <-f.ps.stopCh:
				default:
					close(f.ps.stopCh)
				}
				select {
				case <-done:
				case <-time.After(goodbyeWindowWait + rtxArrive):
					t.Error("owner loop did not stop")
				}
			})
			dispatch := func(raw []byte) {
				t.Helper()
				if err := f.peerTr.SendFrom(transport.AddNonESPMarker(raw),
					nttPeerAddr(t, f.peerTr), nttPeerAddr(t, f.myTr)); err != nil {
					t.Fatal(err)
				}
				pkt := mbReceive(t, f.myTr)
				routeInbound(f.local, pkt, table, f.myTr, log)
			}
			cookie := bytes.Repeat([]byte{0x73}, 32)
			update := mbInformational(t, f.peer, msgID, false, []wire.PayloadEntry{
				mbNotify(wire.NotifyUpdateSAAddresses, nil),
				mbNotify(wire.NotifyCookie2, cookie),
			})
			dispatch(update)
			first := mbReceive(t, f.peerTr)
			mbEndpoint(t, first.RemoteAddr, nttPeerAddr(t, f.myTr))
			echo := mbRequireNotify(t, mbDecrypt(t, f.peer, first.Data), wire.NotifyCookie2)
			if !bytes.Equal(echo.NotificationData, cookie) {
				t.Fatal("owner handoff changed COOKIE2")
			}
			dispatch(update)
			replay := mbReceive(t, f.peerTr)
			mbEndpoint(t, replay.RemoteAddr, nttPeerAddr(t, f.myTr))
			if !bytes.Equal(replay.Data, first.Data) {
				t.Fatal("owner handoff rebuilt the cached response")
			}
			dispatch(mbInformational(t, f.peer, msgID+1, false, nil))
			dpdReply := mbReceive(t, f.peerTr)
			mbEndpoint(t, dpdReply.RemoteAddr, nttPeerAddr(t, f.myTr))
			if len(mbDecrypt(t, f.peer, dpdReply.Data)) != 0 ||
				parseMsg(t, dpdReply.Data).Header.MessageID != msgID+1 {
				t.Fatal("owner handoff did not return the empty DPD response")
			}
			dispatch(mbInformational(t, f.peer, probeID, true, nil))
			if goodbye {
				deleted := mbReceive(t, f.peerTr)
				mbEndpoint(t, deleted.RemoteAddr, nttPeerAddr(t, f.myTr))
				found := false
				for _, entry := range mbDecrypt(t, f.peer, deleted.Data) {
					if d, ok := entry.Payload.(*wire.PayloadDelete); ok && d.ProtocolID == wire.ProtocolIKE {
						found = true
					}
				}
				if !found {
					t.Fatal("graceful drain did not send its IKE Delete after the answer")
				}
			} else {
				close(f.ps.stopCh)
			}
			select {
			case <-done:
				if loopErr != nil {
					t.Fatal(loopErr)
				}
			case <-time.After(goodbyeWindowWait + rtxArrive):
				t.Fatal("owner loop did not finish")
			}
		})
	}
}

// Local NAT policy cannot authorize a fatal extension error to a peer that has
// not shown it understands that extension. NO_NATS_ALLOWED is sufficient evidence
// independently of a MOBIKE_SUPPORTED offer (RFC 4555 Section 3.9).
// RFC requirement: RFC7296-2.21.2-3 positive -- a mismatching NO_NATS_ALLOWED in
// first IKE_AUTH receives the extension error the peer demonstrated it understands.
// RFC requirement: RFC7296-2.21.2-3 negative -- an unextended peer receives only a
// base-protocol policy refusal, or establishes when local policy permits its path.
func TestMobikeAuthExtensionErrorsRequirePeerUnderstanding(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		protected, prohibit, mismatch bool
		refusal                       uint16
	}{
		{name: "unextended-policy-refusal", prohibit: true, refusal: wire.NotifyAuthenticationFailed},
		{name: "matching-protection", protected: true, prohibit: true},
		{name: "demonstrated-extension-mismatch", protected: true, prohibit: true, mismatch: true, refusal: wire.NotifyUnexpectedNATDetected},
		{name: "unextended-accepted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := slogutil.DiscardLogger()
			iniTr := mbTransport(t, "127.0.0.1:0", false)
			respTr := mbTransport(t, "127.0.0.1:0", false)
			oldPort := ikeTestPortFn
			ikeTestPortFn = func() string { return strconv.Itoa(nttPort(t, respTr)) }
			t.Cleanup(func() { ikeTestPortFn = oldPort })
			iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "first-auth-policy")
			iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
			respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
			iniPeer.ProhibitNAT, respPeer.ProhibitNAT = tc.protected, tc.prohibit
			group, esp := testIKEGroup(), testESPGroup()
			ini, err := newInitiatorSA("ze", iniPeer, group, esp)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := newResponderSA("ze", respPeer, group, esp, ini.InitiatorSPI)
			if err != nil {
				t.Fatal(err)
			}
			initial := parseMsg(t, buildSAInitRequest(ini, group))
			initial.Payloads = slices.DeleteFunc(initial.Payloads, func(entry wire.PayloadEntry) bool {
				n, ok := entry.Payload.(*wire.PayloadNotify)
				return ok && (n.NotifyMsgType == wire.NotifyNATDetectionSourceIP ||
					n.NotifyMsgType == wire.NotifyNATDetectionDestIP)
			})
			buf := make([]byte, 4096)
			n, err := initial.CheckedWriteTo(buf, 0)
			if err != nil {
				t.Fatal(err)
			}
			ini.InitiatorSAInitMsg, ini.State = buf[:n], StateSAInitSent
			handleSAInitRequest(resp, initial, buf[:n], nil, nttPeerAddr(t, iniTr), log)
			ini.bindSockets(iniTr, nil)
			resp.bindSockets(respTr, nil)
			table := NewSATable()
			table.Insert(ini)
			handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
			request := mbReceive(t, respTr)
			if tc.mismatch {
				inner := mbDecrypt(t, resp, request.Data)
				tuple := mbRequireNotify(t, inner, wire.NotifyNoNATsAllowed)
				tuple.NotificationData[0] ^= 1
				hdr := parseMsg(t, request.Data).Header
				request.Data, err = buildEncryptedMessageEx(ini, inner, hdr.MessageID, hdr.ExchangeType, hdr.Flags)
				if err != nil {
					t.Fatal(err)
				}
			}
			if notifyOf(mbDecrypt(t, resp, request.Data), wire.NotifyMobikeSupported) != nil {
				t.Fatal("fixture unexpectedly negotiated MOBIKE")
			}
			if resp.NATDetected {
				t.Fatal("fixture reached the old NATDetected guard")
			}
			ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: group, espGroup: esp}
			ps.handleResponderInbound(resp, parseMsg(t, request.Data), request, respTr, log)
			response := mbReceive(t, iniTr)
			if tc.refusal == 0 {
				if resp.State != StateEstablished || ps.getChildSA() == nil {
					t.Fatal("acceptable first AUTH without MOBIKE failed")
				}
				handleAuthResponse(ini, parseMsg(t, response.Data), response.Data, table, iniTr, log)
				if ini.State != StateEstablished {
					t.Fatal("acceptable first AUTH did not complete at the peer")
				}
			} else {
				mbOnlyNotify(t, mbDecrypt(t, ini, response.Data), tc.refusal, nil)
				if resp.State != StateDead || ps.getChildSA() != nil {
					t.Fatal("refused first AUTH installed a Child")
				}
			}
		})
	}
}

// TestMobikeCookie2EchoBounds sends encrypted requests through the owner entry and
// decrypts the responses at the peer, including both inclusive length boundaries.
// RFC requirement: RFC4555-4.2.5-1 positive -- 8- and 64-octet COOKIE2 values are accepted.
// RFC requirement: RFC4555-4.2.5-1 negative -- 7- and 65-octet values receive INVALID_SYNTAX.
// RFC requirement: RFC4555-3.7-1 positive -- accepted COOKIE2 notifications are echoed verbatim.
// RFC requirement: RFC4555-3.7-1 negative -- an absent COOKIE2 produces no unsolicited echo.
// RFC 4555 Section 4.2.5: "The data associated with this notification MUST be between
// 8 and 64 octets in length (inclusive), and MUST be chosen by the exchange initiator
// in a way that is unpredictable to the exchange responder."
// MUTATION: remove validateMobikeRequest's COOKIE2 bounds or the response echo.
func TestMobikeCookie2EchoBounds(t *testing.T) {
	for _, length := range []int{0, 7, 8, 64, 65} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			ini, resp, ps := establishPSK(t)
			peerTr, myTr := rtxPeerLink(t)
			resp.mobike.enabled = true
			resp.mobike.local = nttPeerAddr(t, myTr)
			resp.peerEndpoint = nttPeerAddr(t, peerTr)
			cookie := bytes.Repeat([]byte{0xa7}, length)
			var inner []wire.PayloadEntry
			if length != 0 {
				inner = []wire.PayloadEntry{mbNotify(wire.NotifyCookie2, cookie)}
			}
			request := mbInformational(t, ini, resp.ExpectedMsgID, false, inner)
			ps.handleOwnedInbound(resp, transport.Packet{
				Data: request, RemoteAddr: nttPeerAddr(t, peerTr), LocalAddr: nttPeerAddr(t, myTr),
			}, myTr, nil, slogutil.DiscardLogger())
			answer := mbReceive(t, peerTr)
			payloads := mbResponse(t, ini, answer.Data, parseMsg(t, request).Header.MessageID)
			switch length {
			case 0:
				if len(payloads) != 0 {
					t.Fatalf("empty INFORMATIONAL response = %+v, want no payloads", payloads)
				}
			case 7, 65:
				mbOnlyNotify(t, payloads, wire.NotifyInvalidSyntax, nil)
			case 8, 64:
				mbOnlyNotify(t, payloads, wire.NotifyCookie2, cookie)
			}
		})
	}
}

// TestMobikeCookie2ResponseControlsMigration exercises the pending exchange created
// by the real request producer. A correct echo permits endpoint migration and a
// subsequent encrypted probe; a missing or changed value requests Child teardown.
// RFC requirement: RFC4555-3.7-4 positive -- the matching echo reaches MigrateTunnel.
// RFC requirement: RFC4555-3.7-4 negative -- missing and differing echoes cannot migrate.
// RFC requirement: RFC4555-3.7-5 positive -- missing and differing echoes close the SA
// and return the owner-loop teardown outcome, whose cleanup removes both Child SPIs.
// RFC requirement: RFC4555-3.7-5 negative -- a matching echo keeps the SA usable.
// RFC 4555 Section 3.7: "When processing the response, the original sender MUST
// verify that the value is the same one as sent. If the values do not match, the
// IKE_SA MUST be closed."
// MUTATION: bypass handleMobikeResponse's COOKIE2 comparison or its fatal outcome.
func TestMobikeCookie2ResponseControlsMigration(t *testing.T) {
	for _, mode := range []string{"match", "different", "missing"} {
		t.Run(mode, func(t *testing.T) {
			f := mbInitiator(t)
			log := slogutil.DiscardLogger()
			if err := f.ps.startMobikeRequest(f.local, f.myTr, true, log); err != nil {
				t.Fatalf("start address update: %v", err)
			}
			request := mbReceive(t, f.peerTr)
			inner := mbDecrypt(t, f.peer, request.Data)
			cookie := mbRequireNotify(t, inner, wire.NotifyCookie2).NotificationData
			mbCookieLength(t, cookie)
			mbRequireNotify(t, inner, wire.NotifyUpdateSAAddresses)
			mbRequireNotify(t, inner, wire.NotifyNoAdditionalAddresses)
			if f.dp.count != 0 {
				t.Fatal("tunnel migrated before the return-routability answer")
			}
			returned := bytes.Clone(cookie)
			if mode == "different" {
				returned[len(returned)-1] ^= 1
			}
			var answer []wire.PayloadEntry
			if mode != "missing" {
				answer = []wire.PayloadEntry{mbNotify(wire.NotifyCookie2, returned)}
			}
			raw := mbInformational(t, f.peer, parseMsg(t, request.Data).Header.MessageID, true, answer)
			out := f.ps.handleOwnedInbound(f.local, transport.Packet{
				Data: raw, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: nttPeerAddr(t, f.myTr),
			}, f.myTr, f.dp, log)
			if mode != "match" {
				if !out.reestablish {
					t.Fatal("COOKIE2 failure did not ask the owner to tear down the Child SA")
				}
				if f.local.State != StateDead {
					t.Fatalf("COOKIE2 failure left IKE SA in %v", f.local.State)
				}
				if f.dp.count != 0 {
					t.Fatal("COOKIE2 failure migrated the tunnel")
				}
				// This is the action maintainSA takes for reestablish. The outcome above
				// is asserted first, so unconditional fixture cleanup cannot hide a miss.
				f.ps.cleanupChild(f.dp, nil, log)
				if len(f.dp.removed) != 2 {
					t.Fatalf("removed Child SPIs = %v, want one removal of each SPI", f.dp.removed)
				}
				if !slices.Contains(f.dp.removed, f.child.InboundSPI) {
					t.Fatalf("inbound Child SPI %x was not removed", f.child.InboundSPI)
				}
				if !slices.Contains(f.dp.removed, f.child.OutboundSPI) {
					t.Fatalf("outbound Child SPI %x was not removed", f.child.OutboundSPI)
				}
				return
			}
			if out.reestablish {
				t.Fatal("matching COOKIE2 requested teardown")
			}
			mbCheckMigration(t, f, nttPeerAddr(t, f.myTr), nttPeerAddr(t, f.peerTr))
			if len(f.dp.removed) != 0 {
				t.Fatalf("matching COOKIE2 removed Child SPIs %v", f.dp.removed)
			}
			probe := &dpdState{}
			sendDPD(f.local, f.myTr, probe, log)
			later := mbReceive(t, f.peerTr)
			if parseMsg(t, later.Data).Header.MessageID != parseMsg(t, request.Data).Header.MessageID+1 {
				t.Fatal("live SA did not send its next request after the COOKIE2 answer")
			}
			mbProbeRequest(t, f.peer, later.Data)
		})
	}
}

// TestMobikeCookie2IgnoresForgedAndUnrelatedAnswers proves that COOKIE2 validation
// belongs to the authenticated, matching exchange. Neither an invalid ICV nor an
// unrelated message ID closes the SA or permits migration; the real answer still works.
// MUTATION: process pending COOKIE2 before authentication or before matching Message ID.
func TestMobikeCookie2IgnoresForgedAndUnrelatedAnswers(t *testing.T) {
	f := mbInitiator(t)
	log := slogutil.DiscardLogger()
	if err := f.ps.startMobikeRequest(f.local, f.myTr, true, log); err != nil {
		t.Fatal(err)
	}
	request := mbReceive(t, f.peerTr)
	msgID := parseMsg(t, request.Data).Header.MessageID
	cookie := mbRequireNotify(t, mbDecrypt(t, f.peer, request.Data), wire.NotifyCookie2).NotificationData
	good := mbInformational(t, f.peer, msgID, true, []wire.PayloadEntry{mbNotify(wire.NotifyCookie2, cookie)})
	forged := bytes.Clone(good)
	forged[len(forged)-1] ^= 1
	unrelated := mbInformational(t, f.peer, msgID+1, true, nil)
	for _, raw := range [][]byte{forged, unrelated} {
		out := f.ps.handleOwnedInbound(f.local, transport.Packet{
			Data: raw, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: nttPeerAddr(t, f.myTr),
		}, f.myTr, f.dp, log)
		if out.reestablish {
			t.Fatal("an unauthenticated or unrelated answer requested teardown")
		}
		if f.dp.count != 0 {
			t.Fatal("an unauthenticated or unrelated answer authorized migration")
		}
	}
	f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: good, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: nttPeerAddr(t, f.myTr),
	}, f.myTr, f.dp, log)
	mbCheckMigration(t, f, nttPeerAddr(t, f.myTr), nttPeerAddr(t, f.peerTr))
}

// TestMobikeNoNATsIPv4Tuple checks each address and port against an actual received
// UDP tuple. Rejected notifications cannot choose the response path or move the SA;
// retransmission gets the identical encrypted error at the request endpoint.
// RFC requirement: RFC4555-3.9-3 positive -- the full matching IPv4 tuple is accepted.
// RFC requirement: RFC4555-3.9-3 negative -- changing either address or either port
// produces UNEXPECTED_NAT_DETECTED at the endpoint that sent the request.
// RFC requirement: RFC4555-3.9-4 positive -- rejected tuple data causes no migration
// or endpoint adoption, as shown by the next self-initiated request's destination.
// RFC requirement: RFC4555-3.9-4 negative -- a matching tuple permits a COOKIE2 echo.
// RFC 4555 Section 3.9: "The exchange responder MUST verify that the contents of the
// NO_NATS_ALLOWED notification match the addresses in the IP header."
// MUTATION: omit one tuple comparison, use the notification as an endpoint, or send
// the cached rejection to the SA's established peer instead of the request source.
func TestMobikeNoNATsIPv4Tuple(t *testing.T) {
	for _, field := range []string{"match", "source", "destination", "source-port", "destination-port"} {
		t.Run(field, func(t *testing.T) {
			ini, resp, ps := establishPSK(t)
			oldTr := mbTransport(t, "127.0.0.1:0", false)
			peerTr := mbTransport(t, "127.0.0.1:0", false)
			myTr := mbTransport(t, "127.0.0.1:0", false)
			resp.mobike.enabled = true
			resp.mobike.local = nttPeerAddr(t, myTr)
			resp.peerEndpoint = nttPeerAddr(t, oldTr)
			dp := &mbDataplane{}
			cookie := []byte("no-nats-cookie2!")
			tuple := mbNoNATs(nttPeerAddr(t, peerTr), nttPeerAddr(t, myTr))
			switch field {
			case "source":
				tuple[3] ^= 1
			case "destination":
				tuple[7] ^= 1
			case "source-port":
				tuple[9] ^= 1
			case "destination-port":
				tuple[11] ^= 1
			}
			inner := []wire.PayloadEntry{
				mbNotify(wire.NotifyNoAdditionalAddresses, nil),
				mbNotify(wire.NotifyNoNATsAllowed, tuple),
				mbNotify(wire.NotifyCookie2, cookie),
			}
			// An UPDATE on the rejected path would mutate the Child if validation
			// were moved below the migration branch.
			if field != "match" {
				inner = append(inner, mbNotify(wire.NotifyUpdateSAAddresses, nil))
			}
			request := mbInformational(t, ini, resp.ExpectedMsgID, false, inner)
			if err := peerTr.Send(request, nttPeerAddr(t, myTr)); err != nil {
				t.Fatal(err)
			}
			pkt := mbReceive(t, myTr)
			ps.handleOwnedInbound(resp, pkt, myTr, dp, slogutil.DiscardLogger())
			answer := mbReceive(t, peerTr)
			mbEndpoint(t, answer.RemoteAddr, nttPeerAddr(t, myTr))
			payloads := mbResponse(t, ini, answer.Data, parseMsg(t, request).Header.MessageID)
			if field == "match" {
				mbOnlyNotify(t, payloads, wire.NotifyCookie2, cookie)
				return
			}
			mbErrorEcho(t, payloads, wire.NotifyUnexpectedNATDetected, cookie)
			ps.handleOwnedInbound(resp, pkt, myTr, dp, slogutil.DiscardLogger())
			replay := mbReceive(t, peerTr)
			if !bytes.Equal(replay.Data, answer.Data) {
				t.Fatal("retransmitted NO_NATS rejection changed its encrypted response")
			}
			if dp.count != 0 {
				t.Fatal("rejected NO_NATS tuple migrated a tunnel")
			}
			probe := &dpdState{}
			sendDPD(resp, myTr, probe, slogutil.DiscardLogger())
			oldPath := mbReceive(t, oldTr)
			mbProbeRequest(t, ini, oldPath.Data)
		})
	}
}

// TestMobikeNoNATsReplyUsesReceivedDestination covers a wildcard bind: the response
// source must be the destination of this packet, even while the SA uses another path.
// RFC requirement: RFC4555-3.9-4 positive -- the rejected notification cannot override
// the actual request tuple, including the source selected on a wildcard UDP socket.
// MUTATION: send a NO_NATS rejection using sendRaw instead of the received tuple.
func TestMobikeNoNATsReplyUsesReceivedDestination(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("explicit wildcard UDP source selection requires Linux IP_PKTINFO")
	}
	ini, resp, ps := establishPSK(t)
	peerTr := mbTransport(t, "127.0.0.1:0", false)
	myTr := mbTransport(t, "0.0.0.0:0", false)
	local := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: nttPort(t, myTr)}
	resp.mobike.enabled = true
	resp.mobike.local = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: local.Port}
	resp.peerEndpoint = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 3), Port: nttPort(t, peerTr)}
	tuple := mbNoNATs(nttPeerAddr(t, peerTr), local)
	tuple[3] ^= 1
	request := mbInformational(t, ini, resp.ExpectedMsgID, false, []wire.PayloadEntry{
		mbNotify(wire.NotifyUpdateSAAddresses, nil), mbNotify(wire.NotifyNoNATsAllowed, tuple),
	})
	if err := peerTr.Send(request, local); err != nil {
		t.Fatal(err)
	}
	pkt := mbReceive(t, myTr)
	mbEndpoint(t, pkt.LocalAddr, local)
	ps.handleOwnedInbound(resp, pkt, myTr, &mbDataplane{}, slogutil.DiscardLogger())
	answer := mbReceive(t, peerTr)
	mbEndpoint(t, answer.RemoteAddr, local)
	mbOnlyNotify(t, mbResponse(t, ini, answer.Data, parseMsg(t, request).Header.MessageID),
		wire.NotifyUnexpectedNATDetected, nil)
	ps.handleOwnedInbound(resp, pkt, myTr, &mbDataplane{}, slogutil.DiscardLogger())
	replay := mbReceive(t, peerTr)
	mbEndpoint(t, replay.RemoteAddr, local)
	if !bytes.Equal(replay.Data, answer.Data) {
		t.Fatal("wildcard-source retransmission changed the rejection")
	}
}

// TestMobikeNoNATsIPv6Tuple uses a complete IPv6 packet fixture because the current
// transport opens IPv4 sockets. It decrypts the response cached for that request.
// RFC requirement: RFC4555-3.9-3 positive -- all 36 IPv6 tuple octets match and draw an echo.
// RFC requirement: RFC4555-3.9-3 negative -- either IPv6 address or port mismatch is rejected.
// MUTATION: parse an IPv6 NO_NATS tuple as IPv4 or ignore its trailing address octets.
func TestMobikeNoNATsIPv6Tuple(t *testing.T) {
	for _, offset := range []int{-1, 15, 31, 33, 35} {
		t.Run(strconv.Itoa(offset), func(t *testing.T) {
			ini, resp, ps := establishPSK(t)
			remote := &net.UDPAddr{IP: net.ParseIP("2001:db8:1::11"), Port: 43001}
			local := &net.UDPAddr{IP: net.ParseIP("2001:db8:2::22"), Port: 43002}
			resp.mobike.enabled = true
			resp.mobike.local = local
			resp.peerEndpoint = remote
			tuple := mbNoNATs(remote, local)
			if offset >= 0 {
				tuple[offset] ^= 1
			}
			cookie := []byte("ipv6-cookie2")
			request := mbInformational(t, ini, resp.ExpectedMsgID, false, []wire.PayloadEntry{
				mbNotify(wire.NotifyNoAdditionalAddresses, nil),
				mbNotify(wire.NotifyNoNATsAllowed, tuple), mbNotify(wire.NotifyCookie2, cookie),
			})
			ps.handleOwnedInbound(resp, transport.Packet{
				Data: request, RemoteAddr: remote, LocalAddr: local,
			}, nil, &mbDataplane{}, slogutil.DiscardLogger())
			payloads := mbResponse(t, ini, resp.lastResponse, parseMsg(t, request).Header.MessageID)
			if offset == -1 {
				mbOnlyNotify(t, payloads, wire.NotifyCookie2, cookie)
				return
			}
			mbErrorEcho(t, payloads, wire.NotifyUnexpectedNATDetected, cookie)
		})
	}
}

// TestMobikeAdvertisementAfterSourceChange checks the wire history of an update
// retransmitted from a second source. The old answer cannot authorize migration;
// a new request must advertise the current source and carry a fresh COOKIE2.
// RFC requirement: RFC4555-4.2.5-1 positive -- consecutive generated COOKIE2 values
// differ, and the fresh token remains within the 8..64-octet bounds.
// RFC requirement: RFC4555-3.6-1 positive -- a source-changing retransmission is
// followed by a new INFORMATIONAL request at the next Message ID.
// RFC requirement: RFC4555-3.6-1 negative -- retransmission on the same source needs
// no fresh advertisement; the next emitted request is the deliberately sent DPD.
// RFC 4555 Section 3.6: "If the request to update the addresses is retransmitted
// using several different source addresses, a new INFORMATIONAL request MUST be sent."
// MUTATION: remove pending.changed handling from setMobikePath or handleMobikeResponse.
// RFC requirement: RFC4555-3.9-1 positive -- the original and replacement
// advertisements carry their own protected tuples, while retransmission keeps
// the original authenticated bytes.
func TestMobikeAdvertisementAfterSourceChange(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("explicit wildcard UDP source selection requires Linux IP_PKTINFO")
	}
	for _, changed := range []bool{false, true} {
		t.Run(strconv.FormatBool(changed), func(t *testing.T) {
			f := mbInitiator(t)
			f.local.PeerCfg.ProhibitNAT = true
			f.myTr = mbTransport(t, "0.0.0.0:0", false)
			firstLocal := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: nttPort(t, f.myTr)}
			f.local.mobike.local = firstLocal
			log := slogutil.DiscardLogger()
			if err := f.ps.startMobikeRequest(f.local, f.myTr, true, log); err != nil {
				t.Fatal(err)
			}
			first := mbReceive(t, f.peerTr)
			mbEndpoint(t, first.RemoteAddr, firstLocal)
			firstID := parseMsg(t, first.Data).Header.MessageID
			firstInner := mbDecrypt(t, f.peer, first.Data)
			mbRequireNotify(t, firstInner, wire.NotifyNoAdditionalAddresses)
			protected := mbRequireNotify(t, firstInner, wire.NotifyNoNATsAllowed)
			if !bytes.Equal(protected.NotificationData, mbNoNATs(firstLocal, nttPeerAddr(t, f.peerTr))) {
				t.Fatal("first update did not protect its actual send tuple")
			}
			firstCookie := mbRequireNotify(t, firstInner, wire.NotifyCookie2).NotificationData
			currentLocal := firstLocal
			if changed {
				currentLocal = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: firstLocal.Port}
				setMobikePath(f.local, currentLocal, nttPeerAddr(t, f.peerTr))
			}
			f.ps.serviceRequestRetransmit(f.local, &dpdState{}, f.myTr,
				time.Now().Add(retransmitBackoff(1)+time.Second), log)
			repeated := mbReceive(t, f.peerTr)
			mbEndpoint(t, repeated.RemoteAddr, currentLocal)
			if !bytes.Equal(repeated.Data, first.Data) {
				t.Fatal("source change rebuilt the outstanding request instead of retransmitting it")
			}
			answer := mbInformational(t, f.peer, firstID, true,
				[]wire.PayloadEntry{mbNotify(wire.NotifyCookie2, firstCookie)})
			out := f.ps.handleOwnedInbound(f.local, transport.Packet{
				Data: answer, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: currentLocal,
			}, f.myTr, f.dp, log)
			if out.reestablish {
				t.Fatal("a matching answer closed the SA")
			}
			if !changed {
				mbCheckMigration(t, f, currentLocal, nttPeerAddr(t, f.peerTr))
				sendDPD(f.local, f.myTr, &dpdState{}, log)
				next := mbReceive(t, f.peerTr)
				if parseMsg(t, next.Data).Header.MessageID != firstID+1 {
					t.Fatal("same-source answer spent an extra Message ID")
				}
				mbProbeRequest(t, f.peer, next.Data)
				return
			}
			if f.dp.count != 0 {
				t.Fatal("an answer to the source-ambiguous request authorized migration")
			}
			fresh := mbReceive(t, f.peerTr)
			mbEndpoint(t, fresh.RemoteAddr, currentLocal)
			freshID := parseMsg(t, fresh.Data).Header.MessageID
			if freshID != firstID+1 {
				t.Fatalf("fresh advertisement id = %d, want %d", freshID, firstID+1)
			}
			if parseMsg(t, fresh.Data).Header.Flags&wire.FlagResponse != 0 {
				t.Fatal("fresh advertisement is marked as a response")
			}
			freshInner := mbDecrypt(t, f.peer, fresh.Data)
			mbRequireNotify(t, freshInner, wire.NotifyUpdateSAAddresses)
			mbRequireNotify(t, freshInner, wire.NotifyNoAdditionalAddresses)
			protected = mbRequireNotify(t, freshInner, wire.NotifyNoNATsAllowed)
			if !bytes.Equal(protected.NotificationData, mbNoNATs(currentLocal, nttPeerAddr(t, f.peerTr))) {
				t.Fatal("fresh update retained the old protected tuple")
			}
			freshCookie := mbRequireNotify(t, freshInner, wire.NotifyCookie2).NotificationData
			mbCookieLength(t, freshCookie)
			if bytes.Equal(freshCookie, firstCookie) {
				t.Fatal("fresh advertisement reused the earlier routability token")
			}
			answer = mbInformational(t, f.peer, freshID, true,
				[]wire.PayloadEntry{mbNotify(wire.NotifyCookie2, freshCookie)})
			f.ps.handleOwnedInbound(f.local, transport.Packet{
				Data: answer, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: currentLocal,
			}, f.myTr, f.dp, log)
			mbCheckMigration(t, f, currentLocal, nttPeerAddr(t, f.peerTr))
		})
	}
}

// TestMobikeAuthNegotiation uses the actual IKE_AUTH send and receive paths, and
// proves the negotiated capability through subsequent encrypted COOKIE2 exchanges.
// RFC requirement: RFC4555-4.2.1-1 positive -- both generated MOBIKE_SUPPORTED
// notifications have empty data, Protocol ID zero, and no SPI.
// RFC requirement: RFC4555-4.2.1-1 negative -- nonempty received data on either
// IKE_AUTH direction leaves negotiation usable, rather than rejecting the extension.
// RFC requirement: RFC4555-x-1 positive -- both peers announce support in IKE_AUTH.
// RFC requirement: RFC4555-x-1 negative -- an omitted offer prevents that receiver
// from enabling COOKIE2, even though authentication and ordinary INFORMATIONAL work.
// RFC requirement: RFC4555-x-2 positive -- IKE_AUTH is sent on the bound NAT-T
// socket with the non-ESP marker despite matching initial NAT detection hashes.
// RFC 4555 Section 4.2.1: "The notification data field MUST be left empty
// (zero-length) when sending, and its contents (if any) MUST be ignored when this
// notification is received."
// MUTATION: write MOBIKE_SUPPORTED data in mobikeAuthOffer, reject nonempty data
// in acceptMobikeOffer, omit one AUTH offer, or skip the no-NAT float.
func TestMobikeAuthNegotiation(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		requestData, responseData []byte
		omitRequest, omitResponse bool
	}{
		{name: "empty"},
		{name: "request-extension-data", requestData: []byte{0x11, 0x22, 0x33}},
		{name: "response-extension-data", responseData: []byte{0x44, 0x55}},
		{name: "no-request-offer", omitRequest: true},
		{name: "no-response-offer", omitResponse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ini, resp, ps, iniTr, respTr := mbAuthHandshake(t, "",
				tc.requestData, tc.responseData, tc.omitRequest, tc.omitResponse, false)
			for _, role := range []struct {
				name            string
				local, peer     *SA
				ps              *PeerSession
				localTr, peerTr *transport.UDPTransport
				enabled         bool
			}{
				{"responder", resp, ini, ps, respTr, iniTr, !tc.omitRequest},
				{"initiator", ini, resp, &PeerSession{peerName: ini.PeerName}, iniTr, respTr,
					!tc.omitRequest && !tc.omitResponse},
			} {
				t.Run(role.name, func(t *testing.T) {
					cookie := []byte("auth-cookie2")
					request := mbInformational(t, role.peer, role.local.ExpectedMsgID, false,
						[]wire.PayloadEntry{mbNotify(wire.NotifyCookie2, cookie)})
					role.ps.handleOwnedInbound(role.local, transport.Packet{
						Data: request, RemoteAddr: nttPeerAddr(t, role.peerTr),
						LocalAddr: nttPeerAddr(t, role.localTr), NATT: true,
					}, role.localTr, nil, slogutil.DiscardLogger())
					answer := mbReceive(t, role.peerTr)
					payloads := mbResponse(t, role.peer, answer.Data, parseMsg(t, request).Header.MessageID)
					if role.enabled {
						mbOnlyNotify(t, payloads, wire.NotifyCookie2, cookie)
						return
					}
					if len(payloads) != 0 {
						t.Fatalf("unnegotiated MOBIKE produced payloads: %+v", payloads)
					}
				})
			}
		})
	}
}

// TestMobikeAuthWithoutNATTSocketKeepsBaseIKE contrasts the no-NAT MOBIKE float
// with an ordinary PSK exchange whose transport cannot negotiate the extension.
// RFC requirement: RFC4555-x-2 negative -- without a NAT-T socket, the generated
// IKE_AUTH contains no MOBIKE_SUPPORTED and the base handshake still authenticates.
// MUTATION: offer MOBIKE when mobikeAvailable has no usable NAT-T socket.
func TestMobikeAuthWithoutNATTSocketKeepsBaseIKE(t *testing.T) {
	ini, resp, _ := establishPSK(t)
	for _, message := range []struct {
		receiver *SA
		raw      []byte
	}{{resp, ini.LastSentMsg}, {ini, resp.LastSentMsg}} {
		for _, entry := range mbDecrypt(t, message.receiver, message.raw) {
			n, ok := entry.Payload.(*wire.PayloadNotify)
			if !ok {
				continue
			}
			if n.NotifyMsgType == wire.NotifyMobikeSupported {
				t.Fatal("base IKE_AUTH advertised unusable MOBIKE")
			}
		}
	}
}

// TestMobikePathProbeRetainsTunnelEndpoint proves both the NAT-detection answer and
// the absence of dynamic endpoint adoption on a path probe. The subsequent DPD must
// reach the original socket, even though the request arrived from another socket.
// RFC requirement: RFC4555-3.8-1 positive -- an authenticated probe without UPDATE
// neither migrates the Child nor redirects the next self-initiated IKE request.
// RFC requirement: RFC4555-3.8-1 negative -- the alternate path still receives its
// encrypted response, so retaining the endpoint is not a blanket packet discard.
// RFC requirement: RFC4555-3.8-2 positive -- the INFORMATIONAL response contains
// source and destination NAT detection hashes for its actual observed UDP tuple.
// RFC requirement: RFC4555-3.8-2 negative -- an ordinary empty request receives an
// empty response, without unsolicited NAT detection notifications.
// RFC 4555 Section 3.8: "The host not behind a NAT MUST NOT use these dynamic
// updates for IKEv2 packets, but MAY use them for ESP packets."
// MUTATION: permit adoptAuthenticatedEndpoint on MOBIKE probes, or omit/reverse
// the hashes produced by mobikeResponsePayloads.
func TestMobikePathProbeRetainsTunnelEndpoint(t *testing.T) {
	for _, natDetection := range []bool{false, true} {
		t.Run(strconv.FormatBool(natDetection), func(t *testing.T) {
			ini, resp, ps := establishPSK(t)
			oldTr := mbTransport(t, "127.0.0.1:0", false)
			peerTr := mbTransport(t, "127.0.0.1:0", false)
			myTr := mbTransport(t, "127.0.0.1:0", false)
			local, remote := nttPeerAddr(t, myTr), nttPeerAddr(t, peerTr)
			resp.mobike.enabled = true
			resp.mobike.local = local
			resp.peerEndpoint = nttPeerAddr(t, oldTr)
			var inner []wire.PayloadEntry
			if natDetection {
				inner = []wire.PayloadEntry{
					mbNotify(wire.NotifyNATDetectionSourceIP,
						transport.NATDetectionHash(ini.InitiatorSPI, ini.ResponderSPI, remote.IP, uint16(remote.Port))),
					mbNotify(wire.NotifyNATDetectionDestIP,
						transport.NATDetectionHash(ini.InitiatorSPI, ini.ResponderSPI, local.IP, uint16(local.Port))),
				}
			}
			request := mbInformational(t, ini, resp.ExpectedMsgID, false, inner)
			dp := &mbDataplane{}
			ps.handleOwnedInbound(resp, transport.Packet{
				Data: request, RemoteAddr: remote, LocalAddr: local,
			}, myTr, dp, slogutil.DiscardLogger())
			answer := mbReceive(t, peerTr)
			payloads := mbResponse(t, ini, answer.Data, parseMsg(t, request).Header.MessageID)
			if natDetection {
				if len(payloads) != 2 {
					t.Fatalf("NAT detection response has %d payloads, want 2", len(payloads))
				}
				source := mbRequireNotify(t, payloads, wire.NotifyNATDetectionSourceIP)
				dest := mbRequireNotify(t, payloads, wire.NotifyNATDetectionDestIP)
				if !bytes.Equal(source.NotificationData,
					transport.NATDetectionHash(ini.InitiatorSPI, ini.ResponderSPI, local.IP, uint16(local.Port))) {
					t.Fatal("NAT detection source hash does not describe the response source")
				}
				if !bytes.Equal(dest.NotificationData,
					transport.NATDetectionHash(ini.InitiatorSPI, ini.ResponderSPI, remote.IP, uint16(remote.Port))) {
					t.Fatal("NAT detection destination hash does not describe the request sender")
				}
			} else if len(payloads) != 0 {
				t.Fatalf("empty request received unsolicited payloads: %+v", payloads)
			}
			if dp.count != 0 {
				t.Fatal("a path probe without UPDATE migrated the Child")
			}
			sendDPD(resp, myTr, &dpdState{}, slogutil.DiscardLogger())
			probe := mbReceive(t, oldTr)
			mbProbeRequest(t, ini, probe.Data)
		})
	}
}

// TestMobikeNoNATsUpdateWaitsForRoutability drives a matching tuple through the
// responder's UPDATE, owner tick and COOKIE2 response, then checks the dataplane
// migration. Retransmitting UPDATE cannot repeat the migration.
// RFC requirement: RFC4555-3.9-3 positive -- a matching IPv4 tuple permits the
// requested endpoint migration after return routability succeeds.
// RFC requirement: RFC4555-3.9-4 negative -- accepted header endpoints reach the
// migration API; the rejected notification cases cannot do so.
// MUTATION: accept every NO_NATS tuple, migrate before COOKIE2, or replay the update
// while handling its duplicate instead of sending the cached response.
func TestMobikeNoNATsUpdateWaitsForRoutability(t *testing.T) {
	ini, resp, _ := establishPSK(t)
	f := mbOwner(t, resp, ini, false)
	f.local.PeerCfg.ProhibitNAT = true
	log := slogutil.DiscardLogger()
	local, remote := nttPeerAddr(t, f.myTr), nttPeerAddr(t, f.peerTr)
	cookie := []byte("peer-update-cookie")
	request := mbInformational(t, f.peer, f.local.ExpectedMsgID, false, []wire.PayloadEntry{
		mbNotify(wire.NotifyUpdateSAAddresses, nil),
		mbNotify(wire.NotifyNoNATsAllowed, mbNoNATs(remote, local)),
		mbNotify(wire.NotifyCookie2, cookie),
	})
	pkt := transport.Packet{Data: request, RemoteAddr: remote, LocalAddr: local}
	f.ps.handleOwnedInbound(f.local, pkt, f.myTr, f.dp, log)
	accepted := mbReceive(t, f.peerTr)
	mbOnlyNotify(t, mbResponse(t, f.peer, accepted.Data, parseMsg(t, request).Header.MessageID),
		wire.NotifyCookie2, cookie)
	if f.dp.count != 0 {
		t.Fatal("valid NO_NATS update migrated before return routability completed")
	}
	f.ps.serviceMobike(f.local, f.myTr, time.Now(), log)
	check := mbReceive(t, f.peerTr)
	checkID := parseMsg(t, check.Data).Header.MessageID
	if parseMsg(t, check.Data).Header.Flags&wire.FlagResponse != 0 {
		t.Fatal("responder did not initiate a return-routability request")
	}
	checkCookie := mbRequireNotify(t, mbDecrypt(t, f.peer, check.Data), wire.NotifyCookie2).NotificationData
	mbCookieLength(t, checkCookie)
	answer := mbInformational(t, f.peer, checkID, true, []wire.PayloadEntry{mbNotify(wire.NotifyCookie2, checkCookie)})
	f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: answer, RemoteAddr: remote, LocalAddr: local,
	}, f.myTr, f.dp, log)
	mbCheckMigration(t, f, local, remote)
	f.ps.handleOwnedInbound(f.local, pkt, f.myTr, f.dp, log)
	replay := mbReceive(t, f.peerTr)
	if !bytes.Equal(replay.Data, accepted.Data) {
		t.Fatal("post-migration UPDATE retransmission changed its response")
	}
	if f.dp.count != 1 {
		t.Fatalf("UPDATE retransmission migrated %d times, want one original migration", f.dp.count)
	}
}

// TestMobikeOriginalRoleSurvivesRekey answers a pending request after inheriting
// the first SA's MOBIKE role into an IKE SA whose I-bit role is reversed. Only the
// first SA's initiator may announce UPDATE_SA_ADDRESSES after changing paths.
// RFC 4555 Section 1.3: "In this document, the term \"initiator\" means the party
// who originally initiated the first IKE_SA (in a series of possibly several
// rekeyed IKE_SAs); \"responder\" is the other peer."
// MUTATION: use IsInitiator instead of mobike.originalInitiator for the fresh update.
func TestMobikeOriginalRoleSurvivesRekey(t *testing.T) {
	for _, originalInitiator := range []bool{false, true} {
		t.Run(strconv.FormatBool(originalInitiator), func(t *testing.T) {
			ini, resp, _ := establishPSK(t)
			local, peer := ini, resp
			if originalInitiator {
				local, peer = resp, ini
			}
			f := mbOwner(t, local, peer, false)
			// The current pair supplies the rekey's SK directions. Its predecessor
			// had the opposite IKE role, as when the other peer initiated the rekey.
			old := &SA{IsInitiator: originalInitiator}
			old.mobike.enabled = true
			old.mobike.originalInitiator = originalInitiator
			old.mobike.local = nttPeerAddr(t, f.myTr)
			old.peerEndpoint = nttPeerAddr(t, f.peerTr)
			f.local.inheritSendPath(old)
			log := slogutil.DiscardLogger()
			if err := f.ps.startMobikeRequest(f.local, f.myTr, originalInitiator, log); err != nil {
				t.Fatal(err)
			}
			first := mbReceive(t, f.peerTr)
			firstCookie := mbRequireNotify(t, mbDecrypt(t, f.peer, first.Data), wire.NotifyCookie2).NotificationData
			nextTr := mbTransport(t, "127.0.0.1:0", false)
			setMobikePath(f.local, nttPeerAddr(t, f.myTr), nttPeerAddr(t, nextTr))
			response := mbInformational(t, f.peer, parseMsg(t, first.Data).Header.MessageID, true,
				[]wire.PayloadEntry{mbNotify(wire.NotifyCookie2, firstCookie)})
			f.ps.handleOwnedInbound(f.local, transport.Packet{
				Data: response, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: nttPeerAddr(t, f.myTr),
			}, f.myTr, f.dp, log)
			fresh := mbReceive(t, nextTr)
			inner := mbDecrypt(t, f.peer, fresh.Data)
			mbRequireNotify(t, inner, wire.NotifyCookie2)
			hasUpdate := false
			for _, entry := range inner {
				n, ok := entry.Payload.(*wire.PayloadNotify)
				if !ok {
					continue
				}
				if n.NotifyMsgType == wire.NotifyUpdateSAAddresses {
					hasUpdate = true
				}
			}
			if hasUpdate != originalInitiator {
				t.Fatalf("fresh UPDATE = %v, first IKE role initiator = %v", hasUpdate, originalInitiator)
			}
		})
	}
}

// TestMobikeMappedPortSurvivesChildRekey migrates an established Child to a
// NAT-mapped peer port, then completes CREATE_CHILD_SA through the owner entry.
// The replacement's installed outbound SA must keep the mapped peer port.
// MUTATION: replace inherited udpLocalPort/udpRemotePort with 4500 in either rekey
// construction or installChildSA.
func TestMobikeMappedPortSurvivesChildRekey(t *testing.T) {
	ini, resp, _ := establishPSK(t)
	f := mbOwner(t, ini, resp, true)
	log := slogutil.DiscardLogger()
	local, remote := nttPeerAddr(t, f.myTr), nttPeerAddr(t, f.peerTr)
	peerChild, err := createFirstChildSA(f.peer, testESPGroup(),
		f.peer.PeerCfg.LocalAddress, f.peer.PeerCfg.RemoteAddress, 0, nil, log)
	if err != nil {
		t.Fatalf("peer initial Child: %v", err)
	}
	if err := f.ps.startMobikeRequest(f.local, f.myTr, true, log); err != nil {
		t.Fatal(err)
	}
	request := mbReceive(t, f.peerTr)
	cookie := mbRequireNotify(t, mbDecrypt(t, f.peer, request.Data), wire.NotifyCookie2).NotificationData
	answer := mbInformational(t, f.peer, parseMsg(t, request.Data).Header.MessageID, true,
		[]wire.PayloadEntry{
			mbNotify(wire.NotifyCookie2, cookie),
			// The peer names its private source; the received UDP port/address
			// differs, which is the NAT detection trigger under test.
			mbNotify(wire.NotifyNATDetectionSourceIP,
				transport.NATDetectionHash(f.local.InitiatorSPI, f.local.ResponderSPI,
					net.IPv4(10, 0, 0, 2), transport.NATTPort)),
			mbNotify(wire.NotifyNATDetectionDestIP,
				transport.NATDetectionHash(f.local.InitiatorSPI, f.local.ResponderSPI,
					local.IP, uint16(local.Port))),
		})
	f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: answer, RemoteAddr: remote, LocalAddr: local,
	}, f.myTr, f.dp, log)
	mbCheckMigration(t, f, local, remote)
	if !f.dp.migrations[0].NATDetected {
		t.Fatal("NAT detection mismatch did not request ESP encapsulation at migration")
	}
	f.ps.startChildRekey(f.local, f.myTr, log)
	rekey := mbReceive(t, f.peerTr)
	if parseMsg(t, rekey.Data).Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Fatal("rekey did not emit CREATE_CHILD_SA")
	}
	rekeyAnswer, replacement, err := respondChildRekey(f.peer, mbDecrypt(t, f.peer, rekey.Data),
		peerChild, parseMsg(t, rekey.Data).Header.MessageID, nil, log)
	if err != nil {
		t.Fatalf("peer answers Child rekey: %v", err)
	}
	if replacement == nil {
		t.Fatal("peer refused the Child rekey")
	}
	out := f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: rekeyAnswer, RemoteAddr: remote, LocalAddr: local,
	}, f.myTr, f.dp, log)
	if out.newChild == nil {
		t.Fatal("Child rekey did not install a replacement")
	}
	if len(f.dp.sas) != 4 {
		t.Fatalf("installed SA count = %d, want original and replacement pairs", len(f.dp.sas))
	}
	outboundCount := 0
	for _, installed := range f.dp.sas[2:] {
		if installed.Dir != dataplane.SADirOut {
			continue
		}
		outboundCount++
		if !installed.UDPEncap {
			t.Fatalf("rekey direction %d lost UDP encapsulation", installed.Dir)
		}
		source, dest := uint16(local.Port), uint16(remote.Port)
		if installed.UDPEncapSPort != source {
			t.Fatalf("rekey direction %d source port = %d, want %d",
				installed.Dir, installed.UDPEncapSPort, source)
		}
		if installed.UDPEncapDPort != dest {
			t.Fatalf("rekey direction %d destination port = %d, want %d",
				installed.Dir, installed.UDPEncapDPort, dest)
		}
	}
	if outboundCount != 1 {
		t.Fatalf("replacement outbound SA count = %d, want 1", outboundCount)
	}
}

// TestMobikeChangedPathStillChecksCookie2 prevents the fresh-advertisement branch
// from treating a changed path as permission to accept an invalid routability token.
// RFC requirement: RFC4555-3.7-4 negative -- a mismatched COOKIE2 remains invalid
// when the outstanding request's path has changed.
// RFC requirement: RFC4555-3.7-5 positive -- the mismatch still requests teardown.
// MUTATION: process pending.changed before comparing COOKIE2 in handleMobikeResponse.
func TestMobikeChangedPathStillChecksCookie2(t *testing.T) {
	f := mbInitiator(t)
	log := slogutil.DiscardLogger()
	if err := f.ps.startMobikeRequest(f.local, f.myTr, true, log); err != nil {
		t.Fatal(err)
	}
	request := mbReceive(t, f.peerTr)
	cookie := bytes.Clone(mbRequireNotify(t, mbDecrypt(t, f.peer, request.Data), wire.NotifyCookie2).NotificationData)
	cookie[0] ^= 1
	nextTr := mbTransport(t, "127.0.0.1:0", false)
	setMobikePath(f.local, nttPeerAddr(t, f.myTr), nttPeerAddr(t, nextTr))
	answer := mbInformational(t, f.peer, parseMsg(t, request.Data).Header.MessageID, true,
		[]wire.PayloadEntry{mbNotify(wire.NotifyCookie2, cookie)})
	out := f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: answer, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: nttPeerAddr(t, f.myTr),
	}, f.myTr, f.dp, log)
	if !out.reestablish {
		t.Fatal("path change let a mismatched COOKIE2 avoid owner-loop teardown")
	}
	if f.dp.count != 0 {
		t.Fatal("path change let a mismatched COOKIE2 authorize migration")
	}
}

// RFC requirement: RFC4555-3.9-1 positive -- parsed prohibition protects the
// first IKE_AUTH and subsequent address updates with the actual send tuple.
// RFC requirement: RFC4555-3.9-1 negative -- the existing default and explicit
// allow policy retain NAT-T without unsolicited NO_NATS_ALLOWED.
// RFC 4555 Section 3.9: address-updating messages "MUST also include a
// NO_NATS_ALLOWED notification" when NAT Traversal is not enabled.
// MUTATION: drop either sender, ignore the configured policy, or use configured
// addresses rather than the live source/destination ports.
func TestMobikeConfiguredNATPolicy(t *testing.T) {
	for _, policy := range []string{"", "allow", "prohibit"} {
		t.Run("policy="+policy, func(t *testing.T) {
			ini, resp, _, iniTr, respTr := mbAuthHandshake(t, policy, nil, nil, false, false, false)
			ps := &PeerSession{peerName: ini.PeerName}
			if err := ps.startMobikeRequest(ini, iniTr, true, slogutil.DiscardLogger()); err != nil {
				t.Fatal(err)
			}
			request := mbReceive(t, respTr)
			inner := mbDecrypt(t, resp, request.Data)
			notification := notifyOf(inner, wire.NotifyNoNATsAllowed)
			if policy != "prohibit" {
				if notification != nil {
					t.Fatal("NAT-allowed update prohibited translation")
				}
				return
			}
			if notification == nil || !bytes.Equal(notification.NotificationData,
				mbNoNATs(request.RemoteAddr, nttPeerAddr(t, respTr))) {
				t.Fatal("prohibited update did not protect its observed tuple")
			}
		})
	}
	if _, err := mbNATPolicyConfig("ignore"); err == nil {
		t.Fatal("unknown NAT policy was accepted")
	}
}

// NAT capability is not permission. The private source hash is authenticated
// as part of IKE_SA_INIT; the responder observes the translated loopback source.
// Both policies send IKE_AUTH, but prohibition refuses it before any Child install.
func TestMobikeNATPolicyDuringAuthentication(t *testing.T) {
	for _, policy := range []string{"", "prohibit"} {
		t.Run("policy="+policy, func(t *testing.T) {
			mbAuthHandshake(t, policy, nil, nil, false, false, true)
		})
	}
}

// RFC 4555 Section 3.9 starts a new IKE_SA_INIT after UNEXPECTED_NAT_DETECTED.
// Valid AUTH and Child payloads beside that notification cannot cancel rejection.
func TestMobikeNATRejectionOverridesSuccessfulAuthPayloads(t *testing.T) {
	ini, resp, _, _, _ := mbAuthHandshake(t, "", nil, nil, false, false, false)
	response := resp.LastSentMsg
	header := parseMsg(t, response).Header
	inner := mbDecrypt(t, ini, response)
	inner = append(inner, mbNotify(wire.NotifyUnexpectedNATDetected, nil))
	rejected, err := buildEncryptedMessageEx(resp, inner, header.MessageID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		t.Fatal(err)
	}
	ini.State = StateAuthSent
	handleAuthResponse(ini, parseMsg(t, rejected), rejected, nil, nil, slogutil.DiscardLogger())
	if ini.State != StateDead {
		t.Fatal("successful AUTH payloads overrode UNEXPECTED_NAT_DETECTED")
	}
}

// RFC requirement: RFC4555-3.9-1 positive -- the handshake timer retransmits
// the original protected IKE_AUTH from its floated source, not the INIT socket.
// MUTATION: send the handshake retransmit directly through the original transport.
func TestMobikeAuthRetransmitKeepsProtectedTuple(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniTr := mbTransport(t, "127.0.0.1:0", true)
	peerTr := mbTransport(t, "127.0.0.1:0", true)
	oldPort, oldAfter := ikeTestPortFn, afterFunc
	ikeTestPortFn = func() string { return strconv.Itoa(nttPort(t, peerTr) - 1) }
	t.Cleanup(func() { ikeTestPortFn, afterFunc = oldPort, oldAfter })
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "mobike-retry")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	iniPeer.ProhibitNAT = true
	group, esp := testIKEGroup(), testESPGroup()
	ps := &PeerSession{peerName: "retry", peerCfg: iniPeer, espGroup: esp, natt: iniTr, stopCh: make(chan struct{})}
	table := NewSATable()
	var responder *SA
	var first transport.Packet
	round := 0
	afterFunc = func(time.Duration) <-chan time.Time {
		current := ps.getSA()
		if round == 0 {
			current.mobike.canMigrate = true
			var err error
			responder, err = newResponderSA("retry", respPeer, group, esp, current.InitiatorSPI)
			if err != nil {
				t.Fatal(err)
			}
			initial := current.LastSentMsg
			handleSAInitRequest(responder, parseMsg(t, initial), initial, nil, nil, log)
			handleInbound(current, transport.Packet{Data: responder.LastSentMsg}, table, nil, log)
			first = mbReceive(t, peerTr)
			if parseMsg(t, first.Data).Header.ExchangeType != wire.ExchangeIKEAuth {
				t.Fatal("first floated packet is not IKE_AUTH")
			}
			current.RetransmitTime = time.Now().Add(-time.Second)
		} else {
			repeated := mbReceive(t, peerTr)
			mbEndpoint(t, repeated.RemoteAddr, nttPeerAddr(t, iniTr))
			if !bytes.Equal(first.Data, repeated.Data) {
				t.Fatal("AUTH retransmission rebuilt the authenticated request")
			}
			protected := mbRequireNotify(t, mbDecrypt(t, responder, repeated.Data), wire.NotifyNoNATsAllowed)
			if !bytes.Equal(protected.NotificationData, mbNoNATs(repeated.RemoteAddr, nttPeerAddr(t, peerTr))) {
				t.Fatal("AUTH retransmission changed its protected transport tuple")
			}
			close(ps.stopCh)
		}
		round++
		tick := make(chan time.Time, 1)
		tick <- time.Now()
		return tick
	}
	if err := ps.runInitiator(iniPeer, group, table, nil, nil, log); !errors.Is(err, errStopped) {
		t.Fatalf("handshake stop: %v", err)
	}
	if round != 2 {
		t.Fatalf("handshake timer serviced %d rounds, want initial response and retransmission", round)
	}
}

func TestMobikeProhibitionRejectsUnprotectedUpdate(t *testing.T) {
	ini, resp, _ := establishPSK(t)
	f := mbOwner(t, resp, ini, false)
	f.local.PeerCfg.ProhibitNAT = true
	alternate := mbTransport(t, "127.0.0.1:0", false)
	cookie := []byte("unprotected-update")
	request := mbInformational(t, f.peer, f.local.ExpectedMsgID, false,
		[]wire.PayloadEntry{
			mbNotify(wire.NotifyUpdateSAAddresses, nil),
			mbNotify(wire.NotifyCookie2, cookie),
		})
	log := slogutil.DiscardLogger()
	f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: request, RemoteAddr: nttPeerAddr(t, alternate), LocalAddr: nttPeerAddr(t, f.myTr),
	}, f.myTr, f.dp, log)
	answer := mbReceive(t, alternate)
	mbErrorEcho(t, mbDecrypt(t, f.peer, answer.Data), wire.NotifyUnexpectedNATDetected, cookie)
	if f.dp.count != 0 {
		t.Fatal("an unprotected update migrated a NAT-prohibited Child")
	}
	sendDPD(f.local, f.myTr, &dpdState{}, log)
	mbProbeRequest(t, f.peer, mbReceive(t, f.peerTr).Data)
}

// A peer cannot turn a prohibiting tunnel into a NAT-traversing tunnel merely
// by echoing COOKIE2. The same NAT detection response succeeds under the default
// policy in TestMobikeMappedPortSurvivesChildRekey.
func TestMobikeProhibitionRejectsTranslatedResponse(t *testing.T) {
	f := mbInitiator(t)
	f.local.PeerCfg.ProhibitNAT = true
	log := slogutil.DiscardLogger()
	if err := f.ps.startMobikeRequest(f.local, f.myTr, true, log); err != nil {
		t.Fatal(err)
	}
	request := mbReceive(t, f.peerTr)
	cookie := mbRequireNotify(t, mbDecrypt(t, f.peer, request.Data), wire.NotifyCookie2).NotificationData
	local, remote := nttPeerAddr(t, f.myTr), nttPeerAddr(t, f.peerTr)
	answer := mbInformational(t, f.peer, parseMsg(t, request.Data).Header.MessageID, true,
		[]wire.PayloadEntry{
			mbNotify(wire.NotifyCookie2, cookie),
			mbNotify(wire.NotifyNATDetectionSourceIP,
				transport.NATDetectionHash(f.local.InitiatorSPI, f.local.ResponderSPI,
					net.IPv4(10, 0, 0, 2), transport.NATTPort)),
			mbNotify(wire.NotifyNATDetectionDestIP,
				transport.NATDetectionHash(f.local.InitiatorSPI, f.local.ResponderSPI,
					local.IP, uint16(local.Port))),
		})
	out := f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: answer, RemoteAddr: remote, LocalAddr: local,
	}, f.myTr, f.dp, log)
	if !out.reestablish || f.local.State != StateDead || f.dp.count != 0 {
		t.Fatal("a matching COOKIE2 overrode the configured NAT prohibition")
	}
	f.ps.cleanupChild(f.dp, nil, log)
	if len(f.dp.removed) != 2 || !slices.Contains(f.dp.removed, f.child.InboundSPI) ||
		!slices.Contains(f.dp.removed, f.child.OutboundSPI) {
		t.Fatalf("prohibited path retained Child states: removed %v", f.dp.removed)
	}
}

func mbNATPolicyConfig(policy string) (*ipsec.IPsecConfig, error) {
	tree := config.NewTree()
	peers := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec").GetOrCreateContainer("site-to-site")
	peer := config.NewTree()
	if policy != "" {
		peer.Set("nat-traversal", policy)
	}
	peers.AddListEntry("peer", "ze", peer)
	return ipsec.ParseIPsecConfig(tree)
}

// mbDataplane records the migration boundary separately from ordinary installation.
// A remove/install implementation is visible through mockDP and fails the assertions.
// The fixed array bounds this fixture to four migrations per exchange scenario.
type mbDataplane struct {
	mockDP
	migrations [4]dataplane.TunnelMigration
	count      int
}

func (dp *mbDataplane) MigrateTunnel(m dataplane.TunnelMigration) error {
	if dp.count == len(dp.migrations) {
		return fmt.Errorf("unexpected fifth tunnel migration")
	}
	dp.migrations[dp.count] = m
	dp.count++
	return nil
}

type mbFixture struct {
	local, peer         *SA
	ps                  *PeerSession
	child               *ChildSA
	dp                  *mbDataplane
	peerTr, myTr        *transport.UDPTransport
	policies            []dataplane.SPParams
	oldLocal, oldRemote net.IP
}

// mbInitiator prepares a keyed SA with an installed Child and a different candidate
// outer path. Only setup enables MOBIKE here; the authentication tests negotiate it.
func mbInitiator(t *testing.T) *mbFixture {
	t.Helper()
	ini, resp, _ := establishPSK(t)
	return mbOwner(t, ini, resp, false)
}

func mbOwner(t *testing.T, local, peer *SA, natt bool) *mbFixture {
	t.Helper()
	var peerTr, myTr *transport.UDPTransport
	if natt {
		peerTr = mbTransport(t, "127.0.0.1:0", true)
		myTr = mbTransport(t, "127.0.0.1:0", true)
		local.bindSockets(nil, myTr)
		local.floatToNATTPort()
	} else {
		peerTr, myTr = rtxPeerLink(t)
	}
	dp := &mbDataplane{}
	child, err := createFirstChildSA(local, testESPGroup(),
		local.PeerCfg.LocalAddress, local.PeerCfg.RemoteAddress, 7, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("install initial Child: %v", err)
	}
	ps := &PeerSession{peerName: local.PeerName, peerCfg: local.PeerCfg, espGroup: testESPGroup()}
	ps.setChildSA(child)
	local.mobike.enabled = true
	local.mobike.originalInitiator = local.IsInitiator
	local.mobike.local = nttPeerAddr(t, myTr)
	local.peerEndpoint = nttPeerAddr(t, peerTr)
	return &mbFixture{
		local: local, peer: peer, ps: ps, child: child, dp: dp, peerTr: peerTr, myTr: myTr,
		policies: append([]dataplane.SPParams(nil), dp.policies...),
		oldLocal: bytes.Clone(child.LocalAddr), oldRemote: bytes.Clone(child.RemoteAddr),
	}
}

func mbCheckMigration(t *testing.T, f *mbFixture, local, remote *net.UDPAddr) {
	t.Helper()
	if f.dp.count != 1 {
		t.Fatalf("migration count = %d, want 1", f.dp.count)
	}
	got := &f.dp.migrations[0]
	if !got.OldLocal.Equal(f.oldLocal) {
		t.Fatalf("old local = %s", got.OldLocal)
	}
	if !got.OldRemote.Equal(f.oldRemote) {
		t.Fatalf("old remote = %s", got.OldRemote)
	}
	if !got.NewLocal.Equal(local.IP) {
		t.Fatalf("new local = %s, want %s", got.NewLocal, local.IP)
	}
	if !got.NewRemote.Equal(remote.IP) {
		t.Fatalf("new remote = %s, want %s", got.NewRemote, remote.IP)
	}
	if got.LocalPort != uint16(local.Port) {
		t.Fatalf("migration local port = %d, want %d", got.LocalPort, local.Port)
	}
	if got.RemotePort != uint16(remote.Port) {
		t.Fatalf("migration remote port = %d, want %d", got.RemotePort, remote.Port)
	}
	if got.InboundSPI != f.child.InboundSPI {
		t.Fatal("migration changed the inbound SPI")
	}
	if got.OutboundSPI != f.child.OutboundSPI {
		t.Fatal("migration changed the outbound SPI")
	}
	if got.IfID != 7 {
		t.Fatalf("migration if_id = %d, want 7", got.IfID)
	}
	if got.ReqID != f.child.ReqID {
		t.Fatal("migration changed the Child reqid")
	}
	if !reflect.DeepEqual(got.Policies, f.policies) {
		t.Fatalf("migration lost original selectors or ownership: got %+v, want %+v", got.Policies, f.policies)
	}
	if len(f.dp.sas) != 2 {
		t.Fatalf("migration reinstalled SAs: installation count = %d, want initial pair only", len(f.dp.sas))
	}
	if len(f.dp.removed) != 0 {
		t.Fatalf("migration removed SAs: %v", f.dp.removed)
	}
}

func mbNotify(kind uint16, data []byte) wire.PayloadEntry {
	return wire.PayloadEntry{Payload: &wire.PayloadNotify{NotifyMsgType: kind, NotificationData: data}}
}

func mbInformational(t *testing.T, sender *SA, msgID uint32, response bool, inner []wire.PayloadEntry) []byte {
	t.Helper()
	flags := initiatorFlag(sender)
	if response {
		flags |= wire.FlagResponse
	}
	raw, err := buildEncryptedMessageEx(sender, inner, msgID, wire.ExchangeInformational, flags)
	if err != nil {
		t.Fatalf("encrypt INFORMATIONAL: %v", err)
	}
	return raw
}

func mbDecrypt(t *testing.T, receiver *SA, raw []byte) []wire.PayloadEntry {
	t.Helper()
	inner, err := decryptAndParse(receiver, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt received message: %v", err)
	}
	return inner
}

func mbResponse(t *testing.T, receiver *SA, raw []byte, msgID uint32) []wire.PayloadEntry {
	t.Helper()
	hdr := parseMsg(t, raw).Header
	if hdr.ExchangeType != wire.ExchangeInformational {
		t.Fatalf("response exchange = %d, want INFORMATIONAL", hdr.ExchangeType)
	}
	if hdr.MessageID != msgID {
		t.Fatalf("response Message ID = %d, want %d", hdr.MessageID, msgID)
	}
	if hdr.Flags&wire.FlagResponse == 0 {
		t.Fatal("received a request instead of the response")
	}
	return mbDecrypt(t, receiver, raw)
}

// mbProbeRequest accepts the NAT-detection payloads a liveness probe may carry,
// while refusing a replacement address advertisement or a teardown message.
func mbProbeRequest(t *testing.T, receiver *SA, raw []byte) {
	t.Helper()
	hdr := parseMsg(t, raw).Header
	if hdr.ExchangeType != wire.ExchangeInformational {
		t.Fatalf("probe exchange = %d, want INFORMATIONAL", hdr.ExchangeType)
	}
	if hdr.Flags&wire.FlagResponse != 0 {
		t.Fatal("probe is marked as a response")
	}
	for _, entry := range mbDecrypt(t, receiver, raw) {
		if _, ok := entry.Payload.(*wire.PayloadDelete); ok {
			t.Fatal("received teardown instead of a live-SA probe")
		}
		n, ok := entry.Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		switch n.NotifyMsgType {
		case wire.NotifyUpdateSAAddresses, wire.NotifyNoAdditionalAddresses:
			t.Fatal("received a fresh address advertisement instead of a live-SA probe")
		}
	}
}

func mbRequireNotify(t *testing.T, inner []wire.PayloadEntry, kind uint16) *wire.PayloadNotify {
	t.Helper()
	for i := range inner {
		n, ok := inner[i].Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		if n.NotifyMsgType == kind {
			return n
		}
	}
	t.Fatalf("message has no notify %d: %+v", kind, inner)
	return nil
}

func mbOnlyNotify(t *testing.T, inner []wire.PayloadEntry, kind uint16, data []byte) {
	t.Helper()
	if len(inner) != 1 {
		t.Fatalf("response carries %d payloads, want only notify %d", len(inner), kind)
	}
	n := mbRequireNotify(t, inner, kind)
	if n.ProtocolID != 0 {
		t.Fatalf("notify protocol = %d, want zero", n.ProtocolID)
	}
	if len(n.SPI) != 0 {
		t.Fatalf("notify SPI = %x, want empty", n.SPI)
	}
	if !bytes.Equal(n.NotificationData, data) {
		t.Fatalf("notify data = %x, want %x", n.NotificationData, data)
	}
}

func mbErrorEcho(t *testing.T, inner []wire.PayloadEntry, code uint16, cookie []byte) {
	t.Helper()
	if len(inner) != 2 {
		t.Fatalf("error response carries %d payloads, want error and COOKIE2", len(inner))
	}
	n := mbRequireNotify(t, inner, code)
	if len(n.NotificationData) != 0 {
		t.Fatalf("error notify data = %x, want empty", n.NotificationData)
	}
	echo := mbRequireNotify(t, inner, wire.NotifyCookie2)
	if !bytes.Equal(echo.NotificationData, cookie) {
		t.Fatalf("error response COOKIE2 = %x, want %x", echo.NotificationData, cookie)
	}
}

func mbCookieLength(t *testing.T, cookie []byte) {
	t.Helper()
	if len(cookie) < 8 {
		t.Fatalf("generated COOKIE2 has %d octets, minimum is 8", len(cookie))
	}
	if len(cookie) > 64 {
		t.Fatalf("generated COOKIE2 has %d octets, maximum is 64", len(cookie))
	}
}

func mbTransport(t *testing.T, bind string, natt bool) *transport.UDPTransport {
	t.Helper()
	var tr *transport.UDPTransport
	var err error
	if natt {
		tr, err = transport.NewNATTTransport(bind, slogutil.DiscardLogger())
	} else {
		tr, err = transport.NewUDPTransport(bind, slogutil.DiscardLogger())
	}
	if err != nil {
		t.Fatalf("open transport %s: %v", bind, err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tr.Run()
	}()
	t.Cleanup(func() {
		if err := tr.Close(); err != nil {
			t.Errorf("close test transport: %v", err)
		}
		<-done
	})
	return tr
}

// mbReceive preserves socket metadata for source-routing assertions and strips the
// non-ESP marker exactly where the engine's dispatcher does for NAT-T input.
func mbReceive(t *testing.T, tr *transport.UDPTransport) transport.Packet {
	t.Helper()
	select {
	case pkt := <-tr.Recv():
		if tr.IsNATT() {
			raw, ok := transport.StripNonESPMarker(pkt.Data)
			if !ok {
				t.Fatal("NAT-T socket received IKE without the non-ESP marker")
			}
			pkt.Data = raw
		}
		return pkt
	case <-time.After(rtxArrive):
		t.Fatal("expected UDP datagram did not arrive")
	}
	return transport.Packet{}
}

func mbEndpoint(t *testing.T, got, want *net.UDPAddr) {
	t.Helper()
	if got == nil {
		t.Fatalf("missing UDP endpoint, want %s", want)
	}
	if !got.IP.Equal(want.IP) {
		t.Fatalf("UDP address = %s, want %s", got.IP, want.IP)
	}
	if got.Port != want.Port {
		t.Fatalf("UDP port = %d, want %d", got.Port, want.Port)
	}
}

// mbNoNATs is the peer's wire fixture, with both address fields followed by the
// network-order source and destination ports (RFC 4555 Section 4.2.6).
func mbNoNATs(source, destination *net.UDPAddr) []byte {
	from, to := source.IP.To16(), destination.IP.To16()
	if source.IP.To4() != nil {
		from, to = source.IP.To4(), destination.IP.To4()
	}
	out := make([]byte, len(from)+len(to)+4)
	n := copy(out, from)
	n += copy(out[n:], to)
	binary.BigEndian.PutUint16(out[n:], uint16(source.Port))
	binary.BigEndian.PutUint16(out[n+2:], uint16(destination.Port))
	return out
}

// mbAuthHandshake captures both generated IKE_AUTH messages before altering the
// notification received by either side. AUTH covers the signed INIT messages and
// identities, so replacing this status notification preserves genuine authentication.
func mbAuthHandshake(t *testing.T, policy string, requestData, responseData []byte, omitRequest, omitResponse, translated bool) (
	*SA, *SA, *PeerSession, *transport.UDPTransport, *transport.UDPTransport,
) {
	t.Helper()
	log := slogutil.DiscardLogger()
	iniTr := mbTransport(t, "127.0.0.1:0", true)
	respTr := mbTransport(t, "127.0.0.1:0", true)
	oldPort := ikeTestPortFn
	ikeTestPortFn = func() string { return strconv.Itoa(nttPort(t, respTr) - 1) }
	t.Cleanup(func() { ikeTestPortFn = oldPort })
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "mobike-auth-psk")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	parsed, err := mbNATPolicyConfig(policy)
	if err != nil {
		t.Fatal(err)
	}
	iniPeer.ProhibitNAT = parsed.Peers["ze"].ProhibitNAT
	respPeer.ProhibitNAT = parsed.Peers["ze"].ProhibitNAT
	group, esp := testIKEGroup(), testESPGroup()
	ini, err := newInitiatorSA("ze", iniPeer, group, esp)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := newResponderSA("ze", respPeer, group, esp, ini.InitiatorSPI)
	if err != nil {
		t.Fatal(err)
	}
	table := NewSATable()
	table.Insert(ini)
	if translated {
		ini.PeerCfg.LocalAddress = "10.0.0.2"
	}
	initial := buildSAInitRequest(ini, group)
	ini.PeerCfg.LocalAddress = iniPeer.LocalAddress
	ini.InitiatorSAInitMsg = initial
	ini.State = StateSAInitSent
	handleSAInitRequest(resp, parseMsg(t, initial), initial, nil, nttPeerAddr(t, iniTr), log)
	if translated && !resp.NATDetected {
		t.Fatal("the translated source did not reach initial NAT detection")
	}
	ini.bindSockets(nil, iniTr)
	resp.bindSockets(nil, respTr)
	// The fixture provides mbDataplane when the established owner handles
	// mobility. Record that backend capability without a process-global load.
	ini.mobike.canMigrate = true
	resp.mobike.canMigrate = true
	handleInbound(ini, transport.Packet{Data: resp.LastSentMsg}, table, nil, log)
	request := mbReceive(t, respTr)
	mbEndpoint(t, request.RemoteAddr, nttPeerAddr(t, iniTr))
	if parseMsg(t, request.Data).Header.ExchangeType != wire.ExchangeIKEAuth {
		t.Fatal("NAT-T capture did not carry IKE_AUTH")
	}
	if ini.NATDetected {
		t.Fatal("fixture detected a NAT, so it cannot prove the MOBIKE-only port float")
	}
	mbAuthOfferEmpty(t, mbDecrypt(t, resp, request.Data))
	protected := notifyOf(mbDecrypt(t, resp, request.Data), wire.NotifyNoNATsAllowed)
	if policy == "prohibit" {
		if protected == nil || !bytes.Equal(protected.NotificationData,
			mbNoNATs(request.RemoteAddr, nttPeerAddr(t, respTr))) {
			t.Fatal("first IKE_AUTH did not protect its actual send tuple")
		}
	} else if protected != nil {
		t.Fatal("NAT-allowed IKE_AUTH prohibited translation")
	}
	request.Data = mbRewriteAuthOffer(t, ini, resp, request.Data, requestData, omitRequest)
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: group, espGroup: esp}
	ps.handleResponderInbound(resp, parseMsg(t, request.Data), request, respTr, log)
	answer := mbReceive(t, iniTr)
	mbEndpoint(t, answer.RemoteAddr, nttPeerAddr(t, respTr))
	if translated && policy == "prohibit" {
		mbRequireNotify(t, mbDecrypt(t, ini, answer.Data), wire.NotifyUnexpectedNATDetected)
		handleInbound(ini, answer, table, iniTr, log)
		if ini.State != StateDead || resp.State != StateDead || ps.getChildSA() != nil {
			t.Fatal("NAT prohibition let a translated handshake establish a Child")
		}
		return ini, resp, ps, iniTr, respTr
	}
	if !omitRequest {
		mbAuthOfferEmpty(t, mbDecrypt(t, ini, answer.Data))
		answer.Data = mbRewriteAuthOffer(t, resp, ini, answer.Data, responseData, omitResponse)
	}
	handleInbound(ini, answer, table, iniTr, log)
	if resp.State != StateEstablished {
		t.Fatalf("responder did not authenticate: %s", resp.State)
	}
	if ini.State != StateEstablished {
		t.Fatalf("initiator did not authenticate: %s", ini.State)
	}
	return ini, resp, ps, iniTr, respTr
}

func mbAuthOfferEmpty(t *testing.T, inner []wire.PayloadEntry) {
	t.Helper()
	n := mbRequireNotify(t, inner, wire.NotifyMobikeSupported)
	if len(n.NotificationData) != 0 {
		t.Fatalf("generated MOBIKE_SUPPORTED data = %x, want empty", n.NotificationData)
	}
	if n.ProtocolID != 0 {
		t.Fatalf("generated MOBIKE_SUPPORTED protocol = %d, want zero", n.ProtocolID)
	}
	if len(n.SPI) != 0 {
		t.Fatalf("generated MOBIKE_SUPPORTED SPI = %x, want empty", n.SPI)
	}
}

func mbRewriteAuthOffer(t *testing.T, sender, receiver *SA, raw, data []byte, omit bool) []byte {
	t.Helper()
	if data == nil {
		if !omit {
			return raw
		}
	}
	inner := mbDecrypt(t, receiver, raw)
	for i := range inner {
		n, ok := inner[i].Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		if n.NotifyMsgType != wire.NotifyMobikeSupported {
			continue
		}
		if omit {
			inner = append(inner[:i], inner[i+1:]...)
		} else {
			n.NotificationData = data
		}
		hdr := parseMsg(t, raw).Header
		rebuilt, err := buildEncryptedMessageEx(sender, inner, hdr.MessageID, hdr.ExchangeType, hdr.Flags)
		if err != nil {
			t.Fatalf("re-encrypt IKE_AUTH: %v", err)
		}
		return rebuilt
	}
	t.Fatal("IKE_AUTH fixture has no MOBIKE_SUPPORTED to replace")
	return nil
}

// A path check begun after an IKE rekey response must survive promotion. Its
// old-SA Message ID cannot be reused, and the new keys must protect the retry.
func TestMobikePendingCheckSurvivesIKEPromotion(t *testing.T) {
	ini, resp, _ := establishPSK(t)
	f := mbOwner(t, resp, ini, true)
	log := slogutil.DiscardLogger()
	request, pending, err := initiateIKERekey(f.peer, testIKEGroup())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pending.clear)
	f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: request, RemoteAddr: nttPeerAddr(t, f.peerTr),
		LocalAddr: nttPeerAddr(t, f.myTr), NATT: true,
	}, f.myTr, f.dp, log)
	rekeyAnswer := mbReceive(t, f.peerTr)
	newPeer, err := applyIKERekeyResponse(f.peer, pending, mbDecrypt(t, f.peer, rekeyAnswer.Data), log)
	if err != nil {
		t.Fatal(err)
	}
	nextTr := mbTransport(t, "127.0.0.1:0", true)
	setMobikePath(f.local, nttPeerAddr(t, f.myTr), nttPeerAddr(t, nextTr))
	f.local.NATDetected, f.local.PeerBehindNAT = true, true
	if err := f.ps.startMobikeRequest(f.local, f.myTr, false, log); err != nil {
		t.Fatal(err)
	}
	oldCheck := mbReceive(t, nextTr)
	oldCookie := mbRequireNotify(t, mbDecrypt(t, f.peer, oldCheck.Data), wire.NotifyCookie2)

	del := mbInformational(t, f.peer, f.local.ExpectedMsgID, false,
		[]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}})
	out := f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: del, RemoteAddr: nttPeerAddr(t, nextTr),
		LocalAddr: nttPeerAddr(t, f.myTr), NATT: true,
	}, f.myTr, f.dp, log)
	if out.newSA == nil {
		t.Fatal("old-SA Delete did not promote the negotiated IKE replacement")
	}
	mbResponse(t, f.peer, mbReceive(t, nextTr).Data, parseMsg(t, del).Header.MessageID)
	f.ps.serviceMobike(out.newSA, f.myTr, time.Now(), log)
	check := mbReceive(t, nextTr)
	inner := mbDecrypt(t, newPeer, check.Data)
	cookie := mbRequireNotify(t, inner, wire.NotifyCookie2)
	if bytes.Equal(cookie.NotificationData, oldCookie.NotificationData) {
		t.Fatal("promoted IKE SA reused the old check's cookie")
	}
	if notifyOf(inner, wire.NotifyUpdateSAAddresses) != nil {
		t.Fatal("promotion changed the original MOBIKE responder role")
	}
	answer := mbInformational(t, newPeer, parseMsg(t, check.Data).Header.MessageID, true,
		[]wire.PayloadEntry{mbNotify(wire.NotifyCookie2, cookie.NotificationData)})
	result := f.ps.handleOwnedInbound(out.newSA, transport.Packet{
		Data: answer, RemoteAddr: nttPeerAddr(t, nextTr),
		LocalAddr: nttPeerAddr(t, f.myTr), NATT: true,
	}, f.myTr, f.dp, log)
	if result.reestablish {
		t.Fatal("replacement IKE SA rejected its fresh return-routability response")
	}
	mbCheckMigration(t, f, nttPeerAddr(t, f.myTr), nttPeerAddr(t, nextTr))
	if !f.dp.migrations[0].NATDetected {
		t.Fatal("promotion lost the current path's ESP encapsulation requirement")
	}
}

// Error responses must correlate with the rejected exchange, including a
// CREATE_CHILD_SA that carries an invalid protected tuple and its retransmission.
func TestMobikeRekeyErrorKeepsExchangeType(t *testing.T) {
	ini, resp, _ := establishPSK(t)
	f := mbOwner(t, resp, ini, false)
	inner := []wire.PayloadEntry{mbNotify(wire.NotifyNoNATsAllowed,
		mbNoNATs(&net.UDPAddr{IP: net.IPv4(192, 0, 2, 9), Port: 4500}, nttPeerAddr(t, f.myTr)))}
	id := f.local.ExpectedMsgID
	request, err := buildEncryptedMessageEx(f.peer, inner, id, wire.ExchangeCreateChildSA, initiatorFlag(f.peer))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		f.ps.handleOwnedInbound(f.local, transport.Packet{
			Data: request, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: nttPeerAddr(t, f.myTr),
		}, f.myTr, f.dp, slogutil.DiscardLogger())
		answer := mbReceive(t, f.peerTr)
		header := parseMsg(t, answer.Data).Header
		if header.ExchangeType != wire.ExchangeCreateChildSA || header.MessageID != id || header.Flags&wire.FlagResponse == 0 {
			t.Fatalf("uncorrelated rekey error header: %+v", header)
		}
		mbOnlyNotify(t, mbDecrypt(t, f.peer, answer.Data), wire.NotifyUnexpectedNATDetected, nil)
	}
}
