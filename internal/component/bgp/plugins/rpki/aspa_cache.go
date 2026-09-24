// Design: docs/architecture/plugin/rib-storage-design.md -- ASPA record cache
// Overview: rpki.go -- plugin using this cache for ASPA path verification
// Related: roa_cache.go -- ROA cache following the same pattern
package rpki

import (
	"maps"
	"slices"
	"sync"
)

// ASPARecord holds an ASPA record received via RTR: customer AS and its authorized provider set.
// draft-ietf-sidrops-8210bis Section 5.12 distributes these records from cache to router.
type ASPARecord struct {
	CustomerAS uint32
	Providers  []uint32
}

// hopResult is the outcome of checking a single hop pair against the ASPA database.
// draft-ietf-sidrops-aspa-verification Section 5.3: provider authorization check.
type hopResult uint8

const (
	// HopProviderPlus: customer has ASPA record AND provider candidate is in the set.
	HopProviderPlus hopResult = iota
	// HopNotProviderPlus: customer has ASPA record AND provider candidate is NOT in the set.
	HopNotProviderPlus
	// HopNoAttestation: customer has no ASPA record.
	HopNoAttestation
)

// maxASPARecords is the upper bound on customer-AS entries to prevent unbounded growth.
const maxASPARecords = 1_000_000

// aSPACache stores ASPA records indexed by customer AS for provider authorization lookups.
// Thread-safe for concurrent read/write access. Separate from ROACache.
type aSPACache struct {
	records map[uint32]map[uint32]struct{}
	mu      sync.RWMutex
}

// newASPACache creates an empty ASPA cache.
func newASPACache() *aSPACache {
	return &aSPACache{
		records: make(map[uint32]map[uint32]struct{}),
	}
}

// Set stores or replaces the provider set for a customer AS.
// draft-ietf-sidrops-8210bis Section 5.12: an announcement is a full replacement.
func (c *aSPACache) Set(customerAS uint32, providers []uint32) {
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
func (c *aSPACache) Remove(customerAS uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.records, customerAS)
}

// HasRecord returns true if the customer AS has an ASPA record.
func (c *aSPACache) hasRecord(customerAS uint32) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.records[customerAS]
	return ok
}

// IsProvider returns true if providerAS is in the provider set for customerAS.
func (c *aSPACache) isProvider(customerAS, providerAS uint32) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	providers, ok := c.records[customerAS]
	if !ok {
		return false
	}
	_, found := providers[providerAS]
	return found
}

// checkPair checks a single hop pair for ASPA authorization.
// draft-ietf-sidrops-aspa-verification Section 5.3: provider authorization check.
func (c *aSPACache) checkPair(providerCandidate, customerAS uint32) hopResult {
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
func (c *aSPACache) ApplyDelta(dels []uint32, adds []ASPARecord) {
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

// Replace swaps the whole record set for recs in one lock acquisition and returns every
// customer AS that was in the old set, the new set, or both. That is the set of customers
// whose paths a caller MUST verify again.
//
// It is the ASPA half of what ROACache.Replace does for VRPs, and it exists for the same
// reason: a full sync carries a cache server's whole set, so a record ze holds and the set
// does not name is withdrawn rather than unchanged.
func (c *aSPACache) Replace(recs []ASPARecord) []uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(recs) == 0 {
		changed := slices.Collect(maps.Keys(c.records))
		c.records = make(map[uint32]map[uint32]struct{})
		return changed
	}

	changed := make(map[uint32]struct{}, len(c.records)+len(recs))
	for customerAS := range c.records {
		changed[customerAS] = struct{}{}
	}

	c.records = make(map[uint32]map[uint32]struct{}, len(recs))
	dropped := 0
	for _, rec := range recs {
		if len(c.records) >= maxASPARecords {
			dropped++
			continue
		}
		provSet := make(map[uint32]struct{}, len(rec.Providers))
		for _, p := range rec.Providers {
			provSet[p] = struct{}{}
		}
		c.records[rec.CustomerAS] = provSet
		changed[rec.CustomerAS] = struct{}{}
	}

	// One line for the whole sync. A cache that overruns the limit overruns it for every
	// record after the limit, and a line for each would be a million lines.
	if dropped > 0 {
		logger().Warn("aspa: cache full, records dropped from the full sync",
			"dropped", dropped, "limit", maxASPARecords)
	}

	return slices.Collect(maps.Keys(changed))
}

// Clear removes all ASPA records.
func (c *aSPACache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = make(map[uint32]map[uint32]struct{})
}

// Count returns the number of customer-AS entries.
func (c *aSPACache) count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.records)
}

// ASPADiagEntry is an ASPA record for diagnostic output.
type ASPADiagEntry struct {
	CustomerAS uint32
	Providers  []uint32
}

// Entries returns up to limit ASPA records for diagnostic display, in ascending
// customer-AS order with each provider set ascending. Pass 0 for all.
//
// The order is sorted rather than the Go map iteration order because these rows
// reach an operator through `show bgp rpki aspa`, whose answer shape publishes
// the row operators "first", "last" and "display". A map range answers a
// different order on every call, and past the limit a different SUBSET. The sort
// runs on a command goroutine; no verification path reaches here, because
// aspa_verify.go takes checkPair.
func (c *aSPACache) Entries(limit int) []ASPADiagEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	customers := slices.Sorted(maps.Keys(c.records))
	if limit > 0 && limit < len(customers) {
		customers = customers[:limit]
	}
	result := make([]ASPADiagEntry, 0, len(customers))
	for _, customerAS := range customers {
		result = append(result, ASPADiagEntry{
			CustomerAS: customerAS,
			Providers:  slices.Sorted(maps.Keys(c.records[customerAS])),
		})
	}
	return result
}

// lookupCustomer returns the provider set for a customer AS in ascending order,
// or nil if not found. The order is sorted for the reason Entries is: the answer
// becomes the provider list of a `show bgp rpki aspa <customer-asn>` row
// (rpki.go, aspaCommand), and a map range moves it on every call.
func (c *aSPACache) lookupCustomer(customerAS uint32) []uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	provSet, ok := c.records[customerAS]
	if !ok {
		return nil
	}
	// aspaCommand reads nil as "no record", so a record that authorizes no
	// provider MUST answer an empty slice rather than the nil slices.Sorted
	// would give it.
	providers := make([]uint32, 0, len(provSet))
	for p := range provSet {
		providers = append(providers, p)
	}
	slices.Sort(providers)
	return providers
}

// changedCustomers returns the set of customer ASNs affected by a delta.
// Used by the route tracker to determine which routes need re-validation.
func (c *aSPACache) changedCustomers(dels []uint32, adds []ASPARecord) []uint32 {
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
