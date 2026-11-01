// Design: docs/architecture/core-design.md -- prefix-list filter plugin

package filter_prefix

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newPrefixConfigTx is the filter's side of the config transaction: the prefix-list definitions
// are its config. Nothing reads an installed set: a BGP chain judges a
// definition through a match instance compiled from the policy tree
// (condition.go). What the transaction keeps is the verify: a candidate the
// parser refuses is refused before any plugin applies it.
func newPrefixConfigTx() *sdk.ConfigTx[map[string]*prefixList] {
	parse := policy.SectionParser("filter-prefix", errFilterPrefixInvalidBgpConfigJson, parsePrefixLists)
	return sdk.NewConfigTx(map[string]*prefixList(nil), parse, installPrefixLists)
}

// installPrefixLists records what an install accepted. It holds no state, so it
// cannot fail.
func installPrefixLists(defs map[string]*prefixList) error {
	logger().Debug("configured", "prefix-lists", len(defs))
	return nil
}
