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
// Wire format (RFC 8277 Section 2.2): [length_byte][label_stack (3*N bytes)][prefix_bytes].
// Output: map with "prefix" and "labels" keys.
func DecodeNLRIHex(famName, hexStr string) (any, error) {
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

	if len(data) < 4 { // minimum: 1 length + 3 label bytes
		return nil, errTruncatedLabeledUnicastNlri
	}

	totalBits := int(data[0])

	// Parse label stack: each label is 3 bytes (20-bit label + 3-bit TC + 1-bit S)
	pos := 1
	var labels []uint32
	for pos+3 <= len(data) {
		label := uint32(data[pos])<<12 | uint32(data[pos+1])<<4 | uint32(data[pos+2])>>4
		bos := data[pos+2] & 0x01
		labels = append(labels, label)
		pos += 3
		if bos == 1 {
			break
		}
	}

	// Parse prefix
	prefixBits := totalBits - len(labels)*24
	if prefixBits < 0 {
		return nil, fmt.Errorf("invalid labeled unicast: totalBits=%d labels=%d", totalBits, len(labels))
	}

	prefixBytes := nlri.PrefixBytes(prefixBits)
	if pos+prefixBytes > len(data) {
		return nil, errTruncatedPrefixInLabeledUnicastNlri
	}

	var addr netip.Addr
	if fam.AFI == AFIIPv4 {
		var b [4]byte
		copy(b[:], data[pos:pos+prefixBytes])
		addr = netip.AddrFrom4(b)
	} else {
		var b [16]byte
		copy(b[:], data[pos:pos+prefixBytes])
		addr = netip.AddrFrom16(b)
	}
	prefix := netip.PrefixFrom(addr, prefixBits)

	result := map[string]any{
		"prefix": prefix.String(),
	}
	if len(labels) > 0 {
		result["labels"] = labels
	}
	return result, nil
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

	n := NewLabeledUnicast(fam, prefix, labels, pathID)
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
	labeledNLRI := NewLabeledUnicast(fam, parsed.Prefix, labels, parsed.PathID)
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
