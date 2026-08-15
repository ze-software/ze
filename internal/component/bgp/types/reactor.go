// Design: docs/architecture/core-design.md — shared BGP types

package types

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/selector"
)

// BGPReactor defines BGP-specific reactor operations.
// These methods handle route announcement, withdrawal, RIB management,
// transactions, and UPDATE cache operations.
//
// The Reactor struct (internal/plugins/bgp/reactor/) implements both
// BGPReactor and plugin.ReactorLifecycle.
//
// Every method on this interface that puts a message on a peer's wire takes a
// trailing plugin.Sender: the attached process that issued the command, or the
// operator who typed it at the CLI, SSH or REST surface. The peer's
// `attach process <name> { send [ ... ] }` block is the permission, and the
// reactor refuses a peer that grants none (reactor/send_permission.go). Pass
// CommandContext.Sender, which every dispatch path states.
//
// The zero Sender says nobody named the issuer, and the reactor refuses it
// rather than reading it as the operator: an operator command carries the
// operator's own authority, so a caller that leaves this argument out would
// take that authority by omission (ai/rules/evidence.md).
//
// The permission each one is gated on is the message it generates. AnnounceEOR,
// AnnounceNLRIBatch, WithdrawNLRIBatch, SendRoutes and ForwardUpdate need
// `send [ update ]`. SendBoRR, SendEoRR, SendRefresh and SoftClearPeer need
// `send [ refresh ]`. SendRawMessage needs neither, and needs the peer to attach
// the process at all: the bytes it carries are a message of the caller's
// choosing, so no send type describes it (send_permission.go, sendTypeRaw).
//
// pluginName, where a method carries one beside the sender, is NOT an identity.
// ForwardUpdate and ReleaseUpdate use it for cache accounting, it names the
// plugin whose acks are tracked, and it is empty for a caller that is not a
// cache consumer. Reading it as the authority is the mistake this paragraph
// exists to stop: it was the whole of what ForwardUpdate knew about its caller
// until the Review Gate found that a process could relay peer A's UPDATE into
// peer B with it (spec-fixit-peer-process-event-filter).
//
// Two rails that generate an UPDATE are NOT on this interface and are gated the
// same way through plugin.ReactorCacheCoordinator.ForwardUpdatesDirect and
// plugin.ReactorRelayCoordinator.RelayStoredRoute.
type BGPReactor interface {
	// --- Route announce ---

	// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
	// RFC 4271 Section 4.3, RFC 4760, RFC 8654.
	AnnounceNLRIBatch(sel *selector.Selector, batch NLRIBatch, sender plugin.Sender) error

	// AnnounceEOR sends an End-of-RIB marker for the given address family.
	AnnounceEOR(sel *selector.Selector, afi uint16, safi uint8, sender plugin.Sender) error

	// --- Route withdraw ---

	// WithdrawNLRIBatch withdraws a batch of NLRIs.
	// RFC 4271 Section 4.3, RFC 4760.
	WithdrawNLRIBatch(sel *selector.Selector, batch NLRIBatch, sender plugin.Sender) error

	// --- BGP messages (3 methods) ---

	// SendBoRR sends a Beginning of Route Refresh marker to matching peers.
	// RFC 7313 Section 4.
	SendBoRR(sel *selector.Selector, afi uint16, safi uint8, sender plugin.Sender) error

	// SendEoRR sends an End of Route Refresh marker to matching peers.
	// RFC 7313 Section 4.
	SendEoRR(sel *selector.Selector, afi uint16, safi uint8, sender plugin.Sender) error

	// SendRefresh sends a normal ROUTE-REFRESH message to matching peers.
	// RFC 2918 Section 3.
	SendRefresh(sel *selector.Selector, afi uint16, safi uint8, sender plugin.Sender) error

	// SoftClearPeer sends ROUTE-REFRESH for all negotiated families of matching peers.
	// Returns the list of families refreshed.
	// RFC 2918 Section 3: soft reset via route refresh.
	SoftClearPeer(sel *selector.Selector, sender plugin.Sender) ([]string, error)

	// SendRawMessage sends raw bytes to a peer. The peer must attach sender's
	// process; see the attach-only note on this interface.
	SendRawMessage(peerAddr netip.Addr, msgType uint8, payload []byte, sender plugin.Sender) error

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
	SendRoutes(sel *selector.Selector, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool, sender plugin.Sender) (TransactionResult, error)

	// --- UPDATE cache (5 methods) ---

	// ForwardUpdate forwards a cached UPDATE to peers matching the selector.
	// pluginName identifies which plugin is forwarding (for per-plugin ack
	// tracking); sender is the authority, gated on `send [ update ]`.
	ForwardUpdate(sel *selector.Selector, updateID uint64, pluginName string, sender plugin.Sender) error

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
