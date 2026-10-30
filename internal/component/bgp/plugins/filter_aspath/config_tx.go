// Design: docs/architecture/core-design.md -- AS-path regex filter plugin
// Related: filter_aspath.go -- listsByName, the state an install replaces

package filter_aspath

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newAsPathConfigTx is the filter's side of the config transaction: the as-path-list definitions
// are its config, and an install stores them whole, so a filter-update reads
// the old set or the new one and never a mix. Before any install none is
// defined.
func newAsPathConfigTx() *sdk.ConfigTx[map[string]*aspathList] {
	parse := policy.SectionParser("filter-aspath", errFilterAspathInvalidBgpConfigJson, parseAsPathLists)
	return sdk.NewConfigTx(map[string]*aspathList(nil), parse, installAsPathLists)
}

// installAsPathLists makes the definitions every later filter-update reads. It
// cannot fail.
func installAsPathLists(defs map[string]*aspathList) error {
	listsByName.Store(&defs)
	logger().Debug("configured", "as-path-lists", len(defs))
	return nil
}
