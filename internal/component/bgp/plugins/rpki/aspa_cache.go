// Design: docs/architecture/plugin/rib-storage-design.md -- ASPA record cache
// Overview: rpki.go -- plugin using this cache for ASPA path verification
// Related: roa_cache.go -- ROA cache following the same pattern
package rpki

import "sync"

// ASPARecord holds an ASPA record received via RTR: customer AS and its authorized provider set.
// RFC 9582 Section 5.12: ASPA PDU distributes these records from cache to router.
type ASPARecord struct {
	CustomerAS uint32
	Providers  []uint32
}

// HopResult is the outcome of checking a single hop pair against the ASPA database.
// draft-ietf-sidrops-aspa-verification Section 6: check_pair function.
type HopResult uint8

const (
	// HopProviderPlus: customer has ASPA record AND provider candidate is in the set.
	HopProviderPlus HopResult = iota
	// HopNotProviderPlus: customer has ASPA record AND provider candidate is NOT in the set.
	HopNotProviderPlus
	// HopNoAttestation: customer has no ASPA record.
	HopNoAttestation
)

// maxASPARecords is the upper bound on customer-AS entries to prevent unbounded growth.
const maxASPARecords = 1_000_000

// ASPACache stores ASPA records indexed by customer AS for provider authorization lookups.
// Thread-safe for concurrent read/write access. Separate from ROACache.
type ASPACache struct {
	records map[uint32]map[uint32]struct{}
	mu      sync.RWMutex
}

// NewASPACache creates an empty ASPA cache.
func NewASPACache() *ASPACache {
	return &ASPACache{
		records: make(map[uint32]map[uint32]struct{}),
	}
}

// Set stores or replaces the provider set for a customer AS.
// RFC 9582 Section 5.12: announce = full replacement, not delta.
func (c *ASPACache) Set(customerAS uint32, providers []uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.records) >= maxASPARecords {
		if _, exists := c.records[customerAS]; !exists {
			logger().Warn("aspa: cache full, dropping record", "customer-as", customerAS)
			return
		}
	}

	provSet := make(map[uint32]struct{}, len(providers))
	for _, p := range providers {
		provSet[p] = struct{}{}
	}
	c.records[customerAS] = provSet
}

// Remove deletes the ASPA record for a customer AS.
func (c *ASPACache) Remove(customerAS uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.records, customerAS)
}

// HasRecord returns true if the customer AS has an ASPA record.
func (c *ASPACache) hasRecord(customerAS uint32) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.records[customerAS]
	return ok
}

// IsProvider returns true if providerAS is in the provider set for customerAS.
func (c *ASPACache) isProvider(customerAS, providerAS uint32) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	providers, ok := c.records[customerAS]
	if !ok {
		return false
	}
	_, found := providers[providerAS]
	return found
}

// CheckPair checks a single hop pair for ASPA authorization.
// draft-ietf-sidrops-aspa-verification Section 6: check_pair function.
func (c *ASPACache) CheckPair(providerCandidate, customerAS uint32) HopResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	providers, hasRecord := c.records[customerAS]
	if !hasRecord {
		return HopNoAttestation
	}
	if _, isProvider := providers[providerCandidate]; isProvider {
		return HopProviderPlus
	}
	return HopNotProviderPlus
}

// ApplyDelta atomically removes and adds ASPA records.
// Called at End of Data after accumulating ASPA PDUs.
func (c *ASPACache) ApplyDelta(dels []uint32, adds []ASPARecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, customerAS := range dels {
		delete(c.records, customerAS)
	}
	for _, rec := range adds {
		if len(c.records) >= maxASPARecords {
			if _, exists := c.records[rec.CustomerAS]; !exists {
				continue
			}
		}
		provSet := make(map[uint32]struct{}, len(rec.Providers))
		for _, p := range rec.Providers {
			provSet[p] = struct{}{}
		}
		c.records[rec.CustomerAS] = provSet
	}
}

// Clear removes all ASPA records.
func (c *ASPACache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = make(map[uint32]map[uint32]struct{})
}

// Count returns the number of customer-AS entries.
func (c *ASPACache) count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.records)
}

// ChangedCustomers returns the set of customer ASNs affected by a delta.
// Used by the route tracker to determine which routes need re-validation.
func (c *ASPACache) ChangedCustomers(dels []uint32, adds []ASPARecord) []uint32 {
	seen := make(map[uint32]struct{}, len(dels)+len(adds))
	for _, d := range dels {
		seen[d] = struct{}{}
	}
	for _, a := range adds {
		seen[a.CustomerAS] = struct{}{}
	}
	result := make([]uint32, 0, len(seen))
	for k := range seen {
		result = append(result, k)
	}
	return result
}
