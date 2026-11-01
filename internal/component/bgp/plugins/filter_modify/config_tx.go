// Design: docs/architecture/core-design.md -- route modify filter
// Related: filter_modify.go -- defsByName, the state an install replaces
// Related: config.go -- parseModifyDefs, which builds each definition's sets

package filter_modify

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newModifyConfigTx is the filter's side of the config transaction: the
// modify definitions are its config, and an install stores them with one
// pointer, so a route reads the modifiers of one config and never a mix.
// Before any install none is defined and every route fails (evaluateModify).
func newModifyConfigTx() *sdk.ConfigTx[map[string]*modifyDef] {
	parse := policy.SectionParser("filter-modify", errFilterModifyInvalidBgpConfigJson, parseModifyDefs)
	return sdk.NewConfigTx(map[string]*modifyDef(nil), parse, installModifyDefs)
}

// installModifyDefs makes the definitions every later filter-update reads. It
// cannot fail.
func installModifyDefs(defs map[string]*modifyDef) error {
	defsByName.Store(&defs)
	logger().Debug("configured", "modifiers", len(defs))
	return nil
}
