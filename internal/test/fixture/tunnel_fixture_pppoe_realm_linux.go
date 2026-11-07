//go:build linux

// Design: docs/guide/l2tp.md -- relay by realm: a username whose realm names
// no binding is authenticated by the local auth handler.
// Related: tunnel_fixture_pppoe_linux.go -- the discovery client and wire this reuses.
// Related: tunnel_fixture_l2tp_ppp.go -- tunnelPPPControl, the PPP control packet builder.

package fixture

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // RFC 1994 CHAP Algorithm 5 is MD5 on the wire
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// tunnelPPPoESessionEtherType is the session stage ETHER_TYPE (RFC 2516 Section 5).
	tunnelPPPoESessionEtherType = 0x8864
	tunnelPPPProtocolLCP        = 0xc021
	tunnelPPPProtocolCHAP       = 0xc223

	// tunnelRealmNoMatchName carries a realm no relay-realm binding names, so
	// the AC hands the Response to its local auth handler.
	tunnelRealmNoMatchName   = "cpe@other.example"
	tunnelRealmNoMatchSecret = "not-a-local-user"
	tunnelRealmNoMatchMagic  = 0x5a45524d

	// tunnelRealmNoMatchBudget bounds the whole LCP and CHAP exchange.
	tunnelRealmNoMatchBudget = 30 * time.Second
)

// tunnelLCPOption is one LCP configuration option (RFC 1661 Section 6).
type tunnelLCPOption struct {
	kind  byte
	value []byte
}

// tunnelRealmPeer is the subscriber half of one PPPoE session: the LCP options
// it requests and how far LCP has gone. Not safe for concurrent use.
type tunnelRealmPeer struct {
	wire      *tunnelPPPoEWire
	server    net.HardwareAddr
	sid       uint16
	options   []tunnelLCPOption
	requestID byte
	ourAcked  bool
	theirAck  bool
	opened    bool
}

// tunnelPPPoERealmNoMatch takes a PPPoE session past PADS, opens LCP and
// answers the AC's CHAP Challenge as a user whose realm no relay-realm binding
// names. It succeeds only on a CHAP Failure: with no local users the local
// auth handler refuses, and a CHAP Success means the unmatched realm was let in.
func tunnelPPPoERealmNoMatch(ctx context.Context, _ []string) error {
	interfaceName := os.Getenv("TEST_IFACE")
	if interfaceName == "" {
		interfaceName = tunnelPPPoEDefaultInterface
	}
	discovery, err := tunnelPPPoEOpen(interfaceName)
	if err != nil {
		return err
	}
	defer discovery.close()

	// The session socket opens before PADR so the AC's first LCP
	// Configure-Request, sent once PADS is out, is not lost.
	session, err := tunnelPPPoEOpenEtherType(interfaceName, tunnelPPPoESessionEtherType)
	if err != nil {
		return err
	}
	defer session.close()

	server, sid, err := tunnelPPPoERequestSession(ctx, discovery, []byte{0x52, 0x4d})
	if err != nil {
		return err
	}
	fmt.Println("OK: PADS received, session-id=" + strconv.Itoa(int(sid)))
	peer := &tunnelRealmPeer{
		wire:   session,
		server: server,
		sid:    sid,
		options: []tunnelLCPOption{
			{kind: 1, value: []byte{0x05, 0xd4}}, // MRU 1492, RFC 2516 Section 7
			{kind: 5, value: binary.BigEndian.AppendUint32(nil, tunnelRealmNoMatchMagic)},
		},
	}
	return peer.run(ctx)
}

// tunnelPPPoERequestSession runs PADI/PADO then PADR/PADS and answers the AC's
// MAC address and the non-zero session id it assigned.
func tunnelPPPoERequestSession(ctx context.Context, wire *tunnelPPPoEWire, hostUniq []byte) (net.HardwareAddr, uint16, error) {
	for range 6 {
		server, tags, err := tunnelPPPoEDiscover(ctx, wire, hostUniq)
		if err != nil {
			return nil, 0, errors.New("no PADO received")
		}
		request := tunnelPPPoEPacket(tunnelPPPoEPADR, tags[tunnelPPPoEACCookie], hostUniq, "")
		_, sid, _, err := wire.exchange(ctx, server, request, tunnelPPPoEPADS, 2)
		if err != nil {
			continue
		}
		if sid == 0 {
			return nil, 0, errors.New("PADS session ID is 0")
		}
		return server, sid, nil
	}
	return nil, 0, errors.New("no PADS received")
}

// run drives LCP to Opened and CHAP to its verdict. A read timeout
// retransmits the Configure-Request until the AC acknowledges it.
func (peer *tunnelRealmPeer) run(ctx context.Context) error {
	if err := peer.sendConfigureRequest(); err != nil {
		return err
	}
	deadline := time.Now().Add(tunnelRealmNoMatchBudget)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		protocol, packet, err := peer.wire.readSession(peer.sid)
		if err != nil {
			if !errors.Is(err, unix.EAGAIN) && !errors.Is(err, unix.EWOULDBLOCK) {
				return err
			}
			if !peer.ourAcked {
				if err := peer.sendConfigureRequest(); err != nil {
					return err
				}
			}
			continue
		}
		code, identifier, body, ok := tunnelPPPControlParse(packet)
		if !ok {
			continue
		}
		if protocol == tunnelPPPProtocolLCP {
			if err := peer.handleLCP(code, identifier, body); err != nil {
				return err
			}
			continue
		}
		if protocol != tunnelPPPProtocolCHAP {
			continue
		}
		done, err := peer.handleCHAP(code, identifier, body)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	return errors.New("no CHAP Failure within " + tunnelRealmNoMatchBudget.String())
}

// handleLCP answers one LCP packet from the AC (RFC 1661 Section 5).
func (peer *tunnelRealmPeer) handleLCP(code, identifier byte, body []byte) error {
	switch code {
	case 1: // Configure-Request: accept every option the AC asks for.
		peer.theirAck = true
		if err := peer.sendPPP(tunnelPPPProtocolLCP, tunnelPPPControl(2, identifier, body)); err != nil {
			return err
		}
	case 2: // Configure-Ack of our current request.
		if identifier == peer.requestID {
			peer.ourAcked = true
		}
	case 3: // Configure-Nak: adopt each suggested value (RFC 1661 Section 5.3).
		peer.options = tunnelLCPApplyNak(peer.options, body)
		return peer.sendConfigureRequest()
	case 4: // Configure-Reject: drop each rejected option (RFC 1661 Section 5.4).
		peer.options = tunnelLCPApplyReject(peer.options, body)
		return peer.sendConfigureRequest()
	case 5: // Terminate-Request before any CHAP verdict.
		if err := peer.sendPPP(tunnelPPPProtocolLCP, tunnelPPPControl(6, identifier, nil)); err != nil {
			return err
		}
		return errors.New("the AC terminated LCP before sending a CHAP verdict")
	case 9: // Echo-Request: Echo-Reply carries our Magic-Number (RFC 1661 Section 5.8).
		reply := binary.BigEndian.AppendUint32(nil, peer.magic())
		if len(body) > 4 {
			reply = append(reply, body[4:]...)
		}
		return peer.sendPPP(tunnelPPPProtocolLCP, tunnelPPPControl(10, identifier, reply))
	}
	if peer.ourAcked && peer.theirAck && !peer.opened {
		peer.opened = true
		fmt.Println("OK: LCP opened")
	}
	return nil
}

// handleCHAP answers the AC's Challenge and reads its verdict (RFC 1994
// Section 4). It answers done on a Failure, the verdict this fixture expects.
func (peer *tunnelRealmPeer) handleCHAP(code, identifier byte, body []byte) (bool, error) {
	switch code {
	case 1:
		if len(body) < 1 {
			return false, errors.New("CHAP Challenge carried no Value-Size")
		}
		size := int(body[0])
		if 1+size > len(body) {
			return false, errors.New("CHAP Challenge Value-Size overruns the packet")
		}
		// RFC 1994 Section 4.1: the Response Value is "the one-way hash
		// calculated over a stream of octets consisting of the Identifier,
		// followed by (concatenated with) the "secret", followed by
		// (concatenated with) the Challenge Value".
		digest := md5.Sum( //nolint:gosec // RFC 1994 CHAP Algorithm 5 is MD5
			append(append([]byte{identifier}, tunnelRealmNoMatchSecret...), body[1:1+size]...))
		response := append([]byte{byte(len(digest))}, digest[:]...)
		response = append(response, tunnelRealmNoMatchName...)
		if err := peer.sendPPP(tunnelPPPProtocolCHAP, tunnelPPPControl(2, identifier, response)); err != nil {
			return false, err
		}
		fmt.Println("OK: CHAP Response sent as " + tunnelRealmNoMatchName)
		return false, nil
	case 3:
		return false, errors.New("the AC answered CHAP Success for " + tunnelRealmNoMatchName + ", whose realm no binding names and no local user holds")
	case 4:
		fmt.Println("OK: CHAP Failure received: " + string(body))
		return true, nil
	}
	// An authenticator sends only Challenge, Success and Failure (RFC 1994
	// Section 4), so any other code is a defect on the AC side.
	return false, fmt.Errorf("the AC sent CHAP code %d, which only a peer sends", code)
}

// magic answers the Magic-Number we negotiate, or zero once the AC rejected
// it, which RFC 1661 Section 6.4 makes the value an Echo-Reply then carries.
func (peer *tunnelRealmPeer) magic() uint32 {
	for _, option := range peer.options {
		if option.kind == 5 && len(option.value) == 4 {
			return binary.BigEndian.Uint32(option.value)
		}
	}
	return 0
}

func (peer *tunnelRealmPeer) sendConfigureRequest() error {
	peer.requestID++
	peer.ourAcked = false
	var body []byte
	for _, option := range peer.options {
		body = append(body, option.kind, byte(2+len(option.value)))
		body = append(body, option.value...)
	}
	return peer.sendPPP(tunnelPPPProtocolLCP, tunnelPPPControl(1, peer.requestID, body))
}

// sendPPP sends one PPP packet on the session (RFC 2516 Section 5: CODE 0x00,
// the SESSION_ID, LENGTH of the PPP payload, then the PPP protocol field).
func (peer *tunnelRealmPeer) sendPPP(protocol uint16, packet []byte) error {
	payload := make([]byte, 8+len(packet))
	payload[0] = 0x11
	binary.BigEndian.PutUint16(payload[2:4], peer.sid)
	binary.BigEndian.PutUint16(payload[4:6], uint16(2+len(packet)))
	binary.BigEndian.PutUint16(payload[6:8], protocol)
	copy(payload[8:], packet)
	return peer.wire.send(peer.server, payload)
}

// readSession answers the protocol and payload of the next PPP packet the AC
// sent on session sid, skipping our own frames and other sessions.
func (wire *tunnelPPPoEWire) readSession(sid uint16) (uint16, []byte, error) {
	buffer := make([]byte, 1518)
	for {
		n, _, err := unix.Recvfrom(wire.fd, buffer, 0)
		if err != nil {
			return 0, nil, err
		}
		frame := buffer[:n]
		if len(frame) < 22 {
			continue
		}
		if bytes.Equal(frame[6:12], wire.mac) {
			continue
		}
		payload := frame[14:]
		if payload[1] != 0x00 {
			continue
		}
		if binary.BigEndian.Uint16(payload[2:4]) != sid {
			continue
		}
		length := int(binary.BigEndian.Uint16(payload[4:6]))
		if length < 2 {
			continue
		}
		if 6+length > len(payload) {
			continue
		}
		ppp := payload[6 : 6+length]
		return binary.BigEndian.Uint16(ppp[:2]), append([]byte(nil), ppp[2:]...), nil
	}
}

// tunnelPPPControlParse splits an LCP or CHAP packet into code, identifier and
// the data its Length field covers; ok is false for a packet too short to hold
// its own header.
func tunnelPPPControlParse(packet []byte) (byte, byte, []byte, bool) {
	if len(packet) < 4 {
		return 0, 0, nil, false
	}
	length := int(binary.BigEndian.Uint16(packet[2:4]))
	if length < 4 {
		return 0, 0, nil, false
	}
	if length > len(packet) {
		return 0, 0, nil, false
	}
	return packet[0], packet[1], packet[4:length], true
}

// tunnelLCPOptionsParse splits an option list, stopping at a malformed option.
func tunnelLCPOptionsParse(body []byte) []tunnelLCPOption {
	var options []tunnelLCPOption
	for offset := 0; offset+2 <= len(body); {
		length := int(body[offset+1])
		if length < 2 {
			break
		}
		if offset+length > len(body) {
			break
		}
		options = append(options, tunnelLCPOption{kind: body[offset], value: append([]byte(nil), body[offset+2:offset+length]...)})
		offset += length
	}
	return options
}

// tunnelLCPApplyNak replaces each option the AC Nak'd with the value it
// suggested, and appends an option it asked for that we did not send.
func tunnelLCPApplyNak(options []tunnelLCPOption, nak []byte) []tunnelLCPOption {
	for _, suggested := range tunnelLCPOptionsParse(nak) {
		replaced := false
		for i := range options {
			if options[i].kind == suggested.kind {
				options[i].value = suggested.value
				replaced = true
			}
		}
		if !replaced {
			options = append(options, suggested)
		}
	}
	return options
}

// tunnelLCPApplyReject drops every option the AC rejected.
func tunnelLCPApplyReject(options []tunnelLCPOption, reject []byte) []tunnelLCPOption {
	for _, rejected := range tunnelLCPOptionsParse(reject) {
		kept := options[:0]
		for _, option := range options {
			if option.kind != rejected.kind {
				kept = append(kept, option)
			}
		}
		options = kept
	}
	return options
}
