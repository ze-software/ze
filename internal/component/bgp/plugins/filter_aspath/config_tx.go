// Design: docs/architecture/core-design.md -- AS-path regex filter plugin

package filter_aspath

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newAsPathConfigTx is the filter's side of the config transaction: the as-path-list definitions
// are its config. Nothing reads an installed set: a BGP chain judges a
// definition through a match instance compiled from the policy tree
// (condition.go). What the transaction keeps is the verify: a candidate the
// parser refuses is refused before any plugin applies it.
func newAsPathConfigTx() *sdk.ConfigTx[map[string]*aspathList] {
	parse := policy.SectionParser("filter-aspath", errFilterAspathInvalidBgpConfigJson, parseAsPathLists)
	return sdk.NewConfigTx(map[string]*aspathList(nil), parse, installAsPathLists)
}

// installAsPathLists records what an install accepted. It holds no state, so it
// cannot fail.
func installAsPathLists(defs map[string]*aspathList) error {
	logger().Debug("configured", "as-path-lists", len(defs))
	return nil
}
