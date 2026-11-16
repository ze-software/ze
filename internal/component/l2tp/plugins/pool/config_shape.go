// Design: docs/architecture/l2tp/subscriber-session-model.md -- distinguish malformed pool blocks from explicit removal
// Related: config.go -- parses validated object and list shapes into descriptions

package l2tppool

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/configvalue"
)

// poolConfigObject separates an absent optional block from a malformed present
// block. Only absence returns present=false; a present object can be empty.
// All callers use maps decoded from one JSON section, so null is not a map.
func poolConfigObject(parent map[string]any, key, path string) (block map[string]any, present bool, err error) {
	value, present := parent[key]
	if !present {
		return nil, false, nil
	}
	block, ok := value.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%s must be an object; omit the block to remove it", path)
	}
	return block, true, nil
}

// poolConfigList verifies every present entry before ListEntries can skip a
// malformed body. An absent or empty list is a valid list with no entries.
func poolConfigList(parent map[string]any, key, path string) ([]configvalue.ListEntry, error) {
	byKey, _, err := poolConfigObject(parent, key, path)
	if err != nil {
		return nil, err
	}
	for name, value := range byKey {
		if _, ok := value.(map[string]any); !ok {
			return nil, fmt.Errorf("%s %s must be an object; omit the entry to remove it", path, configtext.QuoteCommandWord(name))
		}
	}
	return configvalue.ListEntries(byKey), nil
}
