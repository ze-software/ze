// Design: docs/architecture/core-design.md — routing-table name-to-ID registry

package routingtable

import (
	"errors"
	"fmt"
	"math"
	"sync/atomic"
)

// maxEncodableTableID is the largest table ID this build can program into the
// kernel. The netlink bindings carry a table ID in a Go int
// (netlink.Rule.Table, netlink.Route.Table) and emit the attribute only for
// non-negative values, so on a 32-bit build an ID above MaxInt32 turns
// negative and the table selection is dropped without an error: the rule ends
// up with RT_TABLE_UNSPEC and the route lands in RT_TABLE_MAIN. Reject it here
// instead. On the 64-bit targets Ze ships this bound is above every uint32 and
// never bites, so the full kernel-legal range stays available.
const maxEncodableTableID = uint64(math.MaxInt)

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
// It also rejects an ID this build cannot program without truncation.
func ValidateTableID(id uint32) (uint32, error) {
	return validateTableID(id, maxEncodableTableID)
}

// validateTableID takes the encodable bound as a parameter so the 32-bit
// rejection path stays testable on a 64-bit host, where maxEncodableTableID is
// above every uint32 and the bound can never be reached.
func validateTableID(id uint32, maxEncodable uint64) (uint32, error) {
	if id == 0 {
		return 0, errors.New("table ID 0 is reserved (use name \"default\")")
	}
	if id >= 253 && id <= 255 {
		return 0, fmt.Errorf("table ID %d is reserved (253=default, 254=main, 255=local)", id)
	}
	if uint64(id) > maxEncodable {
		return 0, fmt.Errorf("table ID %d exceeds %d, the largest this build can program through netlink", id, maxEncodable)
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
