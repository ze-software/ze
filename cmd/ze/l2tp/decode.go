// Design: docs/architecture/core-design.md -- l2tp offline decode
//
// Offline L2TPv2 message decoder. Reads a hex blob from stdin (ASCII hex,
// any whitespace or newlines allowed), parses via internal/component/l2tp,
// and emits JSON on stdout.

package l2tp

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	l2tpwire "codeberg.org/thomas-mangin/ze/internal/component/l2tp"
)

// maxStdinBytes caps the size of the hex input read from stdin. A realistic
// L2TPv2 control message is a few hundred bytes; the hex encoding doubles
// that, plus whitespace. 256 KiB is ample for any legitimate input and bounds
// allocation so a malformed pipe cannot exhaust memory.
const maxStdinBytes = 256 * 1024

// maxAVPs caps the number of AVPs the decoder will collect. With AVPHeaderLen
// = 6 and header Length <= 65535, a single message has at most ~10900 AVPs;
// this is a defensive ceiling slightly above that.
const maxAVPs = 16384

type decodedMessage struct {
	Header decodedHeader `json:"header"`
	AVPs   []decodedAVP  `json:"avps"`
}

type decodedHeader struct {
	IsControl   bool   `json:"is-control"`
	HasLength   bool   `json:"has-length"`
	HasSequence bool   `json:"has-sequence"`
	HasOffset   bool   `json:"has-offset"`
	Priority    bool   `json:"priority"`
	Version     uint8  `json:"version"`
	Length      uint16 `json:"length,omitempty"`
	TunnelID    uint16 `json:"tunnel-id"`
	SessionID   uint16 `json:"session-id"`
	Ns          uint16 `json:"ns,omitempty"`
	Nr          uint16 `json:"nr,omitempty"`
	OffsetSize  uint16 `json:"offset-size,omitempty"`
}

type decodedAVP struct {
	VendorID  uint16 `json:"vendor-id"`
	Type      uint16 `json:"type"`
	Name      string `json:"name,omitempty"`
	Mandatory bool   `json:"mandatory"`
	Hidden    bool   `json:"hidden"`
	Reserved  bool   `json:"reserved,omitempty"`
	Value     string `json:"value"` // lowercase hex
}

func cmdDecode(args []string) int {
	fs := flag.NewFlagSet("decode", flag.ContinueOnError)
	pretty := fs.Bool("pretty", false, "indent JSON output")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ze l2tp decode [--pretty] < hex")
		fmt.Fprintln(os.Stderr, "  reads hex L2TPv2 control message from stdin, emits JSON on stdout")
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Bound stdin so an unbounded pipe cannot exhaust memory; L2TP messages
	// are measured in hundreds of bytes, the hex encoding in thousands.
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinBytes+1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read stdin: %v\n", err)
		return 1
	}
	if len(raw) > maxStdinBytes {
		fmt.Fprintf(os.Stderr, "error: input exceeds %d bytes (max); likely not a hex L2TPv2 message\n", maxStdinBytes)
		return 1
	}
	clean := stripWhitespace(string(raw))
	wire, err := hex.DecodeString(clean)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: decode hex: %v\n", err)
		return 1
	}

	h, err := l2tpwire.ParseMessageHeader(wire)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse header: %v\n", err)
		return 1
	}

	out := decodedMessage{
		Header: decodedHeader{
			IsControl:   h.IsControl,
			HasLength:   h.HasLength,
			HasSequence: h.HasSequence,
			HasOffset:   h.HasOffset,
			Priority:    h.Priority,
			Version:     h.Version,
			Length:      h.Length,
			TunnelID:    h.TunnelID,
			SessionID:   h.SessionID,
			Ns:          h.Ns,
			Nr:          h.Nr,
			OffsetSize:  h.OffsetSize,
		},
	}

	// For control messages the payload ends at Length; for data messages we
	// assume the slice is trimmed by the caller.
	end := len(wire)
	if h.HasLength {
		end = int(h.Length)
	}
	it := l2tpwire.NewAVPIterator(wire[h.PayloadOff:end])
	for {
		vendor, at, flags, val, ok := it.Next()
		if !ok {
			break
		}
		if len(out.AVPs) >= maxAVPs {
			fmt.Fprintf(os.Stderr, "error: AVP count exceeds %d (malformed input)\n", maxAVPs)
			return 1
		}
		out.AVPs = append(out.AVPs, decodedAVP{
			VendorID:  vendor,
			Type:      uint16(at),
			Name:      avpName(vendor, at),
			Mandatory: flags&l2tpwire.FlagMandatory != 0,
			Hidden:    flags&l2tpwire.FlagHidden != 0,
			Reserved:  flags&l2tpwire.FlagReserved != 0,
			Value:     hex.EncodeToString(val),
		})
	}
	if err := it.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error: iterate AVPs: %v\n", err)
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	if *pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "error: encode json: %v\n", err)
		return 1
	}
	return 0
}

func stripWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// avpName returns the RFC 2661 catalog name for a vendor-0 AVP, or "" for
// unknown / vendor-specific AVPs.
func avpName(vendor uint16, at l2tpwire.AVPType) string {
	if vendor != 0 {
		return ""
	}
	switch at {
	case l2tpwire.AVPMessageType:
		return "message-type"
	case l2tpwire.AVPResultCode:
		return "result-code"
	case l2tpwire.AVPProtocolVersion:
		return "protocol-version"
	case l2tpwire.AVPFramingCapabilities:
		return "framing-capabilities"
	case l2tpwire.AVPBearerCapabilities:
		return "bearer-capabilities"
	case l2tpwire.AVPTieBreaker:
		return "tie-breaker"
	case l2tpwire.AVPFirmwareRevision:
		return "firmware-revision"
	case l2tpwire.AVPHostName:
		return "host-name"
	case l2tpwire.AVPVendorName:
		return "vendor-name"
	case l2tpwire.AVPAssignedTunnelID:
		return "assigned-tunnel-id"
	case l2tpwire.AVPReceiveWindowSize:
		return "receive-window-size"
	case l2tpwire.AVPChallenge:
		return "challenge"
	case l2tpwire.AVPQ931CauseCode:
		return "q931-cause-code"
	case l2tpwire.AVPChallengeResponse:
		return "challenge-response"
	case l2tpwire.AVPAssignedSessionID:
		return "assigned-session-id"
	case l2tpwire.AVPCallSerialNumber:
		return "call-serial-number"
	case l2tpwire.AVPMinimumBPS:
		return "minimum-bps"
	case l2tpwire.AVPMaximumBPS:
		return "maximum-bps"
	case l2tpwire.AVPBearerType:
		return "bearer-type"
	case l2tpwire.AVPFramingType:
		return "framing-type"
	case l2tpwire.AVPCalledNumber:
		return "called-number"
	case l2tpwire.AVPCallingNumber:
		return "calling-number"
	case l2tpwire.AVPSubAddress:
		return "sub-address"
	case l2tpwire.AVPTxConnectSpeed:
		return "tx-connect-speed"
	case l2tpwire.AVPPhysicalChannelID:
		return "physical-channel-id"
	case l2tpwire.AVPInitialReceivedLCPConfReq:
		return "initial-received-lcp-confreq"
	case l2tpwire.AVPLastSentLCPConfReq:
		return "last-sent-lcp-confreq"
	case l2tpwire.AVPLastReceivedLCPConfReq:
		return "last-received-lcp-confreq"
	case l2tpwire.AVPProxyAuthenType:
		return "proxy-authen-type"
	case l2tpwire.AVPProxyAuthenName:
		return "proxy-authen-name"
	case l2tpwire.AVPProxyAuthenChallenge:
		return "proxy-authen-challenge"
	case l2tpwire.AVPProxyAuthenID:
		return "proxy-authen-id"
	case l2tpwire.AVPProxyAuthenResponse:
		return "proxy-authen-response"
	case l2tpwire.AVPCallErrors:
		return "call-errors"
	case l2tpwire.AVPACCM:
		return "accm"
	case l2tpwire.AVPRandomVector:
		return "random-vector"
	case l2tpwire.AVPPrivateGroupID:
		return "private-group-id"
	case l2tpwire.AVPRxConnectSpeed:
		return "rx-connect-speed"
	case l2tpwire.AVPSequencingRequired:
		return "sequencing-required"
	}
	return ""
}
