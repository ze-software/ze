// Design: docs/architecture/core-design.md -- address-family policy filter
// Related: filter_family.go -- instancesByName, the state an install replaces

package filter_family

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newFamilyConfigTx is the filter's side of the config transaction: the
// family-filter instances are its config, and an install stores them whole,
// so a filter-update reads the old set or the new one and never a mix. Its
// verify is the parse, so a tear-down instance in an export chain (AC-7) is
// refused before any participant applies. Before any install none is defined.
func newFamilyConfigTx() *sdk.ConfigTx[map[string]*familyFilter] {
	parse := policy.SectionsParser("filter-family", errFilterFamilyInvalidBgpConfigJSON, configRootBGP, parseFamilyFilters)
	return sdk.NewConfigTx(map[string]*familyFilter(nil), parse, installFamilyFilters)
}

// installFamilyFilters makes the instances every later filter-update reads.
// It cannot fail.
func installFamilyFilters(instances map[string]*familyFilter) error {
	instancesByName.Store(&instances)
	logger().Debug("configured", "family-filters", len(instances))
	return nil
}
