// Design: docs/architecture/plugin/rib-storage-design.md -- Community attach/delete operations
// Related: rib_commands.go -- core command dispatch

package rib

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
)

var (
	errBgpRibClearInRequiresA                     = errors.New("bgp rib clear in requires a selector (* for all peers)")
	errBgpRibClearOutRequiresA                    = errors.New("bgp rib clear out requires a selector (* for all peers)")
	errBgpRibRetainRoutesRequiresA                = errors.New("bgp rib retain-routes requires a selector (* for all peers)")
	errBgpRibReleaseRoutesRequiresA               = errors.New("bgp rib release-routes requires a selector (* for all peers)")
	errUsageRibInjectPeerFamilyPrefix             = errors.New("usage: rib inject <peer> <family> <prefix> [origin <val>] [nhop <ip>] [aspath <asn,...>] [localpref <n>] [med <n>]")
	errUsageRibWithdrawPeerFamilyPrefix           = errors.New("usage: rib withdraw <peer> <family> <prefix>")
	errMarkStaleRequiresPeerRestartTime           = errors.New("mark-stale requires <peer> <restart-time> [level]")
	errStaleLevelMustBe00                         = errors.New("stale level must be > 0 (0 means fresh)")
	errPurgeStaleRequiresPeer                     = errors.New("purge-stale requires <peer>")
	errAttachCommunityRequiresPeerFamilyCommunity = errors.New("attach-community requires <peer> <family> <community-hex>")
	errDeleteWithCommunityRequiresPeerFamily      = errors.New("delete-with-community requires <peer> <family> <community-hex>")
)

// registerCommunityCommands registers generic community manipulation commands.
// Plugins compose these to implement protocol-specific behavior.
func registerCommunityCommands() {
	cmds := []struct {
		name    string
		help    string
		handler CommandHandler
	}{
		{"bgp rib attach-community", "Attach a community to stale routes for a peer family",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.attachCommunityCommand(args)
			}},
		{"bgp rib delete-with-community", "Delete stale routes that have a specific community",
			func(r *RIBManager, _ string, args []string) (string, string, error) {
				return r.deleteWithCommunityCommand(args)
			}},
	}
	for _, c := range cmds {
		if err := registerCommand(c.name, c.help, c.handler); err != nil {
			logger().Warn("community command registration failed", "command", c.name, "error", err)
		}
	}
}

// attachCommunityCommand handles "bgp rib attach-community <peer> <family> <community-hex>".
// Attaches a 4-byte community to all stale routes for the specified peer and family.
// Also raises StaleLevel to DepreferenceThreshold for attached routes.
// Args: [0]=peer, [1]=family, [2]=community as 8-char hex (e.g., "ffff0006").
func (r *RIBManager) attachCommunityCommand(args []string) (string, string, error) {
	if len(args) < 3 {
		return statusError, "", errAttachCommunityRequiresPeerFamilyCommunity
	}

	peerAddr := args[0]
	familyStr := args[1]
	commHex := args[2]

	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("invalid family %q", familyStr)
	}

	commBytes, err := hex.DecodeString(commHex)
	if err != nil || len(commBytes) != 4 {
		return statusError, "", fmt.Errorf("invalid community hex %q (must be 8 hex chars = 4 bytes)", commHex)
	}

	r.peerMu.Lock()

	peerRIB := r.bgpPeers[peerAddr]
	if peerRIB == nil {
		r.peerMu.Unlock()
		data, _ := json.Marshal(map[string]any{"attached": 0})
		return statusDone, string(data), nil
	}

	type affectedNLRI struct {
		nlri    []byte
		addPath bool
	}
	var affected []affectedNLRI
	ap := peerRIB.IsAddPath(fam)

	attached := 0
	peerRIB.ModifyFamilyAllKeyed(fam, func(nlriBytes []byte, entry *storage.RouteEntry) {
		if entry.StaleLevel == storage.StaleLevelFresh {
			return
		}
		if r.attachCommunity(entry, commBytes) {
			entry.StaleLevel = storage.DepreferenceThreshold
			attached++
			cp := make([]byte, len(nlriBytes))
			copy(cp, nlriBytes)
			affected = append(affected, affectedNLRI{nlri: cp, addPath: ap})
		}
	})

	r.peerMu.Unlock()

	for _, a := range affected {
		change, ok := r.checkBestPathChange(fam, a.nlri, a.addPath, nil)
		if ok {
			publishBestChanges([]bestChangeEntry{change}, fam)
		}
	}

	logger().Debug("attach-community", "peer", peerAddr, "family", familyStr,
		"community", commHex, "attached", attached)

	data, _ := json.Marshal(map[string]any{"attached": attached})
	return statusDone, string(data), nil
}

// deleteWithCommunityCommand handles "bgp rib delete-with-community <peer> <family> <community-hex>".
// Deletes stale routes that contain the specified community.
// Args: [0]=peer, [1]=family, [2]=community as 8-char hex.
func (r *RIBManager) deleteWithCommunityCommand(args []string) (string, string, error) {
	if len(args) < 3 {
		return statusError, "", errDeleteWithCommunityRequiresPeerFamily
	}

	peerAddr := args[0]
	familyStr := args[1]
	commHex := args[2]

	fam, ok := parseFamily(familyStr)
	if !ok {
		return statusError, "", fmt.Errorf("invalid family %q", familyStr)
	}

	commBytes, err := hex.DecodeString(commHex)
	if err != nil || len(commBytes) != 4 {
		return statusError, "", fmt.Errorf("invalid community hex %q (must be 8 hex chars = 4 bytes)", commHex)
	}

	r.peerMu.Lock()

	peerRIB := r.bgpPeers[peerAddr]
	if peerRIB == nil {
		r.peerMu.Unlock()
		data, _ := json.Marshal(map[string]any{"deleted": 0})
		return statusDone, string(data), nil
	}

	ap := peerRIB.IsAddPath(fam)

	// Collect NLRIs to delete (avoid modifying during iteration)
	var toDelete [][]byte
	peerRIB.IterateFamily(fam, func(nlriBytes []byte, entry storage.RouteEntry) bool {
		if entry.StaleLevel == storage.StaleLevelFresh {
			return true
		}
		if entry.HasCommunities() {
			if data, getErr := pool.Communities.Get(entry.Communities); getErr == nil {
				if containsCommunity(data, commBytes) {
					nlriCopy := make([]byte, len(nlriBytes))
					copy(nlriCopy, nlriBytes)
					toDelete = append(toDelete, nlriCopy)
				}
			}
		}
		return true
	})

	deleted := 0
	for _, nlriBytes := range toDelete {
		if peerRIB.Remove(fam, nlriBytes) {
			deleted++
		}
	}

	r.peerMu.Unlock()

	for _, nlriBytes := range toDelete {
		change, ok := r.checkBestPathChange(fam, nlriBytes, ap, nil)
		if ok {
			publishBestChanges([]bestChangeEntry{change}, fam)
		}
	}

	logger().Debug("delete-with-community", "peer", peerAddr, "family", familyStr,
		"community", commHex, "deleted", deleted)

	data, _ := json.Marshal(map[string]any{"deleted": deleted})
	return statusDone, string(data), nil
}

// containsCommunity checks if a community wire blob contains a specific 4-byte community.
func containsCommunity(data, community []byte) bool {
	if len(data)%4 != 0 || len(community) != 4 {
		return false
	}
	for i := 0; i+4 <= len(data); i += 4 {
		if data[i] == community[0] && data[i+1] == community[1] &&
			data[i+2] == community[2] && data[i+3] == community[3] {
			return true
		}
	}
	return false
}

// attachCommunity appends a 4-byte community to a route's community attribute.
// If no community attribute exists, creates one. Idempotent: skips if already present.
// Pool handles are updated: old handle released, new handle interned.
// Returns true on success (or already present).
func (r *RIBManager) attachCommunity(entry *storage.RouteEntry, comm []byte) bool {
	var newData []byte

	if entry.HasCommunities() {
		oldData, err := pool.Communities.Get(entry.Communities)
		if err != nil {
			return false
		}
		if containsCommunity(oldData, comm) {
			return true
		}
		newData = make([]byte, len(oldData)+4)
		copy(newData, oldData)
		copy(newData[len(oldData):], comm)
	} else {
		newData = make([]byte, 4)
		copy(newData, comm)
	}

	newHandle, err := pool.Communities.Intern(newData)
	if err != nil {
		return false
	}

	if entry.HasCommunities() {
		_ = pool.Communities.Release(entry.Communities)
	}
	entry.Communities = newHandle
	return true
}
