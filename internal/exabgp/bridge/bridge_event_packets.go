// Design: docs/architecture/exabgp-bridge.md -- a message as its packet: packets, consolidate, parsed
// Related: bridge_event.go -- ReadEvent, which reads the packet off the ze event
// Related: bridge_script_version.go -- Line, the parsed form these forms build on

package bridge

import (
	"bytes"
	"log/slog"
	"strconv"
)

// Packet is one message's bytes as ze's engine delivers them to a `full`
// subscriber: the message type and the body after the 19-octet header, in the
// hexadecimal ze writes (`raw.update` for an UPDATE, `raw.message` for the other
// kinds, docs/architecture/api/json-format.md). The header is not delivered and
// needs not be: RFC 4271 Section 4.1 fixes it as the all-ones Marker, the
// Length and the Type, and the body states the first and the event the last.
//
// The zero Packet is an event that carried no bytes. Its code is zero, which no
// BGP message type is (RFC 4271 Section 4.1 numbers them from 1), so Present
// answers from the code alone.
type Packet struct {
	code uint8
	body string
}

// Present reports whether the event carried its message's bytes.
func (p Packet) Present() bool { return p.code != 0 }

// packetCodes are the BGP message type codes of the kinds ze delivers bytes for
// (RFC 4271 Section 4.1: "1 - OPEN, 2 - UPDATE, 3 - NOTIFICATION, 4 -
// KEEPALIVE"; RFC 2918 Section 3: "Type: 5 - ROUTE-REFRESH").
var packetCodes = map[string]uint8{
	msgTypeOpen: 1, msgTypeUpdate: 2, msgTypeNotification: 3, msgTypeKeepalive: 4, msgTypeRefresh: 5,
}

// packetTypeNames are ExaBGP's names of a message type in the `type` of a
// packet line, Message.string (src/exabgp/bgp/message/message.py): a
// ROUTE-REFRESH is `route-refresh` there, where its parsed line says `refresh`.
var packetTypeNames = [...]string{1: "open", 2: "update", 3: "notification", 4: "keepalive", 5: "route-refresh"}

// packetBodyHexMax bounds the body a packet can carry: RFC 8654 Section 3
// raises the message maximum to 65535 octets, the 19-octet header included.
const packetBodyHexMax = 2 * (65535 - 19)

// readPacket reads the bytes a `full` event carries for one message kind, or
// the zero Packet when it carries none. A member that is not even-length
// hexadecimal within the message maximum is not the engine's, and it is logged
// rather than written to a script as a packet.
func readPacket(payload map[string]any, kind string) Packet {
	code, isMessage := packetCodes[kind]
	if !isMessage {
		return Packet{}
	}
	raw, ok := payload["raw"].(map[string]any)
	if !ok {
		return Packet{}
	}
	member := "message"
	if kind == msgTypeUpdate {
		member = msgTypeUpdate
	}
	body, ok := raw[member].(string)
	if !ok {
		return Packet{}
	}
	if !packetHex(body) {
		bridgeLog().Warn("exabgp-bridge: the event's raw bytes are not a message body; no packet is written",
			"kind", kind, "length", len(body))
		return Packet{}
	}
	return Packet{code: code, body: body}
}

// packetHex reports whether text is even-length hexadecimal within the message
// maximum.
func packetHex(text string) bool {
	if len(text)%2 != 0 {
		return false
	}
	if len(text) > packetBodyHexMax {
		return false
	}
	for i := range len(text) {
		c := text[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c >= 'a' && c <= 'f' {
			continue
		}
		if c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}
	return true
}

// appendPacketHeader writes the 19-octet header as ExaBGP's hexstring writes it,
// `0x` and uppercase digits (src/exabgp/util/__init__.py).
//
// RFC 4271 Section 4.1: "Marker: This 16-octet field is included for
// compatibility; it MUST be set to all ones." "Length: This 2-octet unsigned
// integer indicates the total length of the message, including the header in
// octets."
func (p Packet) appendHeader(buf []byte) []byte {
	buf = append(buf, "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"...)
	length := 19 + len(p.body)/2
	buf = appendHexByte(buf, uint8(length>>8))
	buf = appendHexByte(buf, uint8(length))
	return appendHexByte(buf, p.code)
}

// appendBody writes the body as ExaBGP's hexstring writes it.
func (p Packet) appendBody(buf []byte) []byte {
	buf = append(buf, "0x"...)
	for i := range len(p.body) {
		c := p.body[i]
		if c >= 'a' && c <= 'f' {
			c -= 'a' - 'A'
		}
		buf = append(buf, c)
	}
	return buf
}

// headerText is the header in ExaBGP's form, for a JSON member.
func (p Packet) headerText() string {
	var scratch [48]byte
	return string(p.appendHeader(scratch[:0]))
}

// bodyText is the body in ExaBGP's form, for a JSON member.
func (p Packet) bodyText() string {
	return string(p.appendBody(make([]byte, 0, 2+len(p.body))))
}

// Forms is the set of forms a script is written a message in, for one
// direction, as its api block asked with `parsed`, `packets` and `consolidate`.
//
// The empty set writes nothing, which is an answer: ExaBGP tells a process that
// asked for no form nothing (FormsFor says when that happens).
type Forms uint8

const (
	// FormParsed is the decoded line, the one every other event has.
	FormParsed Forms = 1 << iota
	// FormPackets is the packet line: the message's type, header and body,
	// with the session's negotiated block (Response.JSON.packets).
	FormPackets
	// FormConsolidated is the decoded line carrying the header and the body.
	FormConsolidated

	// FormsCount is the number of distinct Forms values, for a cache indexed
	// by them.
	FormsCount = 1 << 3
)

// FormsFor answers the forms of one direction from the words a script's api
// block granted, `granted("receive-packets")` and its siblings.
//
// ExaBGP decides a received message in Protocol._tell_api_received and the
// packet-first read, a sent one in Protocol._to_api (src/exabgp/reactor/
// protocol.py at the pin): `consolidate` writes the decoded line with the header
// and body, which a sent message only carries when `packets` is granted too;
// otherwise `packets` writes the packet line and `parsed` the decoded line, both
// when both are granted, the packet first.
//
// A direction that names none of the three words is written the decoded line.
// ExaBGP writes such a process nothing, but a ze feed written by hand names the
// messages alone, and the migration writes every word the block had, so a
// migrated block that asked for a form is held to it.
func FormsFor(direction string, granted func(apiKey string) bool) Forms {
	consolidate := granted(direction + "-consolidate")
	packets := granted(direction + "-packets")
	parsed := granted(direction + "-parsed")
	if consolidate {
		if direction == modeReceive {
			return FormConsolidated
		}
		if packets {
			return FormConsolidated
		}
		return FormParsed
	}
	var forms Forms
	if packets {
		forms |= FormPackets
	}
	if parsed {
		forms |= FormParsed
	}
	if forms == 0 {
		return FormParsed
	}
	return forms
}

// GrantsPacket answers whether an api block's words write some message as its
// bytes, in either direction: FormsFor answers FormPackets or FormConsolidated
// for it. Only then does a runner need the engine's `full` events, which carry
// each message's bytes as hex beside its parsed form (readPacket); every other
// grant is written from the parsed form alone, so the engine is spared the hex
// of every UPDATE (ai/rules/performance.md).
func GrantsPacket(granted func(apiKey string) bool) bool {
	for _, direction := range []string{modeReceive, modeSend} {
		if FormsFor(direction, granted)&(FormPackets|FormConsolidated) != 0 {
			return true
		}
	}
	return false
}

// Lines answers the event as the lines a script reading format is written in
// the given forms, newlines included, or nil when it is written nothing.
//
// The forms are the forms of a MESSAGE. Every other event (a state, the
// negotiated block, an FSM change, a signal) has one form, and is written its
// Line whatever the forms say, as ExaBGP writes those apart from the message
// path.
func (e Event) Lines(format Format, forms Forms, log *slog.Logger) []byte {
	if _, isMessage := packetCodes[e.Kind]; !isMessage {
		return e.Line(format, log)
	}
	var out []byte
	if forms&FormPackets != 0 {
		out = append(out, e.packetLine(format, log)...)
	}
	if forms&FormConsolidated != 0 {
		out = append(out, e.consolidatedLine(format, log)...)
	}
	if forms&FormParsed != 0 {
		out = append(out, e.Line(format, log)...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// packetLine writes Response.packets: in text `neighbor <peer> <direction>
// <type code> header <hex> body <hex>`, the body left out when it is empty
// (Text._header_body); in JSON the `message` object of `category`, `header`
// and `body`, and the negotiated block.
func (e Event) packetLine(format Format, log *slog.Logger) []byte {
	if !e.Packet.Present() {
		log.Warn("exabgp-bridge: the event carries no bytes, so no packet line is written", "event", e.Kind)
		return nil
	}
	switch format {
	case FormatV4Text:
		buf := e.appendPrefix(nil, true)
		buf = append(buf, ' ')
		buf = strconv.AppendUint(buf, uint64(e.Packet.code), 10)
		buf = e.Packet.appendHeaderBodyText(buf)
		return append(buf, '\n')
	case FormatV4JSON, FormatV6JSON:
		result, neighbor := e.exabgpEnvelope(format)
		result["type"] = packetTypeNames[e.Packet.code]
		result["neighbor"] = neighbor
		neighbor["message"] = map[string]any{
			"category": int(e.Packet.code),
			"header":   e.Packet.headerText(),
			"body":     e.Packet.bodyText(),
		}
		if e.Negotiated != nil {
			neighbor[msgTypeNegotiated] = e.Negotiated
		}
		return marshalLine(result, log)
	case FormatUnspecified:
		panic("BUG: bridge format unspecified")
	default:
		panic("BUG: invalid bridge format")
	}
}

// consolidatedLine writes the decoded line with the header and the body: in
// JSON as the `header` and `body` members of the line, the body left out when
// it is empty (Response.JSON._header); in text after the line, and for an
// UPDATE as a line of its own before `update end` (Text.update).
//
// An event that carries no bytes is written its decoded line alone, which is
// what ExaBGP writes when it hands the encoder an empty header and body.
func (e Event) consolidatedLine(format Format, log *slog.Logger) []byte {
	if !e.Packet.Present() {
		return e.Line(format, log)
	}
	switch format {
	case FormatV4Text:
		line := e.Line(format, log)
		if len(line) == 0 {
			return nil
		}
		line = line[:len(line)-1] // every text line ends in '\n'
		if e.Kind != msgTypeUpdate {
			line = e.Packet.appendHeaderBodyText(line)
			return append(line, '\n')
		}
		end := bytes.LastIndexByte(line, '\n') + 1
		last := string(line[end:])
		line = e.Packet.appendHeaderBodyText(line[:end])
		line = append(line, '\n')
		line = append(line, last...)
		return append(line, '\n')
	case FormatV4JSON, FormatV6JSON:
		document := e.exabgpJSON(format)
		document["header"] = e.Packet.headerText()
		if e.Packet.body != "" {
			document["body"] = e.Packet.bodyText()
		}
		return marshalLine(document, log)
	case FormatUnspecified:
		panic("BUG: bridge format unspecified")
	default:
		panic("BUG: invalid bridge format")
	}
}

// appendHeaderBodyText writes Text._header_body: ` header <hex>`, then
// ` body <hex>` unless the body is empty.
func (p Packet) appendHeaderBodyText(buf []byte) []byte {
	buf = append(buf, " header "...)
	buf = p.appendHeader(buf)
	if p.body == "" {
		return buf
	}
	buf = append(buf, " body "...)
	return p.appendBody(buf)
}

// marshalLine serializes one ExaBGP JSON document as a line, or logs and
// answers nil when it does not marshal.
func marshalLine(document map[string]any, log *slog.Logger) []byte {
	encoded, err := marshalExabgpJSON(document)
	if err != nil {
		log.Warn("exabgp-bridge: marshal external JSON failed", "error", err)
		return nil
	}
	return append(encoded, '\n')
}
