// Design: docs/architecture/plugin/rib-storage-design.md — ROA cache for RPKI validation
// Overview: rpki.go — plugin using this cache for validation
// Related: validate.go — validation algorithm using cache lookups
package rpki

import (
	"net"
	"sync"
)

// vrpEntry stores a single VRP record in the cache.
type vrpEntry struct {
	MaxLength uint8
	ASN       uint32
}

// ROACache stores Validated ROA Payloads (VRPs) indexed by prefix for efficient lookup.
// Thread-safe for concurrent read/write access.
type ROACache struct {
	// ipv4 stores VRPs indexed by prefix string (e.g. "10.0.0.0/8").
	// Each prefix maps to a slice of vrpEntry (multiple ROAs can cover same prefix).
	ipv4 map[string][]vrpEntry

	// ipv6 stores VRPs indexed by prefix string.
	ipv6 map[string][]vrpEntry

	total int // running count, avoids O(N) totalLocked() per add

	mu sync.RWMutex
}

// newROACache creates an empty ROA cache.
func newROACache() *ROACache {
	return &ROACache{
		ipv4: make(map[string][]vrpEntry),
		ipv6: make(map[string][]vrpEntry),
	}
}

// maxVRPs is the upper bound on total VRP entries to prevent unbounded growth.
// Global ROA table is typically 200K-500K entries; 1M provides ample headroom.
const maxVRPs = 1_000_000

// Add inserts a VRP into the cache. Silently drops if cache is full or duplicate.
func (c *ROACache) Add(vrp VRP) {
	if vrp.Prefix.IP == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key := vrp.Prefix.String()
	entry := vrpEntry{MaxLength: vrp.MaxLength, ASN: vrp.ASN}

	if vrp.Prefix.IP.To4() != nil {
		for _, e := range c.ipv4[key] {
			if e.ASN == entry.ASN && e.MaxLength == entry.MaxLength {
				return
			}
		}
		if c.total >= maxVRPs {
			logger().Warn("roa: cache full, dropping VRP", "prefix", key)
			return
		}
		c.ipv4[key] = append(c.ipv4[key], entry)
		c.total++
	} else {
		for _, e := range c.ipv6[key] {
			if e.ASN == entry.ASN && e.MaxLength == entry.MaxLength {
				return
			}
		}
		if c.total >= maxVRPs {
			logger().Warn("roa: cache full, dropping VRP", "prefix", key)
			return
		}
		c.ipv6[key] = append(c.ipv6[key], entry)
		c.total++
	}
}

// Remove deletes a VRP from the cache.
func (c *ROACache) Remove(vrp VRP) {
	if vrp.Prefix.IP == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key := vrp.Prefix.String()
	entry := vrpEntry{MaxLength: vrp.MaxLength, ASN: vrp.ASN}

	if vrp.Prefix.IP.To4() != nil {
		before := len(c.ipv4[key])
		c.ipv4[key] = removeEntry(c.ipv4[key], entry)
		c.total -= before - len(c.ipv4[key])
		if len(c.ipv4[key]) == 0 {
			delete(c.ipv4, key)
		}
	} else {
		before := len(c.ipv6[key])
		c.ipv6[key] = removeEntry(c.ipv6[key], entry)
		c.total -= before - len(c.ipv6[key])
		if len(c.ipv6[key]) == 0 {
			delete(c.ipv6, key)
		}
	}
}

// removeEntry removes a matching vrpEntry from a slice.
func removeEntry(entries []vrpEntry, target vrpEntry) []vrpEntry {
	for i, e := range entries {
		if e.ASN == target.ASN && e.MaxLength == target.MaxLength {
			return append(entries[:i], entries[i+1:]...)
		}
	}
	return entries
}

// findCovering returns all VRP entries that cover the given parsed prefix.
// A VRP covers a prefix if the VRP's prefix is equal to or shorter than
// the query prefix, and the query prefix falls within the VRP's address space.
//
// The caller parses the prefix, and that is load-bearing: a string this walk
// cannot read is not a query with an empty answer, it is a query ze cannot
// answer. Validate (validate.go) makes that distinction before it calls here.
func (c *ROACache) findCovering(ipnet *net.IPNet) []vrpEntry {
	if ipnet == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	prefixLen, bits := ipnet.Mask.Size()
	isV4 := bits == 32
	var table map[string][]vrpEntry
	if isV4 {
		table = c.ipv4
	} else {
		table = c.ipv6
	}

	// Check all possible covering prefixes (from most specific to /0).
	// For IPv4: at most 33 lookups. For IPv6: at most 129 lookups.
	var result []vrpEntry
	for pl := prefixLen; pl >= 0; pl-- {
		coverMask := net.CIDRMask(pl, bits)
		coverIP := ipnet.IP.Mask(coverMask)
		coverNet := net.IPNet{IP: coverIP, Mask: coverMask}
		coverKey := coverNet.String()

		if entries, ok := table[coverKey]; ok {
			result = append(result, entries...)
		}
	}

	return result
}

// Count returns the number of VRP entries (IPv4 count, IPv6 count).
func (c *ROACache) Count() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	v4 := 0
	for _, entries := range c.ipv4 {
		v4 += len(entries)
	}
	return v4, c.total - v4
}

// ApplyDelta atomically removes and adds VRPs in a single lock acquisition.
// This prevents concurrent readers from seeing a partial update.
func (c *ROACache) ApplyDelta(dels, adds []VRP) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, vrp := range dels {
		c.removeLocked(vrp)
	}
	for _, vrp := range adds {
		c.addLocked(vrp)
	}
}

// addLocked inserts a VRP. Caller must hold write lock.
func (c *ROACache) addLocked(vrp VRP) {
	if vrp.Prefix.IP == nil {
		return
	}
	key := vrp.Prefix.String()
	entry := vrpEntry{MaxLength: vrp.MaxLength, ASN: vrp.ASN}

	if vrp.Prefix.IP.To4() != nil {
		for _, e := range c.ipv4[key] {
			if e.ASN == entry.ASN && e.MaxLength == entry.MaxLength {
				return
			}
		}
		if c.total >= maxVRPs {
			return
		}
		c.ipv4[key] = append(c.ipv4[key], entry)
		c.total++
	} else {
		for _, e := range c.ipv6[key] {
			if e.ASN == entry.ASN && e.MaxLength == entry.MaxLength {
				return
			}
		}
		if c.total >= maxVRPs {
			return
		}
		c.ipv6[key] = append(c.ipv6[key], entry)
		c.total++
	}
}

// removeLocked deletes a VRP. Caller must hold write lock.
func (c *ROACache) removeLocked(vrp VRP) {
	if vrp.Prefix.IP == nil {
		return
	}
	key := vrp.Prefix.String()
	entry := vrpEntry{MaxLength: vrp.MaxLength, ASN: vrp.ASN}

	if vrp.Prefix.IP.To4() != nil {
		before := len(c.ipv4[key])
		c.ipv4[key] = removeEntry(c.ipv4[key], entry)
		c.total -= before - len(c.ipv4[key])
		if len(c.ipv4[key]) == 0 {
			delete(c.ipv4, key)
		}
	} else {
		before := len(c.ipv6[key])
		c.ipv6[key] = removeEntry(c.ipv6[key], entry)
		c.total -= before - len(c.ipv6[key])
		if len(c.ipv6[key]) == 0 {
			delete(c.ipv6, key)
		}
	}
}

// DiagEntry is a VRP record for diagnostic output.
type DiagEntry struct {
	Prefix    string
	MaxLength uint8
	ASN       uint32
}

// Entries returns up to limit VRP entries for diagnostic display.
// Combines IPv4 and IPv6 entries. Pass 0 for all entries.
func (c *ROACache) Entries(limit int) []DiagEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.total
	if limit > 0 && limit < total {
		total = limit
	}
	result := make([]DiagEntry, 0, total)

	for prefix, entries := range c.ipv4 {
		for _, e := range entries {
			result = append(result, DiagEntry{Prefix: prefix, MaxLength: e.MaxLength, ASN: e.ASN})
			if limit > 0 && len(result) >= limit {
				return result
			}
		}
	}
	for prefix, entries := range c.ipv6 {
		for _, e := range entries {
			result = append(result, DiagEntry{Prefix: prefix, MaxLength: e.MaxLength, ASN: e.ASN})
			if limit > 0 && len(result) >= limit {
				return result
			}
		}
	}
	return result
}

// Lookup returns all VRP entries covering the given prefix, formatted for diagnostics.
// Each entry's Prefix is the VRP's own prefix (the covering prefix), not the query.
func (c *ROACache) Lookup(prefix string) []DiagEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return nil
	}

	prefixLen, bits := ipnet.Mask.Size()
	isV4 := bits == 32
	var table map[string][]vrpEntry
	if isV4 {
		table = c.ipv4
	} else {
		table = c.ipv6
	}

	var result []DiagEntry
	for pl := prefixLen; pl >= 0; pl-- {
		coverMask := net.CIDRMask(pl, bits)
		coverIP := ipnet.IP.Mask(coverMask)
		coverNet := net.IPNet{IP: coverIP, Mask: coverMask}
		coverKey := coverNet.String()

		if entries, ok := table[coverKey]; ok {
			for _, e := range entries {
				result = append(result, DiagEntry{Prefix: coverKey, MaxLength: e.MaxLength, ASN: e.ASN})
			}
		}
	}
	return result
}

// Clear removes all VRP entries.
func (c *ROACache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ipv4 = make(map[string][]vrpEntry)
	c.ipv6 = make(map[string][]vrpEntry)
	c.total = 0
}
