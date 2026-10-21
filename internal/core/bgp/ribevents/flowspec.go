// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization and selection
// RFC: rfc/short/rfc8955.md -- Section 6, updated by RFC 9117 Section 4
package ribevents

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
)

// FlowSpecChange carries one selected, authorized rule. Byte slices are owned
// by the event. NLRI includes its native length and RD, without ADD-PATH.
// Communities contain concatenated values without their attribute headers:
// eight octets for ExtendedCommunities, twenty for IPv6ExtendedCommunities.
type FlowSpecChange struct {
	Family                  family.Family
	NLRI                    []byte
	ExtendedCommunities     []byte
	IPv6ExtendedCommunities []byte
	Withdraw                bool
}

var FlowSpecChanged = events.Register[*FlowSpecChange](Namespace, "flowspec-change")

// FlowSpecPath supplies the current received generation for recovery export.
// Attributes is an owned path-attribute block, including MP_REACH_NLRI.
type FlowSpecPath struct {
	Attributes []byte
	MsgID      uint64
}

type flowSpecProvider struct {
	eligible ValidationLookup
	present  func(ValidationRoute) bool
	path     func(ValidationRoute) (FlowSpecPath, bool)
	routes   func() []ValidationRoute
}

var flowSpecLookup atomic.Pointer[flowSpecProvider]

// IsFlowSpec reports the two SAFIs whose routes require unicast authorization.
func IsFlowSpec(fam family.Family) bool {
	return fam.SAFI == family.SAFIFlowSpec || fam.SAFI == family.SAFIFlowSpecVPN
}

// FlowSpecKey retains native NLRI framing with the shortest length encoding.
// Callers MUST remove ADD-PATH first. Malformed lengths return an empty key.
// RFC 8955 Section 4 permits either length encoding below 240 octets; both
// name one rule and MUST share eligibility, replacement and withdrawal state.
func FlowSpecKey(raw []byte) string {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	payload, err := nlrisplit.GetPrefixKey(fam)(raw, nil, false)
	if err != nil {
		return ""
	}
	if len(payload) < 240 {
		if len(raw) == len(payload)+2 {
			return string(raw[1:])
		}
	}
	return string(raw)
}

// RegisterFlowSpecLookup installs the selecting RIB's mandatory gate. Its owner
// MUST register before receiving peer events and MUST unregister after stopping
// delivery. This registration is independent of optional RPKI validation.
func RegisterFlowSpecLookup(eligible ValidationLookup, present func(ValidationRoute) bool, path func(ValidationRoute) (FlowSpecPath, bool), routes func() []ValidationRoute) {
	if eligible == nil {
		flowSpecLookup.Store(nil)
		return
	}
	flowSpecLookup.Store(&flowSpecProvider{eligible: eligible, present: present, path: path, routes: routes})
}

// LookupFlowSpecPath returns owned attributes for the current received path.
// Missing providers and removed routes return false, so replay can withdraw.
func LookupFlowSpecPath(route ValidationRoute) (FlowSpecPath, bool) {
	provider := flowSpecLookup.Load()
	if provider == nil || provider.path == nil {
		return FlowSpecPath{}, false
	}
	return provider.path(route)
}

// FlowSpecRoutes returns owned keys for currently authorized received paths.
// It supplies peer-up replay when the optional Adj-RIB-In plugin is absent.
func FlowSpecRoutes() []ValidationRoute {
	provider := flowSpecLookup.Load()
	if provider == nil || provider.routes == nil {
		return nil
	}
	return provider.routes()
}

// FlowSpecRedirectRefused answers whether the selecting RIB refused the
// redirect-to-IP action (0x0800) of the FlowSpec UPDATE msgID received from
// peer, under draft-simpson-idr-flowspec-redirect-02 Section 3's EBGP check. A
// forward rail asks it once per UPDATE and, on true, removes every 0x0800
// community from the copy it sends. With no provider the answer is false: the
// eligibility gate (RouteEligible) already forwards no FlowSpec route then.
func FlowSpecRedirectRefused(peer netip.Addr, msgID uint64) bool {
	refused := flowSpecRedirect.Load()
	return refused != nil && (*refused)(peer, msgID)
}

var flowSpecRedirect atomic.Pointer[func(netip.Addr, uint64) bool]

// RegisterFlowSpecRedirectLookup installs the RIB's redirect verdict beside
// RegisterFlowSpecLookup, with the same register and unregister contract. nil
// unregisters it.
func RegisterFlowSpecRedirectLookup(refused func(peer netip.Addr, msgID uint64) bool) {
	if refused == nil {
		flowSpecRedirect.Store(nil)
		return
	}
	flowSpecRedirect.Store(&refused)
}
