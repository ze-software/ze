// Design: docs/architecture/wire/nlri-bgpls.md -- native topology sources
// Related: internal/core/events/typed.go -- synchronous typed event delivery

// Package linkstateevents carries native routing-database snapshots to consumers.
// Sources retain ownership of every slice; subscribers finish reading before
// Emit returns. A snapshot replaces the entire named domain, including removals.
package linkstateevents

import (
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/events"
)

const EventType = "link-state-snapshot"

// Request asks every running source to publish its current domains. Sources
// subscribe before their initial publication and publish empty domains on stop.
var Request = events.RegisterSignal("link-state", "snapshot-request")

// Protocol is the source protocol identifier assigned by RFC 9552 Section 5.2.
type Protocol uint8

const (
	ISISLevel1 Protocol = 1
	ISISLevel2 Protocol = 2
	OSPFv2 Protocol = 3
	Direct Protocol = 4
	Static Protocol = 5
	OSPFv3 Protocol = 6
	BGP Protocol = 7
)

// Domain distinguishes independently replaceable databases. Instance identifies
// a source's configured instance; Area identifies an OSPF area or IS-IS level.
// Identifier is the RFC 9552 routing-universe identifier placed in the NLRI.
type Domain struct {
	Protocol Protocol
	Instance uint64
	Area uint32
	Identifier uint64
}

// NodeID contains source identity, not a BGP next hop. RouterID holds the IGP
// router identifier, including the pseudonode suffix when present.
type NodeID struct {
	ASN uint32
	BGPLSID uint32
	HasBGPLSID bool
	Area uint32
	HasArea bool
	RouterID []byte
	BGPRouterID netip.Addr
	Confederation uint32
}

// TLV is a recognized BGP-LS attribute value translated by the source adapter.
// Unknown IGP TLVs belong in Opaque with their native provenance instead.
type TLV struct {
	Type uint16
	Value []byte
}

// Provenance identifies the LSA class that actually contained an unknown TLV.
// It prevents an exporter from placing unrelated OSPF data in an opaque field.
type Provenance uint8

const (
	ISIS Provenance = iota + 1
	OSPFRouterInformation
	OSPFv2ExtendedLink
	OSPFv3ExtendedRouter
	OSPFv3ExtendedLink
	OSPFv2ExtendedPrefix
	OSPFv3ExtendedInterAreaPrefix
	OSPFv3ExtendedIntraAreaPrefix
	OSPFv3ExtendedASExternal
	OSPFv3ExtendedNSSA
)

type Opaque struct {
	Source Provenance
	Value []byte
}

type Node struct {
	ID NodeID
	Attributes []TLV
	Opaque []Opaque
}

// Link.Topologies lists every topology containing this directed link. Empty
// means the default topology. The exporter emits one NLRI per distinct ID.
type Link struct {
	Local NodeID
	Remote NodeID
	LocalID uint32
	RemoteID uint32
	HasLinkIDs bool
	LocalAddresses []netip.Addr
	RemoteAddresses []netip.Addr
	Topologies []uint16
	Attributes []TLV
	Opaque []Opaque
}

// Prefix.Topology is explicit even when the source uses a shared reachability
// encoding. RouteType is the OSPF Route Type descriptor; zero omits it.
type Prefix struct {
	Node NodeID
	Prefix netip.Prefix
	Topology uint16
	RouteType uint8
	Attributes []TLV
	Opaque []Opaque
}

// SID describes a native SRv6 endpoint. Attributes include its mandatory
// Endpoint Behavior TLV and any source-advertised structure or peer attributes.
type SID struct {
	Node NodeID
	SID netip.Addr
	Topology uint16
	Attributes []TLV
}

// Snapshot is a complete, consistent view of one live source domain. Generation
// increases across mutations within one running source; zero is allowed on
// initial publication. Consumers never infer withdrawals from partial batches.
type Snapshot struct {
	Domain Domain
	Generation uint64
	Nodes []Node
	Links []Link
	Prefixes []Prefix
	SIDs []SID
	// Unreachable names originators which native IGP SPF currently determines
	// are unreachable. Objects remain in this complete LSDB view for consumers
	// that need database provenance; BGP-LS suppresses their advertisements.
	// A subsequent snapshot omitting an originator here restores its objects.
	Unreachable []NodeID
}

var sources struct {
	sync.RWMutex
	names []string
}

// RegisterSource declares a source namespace at init and returns its local
// event handle. The registry stores names, never protocol callbacks or state.
func RegisterSource(namespace string) *events.Event[*Snapshot] {
	handle := events.Register[*Snapshot](namespace, EventType)
	sources.Lock()
	if !slices.Contains(sources.names, namespace) {
		sources.names = append(sources.names, namespace)
	}
	sources.Unlock()
	return handle
}

// Sources returns the registered source namespaces in deterministic order.
func Sources() []string {
	sources.RLock()
	names := slices.Clone(sources.names)
	sources.RUnlock()
	slices.Sort(names)
	return names
}
