// Design: docs/architecture/plugin/rib-storage-design.md -- sent ownership receipts.
package rib

import (
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/family"
)

// sentPathSources decodes external JSON receipts once per event, never once per
// route. Structured events borrow the typed sidecar directly from RawMessage.
func sentPathSources(meta map[string]any) ([]wireu.SentPathSource, error) {
	raw, exists := meta[bgptypes.SentPathSourcesMeta]
	if !exists {
		return nil, nil
	}
	if paths, ok := raw.([]wireu.SentPathSource); ok {
		return paths, nil
	}
	rows, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid sent path provenance %T", raw)
	}
	paths := make([]wireu.SentPathSource, 0, len(rows))
	for _, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid sent path record %T", rawRow)
		}
		name, ok := row["family"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid sent path family %T", row["family"])
		}
		fam, ok := family.LookupFamily(name)
		if !ok {
			return nil, fmt.Errorf("unknown sent path family %q", name)
		}
		ordinal, ordinalOK := sentUint32(row["ordinal"])
		pathID, pathOK := sentUint32(row["path-id"])
		if !ordinalOK || !pathOK {
			return nil, fmt.Errorf("invalid sent path ordinal or identifier")
		}
		paths = append(paths, wireu.SentPathSource{Family: fam, Ordinal: ordinal, PathID: pathID})
	}
	return paths, nil
}

func sentUint32(raw any) (uint32, bool) {
	switch value := raw.(type) {
	case uint32:
		return value, true
	case float64:
		if value >= 0 && value <= 4294967295 && float64(uint32(value)) == value {
			return uint32(value), true
		}
	}
	return 0, false
}

func sentSourceOwner(meta map[string]any) uint64 {
	text, ok := meta[bgptypes.SourceOwnerMeta].(string)
	if !ok {
		return 0
	}
	owner, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0
	}
	return owner
}

func setSentPathSource(entry *ribOutEntry, fam family.Family, ordinal uint32, paths []wireu.SentPathSource, index *int) {
	for *index < len(paths) {
		path := paths[*index]
		if path.Family != fam || path.Ordinal < ordinal {
			(*index)++
			continue
		}
		if path.Ordinal == ordinal {
			entry.SourcePath = path.PathID
			entry.SourceAddPath = true
			(*index)++
		}
		return
	}
}

// sentSourceState answers the two questions the RIB replay rails ask of the
// received route an Adj-RIB-Out copy was sent from, with one by-key lookup of
// the source's retained paths. Caller holds peerMu.
//
// current validates history against the mandatory receive inventory. It
// performs no selection: the same source, identifier presence/value and
// received revision must still exist. The short retained snapshot is released
// before any plugin dispatch.
//
// receivedLLGRStale is the ReceivedLLGRStale of the source's current path for
// the copy's add-path mode and path identifier, whatever its MsgID: a source
// that re-announced the route still LLGR_STALE makes the copy not current, and
// the route is still LLGR-stale (design review r3, N-1). It is read here, from
// the one holder of the fact, rather than copied onto ribOutEntry, so mark-stale
// and attach-community, which write the copy's StaleLevel, never reach it.
func (r *RIBManager) sentSourceState(fam family.Family, key ribOutKey, sent ribOutEntry) (current, receivedLLGRStale bool) {
	if sent.LocalOrigin {
		return sent.SourcePeer == "" && sent.SourceOwner == 0, false
	}
	source, err := netip.ParseAddr(sent.SourcePeer)
	if err != nil {
		return false, false
	}
	received := r.bgpPeers[source]
	if received == nil {
		return false, false
	}
	var inline [4]storage.PrefixPath
	var paths []storage.PrefixPath
	var addPath bool
	if storage.IsCIDRFamily(fam) {
		prefix := key.Prefix
		if fam.SAFI == family.SAFIMPLSLabel {
			_, parsed, valid := parsePrevKey(fam, []byte(key.Native), false)
			if !valid {
				return false, false
			}
			prefix = parsed
		}
		paths, addPath = received.AppendPrefixPathsRetained(fam, prefix, inline[:0])
	} else {
		paths, addPath = received.AppendKeyPathsRetained(fam, []byte(key.Native), inline[:0])
	}
	revisioned := sent.SourceOwner != 0 && sent.SourceMessageID != 0
	for i := range paths {
		path := &paths[i]
		if addPath == sent.SourceAddPath && path.PathID == sent.SourcePath {
			receivedLLGRStale = receivedLLGRStale || path.Entry.ReceivedLLGRStale
			current = current || (revisioned && path.Entry.MsgID == sent.SourceMessageID)
		}
		path.Release()
	}
	return current, receivedLLGRStale
}

// replayStaleLevel is the stale level a RIB rail hands the readvertise egress
// filters for one Adj-RIB-Out copy (storage.AdvertisedStaleLevel). Caller holds
// peerMu.
//
// RFC 9494 Section 4.3: the copy of a route received with LLGR_STALE travels at
// the LLGR stale level, which LLGREgressFilter withholds from a neighbor without
// LLGR.
func (r *RIBManager) replayStaleLevel(fam family.Family, key ribOutKey, sent ribOutEntry) uint8 {
	_, receivedLLGRStale := r.sentSourceState(fam, key, sent)
	return storage.AdvertisedStaleLevel(sent.StaleLevel, receivedLLGRStale)
}
