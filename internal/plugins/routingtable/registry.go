// Design: plan/spec-gap-2-static-route-enhancements.md -- routing-table registry

package routingtable

import (
	"fmt"
	"sync/atomic"
)

// Registry maps routing table names to kernel table IDs.
// "default" is built-in (table 0, kernel RT_TABLE_MAIN 254).
type Registry struct {
	tables map[string]uint32
}

// New creates a Registry from a name-to-ID map.
func New(tables map[string]uint32) *Registry {
	return &Registry{tables: tables}
}

// Resolve returns the kernel table ID for a named routing table.
func (r *Registry) Resolve(name string) (uint32, error) {
	if name == "default" || name == "" {
		return 0, nil
	}
	if r == nil || r.tables == nil {
		return 0, fmt.Errorf("routing-table %q: not found", name)
	}
	id, ok := r.tables[name]
	if !ok {
		return 0, fmt.Errorf("routing-table %q: not found", name)
	}
	return id, nil
}

// ValidateTableID checks that a table ID is in the allowed range.
// Reserved: 0 (use "default"), 253 (RT_TABLE_DEFAULT), 254 (RT_TABLE_MAIN), 255 (RT_TABLE_LOCAL).
func ValidateTableID(id uint32) (uint32, error) {
	if id == 0 {
		return 0, fmt.Errorf("table ID 0 is reserved (use name \"default\")")
	}
	if id >= 253 && id <= 255 {
		return 0, fmt.Errorf("table ID %d is reserved (253=default, 254=main, 255=local)", id)
	}
	return id, nil
}

var globalRegistry atomic.Pointer[Registry]

// GetRegistry returns the current routing-table registry.
func GetRegistry() *Registry {
	return globalRegistry.Load()
}

// SetRegistry sets the global routing-table registry.
func SetRegistry(r *Registry) {
	globalRegistry.Store(r)
}
