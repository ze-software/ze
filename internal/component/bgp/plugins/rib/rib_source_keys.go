// Design: docs/architecture/plugin/rib-storage-design.md -- independent forwarding and learned scopes.
package rib

import (
	"bytes"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// captureScopedSourceKeysLocked preserves only this operation's current native
// keys while manual forwarding can hide a different learned winner. It creates
// no history. Caller MUST hold peerMu and later reconcileSourceKeys unlocked.
func (r *RIBManager) captureScopedSourceKeysLocked(routes *storage.PeerRIB) []affectedPrefix {
	if len(r.ribInPool[bgpProtocolID]) == 0 {
		return nil
	}
	if routes == nil {
		return nil
	}
	var keys []affectedPrefix
	flags := routes.AddPathFamilies()
	for _, fam := range routes.Families() {
		if ribevents.IsFlowSpec(fam) {
			continue
		}
		routes.IterateFamily(fam, func(raw []byte, _ storage.RouteEntry) bool {
			keys = append(keys, affectedPrefix{fam: fam, nlriBytes: bytes.Clone(raw), addPath: flags[fam]})
			return true
		})
	}
	return keys
}

// reconcileSourceKeys MUST follow captureScopedSourceKeysLocked with peerMu
// released. Its finite operation-local key set is also its memory/work bound.
func (r *RIBManager) reconcileSourceKeys(keys []affectedPrefix) {
	for _, key := range keys {
		if change, changed := r.checkBestPathChange(key.fam, key.nlriBytes, key.addPath, nil); changed {
			publishBestChanges([]bestChangeEntry{change}, key.fam)
		}
	}
}
