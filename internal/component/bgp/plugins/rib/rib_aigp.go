// Design: docs/architecture/route-selection.md
// RFC: rfc/short/rfc7311.md -- Sections 3.4.3 and 4.1
package rib

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// runAIGPSelection never enters BGP or Loc-RIB locks from an OnChange callback.
// Bursts coalesce, while a change during a scan leaves a second scan pending.
// The initial scan covers changes made before the subscription was installed.
func (r *RIBManager) runAIGPSelection(ctx context.Context) {
	if r.forkRIB != nil {
		r.runRemoteMetrics(ctx)
		return
	}
	loc := r.locRIB
	if loc == nil {
		return
	}
	changed := make(chan struct{}, 1)
	unsubscribe := loc.OnChange(func(_ locrib.Change) {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	defer unsubscribe()
	r.reselectAIGPRoutes()
	for {
		select {
		case <-ctx.Done():
			return
		case <-changed:
			r.reselectAIGPRoutes()
		}
	}
}

func (r *RIBManager) reselectAIGPRoutes() {
	// Copy keys while their pools are retained by PeerRIB.Iterate, then release
	// every read lock before checkBestPathChange can publish into Loc-RIB.
	var keys []affectedPrefix
	r.peerMu.RLock()
	for _, routes := range r.bgpPeers {
		addPath := routes.AddPathFamilies()
		routes.Iterate(func(fam family.Family, nlri []byte, entry storage.RouteEntry) bool {
			keys = append(keys, affectedPrefix{fam: fam, nlriBytes: bytes.Clone(nlri), addPath: addPath[fam]})
			return true
		})
	}
	r.peerMu.RUnlock()
	for _, key := range keys {
		change, changed := r.checkBestPathChange(key.fam, key.nlriBytes, key.addPath, nil)
		if changed {
			publishBestChanges([]bestChangeEntry{change}, key.fam)
		}
	}
}

// entryAIGP reads the first metric from OtherAttrs' type/flags/uint16-length
// storage framing. That framing differs from a BGP path-attribute header.
func entryAIGP(entry storage.RouteEntry) (uint64, bool) {
	bundle := entry.GetBundle()
	if !bundle.HasOtherAttrs() {
		return 0, false
	}
	data, err := pool.OtherAttrs.Get(bundle.OtherAttrs)
	if err != nil {
		return 0, false
	}
	for off := 0; off+4 <= len(data); {
		code := attribute.AttributeCode(data[off])
		length := int(binary.BigEndian.Uint16(data[off+2:]))
		off += 4
		if length > len(data)-off {
			return 0, false
		}
		if code == attribute.AttrAIGP {
			value := data[off : off+length]
			metric, err := attribute.AIGPMetricOffset(value)
			if err != nil {
				return 0, false
			}
			if metric < 0 {
				return 0, false
			}
			return binary.BigEndian.Uint64(value[metric:]), true
		}
		off += length
	}
	return 0, false
}
