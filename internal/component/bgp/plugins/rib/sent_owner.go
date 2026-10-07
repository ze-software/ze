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

// sentSourceCurrent validates history against the mandatory receive inventory.
// It performs no selection: the same source, identifier presence/value and
// received revision must still exist. The short retained snapshot is released
// before any plugin dispatch. Caller holds peerMu.
func (r *RIBManager) sentSourceCurrent(fam family.Family, key ribOutKey, sent ribOutEntry) bool {
	if sent.LocalOrigin {
		return sent.SourcePeer == "" && sent.SourceOwner == 0
	}
	if sent.SourceOwner == 0 || sent.SourceMessageID == 0 {
		return false
	}
	source, err := netip.ParseAddr(sent.SourcePeer)
	if err != nil {
		return false
	}
	received := r.bgpPeers[source]
	if received == nil {
		return false
	}
	var inline [4]storage.PrefixPath
	var paths []storage.PrefixPath
	var addPath bool
	if storage.IsCIDRFamily(fam) {
		prefix := key.Prefix
		if fam.SAFI == family.SAFIMPLSLabel {
			_, parsed, valid := parsePrevKey(fam, []byte(key.Native), false)
			if !valid {
				return false
			}
			prefix = parsed
		}
		paths, addPath = received.AppendPrefixPathsRetained(fam, prefix, inline[:0])
	} else {
		paths, addPath = received.AppendKeyPathsRetained(fam, []byte(key.Native), inline[:0])
	}
	present := false
	for i := range paths {
		path := &paths[i]
		present = present || (addPath == sent.SourceAddPath && path.PathID == sent.SourcePath &&
			path.Entry.MsgID == sent.SourceMessageID)
		path.Release()
	}
	return present
}
