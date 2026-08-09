// Design: docs/architecture/bgp/filter-irr.md -- shared PrefixStore wiring for the IRR filter plugin

package filter_irr

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/core/paths"
)

// cacheStorePath returns the zefs file the shared PrefixStore persists to.
// It is empty when no config dir is known, in which case persistence is a
// no-op and the store works in memory only.
func cacheStorePath() string {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "database.zefs")
}

// loadFromStore applies cached prefix data from the shared PrefixStore to
// enrolled ASNs that have no in-memory list yet. Entries the plugin never
// enrolled (AS-SETs, other consumers' ASNs) are ignored -- this preserves the
// enrollment gate the previous single-blob cache enforced.
func (plug *irrPlugin) loadFromStore() {
	if plug.prefixStore == nil {
		return
	}
	plug.mu.Lock()
	defer plug.mu.Unlock()
	for asn, st := range plug.byASN {
		if st.list != nil && len(st.list.entries) > 0 {
			continue
		}
		entry := plug.prefixStore.Get(asnName(asn))
		if entry == nil {
			continue
		}
		pl := entry.PrefixList()
		entries := prefixListFromIRR(pl)
		if len(entries) == 0 {
			continue
		}
		st.list = &irrPrefixList{entries: entries}
		st.v4Count = len(pl.IPv4)
		st.v6Count = len(pl.IPv6)
		if st.asSet == "" && entry.ASSet != "" {
			st.asSet = entry.ASSet
		}
		// A cache hit gives this ASN a usable prefix-list as a FALLBACK, but it is
		// not a completed network resolution: firstDone stays open so the first
		// filter UPDATE still waits (bounded) for the fresh resolution and only
		// falls back to this cached list if the IRR server is slow/unreachable.
		// This keeps the cached data from being stale-but-authoritative for the
		// first UPDATE while still serving it when the network is down.
		logger().Info("irr: loaded from store", "asn", asn, "v4", len(pl.IPv4), "v6", len(pl.IPv6))
	}
}
