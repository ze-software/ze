// Design: docs/architecture/api/json-format.md — JSON output rendering
// Overview: text.go — message format dispatch

package format

import (
	"encoding/hex"
	"net/netip"
	"slices"
	"strconv"

	bgpfilter "github.com/ze-software/ze/internal/component/bgp/filter"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// appendFilterResultJSON appends FilterResult as JSON to buf (ze-bgp JSON).
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family. closeEnvelope controls whether the
// outer `}}}\n` is written: true for standalone parsed output, false when the
// caller (formatFullFromResult) needs to inject additional fields before the close.
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
//	    "peer": {"local": {"address": "...", "as": ...}, "name": "...", "remote": {"address": "...", "as": ...}},
//	    "update": {
//	      "attr": {"origin": "igp", ...},
//	      "nlri": {
//	        "ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]
//	      }
//	    }
//	  }
//	}
//
// messageType is "update" or "sent" -- threaded in so callers do not run
// strings.Replace surgery on the produced JSON.
func appendFilterResultJSON(buf []byte, peer *plugin.PeerInfo, result bgpfilter.FilterResult, msgID uint64, direction rpc.MessageDirection, ctx *bgpctx.EncodingContext, messageType string, closeEnvelope bool) []byte {
	// ze-bgp JSON outer wrapper
	buf = append(buf, `{"type":"bgp","bgp":{`...)

	// Message metadata with type inside
	buf = append(buf, `"message":{"type":"`...)
	buf = append(buf, messageType...)
	buf = append(buf, '"')
	if msgID > 0 {
		buf = append(buf, `,"id":`...)
		buf = strconv.AppendUint(buf, msgID, 10)
	}
	if direction != rpc.DirectionUnspecified {
		buf = append(buf, `,"direction":"`...)
		buf = direction.AppendTo(buf)
		buf = append(buf, '"')
	}
	buf = append(buf, '}', ',')

	// Peer at bgp level
	buf = appendPeerJSON(buf, peer)

	// Update container with attr and nlri inside
	buf = append(buf, `,"update":{`...)

	// Attributes inside update
	if len(result.Attributes) > 0 {
		buf = append(buf, `"attr":{`...)
		buf = appendAttributesJSON(buf, result, false)
		buf = append(buf, `},`...)
	}

	announced := result.AnnouncedByFamily(ctx)
	withdrawn := result.WithdrawnByFamily(ctx)

	// NLRIs inside update: emit per-family operations WITHOUT building an
	// intermediate map (map creation was the dominant allocation on warm
	// scratch before fmt-1). Families are discovered in iteration order
	// (announced first, then new families from withdrawn). Map iteration
	// order of the legacy code was non-deterministic; tests already
	// tolerate this, and parity assertions use JSON-parsed comparison for
	// the map-containing regions.
	buf = append(buf, `"nlri":{`...)
	buf = appendFamiliesJSON(buf, announced, withdrawn)
	buf = append(buf, '}')

	if closeEnvelope {
		// Close update, bgp, and outer wrapper.
		buf = append(buf, "}}}\n"...)
	}
	// INVARIANT: when closeEnvelope is false, buf ends with `,"nlri":{...}}`
	// -- the last `}` closes `nlri`, leaving `"update":{...`, `"bgp":{...`,
	// and the outer `{` all open. appendFullFromResult depends on this exact
	// shape to inject `,"raw":{...},"route-meta":{...}}}\n`. Any change to
	// what this writer leaves open MUST update appendFullFromResult in
	// lockstep (the legacy strings.HasSuffix guard is gone).
	return buf
}

// The two values familyOperation.Action takes. They reach a consumer as the
// "action" member of a per-family operation object, so the spelling is part of
// the published format. The append-based writers embed them inside a larger
// JSON fragment literal and so cannot use these constants.
const (
	actionAdd = "add"
	actionDel = "del"
)

// familyOperation represents a single operation (add/del) for a family.
// Retained for FormatDecodeUpdateJSON (codec.go) which still uses the map
// grouping since it mixes legacy IPv4 + MP routes from multiple sources.
type familyOperation struct {
	Action  string      // actionAdd or actionDel
	NextHop string      // Only for actionAdd operations
	NLRIs   []nlri.NLRI // NLRIs in this operation
}

// appendFamiliesJSON writes the per-family operation object directly from
// the ordered announced / withdrawn slices, without building an intermediate
// map keyed by family. Families are discovered in iteration order: announced
// first, then withdrawn families not previously seen. Matches the legacy
// (non-deterministic) map iteration in the common case of one entry per
// family; tests compare the map region via JSON-parsed equality.
func appendFamiliesJSON(buf []byte, announced, withdrawn []bgpfilter.FamilyNLRI) []byte {
	// Stack-local scan array: typical UPDATE has ≤2 families; 8 covers every
	// case we see in production while keeping the array on the stack.
	var seenScratch [8]family.Family
	seen := seenScratch[:0]

	contains := func(f family.Family) bool {
		return slices.Contains(seen, f)
	}

	writeFamily := func(buf []byte, fam family.Family, isFirst bool) []byte {
		if !isFirst {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = fam.AppendTo(buf)
		buf = append(buf, `":[`...)
		opFirst := true
		// Emit all announced ops for this family (action=add).
		for i := range announced {
			if announced[i].Family != fam {
				continue
			}
			if !opFirst {
				buf = append(buf, ',')
			}
			opFirst = false
			buf = append(buf, '{')
			nh := announced[i].NextHop
			if nh.IsValid() {
				buf = append(buf, `"next-hop":"`...)
				buf = nh.AppendTo(buf)
				buf = append(buf, `",`...)
			}
			buf = append(buf, `"action":"add","nlri":[`...)
			for j, n := range announced[i].NLRIs {
				if j > 0 {
					buf = append(buf, ',')
				}
				buf = appendNLRIJSONValue(buf, n, fam)
			}
			buf = append(buf, `]}`...)
		}
		// Emit all withdrawn ops for this family (action=del).
		for i := range withdrawn {
			if withdrawn[i].Family != fam {
				continue
			}
			if !opFirst {
				buf = append(buf, ',')
			}
			opFirst = false
			buf = append(buf, `{"action":"del","nlri":[`...)
			for j, n := range withdrawn[i].NLRIs {
				if j > 0 {
					buf = append(buf, ',')
				}
				buf = appendNLRIJSONValue(buf, n, fam)
			}
			buf = append(buf, `]}`...)
		}
		buf = append(buf, ']')
		return buf
	}

	first := true
	for i := range announced {
		fam := announced[i].Family
		if contains(fam) {
			continue
		}
		// append grows beyond cap(seen) into the heap when families exceed
		// the stack array; that is the expected fallback for pathological
		// UPDATEs with more than 8 distinct families.
		seen = append(seen, fam)
		buf = writeFamily(buf, fam, first)
		first = false
	}
	for i := range withdrawn {
		fam := withdrawn[i].Family
		if contains(fam) {
			continue
		}
		seen = append(seen, fam)
		buf = writeFamily(buf, fam, first)
		first = false
	}
	return buf
}

// appendNLRIJSONValue appends a single NLRI as JSON value to buf.
// Simple prefixes without path-id are output as strings: "10.0.0.0/24".
// Complex NLRIs (ADD-PATH, VPN, EVPN, FlowSpec) are output as objects with structured fields.
//
// Hot path: NLRI types implementing nlri.JSONAppender stream JSON directly into
// buf (zero-alloc for simple plugins, single json.Marshal for FlowSpec).
// Registry hex decoder path is retained as a fallback for external plugins that
// live over RPC and have no in-process Go type to assert against.
//
// RFC 4364: VPN NLRI includes RD and labels.
// RFC 7432: EVPN NLRI includes route-type, ESI, etc.
// RFC 8277: Labeled Unicast NLRI includes labels.
// RFC 8955: FlowSpec NLRI includes match components.
func appendNLRIJSONValue(buf []byte, n nlri.NLRI, fam family.Family) []byte {
	// Fast path: concrete in-process NLRI types implement nlri.JSONAppender and
	// write their JSON directly, skipping wire encode + hex + re-parse + map.
	if w, ok := n.(nlri.JSONAppender); ok {
		return w.AppendJSON(buf)
	}

	// Fallback: external plugins over RPC. Wire-encode, hex, dispatch via registry.
	// Registry APIs are string-keyed; stringify the typed family once at this boundary.
	familyStr := fam.String()
	if registry.PluginForFamily(familyStr) != "" {
		hexData := hex.EncodeToString(n.Bytes())
		decoded, err := registry.DecodeNLRIByFamily(familyStr, hexData)
		if err == nil {
			return append(buf, decoded...)
		}
	}

	pathID := n.PathID()

	// Simple prefix without path-id: output as string
	if pathID == 0 {
		if p, ok := n.(prefixer); ok {
			buf = append(buf, '"')
			buf = p.Prefix().AppendTo(buf)
			buf = append(buf, '"')
			return buf
		}
	}

	// Complex NLRI (has path-id or not a simple prefix): output as object
	return appendNLRIJSON(buf, n)
}

// prefixer is implemented by NLRI types that have a Prefix() method.
type prefixer interface {
	Prefix() netip.Prefix
}

// appendNLRIJSON appends a single NLRI as JSON object to buf.
// RFC 7911: Outputs structured format with path-id when present.
// Format: {"prefix":"10.0.0.0/24"} or {"prefix":"10.0.0.0/24","path-id":1}.
func appendNLRIJSON(buf []byte, n nlri.NLRI) []byte {
	buf = append(buf, `{"prefix":"`...)

	// Use type assertion to get prefix cleanly
	if p, ok := n.(prefixer); ok {
		buf = p.Prefix().AppendTo(buf)
	} else {
		// Fallback for complex NLRI types (EVPN, FlowSpec, etc.)
		// Escape for JSON safety (handles quotes, backslashes, control chars)
		buf = appendJSONString(buf, n.String())
	}
	buf = append(buf, '"')

	if pathID := n.PathID(); pathID != 0 {
		buf = append(buf, `,"path-id":`...)
		buf = strconv.AppendUint(buf, uint64(pathID), 10)
	}

	buf = append(buf, '}')
	return buf
}

// appendAttributesJSON appends attributes from FilterResult for JSON.
// When includeFlags is true, each attribute is wrapped with RFC 4271 flag booleans.
func appendAttributesJSON(buf []byte, result bgpfilter.FilterResult, includeFlags bool) []byte {
	if len(result.Attributes) == 0 {
		return buf
	}

	first := true
	for code, attr := range result.Attributes {
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = appendAttributeJSON(buf, code, attr, includeFlags)
	}
	return buf
}

// appendFamilyOpsJSON appends family operations as JSON object entries.
// Shared by appendFilterResultJSON and FormatDecodeUpdateJSON.
// Map is keyed by canonical family string ("ipv4/unicast"); values are typed
// family.Family for downstream registry dispatch without re-parse.
func appendFamilyOpsJSON(buf []byte, familyOps map[string][]familyOperation) []byte {
	first := true
	for fam, ops := range familyOps {
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = append(buf, '"')
		buf = append(buf, fam...)
		buf = append(buf, `":[`...)
		// Registered families resolve to a typed Family; unknown name yields
		// the zero value, which makes appendNLRIJSONValue's registry lookup miss
		// and fall through to generic wire-encoding paths.
		famTyped, _ := family.LookupFamily(fam)
		for i, op := range ops {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, '{')
			if op.Action == actionAdd && op.NextHop != "" && op.NextHop != "invalid IP" {
				buf = append(buf, `"next-hop":"`...)
				buf = append(buf, op.NextHop...)
				buf = append(buf, `",`...)
			}
			buf = append(buf, `"action":"`...)
			buf = append(buf, op.Action...)
			buf = append(buf, `","nlri":[`...)
			for j, n := range op.NLRIs {
				if j > 0 {
					buf = append(buf, ',')
				}
				buf = appendNLRIJSONValue(buf, n, famTyped)
			}
			buf = append(buf, `]}`...)
		}
		buf = append(buf, ']')
	}
	return buf
}

// appendAttributeJSON appends a single attribute for JSON.
// Known attribute types are formatted with named keys; unknown types use "attr-N" with hex value.
// When includeFlags is true, each attribute value is wrapped with RFC 4271 flag booleans:
// "name":{"value":<v>,"optional":bool,"transitive":bool,"partial":bool}.
func appendAttributeJSON(buf []byte, code attribute.AttributeCode, attr attribute.Attribute, includeFlags bool) []byte {
	if f := attribute.GetJSONFormatter(code); f != nil {
		mark := len(buf)
		buf = attrKeyOpen(buf, f.Key, includeFlags)
		if result := f.AppendValue(buf, attr); result != nil {
			buf = attrFlagsClose(result, attr.Flags(), includeFlags)
			return buf
		}
		buf = buf[:mark]
	}
	// Unknown or unhandled attribute code — format as "attr-N": "hex".
	// attr.Len() bounds hex output; stack scratch for the common case, heap
	// spill via make for pathological inputs (RFC 4271 extended max is 65535).
	var scratch [512]byte
	raw := scratch[:0]
	if n := attr.Len(); n > cap(raw) {
		raw = make([]byte, n)
	} else {
		raw = scratch[:n]
	}
	attr.WriteTo(raw, 0)
	buf = append(buf, `"attr-`...)
	buf = strconv.AppendUint(buf, uint64(code), 10)
	if includeFlags {
		buf = append(buf, `":{"value":"`...)
	} else {
		buf = append(buf, `":"`...)
	}
	buf = hex.AppendEncode(buf, raw)
	buf = append(buf, '"')
	buf = attrFlagsClose(buf, attr.Flags(), includeFlags)
	return buf
}

// attrKeyOpen writes `"name":` or `"name":{"value":` depending on includeFlags.
func attrKeyOpen(buf []byte, name string, includeFlags bool) []byte {
	buf = append(buf, '"')
	buf = append(buf, name...)
	buf = append(buf, `":`...)
	if includeFlags {
		buf = append(buf, `{"value":`...)
	}
	return buf
}

// attrFlagsClose appends RFC 4271 attribute flag booleans and closes the wrapper object.
func attrFlagsClose(buf []byte, flags attribute.AttributeFlags, includeFlags bool) []byte {
	if !includeFlags {
		return buf
	}
	buf = append(buf, `,"optional":`...)
	buf = strconv.AppendBool(buf, flags.IsOptional())
	buf = append(buf, `,"transitive":`...)
	buf = strconv.AppendBool(buf, flags.IsTransitive())
	buf = append(buf, `,"partial":`...)
	buf = strconv.AppendBool(buf, flags.IsPartial())
	buf = append(buf, '}')
	return buf
}

// appendStateChangeJSON appends the ze-bgp state-change JSON envelope to buf,
// terminated by '\n'. reason is only present for "down" events; "up" has no
// close reason.
func appendStateChangeJSON(buf []byte, peer *plugin.PeerInfo, state rpc.SessionState, reason string, unheldRoles []string) []byte {
	buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"state"},`...)
	buf = appendPeerJSON(buf, peer)
	buf = append(buf, `,"state":"`...)
	buf = state.AppendTo(buf)
	if reason != "" {
		buf = append(buf, `","reason":"`...)
		buf = append(buf, reason...)
	}
	buf = append(buf, '"')
	buf = appendUnheldRolesJSON(buf, unheldRoles)
	buf = append(buf, `}}`...)
	buf = append(buf, '\n')
	return buf
}

// appendUnheldRolesJSON appends the retracted-role list as a member of the
// enclosing object, and appends nothing at all when the list is empty -- which
// it is for every event on a daemon that runs no claiming plugin.
//
// Each token is escaped: an external plugin declares its own claims over the
// Stage-1 RPC (rpc.RegistrationInput), so a token is not repository text and a
// quote in one would otherwise break the framing of every state event.
func appendUnheldRolesJSON(buf []byte, unheldRoles []string) []byte {
	if len(unheldRoles) == 0 {
		return buf
	}
	buf = append(buf, `,"unheld-roles":[`...)
	for i, role := range unheldRoles {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = appendJSONString(buf, role)
		buf = append(buf, '"')
	}
	buf = append(buf, ']')
	return buf
}
