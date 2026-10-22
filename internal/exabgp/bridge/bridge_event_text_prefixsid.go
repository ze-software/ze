// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP TEXT event encoder
// Overview: bridge_event_text.go -- the attributes of a text UPDATE line
//
// The BGP Prefix-SID attribute of a text UPDATE line, and the one-bare list
// rule for an attribute whose octets are not at hand.

package bridge

// bridgeAttrPrefixSID is ze's event key for the BGP Prefix-SID attribute, and
// ExaBGP's text word for it (AttributeCollection.representation).
const bridgeAttrPrefixSID = "bgp-prefix-sid"

// appendPrefixSIDText writes PrefixSid.__str__ (attribute/sr/prefixsid.py,
// 5.0.1) for the SR-MPLS form ze decodes:
//
//	label_index = next((i for i in self.sr_attrs if i.TLV == 1), None)
//	if label_index is not None:
//	    srgb = next((i for i in self.sr_attrs if i.TLV == SR_TLV_SRGB), None)
//	    if srgb is not None:
//	        return f'[ {label_index!s}, {srgb!s} ]'
//	    return f'[ {label_index!s} ]'
//
// SrLabelIndex.__repr__ is the index alone, and SrGb.__repr__ is
// `[ ( base,range ), ... ]`. ze states the two TLVs as `sr-label-index` and
// `sr-srgbs` ([[base, range], ...]). A value of another shape (an SRv6
// service, which ExaBGP writes TLV by TLV) is written as ze states it, inside
// the same brackets, so the line still names what arrived.
func appendPrefixSIDText(buf []byte, value any) []byte {
	object, isObject := value.(map[string]any)
	index, hasIndex := object["sr-label-index"]
	if !isObject || !hasIndex {
		buf = append(buf, "[ "...)
		buf = appendAttributeMemberText(buf, value)
		return append(buf, " ]"...)
	}
	buf = append(buf, "[ "...)
	buf = appendJSONNumber(buf, index)
	if ranges, hasSRGB := object["sr-srgbs"].([]any); hasSRGB {
		buf = append(buf, ", [ "...)
		for position, entry := range ranges {
			if position > 0 {
				buf = append(buf, ", "...)
			}
			pair, _ := entry.([]any)
			buf = append(buf, "( "...)
			if len(pair) == 2 {
				buf = appendJSONNumber(buf, pair[0])
				buf = append(buf, ',')
				buf = appendJSONNumber(buf, pair[1])
			}
			buf = append(buf, " )"...)
		}
		buf = append(buf, " ]"...)
	}
	return append(buf, " ]"...)
}

// appendBareSingleText writes a list-valued attribute as Communities.__repr__
// does, from ze's own list: one member bare, more inside `[ ]`.
func appendBareSingleText(buf []byte, value any) []byte {
	list, isList := value.([]any)
	if !isList || len(list) > 1 {
		return appendAttributeValueText(buf, value)
	}
	return appendAttributeMemberText(buf, list[0])
}
