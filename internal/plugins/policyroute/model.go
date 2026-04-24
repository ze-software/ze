package policyroute

import "net/netip"

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
type PolicyMatch struct {
	SourceAddress      string
	DestinationAddress string
	SourcePort         string
	DestinationPort    string
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
