// Design: docs/architecture/config/transaction-protocol.md -- reconstruct delivered config sections
// Related: tree.go -- ToPluginMap; internal/core/configorder -- delivered list order
package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/configorder"
)

// TreeFromPluginMap reconstructs containers and keyed lists from the shape
// ToPluginMap delivers, before or after JSON transport. A map whose children
// are all maps exposes both views because the wire shape does not distinguish
// a container from a keyed list. Delivered list order is retained; an absent
// order is never invented for a multi-entry list. The caller MUST NOT mutate m
// until this call returns.
func TreeFromPluginMap(m map[string]any) (*Tree, error) {
	type pendingMap struct {
		source map[string]any
		target *Tree
	}
	root := NewTree()
	pending := []pendingMap{{source: m, target: root}}
	// Each map in the finite decoded config is visited once. An explicit queue
	// keeps externally supplied nesting off the call stack.
	for next := 0; next < len(pending); next++ {
		item := pending[next]
		for key, value := range item.source {
			if after, ok := strings.CutPrefix(key, configorder.KeyPrefix); ok {
				listName := after
				if _, ok := item.source[listName].(map[string]any); !ok {
					return nil, fmt.Errorf("%s: order has no keyed list", key)
				}
				continue
			}
			switch v := value.(type) {
			case string:
				item.target.Set(key, v)
				// A string can also be a singleton leaf-list; retain both views.
				item.target.AppendValue(key, v)
			case float64:
				item.target.Set(key, strconv.FormatFloat(v, 'f', -1, 64))
			case bool:
				item.target.Set(key, strconv.FormatBool(v))
			case map[string]any:
				child := NewTree()
				item.target.SetContainer(key, child)
				pending = append(pending, pendingMap{source: v, target: child})
				allMaps := len(v) > 0
				for _, entry := range v {
					if _, ok := entry.(map[string]any); !ok {
						allMaps = false
						break
					}
				}
				_, ordered := item.source[configorder.OrderKey(key)]
				if ordered {
					entries, err := configorder.Entries(item.source, key, "")
					if err != nil {
						return nil, err
					}
					for _, entry := range entries {
						item.target.listOrder[key] = append(item.target.listOrder[key], entry.Key)
					}
				}
				if !allMaps {
					continue
				}
				// Both trees are private until return. The queued visit fills
				// this shared map, so the two views reference the same entries.
				item.target.lists[key] = child.containers
				if len(v) == 1 {
					if !ordered {
						for name := range v {
							item.target.listOrder[key] = []string{name}
						}
					}
				}
			case []string:
				for _, text := range v {
					item.target.AppendValue(key, text)
				}
			case []any:
				for index, entry := range v {
					text, ok := entry.(string)
					if !ok {
						return nil, fmt.Errorf("%s[%d]: expected string, got %T", key, index, entry)
					}
					item.target.AppendValue(key, text)
				}
			default:
				return nil, fmt.Errorf("%s: unsupported plugin config value %T", key, value)
			}
		}
	}
	return root, nil
}
