// Design: docs/architecture/api/process-protocol.md -- a reload re-sends the capability declaration
// Overview: traverse.go -- BGP config JSON traversal helpers

package configjson

import (
	"errors"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// configRootBGP is the config root every bgp capability plugin declares from.
const configRootBGP = "bgp"

// errInvalidBGPSection refuses a bgp section whose data is not JSON: its
// capabilities cannot be read, and reading none would declare that the plugin
// advertises nothing.
var errInvalidBGPSection = errors.New("bgp config section is not valid JSON")

// BGPCapabilityExtractor returns the sdk.CapabilityExtractor of a plugin whose
// capabilities come from the bgp root. extract runs on each bgp section's data
// and returns that section's declarations.
func BGPCapabilityExtractor(extract func(data string) ([]sdk.CapabilityDecl, error)) sdk.CapabilityExtractor {
	return func(sections []sdk.ConfigSection) (sdk.CapabilityDeclaration, error) {
		var declaration sdk.CapabilityDeclaration
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			if _, ok := ParseBGPSubtree(section.Data); !ok {
				return sdk.CapabilityDeclaration{}, errInvalidBGPSection
			}
			caps, err := extract(section.Data)
			if err != nil {
				return sdk.CapabilityDeclaration{}, err
			}
			declaration.Capabilities = append(declaration.Capabilities, caps...)
		}
		return declaration, nil
	}
}
