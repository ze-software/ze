// Design: docs/architecture/api/json-format.md — non-UPDATE message formatting
// Related: text_update.go — UPDATE-path formatters that reuse peer/JSON helpers
// Related: peer_json.go — string-returning peer JSON + escape helpers for external callers
// Related: text_human.go — formatStateChangeText (text StateChange)
// Related: text_json.go — formatStateChangeJSON (JSON StateChange)
// Related: ../textparse/keywords.go — shared keyword constants and alias resolution
//
// Non-UPDATE message text serialization. All formatters append into a
// caller-provided []byte. No fmt.Sprintf, no strings.Builder, no
// strings.Join, no strings.Replacer; see `.claude/rules/buffer-first.md`.

package format

import (
	"encoding/hex"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// appendJSONString appends s to buf wrapped in JSON string escaping rules.
// Escapes: \ " and control characters (0x00-0x1F). Byte-identical output
// to the legacy writeJSONEscapedString + jsonSafeReplacer combo over the
// test corpus (empty, ASCII, control chars, embedded quotes/backslashes,
// multi-byte UTF-8 sequences).
func appendJSONString(buf []byte, s string) []byte {
	const hexDigits = "0123456789abcdef"
	for i := range len(s) {
		c := s[i]
		switch c {
		case '\\':
			buf = append(buf, '\\', '\\')
			continue
		case '"':
			buf = append(buf, '\\', '"')
			continue
		case '\n':
			buf = append(buf, '\\', 'n')
			continue
		case '\r':
			buf = append(buf, '\\', 'r')
			continue
		case '\t':
			buf = append(buf, '\\', 't')
			continue
		}
		if c < 0x20 {
			buf = append(buf, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0x0F])
			continue
		}
		buf = append(buf, c)
	}
	return buf
}

// appendReplacingByte appends s to buf replacing every occurrence of `from`
// with `to`. Intended for token-sanitizing user-visible strings (e.g.,
// NOTIFICATION error names "Administrative Shutdown" → "Administrative-Shutdown").
// Byte-identical to strings.ReplaceAll(s, string(from), string(to)) for
// ASCII inputs.
func appendReplacingByte(buf []byte, s string, from, to byte) []byte {
	for i := range len(s) {
		c := s[i]
		if c == from {
			buf = append(buf, to)
		} else {
			buf = append(buf, c)
		}
	}
	return buf
}

// appendPeerJSON appends the peer JSON object (no leading comma) to buf.
// Structure matches the YANG peer-info grouping: address, group, local,
// name, remote.as. Key order is alphabetical (matching json.Marshal output
// from peerMap).
func appendPeerJSON(buf []byte, peer *plugin.PeerInfo) []byte {
	buf = append(buf, `"peer":{"address":"`...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, '"')
	if peer.GroupName != "" {
		buf = append(buf, `,"group":"`...)
		buf = appendJSONString(buf, peer.GroupName)
		buf = append(buf, '"')
	}
	if peer.LocalAS > 0 || peer.LocalAddress.IsValid() {
		buf = append(buf, `,"local":{`...)
		first := true
		if peer.LocalAddress.IsValid() {
			buf = append(buf, `"address":"`...)
			buf = peer.LocalAddress.AppendTo(buf)
			buf = append(buf, '"')
			first = false
		}
		if peer.LocalAS > 0 {
			if !first {
				buf = append(buf, ',')
			}
			buf = append(buf, `"as":`...)
			buf = strconv.AppendUint(buf, uint64(peer.LocalAS), 10)
		}
		buf = append(buf, '}')
	}
	buf = append(buf, `,"name":"`...)
	buf = appendJSONString(buf, peer.Name)
	buf = append(buf, `","remote":{"as":`...)
	buf = strconv.AppendUint(buf, uint64(peer.PeerAS), 10)
	buf = append(buf, `}}`...)
	return buf
}

// AppendOpen appends an OPEN message text line to buf, terminated by '\n'.
// Format: peer <ip> remote as <asn> <direction> open <msg-id> router-id <id> hold-time <t> [cap <code> <name> <value>]* .
func AppendOpen(buf []byte, peer *plugin.PeerInfo, open DecodedOpen, direction string, msgID uint64) []byte {
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	buf = strconv.AppendUint(buf, uint64(open.ASN), 10)
	buf = append(buf, ' ')
	buf = append(buf, direction...)
	buf = append(buf, " open "...)
	buf = strconv.AppendUint(buf, msgID, 10)
	buf = append(buf, " router-id "...)
	buf = append(buf, open.RouterID...)
	buf = append(buf, " hold-time "...)
	buf = strconv.AppendUint(buf, uint64(open.HoldTime), 10)
	for _, cap := range open.Capabilities {
		buf = append(buf, " cap "...)
		buf = strconv.AppendUint(buf, uint64(cap.Code), 10)
		buf = append(buf, ' ')
		buf = append(buf, cap.Name...)
		if cap.Value != "" {
			buf = append(buf, ' ')
			buf = append(buf, cap.Value...)
		}
	}
	buf = append(buf, '\n')
	return buf
}

// AppendNotification appends a NOTIFICATION message text line to buf, terminated by '\n'.
// Format: peer <ip> remote as <asn> <direction> notification <msg-id> code <n> subcode <n> code-name <name> subcode-name <name> data <hex> .
// Names are hyphenated for single-word parsing (e.g., "Administrative-Shutdown").
func AppendNotification(buf []byte, peer *plugin.PeerInfo, notify DecodedNotification, direction string, msgID uint64) []byte {
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	buf = strconv.AppendUint(buf, uint64(peer.PeerAS), 10)
	buf = append(buf, ' ')
	buf = append(buf, direction...)
	buf = append(buf, " notification "...)
	buf = strconv.AppendUint(buf, msgID, 10)
	buf = append(buf, " code "...)
	buf = strconv.AppendUint(buf, uint64(notify.ErrorCode), 10)
	buf = append(buf, " subcode "...)
	buf = strconv.AppendUint(buf, uint64(notify.ErrorSubcode), 10)
	buf = append(buf, " code-name "...)
	buf = appendReplacingByte(buf, notify.ErrorCodeName, ' ', '-')
	buf = append(buf, " subcode-name "...)
	buf = appendReplacingByte(buf, notify.ErrorSubcodeName, ' ', '-')
	buf = append(buf, " data "...)
	if len(notify.Data) > 0 {
		buf = hex.AppendEncode(buf, notify.Data)
	}
	buf = append(buf, '\n')
	return buf
}

// AppendKeepalive appends a KEEPALIVE message text line to buf, terminated by '\n'.
// Format: peer <ip> remote as <asn> <direction> keepalive <msg-id> .
func AppendKeepalive(buf []byte, peer *plugin.PeerInfo, direction string, msgID uint64) []byte {
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	buf = strconv.AppendUint(buf, uint64(peer.PeerAS), 10)
	buf = append(buf, ' ')
	buf = append(buf, direction...)
	buf = append(buf, " keepalive "...)
	buf = strconv.AppendUint(buf, msgID, 10)
	buf = append(buf, '\n')
	return buf
}

// AppendRouteRefresh appends a ROUTE-REFRESH message text line to buf, terminated by '\n'.
// RFC 7313: type token is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
// Format: peer <ip> remote as <asn> <direction> <type> <msg-id> family <family> .
func AppendRouteRefresh(buf []byte, peer *plugin.PeerInfo, decoded DecodedRouteRefresh, direction string, msgID uint64) []byte {
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	buf = strconv.AppendUint(buf, uint64(peer.PeerAS), 10)
	buf = append(buf, ' ')
	buf = append(buf, direction...)
	buf = append(buf, ' ')
	buf = append(buf, decoded.SubtypeName...)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, msgID, 10)
	buf = append(buf, " family "...)
	buf = append(buf, decoded.Family...)
	buf = append(buf, '\n')
	return buf
}

// AppendStateChange appends a peer state change event to buf.
// State events are separate from BGP protocol messages.
// Common states: "up", "down", "connected", "established".
// reason is the close reason (empty for "up"): "tcp-failure", "notification", etc.
// Delegates to the existing formatStateChangeText/JSON helpers (out of scope
// for this migration) and appends their result; this is the only non-zero
// boundary allocation inside AppendStateChange.
func AppendStateChange(buf []byte, peer *plugin.PeerInfo, state, reason, encoding string) []byte {
	if encoding == plugin.EncodingJSON {
		return append(buf, formatStateChangeJSON(peer, state, reason)...)
	}
	return append(buf, formatStateChangeText(peer, state, reason)...)
}

// AppendEOR appends an End-of-RIB marker event to buf.
// RFC 4724 Section 2: EOR signals that initial routing information exchange is complete.
// family is the address family (e.g., "ipv4/unicast", "ipv6/unicast").
func AppendEOR(buf []byte, peer *plugin.PeerInfo, family, encoding string) []byte {
	if encoding == plugin.EncodingJSON {
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"eor"},`...)
		buf = appendPeerJSON(buf, peer)
		buf = append(buf, `,"eor":{"family":"`...)
		buf = append(buf, family...)
		buf = append(buf, `"}}}`...)
		buf = append(buf, '\n')
		return buf
	}
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	buf = strconv.AppendUint(buf, uint64(peer.PeerAS), 10)
	buf = append(buf, " eor "...)
	buf = append(buf, family...)
	buf = append(buf, '\n')
	return buf
}

// AppendCongestion appends a forward-path congestion event to buf.
// eventType is "congested" or "resumed".
func AppendCongestion(buf []byte, peer *plugin.PeerInfo, eventType, encoding string) []byte {
	if encoding == plugin.EncodingJSON {
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"`...)
		buf = append(buf, eventType...)
		buf = append(buf, `"},`...)
		buf = appendPeerJSON(buf, peer)
		buf = append(buf, `}}`...)
		buf = append(buf, '\n')
		return buf
	}
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	buf = strconv.AppendUint(buf, uint64(peer.PeerAS), 10)
	buf = append(buf, ' ')
	buf = append(buf, eventType...)
	buf = append(buf, '\n')
	return buf
}
