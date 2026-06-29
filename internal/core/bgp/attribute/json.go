// Design: docs/architecture/api/json-format.md -- attribute JSON rendering registry

package attribute

import (
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
