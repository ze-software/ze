package firewall

import (
	"fmt"
	"sort"
	"sync"
)

var tableRegistry = struct {
	mu     sync.Mutex
	owners map[string][]Table
}{
	owners: make(map[string][]Table),
}

// RegisterTables stores a component's desired nftables tables under an
// owner key. Call ApplyAll to reconcile the merged set against the kernel.
func RegisterTables(owner string, tables []Table) {
	tableRegistry.mu.Lock()
	defer tableRegistry.mu.Unlock()
	if tables == nil {
		delete(tableRegistry.owners, owner)
		return
	}
	tableRegistry.owners[owner] = tables
}

// ApplyAll merges tables from all registered owners and calls
// backend.Apply with the full set. This ensures no component's
// Apply call deletes another component's tables.
func ApplyAll() error {
	tableRegistry.mu.Lock()
	var all []Table
	owners := make([]string, 0, len(tableRegistry.owners))
	for owner := range tableRegistry.owners {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	for _, owner := range owners {
		all = append(all, tableRegistry.owners[owner]...)
	}
	tableRegistry.mu.Unlock()

	backendsMu.Lock()
	b := activeBackend
	backendsMu.Unlock()

	if b == nil {
		return fmt.Errorf("firewall backend not loaded")
	}
	return b.Apply(all)
}
