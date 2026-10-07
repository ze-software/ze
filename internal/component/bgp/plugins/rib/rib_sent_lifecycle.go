// Design: docs/architecture/plugin/rib-storage-design.md -- one sent lifecycle inventory.
package rib

import (
	"encoding/binary"
	"net/netip"
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

// receivedOwner matches route identity and received generation, not egress
// ADD-PATH identifiers. A generation may advertise several paths for one prefix;
// their attributes and lifecycle are identical until one is freshly replaced.
type receivedOwner struct {
	key     ribOutKey
	message uint64
}

func receivedOwnerKey(fam family.Family, raw []byte, addPath bool, message uint64) (receivedOwner, bool) {
	if fam.SAFI == family.SAFIMPLSLabel {
		// The received FamilyRIB stores labeled routes as stripped CIDR bytes.
		fam.SAFI = family.SAFIUnicast
	}
	key, ok := ribOutRouteKey(fam, raw, addPath)
	key.PathID = 0
	return receivedOwner{key: key, message: message}, ok
}

// reconcileSentSourceLocked projects current received ownership onto the existing
// sent inventory. Its temporary index dies with this operation; it is not a
// second retained-route inventory. A zero family selects all represented families.
// Caller MUST hold peerMu and MUST dispatch returned writes after unlocking.
func (r *RIBManager) reconcileSentSourceLocked(source netip.Addr, selected family.Family, community []byte) []sentLifecycleWrite {
	families := make(map[family.Family]bool)
	sourcePeer := source.String()
	for _, destinations := range r.ribOut {
		for fam, routes := range destinations {
			if selected != (family.Family{}) && fam != selected {
				continue
			}
			for _, entry := range routes {
				if entry.SourcePeer == sourcePeer {
					families[fam] = true
					break
				}
			}
		}
	}
	var writes []sentLifecycleWrite
	for fam := range families {
		owners := make(map[receivedOwner]uint8)
		if received := r.bgpPeers[source]; received != nil {
			addPath := received.IsAddPath(fam)
			received.IterateFamily(fam, func(raw []byte, entry storage.RouteEntry) bool {
				owner, ok := receivedOwnerKey(fam, raw, addPath, entry.MsgID)
				if ok {
					owners[owner] = entry.StaleLevel
				}
				return true
			})
		}
		for destination, destinations := range r.ribOut {
			routes := destinations[fam]
			for key, entry := range routes {
				if entry.SourcePeer != sourcePeer {
					continue
				}
				identity := key
				identity.PathID = 0
				if fam.SAFI == family.SAFIMPLSLabel {
					_, cidr, err := nlrisplit.ExtractLabels([]byte(entry.NativeNLRI), entry.AddPath)
					if err != nil {
						continue
					}
					owner, ok := receivedOwnerKey(fam, cidr, entry.AddPath, entry.SourceMessageID)
					if !ok {
						continue
					}
					identity = owner.key
				}
				level, present := owners[receivedOwner{key: identity, message: entry.SourceMessageID}]
				if !present {
					// Withdrawal needs identity, not a decode/copy of the attributes.
					// The encoded command MUST own every byte before entry release.
					withdraw := ribOutEntry{NativeNLRI: entry.NativeNLRI, AddPath: entry.AddPath}
					command := bgp.FormatWithdrawCommand(reconstructRoute(withdraw, fam, key))
					delete(routes, key)
					entry.release()
					writes = append(writes, sentLifecycleWrite{peer: destination.String(), command: command, message: entry.MsgID})
					continue
				}
				if len(community) == 0 {
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
			if len(routes) == 0 {
				delete(destinations, fam)
			}
			if len(destinations) == 0 {
				delete(r.ribOut, destination)
			}
		}
	}
	return writes
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
