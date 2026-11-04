// Design: docs/architecture/core-design.md — policy filter chain
// Related: filter_library.go — runLibraryChain, the per-peer chain body
// Related: policy_dryrun_display.go — the attribute and NLRI names a dry run shows
// Related: reactor_notify.go — ingress filter invocation point
// Related: reactor_api_forward.go — egress filter invocation point

package reactor

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Policy attribute names: the words a policy dry run shows for each attribute
// and NLRI block (policy_dryrun_display.go, filter_format.go). Every spelling
// of one of these tokens in this package is one of these constants, so a
// rename cannot leave a renderer half-converted.
const (
	policyAttrOrigin                  = "origin"
	policyAttrASPath                  = "as-path"
	policyAttrNextHop                 = "next-hop"
	policyAttrMED                     = "med"
	policyAttrLocalPreference         = "local-preference"
	policyAttrAtomicAggregate         = "atomic-aggregate"
	policyAttrAggregator              = "aggregator"
	policyAttrCommunity               = "community"
	policyAttrOriginatorID            = "originator-id"
	policyAttrClusterList             = "cluster-list"
	policyAttrExtendedCommunity       = "extended-community"
	policyAttrAIGP                    = "aigp"
	policyAttrLargeCommunity          = "large-community"
	policyAttrCommunityAdd            = "community-add"
	policyAttrCommunityRemove         = "community-remove"
	policyAttrLargeCommunityAdd       = "large-community-add"
	policyAttrLargeCommunityRemove    = "large-community-remove"
	policyAttrExtendedCommunityAdd    = "extended-community-add"
	policyAttrExtendedCommunityRemove = "extended-community-remove"
	policyAttrMEDRemove               = "med-remove"
	policyAttrASPathPrepend           = "as-path-prepend"
	policyAttrNLRI                    = "nlri"
	// policyAttrMP opens an NLRI block rendered from MP_REACH_NLRI or
	// MP_UNREACH_NLRI ("mp <family> add|del"). It is a second name of the one
	// NLRI value, never an attribute of its own.
	policyAttrMP = "mp"
)

// Policy chain direction tokens: the direction a chain runs in, and the
// FilterUpdateInput.Direction each plugin filter is sent.
const (
	directionImport = "import"
	directionExport = "export"
)

// PolicyAction is the result of a policy dry run.
type PolicyAction int

const (
	// PolicyAccept passes the update through unchanged.
	PolicyAccept PolicyAction = iota
	// PolicyReject drops the update (short-circuits the chain).
	PolicyReject
	// PolicyModify passes the update with its attributes or NLRI changed.
	PolicyModify
)

// PolicyChainResult is the outcome of a policy dry run over one UPDATE.
type PolicyChainResult struct {
	Action PolicyAction
	// Failed marks a PolicyReject that no filter decided: a member that
	// could not run under a fail-closed on-error, or a chain that could not
	// be compiled. The route is still rejected, but a caller counting
	// outcomes must not record it as a policy decision.
	Failed bool
	// Text is the UPDATE as the dry run shows it after the chain (valid
	// unless Action==PolicyReject).
	Text string
	// Raw is a raw full-payload replacement from a raw filter; terminal.
	// Empty when no raw filter rewrote the payload.
	Raw []byte
	// Teardown (import only) requests NOTIFICATION + session close; route dropped.
	Teardown      bool
	NotifyCode    uint8
	NotifySubcode uint8
	// Withdrawn marks a library chain member's withdraw verdict (dry run
	// only): Action is PolicyAccept and Text is the withdrawal the route
	// becomes, which is what the peer is sent instead of the UPDATE.
	Withdrawn bool
}

// filterTransport is the plugin-server surface a library chain's external
// member talks to. *pluginserver.Server satisfies it, and is what production
// passes.
//
// It exists because that type is CONCRETE and its answer arrives off a live
// plugin socket, so the chain's fail-closed branches (a call error under a
// fail-closed on-error, a reply the chain refuses) could not be driven
// otherwise. A test's transport replaces what the chain TALKS TO, so a test
// using it drives the real chain and the real member.
type filterTransport interface {
	// FilterReads answers what the filter's declaration reads: vocabulary
	// field names, and rpc.FilterFieldRaw for the raw UPDATE body.
	FilterReads(pluginName, filterName string) []string
	// FilterDeclaration answers the declaration's reads, sets and on-error,
	// ok false when no running plugin declares the filter.
	FilterDeclaration(pluginName, filterName string) (reads, sets []string, onError rpc.OnErrorPolicy, ok bool)
	FilterOnError(pluginName, filterName string) rpc.OnErrorPolicy
	// FilterReadvertise answers whether the declaration opts into the stale
	// readvertise rail (rpc.FilterDecl.Readvertise), ok false when no running
	// plugin declares the filter.
	FilterReadvertise(pluginName, filterName string) (readvertise, ok bool)
	// CallFilterUpdate judges route by the plugin's filter, on the transport the
	// plugin runs on: its typed filter slot internal, its filter-update
	// batch forked (D-12).
	CallFilterUpdate(ctx context.Context, pluginName string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error)
}

// filterAPI returns the transport the policy filter path talks to: the plugin
// server in production, a test's stand-in when one is set.
//
// The nil check is load-bearing rather than defensive. A nil *pluginserver.Server
// stored in an interface is NOT a nil interface, and every caller of this method
// tests the result for nil to fail closed. Returning the typed nil would make
// that guard read "the filter engine is present" for a daemon that has none, and
// the route would go out unfiltered.
func (r *Reactor) filterAPI() filterTransport {
	if r.filterTransportSeam != nil {
		return r.filterTransportSeam
	}
	if r.api == nil {
		return nil
	}
	return r.api
}

// decodeFilterRawOverride validates a full UPDATE-body replacement from a raw
// policy filter. A valid UPDATE body is at least 4 bytes: withdrawn-routes
// length(2) + path-attribute length(2), and its section lengths stay inside it.
//
// It answers TWO questions, and a single nil could only answer one of them
// (ai/rules/evidence.md). An EMPTY raw means the filter asked for no
// replacement (nil, nil), which is the ordinary case for every filter that
// answers with sets. A non-nil error means the filter DID ask for a replacement
// and handed over something ze must not send or install, and the error says why.
//
// Folding those together makes the second silently indistinguishable from the
// first: a caller reading nil as "no override" skips the branch and forwards or
// caches the ORIGINAL body, because a raw answer is terminal and no set rides
// beside it. The filter had asked for the body to be replaced, and got the body
// it was replacing. On export that leaks whatever the raw surgery was removing;
// on import it installs it.
//
// The error is therefore a guard MISS, not a policy decision or an absent
// override. libraryCaller returns it before the member can end the chain:
// that member's on-error rejects the route, or explicitly passes it to later
// members without the failed rewrite, with failure identity, counting and
// logging intact. Display-only rendering returns the error to its trace.
//
// Every raw answer, import or export, live or dry run, comes through here, so the RFC 9012 Section 13 checks the receive path applies to
// the peer's bytes apply to the filter's bytes too: a filter's body is a route
// ze distributes, and no later step on either path runs them again.
func decodeFilterRawOverride(raw []byte) (override []byte, err error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) < 4 {
		return nil, errFilterRawTooShort
	}
	sections, err := wire.ParseUpdateSections(raw)
	if err != nil {
		return nil, errFilterRawSections
	}
	// RFC 9012 Section 13
	return filterRawTunnelEncap(raw, sections)
}

// errFilterRawTooShort and errFilterRawSections name the framing refusals of a
// raw answer; filterRawTunnelEncap names the tunnel ones.
var (
	errFilterRawTooShort = errors.New("raw override is shorter than the 4 octets of an UPDATE body")
	errFilterRawSections = errors.New("raw override section lengths run past the end of the body")
)

// filterRawTunnelEncap runs applyTunnelEncap, the receive path's RFC 9012
// Section 13 guard, over a raw filter answer whose sections parse. The guard
// reads only the facts built here: whether attribute 23 is present, where
// MP_REACH_NLRI is and for which family, and whether the body announces a
// route. A raw answer carries no RFC 7606 verdict of its own, so the guard
// starts from no action. The answer bytes are never edited: a removal returns
// a rebuilt copy, and a body with nothing to remove returns raw itself.
// RFC 9012 Section 13: "There is one exception to this rule: if a TLV contains
// a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the
// entire TLV MUST be ignored and MUST be removed from the Tunnel Encapsulation
// attribute before the route carrying that attribute is distributed."
func filterRawTunnelEncap(raw []byte, sections wire.UpdateSections) ([]byte, error) {
	attrs := sections.Attrs(raw)
	hasNLRI := len(sections.NLRI(raw)) > 0
	result := message.RFC7606ValidationResult{HasReachableNLRI: hasNLRI}
	_, _, _, result.TunnelEncapPresent = attribute.AttrFind(attrs, attribute.AttrTunnelEncap)
	if _, _, mp, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); found {
		if start, ok := message.MPNLRIStart(uint8(attribute.AttrMPReachNLRI), mp); ok {
			result.MPReachNLRI = message.MPNLRILocation{Present: true, AFI: binary.BigEndian.Uint16(mp[0:2]), SAFI: mp[2]}
			if start < len(mp) {
				result.HasReachableNLRI = true
			}
		}
	}
	// RFC 9012 Section 13
	out, _ := applyTunnelEncap(wireu.NewWireUpdate(raw, 0), attrs, hasNLRI, &result)
	if result.Action >= message.RFC7606ActionTreatAsWithdraw {
		return nil, errors.New("raw override refused: " + result.Description)
	}
	return out.Payload(), nil
}
