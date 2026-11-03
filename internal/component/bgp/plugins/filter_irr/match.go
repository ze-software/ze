// Design: docs/architecture/bgp/filter-irr.md -- prefix matching for IRR-derived filter lists
// Related: filter_irr.go -- handleFilterUpdate, the verdict built from what this file reads

package filter_irr

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type prefixEntry struct {
	prefix netip.Prefix
	ge     uint8
	le     uint8
}

type irrPrefixList struct {
	entries []prefixEntry
}

func evaluatePrefix(entries []prefixEntry, route netip.Prefix) bool {
	routeBits := uint8(route.Bits())
	routeIs4 := route.Addr().Is4()

	for i := range entries {
		e := &entries[i]
		if e.prefix.Addr().Is4() != routeIs4 {
			continue
		}
		if routeBits < e.ge || routeBits > e.le {
			continue
		}
		if !e.prefix.Contains(route.Addr()) {
			continue
		}
		if routeBits < uint8(e.prefix.Bits()) {
			continue
		}
		return true
	}
	return false
}

func findMatchingEntry(entries []prefixEntry, route netip.Prefix) string {
	routeBits := uint8(route.Bits())
	routeIs4 := route.Addr().Is4()

	for i := range entries {
		e := &entries[i]
		if e.prefix.Addr().Is4() != routeIs4 {
			continue
		}
		if routeBits < e.ge || routeBits > e.le {
			continue
		}
		if !e.prefix.Contains(route.Addr()) {
			continue
		}
		if routeBits < uint8(e.prefix.Bits()) {
			continue
		}
		return e.prefix.String()
	}
	return ""
}

// The route fields the filter reads (filterDecls): the NLRI the UPDATE
// announces, which the list judges, and the NLRI it withdraws, which decide
// whether a route that announces nothing the list admits still has
// withdrawals to keep (docs/plugin-development/protocol.md, "The Route Fields
// of filter-update").
const (
	fieldPrefix    = "prefix"
	fieldWithdrawn = "withdrawn"
)

// errNLRIValue is answered for an NLRI element of the prefix or withdrawn
// field that names no prefix.
var errNLRIValue = errors.New("an NLRI element names no prefix")

// familyNLRI is the NLRI of one family that one UPDATE announces or
// withdraws, as the prefix or withdrawn field carries them. prefixes holds
// each NLRI's prefix. For an announced family, values holds each NLRI in the
// field's shape, the value a prefix remove names, and admitted marks the
// ones the ASN's list authorizes. A withdrawn family is never judged, so its
// values and admitted are nil.
type familyNLRI struct {
	family   string
	withdraw bool
	prefixes []netip.Prefix
	values   []json.RawMessage
	admitted []bool
}

// routeNLRI is the judged NLRI of one UPDATE. Only a CIDR family
// (family.Family.PlainCIDR: unicast, multicast, labeled unicast) is judged and
// listed in families; the NLRI of every other family passes as received.
// accepted and rejected count the judged announced NLRI the list does and
// does not authorize; they decide the verdict. withdraws is set when the
// UPDATE withdraws any NLRI, of any family.
type routeNLRI struct {
	families  []familyNLRI
	accepted  int
	rejected  int
	withdraws bool
}

// judgeNLRI reads the prefix and withdrawn fields and judges each announced
// NLRI of a CIDR family against list; a nil list, an ASN whose list is not
// here yet, admits nothing. Families are visited in name order, so one route
// always answers the same removes in the same order. A field that is not
// the shape the engine writes answers an error, never a partial judgment.
func judgeNLRI(fields []byte, list *irrPrefixList) (routeNLRI, error) {
	var out routeNLRI
	if len(fields) == 0 {
		return out, nil
	}
	var decoded struct {
		Prefix    map[string][]json.RawMessage `json:"prefix"`
		Withdrawn map[string][]json.RawMessage `json:"withdrawn"`
	}
	if err := json.Unmarshal(fields, &decoded); err != nil {
		return out, fmt.Errorf("the route fields do not decode: %w", err)
	}
	for _, name := range slices.Sorted(maps.Keys(decoded.Withdrawn)) {
		values := decoded.Withdrawn[name]
		if len(values) > 0 {
			out.withdraws = true
		}
		if !judgedFamily(name) {
			continue
		}
		block := familyNLRI{family: name, withdraw: true, prefixes: make([]netip.Prefix, len(values))}
		for i, value := range values {
			prefix, err := nlriPrefix(value)
			if err != nil {
				return routeNLRI{}, fmt.Errorf("withdrawn %s element %d: %w", name, i+1, err)
			}
			block.prefixes[i] = prefix
		}
		out.families = append(out.families, block)
	}
	for _, name := range slices.Sorted(maps.Keys(decoded.Prefix)) {
		if !judgedFamily(name) {
			continue
		}
		values := decoded.Prefix[name]
		block := familyNLRI{family: name, prefixes: make([]netip.Prefix, len(values)), values: values,
			admitted: make([]bool, len(values))}
		for i, value := range values {
			prefix, err := nlriPrefix(value)
			if err != nil {
				return routeNLRI{}, fmt.Errorf("prefix %s element %d: %w", name, i+1, err)
			}
			block.prefixes[i] = prefix
			if list != nil && evaluatePrefix(list.entries, prefix) {
				block.admitted[i] = true
				out.accepted++
				continue
			}
			out.rejected++
		}
		out.families = append(out.families, block)
	}
	return out, nil
}

// judgedFamily reports whether the IRR list judges the NLRI of the family
// named name: a CIDR family this process knows. Any other family's NLRI is
// not a prefix the list can authorize, and passes unjudged.
func judgedFamily(name string) bool {
	fam, known := family.LookupFamily(name)
	return known && fam.PlainCIDR()
}

// nlriPrefix reads the prefix of one NLRI element in the event's NLRI shape:
// the prefix string, or an object whose "prefix" member is that string, as
// under ADD-PATH ({"prefix":...,"path-id":...}) and for labeled unicast
// ({"labels":...,"prefix":...}).
func nlriPrefix(value json.RawMessage) (netip.Prefix, error) {
	var text string
	if len(value) > 0 && value[0] == '"' {
		if err := json.Unmarshal(value, &text); err != nil {
			return netip.Prefix{}, errNLRIValue
		}
	} else {
		var shaped struct {
			Prefix *string `json:"prefix"`
		}
		if err := json.Unmarshal(value, &shaped); err != nil {
			return netip.Prefix{}, errNLRIValue
		}
		if shaped.Prefix == nil {
			return netip.Prefix{}, errNLRIValue
		}
		text = *shaped.Prefix
	}
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("%q is not a prefix", text)
	}
	return prefix, nil
}

// rejectedRemoves answers one prefix remove for each announced NLRI the
// list does not authorize, each naming the NLRI in the shape the prefix
// field carried it, so the engine drops exactly those entries and keeps the
// rest of the UPDATE (policychain applyPrefixIntent).
func (n *routeNLRI) rejectedRemoves() []rpc.FilterSet {
	removes := make([]rpc.FilterSet, 0, n.rejected)
	for i := range n.families {
		block := &n.families[i]
		for j, admitted := range block.admitted {
			if admitted {
				continue
			}
			removes = append(removes, rpc.FilterSet{Field: fieldPrefix, Action: rpc.FilterSetActionRemove,
				Value: block.values[j]})
		}
	}
	return removes
}
