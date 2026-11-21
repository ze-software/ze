//go:build linux

// Design: docs/architecture/l2tp/subscriber-session-model.md -- subscriber route lifetime
// Related: tunnel_fixture_pppoe_realm_linux.go -- existing PPPoE discovery and LCP peer

package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"slices"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	redistPPPoEPAP  = 0xc023
	redistPPPoEIPCP = 0x8021
)

type redistPPPoESubscriber struct {
	username string
	ip       netip.Addr
	framed   netip.Prefix
}

var redistPPPoESubscribers = [...]redistPPPoESubscriber{
	{"target", netip.MustParseAddr("10.99.23.10"), netip.MustParsePrefix("198.51.100.0/24")},
	{"control", netip.MustParseAddr("10.99.23.11"), netip.MustParsePrefix("203.0.113.0/24")},
}

func init() {
	Register("pppoe/redistribute-subscriber-route", redistPPPoEDriver, StopsDaemon)
	Register("pppoe/redistribute-subscriber-route/radius", redistPPPoERadius)
}

func redistPPPoEDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("subscriber route fixture needs the veth peer and BGP endpoint")
	}
	return Observe(ctx, "pppoe-subscriber-route-test", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return redistPPPoEScenario(ctx, plugin, args[0], args[1])
	})
}

func redistPPPoEScenario(ctx context.Context, plugin *sdk.Plugin, access, endpoint string) error {
	var dialer net.Dialer
	bgp, err := dialer.DialContext(ctx, "tcp4", endpoint)
	if err != nil {
		return fmt.Errorf("connect independent BGP receiver: %w", err)
	}
	defer bgp.Close() //nolint:errcheck // owns only the fixture's connection
	if err := redistPPPoEBGPOpen(bgp); err != nil {
		return fmt.Errorf("open BGP receiver: %w", err)
	}
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
		rows, err := peerRows03(ctx, plugin, "receiver")
		if err != nil {
			return false
		}
		for _, row := range rows {
			if row["fsm-state"] == "ESTABLISHED" {
				return true
			}
		}
		return false
	}) {
		return errors.New("BGP receiver did not establish before PPPoE")
	}
	discovery, err := tunnelPPPoEOpen(access)
	if err != nil {
		return err
	}
	defer discovery.close()
	sessionWire, err := tunnelPPPoEOpenEtherType(access, tunnelPPPoESessionEtherType)
	if err != nil {
		return err
	}
	defer sessionWire.close()
	live := make(map[string]bool, 4)
	var sessions [len(redistPPPoESubscribers)]*tunnelRealmPeer
	var previousCookie []byte
	for i, subscriber := range redistPPPoESubscribers {
		peer, cookie, err := redistPPPoEAdmit(ctx, discovery, sessionWire, byte(i+1), previousCookie)
		if err != nil {
			return fmt.Errorf("discover %s PPPoE subscriber: %w", subscriber.username, err)
		}
		previousCookie = cookie
		err = redistPPPoEIPCPExchange(ctx, peer, subscriber, byte(i+1))
		if err != nil {
			return fmt.Errorf("bring up %s PPPoE subscriber: %w", subscriber.username, err)
		}
		sessions[i] = peer
		if err := redistPPPoEBGPRoutes(bgp, live, subscriber, true); err != nil {
			return err
		}
	}
	if sessions[0].sid == sessions[1].sid {
		return errors.New("target and control were given the same live session ID")
	}
	if err := redistPPPoEWaitSessions(ctx, plugin, sessions[0].sid, sessions[1].sid); err != nil {
		return err
	}
	for i, subscriber := range redistPPPoESubscribers {
		peer := sessions[i]
		// RFC 2516 Section 5.5: the PADT carries the exact admitted session ID.
		packet := []byte{0x11, 0xa7, 0, 0, 0, 0}
		binary.BigEndian.PutUint16(packet[2:4], peer.sid)
		if err := discovery.send(peer.server, packet); err != nil {
			return fmt.Errorf("disconnect original %s session: %w", subscriber.username, err)
		}
		if err := redistPPPoEBGPRoutes(bgp, live, subscriber, false); err != nil {
			return err
		}
		if i == 0 {
			control := redistPPPoESubscribers[1]
			if len(live) != 2 || !live[netip.PrefixFrom(control.ip, 32).String()] || !live[control.framed.String()] {
				return fmt.Errorf("target disconnect changed the control's BGP routes: %v", live)
			}
			if err := redistPPPoEWaitSessions(ctx, plugin, sessions[1].sid); err != nil {
				return err
			}
		}
	}
	if err := redistPPPoEWaitSessions(ctx, plugin); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stderr, "OK: PPPoE IPCP addresses and RADIUS routes advertised and withdrawn on the same BGP connection, with the control surviving the first disconnect")
	return err
}

func redistPPPoEAdmit(ctx context.Context, discovery, wire *tunnelPPPoEWire, number byte, previous []byte) (*tunnelRealmPeer, []byte, error) {
	hostUniq := []byte{0x23, number}
	var server net.HardwareAddr
	var cookie []byte
	var lastErr error
	// Host-Uniq only distinguishes the sender's requests. Ze deduplicates
	// PADR by cookie for the same MAC, so require a new cookie on the wire.
	ready := Poll(ctx, 20, 100*time.Millisecond, func() bool {
		var tags map[uint16][]byte
		server, tags, lastErr = tunnelPPPoEDiscover(ctx, discovery, hostUniq)
		cookie = tags[tunnelPPPoEACCookie]
		return lastErr == nil && len(cookie) != 0 && !bytes.Equal(cookie, previous)
	})
	if !ready {
		if lastErr != nil {
			return nil, nil, fmt.Errorf("PPPoE discovery did not provide a distinct AC-Cookie: %w", lastErr)
		}
		return nil, nil, errors.New("PPPoE discovery did not provide a distinct AC-Cookie: the last offer's cookie was empty or repeated")
	}
	request := tunnelPPPoEPacket(tunnelPPPoEPADR, cookie, hostUniq, "")
	source, sid, _, err := discovery.exchange(ctx, server, request, tunnelPPPoEPADS, 3)
	if err != nil {
		return nil, nil, err
	}
	if sid == 0 || !bytes.Equal(server, source) {
		return nil, nil, fmt.Errorf("unexpected PADS session %d from %s, want AC %s", sid, source, server)
	}
	peer := &tunnelRealmPeer{wire: wire, server: server, sid: sid, options: []tunnelLCPOption{
		{kind: 1, value: []byte{0x05, 0xd4}},
		{kind: 5, value: []byte{0x5a, 0x45, 0x23, number}},
	}}
	return peer, cookie, nil
}

func redistPPPoEIPCPExchange(ctx context.Context, peer *tunnelRealmPeer, subscriber redistPPPoESubscriber, number byte) error {
	if err := peer.sendConfigureRequest(); err != nil {
		return err
	}
	deadline := time.Now().Add(30 * time.Second)
	var papSent, ipcpSent, localAck, remoteAck bool
	var ipcpID byte
	var requested netip.Addr
	sendIPCP := func(address netip.Addr) error {
		ipcpID++
		requested = address
		option := append([]byte{3, 6}, address.AsSlice()...)
		return peer.sendPPP(redistPPPoEIPCP, tunnelPPPControl(1, ipcpID, option))
	}
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		protocol, packet, err := peer.wire.readSession(peer.sid)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			return err
		}
		code, id, body, ok := tunnelPPPControlParse(packet)
		if !ok {
			continue
		}
		switch protocol {
		case tunnelPPPProtocolLCP:
			if err := peer.handleLCP(code, id, body); err != nil {
				return err
			}
			if peer.opened && !papSent {
				user := []byte(subscriber.username)
				credentials := append([]byte{byte(len(user))}, user...)
				const password = "fixture-password"
				credentials = append(credentials, byte(len(password)))
				credentials = append(credentials, password...)
				if err := peer.sendPPP(redistPPPoEPAP, tunnelPPPControl(1, number, credentials)); err != nil {
					return err
				}
				papSent = true
			}
		case redistPPPoEPAP:
			if !papSent || id != number || code != 2 {
				return fmt.Errorf("unexpected PAP verdict code %d id %d", code, id)
			}
			if !ipcpSent {
				if err := sendIPCP(netip.IPv4Unspecified()); err != nil {
					return err
				}
				ipcpSent = true
			}
		case redistPPPoEIPCP:
			switch code {
			case 1:
				if err := peer.sendPPP(redistPPPoEIPCP, tunnelPPPControl(2, id, body)); err != nil {
					return err
				}
				remoteAck = true
			case 2, 3:
				if !ipcpSent || id != ipcpID {
					continue
				}
				address, ok := netip.AddrFromSlice(tunnelIPCPAddress(body))
				if !ok || address.Unmap() != subscriber.ip {
					return fmt.Errorf("IPCP answered with address %v, want RADIUS assignment %s", address, subscriber.ip)
				}
				if code == 3 {
					if err := sendIPCP(subscriber.ip); err != nil {
						return err
					}
				} else {
					if requested != subscriber.ip {
						return errors.New("IPCP Ack replaced the requested address instead of acknowledging it")
					}
					localAck = true
				}
			}
		}
		if localAck && remoteAck {
			return nil
		}
	}
	return fmt.Errorf("PPP did not negotiate IPCP on sid %d (PAP sent %t, IPCP sent %t, acknowledgments %t/%t)", peer.sid, papSent, ipcpSent, localAck, remoteAck)
}

func redistPPPoEWaitSessions(ctx context.Context, plugin *sdk.Plugin, want ...uint16) error {
	var last []uint16
	var lastErr error
	expected := slices.Clone(want)
	slices.Sort(expected)
	if Poll(ctx, 25, 100*time.Millisecond, func() bool {
		var rows []struct {
			SID   uint16 `json:"sid"`
			State string `json:"state"`
		}
		var status string
		status, lastErr = Dispatch(ctx, plugin, "show pppoe sessions", &rows)
		last = last[:0]
		if lastErr != nil {
			return false
		}
		if status != statusDone {
			lastErr = fmt.Errorf("show pppoe sessions returned %s", status)
			return false
		}
		for _, row := range rows {
			if row.State != "session" {
				return false
			}
			last = append(last, row.SID)
		}
		slices.Sort(last)
		return slices.Equal(last, expected)
	}) {
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("live PPPoE sessions %v, want %v (last read: %w)", last, want, lastErr)
	}
	return fmt.Errorf("live PPPoE sessions %v, want %v", last, want)
}

func redistPPPoEBGPOpen(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
	open := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0xc0000202}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return err
	}
	if err := delayOpenFixtureRead(conn, byte(msgtype.TypeOPEN)); err != nil {
		return err
	}
	if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
		return err
	}
	return delayOpenFixtureRead(conn, byte(msgtype.TypeKEEPALIVE))
}

func redistPPPoEBGPRoutes(conn net.Conn, live map[string]bool, subscriber redistPPPoESubscriber, announce bool) error {
	pending := map[string]bool{netip.PrefixFrom(subscriber.ip, 32).String(): true, subscriber.framed.String(): true}
	if err := conn.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
		return err
	}
	for len(pending) != 0 {
		var header [message.HeaderLen]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return fmt.Errorf("BGP %s missing announce=%t for %v: %w", subscriber.username, announce, pending, err)
		}
		frame, err := message.ParseHeader(header[:])
		if err != nil {
			return err
		}
		if err := frame.ValidateLengthWithMax(false); err != nil {
			return err
		}
		var buffer [message.MaxMsgLen]byte
		body := buffer[:int(frame.Length)-message.HeaderLen]
		if _, err := io.ReadFull(conn, body); err != nil {
			return err
		}
		if frame.Type == msgtype.TypeKEEPALIVE {
			continue
		}
		if frame.Type != msgtype.TypeUPDATE {
			return fmt.Errorf("lost established BGP receiver to %s", frame.Type)
		}
		sections, err := wire.ParseUpdateSections(body)
		if err != nil {
			return err
		}
		if err := redistPPPoEApplyNLRI(sections.Withdrawn(body), live, pending, false, announce); err != nil {
			return err
		}
		advertised := sections.NLRI(body)
		if len(advertised) != 0 {
			_, _, nextHop, ok := attribute.AttrFind(sections.Attrs(body), attribute.AttrNextHop)
			if !ok || !bytes.Equal(nextHop, []byte{127, 0, 0, 4}) {
				return fmt.Errorf("redistribution NEXT_HOP %v, want Ze's BGP address 127.0.0.4", nextHop)
			}
		}
		if err := redistPPPoEApplyNLRI(advertised, live, pending, true, announce); err != nil {
			return err
		}
	}
	return nil
}

func redistPPPoEApplyNLRI(data []byte, live, pending map[string]bool, announce, wantAnnouncement bool) error {
	for len(data) != 0 {
		route, rest, err := nlri.ParseINET(family.AFIIPv4, family.SAFIUnicast, data, false)
		if err != nil {
			return err
		}
		prefix := route.String()
		if announce != wantAnnouncement || !pending[prefix] || live[prefix] == announce {
			return fmt.Errorf("unexpected BGP route %s announce=%t, want changes %v announce=%t", prefix, announce, pending, wantAnnouncement)
		}
		delete(pending, prefix)
		if announce {
			live[prefix] = true
		} else {
			delete(live, prefix)
		}
		data = rest
	}
	return nil
}

func redistPPPoERadius(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("subscriber route RADIUS needs the port and shared key")
	}
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // owned fixture socket
	var buffer [4096]byte
	for ctx.Err() == nil {
		if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			return err
		}
		n, remote, err := conn.ReadFromUDP(buffer[:])
		if err != nil {
			if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
				continue
			}
			return err
		}
		if n < 20 || n != int(binary.BigEndian.Uint16(buffer[2:4])) {
			return errors.New("incomplete RADIUS request")
		}
		request := buffer[:n]
		var attributes []byte
		code := byte(5)
		switch request[0] {
		case 1:
			username, _ := tunnelRadiusValue(tunnelRadiusDescribe(request), "User-Name")
			i := slices.IndexFunc(redistPPPoESubscribers[:], func(subscriber redistPPPoESubscriber) bool {
				return subscriber.username == username
			})
			if i < 0 {
				return fmt.Errorf("unexpected PPPoE RADIUS user %q", username)
			}
			subscriber := redistPPPoESubscribers[i]
			code = 2
			attributes = tunnelRadiusAttr(8, subscriber.ip.AsSlice())
			attributes = append(attributes, tunnelRadiusUint32Attr(6, 2)...)
			attributes = append(attributes, tunnelRadiusUint32Attr(7, 1)...)
			attributes = append(attributes, tunnelRadiusAttr(22, []byte(subscriber.framed.String()+" 0.0.0.0 7"))...)
		case 4:
		default:
			return fmt.Errorf("unexpected RADIUS code %d", request[0])
		}
		if _, err := conn.WriteToUDP(tunnelRadiusAnswer(code, request, attributes, []byte(args[1])), remote); err != nil {
			return err
		}
	}
	return nil
}
