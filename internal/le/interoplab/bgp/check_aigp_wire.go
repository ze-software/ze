// Design: docs/architecture/testing/interop.md -- AIGP wire and startup fences.
package bgp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func releaseSynchronizedAIGPSource(ctx context.Context, check *interoplab.CheckContext) error {
	for _, host := range []uint8{3, 5, 9, 10} {
		address := networkHostAddress(check.Network, host)
		command := zeCommand("show bgp peer " + address + " detail")
		last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
			Timeout: 45 * time.Second, Interval: time.Second, Description: "AIGP initial EOR for " + address,
		}, func(probe context.Context) (string, error) {
			return check.Lab.Query(probe, "ze", command, queryEnvironment("ze", command))
		}, func(output string) bool { return aigpPeerSynchronized(output, address) })
		if err != nil {
			return withLastOutput(err, last)
		}
	}
	command := zeCommand(aigpReleaseCommand + " " + networkHostAddress(check.Network, 9) + " " + networkHostAddress(check.Network, 2))
	answer, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return err
	}
	var receipt struct {
		Announced *int `json:"announced"`
		Withdrawn *int `json:"withdrawn"`
	}
	if err := json.Unmarshal([]byte(answer), &receipt); err != nil {
		return err
	}
	if receipt.Announced == nil || receipt.Withdrawn == nil || *receipt.Announced != 1 || *receipt.Withdrawn != 0 {
		return fmt.Errorf("invalid AIGP source readiness receipt: %s", answer)
	}
	return nil
}

func aigpPeerSynchronized(output, address string) bool {
	var detail struct {
		Peers map[string]struct {
			State       string  `json:"state"`
			EOR         uint64  `json:"eor-sent"`
			Established uint64  `json:"connections-established"`
			Dropped     *uint64 `json:"connections-dropped"`
		} `json:"peers"`
	}
	if json.Unmarshal([]byte(output), &detail) != nil {
		return false
	}
	peer := detail.Peers[address]
	return strings.EqualFold(peer.State, "established") && peer.EOR > 0 && peer.Established > 0 && peer.Dropped != nil
}

func waitAIGPWire(ctx context.Context, check *interoplab.CheckContext, transitions int) error {
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "exact AIGP recipient wire transition",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerSpeaker, []string{"cat", aigpWireCapture}, nil)
	}, func(output string) bool {
		return requireAIGPWire(output, networkHostAddress(check.Network, 2), transitions) == nil
	})
	return withLastOutput(err, last)
}

// requireAIGPWire checks the same connection's full history. No table-only
// observation substitutes for the exact withdrawal, metric TLV and next hop.
// The five recovery states are unknown, 11, 7, zero, 7; direct cost stays 7.
func requireAIGPWire(capture, nextHop string, transitions int) error {
	if transitions < 1 || transitions > 5 {
		return errors.New("invalid AIGP wire transition count")
	}
	hop, err := netip.ParseAddr(nextHop)
	if err != nil || !hop.Is4() {
		return errors.New("invalid AIGP wire next hop")
	}
	wantHop := hop.As4()
	decoder := json.NewDecoder(strings.NewReader(capture))
	opened, eor, direct, fence := false, false, false, false
	observed := 0
	wantMetrics := [...]int64{-1, 111, 107, -1, 107}
	for {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil || len(frame) < 19 {
			return errors.New("invalid captured BGP frame")
		}
		if row.Delivered != "" || int(binary.BigEndian.Uint16(frame[16:18])) != len(frame) {
			return errors.New("captured BGP frame was altered or truncated")
		}
		for _, octet := range frame[:16] {
			if octet != 255 {
				return errors.New("captured BGP frame has an invalid marker")
			}
		}
		switch frame[18] {
		case bgpOpen:
			if opened {
				return errors.New("wire recipient session was replaced")
			}
			opened = true
		case bgpKeepalive:
		case bgpUpdate:
			body := frame[19:]
			if bytes.Equal(body, []byte{0, 0, 0, 0}) {
				eor = true
				continue
			}
			if len(body) < 4 {
				return errors.New("short captured UPDATE")
			}
			withdrawn := int(binary.BigEndian.Uint16(body))
			if withdrawn > len(body)-4 {
				return errors.New("invalid captured withdrawal length")
			}
			start := 4 + withdrawn
			end := start + int(binary.BigEndian.Uint16(body[2+withdrawn:]))
			if end > len(body) {
				return errors.New("invalid captured attribute length")
			}
			nlri := body[end:]
			announcedSubjects, err := aigpWireSubjects(nlri)
			if err != nil {
				return err
			}
			withdrawnSubjects, err := aigpWireSubjects(body[2 : 2+withdrawn])
			if err != nil {
				return err
			}
			if withdrawnSubjects&1 != 0 {
				return errors.New("configured source-cost route was withdrawn")
			}
			if (announcedSubjects != 0 && len(nlri) != 4) || (withdrawnSubjects != 0 && withdrawn != 4) {
				return errors.New("AIGP subjects were not received as separate exact routes")
			}
			isRecovery := bytes.Equal(nlri, []byte{24, 10, 10, 3})
			isDirect := bytes.Equal(nlri, []byte{24, 10, 10, 2})
			isWithdrawal := bytes.Equal(body[2:2+withdrawn], []byte{24, 10, 10, 3})
			if bytes.Equal(nlri, []byte{24, 10, 10, 4}) {
				fence = true
			}
			if !isRecovery && !isDirect && !isWithdrawal {
				continue
			}
			if !opened || !eor {
				return errors.New("AIGP subject preceded initial synchronization")
			}
			if isDirect {
				if err := requireAIGPWireAttributes(body[start:end], wantHop, 107); err != nil {
					return err
				}
				direct = true
				continue
			}
			if !direct || observed >= transitions {
				return errors.New("unexpected or reordered AIGP recovery transition")
			}
			metric := wantMetrics[observed]
			if metric < 0 {
				if !bytes.Equal(body, []byte{0, 4, 24, 10, 10, 3, 0, 0}) {
					return errors.New("AIGP cost refusal is not the exact whole-route withdrawal")
				}
			} else if !isRecovery || withdrawn != 0 {
				return errors.New("missing AIGP recovery announcement")
			} else if err := requireAIGPWireAttributes(body[start:end], wantHop, uint64(metric)); err != nil {
				return err
			}
			observed++
		default:
			return fmt.Errorf("unexpected received BGP message %d", frame[18])
		}
	}
	if !direct || !fence || observed != transitions {
		return fmt.Errorf("incomplete AIGP wire proof: direct=%t fence=%t transitions=%d/%d", direct, fence, observed, transitions)
	}
	return nil
}

func requireAIGPWireAttributes(attrs []byte, hop [4]byte, metric uint64) error {
	var wantMetric [11]byte
	wantMetric[0], wantMetric[2] = 1, 11
	binary.BigEndian.PutUint64(wantMetric[3:], metric)
	foundHop, foundMetric := false, false
	for len(attrs) != 0 {
		if len(attrs) < 3 {
			return errors.New("truncated wire attribute header")
		}
		flags, code, header, length := attrs[0], attrs[1], 3, int(attrs[2])
		if flags&16 != 0 {
			if len(attrs) < 4 {
				return errors.New("truncated extended wire attribute header")
			}
			header, length = 4, int(binary.BigEndian.Uint16(attrs[2:4]))
		}
		if length > len(attrs)-header {
			return errors.New("truncated wire attribute value")
		}
		value := attrs[header : header+length]
		switch code {
		case 3:
			if foundHop || flags != 0x40 || !bytes.Equal(value, hop[:]) {
				return errors.New("wire NEXT_HOP is not the exact self address")
			}
			foundHop = true
		case 26:
			if foundMetric || flags != 0x80 || !bytes.Equal(value, wantMetric[:]) {
				return fmt.Errorf("wire AIGP is not the exact metric%d TLV", metric)
			}
			foundMetric = true
		}
		attrs = attrs[header+length:]
	}
	if !foundHop || !foundMetric {
		return errors.New("wire announcement lacks NEXT_HOP or AIGP")
	}
	return nil
}

func aigpWireSubjects(nlri []byte) (uint8, error) {
	var subjects uint8
	for len(nlri) != 0 {
		bits := int(nlri[0])
		length := 1 + (bits+7)/8
		if bits > 32 || length > len(nlri) {
			return 0, errors.New("invalid IPv4 wire NLRI")
		}
		if bytes.Equal(nlri[:length], []byte{24, 10, 10, 2}) {
			subjects |= 1
		}
		if bytes.Equal(nlri[:length], []byte{24, 10, 10, 3}) {
			subjects |= 2
		}
		nlri = nlri[length:]
	}
	return subjects, nil
}
