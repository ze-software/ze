// Design: docs/architecture/static-routes.md -- the candidate interface section static reads beside candidateTables
// Related: config.go -- parseIfaceConfig, the full parser of the same section

package iface

import (
	"encoding/json"
	"fmt"
	"slices"
)

// dynamicKinds names the interface kinds whose device iface creates on demand,
// so a configured entry of the kind can have no device at a given moment. A
// pppoe-client's device exists only while its session is up. A kind iface later
// creates the same way joins this list, and no consumer changes: a consumer
// asks DeclaredDynamicNames and never names a kind.
var dynamicKinds = []string{"pppoe-client"}

// DeclaredNames is the set of interface names one candidate interface section
// declares as dynamic: names whose device iface creates on demand and that may
// be absent. The zero value declares no name. Safe for concurrent reads; it is
// never written after DeclaredDynamicNames returns it.
type DeclaredNames struct {
	names map[string]struct{}
}

// Has reports whether name is declared dynamic by the section the set came from.
func (d DeclaredNames) Has(name string) bool {
	_, declared := d.names[name]
	return declared
}

// Names returns the declared names in sorted order.
func (d DeclaredNames) Names() []string {
	out := make([]string, 0, len(d.names))
	for name := range d.names {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// DeclaredDynamicNames answers, from the data of one delivered "interface"
// section, the names of the entries whose device iface creates on demand.
//
// It is pure: it reads no installed config, no resolver mapping and no kernel
// state. A consumer verifying a candidate (the static plugin) therefore gets the
// candidate's answer whichever participant applies first, because the engine
// does not order participants within an apply
// (docs/architecture/config/transaction-protocol.md, "Phase 2: Apply"), and it
// gets the same answer when it runs as an external process.
//
// A section without the interface container, which is how a deleted root
// arrives ("{}"), declares no name. Data that is not JSON, or a container or
// list of the wrong shape, is refused: the set cannot be answered from it.
func DeclaredDynamicNames(data string) (DeclaredNames, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return DeclaredNames{}, fmt.Errorf("interface section: unmarshal: %w", err)
	}
	declared := DeclaredNames{names: make(map[string]struct{})}
	container, present := root[configRootInterface]
	if present {
		if err := declared.addDynamicEntries(container); err != nil {
			return DeclaredNames{}, err
		}
	}
	return declared, nil
}

// addDynamicEntries records the key of every entry of a dynamic kind under the
// interface container. A list is a map keyed by its key (Tree.ToMap).
func (d DeclaredNames) addDynamicEntries(container any) error {
	ifaceMap, isMap := container.(map[string]any)
	if !isMap {
		return fmt.Errorf("interface section: %q is %T, not a container", configRootInterface, container)
	}
	for _, kind := range dynamicKinds {
		list, present := ifaceMap[kind]
		if !present {
			continue
		}
		entries, isList := list.(map[string]any)
		if !isList {
			return fmt.Errorf("interface section: %q is %T, not a list", kind, list)
		}
		for name := range entries {
			d.names[name] = struct{}{}
		}
	}
	return nil
}
