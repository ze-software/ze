// Design: docs/architecture/core-design.md — firewall table registry

package firewall

import (
	"errors"
	"sort"
	"sync"
)

var errFirewallBackendNotLoaded = errors.New("firewall backend not loaded")

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
// backend.Apply with the full set. Tables with the same Name and Family
// from different owners are merged: their Chains, Sets, and Flowtables
// are concatenated so that e.g. a plugin can register sets for a table
// whose chains are owned by the firewall engine.
func ApplyAll() error {
	tableRegistry.mu.Lock()
	totalCap := 0
	for _, t := range tableRegistry.owners {
		totalCap += len(t)
	}
	all := make([]Table, 0, totalCap)
	owners := make([]string, 0, len(tableRegistry.owners))
	for owner := range tableRegistry.owners {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	for _, owner := range owners {
		all = append(all, tableRegistry.owners[owner]...)
	}
	tableRegistry.mu.Unlock()
	all = mergeSameNameTables(all)

	backendsMu.Lock()
	b := activeBackend
	backendsMu.Unlock()

	if b == nil {
		return errFirewallBackendNotLoaded
	}
	return b.Apply(all)
}

type tableKey struct {
	name   string
	family TableFamily
}

func mergeSameNameTables(tables []Table) []Table {
	if len(tables) <= 1 {
		return tables
	}
	groups := make(map[tableKey]int, len(tables))
	merged := make([]Table, 0, len(tables))
	for _, t := range tables {
		k := tableKey{t.Name, t.Family}
		if idx, ok := groups[k]; ok {
			merged[idx].Chains = append(merged[idx].Chains, t.Chains...)
			merged[idx].Sets = append(merged[idx].Sets, t.Sets...)
			merged[idx].Flowtables = append(merged[idx].Flowtables, t.Flowtables...)
		} else {
			groups[k] = len(merged)
			merged = append(merged, t)
		}
	}
	return merged
}
