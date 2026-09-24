// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- wildcard replies and parallel Child ownership.
package engine

import (
	"bytes"
	"fmt"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Initial success, cached success, stateless COOKIE and INVALID_KE all leave
// the wildcard NAT-T socket from the address at which the request arrived.
func TestMobikeInitialResponsesUseReceivedDestination(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux wildcard source selection")
	}
	for _, mode := range []string{"success", "cookie", "invalid-ke"} {
		t.Run(mode, func(t *testing.T) {
			log := slogutil.DiscardLogger()
			resetCookieSecret(t)
			threshold := uint32(1000)
			if mode == "cookie" {
				threshold = 0
			}
			withCookieThreshold(t, threshold)
			peerTr := mbTransport(t, "127.0.0.1:0", true)
			serverTr := mbTransport(t, "0.0.0.0:0", true)
			local := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: nttPort(t, serverTr)}
			iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "source-proof")
			iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.2"
			respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.2", "127.0.0.1"
			respPeer.ConnectionType = ipsec.ConnectionRespond
			group, esp := testIKEGroup(), testESPGroup()
			ini, err := newInitiatorSA("ze", iniPeer, group, esp)
			if err != nil {
				t.Fatal(err)
			}
			request := parseMsg(t, buildSAInitRequest(ini, group))
			if mode == "invalid-ke" {
				for _, entry := range request.Payloads {
					if ke, ok := entry.Payload.(*wire.PayloadKE); ok {
						ke.DHGroup ^= 1
					}
				}
			}
			buf := make([]byte, 4096)
			n, err := request.CheckedWriteTo(buf, 0)
			if err != nil {
				t.Fatal(err)
			}
			ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: group, espGroup: esp, natt: serverTr}
			setActivePeers(map[string]*PeerSession{"ze": ps})
			t.Cleanup(func() { setActivePeers(nil) })
			table := NewSATable()
			var first []byte
			for attempt := range 2 {
				if err := peerTr.Send(transport.AddNonESPMarker(buf[:n]), local); err != nil {
					t.Fatal(err)
				}
				pkt := mbReceive(t, serverTr)
				if attempt == 1 && mode == "success" {
					routeInbound(ps.getSA(), pkt, table, serverTr, log)
				} else if !tryResponderSAInit(pkt, ini.InitiatorSPI, [8]byte{}, table, serverTr, log) {
					t.Fatal("initial request was not consumed")
				}
				answer := mbReceive(t, peerTr)
				mbEndpoint(t, answer.RemoteAddr, local)
				message := parseMsg(t, answer.Data)
				if message.Header.ExchangeType != wire.ExchangeIKESAInit || message.Header.Flags&wire.FlagResponse == 0 {
					t.Fatal("received no correlated IKE_SA_INIT response")
				}
				switch mode {
				case "cookie":
					mbRequireNotify(t, message.Payloads, wire.NotifyCookie)
				case "invalid-ke":
					mbRequireNotify(t, message.Payloads, wire.NotifyInvalidKEPayload)
					return // INVALID_KE ends this half-open attempt; retry uses a fresh SA.
				case "success":
					if attempt == 1 && !bytes.Equal(first, answer.Data) {
						t.Fatal("cached initial response changed")
					}
					first = bytes.Clone(answer.Data)
				}
			}
		})
	}
}

// Both publication orders must leave the promoted Child's policy templates
// resolving to its own states, not to the tuple selected by the retiring IKE SA.
func TestMobikeParallelAuthKeepsPromotedPolicyEndpoints(t *testing.T) {
	for _, duringInstall := range []bool{false, true} {
		t.Run(fmt.Sprintf("during-install=%t", duringInstall), func(t *testing.T) {
			dp := mbUseHandoffDataplane(t)
			ini, resp, ps, iniTr, respTr := mbAuthHandshake(t, "allow", nil, nil, false, false, false)
			log := slogutil.DiscardLogger()
			ps.ownedSA.Store(resp)
			ps.setSA(resp)
			moved := mbTransport(t, "127.0.0.2:0", true)
			local, remote := nttPeerAddr(t, respTr), nttPeerAddr(t, moved)
			update := mbInformational(t, ini, resp.ExpectedMsgID, false,
				[]wire.PayloadEntry{mbNotify(wire.NotifyUpdateSAAddresses, nil)})
			ps.handleOwnedInbound(resp, transport.Packet{Data: update, LocalAddr: local, RemoteAddr: remote, NATT: true}, respTr, dp, log)
			mbResponse(t, ini, mbReceive(t, moved).Data, parseMsg(t, update).Header.MessageID)
			resp.mobike.lastKeepalive = time.Now()
			ps.serviceMobike(resp, respTr, dp, time.Now(), log)
			check := mbReceive(t, moved)
			cookie := mbRequireNotify(t, mbDecrypt(t, ini, check.Data), wire.NotifyCookie2).NotificationData
			answer := mbInformational(t, ini, parseMsg(t, check.Data).Header.MessageID, true,
				[]wire.PayloadEntry{mbNotify(wire.NotifyCookie2, cookie)})
			cookiePacket := transport.Packet{Data: answer, LocalAddr: local, RemoteAddr: remote, NATT: true}

			next, auth := mbParallelAuth(t, ini, ps, iniTr, respTr)
			table := NewSATable()
			table.Insert(resp)
			table.Insert(next)
			ps.setPendingSA(next)
			authMessage := parseMsg(t, auth.Data)
			if duringInstall {
				dp.installed = make(chan struct{})
				dp.release = make(chan struct{})
				authDone := make(chan struct{})
				t.Cleanup(func() {
					dp.releaseInstall()
					select {
					case <-authDone:
					case <-time.After(time.Second):
						t.Error("parallel IKE_AUTH did not stop during cleanup")
					}
				})
				go func() {
					defer close(authDone)
					ps.handleResponderInbound(next, authMessage, auth, respTr, log)
				}()
				select {
				case <-dp.installed:
				case <-time.After(time.Second):
					t.Fatal("parallel IKE_AUTH did not reach policy installation")
				}
				ownerDone := make(chan struct{})
				ownerStarted := make(chan struct{})
				t.Cleanup(func() {
					dp.releaseInstall()
					select {
					case <-ownerDone:
					case <-time.After(time.Second):
						t.Error("retiring owner did not stop during cleanup")
					}
				})
				go func() {
					defer close(ownerDone)
					close(ownerStarted)
					ps.handleOwnedInbound(resp, cookiePacket, respTr, dp, log)
				}()
				<-ownerStarted
				// Keep installation paused while the owner has an opportunity to
				// process the authenticated answer. Serialization leaves it blocked;
				// the old implementation completes and corrupts the pending policy.
				select {
				case <-ownerDone:
				case <-time.After(100 * time.Millisecond):
				}
				dp.releaseInstall()
				for _, done := range []chan struct{}{authDone, ownerDone} {
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Fatal("Child lifecycle operation did not finish")
					}
				}
			} else {
				ps.handleResponderInbound(next, authMessage, auth, respTr, log)
				ps.handleOwnedInbound(resp, cookiePacket, respTr, dp, log)
			}
			mbReceive(t, iniTr) // The new IKE_AUTH response, not an injected publication.
			if next.State != StateEstablished || ps.getPendingChild() == nil {
				t.Fatal("parallel authenticated handshake did not publish a Child")
			}
			ps.cleanupChild(dp, nil, log)
			ps.ownedSA.Store(nil)
			if ps.resolvePendingAfterOwnerLoop(table, dp, nil, log) != pendingContinue {
				t.Fatal("authenticated parallel SA was not promoted")
			}
			child := ps.getChildSA()
			if child == nil || ps.getSA() != next {
				t.Fatal("handoff lost the authenticated Child")
			}
			dp.assertPolicyResolves(t, child)
		})
	}
}

// mbParallelAuth prepares a second genuine PSK exchange up to its final request,
// leaving its publication to the same responder entry point used by dispatch.
func mbParallelAuth(t *testing.T, oldInitiator *SA, ps *PeerSession, iniTr, respTr *transport.UDPTransport) (*SA, transport.Packet) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, err := newInitiatorSA(ps.peerName, oldInitiator.PeerCfg, ps.ikeGroup, ps.espGroup)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := newResponderSA(ps.peerName, ps.peerCfg, ps.ikeGroup, ps.espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatal(err)
	}
	request := buildSAInitRequest(ini, ps.ikeGroup)
	ini.InitiatorSAInitMsg = request
	ini.State = StateSAInitSent
	handleSAInitRequest(resp, parseMsg(t, request), request, nil, nttPeerAddr(t, iniTr), log)
	table := NewSATable()
	table.Insert(ini)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	resp.bindSockets(nil, respTr)
	return resp, transport.Packet{Data: ini.LastSentMsg, LocalAddr: nttPeerAddr(t, respTr), RemoteAddr: nttPeerAddr(t, iniTr), NATT: true}
}

// The kernel keys policies by selector, not outer endpoints. This model retains
// both policy templates and state endpoints so a surviving but unusable policy
// fails just as an absent policy does.
type mbHandoffDP struct {
	*spdDP
	mu        sync.Mutex
	endpoints map[uint32]dataplane.SAParams
	installed chan struct{}
	release   chan struct{}
	released  sync.Once
}

var mbHandoffBackend struct {
	sync.Once
	err    error
	active *mbHandoffDP
}

func mbUseHandoffDataplane(t *testing.T) *mbHandoffDP {
	t.Helper()
	const name = "ike-engine-test-mobike-handoff"
	mbHandoffBackend.Do(func() {
		mbHandoffBackend.err = dataplane.Register(name, func() (dataplane.Dataplane, error) { return mbHandoffBackend.active, nil })
	})
	if mbHandoffBackend.err != nil {
		t.Fatal(mbHandoffBackend.err)
	}
	dp := &mbHandoffDP{spdDP: newSPDDP(), endpoints: make(map[uint32]dataplane.SAParams)}
	mbHandoffBackend.active = dp
	if err := dataplane.Load(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Error(err)
		}
		mbHandoffBackend.active = nil
	})
	return dp
}

func (d *mbHandoffDP) InstallSA(p dataplane.SAParams) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.endpoints[p.SPI] = p
	return d.spdDP.InstallSA(p)
}

func (d *mbHandoffDP) RemoveSA(spi uint32, dst net.IP, proto uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.endpoints, spi)
	return d.spdDP.RemoveSA(spi, dst, proto)
}

func (d *mbHandoffDP) InstallPolicy(p dataplane.SPParams) error {
	d.mu.Lock()
	err := d.spdDP.InstallPolicy(p)
	pause := d.installed != nil && p.Dir == dataplane.SADirOut
	d.mu.Unlock()
	if pause {
		close(d.installed)
		<-d.release
	}
	return err
}

func (d *mbHandoffDP) MigrateTunnel(m dataplane.TunnelMigration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, spi := range []uint32{m.InboundSPI, m.OutboundSPI} {
		state, ok := d.endpoints[spi]
		if !ok {
			return fmt.Errorf("state %#x does not exist", spi)
		}
		state.Src, state.Dst = m.NewLocal, m.NewRemote
		if state.Dir == dataplane.SADirIn {
			state.Src, state.Dst = m.NewRemote, m.NewLocal
		}
		d.endpoints[spi] = state
	}
	for _, policy := range m.Policies {
		key := spdKeyOf(policy)
		current, ok := d.policies[key]
		if !ok {
			return fmt.Errorf("policy %v does not exist", key)
		}
		current.TunnelSrc, current.TunnelDst = m.NewLocal, m.NewRemote
		if current.Dir == dataplane.SADirIn {
			current.TunnelSrc, current.TunnelDst = m.NewRemote, m.NewLocal
		}
		d.policies[key] = current
	}
	return nil
}

func (d *mbHandoffDP) releaseInstall() {
	d.released.Do(func() { close(d.release) })
}

func (d *mbHandoffDP) assertPolicyResolves(t *testing.T, child *ChildSA) {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, dir := range spdPolicyDirs {
		policy, ok := d.policies[spdKeyOf(childPolicyParams(child, dir))]
		if !ok {
			t.Fatalf("promoted Child has no %s policy", dirName(dir))
		}
		spi := child.OutboundSPI
		if dir == dataplane.SADirIn {
			spi = child.InboundSPI
		}
		state, ok := d.endpoints[spi]
		if !ok || !policy.TunnelSrc.Equal(state.Src) || !policy.TunnelDst.Equal(state.Dst) {
			t.Fatalf("promoted %s policy %s -> %s does not resolve to state %#x (%s -> %s)", dirName(dir),
				policy.TunnelSrc, policy.TunnelDst, spi, state.Src, state.Dst)
		}
	}
}

// A valid late CREATE_CHILD_SA must not steal templates already transferred by
// a parallel IKE_AUTH. Requests receive a closing-collision refusal; a response
// to our own outstanding rekey closes the old IKE SA and its peer-side children.
func TestMobikeParallelAuthRetiresChildRekey(t *testing.T) {
	for _, response := range []bool{false, true} {
		t.Run(fmt.Sprintf("response=%t", response), func(t *testing.T) {
			dp := mbUseHandoffDataplane(t)
			ini, resp, ps, iniTr, respTr := mbAuthHandshake(t, "allow", nil, nil, false, false, false)
			log := slogutil.DiscardLogger()
			ps.ownedSA.Store(resp)
			ps.setSA(resp)
			moved := mbTransport(t, "127.0.0.2:0", true)
			local, remote := nttPeerAddr(t, respTr), nttPeerAddr(t, moved)
			resp.mobike.local, resp.peerEndpoint = local, remote
			if err := ps.migrateMobikeChild(resp, dp); err != nil {
				t.Fatal(err)
			}
			ini.mobike.local, ini.peerEndpoint = remote, local
			peerChild, err := createFirstChildSA(ini, ps.espGroup, remote.IP.String(), local.IP.String(), 0, &mockDP{}, log)
			if err != nil {
				t.Fatal(err)
			}
			var packet []byte
			if response {
				ps.startChildRekey(resp, respTr, log)
				request := mbReceive(t, moved)
				packet, _, err = respondChildRekey(ini, mbDecrypt(t, ini, request.Data), peerChild,
					parseMsg(t, request.Data).Header.MessageID, &mockDP{}, log)
			} else {
				var pending *pendingRekey
				packet, pending, err = initiateChildRekey(ini, peerChild)
				if pending != nil {
					defer pending.clear()
				}
			}
			if err != nil {
				t.Fatalf("prepare a valid rekey: %v", err)
			}

			next, auth := mbParallelAuth(t, ini, ps, iniTr, respTr)
			table := NewSATable()
			table.Insert(resp)
			table.Insert(next)
			ps.setPendingSA(next)
			ps.handleResponderInbound(next, parseMsg(t, auth.Data), auth, respTr, log)
			mbReceive(t, iniTr)
			pendingChild := ps.getPendingChild()
			if next.State != StateEstablished || pendingChild == nil {
				t.Fatal("parallel IKE_AUTH did not publish its Child")
			}
			if pendingChild.RemoteAddr.Equal(ps.getChildSA().RemoteAddr) {
				t.Fatal("fixture did not separate pending and retiring Child endpoints")
			}
			dp.assertPolicyResolves(t, pendingChild)

			pkt := transport.Packet{Data: packet, LocalAddr: local, RemoteAddr: remote, NATT: true}
			ps.handleOwnedInbound(resp, pkt, respTr, dp, log)
			reply := mbReceive(t, moved)
			inner := mbDecrypt(t, ini, reply.Data)
			header := parseMsg(t, reply.Data).Header
			if response {
				if header.ExchangeType != wire.ExchangeInformational || header.Flags&wire.FlagResponse != 0 {
					t.Fatal("late Child response did not close the retiring IKE SA")
				}
				if len(inner) != 1 {
					t.Fatalf("retirement carries %d payloads, want one IKE Delete", len(inner))
				}
				del, ok := inner[0].Payload.(*wire.PayloadDelete)
				if !ok || del.ProtocolID != wire.ProtocolIKE {
					t.Fatal("retirement did not delete the IKE SA and its peer-side children")
				}
			} else {
				if header.ExchangeType != wire.ExchangeCreateChildSA || header.Flags&wire.FlagResponse == 0 ||
					header.MessageID != parseMsg(t, packet).Header.MessageID {
					t.Fatal("closing-collision refusal did not answer this rekey request")
				}
				mbOnlyNotify(t, inner, wire.NotifyTemporaryFailure, nil)
				ps.handleOwnedInbound(resp, pkt, respTr, dp, log)
				if replay := mbReceive(t, moved); !bytes.Equal(replay.Data, reply.Data) {
					t.Fatal("retransmission changed the closing-collision refusal")
				}
			}
			dp.assertPolicyResolves(t, pendingChild)
			ps.cleanupChild(dp, nil, log)
			ps.ownedSA.Store(nil)
			if ps.resolvePendingAfterOwnerLoop(table, dp, nil, log) != pendingContinue {
				t.Fatal("parallel Child was not promoted")
			}
			dp.assertPolicyResolves(t, ps.getChildSA())
			if len(dp.states) != 2 || !dp.states[pendingChild.InboundSPI] || !dp.states[pendingChild.OutboundSPI] {
				t.Fatalf("retirement leaked or replaced Child states: %v", dp.states)
			}
		})
	}
}
