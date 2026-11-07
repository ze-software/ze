// Design: docs/architecture/static-routes.md -- static route data model

package static

import "net/netip"

type actionType uint8

const (
	actionForward   actionType = 1
	actionBlackhole actionType = 2
	actionReject    actionType = 3
)

func (a actionType) String() string {
	switch a {
	case actionForward:
		return "forward"
	case actionBlackhole:
		return "blackhole"
	case actionReject:
		return "reject"
	default:
		panic("BUG: static: invalid route action")
	}
}

type nextHop struct {
	Address    netip.Addr
	Interface  string
	Weight     uint16
	BFDProfile string
	// Dynamic is set by classifyDeclared at verify when the candidate interface
	// section declares Interface as a device iface creates on demand and that
	// may be absent (iface.DeclaredDynamicNames). Such a next-hop is a waiting
	// member: never skipped, inactive until its device is up. False is the
	// undeclared answer, and keeps the resolve-or-skip path at apply. The flag
	// travels inside the route, so the pending delivery, the apply and the
	// journal's undo each carry the declared set verified with their routes, and
	// routesEqual sees a change of classification as a change of route.
	Dynamic bool
}

type staticRoute struct {
	Prefix      netip.Prefix
	Table       uint32
	Description string
	Metric      uint32
	// Tag is meaningful only when HasTag is set: 0 is a real tag, and a route
	// with no tag leaf carries none (a Loc-RIB filter reading tag fails it).
	Tag    uint32
	HasTag bool
	// Distance is this route's own administrative distance, which wins over
	// `rib { distance { static } }` for this route alone. It is meaningful only
	// when HasDistance is set: 0 is a real distance, the best one.
	Distance    uint8
	HasDistance bool
	Action      actionType
	NextHops    []nextHop
}
