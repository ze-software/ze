// Design: docs/architecture/update-building.md -- selected output in Adj-RIB-Out.
// Related: selected_write.go -- authenticated selected transaction.
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
)

// selectedOutput belongs to one current Adj-RIB-Out entry, never a tombstone.
// Its zero value denotes an entry written by another output rail. key is the
// original selection's native identity, before a destination policy edits it.
type selectedOutput struct {
	owner    plugin.SelectedOwner
	key      fwdPathKey
	revision uint64
	attempt  uint64
	kind     filterapi.ExportKind
}

// selectedSlots keeps the usual one output slot inline. Only a policy that
// emits several paths for one selected key needs a slice. This derived index
// has exactly the lifetime and population of its current Adj-RIB-Out entries.
type selectedSlots struct {
	first adjOutKey
	rest  []adjOutKey
}

// setSelected MUST run under table.mu beside setOwner, before an old entry can
// be returned to the table's slot pool. A nonselected replacement drops its
// predecessor's index membership in the same final writer transaction.
func (table *adjRIBOut) setSelected(key adjOutKey, entry *adjRIBOutRoute, next selectedOutput) {
	if entry.selection.owner.Done() != nil {
		table.dropSelected(key, entry.selection.key)
	}
	entry.selection = next
	if next.owner.Done() == nil {
		return
	}
	if table.selectedPaths == nil {
		table.selectedPaths = make(map[fwdPathKey]selectedSlots)
	}
	slots, found := table.selectedPaths[next.key]
	if !found {
		table.selectedPaths[next.key] = selectedSlots{first: key}
		return
	}
	slots.rest = append(slots.rest, key)
	table.selectedPaths[next.key] = slots
}

func (table *adjRIBOut) dropSelected(key adjOutKey, original fwdPathKey) {
	slots, found := table.selectedPaths[original]
	if !found {
		return
	}
	if slots.first == key {
		if len(slots.rest) == 0 {
			delete(table.selectedPaths, original)
			return
		}
		slots.first = slots.rest[len(slots.rest)-1]
		slots.rest[len(slots.rest)-1] = adjOutKey{}
		slots.rest = slots.rest[:len(slots.rest)-1]
		table.selectedPaths[original] = slots
		return
	}
	for i, candidate := range slots.rest {
		if candidate != key {
			continue
		}
		slots.rest[i] = slots.rest[len(slots.rest)-1]
		slots.rest[len(slots.rest)-1] = adjOutKey{}
		slots.rest = slots.rest[:len(slots.rest)-1]
		table.selectedPaths[original] = slots
		return
	}
}
