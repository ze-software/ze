// Design: docs/architecture/api/json-format.md -- typed route action enum

package types

import "fmt"

// RouteAction is the typed wire token describing what happened to a route.
// "add"/"del" are emitted by FamilyOperation (the wire-level command);
// "update"/"withdraw" are emitted by BestChangeEntry (best-path transitions).
// One typed enum covers both surfaces -- consumers compare on identity and
// MarshalText preserves the wire string for external plugins.
type RouteAction uint8

const (
	RouteActionUnspecified RouteAction = 0
	RouteActionAdd         RouteAction = 1
	RouteActionDel         RouteAction = 2
	RouteActionUpdate      RouteAction = 3
	RouteActionWithdraw    RouteAction = 4
	// RouteActionCount is one past the highest valid RouteAction. Use as
	// the size of arrays indexed by RouteAction so per-action caches
	// (pre-bound metric Counters, dispatch tables, etc.) can be indexed
	// directly with zero allocation.
	RouteActionCount = 5
)

const (
	routeActionWireAdd      = "add"
	routeActionWireDel      = "del"
	routeActionWireUpdate   = "update"
	routeActionWireWithdraw = "withdraw"
)

func (a RouteAction) String() string {
	switch a {
	case RouteActionAdd:
		return routeActionWireAdd
	case RouteActionDel:
		return routeActionWireDel
	case RouteActionUpdate:
		return routeActionWireUpdate
	case RouteActionWithdraw:
		return routeActionWireWithdraw
	case RouteActionUnspecified:
		return "unspecified"
	}
	return "unspecified"
}

func (a RouteAction) AppendTo(buf []byte) []byte { return append(buf, a.String()...) }

func (a RouteAction) MarshalText() ([]byte, error) {
	if a == RouteActionUnspecified {
		return nil, fmt.Errorf("types: unspecified RouteAction is invalid on the wire")
	}
	return []byte(a.String()), nil
}

func (a *RouteAction) UnmarshalText(data []byte) error {
	switch string(data) {
	case routeActionWireAdd:
		*a = RouteActionAdd
	case routeActionWireDel:
		*a = RouteActionDel
	case routeActionWireUpdate:
		*a = RouteActionUpdate
	case routeActionWireWithdraw:
		*a = RouteActionWithdraw
	default:
		return fmt.Errorf("types: unknown route action %q", string(data))
	}
	return nil
}

// BGPProtocolType distinguishes iBGP from eBGP routes. This is a BGP-internal
// 2-value classification, not a cross-protocol identity (see redistevents.ProtocolID).
type BGPProtocolType uint8

const (
	BGPProtocolUnspecified BGPProtocolType = 0
	BGPProtocolEBGP        BGPProtocolType = 1
	BGPProtocolIBGP        BGPProtocolType = 2
	BGPProtocolCount       BGPProtocolType = 3
)

func (p BGPProtocolType) String() string {
	switch p {
	case BGPProtocolEBGP:
		return "ebgp"
	case BGPProtocolIBGP:
		return "ibgp"
	case BGPProtocolUnspecified, BGPProtocolCount:
		return "unspecified"
	}
	return "unspecified"
}

func (p BGPProtocolType) AppendTo(buf []byte) []byte { return append(buf, p.String()...) }

func (p BGPProtocolType) MarshalText() ([]byte, error) {
	if p == BGPProtocolUnspecified || p >= BGPProtocolCount {
		return nil, fmt.Errorf("types: invalid BGPProtocolType %d on the wire", p)
	}
	return []byte(p.String()), nil
}

func (p *BGPProtocolType) UnmarshalText(data []byte) error {
	switch string(data) {
	case "ebgp":
		*p = BGPProtocolEBGP
	case "ibgp":
		*p = BGPProtocolIBGP
	default:
		return fmt.Errorf("types: unknown BGP protocol type %q", string(data))
	}
	return nil
}
