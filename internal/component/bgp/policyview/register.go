// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: readers.go -- the readers registered here

package policyview

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// The BGP field readers, one per field the BGP import and export attachments
// carry (internal/component/bgp/config/register.go). The field names stay
// declared in the policy library; only how BGP reads each one lives here.
// metric has no reader: BGP names that fact med (spec D-8).
func init() {
	MustRegisterReader(policy.FieldPrefix, readPrefix)
	MustRegisterReader(policy.FieldWithdrawn, readWithdrawn)
	MustRegisterReader(policy.FieldFamily, readFamily)
	MustRegisterReader(policy.FieldProtocol, readProtocol)
	MustRegisterReader(policy.FieldNextHop, readNextHop)
	MustRegisterReader(policy.FieldOriginAS, readOriginAS)
	MustRegisterReader(policy.FieldCommunity, readCommunity)
	MustRegisterReader(policy.FieldLargeCommunity, readLargeCommunity)
	MustRegisterReader(policy.FieldExtendedCommunity, readExtendedCommunity)
	MustRegisterReader(policy.FieldASPath, readASPath)
	MustRegisterReader(policy.FieldLocalPreference, readLocalPreference)
	MustRegisterReader(policy.FieldOrigin, readOrigin)
	MustRegisterReader(policy.FieldAIGP, readAIGP)
	MustRegisterReader(policy.FieldMED, readMED)
	MustRegisterReader(policy.FieldAtomicAggregate, readAtomicAggregate)
	MustRegisterReader(policy.FieldAggregator, readAggregator)
	MustRegisterReader(policy.FieldOriginatorID, readOriginatorID)
	MustRegisterReader(policy.FieldClusterList, readClusterList)
	for _, fact := range sessionFacts {
		MustRegisterReader(fact.field, readSessionFact(fact.answer))
		mustRegisterWriter(fact.field, fact.write, 0, "")
		if fact.peer {
			registry.RegisterSessionFact(fact.field.String())
		}
	}
	// rebuilt is the engine's metadata: the reactor marks a route it rebuilt
	// from stored state rather than received (filterapi.MetaKeyRebuilt).
	MustRegisterMetadata(FieldRebuilt, MetadataFlag)
}

// The session fact fields, registered by the BGP engine that owns the facts
// (spec-filter-wire-6-stage-filters D-7, D-9). The plain names describe the
// session's peer: the sender on import, the destination on export. The
// source-peer forms describe the peer an exported route came from.
var (
	FieldPeerName = policy.MustRegisterField("peer-name", policy.RouteView{})
	// FieldRebuilt is the route metadata key the reactor sets on a route it
	// rebuilt (metadata.go).
	FieldRebuilt        = policy.MustRegisterField(filterapi.MetaKeyRebuilt, policy.RouteView{})
	FieldPeerGroup      = policy.MustRegisterField("peer-group", policy.RouteView{})
	FieldPeerAddress    = policy.MustRegisterField("peer-address", policy.RouteView{})
	FieldLocalAS        = policy.MustRegisterField("local-as", policy.RouteView{})
	FieldPeerAS         = policy.MustRegisterField("peer-as", policy.RouteView{})
	FieldRouteServerAS  = policy.MustRegisterField("route-server-as", policy.RouteView{})
	FieldIBGP           = policy.MustRegisterField("ibgp", policy.RouteView{})
	FieldRSClient       = policy.MustRegisterField("rs-client", policy.RouteView{})
	FieldLLGRAdvertised = policy.MustRegisterField("llgr-advertised", policy.RouteView{})

	FieldSourcePeerName    = policy.MustRegisterField("source-peer-name", policy.RouteView{})
	FieldSourcePeerGroup   = policy.MustRegisterField("source-peer-group", policy.RouteView{})
	FieldSourcePeerAddress = policy.MustRegisterField("source-peer-address", policy.RouteView{})
	FieldSourcePeerAS      = policy.MustRegisterField("source-peer-as", policy.RouteView{})
	FieldSourceIBGP        = policy.MustRegisterField("source-ibgp", policy.RouteView{})
	FieldSourceRSClient    = policy.MustRegisterField("source-rs-client", policy.RouteView{})
	// FieldSourceLLGRAdvertised is llgr-advertised of an exported route's
	// source, answered only for a route rebuilt from storage
	// (answerSourceLLGRAdvertised).
	FieldSourceLLGRAdvertised = policy.MustRegisterField("source-llgr-advertised", policy.RouteView{})
)

// sessionFacts is the one declaration of how each session fact field is
// answered and written, and whether it is a fact of the peer itself; init
// registers a reader and a writer per row, and each peer fact as a fact a
// filter insertion's session condition may name (registry.RegisterSessionFact).
var sessionFacts = []sessionFact{
	{FieldPeerName, textFact((*Session).peerFacts, func(p *filterapi.PeerFilterInfo) string { return p.Name }), writeText, true},
	{FieldPeerGroup, textFact((*Session).peerFacts, func(p *filterapi.PeerFilterInfo) string { return p.GroupName }), writeText, true},
	{FieldPeerAddress, addressFact((*Session).peerFacts), writeAddress, true},
	{FieldLocalAS, asFact((*Session).peerFacts, func(p *filterapi.PeerFilterInfo) uint32 { return p.LocalAS }), writeNumber, true},
	{FieldPeerAS, asFact((*Session).peerFacts, func(p *filterapi.PeerFilterInfo) uint32 { return p.PeerAS }), writeNumber, true},
	{FieldRouteServerAS, answerRouteServerAS, writeNumber, true},
	{FieldIBGP, flagFact((*Session).peerFacts, func(p *filterapi.PeerFilterInfo) bool { return p.IsIBGP }), writeFlag, true},
	{FieldRSClient, flagFact((*Session).peerFacts, func(p *filterapi.PeerFilterInfo) bool { return p.RSClient }), writeFlag, true},
	{FieldLLGRAdvertised, answerLLGRAdvertised, writeFlag, false},
	{FieldSourcePeerName, textFact((*Session).sourceFacts, func(p *filterapi.PeerFilterInfo) string { return p.Name }), writeText, false},
	{FieldSourcePeerGroup, textFact((*Session).sourceFacts, func(p *filterapi.PeerFilterInfo) string { return p.GroupName }), writeText, false},
	{FieldSourcePeerAddress, addressFact((*Session).sourceFacts), writeAddress, false},
	{FieldSourcePeerAS, asFact((*Session).sourceFacts, func(p *filterapi.PeerFilterInfo) uint32 { return p.PeerAS }), writeNumber, false},
	{FieldSourceIBGP, flagFact((*Session).sourceFacts, func(p *filterapi.PeerFilterInfo) bool { return p.IsIBGP }), writeFlag, false},
	{FieldSourceRSClient, flagFact((*Session).sourceFacts, func(p *filterapi.PeerFilterInfo) bool { return p.RSClient }), writeFlag, false},
	{FieldSourceLLGRAdvertised, answerSourceLLGRAdvertised, writeFlag, false},
}
