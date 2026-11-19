// Design: docs/architecture/policyroute/policy-routing.md -- rule and mark data types

package policyroute

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/firewall"
)

// PolicyRoute is a named policy applied to ingress interfaces.
type PolicyRoute struct {
	Name       string
	Interfaces []InterfaceSpec
	Rules      []PolicyRule
}

// InterfaceSpec identifies an interface to match, optionally with wildcard.
type InterfaceSpec struct {
	Name     string
	Wildcard bool
}

// PolicyRule is a single rule within a policy route.
// Order controls evaluation sequence (lower values first).
type PolicyRule struct {
	Name   string
	Order  uint32
	Match  PolicyMatch
	Action PolicyAction
}

// PolicyMatch describes a packet match criterion.
//
// SourceSet, DestinationSet, SourcePortSet and DestinationPortSet hold the
// firewall set an @name address or port leaf names, copied from the firewall
// table that declares it (resolveSetReferences, sets.go). They stay nil for a
// prefix or a port list, and translate refuses an @name whose set was never
// resolved rather than emit a match on a set ze_pr lacks.
type PolicyMatch struct {
	SourceAddress      string
	DestinationAddress string
	SourceSet          *firewall.Set
	DestinationSet     *firewall.Set
	SourcePort         string
	DestinationPort    string
	SourcePortSet      *firewall.Set
	DestinationPortSet *firewall.Set
	Protocol           string
	TCPFlags           string
}

// PolicyActionType identifies the action to take for matching packets.
type PolicyActionType uint8

const (
	ActionAccept PolicyActionType = iota + 1
	ActionDrop
	ActionTable
	ActionNextHop
)

// PolicyAction describes what to do with matching packets.
type PolicyAction struct {
	Type    PolicyActionType
	Table   uint32
	NextHop netip.Addr
	TCPMSS  uint16
}
