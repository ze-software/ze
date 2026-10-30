// Design: docs/architecture/core-design.md -- route modify filter
// Related: filter_modify.go -- defsByName, the state an install replaces
// Related: config.go -- parseModifyDefs, which attaches the delivery's base

package filter_modify

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newModifyConfigTx is the filter's side of the config transaction: the
// modify definitions, each carrying its delivery's absent-attribute base, are
// its config, and an install stores them with one pointer, so a filter-update
// reads the modifiers and the base of one config and never a mix. Before any
// install none is defined and every filter-update is rejected.
func newModifyConfigTx() *sdk.ConfigTx[map[string]*modifyDef] {
	parse := policy.SectionsParser("filter-modify", errFilterModifyInvalidBgpConfigJson, configRootBGP, parseModifyDefs)
	return sdk.NewConfigTx(map[string]*modifyDef(nil), parse, installModifyDefs)
}

// installModifyDefs makes the definitions every later filter-update reads. It
// cannot fail.
func installModifyDefs(defs map[string]*modifyDef) error {
	defsByName.Store(&defs)
	logger().Debug("configured", "modifiers", len(defs))
	return nil
}
