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
