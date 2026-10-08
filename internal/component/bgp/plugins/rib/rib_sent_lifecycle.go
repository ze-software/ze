// Design: docs/architecture/plugin/rib-storage-design.md -- one sent lifecycle inventory.
package rib

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"strconv"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// sentLifecycleWrite owns encoded command bytes, never borrowed pool storage.
// The collecting caller MUST hold peerMu; it MUST dispatch after releasing it.
type sentLifecycleWrite struct {
	peer    string
	command string
	message uint64
}

// receivedOwner retains native identity, received ADD-PATH presence/identifier,
// and generation. One UPDATE can carry several paths for the same prefix.
type receivedOwner struct {
	key     ribOutKey
	message uint64
	addPath bool
}

func receivedOwnerKey(fam family.Family, raw []byte, addPath bool, message uint64) (receivedOwner, bool) {
	if fam.SAFI == family.SAFIMPLSLabel {
		// The received FamilyRIB stores labeled routes as stripped CIDR bytes.
		fam.SAFI = family.SAFIUnicast
	}
	key, ok := ribOutRouteKey(fam, raw, addPath)
	return receivedOwner{key: key, message: message, addPath: addPath}, ok
}

// removedReceivedOwners is an operation-local index, never a retained inventory.
// Its zero value selects nothing. Callers MUST collect it before removing the
// received entries and while holding peerMu.
type removedReceivedOwners map[family.Family]map[receivedOwner]struct{}

func (removed *removedReceivedOwners) add(fam family.Family, raw []byte, addPath bool, message uint64) {
	owner, ok := receivedOwnerKey(fam, raw, addPath, message)
	if !ok {
		return
	}
	if *removed == nil {
		*removed = make(removedReceivedOwners)
	}
	owners := (*removed)[fam]
	if owners == nil {
		owners = make(map[receivedOwner]struct{})
		(*removed)[fam] = owners
	}
	owners[owner] = struct{}{}
}

// sentReceivedOwner translates egress framing back to the existing ingress
// receipt. Egress PathID, including zero, is never an ingress identifier.
func sentReceivedOwner(fam family.Family, key ribOutKey, entry *ribOutEntry) (receivedOwner, bool) {
	if fam.SAFI == family.SAFIMPLSLabel {
		_, cidr, err := nlrisplit.ExtractLabels([]byte(entry.NativeNLRI), entry.AddPath)
		if err != nil {
			return receivedOwner{}, false
		}
		owner, ok := receivedOwnerKey(fam, cidr, entry.AddPath, entry.SourceMessageID)
		if !ok {
			return receivedOwner{}, false
		}
		key = owner.key
	}
	key.PathID = entry.SourcePath
	return receivedOwner{key: key, message: entry.SourceMessageID, addPath: entry.SourceAddPath}, true
}

// withdrawRemovedSentLocked withdraws only the received owners this operation
// removed. A fresh received revision can precede both forwarding and sent
// projection; absence of an old revision alone is not a purge instruction.
// Caller MUST hold peerMu and MUST dispatch returned writes after unlocking.
func (r *RIBManager) withdrawRemovedSentLocked(source netip.Addr, removed removedReceivedOwners) []sentLifecycleWrite {
	var writes []sentLifecycleWrite
	sourcePeer := source.String()
	for destination, destinations := range r.ribOut {
		for fam, owners := range removed {
			routes := destinations[fam]
			for key, entry := range routes {
				if entry.SourcePeer != sourcePeer {
					continue
				}
				owner, ok := sentReceivedOwner(fam, key, &entry)
				if !ok {
					continue
				}
				if _, removed := owners[owner]; !removed {
					continue
				}
				writes = append(writes, sentRemoval(destination, fam, key, &entry))
				delete(routes, key)
				entry.release()
			}
			if len(routes) == 0 {
				delete(destinations, fam)
			}
		}
		if len(destinations) == 0 {
			delete(r.ribOut, destination)
		}
	}
	return writes
}

// pruneSentSourceLocked removes whole source families, not selected received
// generations. An empty keep list is the explicit release-routes operation.
// Caller MUST hold peerMu and MUST dispatch returned writes after unlocking.
func (r *RIBManager) pruneSentSourceLocked(source netip.Addr, keep []family.Family) []sentLifecycleWrite {
	var writes []sentLifecycleWrite
	sourcePeer := source.String()
	for destination, destinations := range r.ribOut {
		for fam, routes := range destinations {
			if slices.Contains(keep, fam) {
				continue
			}
			for key, entry := range routes {
				if entry.SourcePeer != sourcePeer {
					continue
				}
				writes = append(writes, sentRemoval(destination, fam, key, &entry))
				delete(routes, key)
				entry.release()
			}
			if len(routes) == 0 {
				delete(destinations, fam)
			}
		}
		if len(destinations) == 0 {
			delete(r.ribOut, destination)
		}
	}
	return writes
}

// sentRemoval MUST encode owned command bytes before the caller releases entry.
// It preserves the actual sent receipt for final-writer admission.
func sentRemoval(destination netip.Addr, fam family.Family, key ribOutKey, entry *ribOutEntry) sentLifecycleWrite {
	withdraw := ribOutEntry{NativeNLRI: entry.NativeNLRI, AddPath: entry.AddPath}
	return sentLifecycleWrite{peer: destination.String(),
		command: bgp.FormatWithdrawCommand(reconstructRoute(withdraw, fam, key)), message: entry.MsgID}
}

// attachSentSourceCommunityLocked decorates only matching stale received paths.
// The command deletes no received entry, so it MUST NOT infer withdrawals from
// an asynchronous sent projection. Caller MUST hold peerMu.
func (r *RIBManager) attachSentSourceCommunityLocked(source netip.Addr, fam family.Family, community []byte) {
	received := r.bgpPeers[source]
	if received == nil {
		return
	}
	owners := make(map[receivedOwner]uint8)
	addPath := received.IsAddPath(fam)
	received.IterateFamily(fam, func(raw []byte, entry storage.RouteEntry) bool {
		owner, ok := receivedOwnerKey(fam, raw, addPath, entry.MsgID)
		if ok {
			owners[owner] = entry.StaleLevel
		}
		return true
	})
	sourcePeer := source.String()
	for _, destinations := range r.ribOut {
		routes := destinations[fam]
		for key, entry := range routes {
			if entry.SourcePeer != sourcePeer {
				continue
			}
			owner, ok := sentReceivedOwner(fam, key, &entry)
			if !ok {
				continue
			}
			level, present := owners[owner]
			if !present {
				continue
			}
			if level < storage.DepreferenceThreshold {
				continue
			}
			if !attachSentCommunity(&entry, community) {
				continue
			}
			entry.StaleLevel = level
			routes[key] = entry
		}
	}
}

// attachSentCommunity replaces one owned reference to the deduplicated immutable
// attribute blob. It MUST never modify bytes returned by pool.RibOut.Get.
func attachSentCommunity(entry *ribOutEntry, community []byte) bool {
	wire, err := pool.RibOut.Get(entry.AttrHandle)
	if err != nil {
		return false
	}
	var replacement []byte
	found := false
	for offset := 0; offset < len(wire); {
		if len(wire)-offset < 3 {
			return false
		}
		header := 3
		length := int(wire[offset+2])
		if wire[offset]&0x10 != 0 {
			if len(wire)-offset < 4 {
				return false
			}
			header = 4
			length = int(binary.BigEndian.Uint16(wire[offset+2:]))
		}
		end := offset + header + length
		if end > len(wire) {
			return false
		}
		if wire[offset+1] == byte(attribute.AttrCommunity) {
			value := wire[offset+header : end]
			if containsCommunity(value, community) {
				return true
			}
			if len(value)+len(community) > 65535 {
				return false
			}
			newLength := len(value) + len(community)
			replacement = make([]byte, 0, len(wire)+len(community)+1)
			replacement = append(replacement, wire[:offset]...)
			flags := wire[offset] &^ 0x10
			if newLength > 255 {
				replacement = append(replacement, flags|0x10, byte(attribute.AttrCommunity), byte(newLength>>8), byte(newLength))
			} else {
				replacement = append(replacement, flags, byte(attribute.AttrCommunity), byte(newLength))
			}
			replacement = append(replacement, value...)
			replacement = append(replacement, community...)
			replacement = append(replacement, wire[end:]...)
			found = true
			break
		}
		offset = end
	}
	if !found {
		replacement = make([]byte, 0, len(wire)+3+len(community))
		replacement = append(replacement, wire...)
		replacement = appendAttr(replacement, byte(attribute.AttrCommunity), 0xc0, community)
	}
	handle, err := pool.RibOut.Intern(replacement)
	if err != nil {
		return false
	}
	entry.release()
	entry.AttrHandle = handle
	return true
}

func (r *RIBManager) dispatchSentLifecycle(writes []sentLifecycleWrite) {
	for _, write := range writes {
		meta := map[string]any{"rib-lifecycle": true, metaKeyReplay: true,
			bgptypes.SentOwnerMessageMeta: strconv.FormatUint(write.message, 10)}
		r.updateRouteWithMeta(write.peer, write.command, meta)
	}
}
