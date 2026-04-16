// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS in-process JSON writer
//
// AppendJSON lets BGP-LS NLRI types participate in the nlri.JSONWriter fast
// path (formatNLRIJSONValue). Without this file, BGP-LS falls through
// formatNLRIJSON and emits {"prefix":"<String()>"} (e.g. "node protocol OSPFv2
// asn 65000") instead of the structured RFC 7752 / 9514 JSON.
//
// Outer JSON shape is streamed directly (eliminates the outer map + Marshal
// round-trip). Nested descriptor arrays still come from the parseBGPLS*TLVs
// helpers and are emitted via json.Marshal per element -- cold path, TLV
// trees are deep, full streaming would duplicate every walker branch.
//
// Keys are emitted in alphabetical order to match the byte shape produced by
// bgplsToJSON + json.Marshal (Go's json package sorts map keys). Any
// downstream consumer that diffs JSON text sees the same bytes whether the
// NLRI went through the RPC decode path or the in-process fast path.

package ls

import (
	"encoding/json"
	"strconv"
	"strings"
)

// AppendJSON satisfies nlri.JSONWriter for BGP-LS Node NLRI (Type 1).
func (n *BGPLSNode) AppendJSON(sb *strings.Builder) { appendBGPLSJSON(sb, n) }

// AppendJSON satisfies nlri.JSONWriter for BGP-LS Link NLRI (Type 2).
func (l *BGPLSLink) AppendJSON(sb *strings.Builder) { appendBGPLSJSON(sb, l) }

// AppendJSON satisfies nlri.JSONWriter for BGP-LS IPv4/IPv6 Prefix NLRI (Types 3/4).
func (p *BGPLSPrefix) AppendJSON(sb *strings.Builder) { appendBGPLSJSON(sb, p) }

// AppendJSON satisfies nlri.JSONWriter for BGP-LS SRv6 SID NLRI (Type 6, RFC 9514).
func (s *BGPLSSRv6SID) AppendJSON(sb *strings.Builder) { appendBGPLSJSON(sb, s) }

// appendBGPLSJSON streams the BGP-LS JSON representation of n into sb in
// alphabetical key order (matches json.Marshal(bgplsToJSON(...))).
// Mirrors bgplsToJSON (plugin.go) -- update both if the shape changes.
func appendBGPLSJSON(sb *strings.Builder, n BGPLSNLRI) {
	data := bgplsRawBytes(n)

	sb.WriteByte('{')

	switch n.NLRIType() {
	case BGPLSNodeNLRI:
		// Keys: l3-routing-topology, ls-nlri-type, node-descriptors, protocol-id
		writeTopology(sb, n)
		sb.WriteByte(',')
		writeType(sb, n)
		sb.WriteString(`,"node-descriptors":`)
		marshalAny(sb, parseBGPLSNodeTLVs(data))
		sb.WriteByte(',')
		writeProtocol(sb, n)

	case BGPLSLinkNLRI:
		// Keys: interface-addresses, l3-routing-topology, link-identifiers,
		// local-node-descriptors, ls-nlri-type, multi-topology-ids,
		// neighbor-addresses, protocol-id, remote-node-descriptors.
		localDescs, remoteDescs, info := parseBGPLSLinkTLVs(data)
		sb.WriteString(`"interface-addresses":`)
		marshalAny(sb, info.ifAddrs)
		sb.WriteByte(',')
		writeTopology(sb, n)
		sb.WriteString(`,"link-identifiers":`)
		marshalAny(sb, info.linkIDs)
		sb.WriteString(`,"local-node-descriptors":`)
		marshalAny(sb, localDescs)
		sb.WriteByte(',')
		writeType(sb, n)
		sb.WriteString(`,"multi-topology-ids":`)
		marshalAny(sb, info.mtIDs)
		sb.WriteString(`,"neighbor-addresses":`)
		marshalAny(sb, info.neighAddrs)
		sb.WriteByte(',')
		writeProtocol(sb, n)
		sb.WriteString(`,"remote-node-descriptors":`)
		marshalAny(sb, remoteDescs)

	case BGPLSPrefixV4NLRI, BGPLSPrefixV6NLRI:
		// Keys: ip-reach-prefix, ip-reachability-tlv (both only if prefix!=""),
		// l3-routing-topology, ls-nlri-type, multi-topology-ids,
		// node-descriptors, protocol-id.
		nodeDescs, prefixInfo := parseBGPLSPrefixTLVs(data, n.NLRIType())
		if prefixInfo.prefix != "" {
			sb.WriteString(`"ip-reach-prefix":`)
			writeJSONString(sb, prefixInfo.prefix)
			sb.WriteString(`,"ip-reachability-tlv":`)
			writeJSONString(sb, prefixInfo.prefix)
			sb.WriteByte(',')
		}
		writeTopology(sb, n)
		sb.WriteByte(',')
		writeType(sb, n)
		sb.WriteString(`,"multi-topology-ids":`)
		marshalAny(sb, prefixInfo.mtIDs)
		sb.WriteString(`,"node-descriptors":`)
		marshalAny(sb, nodeDescs)
		sb.WriteByte(',')
		writeProtocol(sb, n)

	case BGPLSSRv6SIDNLRI:
		// Keys: l3-routing-topology, ls-nlri-type, node-descriptors,
		// protocol-id, srv6-sid (only if SID present).
		writeTopology(sb, n)
		sb.WriteByte(',')
		writeType(sb, n)
		sb.WriteString(`,"node-descriptors":`)
		marshalAny(sb, parseBGPLSNodeTLVs(data))
		sb.WriteByte(',')
		writeProtocol(sb, n)
		if v, ok := n.(*BGPLSSRv6SID); ok && len(v.SRv6SID.SRv6SID) > 0 {
			sb.WriteString(`,"srv6-sid":`)
			writeJSONString(sb, formatIPv6Compressed(v.SRv6SID.SRv6SID))
		}

	default: // unknown type — emit only the always-present keys
		writeTopology(sb, n)
		sb.WriteByte(',')
		writeType(sb, n)
		sb.WriteByte(',')
		writeProtocol(sb, n)
	}

	sb.WriteByte('}')
}

// writeType writes `"ls-nlri-type":"<name>"` (no leading comma).
func writeType(sb *strings.Builder, n BGPLSNLRI) {
	sb.WriteString(`"ls-nlri-type":`)
	writeJSONString(sb, bgplsNLRITypeString(uint16(n.NLRIType())))
}

// writeTopology writes `"l3-routing-topology":<identifier>` (no leading comma).
func writeTopology(sb *strings.Builder, n BGPLSNLRI) {
	sb.WriteString(`"l3-routing-topology":`)
	sb.WriteString(strconv.FormatUint(n.Identifier(), 10))
}

// writeProtocol writes `"protocol-id":<proto>` (no leading comma).
func writeProtocol(sb *strings.Builder, n BGPLSNLRI) {
	sb.WriteString(`"protocol-id":`)
	sb.WriteString(strconv.Itoa(int(n.ProtocolID())))
}

// bgplsRawBytes returns the wire bytes for n, preferring the cached slice set
// by ParseBGPLS so wire-parsed NLRIs skip a fresh WriteTo allocation on every
// AppendJSON call. Returns n.Bytes() for programmatically-constructed NLRIs.
func bgplsRawBytes(n BGPLSNLRI) []byte {
	type cacher interface{ cachedBytes() []byte }
	if c, ok := n.(cacher); ok {
		if b := c.cachedBytes(); b != nil {
			return b
		}
	}
	return n.Bytes()
}

// marshalAny encodes v with json.Marshal and appends the result to sb.
// On error the JSON "null" literal is written so the surrounding object stays
// valid JSON -- callers are walking wire TLVs, not untrusted user input, so a
// marshal failure means a programming bug in a parseBGPLS* helper.
func marshalAny(sb *strings.Builder, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		sb.WriteString("null")
		return
	}
	sb.Write(b)
}

// writeJSONString writes s as a JSON-quoted string using strconv.Quote.
// Strings from the BGP-LS formatters are ASCII (IP prefixes, type names,
// IGP router IDs); strconv.Quote escapes control chars defensively.
func writeJSONString(sb *strings.Builder, s string) {
	sb.WriteString(strconv.Quote(s))
}
