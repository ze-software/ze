// Design: docs/architecture/wire/nlri.md — route target constraint plugin
// RFC: rfc/short/rfc4684.md
// Related: types.go — the RTC NLRI this encoder builds and writes

package rtc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The words of a Route Target membership, as `update text` and ExaBGP's
// `announce ipv4 rtc` write them.
const (
	keyOriginAS    = "origin-as"
	keyRouteTarget = "route-target"
	keyDefault     = "default"
)

var (
	errRTCNeedsMembership     = errors.New("rtc needs `default`, or `origin-as <asn> route-target <asn>:<n>|<ip>:<n>`")
	errRTCNeedsOriginAS       = errors.New("rtc route-target needs origin-as <asn> beside it")
	errRTCNeedsRouteTarget    = errors.New("rtc origin-as needs route-target <asn>:<n>|<ip>:<n> beside it")
	errRTCDefaultTakesNoValue = errors.New("rtc default is the zero-length membership and takes no origin-as or route-target")
	errRTCValueMissing        = errors.New("rtc keyword needs a value")
	errRTCUnknownWord         = errors.New("rtc takes only default, origin-as, route-target")
	errRTCOriginAS            = errors.New("rtc origin-as is not an AS number from 0 to 4294967295")
	errRTCRouteTarget         = errors.New("rtc route-target is not a route target: write <asn>:<n> or <ip>:<n>")
)

// rtcPrefixBytes is the NLRI length past the prefix-length octet of a
// non-default membership: four octets of origin AS and eight of route target.
const rtcPrefixBytes = 12

// EncodeNLRIHex encodes one Route Target membership NLRI from its text and
// answers uppercase hex. It implements the InProcessNLRIEncoder signature.
//
// Args: ["default"], or ["origin-as", "65000", "route-target", "65000:1"].
//
// RFC 4684 Section 4: "The NLRI field in the MP_REACH_NLRI and MP_UNREACH_NLRI
// is a prefix of 0 to 96 bits, encoded as defined in Section 4 of [5]."
//
//	+-------------------------------+
//	| length (1 octet): 96, or 0    |  offset 0
//	+-------------------------------+
//	| origin as        (4 octets)   |  offset 1
//	+-------------------------------+
//	| route target     (8 octets)   |  offset 5: type (2), value (6)
//	+-------------------------------+
//
// RFC 4684 Section 4: "Except for the default route target, which is encoded
// as a zero-length prefix, the minimum prefix length is 32 bits." The encoder
// writes the whole 96 bits or the zero-length default, never a partial prefix.
func EncodeNLRIHex(family string, args []string) (string, error) {
	if family != familyIPv4RTC {
		return "", fmt.Errorf("unsupported family for RTC: %s", family)
	}
	membership, err := parseMembership(args)
	if err != nil {
		return "", err
	}
	var buf [1 + rtcPrefixBytes]byte
	n := membership.WriteTo(buf[:], 0)
	return textbuf.StringHexUpper(buf[:n]), nil
}

// parseMembership reads the membership words into an RTC NLRI. A membership
// names both fields or neither: a missing one is refused, never written as zero.
func parseMembership(args []string) (*RTC, error) {
	if len(args) == 0 {
		return nil, errRTCNeedsMembership
	}
	if strings.EqualFold(args[0], keyDefault) {
		if len(args) > 1 {
			return nil, errRTCDefaultTakesNoValue
		}
		return &RTC{}, nil
	}

	var originAS uint32
	var target routeTarget
	var hasOriginAS, hasTarget bool
	for i := 0; i < len(args); i += 2 {
		key := strings.ToLower(args[i])
		if key == keyDefault {
			return nil, errRTCDefaultTakesNoValue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("%w: %s", errRTCValueMissing, key)
		}
		value := args[i+1]
		switch key {
		case keyOriginAS:
			asn, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("%w: %q", errRTCOriginAS, value)
			}
			originAS = uint32(asn)
			hasOriginAS = true
		case keyRouteTarget:
			parsed, err := parseRouteTarget(value)
			if err != nil {
				return nil, err
			}
			target = parsed
			hasTarget = true
		default:
			return nil, fmt.Errorf("%w: %q", errRTCUnknownWord, args[i])
		}
	}
	if !hasOriginAS {
		return nil, errRTCNeedsOriginAS
	}
	if !hasTarget {
		return nil, errRTCNeedsRouteTarget
	}
	return newRTC(originAS, target), nil
}

// parseRouteTarget reads `<asn>:<n>`, `<ip>:<n>` or `target:...` through the
// extended community parser `update text` uses for `extended-community`, so the
// two accept the same spellings, and refuses any community that is not a Route
// Target.
//
// RFC 4360 Section 4: "The value of the high-order octet of the Type field for
// the Route Target Community can be 0x00, 0x01, or 0x02. The value of the
// low-order octet of the Type field for this community is 0x02." Both octets
// are checked.
func parseRouteTarget(value string) (routeTarget, error) {
	spelling := value
	if !strings.HasPrefix(strings.ToLower(value), "target:") {
		spelling = "target:" + value
	}
	communities, _, err := route.ParseExtendedCommunities([]string{spelling})
	if err != nil {
		return routeTarget{}, fmt.Errorf("%w: %q: %w", errRTCRouteTarget, value, err)
	}
	if len(communities) != 1 {
		return routeTarget{}, fmt.Errorf("%w: %q", errRTCRouteTarget, value)
	}
	community := communities[0]
	if community[0] > 0x02 {
		return routeTarget{}, fmt.Errorf("%w: %q", errRTCRouteTarget, value)
	}
	if community[1] != 0x02 {
		return routeTarget{}, fmt.Errorf("%w: %q", errRTCRouteTarget, value)
	}
	rt := routeTarget{Type: binary.BigEndian.Uint16(community[:2])}
	copy(rt.Value[:], community[2:])
	return rt, nil
}
