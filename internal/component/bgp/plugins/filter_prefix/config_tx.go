// Design: docs/architecture/core-design.md -- prefix-list filter plugin
// Related: filter_prefix.go -- listsByName, the state an install replaces

package filter_prefix

import (
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newPrefixConfigTx is the filter's side of the config transaction: the prefix-list definitions
// are its config, and an install stores them whole, so a filter-update reads
// the old set or the new one and never a mix. Before any install none is
// defined.
func newPrefixConfigTx() *sdk.ConfigTx[map[string]*prefixList] {
	parse := configjson.BGPSectionParser("filter-prefix", errFilterPrefixInvalidBgpConfigJson, parsePrefixLists)
	return sdk.NewConfigTx(map[string]*prefixList(nil), parse, installPrefixLists)
}

// installPrefixLists makes the definitions every later filter-update reads. It
// cannot fail.
func installPrefixLists(defs map[string]*prefixList) error {
	listsByName.Store(&defs)
	logger().Debug("configured", "prefix-lists", len(defs))
	return nil
}
