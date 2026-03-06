// Design: docs/architecture/core-design.md — shared BGP types

package types

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

// BGPReactor defines BGP-specific reactor operations.
// These methods handle route announcement, withdrawal, RIB management,
// transactions, and UPDATE cache operations.
//
// The Reactor struct (internal/plugins/bgp/reactor/) implements both
// BGPReactor and plugin.ReactorLifecycle.
type BGPReactor interface {
	// --- Route announce ---

	// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
	// RFC 4271 Section 4.3, RFC 4760, RFC 8654.
	AnnounceNLRIBatch(peerSelector string, batch NLRIBatch) error

	// AnnounceEOR sends an End-of-RIB marker for the given address family.
	AnnounceEOR(peerSelector string, afi uint16, safi uint8) error

	// --- Route withdraw ---

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

	// SoftClearPeer sends ROUTE-REFRESH for all negotiated families of matching peers.
	// Returns the list of families refreshed.
	// RFC 2918 Section 3: soft reset via route refresh.
	SoftClearPeer(peerSelector string) ([]string, error)

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
