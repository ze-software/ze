// Design: docs/labs/l2tp-interop.md -- complete PPP framing capture and compressed LCP reception.
// Related: ppp_code_reject.go -- original Code-Reject assertions and strict PPP reader.
// Related: avprelay/pppframing.go -- independent wire producer and capture ordinals.
// RFC 1661 Section 6.6 -- see rfc/short/rfc1661.md.
package l2tp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// The relay receives at most 65536 datagrams and adds six injections.
	// This tail is larger than that population; ordinal checks still refuse
	// any omitted normal data rather than trusting the Docker tail alone.
	pppCaptureLines         = 1 << 17
	compressedPPPMarker     = "/run/compressed-lcp"
	relayCompressedPPP      = "inject-compressed-lcp"
	compressedPPPIdentifier = 0x6f
	compressedPPPData       = "ff03-receive"
	codeConfigureAck        = 2
	codeEchoRequest         = 9
	codeEchoReply           = 10
)

// pppFramingError carries a Ze-output framing failure through an earlier poll
// to the dedicated framing assertion, so a recorder cannot cite a sibling red.
type pppFramingError struct{ cause error }

func (e *pppFramingError) Error() string { return e.cause.Error() }
func (e *pppFramingError) Unwrap() error { return e.cause }

// pppCodeRejectFailure preserves the existing numbered error shape and keeps
// a framing failure on its own assertion before Wait can flatten its type.
func pppCodeRejectFailure(name string, assertion int, cause error) error {
	if _, framing := errors.AsType[*pppFramingError](cause); framing {
		assertion = 7
	}
	return fmt.Errorf("scenario %s assertion %d: %w", name, assertion, cause)
}

// pppFramingLog holds both the complete data population and the control-frame
// population HEAD judged after its codeRejectMarker check. The latter includes
// the armed trigger before the first injection; injection is not its fence.
type pppFramingLog struct {
	frames       []pppFrame
	armed        []pppFrame
	phaseStarted bool
}

// framingPPPFrames is the full-session view, including IP. Startup is a valid
// capture before the Code-Reject phase has been armed.
func framingPPPFrames(text string) ([]pppFrame, error) {
	capture, err := readPPPFramingLog(text)
	return capture.frames, err
}

// armedPPPFrames preserves HEAD's marker-based population for the original
// assertions. A missing phase witness is an error, never an empty accepted
// stability interval.
func armedPPPFrames(text string) ([]pppFrame, error) {
	capture, err := readPPPFramingLog(text)
	if err != nil {
		return nil, err
	}
	if !capture.phaseStarted {
		return nil, errors.New("PPP capture has no armed normal-data phase witness")
	}
	return capture.armed, nil
}

// readPPPFramingLog requires consecutive normal-data ordinals from one and an
// explicit marker state on every normal row. Once armed, the state MUST NOT
// retreat. The relay samples it after forwarding and before logging/injecting,
// at the same point HEAD gated its first logged normal datagram.
func readPPPFramingLog(text string) (pppFramingLog, error) {
	capture := pppFramingLog{frames: make([]pppFrame, 0, 64), armed: make([]pppFrame, 0, 16)}
	if len(text) > 16<<20 {
		return capture, errors.New("PPP capture exceeds 16 MiB")
	}
	ordinal := 0
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event struct {
			avpWireObservation
			DataOrdinal     int   `json:"data_ordinal"`
			CodeRejectArmed *bool `json:"code_reject_armed"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return capture, fmt.Errorf("decode complete PPP capture: %w", err)
		}
		datagram, err := hex.DecodeString(event.Forwarded)
		if err != nil {
			return capture, fmt.Errorf("decode captured PPP datagram: %w", err)
		}
		if len(datagram) >= 2 && datagram[0]&0x80 != 0 {
			continue
		}
		if event.Direction == relayCompressedPPP {
			if !capture.phaseStarted {
				return capture, errors.New("compressed injection precedes the armed normal-data witness")
			}
			packet, err := compressedPPPInformation(datagram)
			if err != nil {
				return capture, err
			}
			frame := pppFrame{direction: relayCompressedPPP, protocol: protocolLCP, packet: packet}
			capture.frames = append(capture.frames, frame)
			capture.armed = append(capture.armed, frame)
			continue
		}
		normal := event.Direction == relayFromZe
		if event.Direction == relayFromLAC {
			normal = true
		}
		if normal {
			ordinal++
			if event.DataOrdinal != ordinal {
				return capture, fmt.Errorf("PPP capture has data ordinal %d, want %d from the session's first datagram", event.DataOrdinal, ordinal)
			}
			if event.CodeRejectArmed == nil {
				return capture, errors.New("normal PPP capture lacks code_reject_armed metadata")
			}
			if capture.phaseStarted {
				if !*event.CodeRejectArmed {
					return capture, errors.New("PPP Code-Reject armed phase retreated")
				}
			}
			if *event.CodeRejectArmed {
				capture.phaseStarted = true
			}
		} else {
			if event.Direction != relayInjected {
				return capture, errors.New("unknown PPP relay direction")
			}
			if !capture.phaseStarted {
				return capture, errors.New("original probe precedes the armed normal-data witness")
			}
		}
		protocol, packet, err := pppInformation(datagram)
		if err != nil {
			if event.Direction == relayFromZe {
				return capture, &pppFramingError{cause: err}
			}
			return capture, err
		}
		frame := pppFrame{direction: event.Direction, protocol: protocol, packet: packet}
		capture.frames = append(capture.frames, frame)
		if capture.phaseStarted {
			if protocol&0x8000 != 0 {
				capture.armed = append(capture.armed, frame)
			}
		}
	}
	if ordinal == 0 {
		return capture, errors.New("PPP capture holds no normal data datagram")
	}
	return capture, nil
}

// pppNegotiation is the pair of Magic-Numbers the actual Configure exchanges
// acknowledged, after proving neither acknowledged request contains ACFC.
type pppNegotiation struct{ peerMagic, zeMagic uint32 }

// negotiatedPPPFraming ties each Configure-Ack to the byte-identical options
// in the opposite peer's request, before the first original probe injection.
// RFC 1661 Section 6.6: "By default, all implementations MUST transmit frames
// with Address and Control fields appropriate to the link framing."
func negotiatedPPPFraming(frames []pppFrame) (pppNegotiation, error) {
	requests := make(map[string]map[byte][]byte, 2)
	var negotiated pppNegotiation
	peerAcked, zeAcked := false, false
	for _, frame := range frames {
		if frame.direction == relayInjected {
			break
		}
		if frame.protocol != protocolLCP {
			continue
		}
		if len(frame.packet) < 4 {
			return negotiated, errors.New("short captured LCP packet")
		}
		packet := frame.packet
		size := int(binary.BigEndian.Uint16(packet[2:]))
		if size < 4 {
			return negotiated, errors.New("invalid captured LCP Length")
		}
		if size > len(packet) {
			return negotiated, errors.New("truncated captured LCP packet")
		}
		packet = packet[:size]
		if packet[0] == codeConfigureRequest {
			if requests[frame.direction] == nil {
				requests[frame.direction] = make(map[byte][]byte)
			}
			requests[frame.direction][packet[1]] = packet
			continue
		}
		if packet[0] != codeConfigureAck {
			continue
		}
		requester := relayFromZe
		if frame.direction == relayFromZe {
			requester = relayFromLAC
		}
		request := requests[requester][packet[1]]
		if len(request) == 0 {
			return negotiated, errors.New("LCP Configure-Ack has no captured request")
		}
		if !bytes.Equal(request[2:], packet[2:]) {
			return negotiated, errors.New("LCP Configure-Ack changed the request options")
		}
		magic, err := acknowledgedPPPMagic(packet[4:])
		if err != nil {
			return negotiated, err
		}
		if frame.direction == relayFromZe {
			negotiated.peerMagic, peerAcked = magic, true
		} else {
			negotiated.zeMagic, zeAcked = magic, true
		}
	}
	if !peerAcked {
		return negotiated, errors.New("startup capture has no Ze Configure-Ack for the LAC")
	}
	if !zeAcked {
		return negotiated, errors.New("startup capture has no LAC Configure-Ack for Ze")
	}
	return negotiated, nil
}

// acknowledgedPPPMagic reads the acknowledged peer options. A missing
// Magic-Number means zero, as RFC 1661 Section 6.4 requires for Echo packets.
// RFC 1661 Section 6.6: "This Configuration Option is sent to inform the peer
// that the implementation can receive compressed Address and Control fields."
func acknowledgedPPPMagic(options []byte) (uint32, error) {
	var magic uint32
	for len(options) > 0 {
		if len(options) < 2 {
			return 0, errors.New("short acknowledged LCP option")
		}
		size := int(options[1])
		if size < 2 {
			return 0, errors.New("invalid acknowledged LCP option Length")
		}
		if size > len(options) {
			return 0, errors.New("truncated acknowledged LCP option")
		}
		if options[0] == 8 {
			return 0, errors.New("ACFC was negotiated in the compressed-input negative")
		}
		if options[0] == 5 {
			if size != 6 {
				return 0, errors.New("acknowledged Magic-Number has the wrong Length")
			}
			magic = binary.BigEndian.Uint32(options[2:6])
		}
		options = options[size:]
	}
	return magic, nil
}

// armCompressedPPPEcho supplies the Magic-Number the real startup exchange
// acknowledged. The value is a positional argument, never shell syntax.
// The relay reads this exact eight-hex-digit marker only when injecting.
func armCompressedPPPEcho(ctx context.Context, lab labOperations) error {
	logs, err := lab.Logs(ctx, peerAVPRelay, pppCaptureLines)
	if err != nil {
		return err
	}
	frames, err := framingPPPFrames(logs.Text)
	if err != nil {
		return err
	}
	negotiated, err := negotiatedPPPFraming(frames)
	if err != nil {
		return err
	}
	_, err = lab.Exec(ctx, peerAVPRelay, []string{"sh", "-c", "printf '%s\\n' \"$1\" > \"$2\"", "compressed-lcp", fmt.Sprintf("%08x", negotiated.peerMagic), compressedPPPMarker}, nil)
	return err
}

// compressedPPPInformation recognizes only the one intentionally unframed
// Echo-Request. It does not accept any ordinary unframed observation.
// RFC 1661 Section 6.6: "If a compressed frame is received when Address-and-Control-Field-Compression has not been negotiated, the implementation MAY silently discard the frame."
func compressedPPPInformation(datagram []byte) ([]byte, error) {
	if len(datagram) < 6 {
		return nil, errors.New("short compressed L2TP data header")
	}
	flags := binary.BigEndian.Uint16(datagram)
	start := 2
	if flags&0x4000 != 0 {
		start += 2
	}
	start += 4
	if flags&0x0800 != 0 {
		start += 4
	}
	if flags&0x0200 != 0 {
		if len(datagram) < start+2 {
			return nil, errors.New("short compressed L2TP offset")
		}
		start += 2 + int(binary.BigEndian.Uint16(datagram[start:]))
	}
	if len(datagram) < start+2 {
		return nil, errors.New("short compressed PPP Protocol")
	}
	if binary.BigEndian.Uint16(datagram[start:]) != protocolLCP {
		return nil, errors.New("compressed injection is not LCP")
	}
	packet := datagram[start+2:]
	if len(packet) != 8+len(compressedPPPData) {
		return nil, errors.New("compressed Echo-Request has the wrong length")
	}
	if packet[0] != codeEchoRequest {
		return nil, errors.New("compressed injection is not an Echo-Request")
	}
	if packet[1] != compressedPPPIdentifier {
		return nil, errors.New("compressed Echo-Request has another Identifier")
	}
	if int(binary.BigEndian.Uint16(packet[2:])) != len(packet) {
		return nil, errors.New("compressed Echo-Request has an invalid Length")
	}
	if string(packet[8:]) != compressedPPPData {
		return nil, errors.New("compressed Echo-Request carries another payload")
	}
	return packet, nil
}

// requireCompressedPPPEcho proves the exact malformed reception experiment and
// its useful response. Linux 7.3 pppol2tp_recv passes the compressed protocol
// bytes to PPP; this is permitted acceptance, not a claimed rejecting branch.
func requireCompressedPPPEcho(frames []pppFrame) error {
	negotiated, err := negotiatedPPPFraming(frames)
	if err != nil {
		return err
	}
	injected := 0
	replied := false
	for _, frame := range frames {
		if frame.direction == relayCompressedPPP {
			injected++
			if len(frame.packet) < 8 {
				return errors.New("short compressed Echo-Request")
			}
			if binary.BigEndian.Uint32(frame.packet[4:8]) != negotiated.peerMagic {
				return errors.New("compressed Echo-Request did not carry the acknowledged peer Magic-Number")
			}
			continue
		}
		if frame.direction != relayFromZe {
			continue
		}
		if frame.protocol != protocolLCP {
			continue
		}
		if len(frame.packet) < 2 {
			continue
		}
		if frame.packet[0] != codeEchoReply {
			continue
		}
		if frame.packet[1] != compressedPPPIdentifier {
			continue
		}
		if injected != 1 {
			return errors.New("correlated Echo-Reply preceded the compressed request")
		}
		if len(frame.packet) != 8+len(compressedPPPData) {
			return errors.New("compressed request's Echo-Reply has the wrong length")
		}
		if int(binary.BigEndian.Uint16(frame.packet[2:])) != len(frame.packet) {
			return errors.New("compressed request's Echo-Reply has an invalid Length")
		}
		if binary.BigEndian.Uint32(frame.packet[4:8]) != negotiated.zeMagic {
			return errors.New("Echo-Reply did not carry Ze's acknowledged Magic-Number")
		}
		if string(frame.packet[8:]) != compressedPPPData {
			return errors.New("Echo-Reply did not preserve the request Data")
		}
		replied = true
	}
	if injected != 1 {
		return fmt.Errorf("capture holds %d compressed probes, want one", injected)
	}
	if !replied {
		return errors.New("no correlated uncompressed Echo-Reply from Ze")
	}
	return nil
}

// requirePPPFullSessionFraming requires useful witnesses for each phase, so an
// empty or late capture cannot satisfy a universal Address/Control assertion.
// RFC 1661 Section 6.6: "The Address and Control fields MUST NOT be compressed
// when sending any LCP packet."
func requirePPPFullSessionFraming(text string) error {
	frames, err := framingPPPFrames(text)
	if err != nil {
		return err
	}
	if _, err := negotiatedPPPFraming(frames); err != nil {
		return err
	}
	if len(zeCodeRejects(frames, protocolLCP)) != 2 {
		return errors.New("full capture lacks Ze's two LCP Code-Reject witnesses")
	}
	if len(zeCodeRejects(frames, protocolIPCP)) != 2 {
		return errors.New("full capture lacks Ze's two IPCP Code-Reject witnesses")
	}
	if err := requireCompressedPPPEcho(frames); err != nil {
		return err
	}
	for _, frame := range frames {
		if frame.direction == relayFromZe && frame.protocol == 0x0021 {
			return nil
		}
	}
	return errors.New("full capture lacks Ze-to-LAC IP traffic")
}
