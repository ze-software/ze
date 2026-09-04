package fixture

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // L2TP CHAP and RADIUS require MD5 on the wire
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func tunnelL2TPRadiusAccountingPeer(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	challenge, _ := tunnelHex("0f1e2d3c4b5a69788796a5b4c3d2e1f0")
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown
	reply, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(0x0321, "py-lac", challenge), 40, 250*time.Millisecond)
	if err != nil {
		return errors.New("no SCCRP received")
	}
	avps, err := tunnelL2TPParseAVPs(reply)
	if err != nil || len(avps[9]) != 2 || len(avps[11]) == 0 {
		return errors.New("SCCRP missing Assigned Tunnel ID or Challenge")
	}
	state := &tunnelAccountingPeer{
		conn: conn, target: target, localTID: binary.BigEndian.Uint16(avps[9]),
		ns: 1, nr: 1, startedAt: time.Now().Unix(),
	}
	digest := md5.Sum( //nolint:gosec // RFC 2661 Section 4.4.3 makes the Challenge Response a CHAP value, and RFC 1994 CHAP is MD5
		append(append([]byte{3}, tunnelL2TPTestSecret()...), avps[11]...))
	state.sendControl(append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(3)), tunnelL2TPAVP(true, 13, digest[:])...), 0)
	icrq := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(10)), tunnelL2TPAVP(true, 14, tunnelL2TPU16(700))...)
	icrq = append(icrq, tunnelL2TPAVP(true, 15, tunnelL2TPU32(42))...)
	// RFC 2661 Section 4.4.3 Calling Number (AVP 22): "encodes the originating
	// number for the incoming call ... The M-bit for this AVP MUST be set to
	// 1." parseICRQ stores it, and ze repeats it as Calling-Station-Id
	// (RFC 2865 Section 5.31) in every Accounting-Request of the session.
	icrq = append(icrq, tunnelL2TPAVP(true, 22, []byte(tunnelCallingNumber))...)
	state.sendControl(icrq, 0)
	deadline := time.Now().Add(10 * time.Second)
	for state.zeSID == 0 && time.Now().Before(deadline) {
		packet, readErr := state.readPacket(time.Second)
		if readErr != nil {
			continue
		}
		parsed, parseErr := tunnelL2TPParseAVPs(packet)
		if parseErr == nil && tunnelL2TPMessageType(parsed) == 11 && len(parsed[14]) == 2 {
			state.zeSID = binary.BigEndian.Uint16(parsed[14])
		}
		if len(packet) >= 12 && int(binary.BigEndian.Uint16(packet[2:4])) > 12 {
			ns := binary.BigEndian.Uint16(packet[8:10])
			if ns == state.nr {
				state.nr = ns + 1
			}
		}
	}
	if state.zeSID == 0 {
		return errors.New("no ICRP received")
	}
	fmt.Printf("OK: ICRP received, ze session id %d on tunnel %d\n", state.zeSID, state.localTID)
	peerOptions := append([]byte{1, 4, 0x05, 0xdc}, []byte{5, 6, 0x11, 0x22, 0x33, 0x44}...)
	// RFC 2661 Section 4.4.5 carries the LAC's Last-Sent LCP CONFREQ in AVP 27,
	// and EvaluateProxyLCP (internal/component/l2tp/ppp/proxy.go) reads the
	// authentication protocol from that AVP alone: the `l2tp auth-method` leaf
	// is not consulted on the proxy path. The LCP Authentication-Protocol
	// option (type 3) names CHAP (0xc223) with Algorithm 5, which RFC 1994
	// Section 4.1 makes MD5, so ze authenticates the peer over the tunnel and
	// can build an Access-Request that carries a credential. RFC 2865
	// Section 4.1 admits no Access-Request without one.
	lacOptions := append([]byte{1, 4, 0x05, 0xdc}, []byte{3, 5, 0xc2, 0x23, 0x05}...)
	lacOptions = append(lacOptions, 5, 6, 0x55, 0x66, 0x77, 0x88)
	iccn := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(12)), tunnelL2TPAVP(true, 24, tunnelL2TPU32(10000000))...)
	iccn = append(iccn, tunnelL2TPAVP(true, 19, tunnelL2TPU32(2))...)
	iccn = append(iccn, tunnelL2TPAVP(false, 26, peerOptions)...)
	iccn = append(iccn, tunnelL2TPAVP(false, 27, lacOptions)...)
	iccn = append(iccn, tunnelL2TPAVP(false, 28, peerOptions)...)
	state.sendControl(iccn, state.zeSID)
	fmt.Println("OK: ICCN sent with proxy LCP AVPs 26, 27 and 28")
	if err := state.negotiateIPCP(ctx); err != nil {
		return err
	}
	return state.checkRadius(ctx)
}

// tunnelCallingNumber is the originating number this peer puts in the ICRQ.
// It is a UK number from the range Ofcom reserves for drama and testing, and
// it carries no space, because the RADIUS-RX log line is read field by field.
const tunnelCallingNumber = "+441632960123"

// tunnelTerminateCauseLostCarrier is RFC 2866 Section 5.10 value 2, "DCD was
// dropped on the port". Ze reports it when its LCP echo probes stop being
// answered, which is how this peer ends the session: it goes silent, and ze
// tears the session down on its own timer.
const tunnelTerminateCauseLostCarrier = "2"

type tunnelAccountingPeer struct {
	conn           *net.UDPConn
	target         *net.UDPAddr
	localTID       uint16
	zeSID          uint16
	ns             uint16
	nr             uint16
	zeConfReqAcked bool
	ourIPCPID      byte
	ourRequestAddr net.IP
	ourIPCPAcked   bool
	negotiatedAddr net.IP
	// silent stops this peer answering LCP echo probes, and makes ze's own
	// teardown messages expected rather than a failure. It is set once the
	// Interim-Update has been asserted, to drive the Accounting-Stop.
	silent bool
	// startedAt is the wall clock this peer began at. Event-Timestamp is
	// asserted against it, so a zero or a stale stamp cannot pass.
	startedAt int64
}

func (peer *tunnelAccountingPeer) sendControl(body []byte, sid uint16) {
	_, _ = peer.conn.WriteToUDP(tunnelL2TPControl(peer.localTID, sid, peer.ns, peer.nr, body), peer.target)
	peer.ns++
}

func (peer *tunnelAccountingPeer) readPacket(timeout time.Duration) ([]byte, error) {
	buffer := make([]byte, 2048)
	_ = peer.conn.SetReadDeadline(time.Now().Add(timeout))
	n, _, err := peer.conn.ReadFromUDP(buffer)
	return append([]byte(nil), buffer[:n]...), err
}

func (peer *tunnelAccountingPeer) sendPPP(protocol uint16, payload []byte) {
	packet := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint16(packet[0:2], 2)
	binary.BigEndian.PutUint16(packet[2:4], peer.localTID)
	binary.BigEndian.PutUint16(packet[4:6], peer.zeSID)
	binary.BigEndian.PutUint16(packet[6:8], protocol)
	copy(packet[8:], payload)
	_, _ = peer.conn.WriteToUDP(packet, peer.target)
}

func tunnelPPPControl(code, identifier byte, body []byte) []byte {
	packet := make([]byte, 4+len(body))
	packet[0], packet[1] = code, identifier
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	copy(packet[4:], body)
	return packet
}

func (peer *tunnelAccountingPeer) negotiateIPCP(ctx context.Context) error {
	peer.ourRequestAddr = net.IPv4zero.To4()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if peer.zeConfReqAcked && peer.ourIPCPAcked {
			fmt.Printf("OK: IPCP opened, ze assigned %s\n", peer.negotiatedAddr)
			return nil
		}
		packet, err := peer.readPacket(250 * time.Millisecond)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		if err := peer.handleAccountingPacket(packet); err != nil {
			return err
		}
	}
	return errors.New("IPCP did not open within 30s")
}

func (peer *tunnelAccountingPeer) handleAccountingPacket(packet []byte) error {
	if len(packet) < 6 {
		return nil
	}
	flags := binary.BigEndian.Uint16(packet[:2])
	if flags&0x8000 != 0 {
		if len(packet) >= 12 {
			length := int(binary.BigEndian.Uint16(packet[2:4]))
			if length > 12 && length <= len(packet) {
				ns := binary.BigEndian.Uint16(packet[8:10])
				if ns == peer.nr {
					peer.nr = ns + 1
				}
				avps, _ := tunnelL2TPParseAVPs(packet)
				switch tunnelL2TPMessageType(avps) {
				case 4:
					if peer.silent {
						return nil
					}
					return errors.New("ze tore the tunnel down")
				case 14:
					// A CDN is what ze owes a session it has torn down, so it
					// is the expected end of the silent phase, not a failure.
					if peer.silent {
						return nil
					}
					return errors.New("ze disconnected the call")
				default:
					_, _ = peer.conn.WriteToUDP(tunnelL2TPControl(peer.localTID, 0, peer.ns, peer.nr, nil), peer.target)
				}
			}
		}
		return nil
	}
	offset := 2
	if flags&0x4000 != 0 {
		offset += 2
	}
	offset += 4
	if flags&0x0800 != 0 {
		offset += 4
	}
	if flags&0x0200 != 0 {
		if len(packet) < offset+2 {
			return nil
		}
		offset += 2 + int(binary.BigEndian.Uint16(packet[offset:offset+2]))
	}
	if len(packet) < offset+6 {
		return nil
	}
	payload := packet[offset:]
	if bytes.HasPrefix(payload, []byte{0xff, 0x03}) {
		payload = payload[2:]
	}
	if len(payload) < 6 {
		return nil
	}
	// A silent peer has stopped answering: no echo reply, no CHAP, no IPCP.
	// That is the state ze reports as Lost Carrier once its echo probes go
	// unanswered past the limit.
	if peer.silent {
		return nil
	}
	protocol := binary.BigEndian.Uint16(payload[:2])
	code, identifier := payload[2], payload[3]
	length := int(binary.BigEndian.Uint16(payload[4:6]))
	if length < 4 || 2+length > len(payload) {
		return nil
	}
	body := payload[6 : 2+length]
	if protocol == 0xc223 {
		return peer.answerCHAP(code, identifier, body)
	}
	if protocol == 0xc021 {
		switch code {
		case 9:
			peer.sendPPP(0xc021, tunnelPPPControl(10, identifier, []byte{0x11, 0x22, 0x33, 0x44}))
		case 5:
			peer.sendPPP(0xc021, tunnelPPPControl(6, identifier, nil))
			return errors.New("ze terminated LCP")
		}
		return nil
	}
	if protocol != 0x8021 {
		return nil
	}
	switch code {
	case 1:
		peer.sendPPP(0x8021, tunnelPPPControl(2, identifier, body))
		if !peer.zeConfReqAcked {
			peer.zeConfReqAcked = true
			peer.sendIPCPRequest()
		}
	case 3:
		address := tunnelIPCPAddress(body)
		if address == nil {
			return errors.New("IPCP Configure-Nak without IP-Address option")
		}
		if address.Equal(peer.ourRequestAddr) {
			return fmt.Errorf("ze re-Naked suggested address %s", address)
		}
		peer.ourRequestAddr = address
		peer.sendIPCPRequest()
	case 2:
		if identifier == peer.ourIPCPID && !peer.ourIPCPAcked {
			peer.ourIPCPAcked = true
			peer.negotiatedAddr = append(net.IP(nil), peer.ourRequestAddr...)
		}
	case 4:
		return errors.New("ze rejected IPCP IP-Address option")
	}
	return nil
}

// tunnelCHAPPeerName is the subscriber identity the peer puts in the Name
// field of its CHAP Response. ze copies that name into User-Name, and RFC 2865
// Section 5 says "Text of length zero (0) MUST NOT be sent", so the name must
// not be empty.
const tunnelCHAPPeerName = "subscriber@ze"

// tunnelCHAPPeerSecret is the CHAP secret this peer shares with the
// authenticator. The RADIUS mock accepts every Access-Request, so no component
// recomputes the digest: what the test proves is that a credential reaches the
// wire, not that a particular secret is right.
const tunnelCHAPPeerSecret = "chapsecret"

// answerCHAP plays the peer half of the RFC 1994 three-way handshake that ze
// drives as authenticator (runCHAPAuthPhase,
// internal/component/l2tp/ppp/chap.go). Code 1 is Challenge, code 3 Success
// and code 4 Failure; body is the CHAP packet after its 4-byte header, which
// for a Challenge is Value-Size, Value and Name.
func (peer *tunnelAccountingPeer) answerCHAP(code, identifier byte, body []byte) error {
	switch code {
	case 1:
		if len(body) < 1 {
			return errors.New("CHAP Challenge carried no Value-Size")
		}
		size := int(body[0])
		if 1+size > len(body) {
			return errors.New("CHAP Challenge Value-Size overruns the packet")
		}
		// RFC 1994 Section 4.1: the Response Value is "the one-way hash
		// calculated over a stream of octets consisting of the Identifier,
		// followed by (concatenated with) the "secret", followed by
		// (concatenated with) the Challenge Value". Algorithm 5 makes that
		// hash MD5.
		digest := md5.Sum( //nolint:gosec // RFC 1994 CHAP Algorithm 5 is MD5
			append(append([]byte{identifier}, tunnelCHAPPeerSecret...), body[1:1+size]...))
		response := append([]byte{byte(len(digest))}, digest[:]...)
		peer.sendPPP(0xc223, tunnelPPPControl(2, identifier, append(response, tunnelCHAPPeerName...)))
		fmt.Printf("OK: CHAP Response sent for challenge identifier %d as %s\n", identifier, tunnelCHAPPeerName)
	case 3:
		fmt.Println("OK: CHAP Success received")
	case 4:
		return fmt.Errorf("ze refused the CHAP Response: %s", body)
	}
	return nil
}

func (peer *tunnelAccountingPeer) sendIPCPRequest() {
	peer.ourIPCPID++
	option := append([]byte{3, 6}, peer.ourRequestAddr.To4()...)
	peer.sendPPP(0x8021, tunnelPPPControl(1, peer.ourIPCPID, option))
}

func tunnelIPCPAddress(options []byte) net.IP {
	for offset := 0; offset+2 <= len(options); {
		length := int(options[offset+1])
		if length < 2 || offset+length > len(options) {
			return nil
		}
		if options[offset] == 3 && length == 6 {
			return net.IP(append([]byte(nil), options[offset+2:offset+6]...))
		}
		offset += length
	}
	return nil
}

func (peer *tunnelAccountingPeer) checkRadius(ctx context.Context) error {
	access, err := peer.waitRadius(ctx, "Access-Request", 30*time.Second, func(line string) bool {
		return strings.HasPrefix(line, "RADIUS-RX Access-Request ")
	})
	if err != nil {
		return err
	}
	nasID := os.Getenv("TEST_NAS_ID")
	if nasID == "" {
		nasID = "lns1"
	}
	portPattern := regexp.MustCompile(`NAS-Port-Id=(\S+)`)
	match := portPattern.FindStringSubmatch(access)
	if len(match) != 2 {
		return fmt.Errorf("Access-Request carried no NAS-Port-Id: %s", access)
	}
	authPortID := match[1]
	validPortID := regexp.MustCompile(`^` + regexp.QuoteMeta(nasID) + `:\d+\.\d+$`)
	if !validPortID.MatchString(authPortID) {
		return fmt.Errorf("Access-Request NAS-Port-Id %q does not match template", authPortID)
	}
	fmt.Printf("OK: Access-Request NAS-Port-Id resolved from the template as %s\n", authPortID)
	accounting, err := peer.waitRadius(ctx, "Accounting-Start", 30*time.Second, func(line string) bool {
		return strings.HasPrefix(line, "RADIUS-RX Accounting-Request ") && strings.Contains(line, "Acct-Status-Type=Start")
	})
	if err != nil {
		return err
	}
	addressPattern := regexp.MustCompile(`Framed-IP-Address=(\S+)`)
	address := addressPattern.FindStringSubmatch(accounting)
	if len(address) != 2 || address[1] != peer.negotiatedAddr.String() {
		return fmt.Errorf("Accounting-Start address does not match IPCP: %s", accounting)
	}
	fmt.Printf("OK: Accounting-Start Framed-IP-Address matches the IPCP-negotiated address %s\n", address[1])
	accountingPort := portPattern.FindStringSubmatch(accounting)
	if len(accountingPort) != 2 || accountingPort[1] != authPortID {
		return fmt.Errorf("NAS-Port-Id is not stable across auth and accounting: %s", accounting)
	}
	fmt.Printf("OK: Accounting-Start carries the same NAS-Port-Id %s\n", authPortID)
	if err := peer.checkRecordAttributes("Accounting-Start", accounting, ""); err != nil {
		return err
	}
	// The Interim-Update is the second record ze sends, one acct-interval
	// after the session came up. The config sets that leaf to 60 seconds,
	// which is the floor its YANG range allows, so this wait is the slowest
	// part of the test and the deadline is sized well past it.
	interim, err := peer.waitRadius(ctx, "Accounting Interim-Update", 100*time.Second, func(line string) bool {
		return strings.HasPrefix(line, "RADIUS-RX Accounting-Request ") && strings.Contains(line, "Acct-Status-Type=Interim-Update")
	})
	if err != nil {
		return err
	}
	if err := peer.checkRecordAttributes("Accounting-Interim-Update", interim, ""); err != nil {
		return err
	}
	// Going silent ends the session the way a subscriber whose link died ends
	// it: ze's LCP echo probes stop being answered and ze tears the session
	// down itself, reporting Lost Carrier. The config runs those probes at one
	// second, so the teardown follows within a few seconds.
	peer.silent = true
	fmt.Println("OK: peer stopped answering LCP echo probes")
	stop, err := peer.waitRadius(ctx, "Accounting-Stop", 60*time.Second, func(line string) bool {
		return strings.HasPrefix(line, "RADIUS-RX Accounting-Request ") && strings.Contains(line, "Acct-Status-Type=Stop")
	})
	if err != nil {
		return err
	}
	return peer.checkRecordAttributes("Accounting-Stop", stop, tunnelTerminateCauseLostCarrier)
}

// checkRecordAttributes asserts, on one decoded RADIUS-RX line, the four
// attributes every subscriber Accounting-Request carries: Event-Timestamp
// (RFC 2869 Section 5.3), Calling-Station-Id (RFC 2865 Section 5.31),
// Acct-Delay-Time (RFC 2866 Section 5.2) and Acct-Terminate-Cause (RFC 2866
// Section 5.10).
//
// cause is the Section 5.10 value the record must report. An EMPTY cause means
// the attribute MUST be absent, because Section 5.10 says it "can only be
// present in Accounting-Request records where the Acct-Status-Type is set to
// Stop". That absence is the assertion an append in the wrong place fails.
func (peer *tunnelAccountingPeer) checkRecordAttributes(record, line, cause string) error {
	stamp, found := tunnelRadiusValue(line, "Event-Timestamp")
	if !found {
		return fmt.Errorf("%s carries no Event-Timestamp: %s", record, line)
	}
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%s Event-Timestamp %q is not an integer: %s", record, stamp, line)
	}
	if seconds < peer.startedAt {
		return fmt.Errorf("%s Event-Timestamp %d predates the run, which began at %d", record, seconds, peer.startedAt)
	}
	if seconds > time.Now().Unix()+1 {
		return fmt.Errorf("%s Event-Timestamp %d is in the future of this peer's clock", record, seconds)
	}
	station, found := tunnelRadiusValue(line, "Calling-Station-Id")
	if !found {
		return fmt.Errorf("%s carries no Calling-Station-Id: %s", record, line)
	}
	if station != tunnelCallingNumber {
		return fmt.Errorf("%s Calling-Station-Id is %q, not the ICRQ Calling Number %q", record, station, tunnelCallingNumber)
	}
	delay, found := tunnelRadiusValue(line, "Acct-Delay-Time")
	if !found {
		return fmt.Errorf("%s carries no Acct-Delay-Time: %s", record, line)
	}
	if _, err := strconv.ParseUint(delay, 10, 32); err != nil {
		return fmt.Errorf("%s Acct-Delay-Time %q is not an unsigned integer: %s", record, delay, line)
	}
	reported, found := tunnelRadiusValue(line, "Acct-Terminate-Cause")
	if cause == "" {
		if found {
			return fmt.Errorf("%s carries Acct-Terminate-Cause=%s, which RFC 2866 Section 5.10 allows on a Stop record alone: %s", record, reported, line)
		}
		fmt.Printf("OK: %s carries Event-Timestamp %s, Calling-Station-Id %s, Acct-Delay-Time %s and no Acct-Terminate-Cause\n",
			record, stamp, station, delay)
		return nil
	}
	if !found {
		return fmt.Errorf("%s carries no Acct-Terminate-Cause: %s", record, line)
	}
	if reported != cause {
		return fmt.Errorf("%s reports Acct-Terminate-Cause %s, not %s", record, reported, cause)
	}
	fmt.Printf("OK: %s carries Event-Timestamp %s, Calling-Station-Id %s, Acct-Delay-Time %s and Acct-Terminate-Cause %s\n",
		record, stamp, station, delay, reported)
	return nil
}

// tunnelRadiusValue reads one NAME=VALUE field out of a decoded RADIUS-RX
// line. The second return says whether the attribute was there at all, which
// is what an absence assertion needs.
func tunnelRadiusValue(line, name string) (string, bool) {
	prefix := name + "="
	for field := range strings.FieldsSeq(line) {
		if value, found := strings.CutPrefix(field, prefix); found {
			return value, true
		}
	}
	return "", false
}

// waitRadius reads radius-rx.log until one line satisfies predicate, and keeps
// answering ze in the meantime so the session stays up while it waits. The
// description names what the caller waited for, so a timeout says which record
// never arrived.
func (peer *tunnelAccountingPeer) waitRadius(ctx context.Context, description string, timeout time.Duration, predicate func(string) bool) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		contents, _ := os.ReadFile("radius-rx.log")
		for line := range strings.SplitSeq(string(contents), "\n") {
			if predicate(line) {
				return line, nil
			}
		}
		packet, err := peer.readPacket(250 * time.Millisecond)
		if err == nil {
			if err := peer.handleAccountingPacket(packet); err != nil {
				return "", err
			}
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return "", fmt.Errorf("%s did not arrive within %s", description, timeout)
}
