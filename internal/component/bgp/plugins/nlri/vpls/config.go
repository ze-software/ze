// Design: docs/architecture/config/syntax.md -- VPLS config route parsing
// RFC: rfc/short/rfc4761.md -- VPLS NLRI wire format (L2VPN AFI 25, SAFI 65)
// Related: encode.go -- VPLS NLRI encoder (EncodeNLRIHex)

package vpls

import (
	"encoding/hex"
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

var errMissingVplsOperation = errors.New("missing operation keyword (add/del/eor) for family l2vpn/vpls")

// VPLS path attribute wire constants.
const (
	attrCodeOrigin       uint8 = 1  // ORIGIN (RFC 4271).
	attrCodeMED          uint8 = 4  // MULTI_EXIT_DISC (RFC 4271).
	attrCodeCommunity    uint8 = 8  // COMMUNITIES (RFC 1997).
	attrCodeOriginatorID uint8 = 9  // ORIGINATOR_ID (RFC 4456).
	attrCodeClusterList  uint8 = 10 // CLUSTER_LIST (RFC 4456).
	attrCodeExtComm      uint8 = 16 // EXTENDED_COMMUNITIES (RFC 4360).

	flagWellKnownTrans = 0x40 // Well-known transitive (ORIGIN).
	flagOptional       = 0x80 // Optional non-transitive (MED, ORIGINATOR_ID, CLUSTER_LIST).
	flagOptTrans       = 0xC0 // Optional transitive (COMMUNITIES, EXT_COMMUNITIES).
)

// VPLS NLRI argument keys (forwarded to EncodeNLRIHex).
const (
	keyRD            = "rd"
	keyVEID          = "ve-id"
	keyVEBlockOffset = "ve-block-offset"
	keyVEBlockSize   = "ve-block-size"
	keyLabelBase     = "label-base"
)

// parseConfigRoute implements registry.InProcessConfigRouteParser for VPLS.
// It builds the RFC 4761 VPLS NLRI from the content tokens and assembles the
// generic path attributes from the pre-parsed attribute block. AS_PATH and
// LOCAL_PREF are carried through typed (built by BuildPlugin with session
// context); ORIGIN/MED/COMMUNITIES/ORIGINATOR_ID/CLUSTER_LIST/EXT_COMMUNITIES
// are pre-built here so VPLS owns its wire encoding (community / ext-community
// sort order).
func parseConfigRoute(req registry.ConfigRouteRequest) (registry.PluginRoute, error) {
	args, err := vplsArgsFromContent(req.Content)
	if err != nil {
		return registry.PluginRoute{}, err
	}

	nlriHex, err := EncodeNLRIHex(familyVPLS, args)
	if err != nil {
		return registry.PluginRoute{}, fmt.Errorf("build VPLS NLRI: %w", err)
	}
	nlri, err := hex.DecodeString(nlriHex)
	if err != nil {
		return registry.PluginRoute{}, fmt.Errorf("decode VPLS NLRI hex: %w", err)
	}

	var attrs []registry.PluginRouteAttr

	// ORIGIN (code 1): always present (matches BuildVPLS), value from config.
	attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeOrigin, Flags: flagWellKnownTrans, Value: []byte{req.Origin}})

	// MED (code 4): only when > 0.
	if req.MED > 0 {
		attrs = append(attrs, registry.PluginRouteAttr{
			Code: attrCodeMED, Flags: flagOptional,
			Value: []byte{byte(req.MED >> 24), byte(req.MED >> 16), byte(req.MED >> 8), byte(req.MED)},
		})
	}

	// COMMUNITIES (code 8): sorted ascending (RFC 1997 / BuildVPLS convention).
	if len(req.Community) > 0 {
		sorted := slices.Clone(req.Community)
		slices.Sort(sorted)
		val := make([]byte, 0, 4*len(sorted))
		for _, c := range sorted {
			val = append(val, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
		}
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeCommunity, Flags: flagOptTrans, Value: val})
	}

	// ORIGINATOR_ID (code 9): 4-byte router-id.
	if req.OriginatorID != 0 {
		o := req.OriginatorID
		attrs = append(attrs, registry.PluginRouteAttr{
			Code: attrCodeOriginatorID, Flags: flagOptional,
			Value: []byte{byte(o >> 24), byte(o >> 16), byte(o >> 8), byte(o)},
		})
	}

	// CLUSTER_LIST (code 10): N x 4-byte cluster IDs (order preserved, RFC 4456).
	if len(req.ClusterList) > 0 {
		val := make([]byte, 0, 4*len(req.ClusterList))
		for _, c := range req.ClusterList {
			val = append(val, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
		}
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeClusterList, Flags: flagOptional, Value: val})
	}

	// EXTENDED_COMMUNITIES (code 16): sorted by type for RFC 4360 compliance.
	if len(req.ExtCommunity) > 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeExtComm, Flags: flagOptTrans, Value: sortExtCommunities(req.ExtCommunity)})
	}

	return registry.PluginRoute{
		IsIPv6:          req.IsIPv6,
		NLRI:            nlri,
		NextHop:         req.NextHop,
		Attrs:           attrs,
		ASPath:          req.ASPath,
		LocalPreference: req.LocalPreference,
	}, nil
}

// vplsArgsFromContent converts NLRI content tokens into the key-value args
// EncodeNLRIHex expects. The VPLS grammar carries the RD before the operation
// keyword: "rd RD add ve-id N ve-block-offset N ve-block-size N label-base N".
// Operation keywords (add/del/eor) may appear anywhere and are skipped; every
// other token is a key followed by its value.
func vplsArgsFromContent(content []string) ([]string, error) {
	var rd, veID, offset, size, base string
	sawOp := false
	for i := 0; i < len(content); {
		tok := content[i]
		if tok == "add" || tok == "del" || tok == "eor" {
			sawOp = true
			i++
			continue
		}
		if i+1 >= len(content) {
			break
		}
		key, val := content[i], content[i+1]
		switch key {
		case keyRD:
			rd = val
		case keyVEID, "endpoint":
			veID = val
		case keyVEBlockOffset, "offset":
			offset = val
		case keyVEBlockSize, "size":
			size = val
		case keyLabelBase, "base":
			base = val
		}
		i += 2
	}

	if !sawOp {
		return nil, errMissingVplsOperation
	}
	if rd == "" {
		return nil, errRdRequiredForVpls
	}

	args := []string{keyRD, rd}
	if veID != "" {
		args = append(args, keyVEID, veID)
	}
	if offset != "" {
		args = append(args, keyVEBlockOffset, offset)
	}
	if size != "" {
		args = append(args, keyVEBlockSize, size)
	}
	if base != "" {
		args = append(args, keyLabelBase, base)
	}
	return args, nil
}

// sortExtCommunities sorts extended communities (8 bytes each) by their 64-bit
// value for RFC 4360 compliance (lower type codes first). Trailing bytes that do
// not form a complete community are discarded.
func sortExtCommunities(data []byte) []byte {
	count := len(data) / 8
	if count < 2 {
		return data
	}
	if count*8 != len(data) {
		data = data[:count*8]
	}
	values := make([]uint64, count)
	for i := range count {
		o := i * 8
		values[i] = uint64(data[o])<<56 | uint64(data[o+1])<<48 | uint64(data[o+2])<<40 |
			uint64(data[o+3])<<32 | uint64(data[o+4])<<24 | uint64(data[o+5])<<16 |
			uint64(data[o+6])<<8 | uint64(data[o+7])
	}
	slices.Sort(values)
	out := make([]byte, len(data))
	for i, v := range values {
		o := i * 8
		out[o] = byte(v >> 56)
		out[o+1] = byte(v >> 48)
		out[o+2] = byte(v >> 40)
		out[o+3] = byte(v >> 32)
		out[o+4] = byte(v >> 24)
		out[o+5] = byte(v >> 16)
		out[o+6] = byte(v >> 8)
		out[o+7] = byte(v)
	}
	return out
}
