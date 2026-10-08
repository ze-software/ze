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
}

type staticRoute struct {
	Prefix      netip.Prefix
	Table       uint32
	Description string
	Metric      uint32
	Tag         uint32
	// Distance is this route's own administrative distance, which wins over
	// `rib { distance { static } }` for this route alone. It is meaningful only
	// when HasDistance is set: 0 is a real distance, the best one.
	Distance    uint8
	HasDistance bool
	Action      actionType
	NextHops    []nextHop
}
