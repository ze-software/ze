package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC4271SentAttributesAscendEvenWithRawConfigAttributes proves the sender
// orders every path attribute by ascending type code, including the raw
// attributes a route carries from config.
//
// VALIDATES: BuildUnicast, BuildGroupedUnicast and BuildLabeledUnicast emit an
// attribute sequence whose type codes strictly ascend (MP_UNREACH_NLRI, which
// these builders never emit, is the one attribute OrderAttributes puts first),
// for a route with no raw attributes and for a route whose raw attributes
// (AIGP 26, an unknown optional transitive 20) sort before LARGE_COMMUNITY 32.
// PREVENTS: raw config attributes appended after the ordered block, so AIGP
// leaves after LARGE_COMMUNITY.
//
// RFC requirement: RFC4271-5-7 positive -- a route with ORIGIN, AS_PATH, NEXT_HOP, MED,
// COMMUNITIES and LARGE_COMMUNITY leaves from all three builders with its attribute
// type codes in ascending order.
// RFC requirement: RFC4271-5-7 negative -- raw attributes of type 20 and 26, supplied after
// the builder's own attributes, still leave in ascending order, before LARGE_COMMUNITY 32.
func TestRFC4271SentAttributesAscendEvenWithRawConfigAttributes(t *testing.T) {
	aigp := []byte{0x80, 26, 11, 1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 5}
	unknown := []byte{0xC0, 20, 2, 0xAB, 0xCD}
	plain := UnicastParams{
		Prefix:           netip.MustParsePrefix("192.0.2.0/24"),
		NextHop:          netip.MustParseAddr("198.51.100.1"),
		Origin:           attribute.OriginIGP,
		ASPath:           []uint32{65002},
		MED:              10,
		Communities:      []uint32{0xFDE90064},
		LargeCommunities: [][3]uint32{{65001, 1, 2}},
	}
	withRaw := plain
	withRaw.RawAttributeBytes = [][]byte{aigp, unknown}

	for _, params := range []UnicastParams{plain, withRaw} {
		raw := len(params.RawAttributeBytes)

		upd := NewUpdateBuilder(65001, false, true, false).BuildUnicast(&params)
		requireAscendingAttributes(t, upd.PathAttributes, raw, "BuildUnicast")

		var grouped []*Update
		err := NewUpdateBuilder(65001, false, true, false).BuildGroupedUnicast(
			[]UnicastParams{params}, 4096, func(u *Update) error {
				grouped = append(grouped, &Update{PathAttributes: append([]byte(nil), u.PathAttributes...)})
				return nil
			})
		require.NoError(t, err)
		require.Len(t, grouped, 1)
		requireAscendingAttributes(t, grouped[0].PathAttributes, raw, "BuildGroupedUnicast")

		labeled := &LabeledUnicastParams{
			Prefix:            params.Prefix,
			NextHop:           params.NextHop,
			Origin:            params.Origin,
			ASPath:            params.ASPath,
			MED:               params.MED,
			Communities:       params.Communities,
			LargeCommunities:  params.LargeCommunities,
			Labels:            []uint32{100},
			RawAttributeBytes: params.RawAttributeBytes,
		}
		upd = NewUpdateBuilder(65001, false, true, false).BuildLabeledUnicast(labeled)
		requireAscendingAttributes(t, upd.PathAttributes, raw, "BuildLabeledUnicast")
	}
}

// requireAscendingAttributes walks a path attribute blob and asserts strictly
// ascending type codes, LARGE_COMMUNITY present, and every raw attribute kept.
func requireAscendingAttributes(t *testing.T, pathAttrs []byte, raw int, builder string) {
	t.Helper()
	var codes []uint8
	for off := 0; off < len(pathAttrs); {
		require.GreaterOrEqual(t, len(pathAttrs)-off, 3, "%s: truncated header", builder)
		hdr := 3
		length := int(pathAttrs[off+2])
		if pathAttrs[off]&0x10 != 0 {
			hdr = 4
			length = int(pathAttrs[off+2])<<8 | int(pathAttrs[off+3])
		}
		codes = append(codes, pathAttrs[off+1])
		off += hdr + length
	}
	for i := 1; i < len(codes); i++ {
		require.Less(t, codes[i-1], codes[i], "%s: attribute codes %v", builder, codes)
	}
	require.Contains(t, codes, uint8(32), "%s: LARGE_COMMUNITY", builder)
	if raw > 0 {
		require.Contains(t, codes, uint8(20), "%s: raw type 20", builder)
		require.Contains(t, codes, uint8(26), "%s: raw AIGP", builder)
	}
}
