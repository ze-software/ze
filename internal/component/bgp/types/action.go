// Design: docs/architecture/api/json-format.md -- typed route action enum

package types

import (
	"errors"
	"fmt"
)

var errTypesUnspecifiedRouteactionIsInvalidOn = errors.New("types: unspecified RouteAction is invalid on the wire")

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
	routeActionWireAdd         = "add"
	routeActionWireDel         = "del"
	routeActionWireUpdate      = "update"
	routeActionWireWithdraw    = "withdraw"
	routeActionWireUnspecified = "unspecified"
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
		return routeActionWireUnspecified
	}
	return routeActionWireUnspecified
}

func (a RouteAction) AppendTo(buf []byte) []byte { return append(buf, a.String()...) }

func (a RouteAction) MarshalText() ([]byte, error) {
	if a == RouteActionUnspecified {
		return nil, errTypesUnspecifiedRouteactionIsInvalidOn
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

// RouteVerb is the forwarding-plane operation a RouteAction maps to. FIB
// backends (fib-kernel, fib-vpp, fib-p4) dispatch on it instead of each
// re-encoding that Withdraw and Del both mean remove and that Unspecified is a
// no-op. This package owns RouteAction, so it is the single home for that
// mapping; every FIB backend already depends on this package for RouteAction.
type RouteVerb uint8

const (
	RouteVerbSkip    RouteVerb = iota // Unspecified or unknown action: no-op
	RouteVerbInstall                  // Add: program a new route
	RouteVerbReplace                  // Update: replace an existing route
	RouteVerbRemove                   // Withdraw or Del: remove a route
)

// Verb maps a RouteAction to the forwarding-plane operation a FIB backend
// performs. Backends that do not distinguish install from replace (e.g. the
// MPLS and SRv6 programming paths) handle RouteVerbInstall and RouteVerbReplace
// together. It is a pure value-enum lookup with no allocation, safe on the hot
// FIB install path.
func (a RouteAction) Verb() RouteVerb {
	switch a {
	case RouteActionAdd:
		return RouteVerbInstall
	case RouteActionUpdate:
		return RouteVerbReplace
	case RouteActionWithdraw, RouteActionDel:
		return RouteVerbRemove
	default: // RouteActionUnspecified and any unknown value
		return RouteVerbSkip
	}
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
		return routeActionWireUnspecified
	}
	return routeActionWireUnspecified
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
