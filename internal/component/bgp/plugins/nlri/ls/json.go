// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS in-process JSON writer
//
// AppendJSON lets BGP-LS NLRI types participate in the nlri.JSONAppender fast
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
)

// AppendJSON satisfies nlri.JSONAppender for BGP-LS Node NLRI (Type 1).
func (n *BGPLSNode) AppendJSON(buf []byte) []byte { return appendBGPLSJSON(buf, n) }

// AppendJSON satisfies nlri.JSONAppender for BGP-LS Link NLRI (Type 2).
func (l *BGPLSLink) AppendJSON(buf []byte) []byte { return appendBGPLSJSON(buf, l) }

// AppendJSON satisfies nlri.JSONAppender for BGP-LS IPv4/IPv6 Prefix NLRI (Types 3/4).
func (p *BGPLSPrefix) AppendJSON(buf []byte) []byte { return appendBGPLSJSON(buf, p) }

// AppendJSON satisfies nlri.JSONAppender for BGP-LS SRv6 SID NLRI (Type 6, RFC 9514).
func (s *BGPLSSRv6SID) AppendJSON(buf []byte) []byte { return appendBGPLSJSON(buf, s) }

// appendBGPLSJSON streams the BGP-LS JSON representation of n into buf in
// alphabetical key order (matches json.Marshal(bgplsToJSON(...))).
// Mirrors bgplsToJSON (plugin.go) -- update both if the shape changes.
func appendBGPLSJSON(buf []byte, n BGPLSNLRI) []byte {
	data := bgplsRawBytes(n)

	buf = append(buf, '{')

	switch n.NLRIType() {
	case BGPLSNodeNLRI:
		// Keys: l3-routing-topology, ls-nlri-type, node-descriptors, protocol-id
		buf = appendTopology(buf, n)
		buf = append(buf, ',')
		buf = appendType(buf, n)
		buf = append(buf, `,"node-descriptors":`...)
		buf = appendMarshaled(buf, parseBGPLSNodeTLVs(data))
		buf = append(buf, ',')
		buf = appendProtocol(buf, n)

	case BGPLSLinkNLRI:
		// Keys: interface-addresses, l3-routing-topology, link-identifiers,
		// local-node-descriptors, ls-nlri-type, multi-topology-ids,
		// neighbor-addresses, protocol-id, remote-node-descriptors.
		localDescs, remoteDescs, info := parseBGPLSLinkTLVs(data)
		buf = append(buf, `"interface-addresses":`...)
		buf = appendMarshaled(buf, info.ifAddrs)
		buf = append(buf, ',')
		buf = appendTopology(buf, n)
		buf = append(buf, `,"link-identifiers":`...)
		buf = appendMarshaled(buf, info.linkIDs)
		buf = append(buf, `,"local-node-descriptors":`...)
		buf = appendMarshaled(buf, localDescs)
		buf = append(buf, ',')
		buf = appendType(buf, n)
		buf = append(buf, `,"multi-topology-ids":`...)
		buf = appendMarshaled(buf, info.mtIDs)
		buf = append(buf, `,"neighbor-addresses":`...)
		buf = appendMarshaled(buf, info.neighAddrs)
		buf = append(buf, ',')
		buf = appendProtocol(buf, n)
		buf = append(buf, `,"remote-node-descriptors":`...)
		buf = appendMarshaled(buf, remoteDescs)

	case BGPLSPrefixV4NLRI, BGPLSPrefixV6NLRI:
		// Keys: ip-reach-prefix, ip-reachability-tlv (both only if prefix!=""),
		// l3-routing-topology, ls-nlri-type, multi-topology-ids,
		// node-descriptors, protocol-id.
		nodeDescs, prefixInfo := parseBGPLSPrefixTLVs(data, n.NLRIType())
		if prefixInfo.prefix != "" {
			buf = append(buf, `"ip-reach-prefix":`...)
			buf = appendJSONQuoted(buf, prefixInfo.prefix)
			buf = append(buf, `,"ip-reachability-tlv":`...)
			buf = appendJSONQuoted(buf, prefixInfo.prefix)
			buf = append(buf, ',')
		}
		buf = appendTopology(buf, n)
		buf = append(buf, ',')
		buf = appendType(buf, n)
		buf = append(buf, `,"multi-topology-ids":`...)
		buf = appendMarshaled(buf, prefixInfo.mtIDs)
		buf = append(buf, `,"node-descriptors":`...)
		buf = appendMarshaled(buf, nodeDescs)
		buf = append(buf, ',')
		buf = appendProtocol(buf, n)

	case BGPLSSRv6SIDNLRI:
		// Keys: l3-routing-topology, ls-nlri-type, node-descriptors,
		// protocol-id, srv6-sid (only if SID present).
		buf = appendTopology(buf, n)
		buf = append(buf, ',')
		buf = appendType(buf, n)
		buf = append(buf, `,"node-descriptors":`...)
		buf = appendMarshaled(buf, parseBGPLSNodeTLVs(data))
		buf = append(buf, ',')
		buf = appendProtocol(buf, n)
		if v, ok := n.(*BGPLSSRv6SID); ok && len(v.SRv6SID.SRv6SID) > 0 {
			buf = append(buf, `,"srv6-sid":`...)
			buf = appendJSONQuoted(buf, formatIPv6Compressed(v.SRv6SID.SRv6SID))
		}

	default: // unknown type — emit only the always-present keys
		buf = appendTopology(buf, n)
		buf = append(buf, ',')
		buf = appendType(buf, n)
		buf = append(buf, ',')
		buf = appendProtocol(buf, n)
	}

	buf = append(buf, '}')
	return buf
}

// appendType writes `"ls-nlri-type":"<name>"` (no leading comma).
func appendType(buf []byte, n BGPLSNLRI) []byte {
	buf = append(buf, `"ls-nlri-type":`...)
	return appendJSONQuoted(buf, bgplsNLRITypeString(uint16(n.NLRIType())))
}

// appendTopology writes `"l3-routing-topology":<identifier>` (no leading comma).
func appendTopology(buf []byte, n BGPLSNLRI) []byte {
	buf = append(buf, `"l3-routing-topology":`...)
	return strconv.AppendUint(buf, n.Identifier(), 10)
}

// appendProtocol writes `"protocol-id":<proto>` (no leading comma).
func appendProtocol(buf []byte, n BGPLSNLRI) []byte {
	buf = append(buf, `"protocol-id":`...)
	return strconv.AppendInt(buf, int64(n.ProtocolID()), 10)
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

// appendMarshaled encodes v with json.Marshal and appends the result to buf.
// On error the JSON "null" literal is written so the surrounding object stays
// valid JSON -- callers are walking wire TLVs, not untrusted user input, so a
// marshal failure means a programming bug in a parseBGPLS* helper.
func appendMarshaled(buf []byte, v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return append(buf, "null"...)
	}
	return append(buf, b...)
}

// appendJSONQuoted writes s as a JSON-quoted string using strconv.AppendQuote.
// Strings from the BGP-LS formatters are ASCII (IP prefixes, type names,
// IGP router IDs); strconv.AppendQuote escapes control chars defensively.
func appendJSONQuoted(buf []byte, s string) []byte {
	return strconv.AppendQuote(buf, s)
}
