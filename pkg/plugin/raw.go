package plugin

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
)

// RegisterRawHandlers registers raw passthrough commands.
func RegisterRawHandlers(d *Dispatcher) {
	d.Register("raw", handleRaw, "Send raw bytes to peer (no validation)")
}

// handleRaw sends raw bytes to a peer without validation.
// Syntax:
//   - raw <type> <encoding> <data>  - message payload (ZeBGP adds header)
//   - raw <encoding> <data>         - full packet (user provides marker+header)
//
// Types: open, update, notification, keepalive, route-refresh
// Encodings: hex, b64
// Data: encoded bytes (empty allowed for keepalive).
func handleRaw(ctx *CommandContext, args []string) (*Response, error) {
	// Require peer selector
	if ctx.Peer == "" || ctx.Peer == "*" {
		return &Response{
			Status: statusError,
			Data:   "raw requires specific peer: peer <addr> raw ...",
		}, fmt.Errorf("raw requires specific peer")
	}

	peerAddr, err := netip.ParseAddr(ctx.Peer)
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid peer address: %s", ctx.Peer),
		}, fmt.Errorf("invalid peer address: %w", err)
	}

	if len(args) < 2 {
		return &Response{
			Status: statusError,
			Data:   "usage: raw [<type>] <encoding> <data>",
		}, fmt.Errorf("raw requires at least encoding and data")
	}

	// Parse arguments: [type] encoding data
	var msgType uint8
	var encoding, data string

	// Check if first arg is a message type
	if mt, ok := parseMessageType(args[0]); ok {
		// raw <type> <encoding> <data>
		msgType = mt
		if len(args) < 2 {
			return &Response{
				Status: statusError,
				Data:   "usage: raw <type> <encoding> <data>",
			}, fmt.Errorf("missing encoding after type")
		}
		encoding = args[1]
		if len(args) > 2 {
			data = args[2]
		}
	} else {
		// raw <encoding> <data> - full packet mode
		msgType = 0 // 0 = full packet
		encoding = args[0]
		if len(args) > 1 {
			data = args[1]
		}
	}

	// Decode payload
	payload, err := decodePayload(encoding, data)
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("decode error: %v", err),
		}, err
	}

	// Send to reactor
	if err := ctx.Reactor.SendRawMessage(peerAddr, msgType, payload); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("send error: %v", err),
		}, err
	}

	respData := map[string]any{
		"peer":  ctx.Peer,
		"bytes": len(payload),
	}
	if msgType != 0 {
		respData["type"] = msgTypeName(msgType)
	} else {
		respData["mode"] = "full-packet"
	}

	return &Response{
		Status: statusDone,
		Data:   respData,
	}, nil
}

// parseMessageType converts string to BGP message type.
// Returns (type, true) if valid, (0, false) if not a type.
func parseMessageType(s string) (uint8, bool) {
	switch strings.ToLower(s) {
	case "open":
		return uint8(message.TypeOPEN), true
	case "update":
		return uint8(message.TypeUPDATE), true
	case "notification":
		return uint8(message.TypeNOTIFICATION), true
	case "keepalive":
		return uint8(message.TypeKEEPALIVE), true
	case "route-refresh":
		return uint8(message.TypeROUTEREFRESH), true
	default:
		return 0, false
	}
}

// msgTypeName returns human-readable name for message type.
func msgTypeName(t uint8) string {
	switch message.MessageType(t) {
	case message.TypeOPEN:
		return "open"
	case message.TypeUPDATE:
		return "update"
	case message.TypeNOTIFICATION:
		return "notification"
	case message.TypeKEEPALIVE:
		return "keepalive"
	case message.TypeROUTEREFRESH:
		return "route-refresh"
	default:
		return fmt.Sprintf("type-%d", t)
	}
}

// decodePayload decodes wire bytes from the specified encoding.
func decodePayload(encoding, data string) ([]byte, error) {
	// Empty data is valid (e.g., keepalive)
	if data == "" {
		return nil, nil
	}

	switch strings.ToLower(encoding) {
	case "hex":
		return hex.DecodeString(data)
	case "b64", "base64":
		return base64.StdEncoding.DecodeString(data)
	default:
		return nil, fmt.Errorf("unknown encoding: %q (valid: hex, b64)", encoding)
	}
}
