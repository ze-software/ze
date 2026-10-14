// Design: docs/architecture/core-design.md -- remove-private-as policy action filter
// Related: filter_remove_private_as.go -- defsByName, the state an install replaces

package filter_remove_private_as

import (
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newRemovePrivateASConfigTx is the filter's side of the config transaction: the remove-private-as definitions
// are its config, and an install stores them whole, so a filter-update reads
// the old set or the new one and never a mix. Before any install none is
// defined.
func newRemovePrivateASConfigTx() *sdk.ConfigTx[map[string]*removePrivateASDef] {
	parse := configjson.BGPSectionParser("filter-remove-private-as", errRemovePrivateASInvalidBgpConfigJSON, parseRemovePrivateASDefs)
	return sdk.NewConfigTx(map[string]*removePrivateASDef(nil), parse, installRemovePrivateASDefs)
}

// installRemovePrivateASDefs makes the definitions every later filter-update reads. It
// cannot fail.
func installRemovePrivateASDefs(defs map[string]*removePrivateASDef) error {
	defsByName.Store(&defs)
	logger().Debug("configured", "remove-private-as", len(defs))
	return nil
}
