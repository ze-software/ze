// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin
// RFC: rfc/short/rfc8277.md

package labeled

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errTruncatedLabeledUnicastNlri         = errors.New("truncated labeled unicast NLRI")
	errTruncatedPrefixInLabeledUnicastNlri = errors.New("truncated prefix in labeled unicast NLRI")
	errPrefixRequiresValue                 = errors.New("prefix requires value")
	errLabelRequiresValue                  = errors.New("label requires value")
	errPathIdRequiresValue                 = errors.New("path-id requires value")
	errPrefixRequiredForLabeledUnicast     = errors.New("prefix required for labeled unicast")
	errLabelRequiredForLabeledUnicast      = errors.New("label required for labeled unicast")
	errMissingRouteCommand                 = errors.New("missing route command")
)

// DecodeNLRIHex decodes labeled unicast NLRI from hex and returns a data structure.
// This implements the InProcessNLRIDecoder signature for the plugin registry.
//
// Wire formats (RFC 8277 Sections 2.2 and 2.4), after an optional Path Identifier:
//
//	Offset 0: [length:1][labels:3*N][prefix:ceil(prefixBits/8)]  announcement
//	Offset 0: [length:1][Compatibility:3][prefix:ceil(prefixBits/8)] withdrawal
//	Offset 1: the label stack or Compatibility field starts here.
//
// Output is one map for a singleton section, otherwise an array of maps.
// addPath and withdraw describe the enclosing message, never the NLRI bytes.
func DecodeNLRIHex(famName, hexStr string, addPath, withdraw bool) (any, error) {
	fam, ok := family.LookupFamily(famName)
	if !ok {
		return nil, fmt.Errorf("unknown family: %s", famName)
	}
	if fam.SAFI != SAFIMPLSLabel {
		return nil, fmt.Errorf("unsupported family for labeled unicast: %s", famName)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	var routes [][]byte
	if withdraw {
		// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility
		// field MUST be ignored." Frame by length, not by label-stack bits.
		routes, err = nlrisplit.SplitWithdrawn(fam, data, addPath)
	} else {
		routes, err = nlrisplit.Split(fam, data, addPath)
	}
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, errTruncatedLabeledUnicastNlri
	}
	if len(routes) == 1 {
		// RFC 8277 Sections 2.2 and 2.4.
		return decodeLabeledNLRI(fam, routes[0], addPath, withdraw)
	}
	results := make([]map[string]any, len(routes))
	for index, raw := range routes {
		// RFC 8277 Sections 2.2 and 2.4.
		results[index], err = decodeLabeledNLRI(fam, raw, addPath, withdraw)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// decodeLabeledNLRI decodes one framed route without modifying its native bytes.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." Only announcements interpret the label entries.
func decodeLabeledNLRI(fam family.Family, data []byte, addPath, withdraw bool) (map[string]any, error) {
	// RFC 7911 Section 3: consume the negotiated four-octet Path Identifier.
	pathID, data, err := nlri.SplitPathID(data, addPath)
	if err != nil {
		return nil, err
	}

	if len(data) < 4 {
		return nil, errTruncatedLabeledUnicastNlri
	}
	var entries []uint32
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var prefixBits, pos int
	if withdraw {
		// RFC 8277 Section 2.4: skip exactly three Compatibility octets.
		data, err = nlrisplit.RouteCIDR(fam, data, scratch[:], true)
		if err != nil {
			return nil, err
		}
		prefixBits = int(data[0])
		pos = 1
	} else {
		// RFC 8277 Section 2.1: retain every full entry, including TC and S.
		entries, _, err = nlri.ParseLabelStack(data[1:])
		if err != nil {
			return nil, errTruncatedLabeledUnicastNlri
		}
		pos = 1 + len(entries)*3
		prefixBits = int(data[0]) - len(entries)*24
	}
	if prefixBits < 0 {
		return nil, fmt.Errorf("invalid labeled unicast prefix length: %d", prefixBits)
	}

	prefixBytes := nlri.PrefixBytes(prefixBits)
	if pos+prefixBytes > len(data) {
		return nil, errTruncatedPrefixInLabeledUnicastNlri
	}

	var addr netip.Addr
	if fam.AFI == AFIIPv4 {
		if prefixBits > 32 {
			return nil, fmt.Errorf("invalid IPv4 labeled unicast prefix length: %d", prefixBits)
		}
		var b [4]byte
		copy(b[:], data[pos:pos+prefixBytes])
		addr = netip.AddrFrom4(b)
	} else {
		if prefixBits > 128 {
			return nil, fmt.Errorf("invalid IPv6 labeled unicast prefix length: %d", prefixBits)
		}
		var b [16]byte
		copy(b[:], data[pos:pos+prefixBytes])
		addr = netip.AddrFrom16(b)
	}
	prefix := netip.PrefixFrom(addr, prefixBits)

	result := map[string]any{
		"prefix": prefix.String(),
	}
	if len(entries) > 0 {
		result["labels"] = labelPairs(entries)
	}
	if addPath {
		result["path-id"] = pathID
	}
	return result, nil
}

// labelPairs states each stack entry as the pair [label, entry].
//
// The label is what a reader matches on and the entry is the three octets it
// came from, carrying the traffic class and the bottom-of-stack bit. Writing
// the label alone left no way to tell a one-label stack from the first entry of
// a longer one. The entry is dropped when it is zero, the one case where it
// says nothing the label did not.
func labelPairs(entries []uint32) [][]uint32 {
	pairs := make([][]uint32, len(entries))
	for index, entry := range entries {
		pairs[index] = []uint32{nlri.LabelValue(entry)}
		if entry != 0 {
			pairs[index] = append(pairs[index], entry)
		}
	}
	return pairs
}

// EncodeNLRIHex encodes labeled unicast NLRI from CLI-style args and returns uppercase hex.
// Args format: ["prefix", "10.0.0.0/24", "label", "100", "path-id", "1"]
// This implements the InProcessNLRIEncoder signature for the plugin registry.
func EncodeNLRIHex(famName string, args []string) (string, error) {
	fam, ok := family.LookupFamily(famName)
	if !ok {
		return "", fmt.Errorf("unknown family: %s", famName)
	}
	if fam.SAFI != SAFIMPLSLabel {
		return "", fmt.Errorf("unsupported family for labeled unicast: %s", famName)
	}

	var prefix netip.Prefix
	var labels []uint32
	var pathID uint32
	var hasPathID bool
	var hasPrefix bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "prefix":
			i++
			if i >= len(args) {
				return "", errPrefixRequiresValue
			}
			p, err := netip.ParsePrefix(args[i])
			if err != nil {
				return "", fmt.Errorf("invalid prefix: %w", err)
			}
			prefix = p
			hasPrefix = true
		case "label":
			i++
			if i >= len(args) {
				return "", errLabelRequiresValue
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid label: %w", err)
			}
			labels = append(labels, uint32(v)) //nolint:gosec // validated by ParseUint with bitSize 32
		case "path-id":
			i++
			if i >= len(args) {
				return "", errPathIdRequiresValue
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid path-id: %w", err)
			}
			pathID = uint32(v) //nolint:gosec // validated by ParseUint with bitSize 32
			// RFC 7911 Section 3 reserves no Path Identifier value, so the
			// keyword is what says one was given. `path-id 0` is a route with
			// identifier zero, not a route without one.
			hasPathID = true
		default:
			return "", fmt.Errorf("unknown labeled unicast keyword: %s", args[i])
		}
	}

	if !hasPrefix {
		return "", errPrefixRequiredForLabeledUnicast
	}
	if len(labels) == 0 {
		return "", errLabelRequiredForLabeledUnicast
	}

	n := NewLabeledUnicast(fam, prefix, labels, pathID, hasPathID)
	nlriBytes := n.Bytes()

	return textbuf.StringHexUpper(nlriBytes), nil
}

// EncodeRoute encodes a labeled unicast (nlri-mpls) route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, famName string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	isIPv6 := strings.HasPrefix(famName, "ipv6/")
	ub := message.GetUpdateBuilder(localAS, isIBGP, asn4, addPath)
	defer message.PutUpdateBuilder(ub)

	// Parse route command - expects "<prefix> next-hop <addr> label <label> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, errMissingRouteCommand
	}

	// Parse using API parser
	parsed, err := route.ParseLabeledUnicastAttributes(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert to LabeledUnicastParams
	params := labeledUnicastRouteToParams(parsed)

	// Build UPDATE
	update := ub.BuildLabeledUnicast(&params)
	if update == nil {
		return nil, nil, message.ErrUnicastNextHopUnusable
	}

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// Build NLRI for -n flag
	var fam Family
	if isIPv6 {
		fam = Family{AFI: AFIIPv6, SAFI: SAFIMPLSLabel}
	} else {
		fam = Family{AFI: AFIIPv4, SAFI: SAFIMPLSLabel}
	}
	// Carry the full label stack into the NLRI (a labeled-unicast prefix may
	// have several labels). Defaulting to a single 0 label keeps a label-less
	// input encodable rather than producing an empty stack.
	labels := parsed.Labels
	if len(labels) == 0 {
		labels = []uint32{0}
	}
	// addPath is the session's ADD-PATH negotiation, which is what decides
	// whether this NLRI carries a Path Identifier at all (RFC 7911 Section 3).
	labeledNLRI := NewLabeledUnicast(fam, parsed.Prefix, labels, parsed.PathID, addPath)
	nlriBytes := labeledNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// labeledUnicastRouteToParams converts LabeledUnicastRoute to LabeledUnicastParams.
func labeledUnicastRouteToParams(r bgptypes.LabeledUnicastRoute) message.LabeledUnicastParams {
	attrs := message.ExtractAttrsFromWire(r.Wire)

	p := message.LabeledUnicastParams{
		Prefix:            r.Prefix,
		NextHop:           r.NextHop,
		PathID:            r.PathID,
		Origin:            attrs.Origin,
		LocalPreference:   attrs.LocalPreference,
		MED:               attrs.MED,
		ASPath:            attrs.ASPath,
		Communities:       attrs.Communities,
		LargeCommunities:  attrs.LargeCommunities,
		ExtCommunityBytes: attrs.ExtCommunityBytes,
	}

	// Labels (copy from route)
	p.Labels = r.Labels

	return p
}
