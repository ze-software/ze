// Design: docs/plugin-development/protocol.md -- The Route Fields of filter-update
// Related: text_update.go -- appendIPv4PrefixesFromWire, the body NLRI writer it shares
// Related: text_json.go -- appendNLRIJSONValue, the per-NLRI value writer it shares

package format

import (
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// AppendAnnouncedNLRIJSON appends the NLRI an UPDATE announces as one JSON
// object keyed by family, each value the list of NLRI in the shapes the UPDATE
// event writes (docs/architecture/api/json-format.md, "NLRI Formats"): the
// body NLRI field under ipv4/unicast, then the MP_REACH_NLRI routes under
// their family, one key for the two when MP_REACH_NLRI is IPv4 unicast too.
// Withdrawn routes are not announced and are not written.
//
// ctx is the receive context of the UPDATE: ADD-PATH is negotiated per family,
// so it decides per family whether each NLRI carries a Path Identifier. A
// field that does not split into routes answers an error and buf as it was,
// never a partial list, which would read as the whole field.
func AppendAnnouncedNLRIJSON(buf, body []byte, mpReach wireu.MPReachWire, ctx *bgpctx.EncodingContext) ([]byte, error) {
	mark := len(buf)
	buf = append(buf, '{')
	ipv4Open := false
	if len(body) > 0 {
		buf = append(buf, `"`...)
		buf = family.IPv4Unicast.AppendTo(buf)
		buf = append(buf, `":[`...)
		var err error
		buf, err = appendIPv4PrefixesFromWire(buf, body, addPathFor(ctx, family.IPv4Unicast))
		if err != nil {
			return buf[:mark], err
		}
		ipv4Open = true
	}
	if mpReach == nil {
		if ipv4Open {
			buf = append(buf, ']')
		}
		return append(buf, '}'), nil
	}
	reachFamily := mpReach.Family()
	nlris, err := mpReach.NLRIs(addPathFor(ctx, reachFamily))
	if err != nil {
		return buf[:mark], err
	}
	// The body routes are already open under ipv4/unicast: an IPv4 unicast
	// MP_REACH_NLRI continues that list rather than stating the key twice,
	// which a reader decoding into a map would lose.
	first := true
	switch {
	case ipv4Open && reachFamily == family.IPv4Unicast:
		first = false
	case ipv4Open:
		buf = append(buf, `],"`...)
		buf = reachFamily.AppendTo(buf)
		buf = append(buf, `":[`...)
	default:
		buf = append(buf, '"')
		buf = reachFamily.AppendTo(buf)
		buf = append(buf, `":[`...)
	}
	for _, n := range nlris {
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = appendNLRIJSONValue(buf, n, reachFamily, false)
	}
	return append(buf, "]}"...), nil
}
