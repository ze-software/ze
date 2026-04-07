// Design: docs/architecture/api/json-format.md — JSON output rendering
// Overview: text.go — message format dispatch

package format

import (
	"encoding/hex"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// formatFilterResultJSON formats FilterResult as JSON (ze-bgp JSON).
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family.
//
// RFC 4271 Section 5.1.3: NEXT_HOP defines the IP address of the router
// that SHOULD be used as next hop to the destinations.
// RFC 4760 Section 3: Each MP_REACH_NLRI has its own next-hop field.
//
// ze-bgp JSON format:
//
//	{
//	  "type": "bgp",
//	  "bgp": {
//	    "message": {"type": "update", "id": 123, "direction": "received"},
//	    "peer": {"address": "...", "name": "...", "remote": {"as": ...}},
//	    "update": {
//	      "attr": {"origin": "igp", ...},
//	      "nlri": {
//	        "ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]
//	      }
//	    }
//	  }
//	}
func formatFilterResultJSON(peer plugin.PeerInfo, result bgpfilter.FilterResult, msgID uint64, direction string, ctx *bgpctx.EncodingContext) string {
	var sb strings.Builder
	var numBuf [20]byte

	// ze-bgp JSON outer wrapper
	sb.WriteString(`{"type":"bgp","bgp":{`)

	// Message metadata with type inside
	sb.WriteString(`"message":{"type":"update"`)
	if msgID > 0 {
		sb.WriteString(`,"id":`)
		sb.Write(strconv.AppendUint(numBuf[:0], msgID, 10))
	}
	if direction != "" {
		sb.WriteString(`,"direction":"`)
		sb.WriteString(direction)
		sb.WriteString(`"`)
	}
	sb.WriteString(`}`)

	// Peer at bgp level
	writePeerJSON(&sb, peer)

	// Update container with attr and nlri inside
	sb.WriteString(`,"update":{`)

	// Attributes inside update
	if len(result.Attributes) > 0 {
		sb.WriteString(`"attr":{`)
		formatAttributesJSON(&sb, result)
		sb.WriteString(`},`)
	}

	// Collect operations by family
	// Map: family -> list of operations (each op has action, next-hop, nlris)
	familyOps := make(map[string][]familyOperation)

	// Add announced routes (action: add)
	announced := result.AnnouncedByFamily(ctx)
	for _, fam := range announced {
		op := familyOperation{
			Action:  "add",
			NextHop: fam.NextHop.String(),
			NLRIs:   fam.NLRIs,
		}
		familyOps[fam.Family] = append(familyOps[fam.Family], op)
	}

	// Add withdrawn routes (action: del)
	withdrawn := result.WithdrawnByFamily(ctx)
	for _, fam := range withdrawn {
		op := familyOperation{
			Action: "del",
			NLRIs:  fam.NLRIs,
		}
		familyOps[fam.Family] = append(familyOps[fam.Family], op)
	}

	// NLRIs inside update
	sb.WriteString(`"nlri":{`)
	formatFamilyOpsJSON(&sb, familyOps)
	sb.WriteString(`}`)

	// Close update, bgp, and outer wrapper
	sb.WriteString("}}}\n")
	return sb.String()
}

// familyOperation represents a single operation (add/del) for a family.
type familyOperation struct {
	Action  string      // "add" or "del"
	NextHop string      // Only for "add" operations
	NLRIs   []nlri.NLRI // NLRIs in this operation
}

// formatNLRIJSONValue formats a single NLRI as JSON value.
// Simple prefixes without path-id are output as strings: "10.0.0.0/24".
// Complex NLRIs (ADD-PATH, VPN, EVPN, FlowSpec) are output as objects with structured fields.
//
// Plugin NLRI types (VPN, EVPN, FlowSpec) are decoded via registry.DecodeNLRIByFamily,
// which routes to the plugin's in-process decoder. This avoids direct plugin type imports.
// Core types (LabeledUnicast) are handled directly since they live in the nlri package.
//
// RFC 4364: VPN NLRI includes RD and labels.
// RFC 7432: EVPN NLRI includes route-type, ESI, etc.
// RFC 8277: Labeled Unicast NLRI includes labels.
// RFC 8955: FlowSpec NLRI includes match components.
func formatNLRIJSONValue(sb *strings.Builder, n nlri.NLRI, familyStr string) {
	// Try registry-based decode for plugin NLRI types (VPN, EVPN, FlowSpec, Labeled).
	// The registry routes to the plugin's InProcessNLRIDecoder by family.
	// Path-id is transport-level (ADD-PATH) and not included in decoder output.
	if registry.PluginForFamily(familyStr) != "" {
		hexData := hex.EncodeToString(n.Bytes())
		decoded, err := registry.DecodeNLRIByFamily(familyStr, hexData)
		if err == nil {
			sb.WriteString(decoded)
			return
		}
	}

	pathID := n.PathID()

	// Simple prefix without path-id: output as string
	if pathID == 0 {
		if p, ok := n.(prefixer); ok {
			var pfxBuf [44]byte // max IPv6 prefix: 39 chars + /3 digits + NUL
			sb.WriteString(`"`)
			sb.Write(p.Prefix().AppendTo(pfxBuf[:0]))
			sb.WriteString(`"`)
			return
		}
	}

	// Complex NLRI (has path-id or not a simple prefix): output as object
	formatNLRIJSON(sb, n)
}

// prefixer is implemented by NLRI types that have a Prefix() method.
type prefixer interface {
	Prefix() netip.Prefix
}

// formatNLRIJSON formats a single NLRI as JSON.
// RFC 7911: Outputs structured format with path-id when present.
// Format: {"prefix":"10.0.0.0/24"} or {"prefix":"10.0.0.0/24","path-id":1}.
func formatNLRIJSON(sb *strings.Builder, n nlri.NLRI) {
	sb.WriteString(`{"prefix":"`)

	// Use type assertion to get prefix cleanly
	if p, ok := n.(prefixer); ok {
		var pfxBuf [44]byte
		sb.Write(p.Prefix().AppendTo(pfxBuf[:0]))
	} else {
		// Fallback for complex NLRI types (EVPN, FlowSpec, etc.)
		// Escape for JSON safety (handles quotes, backslashes, control chars)
		writeJSONEscapedString(sb, n.String())
	}
	sb.WriteString(`"`)

	if pathID := n.PathID(); pathID != 0 {
		var numBuf [20]byte
		sb.WriteString(`,"path-id":`)
		sb.Write(strconv.AppendUint(numBuf[:0], uint64(pathID), 10))
	}

	sb.WriteString(`}`)
}

// formatAttributesJSON formats attributes from FilterResult for JSON.
func formatAttributesJSON(sb *strings.Builder, result bgpfilter.FilterResult) {
	if len(result.Attributes) == 0 {
		return
	}

	first := true
	for code, attr := range result.Attributes {
		if !first {
			sb.WriteString(",")
		}
		first = false
		formatAttributeJSON(sb, code, attr)
	}
}

// formatFamilyOpsJSON writes family operations as JSON object entries.
// Shared by formatFilterResultJSON and FormatDecodeUpdateJSON.
func formatFamilyOpsJSON(sb *strings.Builder, familyOps map[string][]familyOperation) {
	first := true
	for fam, ops := range familyOps {
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(`"`)
		sb.WriteString(fam)
		sb.WriteString(`":[`)
		for i, op := range ops {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`{`)
			if op.Action == "add" && op.NextHop != "" && op.NextHop != "invalid IP" {
				sb.WriteString(`"next-hop":"`)
				sb.WriteString(op.NextHop)
				sb.WriteString(`",`)
			}
			sb.WriteString(`"action":"`)
			sb.WriteString(op.Action)
			sb.WriteString(`","nlri":[`)
			for j, n := range op.NLRIs {
				if j > 0 {
					sb.WriteString(",")
				}
				formatNLRIJSONValue(sb, n, fam)
			}
			sb.WriteString(`]}`)
		}
		sb.WriteString(`]`)
	}
}

// formatAttributeJSON formats a single attribute for JSON.
// Known attribute types are formatted with named keys; unknown types use "attr-N" with hex value.
func formatAttributeJSON(sb *strings.Builder, code attribute.AttributeCode, attr attribute.Attribute) {
	var numBuf [20]byte

	switch code { //nolint:exhaustive // common attributes; unknown handled after switch
	case attribute.AttrOrigin:
		switch o := attr.(type) {
		case *attribute.Origin:
			sb.WriteString(`"origin":"`)
			sb.WriteString(strings.ToLower(o.String()))
			sb.WriteString(`"`)
		case attribute.Origin:
			sb.WriteString(`"origin":"`)
			sb.WriteString(strings.ToLower(o.String()))
			sb.WriteString(`"`)
		}
		return
	case attribute.AttrASPath:
		if ap, ok := attr.(*attribute.ASPath); ok {
			sb.WriteString(`"as-path":[`)
			first := true
			for _, seg := range ap.Segments {
				for _, asn := range seg.ASNs {
					if !first {
						sb.WriteString(",")
					}
					first = false
					sb.Write(strconv.AppendUint(numBuf[:0], uint64(asn), 10))
				}
			}
			sb.WriteString("]")
		}
		return
	case attribute.AttrMED:
		switch m := attr.(type) {
		case *attribute.MED:
			sb.WriteString(`"med":`)
			sb.Write(strconv.AppendUint(numBuf[:0], uint64(uint32(*m)), 10))
		case attribute.MED:
			sb.WriteString(`"med":`)
			sb.Write(strconv.AppendUint(numBuf[:0], uint64(uint32(m)), 10))
		}
		return
	case attribute.AttrLocalPref:
		switch lp := attr.(type) {
		case *attribute.LocalPref:
			sb.WriteString(`"local-preference":`)
			sb.Write(strconv.AppendUint(numBuf[:0], uint64(uint32(*lp)), 10))
		case attribute.LocalPref:
			sb.WriteString(`"local-preference":`)
			sb.Write(strconv.AppendUint(numBuf[:0], uint64(uint32(lp)), 10))
		}
		return
	case attribute.AttrCommunity:
		if c, ok := attr.(*attribute.Communities); ok {
			sb.WriteString(`"communities":[`)
			for i, comm := range *c {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.WriteString(comm.String())
				sb.WriteString(`"`)
			}
			sb.WriteString("]")
		}
		return
	case attribute.AttrLargeCommunity:
		if lc, ok := attr.(*attribute.LargeCommunities); ok {
			sb.WriteString(`"large-communities":[`)
			for i, comm := range *lc {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.WriteString(comm.String())
				sb.WriteString(`"`)
			}
			sb.WriteString("]")
		}
		return
	case attribute.AttrExtCommunity:
		if ec, ok := attr.(*attribute.ExtendedCommunities); ok {
			sb.WriteString(`"extended-communities":[`)
			var hexBuf [16]byte // ext community is 8 bytes -> 16 hex chars
			for i, comm := range *ec {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.Write(hex.AppendEncode(hexBuf[:0], comm[:]))
				sb.WriteString(`"`)
			}
			sb.WriteString("]")
		}
		return
	}
	// Unknown attribute code — format as "attr-N": "hex"
	attrBuf := make([]byte, attr.Len())
	attr.WriteTo(attrBuf, 0)
	sb.WriteString(`"attr-`)
	sb.Write(strconv.AppendUint(numBuf[:0], uint64(code), 10))
	sb.WriteString(`":"`)
	sb.Write(hex.AppendEncode(nil, attrBuf))
	sb.WriteString(`"`)
}

func formatStateChangeJSON(peer plugin.PeerInfo, state, reason string) string {
	// ze-bgp JSON format with reason for down events.
	// reason is only present for "down" events — "up" has no close reason.
	p := peerJSONInline(peer)
	if reason != "" {
		return `{"type":"bgp","bgp":{"message":{"type":"state"},` + p + `,"state":"` + state + `","reason":"` + reason + `"}}` + "\n"
	}
	return `{"type":"bgp","bgp":{"message":{"type":"state"},` + p + `,"state":"` + state + `"}}` + "\n"
}
