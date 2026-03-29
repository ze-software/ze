// Design: docs/architecture/update-building.md — cross-peer UPDATE grouping
// Overview: reactor.go — holds update group index
// Related: reactor_notify.go — update group Add/Remove on peer lifecycle
// Related: peer.go — group join/leave on session events
// Related: reactor_api_batch.go — group-aware NLRI batch building
// Related: reactor_api_forward.go — group-aware UPDATE forwarding

package reactor

import (
	"sync"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// GroupKey identifies an update group. Peers with the same GroupKey receive
// bit-identical UPDATE wire bytes and can share a single build.
//
// CtxID encodes all capability differences (ASN4, ADD-PATH, ExtNH).
// PolicyKey allows grouping by outbound policy; today all peers use 0
// (uniform policy). When per-peer export policy is added, peers with
// different policies get different PolicyKey values and separate groups.
type GroupKey struct {
	CtxID     bgpctx.ContextID
	PolicyKey uint32
}

// UpdateGroup is a set of established peers that share the same GroupKey.
// Members receive identical UPDATE wire bytes for the same route.
type UpdateGroup struct {
	Key     GroupKey
	Members []*Peer
}

// UpdateGroupIndex maps GroupKey to UpdateGroup for efficient lookup.
// When enabled is false, all methods are no-ops and callers fall back
// to per-peer behavior.
//
// Safe for concurrent use. Add/Remove are called from peer goroutines
// (via notifyPeerEstablished/notifyPeerClosed), while GroupsForPeers
// is called from the reactor API path.
type UpdateGroupIndex struct {
	mu      sync.Mutex
	groups  map[GroupKey]*UpdateGroup
	enabled bool
}

// NewUpdateGroupIndex creates an UpdateGroupIndex with the given enabled state.
func NewUpdateGroupIndex(enabled bool) *UpdateGroupIndex {
	return &UpdateGroupIndex{
		groups:  make(map[GroupKey]*UpdateGroup),
		enabled: enabled,
	}
}

// NewUpdateGroupIndexFromEnv creates an UpdateGroupIndex, reading the enabled
// flag from the ze.bgp.reactor.update-groups environment variable (default true).
func NewUpdateGroupIndexFromEnv() *UpdateGroupIndex {
	enabled := env.GetBool("ze.bgp.reactor.update-groups", true)
	return NewUpdateGroupIndex(enabled)
}

// Enabled returns whether update grouping is active.
func (idx *UpdateGroupIndex) Enabled() bool {
	return idx.enabled
}

// Add registers a peer in the index under its current sendCtxID.
// Stores the GroupKey on the peer so Remove can find it even after
// sendCtxID is cleared. No-op when disabled or ctxID=0.
func (idx *UpdateGroupIndex) Add(peer *Peer) {
	if !idx.enabled {
		return
	}
	ctxID := peer.sendCtxID
	if ctxID == 0 {
		return // Not established or context not set
	}

	key := GroupKey{CtxID: ctxID, PolicyKey: 0}
	peer.updateGroupKey = key

	idx.mu.Lock()
	group, ok := idx.groups[key]
	if !ok {
		group = &UpdateGroup{Key: key}
		idx.groups[key] = group
	}
	group.Members = append(group.Members, peer)
	idx.mu.Unlock()
}

// Remove unregisters a peer from the index. Deletes the group if empty.
// Uses the GroupKey stored on the peer by Add, so it works even after
// sendCtxID has been cleared by clearEncodingContexts. No-op when disabled.
func (idx *UpdateGroupIndex) Remove(peer *Peer) {
	if !idx.enabled {
		return
	}
	key := peer.updateGroupKey
	if key.CtxID == 0 {
		return // Never added
	}

	// Clear stored key so double-Remove is safe.
	peer.updateGroupKey = GroupKey{}

	idx.mu.Lock()
	group, ok := idx.groups[key]
	if !ok {
		idx.mu.Unlock()
		return
	}

	// Remove peer from members slice (order doesn't matter, swap with last).
	for i, m := range group.Members {
		if m != peer {
			continue
		}
		last := len(group.Members) - 1
		group.Members[i] = group.Members[last]
		group.Members[last] = nil // avoid dangling pointer
		group.Members = group.Members[:last]
		break
	}

	if len(group.Members) == 0 {
		delete(idx.groups, key)
	}
	idx.mu.Unlock()
}

// GroupsForPeers returns the update groups formed by the given peer subset.
// Each returned UpdateGroup contains only peers from the input slice.
// Returns nil when disabled (caller falls back to per-peer loop).
func (idx *UpdateGroupIndex) GroupsForPeers(peers []*Peer) []UpdateGroup {
	if !idx.enabled {
		return nil
	}

	// Build temporary grouping from the provided peer subset.
	// We group by the peer's current sendCtxID, not by index membership,
	// because the caller may pass a filtered subset of peers.
	tmp := make(map[GroupKey][]*Peer)
	for _, peer := range peers {
		ctxID := peer.sendCtxID
		if ctxID == 0 {
			continue
		}
		key := GroupKey{CtxID: ctxID, PolicyKey: 0}
		tmp[key] = append(tmp[key], peer)
	}

	if len(tmp) == 0 {
		return nil
	}

	result := make([]UpdateGroup, 0, len(tmp))
	for key, members := range tmp {
		result = append(result, UpdateGroup{Key: key, Members: members})
	}
	return result
}
