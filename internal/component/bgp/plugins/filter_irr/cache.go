// Design: plan/spec-filter-irr.md -- zefs persistence for IRR prefix cache

package filter_irr

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

var irrCacheKey = zefs.KeyIRRCache.Pattern

type cachedASN struct {
	ASN   uint32   `json:"asn"`
	ASSet string   `json:"as-set"`
	IPv4  []string `json:"ipv4"`
	IPv6  []string `json:"ipv6"`
}

func cacheStorePath() string {
	configDir := env.Get("ze.config.dir")
	if configDir == "" {
		configDir = paths.DefaultConfigDir()
	}
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "database.zefs")
}

func (plug *irrPlugin) saveCache() {
	plug.saveCacheTo(cacheStorePath())
}

func (plug *irrPlugin) saveCacheTo(storePath string) {
	if storePath == "" {
		return
	}

	entries := plug.buildCacheEntries()

	data, err := json.Marshal(entries)
	if err != nil {
		logger().Warn("irr: cache marshal failed", "error", err)
		return
	}

	store, err := openStore(storePath)
	if err != nil {
		logger().Debug("irr: cache store unavailable", "error", err)
		return
	}
	defer func() { _ = store.Close() }()

	wl, err := store.Lock()
	if err != nil {
		logger().Warn("irr: cache lock failed", "error", err)
		return
	}
	if err := wl.WriteFile(irrCacheKey, data, 0); err != nil {
		logger().Warn("irr: cache write failed", "error", err)
	}
	if err := wl.Release(); err != nil {
		logger().Warn("irr: cache release failed", "error", err)
	}
}

func (plug *irrPlugin) buildCacheEntries() []cachedASN {
	plug.mu.RLock()
	defer plug.mu.RUnlock()

	entries := make([]cachedASN, 0, len(plug.byASN))
	for _, st := range plug.byASN {
		c := cachedASN{ASN: st.asn, ASSet: st.asSet}
		if st.list != nil {
			for _, e := range st.list.entries {
				if e.prefix.Addr().Is4() {
					c.IPv4 = append(c.IPv4, e.prefix.String())
				} else {
					c.IPv6 = append(c.IPv6, e.prefix.String())
				}
			}
		}
		entries = append(entries, c)
	}
	return entries
}

func (plug *irrPlugin) loadCache() {
	plug.loadCacheFrom(cacheStorePath())
}

func (plug *irrPlugin) loadCacheFrom(storePath string) {
	if storePath == "" {
		return
	}

	store, err := openStore(storePath)
	if err != nil {
		return
	}
	defer func() { _ = store.Close() }()

	data, err := store.ReadFile(irrCacheKey)
	if err != nil {
		return
	}

	plug.applyCacheData(data)
}

func (plug *irrPlugin) applyCacheData(data []byte) {
	var entries []cachedASN
	if err := json.Unmarshal(data, &entries); err != nil {
		logger().Warn("irr: cache parse failed", "error", err)
		return
	}

	plug.mu.Lock()
	defer plug.mu.Unlock()

	for _, c := range entries {
		st, exists := plug.byASN[c.ASN]
		if !exists {
			continue
		}
		if st.list != nil && len(st.list.entries) > 0 {
			continue
		}

		var pEntries []prefixEntry
		for _, s := range c.IPv4 {
			p, err := netip.ParsePrefix(s)
			if err == nil {
				pEntries = append(pEntries, prefixEntry{prefix: p, ge: uint8(p.Bits()), le: 32})
			}
		}
		for _, s := range c.IPv6 {
			p, err := netip.ParsePrefix(s)
			if err == nil {
				pEntries = append(pEntries, prefixEntry{prefix: p, ge: uint8(p.Bits()), le: 128})
			}
		}
		if len(pEntries) > 0 {
			st.list = &irrPrefixList{entries: pEntries}
			st.v4Count = len(c.IPv4)
			st.v6Count = len(c.IPv6)
			if c.ASSet != "" && st.asSet == "" {
				st.asSet = c.ASSet
			}
			logger().Info("irr: loaded from cache", "asn", c.ASN, "v4", len(c.IPv4), "v6", len(c.IPv6))
		}
	}
}

func openStore(path string) (*zefs.BlobStore, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return zefs.Open(path)
}
