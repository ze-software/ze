// Design: docs/architecture/core-design.md -- prefix-list filter plugin
// Related: filter_prefix.go -- listsByName, the state an install replaces

package filter_prefix

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newPrefixConfigTx is the filter's side of the config transaction: the
// prefix-list definitions are its config, and an install stores them whole.
// Before any install no list is defined.
func newPrefixConfigTx() *sdk.ConfigTx[map[string]*prefixList] {
	return sdk.NewConfigTx(map[string]*prefixList(nil), parsePrefixSections, installPrefixLists)
}

// parsePrefixSections reads the prefix-list definitions of the bgp section. A
// delivery with no bgp section carries nothing for the filter; a bgp section
// with no prefix-list defines none.
func parsePrefixSections(sections []sdk.ConfigSection) (map[string]*prefixList, bool, error) {
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
		if !ok {
			return nil, false, errFilterPrefixInvalidBgpConfigJson
		}
		lists, err := parsePrefixLists(bgpCfg)
		if err != nil {
			return nil, false, fmt.Errorf("filter-prefix: %w", err)
		}
		return lists, true, nil
	}
	return nil, false, nil
}

// installPrefixLists makes lists the definitions every later filter-update
// reads. It cannot fail.
func installPrefixLists(lists map[string]*prefixList) error {
	listsByName.Store(&lists)
	logger().Debug("configured", "prefix-lists", len(lists))
	return nil
}
