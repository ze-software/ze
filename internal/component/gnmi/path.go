// Design: docs/architecture/api/architecture.md -- gNMI path translation
// Related: server.go -- gNMI server core

package gnmi

import (
	"fmt"
	"slices"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

const maxPathDepth = 64

// pathToSegments converts a gNMI Path to the config path tokens every Ze
// path walker reads. A list element's keys become the entry address the
// config codec reads (config.ListPathStep): the first key leaf's value, then,
// for a multi-leaf key, `name value` for each other key leaf the element
// names, in YANG key order. So
// /bfd/single-hop-session[peer=fe80::b][interface=zbl1]/profile becomes
// ["bfd", "single-hop-session", "fe80::b", "interface", "zbl1", "profile"].
//
// An element the schema does not hold as a list keeps the one-key rule: its
// single key value is the entry name, and more than one key is refused.
func pathToSegments(schema *zeconfig.Schema, p *gpb.Path) ([]string, error) {
	if p == nil {
		return nil, nil
	}
	if len(p.Elem) > maxPathDepth {
		return nil, errPathTooDeep
	}
	segments := make([]string, 0, len(p.Elem)*2)
	var current zeconfig.ChildProvider
	if schema != nil {
		current = schema
	}
	for _, elem := range p.Elem {
		if elem.Name == "" {
			return nil, errEmptyPathElement
		}
		segments = append(segments, elem.Name)
		node := schemaNode(current, elem.Name)
		if list, ok := node.(*zeconfig.ListNode); ok && list.HasMultiLeafKey() {
			address, err := multiLeafKeyAddress(elem, list)
			if err != nil {
				return nil, err
			}
			segments = append(segments, address...)
			current = list
			continue
		}
		if len(elem.Key) > 1 {
			return nil, errMultipleKeys
		}
		for _, v := range elem.Key {
			segments = append(segments, v)
		}
		current, _ = node.(zeconfig.ChildProvider)
	}
	return segments, nil
}

// multiLeafKeyAddress returns the entry address a list element's keys name:
// the first key leaf's value, which every address carries, then each other
// key leaf the element names. A key the list does not have is refused.
func multiLeafKeyAddress(elem *gpb.PathElem, list *zeconfig.ListNode) ([]string, error) {
	if len(elem.Key) == 0 {
		return nil, nil
	}
	leaves := list.KeyLeafNames()
	for name := range elem.Key {
		if !slices.Contains(leaves, name) {
			return nil, fmt.Errorf("gnmi: %s has no key %q (its key is %s)", elem.Name, name, list.KeyName)
		}
	}
	first, ok := elem.Key[leaves[0]]
	if !ok {
		return nil, fmt.Errorf("gnmi: %s needs key %s, the first leaf of its key %s", elem.Name, leaves[0], list.KeyName)
	}
	address := []string{first}
	for _, leaf := range leaves[1:] {
		if value, named := elem.Key[leaf]; named {
			address = append(address, leaf, value)
		}
	}
	return address, nil
}

// schemaNode returns the schema child name of parent, nil when parent is nil
// or holds no such child.
func schemaNode(parent zeconfig.ChildProvider, name string) zeconfig.Node {
	if parent == nil {
		return nil
	}
	return parent.Get(name)
}

// segmentsToPath converts config path tokens back to a gNMI Path. A list
// entry address, read by the config codec, becomes the element's keys, named
// from the schema: the first key leaf, then every other key leaf, an absent
// one at its YANG default when it has one. A keyless list's entry is named
// by its position, under "name", because gNMI addresses an entry by a key.
func segmentsToPath(schema *zeconfig.Schema, segments []string) *gpb.Path {
	p := &gpb.Path{}
	var current zeconfig.ChildProvider
	if schema != nil {
		current = schema
	}
	for i := 0; i < len(segments); i++ {
		elem := &gpb.PathElem{Name: segments[i]}
		node := schemaNode(current, segments[i])
		if list, ok := node.(*zeconfig.ListNode); ok {
			addr, err := zeconfig.ListPathStep(segments[i], list, segments[i+1:])
			if err == nil && addr.Tokens > 0 {
				elem.Key = listEntryKeys(list, addr)
				i += addr.Tokens
			}
		}
		current, _ = node.(zeconfig.ChildProvider)
		p.Elem = append(p.Elem, elem)
	}
	return p
}

// listEntryKeys names an entry address's key values by their key leaves.
func listEntryKeys(list *zeconfig.ListNode, addr zeconfig.ListAddress) map[string]string {
	leaves := list.KeyLeafNames()
	if len(leaves) == 0 {
		return map[string]string{"name": addr.First}
	}
	keys := map[string]string{leaves[0]: addr.First}
	for _, leaf := range leaves[1:] {
		if value, ok := list.KeyLeafDefault(leaf); ok {
			keys[leaf] = value
		}
	}
	for _, pair := range addr.Pairs {
		keys[pair.Leaf] = pair.Value
	}
	return keys
}
