// Design: plan/spec-static-routes.md -- static route data model

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
	}
	return "unknown"
}

type nextHop struct {
	Address    netip.Addr
	Interface  string
	Weight     uint16
	BFDProfile string
}

type staticRoute struct {
	Prefix      netip.Prefix
	Description string
	Metric      uint32
	Tag         uint32
	Action      actionType
	NextHops    []nextHop
}
