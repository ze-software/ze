// Design: docs/architecture/api/json-format.md — peer JSON fragment helper
// Related: text.go — non-UPDATE Append* formatters reuse the append form
// Related: text_update.go — UPDATE-path formatters reuse the string form
// Related: text_json.go — JSON message encoders share the escape helper
// Related: summary.go — UPDATE summary JSON reuses writePeerJSON
//
// Peer-object JSON serialization, separated from text.go so that the strings
// package imports (strings.Builder) stay out of text.go once the non-UPDATE
// append migration is complete (spec AC-8). External callers that still
// emit string-typed JSON continue to use these helpers.
//
// All three helpers delegate string escaping to writeJSONEscapedString so
// that the byte-for-byte output matches the append-form peer JSON emitted
// by format/text.go's appendPeerJSON. This prevents the two paths from
// diverging on pathological peer names (control chars), which config
// validation already forbids but which the code must not silently produce
// as invalid JSON if the validation is ever bypassed.

package format

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// writePeerJSON writes the "peer":{...} JSON fragment to a strings.Builder.
// Structure matches YANG peer-info grouping: address, name, remote.as, group.
// Always includes address, name, and remote.as. Includes group when non-empty.
// Key order is alphabetical (matching json.Marshal output from peerMap).
func writePeerJSON(sb *strings.Builder, peer *plugin.PeerInfo) {
	sb.WriteString(`,"peer":{"address":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteByte('"')
	if peer.GroupName != "" {
		sb.WriteString(`,"group":"`)
		writeJSONEscapedString(sb, peer.GroupName)
		sb.WriteByte('"')
	}
	if peer.LocalAS > 0 || peer.LocalAddress.IsValid() {
		sb.WriteString(`,"local":{`)
		first := true
		if peer.LocalAddress.IsValid() {
			sb.WriteString(`"address":"`)
			sb.WriteString(peer.LocalAddress.String())
			sb.WriteByte('"')
			first = false
		}
		if peer.LocalAS > 0 {
			if !first {
				sb.WriteByte(',')
			}
			sb.WriteString(`"as":`)
			sb.WriteString(strconv.FormatUint(uint64(peer.LocalAS), 10))
		}
		sb.WriteByte('}')
	}
	sb.WriteString(`,"name":"`)
	writeJSONEscapedString(sb, peer.Name)
	sb.WriteString(`","remote":{"as":`)
	sb.WriteString(strconv.FormatUint(uint64(peer.PeerAS), 10))
	sb.WriteString(`}}`)
}

// peerJSONInline returns the peer JSON object as a string (without leading comma).
// Used by fmt.Sprintf sites where a Builder is not available.
// Structure matches YANG peer-info grouping: address, name, remote.as, group.
// Key order is alphabetical (matching json.Marshal output from peerMap).
func peerJSONInline(peer *plugin.PeerInfo) string {
	var sb strings.Builder
	sb.WriteString(`"peer":{"address":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteByte('"')
	if peer.GroupName != "" {
		sb.WriteString(`,"group":"`)
		writeJSONEscapedString(&sb, peer.GroupName)
		sb.WriteByte('"')
	}
	if peer.LocalAS > 0 || peer.LocalAddress.IsValid() {
		sb.WriteString(`,"local":{`)
		first := true
		if peer.LocalAddress.IsValid() {
			sb.WriteString(`"address":"`)
			sb.WriteString(peer.LocalAddress.String())
			sb.WriteByte('"')
			first = false
		}
		if peer.LocalAS > 0 {
			if !first {
				sb.WriteByte(',')
			}
			sb.WriteString(`"as":`)
			sb.WriteString(strconv.FormatUint(uint64(peer.LocalAS), 10))
		}
		sb.WriteByte('}')
	}
	sb.WriteString(`,"name":"`)
	writeJSONEscapedString(&sb, peer.Name)
	sb.WriteString(`","remote":{"as":`)
	sb.WriteString(strconv.FormatUint(uint64(peer.PeerAS), 10))
	sb.WriteString(`}}`)
	return sb.String()
}

// writeJSONEscapedString writes s to sb with JSON string escaping.
// Escapes: \ " \n \r \t and control characters 0x00-0x1F (via \u00XX).
// Matches format.appendJSONString byte-for-byte; used across both string
// and append code paths so peer names / group names produce identical
// output regardless of which helper the formatter reaches through.
func writeJSONEscapedString(sb *strings.Builder, s string) {
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
			continue
		case '"':
			sb.WriteString(`\"`)
			continue
		case '\n':
			sb.WriteString(`\n`)
			continue
		case '\r':
			sb.WriteString(`\r`)
			continue
		case '\t':
			sb.WriteString(`\t`)
			continue
		}
		if r < 0x20 {
			// Control character - use \uXXXX
			fmt.Fprintf(sb, `\u%04x`, r)
			continue
		}
		sb.WriteRune(r)
	}
}
