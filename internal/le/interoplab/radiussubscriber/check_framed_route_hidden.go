// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite, l2tp-lns-radius-framed-route-hidden-xl2tpd
// Related: checkers.go -- the capture, accounting, redial and REST helpers this checker reuses
// RFC 2661 Section 4.3, RFC 2865 Section 5.22, RFC 3579 Section 3.2 -- the hidden AVP, the route, the signature.
package radiussubscriber

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// ipWordDev is the device selector word of the ip and tc commands the
	// checkers run.
	ipWordDev = "dev"
	// lacXL2TPDConf is where the LAC's xl2tpd reads its configuration.
	lacXL2TPDConf = "/etc/xl2tpd/xl2tpd.conf"
	// l2tpCapturePath holds the datagrams ze's container receives on its LNS
	// port, radiusCapturePath those it receives from the server's
	// authentication port.
	l2tpCapturePath   = "/tmp/ze-l2tp.pcap"
	radiusCapturePath = "/tmp/ze-radius.pcap"
	lnsPort           = "1701"
	authPort          = "1812"
	// The scenario's users file returns Framed-Route = "203.0.113.128/25
	// 203.0.113.77 1". The gateway is not the subscriber's address, so ze
	// installs a device route; framedRouteTarget sits inside the prefix, on
	// the LAC's loopback.
	framedRoutePrefix   = "203.0.113.128/25"
	framedRouteTarget   = "203.0.113.129"
	framedRouteMetric   = 1
	framedRouteProtocol = "253"
	subscriberUser      = "alice"
	lacLink             = "ppp0"

	// RFC 2661 Section 3.1 header flags, and Section 4.1 AVP flags and length.
	l2tpTypeBit     = 0x8000
	l2tpLengthBit   = 0x4000
	l2tpSequenceBit = 0x0800
	l2tpOffsetBit   = 0x0200
	l2tpVersionMask = 0x000f
	l2tpVersion     = 2
	avpHiddenBit    = 0x4000
	avpLengthMask   = 0x03ff
	avpHeaderOctets = 6
	// RFC 2661 Section 4.4.1: Message Type is attribute 0, ICRQ is type 10;
	// Section 4.4.4: Assigned Session ID is attribute 14.
	avpMessageType       = 0
	avpAssignedSessionID = 14
	l2tpMessageICRQ      = 10

	// RFC 2865 Section 4.2: Access-Accept is Code 2. RFC 3579 Section 3.2:
	// Message-Authenticator is attribute 80, 18 octets long.
	radiusCodeAccessAccept              = 2
	radiusAttributeMessageAuthenticator = 80
	radiusMessageAuthenticatorOctets    = 18
	radiusHeaderOctets                  = 20
)

// configureFramedRouteHiddenPeers gives this scenario its own users file,
// whose Access-Accept carries the Framed-Route, and its own xl2tpd.conf, which
// adds `challenge = yes` and `hidden bit = yes` to the lab's LAC.
func configureFramedRouteHiddenPeers(source interoplab.ScenarioSource, server, lac *interoplab.PeerConfig) {
	overrideMount(server, serverUsers, filepath.Join(source.Directory, "users"))
	overrideMount(lac, lacXL2TPDConf, filepath.Join(source.Directory, "xl2tpd.conf"))
}

// checkFramedRouteHidden proves four things on one xl2tpd session, with ze as
// LNS and FreeRADIUS as the subscriber's server.
//
// The LAC hides the ICRQ's Assigned Session ID, and the session still comes
// up, because ze reveals it under its shared-secret (revealHiddenAVPs,
// internal/component/l2tp/hidden.go). The lab's xl2tpd is built with
// -DTEST_HIDDEN (Dockerfile.lac): add_callid_avp (avpsend.c) then hides AVP 14
// when the tunnel's hbit is set, after the Random Vector the ICRQ branch of
// control_finish (control.c) adds, through encrypt_avp (aaa.c) under the
// secret handle_challenge read for the LAC's `challenge = yes`.
//
// FreeRADIUS's Access-Accept carries a Message-Authenticator, which rad_encode
// (src/lib/radius.c, release_3_2_10) writes first into every Access-Accept,
// and ze, which requires one by default, accepts it: the session starts.
//
// The Framed-Route rlm_files' file_common moves into the reply is a device
// route through the session's interface, proto 253 metric 1 with no gateway,
// and carries traffic to an address the LAC holds inside the prefix.
//
// `clear l2tp session user alice` ends the session: the Accounting-Stop
// reaches the server, pppd's ppp0 goes, and the route leaves ze's kernel.
// MUTATION: return body unchanged from revealHiddenAVPs; the mandatory hidden
// AVP 14 then refuses the ICRQ with a CDN and no Accounting-Start arrives.
func checkFramedRouteHidden(ctx context.Context, lab *scenarioLab) error {
	first, err := lab.waitStart(ctx, "")
	if err != nil {
		return err
	}
	// The LAC dialed before the checker ran, so the captures start now and the
	// LAC dials again into them.
	if err := lab.startPacketCapture(ctx, l2tpCapturePath, "udp", "dst", "port", lnsPort); err != nil {
		return err
	}
	if err := lab.startPacketCapture(ctx, radiusCapturePath, "udp", "src", "port", authPort); err != nil {
		return err
	}
	if err := lab.redial(ctx); err != nil {
		return err
	}
	second, err := lab.waitStart(ctx, first.sessionID)
	if err != nil {
		return err
	}
	// The first session MUST be gone, or the clear by user name below matches
	// two sessions and clears neither.
	if err := lab.waitStop(ctx, first.sessionID); err != nil {
		return err
	}
	if err := lab.waitHiddenICRQ(ctx); err != nil {
		return err
	}
	if err := lab.waitSignedAccept(ctx); err != nil {
		return err
	}
	if err := lab.waitFramedRoute(ctx); err != nil {
		return err
	}
	if err := lab.framedRoutePing(ctx); err != nil {
		return err
	}
	return lab.clearSubscriber(ctx, second.sessionID)
}

// waitHiddenICRQ waits for an ICRQ in the L2TP capture and requires every
// ICRQ captured to carry its Assigned Session ID with the H bit set.
func (s *scenarioLab) waitHiddenICRQ(ctx context.Context) error {
	icrqs, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "the LAC's ICRQ in ze's L2TP capture",
	}, func(probeCtx context.Context) ([]icrqSessionID, error) {
		frames, err := s.capturedFrames(probeCtx, l2tpCapturePath)
		if err != nil {
			return nil, err
		}
		return capturedICRQs(frames), nil
	}, func(icrqs []icrqSessionID) bool { return len(icrqs) > 0 })
	if err != nil {
		return fmt.Errorf("ze's L2TP capture holds no ICRQ: %w", err)
	}
	return judgeHiddenICRQs(icrqs)
}

// waitSignedAccept waits for an Access-Accept in the RADIUS capture and
// requires every one captured to carry a Message-Authenticator.
func (s *scenarioLab) waitSignedAccept(ctx context.Context) error {
	accepts, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "FreeRADIUS's Access-Accept in ze's RADIUS capture",
	}, func(probeCtx context.Context) ([]accessAccept, error) {
		frames, err := s.capturedFrames(probeCtx, radiusCapturePath)
		if err != nil {
			return nil, err
		}
		return capturedAccepts(frames), nil
	}, func(accepts []accessAccept) bool { return len(accepts) > 0 })
	if err != nil {
		return fmt.Errorf("ze's RADIUS capture holds no Access-Accept: %w", err)
	}
	return judgeSignedAccepts(accepts)
}

// capturedFrames answers the frames of the capture at path so far.
func (s *scenarioLab) capturedFrames(ctx context.Context, path string) ([][]byte, error) {
	capture, err := s.readPacketCapture(ctx, path)
	if err != nil {
		return nil, err
	}
	return captureFrames(capture)
}

// icrqSessionID is what one captured ICRQ says of its Assigned Session ID.
type icrqSessionID struct {
	present bool
	hidden  bool
}

// capturedICRQs answers the Assigned Session ID of each ICRQ the frames carry.
func capturedICRQs(frames [][]byte) []icrqSessionID {
	var icrqs []icrqSessionID
	for _, frame := range frames {
		_, payload, ok := udpOfFrame(frame)
		if !ok {
			continue
		}
		if icrq, isICRQ := icrqAssignedSessionID(payload); isICRQ {
			icrqs = append(icrqs, icrq)
		}
	}
	return icrqs
}

// judgeHiddenICRQs refuses an ICRQ whose Assigned Session ID is absent or in
// clear: either is a LAC that hid nothing, and the session coming up would then
// prove nothing about ze's reveal.
func judgeHiddenICRQs(icrqs []icrqSessionID) error {
	if len(icrqs) == 0 {
		return errors.New("no ICRQ captured")
	}
	for index, icrq := range icrqs {
		if !icrq.present {
			return fmt.Errorf("ICRQ %d carries no Assigned Session ID AVP", index)
		}
		if !icrq.hidden {
			return fmt.Errorf("ICRQ %d carries its Assigned Session ID in clear: the LAC hid nothing "+
				"(is the LAC image built with -DTEST_HIDDEN and `hidden bit = yes`?)", index)
		}
	}
	return nil
}

// icrqAssignedSessionID reads one UDP payload as an L2TP control message and,
// when it is an ICRQ, answers what it says of AVP 14. Every length is checked
// against the octets that remain, and each AVP consumes at least six octets,
// so the walk ends within the payload.
//
// RFC 2661 Section 3.1, the header; the optional fields follow the flags:
//
//	 0                   1                   2                   3
//	|T|L|x|x|S|x|O|P|x|x|x|x|  Ver  |          Length (opt)         |
//	|           Tunnel ID           |           Session ID          |
//	|             Ns (opt)          |             Nr (opt)          |
//	|      Offset Size (opt)        |    Offset pad... (opt)
//
// RFC 2661 Section 4.1, each AVP:
//
//	|M|H| rsvd  |      Length       |           Vendor ID           |
//	|         Attribute Type        |        Attribute Value...
func icrqAssignedSessionID(payload []byte) (icrqSessionID, bool) {
	avps, ok := l2tpControlAVPs(payload)
	if !ok {
		return icrqSessionID{}, false
	}
	// RFC 2661 Section 4.4.1: "The Message Type AVP MUST be the first AVP in a message, immediately following the control message header (defined in section 3.1)."
	if len(avps) < avpHeaderOctets+2 {
		return icrqSessionID{}, false
	}
	head := binary.BigEndian.Uint16(avps[0:2])
	if int(head&avpLengthMask) != avpHeaderOctets+2 || binary.BigEndian.Uint16(avps[2:4]) != 0 {
		return icrqSessionID{}, false
	}
	if binary.BigEndian.Uint16(avps[4:6]) != avpMessageType {
		return icrqSessionID{}, false
	}
	if binary.BigEndian.Uint16(avps[6:8]) != l2tpMessageICRQ {
		return icrqSessionID{}, false
	}
	var icrq icrqSessionID
	for rest := avps[avpHeaderOctets+2:]; len(rest) >= avpHeaderOctets; {
		head := binary.BigEndian.Uint16(rest[0:2])
		length := int(head & avpLengthMask)
		if length < avpHeaderOctets {
			return icrqSessionID{}, false
		}
		if length > len(rest) {
			return icrqSessionID{}, false
		}
		if binary.BigEndian.Uint16(rest[2:4]) == 0 && binary.BigEndian.Uint16(rest[4:6]) == avpAssignedSessionID {
			icrq.present = true
			// RFC 2661 Section 4.1: "Hidden (H) bit: Identifies the hiding of data in the Attribute Value field of an AVP."
			icrq.hidden = head&avpHiddenBit != 0
		}
		rest = rest[length:]
	}
	return icrq, true
}

// l2tpControlAVPs answers the AVPs of an L2TPv2 control message, bounded by
// its Length field, and false for a data message or a short one.
func l2tpControlAVPs(payload []byte) ([]byte, bool) {
	if len(payload) < 2 {
		return nil, false
	}
	flags := binary.BigEndian.Uint16(payload[0:2])
	if flags&l2tpTypeBit == 0 {
		return nil, false
	}
	if flags&l2tpVersionMask != l2tpVersion {
		return nil, false
	}
	offset, end := 2, len(payload)
	if flags&l2tpLengthBit != 0 {
		if len(payload) < offset+2 {
			return nil, false
		}
		length := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
		if length > len(payload) {
			return nil, false
		}
		end = length
		offset += 2
	}
	offset += 4
	if flags&l2tpSequenceBit != 0 {
		offset += 4
	}
	if flags&l2tpOffsetBit != 0 {
		if end < offset+2 {
			return nil, false
		}
		offset += 2 + int(binary.BigEndian.Uint16(payload[offset:offset+2]))
	}
	if offset > end {
		return nil, false
	}
	return payload[offset:end], true
}

// accessAccept is what one captured Access-Accept says of its signature.
type accessAccept struct {
	signed bool
}

// capturedAccepts answers each Access-Accept the frames carry.
func capturedAccepts(frames [][]byte) []accessAccept {
	var accepts []accessAccept
	for _, frame := range frames {
		datagram, ok := radiusOfFrame(frame)
		if !ok {
			continue
		}
		if datagram.code != radiusCodeAccessAccept {
			continue
		}
		accepts = append(accepts, accessAccept{signed: carriesMessageAuthenticator(datagram.octets)})
	}
	return accepts
}

// carriesMessageAuthenticator walks the attributes of one RADIUS packet,
// bounded by its Length field, and answers whether one is an 18-octet
// Message-Authenticator. A packet whose lengths overrun is not signed.
//
// RFC 2865 Section 3: Code 1, Identifier 1, Length 2, Authenticator 16, then
// attributes of Type 1, Length 1 (the whole attribute), Value.
func carriesMessageAuthenticator(octets []byte) bool {
	if len(octets) < radiusHeaderOctets {
		return false
	}
	length := int(binary.BigEndian.Uint16(octets[2:4]))
	if length < radiusHeaderOctets || length > len(octets) {
		return false
	}
	for rest := octets[radiusHeaderOctets:length]; len(rest) >= 2; {
		attributeOctets := int(rest[1])
		if attributeOctets < 2 || attributeOctets > len(rest) {
			return false
		}
		if rest[0] == radiusAttributeMessageAuthenticator && attributeOctets == radiusMessageAuthenticatorOctets {
			return true
		}
		rest = rest[attributeOctets:]
	}
	return false
}

// judgeSignedAccepts refuses an Access-Accept with no Message-Authenticator:
// ze accepting such an answer would be the BlastRADIUS defense switched off.
func judgeSignedAccepts(accepts []accessAccept) error {
	if len(accepts) == 0 {
		return errors.New("no Access-Accept captured")
	}
	for index, accept := range accepts {
		if !accept.signed {
			return fmt.Errorf("Access-Accept %d carries no Message-Authenticator (attribute 80)", index)
		}
	}
	return nil
}

// kernelRoute is one route `ip -j route` prints. Protocol is raw because
// iproute2 prints a protocol it has no name for as its number, in a string.
type kernelRoute struct {
	Destination string          `json:"dst"`
	Gateway     string          `json:"gateway"`
	Device      string          `json:"dev"`
	Protocol    json.RawMessage `json:"protocol"`
	Metric      int             `json:"metric"`
}

func (r kernelRoute) protocol() string {
	return strings.Trim(string(r.Protocol), "\"")
}

// waitFramedRoute waits for the Framed-Route in ze's main table and requires
// it to be a device route through the session's interface, which is the
// interface ze routes the subscriber's own address through.
func (s *scenarioLab) waitFramedRoute(ctx context.Context) error {
	device, err := s.sessionDevice(ctx)
	if err != nil {
		return err
	}
	routes, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "the Framed-Route " + framedRoutePrefix + " in ze's kernel",
	}, func(probeCtx context.Context) ([]kernelRoute, error) {
		return s.zeRoutes(probeCtx, framedRoutePrefix)
	}, func(routes []kernelRoute) bool { return len(routes) > 0 })
	if err != nil {
		return err
	}
	return judgeFramedRoute(routes, device)
}

// judgeFramedRoute requires exactly one route for the prefix: through device,
// with no gateway, proto 253 and metric 1 (RFC 2865 Section 5.22's first
// metric).
func judgeFramedRoute(routes []kernelRoute, device string) error {
	if len(routes) != 1 {
		return fmt.Errorf("ze's kernel holds %d routes for %s, want one: %+v", len(routes), framedRoutePrefix, routes)
	}
	route := routes[0]
	if route.Device != device {
		return fmt.Errorf("the Framed-Route goes through %q, want the session's %q", route.Device, device)
	}
	if route.Gateway != "" {
		return fmt.Errorf("the Framed-Route goes via %s, want a device route: the gateway is not the subscriber's address", route.Gateway)
	}
	if route.protocol() != framedRouteProtocol {
		return fmt.Errorf("the Framed-Route carries proto %s, want %s", route.protocol(), framedRouteProtocol)
	}
	if route.Metric != framedRouteMetric {
		return fmt.Errorf("the Framed-Route carries metric %d, want %d", route.Metric, framedRouteMetric)
	}
	return nil
}

// sessionDevice answers ze's interface for the session: the one its kernel
// routes the address pppd holds on ppp0 through. It MUST be a PPP interface.
func (s *scenarioLab) sessionDevice(ctx context.Context) (string, error) {
	output, err := s.lab.Query(ctx, peerLAC, []string{"ip", "-j", "-4", "addr", verbShow, ipWordDev, lacLink}, nil)
	if err != nil {
		return "", fmt.Errorf("read the LAC's %s address: %w", lacLink, err)
	}
	var links []struct {
		Addresses []struct {
			Local string `json:"local"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal([]byte(output), &links); err != nil {
		return "", fmt.Errorf("decode the LAC's %s address: %w", lacLink, err)
	}
	if len(links) != 1 || len(links[0].Addresses) != 1 {
		return "", fmt.Errorf("the LAC's %s holds no single IPv4 address: %s", lacLink, output)
	}
	subscriber := links[0].Addresses[0].Local
	output, err = s.lab.Query(ctx, peerZe, []string{"ip", "-j", "route", "get", subscriber}, nil)
	if err != nil {
		return "", fmt.Errorf("ze's route to the subscriber %s: %w", subscriber, err)
	}
	var routes []kernelRoute
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return "", fmt.Errorf("decode ze's route to the subscriber %s: %w", subscriber, err)
	}
	if len(routes) != 1 || !strings.HasPrefix(routes[0].Device, "ppp") {
		return "", fmt.Errorf("ze does not route the subscriber %s through one PPP interface: %s", subscriber, output)
	}
	return routes[0].Device, nil
}

// zeRoutes answers ze's main-table routes whose prefix is exactly prefix. An
// empty answer is no route.
func (s *scenarioLab) zeRoutes(ctx context.Context, prefix string) ([]kernelRoute, error) {
	result, err := s.lab.Exec(ctx, peerZe, []string{"ip", "-j", "route", verbShow, prefix}, nil)
	if err != nil {
		return nil, fmt.Errorf("read ze's routes for %s: %w: %s", prefix, err, result.Stderr)
	}
	output := strings.TrimSpace(result.Stdout)
	if output == "" {
		return nil, nil
	}
	var routes []kernelRoute
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return nil, fmt.Errorf("decode ze's routes for %s: %w", prefix, err)
	}
	return routes, nil
}

// framedRoutePing puts framedRouteTarget on the LAC's loopback and pings it
// from ze, whose only route to it is the Framed-Route.
func (s *scenarioLab) framedRoutePing(ctx context.Context) error {
	result, err := s.lab.Exec(ctx, peerLAC, []string{"ip", "addr", "add", framedRouteTarget + "/32", ipWordDev, "lo"}, nil)
	if err != nil {
		return fmt.Errorf("put %s on the LAC's loopback: %w: %s", framedRouteTarget, err, result.Stderr)
	}
	result, err = s.lab.Exec(ctx, peerZe, []string{"ping", "-c", "3", "-W", "2", framedRouteTarget}, nil)
	if err != nil {
		return fmt.Errorf("ze cannot reach %s through the Framed-Route: %w: %s", framedRouteTarget, err, result.Stdout+result.Stderr)
	}
	return nil
}

// clearSubscriber clears the session by user name through ze's REST surface,
// then waits for its Accounting-Stop, for pppd's ppp0 to go, and for the
// Framed-Route to leave ze's kernel.
func (s *scenarioLab) clearSubscriber(ctx context.Context, sessionID string) error {
	answer, err := s.execute(ctx, "clear l2tp session user "+subscriberUser)
	if err != nil {
		return err
	}
	// A successful answer carries session-id and username
	// (teardownMatchedSession, l2tp/cmd/session_teardown_match.go).
	if !strings.Contains(answer, `"session-id"`) || !strings.Contains(answer, `"username"`) {
		return fmt.Errorf("clear l2tp session user %s cleared nothing: %s", subscriberUser, answer)
	}
	if err := s.waitStop(ctx, sessionID); err != nil {
		return err
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "pppd's " + lacLink + " gone from the LAC",
	}, s.lacLinks, func(links []string) bool {
		return !slices.Contains(links, lacLink)
	}); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "the Framed-Route " + framedRoutePrefix + " gone from ze's kernel",
	}, func(probeCtx context.Context) ([]kernelRoute, error) {
		return s.zeRoutes(probeCtx, framedRoutePrefix)
	}, func(routes []kernelRoute) bool { return len(routes) == 0 })
	return err
}

// lacLinks answers the interface names the LAC's kernel holds.
func (s *scenarioLab) lacLinks(ctx context.Context) ([]string, error) {
	output, err := s.lab.Query(ctx, peerLAC, []string{"ip", "-j", "link", verbShow}, nil)
	if err != nil {
		return nil, err
	}
	var links []struct {
		Name string `json:"ifname"`
	}
	if err := json.Unmarshal([]byte(output), &links); err != nil {
		return nil, fmt.Errorf("decode the LAC's links: %w", err)
	}
	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, link.Name)
	}
	return names, nil
}
