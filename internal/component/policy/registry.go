// Design: docs/guide/bgp-policy.md -- the policy root and named filter instances
// Related: fields.go -- the route field vocabulary a chain is checked against
// Related: register.go -- the policy plugin registration and the vocabulary

// Package policy is Ze's protocol-neutral route policy library. It owns the
// top-level policy root, discovers filter types from the ze:filter YANG
// marker, holds the namespace of named filter instances, and checks a chain
// against the route fields its attachment point carries.
package policy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
)

// RootName is the top-level config root the policy component owns. Filter
// types and policy routing both augment it.
const RootName = "policy"

// Instance is one named filter instance: an entry of a ze:filter list.
type Instance struct {
	Name string // the list key, unique across every filter type
	Type string // the filter type: the name of the ze:filter list
}

// Registry is the namespace of named filter instances under the policy root.
// It is built once per config load and read-only afterwards, so it is safe
// for concurrent reads.
type Registry struct {
	instances map[string]Instance
	types     map[string]bool   // ze:filter list names present in the schema
	others    map[string]string // entry name -> the unmarked list holding it
	otherHelp map[string]string // unmarked list name -> its ze:help summary
}

// BuildRegistry collects every filter instance under the policy root.
//
// A filter type is a list under the root whose schema node carries the
// ze:filter marker (config.ListNode.Filter); any other list (policy routing's
// route list) holds entries that are not filters, recorded only so a chain
// that names one is refused for what it is. One instance name under two filter
// types is refused, because a chain names an instance without its type.
//
// policyTree is the parsed policy root and may be nil when the config has
// none. policySchema is the policy root's schema node.
func BuildRegistry(policyTree *config.Tree, policySchema *config.ContainerNode) (*Registry, error) {
	reg := &Registry{
		instances: make(map[string]Instance),
		types:     make(map[string]bool),
		others:    make(map[string]string),
		otherHelp: make(map[string]string),
	}
	if policySchema == nil {
		return nil, fmt.Errorf("policy registry: no schema for the %s root", RootName)
	}

	for _, listName := range policySchema.Children() {
		list, ok := policySchema.Get(listName).(*config.ListNode)
		if !ok {
			continue
		}
		if list.Filter {
			reg.types[listName] = true
		} else {
			reg.otherHelp[listName] = list.ShortHelp
		}
		if policyTree == nil {
			continue
		}
		if err := reg.collect(listName, list.Filter, policyTree); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

// NewRegistry builds a namespace holding exactly the given instances, for a
// caller that knows them without a parsed policy root: a test of chain
// canonicalisation, or a tool replaying a recorded namespace. It refuses one
// name given twice, as BuildRegistry does.
func NewRegistry(instances ...Instance) (*Registry, error) {
	reg := &Registry{
		instances: make(map[string]Instance, len(instances)),
		types:     make(map[string]bool),
		others:    make(map[string]string),
		otherHelp: make(map[string]string),
	}
	for _, inst := range instances {
		if existing, dup := reg.instances[inst.Name]; dup {
			return nil, fmt.Errorf("duplicate filter name %q: given as both %s and %s", inst.Name, existing.Type, inst.Type)
		}
		reg.instances[inst.Name] = inst
		reg.types[inst.Type] = true
	}
	return reg, nil
}

// collect records the entries of one list under the policy root.
func (r *Registry) collect(listName string, filter bool, policyTree *config.Tree) error {
	for name := range policyTree.GetList(listName) {
		if !filter {
			r.others[name] = listName
			continue
		}
		if existing, dup := r.instances[name]; dup {
			return fmt.Errorf(
				"duplicate filter name %q: defined in both policy %s and policy %s; "+
					"rename one, because a filter chain names an instance without its type",
				name, existing.Type, listName,
			)
		}
		r.instances[name] = Instance{Name: name, Type: listName}
	}
	return nil
}

// Lookup returns the filter instance with the given name.
func (r *Registry) Lookup(name string) (Instance, bool) {
	inst, ok := r.instances[name]
	return inst, ok
}

// IsFilterType reports whether a list under the policy root is a filter type.
func (r *Registry) IsFilterType(listName string) bool {
	return r.types[listName]
}

// Names returns every filter instance name in sorted order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.instances))
	for n := range r.instances {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Len returns the number of filter instances.
func (r *Registry) Len() int {
	return len(r.instances)
}

// IsNonFilterEntry answers whether name is an entry under the policy root
// that is not a filter instance (a policy routing rule), the case
// ValidateName refuses by naming what the entry is rather than as unknown.
func (r *Registry) IsNonFilterEntry(name string) bool {
	_, ok := r.others[name]
	return ok
}

// ValidateName refuses a chain reference that names no filter instance.
// context prefixes the error with where the chain is attached. A name holding
// ":" is an external plugin filter (plugin:filter or type:name), resolved at
// runtime because plugins register at stage 1, after config parsing.
func (r *Registry) ValidateName(name, context string) error {
	if strings.Contains(name, ":") {
		return nil
	}
	if _, ok := r.instances[name]; ok {
		return nil
	}
	if listName, ok := r.others[name]; ok {
		return fmt.Errorf("%s: %q is a policy %s entry (%s), not a filter; "+
			"a chain names only entries of a filter type such as policy prefix-list",
			context, name, listName, r.otherHelp[listName])
	}
	return fmt.Errorf("%s: unknown filter %q; define it under policy { <filter-type> %s { ... } }",
		context, name, name)
}
