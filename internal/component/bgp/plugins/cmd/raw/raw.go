// Design: docs/architecture/api/commands.md — BGP raw message handlers
// Overview: doc.go — bgp-cmd-raw plugin registration

package raw

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errRawTakesAnEncodingAndData = errors.New("usage: raw <encoding> <data> [type <type>]")
	errRawTypeNeedsAValue        = errors.New("the type keyword needs a BGP message type after it")
)

// rawTypeKeyword introduces the BGP message type. The model declares it as the
// leaf `type` on the raw container (yang/ze-raw-cmd.yang).
//
// A value the command runs without takes a keyword in front of it
// (ai/rules/cli.md). The dispatcher and rawArguments read that word from here,
// so the two cannot disagree about it.
const rawTypeKeyword = "type"

// rawFullPacket is the message type that means "ze writes no header". The data
// then carries the marker, the length and the type itself.
//
// It is a named sentinel rather than a bare zero for two reasons. The reactor
// BRANCHES on it (Session.SendRawMessage, bgp/reactor/session_write.go). And no
// BGP message type carries the value 0, so it collides with nothing.
const rawFullPacket uint8 = 0

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-raw", Handler: handleRaw, RequiresSelector: true},
	)
}

// handleRaw sends operator-supplied bytes to one peer, with no validation.
//
// The grammar is the model's, not this comment's. yang/ze-raw-cmd.yang declares
// the encoding, the data and the optional message type. The dispatcher then
// refuses a word outside either enumeration before this handler runs.
//
//	send bgp <selector> raw <hex|b64> <data> [type <type>]
//
// With a type keyword ze writes the marker and the header, and the data carries
// the message body alone. Without one the data carries the whole packet.
func handleRaw(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	// Resolve the selector to one peer. Accepts an address OR a configured peer
	// name (and any other selector form); the wildcard is refused, since raw
	// injects bytes into a single session.
	peerAddr, errResp, err := pluginserver.ResolveSinglePeer(ctx, "raw")
	if err != nil {
		return errResp, err
	}

	msgType, encoding, data, err := rawArguments(args)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  err.Error(),
		}, err
	}

	// Decode payload
	payload, err := decodePayload(encoding, data)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("decode error: %v", err),
		}, err
	}

	// Send to reactor (BGP-specific: raw message injection)
	r, errResp2, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp2, bgpErr
	}
	// ctx.Sender is the authority. A process must be attached to this peer, which
	// is the whole permission raw asks for: the payload is a message of the
	// caller's choosing, so the `send` list has no word for it
	// (bgp/reactor/send_permission.go, rawOrigin). An operator is not gated.
	if err := r.SendRawMessage(peerAddr, msgType, payload, ctx.Sender); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("send error: %v", err),
		}, err
	}

	respData := map[string]any{
		"peer":  ctx.Peer,
		"bytes": len(payload),
	}
	if msgType != rawFullPacket {
		respData["type"] = msgTypeName(msgType)
	} else {
		respData["mode"] = "full-packet"
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map(respData),
	}, nil
}

// rawArguments reads the encoding, the data and the optional message type out
// of the tail the dispatcher handed over.
//
// The message type arrives as its keyword and its value, so it can stand
// anywhere in the tail. The two remaining tokens are the encoding and the data,
// in that order. The loop is bounded by the token list the dispatcher built from
// one command line.
//
// The dispatcher has already refused a word outside either enumeration
// (yang/ze-raw-cmd.yang), so the refusals below are the second of a pair of
// checks. They stay because a value that reaches a handler through any other
// route must still be refused rather than read as an encoding.
func rawArguments(args []string) (uint8, string, string, error) {
	msgType := rawFullPacket
	var encoding, data string
	supplied := 0

	for index := 0; index < len(args); index++ {
		if args[index] != rawTypeKeyword {
			switch supplied {
			case 0:
				encoding = args[index]
			case 1:
				data = args[index]
			default:
				return 0, "", "", errRawTakesAnEncodingAndData
			}
			supplied++
			continue
		}

		index++
		if index >= len(args) {
			return 0, "", "", errRawTypeNeedsAValue
		}
		named, ok := parseMessageType(args[index])
		if !ok {
			return 0, "", "", fmt.Errorf("unknown BGP message type %q (valid: open, update, notification, keepalive, route-refresh)", args[index])
		}
		msgType = named
	}

	if supplied != 2 {
		return 0, "", "", errRawTakesAnEncodingAndData
	}
	return msgType, encoding, data, nil
}

// parseMessageType converts string to BGP message type.
// Returns (type, true) if valid, (0, false) if not a type.
func parseMessageType(s string) (uint8, bool) {
	switch strings.ToLower(s) {
	case "open":
		return uint8(msgtype.TypeOPEN), true
	case bgpevents.EventUpdate:
		return uint8(msgtype.TypeUPDATE), true
	case "notification":
		return uint8(msgtype.TypeNOTIFICATION), true
	case "keepalive":
		return uint8(msgtype.TypeKEEPALIVE), true
	case "route-refresh":
		return uint8(msgtype.TypeROUTEREFRESH), true
	default: // not a recognized message type name
		return 0, false
	}
}

// msgTypeName returns human-readable name for message type.
func msgTypeName(t uint8) string {
	switch msgtype.MessageType(t) {
	case msgtype.TypeOPEN:
		return "open"
	case msgtype.TypeUPDATE:
		return "update"
	case msgtype.TypeNOTIFICATION:
		return "notification"
	case msgtype.TypeKEEPALIVE:
		return "keepalive"
	case msgtype.TypeROUTEREFRESH:
		return "route-refresh"
	default: // numeric fallback for unknown types
		return textbuf.StrInt("type-", int64(t))
	}
}

// decodePayload decodes wire bytes from the specified encoding.
func decodePayload(encoding, data string) ([]byte, error) {
	// No operator reaches this branch: the data leaf is mandatory and tokenize
	// keeps no empty token, so a caller through the CLI always supplies octets.
	// It stays for a caller that builds the pair itself, because zero octets is
	// a valid payload for a message ze frames with a header.
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
