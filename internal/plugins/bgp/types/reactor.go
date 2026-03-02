// Design: docs/architecture/core-design.md — shared BGP types

package types

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// BGPReactor defines BGP-specific reactor operations.
// These methods handle route announcement, withdrawal, RIB management,
// transactions, and UPDATE cache operations.
//
// The Reactor struct (internal/plugins/bgp/reactor/) implements both
// BGPReactor and plugin.ReactorLifecycle.
type BGPReactor interface {
	// --- Route announce (9 methods) ---

	// AnnounceRoute announces a route to peers matching the selector.
	AnnounceRoute(peerSelector string, route RouteSpec) error

	// AnnounceFlowSpec announces a FlowSpec route to peers.
	AnnounceFlowSpec(peerSelector string, route FlowSpecRoute) error

	// AnnounceVPLS announces a VPLS route to peers.
	AnnounceVPLS(peerSelector string, route VPLSRoute) error

	// AnnounceL2VPN announces an L2VPN/EVPN route to peers.
	AnnounceL2VPN(peerSelector string, route L2VPNRoute) error

	// AnnounceL3VPN announces an L3VPN (MPLS VPN) route to peers.
	AnnounceL3VPN(peerSelector string, route L3VPNRoute) error

	// AnnounceLabeledUnicast announces an MPLS labeled unicast route (SAFI 4).
	AnnounceLabeledUnicast(peerSelector string, route LabeledUnicastRoute) error

	// AnnounceMUPRoute announces a MUP route (SAFI 85) to peers.
	AnnounceMUPRoute(peerSelector string, route MUPRouteSpec) error

	// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
	// RFC 4271 Section 4.3, RFC 4760, RFC 8654.
	AnnounceNLRIBatch(peerSelector string, batch NLRIBatch) error

	// AnnounceEOR sends an End-of-RIB marker for the given address family.
	AnnounceEOR(peerSelector string, afi uint16, safi uint8) error

	// --- Route withdraw (7 methods) ---

	// WithdrawRoute withdraws a route from peers matching the selector.
	WithdrawRoute(peerSelector string, prefix netip.Prefix) error

	// WithdrawFlowSpec withdraws a FlowSpec route from peers.
	WithdrawFlowSpec(peerSelector string, route FlowSpecRoute) error

	// WithdrawVPLS withdraws a VPLS route from peers.
	WithdrawVPLS(peerSelector string, route VPLSRoute) error

	// WithdrawL2VPN withdraws an L2VPN/EVPN route from peers.
	WithdrawL2VPN(peerSelector string, route L2VPNRoute) error

	// WithdrawL3VPN withdraws an L3VPN route from peers.
	WithdrawL3VPN(peerSelector string, route L3VPNRoute) error

	// WithdrawLabeledUnicast withdraws an MPLS labeled unicast route.
	WithdrawLabeledUnicast(peerSelector string, route LabeledUnicastRoute) error

	// WithdrawMUPRoute withdraws a MUP route from peers.
	WithdrawMUPRoute(peerSelector string, route MUPRouteSpec) error

	// WithdrawNLRIBatch withdraws a batch of NLRIs.
	// RFC 4271 Section 4.3, RFC 4760.
	WithdrawNLRIBatch(peerSelector string, batch NLRIBatch) error

	// --- BGP messages (3 methods) ---

	// SendBoRR sends a Beginning of Route Refresh marker to matching peers.
	// RFC 7313 Section 4.
	SendBoRR(peerSelector string, afi uint16, safi uint8) error

	// SendEoRR sends an End of Route Refresh marker to matching peers.
	// RFC 7313 Section 4.
	SendEoRR(peerSelector string, afi uint16, safi uint8) error

	// SendRefresh sends a normal ROUTE-REFRESH message to matching peers.
	// RFC 2918 Section 3.
	SendRefresh(peerSelector string, afi uint16, safi uint8) error

	// SendRawMessage sends raw bytes to a peer.
	SendRawMessage(peerAddr netip.Addr, msgType uint8, payload []byte) error

	// --- RIB operations ---
	// Engine has no RIB — route storage is owned by plugins (bgp-rib, bgp-adj-rib-in).
	// These methods return empty results. Retained for handler compatibility.

	// RIBInRoutes returns routes from Adj-RIB-In for the given peer.
	RIBInRoutes(peerID string) []rib.RouteJSON

	// RIBStats returns RIB statistics.
	RIBStats() RIBStatsInfo

	// ClearRIBIn clears all routes in Adj-RIB-In. Returns count cleared.
	ClearRIBIn() int

	// --- Transactions (6 methods) ---

	// BeginTransaction starts a new transaction with optional label.
	//
	// Deprecated: Use "commit <name> start" instead.
	BeginTransaction(peerSelector, label string) error

	// CommitTransaction commits the current transaction.
	//
	// Deprecated: Use "commit <name> end" instead.
	CommitTransaction(peerSelector string) (TransactionResult, error)

	// CommitTransactionWithLabel commits, verifying the label matches.
	//
	// Deprecated: Use "commit <name> end" instead.
	CommitTransactionWithLabel(peerSelector, label string) (TransactionResult, error)

	// RollbackTransaction discards all queued routes in the transaction.
	//
	// Deprecated: Use "commit <name> rollback" instead.
	RollbackTransaction(peerSelector string) (TransactionResult, error)

	// InTransaction returns true if a transaction is active.
	//
	// Deprecated: Always returns false.
	InTransaction(peerSelector string) bool

	// TransactionID returns the current transaction label.
	//
	// Deprecated: Always returns empty string.
	TransactionID(peerSelector string) string

	// --- Commit (1 method) ---

	// SendRoutes sends routes directly to matching peers using CommitService.
	SendRoutes(peerSelector string, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool) (TransactionResult, error)

	// --- UPDATE cache (5 methods) ---

	// ForwardUpdate forwards a cached UPDATE to peers matching the selector.
	// pluginName identifies which plugin is forwarding (for per-plugin ack tracking).
	ForwardUpdate(sel *selector.Selector, updateID uint64, pluginName string) error

	// DeleteUpdate removes an update from the cache without forwarding.
	DeleteUpdate(updateID uint64) error

	// RetainUpdate prevents eviction of a cached UPDATE.
	RetainUpdate(updateID uint64) error

	// ReleaseUpdate handles cache release with two paths based on caller identity.
	// Cache consumer (pluginName non-empty): acks the entry (FIFO validated).
	// Non-consumer (pluginName empty): decrements API-level retain count only.
	ReleaseUpdate(updateID uint64, pluginName string) error

	// ListUpdates returns all cached msg-ids.
	ListUpdates() []uint64
}
