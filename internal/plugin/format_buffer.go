// Package plugin provides format functions that write directly from wire bytes.
// These functions avoid intermediate struct allocation by formatting directly
// from buffer data using iterators.
package plugin

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"
)

// Origin string constants for ORIGIN attribute formatting.
const (
	originIGP        = "igp"
	originEGP        = "egp"
	originIncomplete = "incomplete"
)

// FormatPrefixFromBytes formats an NLRI prefix from raw wire bytes.
// data format: [prefixLen (1 byte), prefix bytes (variable)]
// Returns empty string on invalid data.
func FormatPrefixFromBytes(data []byte, isIPv6 bool) string {
	if len(data) == 0 {
		return ""
	}

	prefixLen := int(data[0])
	prefixBytes := nlri.PrefixBytes(prefixLen)

	// Validate we have enough bytes
	if len(data) < 1+prefixBytes {
		return ""
	}

	// Build address from prefix bytes
	if isIPv6 {
		var ip [16]byte
		copy(ip[:], data[1:1+prefixBytes])
		addr := netip.AddrFrom16(ip)
		prefix, err := addr.Prefix(prefixLen)
		if err != nil {
			return ""
		}
		return prefix.String()
	}

	var ip [4]byte
	copy(ip[:], data[1:1+prefixBytes])
	addr := netip.AddrFrom4(ip)
	prefix, err := addr.Prefix(prefixLen)
	if err != nil {
		return ""
	}
	return prefix.String()
}

// FormatASPathJSON writes AS-PATH as JSON array to w.
// data is the raw AS_PATH attribute value (segments concatenated).
// asn4=true for 4-byte ASNs, false for 2-byte.
//
// Output format:
//   - AS_SEQUENCE: [asn1,asn2,asn3]
//   - AS_SET: [{asn1,asn2}]
//   - Mixed: [asn1,asn2,{asn3,asn4}]
func FormatASPathJSON(data []byte, asn4 bool, w io.Writer) error {
	if len(data) == 0 {
		_, err := w.Write([]byte("[]"))
		return err
	}

	iter := attribute.NewASPathIterator(data, asn4)

	_, _ = w.Write([]byte("["))
	first := true

	for segType, asns, ok := iter.Next(); ok; segType, asns, ok = iter.Next() {
		asnIter := attribute.NewASNIterator(asns, asn4)

		// Handle AS_SET (type 1) with braces
		if segType == attribute.ASSet {
			if !first {
				_, _ = w.Write([]byte(","))
			}
			_, _ = w.Write([]byte("{"))
			firstASN := true
			for asn, ok := asnIter.Next(); ok; asn, ok = asnIter.Next() {
				if !firstASN {
					_, _ = w.Write([]byte(","))
				}
				_, _ = fmt.Fprintf(w, "%d", asn)
				firstASN = false
			}
			_, _ = w.Write([]byte("}"))
			first = false
			continue
		}

		// AS_SEQUENCE (type 2) - just comma-separated
		for asn, ok := asnIter.Next(); ok; asn, ok = asnIter.Next() {
			if !first {
				_, _ = w.Write([]byte(","))
			}
			_, _ = fmt.Fprintf(w, "%d", asn)
			first = false
		}
	}

	_, err := w.Write([]byte("]"))
	return err
}

// FormatCommunitiesJSON writes COMMUNITIES as JSON array of strings to w.
// data is the raw COMMUNITIES attribute value (4 bytes per community).
//
// Output format: ["65001:100","65002:200"]
// Well-known communities use names: "no-export", "no-advertise", etc.
func FormatCommunitiesJSON(data []byte, w io.Writer) error {
	if len(data) == 0 || len(data)%4 != 0 {
		_, err := w.Write([]byte("[]"))
		return err
	}

	_, _ = w.Write([]byte("["))

	numComms := len(data) / 4
	for i := 0; i < numComms; i++ {
		if i > 0 {
			_, _ = w.Write([]byte(","))
		}

		comm := binary.BigEndian.Uint32(data[i*4:])
		name := wellKnownCommunityName(comm)
		if name != "" {
			_, _ = fmt.Fprintf(w, `"%s"`, name)
		} else {
			high := comm >> 16
			low := comm & 0xFFFF
			_, _ = fmt.Fprintf(w, `"%d:%d"`, high, low)
		}
	}

	_, err := w.Write([]byte("]"))
	return err
}

// wellKnownCommunityName returns the name for well-known communities.
// RFC 1997 and related RFCs define these values.
func wellKnownCommunityName(comm uint32) string {
	switch comm {
	case 0xFFFFFF01:
		return "no-export"
	case 0xFFFFFF02:
		return "no-advertise"
	case 0xFFFFFF03:
		return "no-export-subconfed"
	case 0xFFFFFF04:
		return "nopeer" // RFC 3765
	case 0xFFFF0000:
		return "graceful-shutdown" // RFC 8326
	case 0xFFFF0001:
		return "accept-own" // RFC 7611
	case 0xFFFF0002:
		return "route-filter-translated-v4" // draft
	case 0xFFFF0003:
		return "route-filter-v4" // draft
	case 0xFFFF0004:
		return "route-filter-translated-v6" // draft
	case 0xFFFF0005:
		return "route-filter-v6" // draft
	case 0xFFFF0006:
		return "llgr-stale" // RFC draft
	case 0xFFFF0007:
		return "no-llgr" // RFC draft
	case 0xFFFF029A:
		return "blackhole" // RFC 7999
	default:
		return ""
	}
}

// FormatOriginJSON writes the ORIGIN attribute value as JSON string.
// value is the single-byte origin code.
func FormatOriginJSON(value byte, w io.Writer) {
	var origin string
	switch value {
	case 0:
		origin = originIGP
	case 1:
		origin = originEGP
	case 2:
		origin = originIncomplete
	default:
		origin = "unknown"
	}
	_, _ = fmt.Fprintf(w, `"%s"`, origin)
}

// FormatMEDJSON writes the MED attribute value as JSON number.
// data is the 4-byte MED value.
func FormatMEDJSON(data []byte, w io.Writer) {
	if len(data) < 4 {
		_, _ = w.Write([]byte("0"))
		return
	}
	med := binary.BigEndian.Uint32(data)
	_, _ = fmt.Fprintf(w, "%d", med)
}

// FormatLocalPrefJSON writes the LOCAL_PREF attribute value as JSON number.
// data is the 4-byte local preference value.
func FormatLocalPrefJSON(data []byte, w io.Writer) {
	if len(data) < 4 {
		_, _ = w.Write([]byte("0"))
		return
	}
	pref := binary.BigEndian.Uint32(data)
	_, _ = fmt.Fprintf(w, "%d", pref)
}
