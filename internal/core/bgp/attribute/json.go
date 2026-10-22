// Design: docs/architecture/api/json-format.md -- attribute JSON rendering registry

package attribute

import (
	"net/netip"
	"strconv"
)

// JSONFormatter describes how to render an attribute as JSON.
// Key is the JSON object key (e.g. "ipv6-extended-communities").
// AppendValue appends only the JSON value (not the key or flag wrapper)
// to buf and returns the extended buffer. Returns nil to signal that the
// attribute cannot be formatted (caller falls through to hex).
type JSONFormatter struct {
	Key         string
	AppendValue func(buf []byte, attr Attribute) []byte
}

// jsonFormatters is the registry of attribute JSON formatters, indexed by
// attribute code. Mirrors knownAttrParsers: fixed-size array, init()-time
// registration, no locking needed.
var jsonFormatters [256]*JSONFormatter

// RegisterJSONFormatter registers a JSON formatter for an attribute code.
// MUST only be called from init() functions. Not safe for concurrent use.
func RegisterJSONFormatter(code AttributeCode, key string, fn func(buf []byte, attr Attribute) []byte) {
	jsonFormatters[code] = &JSONFormatter{Key: key, AppendValue: fn}
}

// GetJSONFormatter returns the registered JSON formatter for an attribute
// code, or nil if none is registered.
func GetJSONFormatter(code AttributeCode) *JSONFormatter {
	return jsonFormatters[code]
}

func appendOriginJSON(buf []byte, attr Attribute) []byte {
	var o Origin
	switch v := attr.(type) {
	case *Origin:
		o = *v
	case Origin:
		o = v
	default:
		return nil
	}
	buf = append(buf, '"')
	buf = appendLowerASCII(buf, o.String())
	return append(buf, '"')
}

func appendLowerASCII(buf []byte, s string) []byte {
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf = append(buf, c)
	}
	return buf
}

func appendNextHopJSON(buf []byte, attr Attribute) []byte {
	nh, ok := attr.(*NextHop)
	if !ok {
		return nil
	}
	buf = append(buf, '"')
	buf = nh.Addr.AppendTo(buf)
	return append(buf, '"')
}

// appendASPathJSON writes the AS path of the plugin EVENT stream.
//
// Every AS number here is asplain, and the bgp/as-notation leaf does NOT reach
// this function. This JSON is a contract between Ze and a plugin, in any
// language. A plugin that matched an AS number would stop matching the moment
// an operator changed a display preference. The notation an operator reads is
// applied where a row is built for them
// (asPathList, internal/component/bgp/plugins/rib/rib_attr_format.go).
func appendASPathJSON(buf []byte, attr Attribute) []byte {
	ap, ok := attr.(*ASPath)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	first := true
	for _, seg := range ap.Segments {
		for _, asn := range seg.ASNs {
			if !first {
				buf = append(buf, ',')
			}
			first = false
			buf = strconv.AppendUint(buf, uint64(asn), 10)
		}
	}
	return append(buf, ']')
}

func appendMEDJSON(buf []byte, attr Attribute) []byte {
	switch m := attr.(type) {
	case *MED:
		return strconv.AppendUint(buf, uint64(uint32(*m)), 10)
	case MED:
		return strconv.AppendUint(buf, uint64(uint32(m)), 10)
	}
	return nil
}

func appendLocalPrefJSON(buf []byte, attr Attribute) []byte {
	switch lp := attr.(type) {
	case *LocalPref:
		return strconv.AppendUint(buf, uint64(uint32(*lp)), 10)
	case LocalPref:
		return strconv.AppendUint(buf, uint64(uint32(lp)), 10)
	}
	return nil
}

// appendAtomicAggregateJSON writes the ATOMIC_AGGREGATE as `true`. The
// attribute has no value, so its presence is the whole statement, and `true`
// is what ExaBGP writes under the same key.
func appendAtomicAggregateJSON(buf []byte, attr Attribute) []byte {
	if _, ok := attr.(AtomicAggregate); !ok {
		return nil
	}
	// RFC 4271 Section 4.3 f): "ATOMIC_AGGREGATE is a well-known discretionary
	// attribute of length 0."
	return append(buf, "true"...)
}

// appendAggregatorJSON writes the AGGREGATOR as the string "<asn>:<address>".
//
// It is the form the update text grammar reads back (Aggregator.AppendText)
// and the form ExaBGP writes under the same key, so one value serves a ze
// reader and the bridge with no translation. The AS number is asplain for the
// reason appendASPathJSON gives.
//
// The received two-octet form reaches this function already widened: the
// RFC 6793 receive procedure runs at ingest (wireu.CollapseAS4Family), so the
// value is the aggregating node whichever width the peer sent.
//
// It answers nil, so the caller writes the hex, for a value with no address:
// a decoded AGGREGATOR always holds one, and a value that does not is not one
// this function can state.
func appendAggregatorJSON(buf []byte, attr Attribute) []byte {
	aggregator, ok := attr.(*Aggregator)
	if !ok {
		return nil
	}
	// RFC 4271 Section 4.3 g): "The attribute contains the last AS number that
	// formed the aggregate route (encoded as 2 octets), followed by the IP
	// address of the BGP speaker that formed the aggregate route (encoded as 4
	// octets)."
	return appendAggregatingNodeJSON(buf, aggregator.ASN, aggregator.Address)
}

// appendAS4AggregatorJSON writes the AS4_AGGREGATOR in the AGGREGATOR's form,
// under its own key. A received one never reaches an event, because ingest
// folds it into the AGGREGATOR (RFC 6793 Section 4.2.3); one ze sends toward
// an OLD speaker does, beside the AGGREGATOR that carries AS_TRANS.
func appendAS4AggregatorJSON(buf []byte, attr Attribute) []byte {
	aggregator, ok := attr.(*AS4Aggregator)
	if !ok {
		return nil
	}
	// RFC 6793 Section 3: "The AS4_AGGREGATOR attribute has the same semantics
	// and the same encoding as the AGGREGATOR attribute, except that it carries
	// a four-octet AS number."
	return appendAggregatingNodeJSON(buf, aggregator.ASN, aggregator.Address)
}

// appendAggregatingNodeJSON writes `"<asn>:<address>"`, or answers nil for an
// address that is not valid.
func appendAggregatingNodeJSON(buf []byte, asn uint32, address netip.Addr) []byte {
	if !address.IsValid() {
		return nil
	}
	buf = append(buf, '"')
	buf = strconv.AppendUint(buf, uint64(asn), 10)
	buf = append(buf, ':')
	buf = address.AppendTo(buf)
	return append(buf, '"')
}
