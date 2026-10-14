// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: traverse.go -- ParseBGPSubtree, which unwraps the bgp section

package configjson

import (
	"fmt"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// rootBGP is the config root whose section a BGP plugin's parser reads.
const rootBGP = "bgp"

// BGPSectionParser builds the sdk.ConfigParser of a BGP plugin whose config
// is what parse reads from the bgp subtree. A delivery with no bgp section
// carries nothing for the plugin. A bgp section that is not a JSON object is
// refused with errInvalid, and a parse error is wrapped with the plugin's
// name, so the operator reads which plugin refused the commit.
func BGPSectionParser[T any](plugin string, errInvalid error, parse func(bgpCfg map[string]any) (T, error)) sdk.ConfigParser[T] {
	return func(sections []sdk.ConfigSection) (T, bool, error) {
		var none T
		for _, section := range sections {
			if section.Root != rootBGP {
				continue
			}
			bgpCfg, ok := ParseBGPSubtree(section.Data)
			if !ok {
				return none, false, errInvalid
			}
			cfg, err := parse(bgpCfg)
			if err != nil {
				return none, false, fmt.Errorf("%s: %w", plugin, err)
			}
			return cfg, true, nil
		}
		return none, false, nil
	}
}
