// Design: docs/architecture/core-design.md -- Applied firewall state snapshot
// Related: backend.go -- Backend interface and registry (what runs Apply)

package firewall

import "sync/atomic"

// lastApplied holds the desired-state snapshot from the most recent
// successful backend.Apply. Readers (CLI show handlers, operational
// RPCs) consume it without going back to the kernel. Engine writes
// it via StoreLastApplied under the package lock.
//
// We use atomic.Pointer[[]Table] so reads never block and are safe
// across goroutines. The pointed-to slice is immutable once stored --
// callers who mutate what they receive violate the contract.
var lastApplied atomic.Pointer[[]Table]

// StoreLastApplied records the tables just sent to backend.Apply.
// Called by the engine only after Apply returns nil, so readers never
// see a partially-applied snapshot. Passing nil clears the snapshot.
//
// Does a deep copy of the table tree (chains, terms, matches, actions,
// sets, elements, flowtables) so a later mutation by the caller cannot
// corrupt the snapshot observed by readers. Today the engine does not
// mutate the parsed config after Apply, but the invariant "readers see
// an immutable snapshot" is enforced here rather than relying on
// caller discipline.
func StoreLastApplied(tables []Table) {
	if tables == nil {
		lastApplied.Store(nil)
		return
	}
	cp := deepCopyTables(tables)
	lastApplied.Store(&cp)
}

// deepCopyTables returns an independent copy of the table tree. Only
// the leaf `Match` / `Action` values are shared -- they are interface
// values carrying small value types (netip.Prefix, port ranges, string
// names) that are themselves immutable, so aliasing them is safe.
func deepCopyTables(src []Table) []Table {
	dst := make([]Table, len(src))
	for i := range src {
		dst[i] = Table{
			Name:   src[i].Name,
			Family: src[i].Family,
		}
		if len(src[i].Chains) > 0 {
			dst[i].Chains = make([]Chain, len(src[i].Chains))
			for j := range src[i].Chains {
				c := src[i].Chains[j]
				dst[i].Chains[j] = Chain{
					Name:     c.Name,
					IsBase:   c.IsBase,
					Type:     c.Type,
					Hook:     c.Hook,
					Priority: c.Priority,
					Policy:   c.Policy,
				}
				if len(c.Terms) > 0 {
					dst[i].Chains[j].Terms = make([]Term, len(c.Terms))
					for k := range c.Terms {
						t := c.Terms[k]
						nt := Term{Name: t.Name}
						if len(t.Matches) > 0 {
							nt.Matches = make([]Match, len(t.Matches))
							copy(nt.Matches, t.Matches)
						}
						if len(t.Actions) > 0 {
							nt.Actions = make([]Action, len(t.Actions))
							copy(nt.Actions, t.Actions)
						}
						dst[i].Chains[j].Terms[k] = nt
					}
				}
			}
		}
		if len(src[i].Sets) > 0 {
			dst[i].Sets = make([]Set, len(src[i].Sets))
			for j := range src[i].Sets {
				s := src[i].Sets[j]
				ns := Set{Name: s.Name, Type: s.Type, Flags: s.Flags}
				if len(s.Elements) > 0 {
					ns.Elements = make([]SetElement, len(s.Elements))
					copy(ns.Elements, s.Elements)
				}
				dst[i].Sets[j] = ns
			}
		}
		if len(src[i].Flowtables) > 0 {
			dst[i].Flowtables = make([]Flowtable, len(src[i].Flowtables))
			for j := range src[i].Flowtables {
				f := src[i].Flowtables[j]
				nf := Flowtable{Name: f.Name, Hook: f.Hook, Priority: f.Priority}
				if len(f.Devices) > 0 {
					nf.Devices = make([]string, len(f.Devices))
					copy(nf.Devices, f.Devices)
				}
				dst[i].Flowtables[j] = nf
			}
		}
	}
	return dst
}

// LastApplied returns the most recently applied table set, or nil if
// no Apply has succeeded yet. The returned slice is immutable; do
// not mutate.
func LastApplied() []Table {
	p := lastApplied.Load()
	if p == nil {
		return nil
	}
	return *p
}

// activeBackendName tracks the name used in the most recent LoadBackend
// call. CLI handlers use it to tell "no backend configured" apart from
// "backend X does not support this read". Cleared when CloseBackend runs.
var activeBackendName atomic.Value // string

// setActiveBackendName is called from LoadBackend after the factory
// succeeds.
func setActiveBackendName(name string) {
	activeBackendName.Store(name)
}

// ActiveBackendName returns the name of the currently loaded backend,
// or "" when no backend is loaded.
func ActiveBackendName() string {
	v, _ := activeBackendName.Load().(string)
	return v
}

// StripZeTablePrefix removes the "ze_" ownership prefix from a kernel
// table name so CLI output matches the bare name the operator typed
// in config. Mirrors internal/component/firewall/cmd.StripPrefix but
// is exported from the firewall package for cross-component callers.
//
// The guard requires MORE than "ze_" -- a table name that is exactly
// "ze_" (prefix-only) is returned unchanged so it never collapses to
// the empty string in the CLI output. Such a name would fail
// ValidateName at verify, but defensive handling keeps the CLI
// self-consistent even against a corrupted snapshot.
func StripZeTablePrefix(name string) string {
	const p = "ze_"
	if len(name) > len(p) && name[:len(p)] == p {
		return name[len(p):]
	}
	return name
}
