// Design: docs/architecture/api/process-protocol.md -- capability declarations fixed for the process life
// Overview: traverse.go -- BGP config JSON traversal helpers

package configjson

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// configRootBGP is the config root every bgp capability plugin declares from.
const configRootBGP = "bgp"

// errInvalidBGPSection refuses a bgp section whose data is not JSON: its peers
// cannot be read, and reading none would accept a changed declaration.
var errInvalidBGPSection = errors.New("bgp config section is not valid JSON")

// BGPCapabilityExtractor returns the sdk.CapabilityExtractor of a plugin whose
// capabilities come from the bgp root. extract runs on each bgp section's data
// and returns that section's declarations; the candidate peers are every
// CapabilitySelector of the same tree.
func BGPCapabilityExtractor(extract func(data string) ([]sdk.CapabilityDecl, error)) sdk.CapabilityExtractor {
	return func(sections []sdk.ConfigSection) (sdk.CapabilityDeclaration, error) {
		var declaration sdk.CapabilityDeclaration
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			bgpTree, ok := ParseBGPSubtree(section.Data)
			if !ok {
				return sdk.CapabilityDeclaration{}, errInvalidBGPSection
			}
			caps, err := extract(section.Data)
			if err != nil {
				return sdk.CapabilityDeclaration{}, err
			}
			declaration.Capabilities = append(declaration.Capabilities, caps...)
			for _, selector := range CapabilitySelectors(bgpTree) {
				declaration.Peers = append(declaration.Peers, selector)
				group, isGroup := strings.CutPrefix(selector, GroupKeyPrefix)
				if !isGroup {
					continue
				}
				if declaration.Labels == nil {
					declaration.Labels = map[string]string{}
				}
				declaration.Labels[selector] = dynamicGroupLabel(group)
			}
		}
		return declaration, nil
	}
}

// dynamicGroupLabel names a dynamic group's template in a refusal. Its
// selector carries GroupKeyPrefix, and "peer group:ix" would send the operator
// to a peer that does not exist.
func dynamicGroupLabel(group string) string {
	var tb textbuf.Buffer
	return tb.Str("peers of dynamic group ").Str(group).String()
}
