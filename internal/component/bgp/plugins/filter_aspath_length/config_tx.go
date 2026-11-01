// Design: docs/architecture/core-design.md -- AS-path length policy filter

package filter_aspath_length

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newAsPathLengthConfigTx is the filter's side of the config transaction: the as-path-length filter definitions
// are its config. Nothing reads an installed set: a BGP chain judges a
// definition through a match instance compiled from the policy tree
// (condition.go). What the transaction keeps is the verify: a candidate the
// parser refuses is refused before any plugin applies it.
func newAsPathLengthConfigTx() *sdk.ConfigTx[map[string]*asPathLengthDef] {
	parse := policy.SectionParser("filter-aspath-length", errAsPathLengthInvalidBgpConfigJSON, parseAsPathLengthDefs)
	return sdk.NewConfigTx(map[string]*asPathLengthDef(nil), parse, installAsPathLengthDefs)
}

// installAsPathLengthDefs records what an install accepted. It holds no state, so it
// cannot fail.
func installAsPathLengthDefs(defs map[string]*asPathLengthDef) error {
	logger().Debug("configured", "filters", len(defs))
	return nil
}
