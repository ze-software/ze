// Design: docs/architecture/api/commands.md — BGP raw message handlers
// Overview: doc.go — bgp-cmd-raw plugin registration

package raw

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-raw", Handler: handleRaw, RequiresSelector: true},
	)
}

// handleRaw sends raw bytes to a peer without validation.
// Syntax:
//   - bgp peer <addr> raw <type> <encoding> <data>  - message payload (ze adds header)
//   - bgp peer <addr> raw <encoding> <data>         - full packet (user provides marker+header)
//
// Types: open, update, notification, keepalive, route-refresh
// Encodings: hex, b64
// Data: encoded bytes (empty allowed for keepalive).
func handleRaw(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	// Require peer selector
	if ctx.Peer == "" || ctx.Peer == "*" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "raw requires specific peer: bgp peer <addr> raw ...",
		}, fmt.Errorf("raw requires specific peer")
	}

	peerAddr, err := netip.ParseAddr(ctx.Peer)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid peer address: %s", ctx.Peer),
		}, fmt.Errorf("invalid peer address: %w", err)
	}

	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
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
			return &plugin.Response{
				Status: plugin.StatusError,
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
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("decode error: %v", err),
		}, err
	}

	// Send to reactor (BGP-specific: raw message injection)
	r, errResp2, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp2, bgpErr
	}
	if err := r.SendRawMessage(peerAddr, msgType, payload); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
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

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   respData,
	}, nil
}

// parseMessageType converts string to BGP message type.
// Returns (type, true) if valid, (0, false) if not a type.
func parseMessageType(s string) (uint8, bool) {
	switch strings.ToLower(s) {
	case "open":
		return uint8(message.TypeOPEN), true
	case events.EventUpdate:
		return uint8(message.TypeUPDATE), true
	case "notification":
		return uint8(message.TypeNOTIFICATION), true
	case "keepalive":
		return uint8(message.TypeKEEPALIVE), true
	case "route-refresh":
		return uint8(message.TypeROUTEREFRESH), true
	default: // not a recognized message type name
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
	default: // numeric fallback for unknown types
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
	default: // unknown encoding format — return explicit error
		return nil, fmt.Errorf("unknown encoding: %q (valid: hex, b64)", encoding)
	}
}
