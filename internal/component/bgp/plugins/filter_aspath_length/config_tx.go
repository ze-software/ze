// Design: docs/architecture/core-design.md -- AS-path length policy filter
// Related: filter_aspath_length.go -- defsByName, the state an install replaces

package filter_aspath_length

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newAsPathLengthConfigTx is the filter's side of the config transaction: the as-path-length filter definitions
// are its config, and an install stores them whole, so a filter-update reads
// the old set or the new one and never a mix. Before any install none is
// defined.
func newAsPathLengthConfigTx() *sdk.ConfigTx[map[string]*asPathLengthDef] {
	parse := policy.SectionParser("filter-aspath-length", errAsPathLengthInvalidBgpConfigJSON, parseAsPathLengthDefs)
	return sdk.NewConfigTx(map[string]*asPathLengthDef(nil), parse, installAsPathLengthDefs)
}

// installAsPathLengthDefs makes the definitions every later filter-update reads. It
// cannot fail.
func installAsPathLengthDefs(defs map[string]*asPathLengthDef) error {
	defsByName.Store(&defs)
	logger().Debug("configured", "filters", len(defs))
	return nil
}
