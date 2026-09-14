// Design: rir.go -- publishedDelegation is the one Go declaration of the five
//
// Goal: prove the registries a refresh reads are the registries an operator can
// name, so a token cannot exist on one side alone.
// Method: load the YANG model and compare the enumeration on the
// system/rir/delegation-source key against RegistryTokens().

package irr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"

	// The blank import registers ze-system-conf.yang, which declares the key
	// read below. Without it the loader answers a model with no system tree.
	_ "github.com/ze-software/ze/internal/component/config/system/yang"
)

// delegationSourceList is the list whose KEY carries the operator-facing
// declaration of the five tokens. The key leaf is not a child of the list, so
// it is read off ListNode.KeyLeaf rather than walked to.
const delegationSourceList = "system/rir/delegation-source"

// TestRegistryTokensMatchTheYANGEnumeration holds the delegation table to the
// model.
//
// Both sides name the five because both have to: publishedDelegation carries
// the URL each registry publishes, and the model has to refuse a misspelled
// one before it reaches delegationSourceURLs. That makes them a declared copy,
// and this is the check that compares them (ai/rules/principles.md).
func TestRegistryTokensMatchTheYANGEnumeration(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err, "the YANG schema must load")

	node, err := schema.Lookup(delegationSourceList)
	require.NoError(t, err, "the schema must declare %s", delegationSourceList)

	list, isList := node.(*config.ListNode)
	require.True(t, isList, "%s must be a list", delegationSourceList)
	require.NotNil(t, list.KeyLeaf, "%s must carry a key leaf", delegationSourceList)
	require.NotEmpty(t, list.KeyLeaf.Enums, "the %s key must carry an enumeration", list.KeyName)

	assert.ElementsMatch(t, list.KeyLeaf.Enums, RegistryTokens(),
		"a refresh reads exactly the registries %s lets an operator name", delegationSourceList)
}

// TestEveryRegistryTokenCarriesAPublishedFile proves the set is not merely the
// right size: each token an operator can name resolves to a URL, so a token
// added to the model without a published file cannot pass as supported.
func TestEveryRegistryTokenCarriesAPublishedFile(t *testing.T) {
	urls, err := delegationSourceURLs(nil)
	require.NoError(t, err, "no operator override reads every published file")
	assert.Len(t, urls, len(RegistryTokens()), "one published file for each registry")

	for _, token := range RegistryTokens() {
		assert.NotEmpty(t, rirWhois[token], "%q must name a whois server", token)
		assert.NotEmpty(t, rirNames[token], "%q must name a canonical registry", token)
	}
}
