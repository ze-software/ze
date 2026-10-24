// Design: docs/architecture/plugin/plugin-system.md -- registration fields, config needs
// Related: ../optional_provider.go -- appendHeldSettings, the walk over the config tree this check mirrors over the schema

// Package configneedtest holds test support for a plugin that declares
// registry.ConfigNeed values. Only test files import it.
//
// Register cannot see YANG, so it cannot tell that a need's path names a leaf
// the schema holds. A path that drifts from the YANG (a renamed leaf, a moved
// container) makes the need hold for no config: the provider is never pulled
// and its absence is never refused, and every test that only checks the
// refusal function stays green. Each owner package therefore runs
// RequirePathsInSchemaForTest against the schema its own YANG builds.
package configneedtest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// RequirePathsInSchemaForTest fails t unless every ConfigNeed of the plugin
// registered as owner names a leaf of the loaded YANG schema, reached through
// containers only. A plugin that is not registered, or that declares no need,
// fails too: the caller asked for a check that has nothing to judge.
//
// The schema is config.YANGSchema, so the caller's test binary MUST link the
// owner's YANG module, which the owner's own package test always does.
func RequirePathsInSchemaForTest(t testing.TB, owner string) {
	t.Helper()

	reg := registry.Lookup(owner)
	if reg == nil {
		t.Fatalf("plugin %q is not registered in this test binary, so its config needs cannot be checked", owner)
	}
	if len(reg.ConfigNeeds) == 0 {
		t.Fatalf("plugin %q declares no config need, so there is no path to check", owner)
	}

	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load the YANG schema: %v", err)
	}
	for _, need := range reg.ConfigNeeds {
		if fault := PathFault(schema, need.Path); fault != nil {
			t.Errorf("plugin %q need %q: %v", owner, strings.Join(need.Path, " "), fault)
		}
	}
}

// errListWithoutEntry reports a need path that names a YANG list and does not
// follow it with registry.AnyEntry. The refusal walks the lowered config
// (config.appendHeldSettings), where a list is a map keyed by the operator's
// entry names, so a keyword after the list would be looked up as an entry key
// and the need would never hold.
var errListWithoutEntry = errors.New("names a YANG list, and the next path element must be " +
	`"` + registry.AnyEntry + `" so the need holds for any entry of it`)

// errEntryWithoutList reports registry.AnyEntry after a node that is not a
// list: there are no entries for it to stand for.
var errEntryWithoutList = errors.New(`follows a node that is not a YANG list, so "` +
	registry.AnyEntry + `" has no entries to stand for`)

// PathFault answers why path cannot carry a config need in schema: an element
// the schema does not hold, a list not followed by registry.AnyEntry, an
// AnyEntry after a node that is not a list, a leaf or other non-container node
// before the last element, or a last element that is not a leaf. It answers
// nil when path names a leaf reached through containers, and through lists
// each followed by AnyEntry. The walk is bounded by len(path).
func PathFault(schema *config.Schema, path []string) error {
	if len(path) == 0 {
		return errors.New("the path is empty")
	}

	var parent config.ChildProvider = schema
	last := len(path) - 1
	for i := 0; i < last; i++ {
		keyword := path[i]
		if keyword == registry.AnyEntry {
			return fmt.Errorf("%q under %q %w", keyword, strings.Join(path[:i], " "), errEntryWithoutList)
		}
		node := parent.Get(keyword)
		if node == nil {
			return fmt.Errorf("the schema has no node %q under %q", keyword, strings.Join(path[:i], " "))
		}
		entries, isList := listEntries(node)
		if isList {
			if path[i+1] != registry.AnyEntry {
				return fmt.Errorf("%q %w", keyword, errListWithoutEntry)
			}
			// The entry marker is consumed with its list, so the next keyword is
			// looked up inside the entry.
			i++
			parent = entries
			continue
		}
		container, isContainer := node.(*config.ContainerNode)
		if !isContainer {
			return fmt.Errorf("%q is not a container, so the path cannot continue through it", keyword)
		}
		parent = container
	}
	if path[last] == registry.AnyEntry {
		return fmt.Errorf("the path ends with %q, so it names a list entry and no leaf", registry.AnyEntry)
	}

	leaf := parent.Get(path[last])
	if leaf == nil {
		return fmt.Errorf("the schema has no node %q under %q", path[last], strings.Join(path[:last], " "))
	}
	if leaf.Kind() != config.NodeLeaf {
		return fmt.Errorf("%q is not a leaf, so no value of it can hold the need", path[last])
	}
	return nil
}

// listEntries answers the schema of one entry of node when node is a YANG
// list, plain or inline, and false for any other node.
func listEntries(node config.Node) (config.ChildProvider, bool) {
	if list, isList := node.(*config.ListNode); isList {
		return list, true
	}
	if inline, isInline := node.(*config.InlineListNode); isInline {
		return inline, true
	}
	return nil, false
}
