// Design: docs/architecture/api/commands.md -- show policy test, its text-before and text-after
// Related: policy_dryrun.go -- PolicyDryRun, which shows the pasted and the resulting UPDATE
// Related: policy_dryrun_library.go -- traceLibraryChain, which shows each member's rewrite
// Related: filter_format.go -- attrForFilter and appendSingleAttr, the attribute pairs shown here
// Related: filter_flowspec.go -- the FlowSpec destination block

package reactor

import (
	"bytes"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// displayAttrOrder is the order show policy test names an UPDATE's path
// attributes in, one "<name> <value>" pair each. An attribute outside it
// (an unknown or owner-registered type) is not shown.
var displayAttrOrder = [...]attribute.AttributeCode{
	attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrNextHop, attribute.AttrMED,
	attribute.AttrLocalPref, attribute.AttrAtomicAggregate, attribute.AttrAggregator,
	attribute.AttrCommunity, attribute.AttrOriginatorID, attribute.AttrClusterList,
	attribute.AttrExtCommunity, attribute.AttrAIGP, attribute.AttrLargeCommunity,
}

// routeDisplay is how show policy test shows one UPDATE (D-11): each path
// attribute it carries as "<name> <value>", in displayAttrOrder, then its NLRI
// blocks. It is read from the wire UPDATE the chain judged or built, never
// from a filter's answer, and it is display only: no filter is sent it and
// nothing parses it back, so it is never a transport. Building one allocates
// the rendered pairs, which a dry-run can afford and a live path never pays.
type routeDisplay struct {
	attrs [len(displayAttrOrder)][]byte
	nlri  []byte
}

// displayUpdate answers the display of the UPDATE wireUpdate holds. An error
// says its attributes do not parse, so nothing true can be shown.
func displayUpdate(wireUpdate *wireu.WireUpdate) (routeDisplay, error) {
	var display routeDisplay
	attrs, err := wireUpdate.Attrs()
	if err != nil {
		return display, err
	}
	if attrs != nil {
		for i, code := range displayAttrOrder {
			parsed := attrForFilter(attrs, code)
			if parsed == nil {
				continue
			}
			display.attrs[i], _ = appendSingleAttr(nil, parsed, true)
		}
	}
	display.nlri = appendNLRIBlocks(nil, wireUpdate)
	return display, nil
}

// appendText appends the display as one line: the attribute pairs, then the
// NLRI blocks, separated by one space.
func (d *routeDisplay) appendText(buf []byte) []byte {
	start := len(buf)
	for _, pair := range d.attrs {
		if len(pair) == 0 {
			continue
		}
		if len(buf) > start {
			buf = append(buf, ' ')
		}
		buf = append(buf, pair...)
	}
	if len(d.nlri) == 0 {
		return buf
	}
	if len(buf) > start {
		buf = append(buf, ' ')
	}
	return append(buf, d.nlri...)
}

// text answers the display as one line, the form text-before and text-after
// carry.
func (d *routeDisplay) text() string { return string(d.appendText(nil)) }

// changed answers the names of the attributes whose shown value differs
// between d and after, in displayAttrOrder, and "nlri" last when the NLRI
// blocks differ: the dry-run's changed-attrs. Nil when nothing differs.
func (d *routeDisplay) changed(after *routeDisplay) []string {
	var names []string
	for i := range d.attrs {
		if bytes.Equal(d.attrs[i], after.attrs[i]) {
			continue
		}
		pair := d.attrs[i]
		if len(pair) == 0 {
			pair = after.attrs[i]
		}
		names = append(names, displayPairName(pair))
	}
	if !bytes.Equal(d.nlri, after.nlri) {
		names = append(names, policyAttrNLRI)
	}
	return names
}

// displayPairName answers the attribute name a shown pair opens with: the
// token before its first space, or the whole pair for a bare token
// (atomic-aggregate). The pair's own renderer is the one place the name is
// spelled.
func displayPairName(pair []byte) string {
	name, _, _ := bytes.Cut(pair, []byte{' '})
	return string(name)
}

// The tokens of an NLRI block. A block is "nlri add|del [<prefix>...]" for the
// legacy NLRI and Withdrawn Routes of RFC 4271, always IPv4 unicast and so with
// no family token, or "mp <family> add|del [<prefix>...]" for an MP_REACH_NLRI
// or MP_UNREACH_NLRI attribute of RFC 4760, whatever its family, IPv4 unicast
// included. The keyword ("nlri", policyAttrNLRI, or "mp", policyAttrMP) names
// the section a block came from. nlriOpAdd announces and nlriOpDel withdraws.
const (
	nlriOpAdd = "add"
	nlriOpDel = "del"
)

// appendNLRIBlocks appends the NLRI blocks of the UPDATE wireUpdate holds, in
// the order legacy NLRI, legacy Withdrawn Routes, MP_REACH_NLRI,
// MP_UNREACH_NLRI, and returns the extended slice. The first block takes no
// leading space when buf is empty.
//
// For families whose NLRI is a plain CIDR prefix (unicast, multicast,
// mpls-label in AFI IPv4/IPv6), the prefix list is emitted inline. FlowSpec
// shows its destination prefix component for every NLRI, without converting
// the NLRI to unicast. Other non-CIDR families are a marker block naming the
// family alone.
//
// The caller owns buf; the blocks are appended with the builtin append, with
// no intermediate string.
func appendNLRIBlocks(buf []byte, wireUpdate *wireu.WireUpdate) []byte {
	start := len(buf)
	ctx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())

	v4AddPath := ctx != nil && ctx.AddPath(family.IPv4Unicast)
	// A section that does not parse shows as a malformed block, never as an
	// absent one: an operator who saw no block would read an UPDATE whose
	// reachability nothing can read as one that carries none.
	// RFC 4271 Section 4.3: the legacy Network Layer Reachability Information.
	raw, err := wireUpdate.NLRI()
	if err != nil {
		buf = appendMalformedBlock(buf, policyAttrNLRI, nlriOpAdd, len(buf) == start)
	} else if len(raw) > 0 {
		buf = appendLegacyBlock(buf, nlriOpAdd, raw, v4AddPath, false, len(buf) == start)
	}
	// RFC 4271 Section 4.3: the legacy Withdrawn Routes.
	raw, err = wireUpdate.Withdrawn()
	if err != nil {
		buf = appendMalformedBlock(buf, policyAttrNLRI, nlriOpDel, len(buf) == start)
	} else if len(raw) > 0 {
		buf = appendLegacyBlock(buf, nlriOpDel, raw, v4AddPath, true, len(buf) == start)
	}

	// MP_REACH_NLRI and MP_UNREACH_NLRI (RFC 4760).
	mp, err := wireUpdate.MPReach()
	if err != nil {
		buf = appendMalformedBlock(buf, policyAttrMP, nlriOpAdd, len(buf) == start)
	} else if mp != nil {
		if nlrisplit.IsFlowSpec(mp.Family()) {
			buf = appendFlowSpecFilterBlock(buf, mp.Family(), nlriOpAdd, mp.NLRIBytes(), ctx != nil && ctx.AddPath(mp.Family()), len(buf) == start)
		} else {
			buf = appendMPBlock(buf, mp.Family(), nlriOpAdd, mp.NLRIBytes(), ctx != nil && ctx.AddPath(mp.Family()), len(buf) == start)
		}
	}
	mpu, err := wireUpdate.MPUnreach()
	if err != nil {
		buf = appendMalformedBlock(buf, policyAttrMP, nlriOpDel, len(buf) == start)
	} else if mpu != nil {
		if nlrisplit.IsFlowSpec(mpu.Family()) {
			buf = appendFlowSpecFilterBlock(buf, mpu.Family(), nlriOpDel, mpu.WithdrawnBytes(), ctx != nil && ctx.AddPath(mpu.Family()), len(buf) == start)
		} else {
			buf = appendMPBlock(buf, mpu.Family(), nlriOpDel, mpu.WithdrawnBytes(), ctx != nil && ctx.AddPath(mpu.Family()), len(buf) == start)
		}
	}

	return buf
}
